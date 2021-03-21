package helpbot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/andersfylling/disgord"
	agpb "github.com/autograde/quickfeed/ag"
	"github.com/jinzhu/gorm"
	"google.golang.org/grpc/metadata"
)

type command func(m *disgord.MessageCreate)

type commandMap map[string]command

func (bot *HelpBot) initCommands() {
	bot.baseCommands = commandMap{
		"help":     bot.baseHelpCommand,
		"register": bot.registerCommand,
	}
	bot.studentCommands = commandMap{
		"help":    bot.studentHelpCommand,
		"gethelp": func(m *disgord.MessageCreate) { bot.helpRequestCommand(m, "help") },
		"approve": func(m *disgord.MessageCreate) { bot.helpRequestCommand(m, "approve") },
		"cancel":  bot.cancelRequestCommand,
		"status":  bot.studentStatusCommand,
	}
	bot.assistantCommands = commandMap{
		"help":       bot.assistantHelpCommand,
		"length":     bot.lengthCommand,
		"list":       bot.listCommand,
		"next":       bot.nextRequestCommand,
		"clear":      bot.clearCommand,
		"unregister": bot.unregisterCommand,
		"whois":      bot.whoIsCommand,
		"cancel":     bot.assistantCancelCommand,
	}
}

var privacy = `
Data collection and privacy:
By using this bot, you consent that your full name, student id, and discord id may be collected for the purposes of
identifying you on this server. If you wish to delete your data, please contact a teaching assistant.`

var baseHelp = createTemplate("baseHelp", `Available commands:
`+"```"+`
{{.Prefix}}help                       Shows this help text
{{.Prefix}}register [github username] Register your discord account as a student.
`+"```"+privacy)

var studentHelp = createTemplate("studentHelp", `Available commands:
`+"```"+`
{{.Prefix}}help:    Shows this help text
{{.Prefix}}gethelp: Request help from a teaching assistant
{{.Prefix}}approve: Get your lab approved by a teaching assistant
{{.Prefix}}cancel:  Cancels your help request and removes you from the queue
{{.Prefix}}status:  Show your position in the queue
`+"```"+`
After requesting help, you can check the response message you got to see your position in the queue.
You will receive a message when you are next in queue.
`+privacy)

var assistant = createTemplate("assistantHelp", `Teaching Assistant commands:
`+"```"+`
{{.Prefix}}help:               Shows this help text
{{.Prefix}}length:             Returns the number of students waiting for help.
{{.Prefix}}list <num>:         Lists the next <num> students in the queue.
{{.Prefix}}next:               Removes and returns the first student from the queue.
{{.Prefix}}clear:              Clears the queue!
{{.Prefix}}unregister @mention Unregisters the mentioned user.
{{.Prefix}}whois @mention      Returns the real name of the mentioned user.
{{.Prefix}}cancel              Cancels your 'waiting' status.
`+"```"+privacy)

func (bot *HelpBot) helpCommand(m *disgord.MessageCreate, helpTmpl *template.Template) error {
	buf := new(strings.Builder)
	err := helpTmpl.Execute(buf, bot.cfg)
	if err != nil {
		return fmt.Errorf("helpCommand: failed to execute template: %w", err)
	}
	replyMsg(bot.client, m, buf.String())
	return nil
}

func (bot *HelpBot) baseHelpCommand(m *disgord.MessageCreate) {
	err := bot.helpCommand(m, baseHelp)
	if err != nil {
		bot.log.Error(err)
	}
}

func (bot *HelpBot) studentHelpCommand(m *disgord.MessageCreate) {
	err := bot.helpCommand(m, studentHelp)
	if err != nil {
		bot.log.Error(err)
	}
}

func (bot *HelpBot) assistantHelpCommand(m *disgord.MessageCreate) {
	err := bot.helpCommand(m, assistant)
	if err != nil {
		bot.log.Error(err)
	}
}

func (bot *HelpBot) helpRequestCommand(m *disgord.MessageCreate, requestType string) {
	// create a transaction such that getPos... and Create... are performed atomically
	tx := bot.db.Begin()
	defer tx.RollbackUnlessCommitted()

	// check if an open request already exists
	pos, err := getPosInQueue(tx, m.Message.Author.ID)
	if err != nil {
		bot.log.Errorln("helpRequest: failed to get user pos in queue")
		replyMsg(bot.client, m, "An error occurred while creating your request.")
		return
	}

	// already in the queue, no need to do anything.
	if pos > 0 {
		replyMsg(bot.client, m, fmt.Sprintf("You are already at postition %d in the queue", pos))
		return
	}

	req := HelpRequest{
		StudentUserID: m.Message.Author.ID,
		Type:          requestType,
		Done:          false,
	}

	err = tx.Create(&req).Error
	if err != nil {
		bot.log.Errorln("helpRequest: failed to create new request:", err)
		replyMsg(bot.client, m, "An error occurred while creating your request.")
		return
	}

	if bot.assignToIdleAssistant(m.Ctx, tx, req) {
		tx.Commit()
		return
	}

	pos, err = getPosInQueue(tx, m.Message.Author.ID)
	if err != nil {
		bot.log.Errorln("helpRequest: failed to get pos in queue after creating request:", err)
		replyMsg(bot.client, m, "An error occurred while creating your request.")
		return
	}
	tx.Commit()

	replyMsg(bot.client, m, fmt.Sprintf("A help request has been created, and you are at position %d in the queue.", pos))
}

func (bot *HelpBot) studentStatusCommand(m *disgord.MessageCreate) {
	pos, err := getPosInQueue(bot.db, m.Message.Author.ID)
	if err != nil {
		bot.log.Errorln("studentStatus: failed to get position in queue:", err)
		replyMsg(bot.client, m, "An error occurred.")
		return
	}
	if pos <= 0 {
		replyMsg(bot.client, m, "You are not in the queue.")
		return
	}
	replyMsg(bot.client, m, fmt.Sprintf("You are at position %d in the queue.", pos))
}

// assignToIdleAssistant will check if any assistants are waiting for a request and pick one of them to handle req.
// db must be a transaction.
func (bot *HelpBot) assignToIdleAssistant(ctx context.Context, db *gorm.DB, req HelpRequest) bool {
	err := db.Where("waiting = ?", true).Order("last_request ASC").First(&req.Assistant).Error
	if gorm.IsRecordNotFoundError(err) {
		return false
	} else if err != nil {
		bot.log.Errorln("Failed to query for idle assistants:", err)
		return false
	}

	studUser, err := bot.client.GetMember(ctx, bot.cfg.Guild, req.StudentUserID)
	if err != nil {
		bot.log.Errorln("Failed to retrieve user info for student:", err)
		return false
	}

	assistantUser, err := bot.client.GetMember(ctx, bot.cfg.Guild, req.Assistant.UserID)
	if err != nil {
		bot.log.Errorln("Failed to retrieve user info for assistant:", err)
		return false
	}

	req.Assistant.LastRequest = time.Now()
	req.Assistant.Waiting = false
	req.Done = true
	req.DoneAt = req.Assistant.LastRequest
	req.Reason = "assistantNext"

	// need to do this update assistant manually, as zero-valued struct fields are ignored
	err = db.Model(&Assistant{}).Where("user_id = ?", req.Assistant.UserID).UpdateColumns(map[string]interface{}{
		"waiting":      req.Assistant.Waiting,
		"last_request": req.Assistant.LastRequest,
	}).Error
	if err != nil {
		bot.log.Errorln("Failed to update assistant status:", err)
		return false
	}

	err = db.Model(&HelpRequest{}).Update(&req).Error
	if err != nil {
		bot.log.Errorln("Failed to update request:", err)
		return false
	}

	if !sendMsg(ctx, bot.client, assistantUser.User, fmt.Sprintf("Next '%s' request is by %s", req.Type,
		getMentionAndNick(studUser))) {
		return false
	}

	if !sendMsg(ctx, bot.client, studUser.User, fmt.Sprintf("You will now receive help from %s.",
		getMentionAndNick(assistantUser))) {
		return false
	}

	return true
}

func getPosInQueue(db *gorm.DB, userID disgord.Snowflake) (rowNumber int, err error) {
	err = db.Raw(`
		select row_number from (
			select
				row_number () over (
					order by created_at asc
				) row_number,
				student_user_id
			from
				help_requests
			where
				done = false
		) t
		where
			student_user_id = ?`,
		userID,
	).Row().Scan(&rowNumber)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return -1, fmt.Errorf("getPosInQueue error: %w", err)
	}

	return rowNumber, nil
}

func (bot *HelpBot) cancelRequestCommand(m *disgord.MessageCreate) {
	err := bot.db.Model(&HelpRequest{}).Where("student_user_id = ?", m.Message.Author.ID).Updates(map[string]interface{}{
		"done":    true,
		"reason":  "userCancel",
		"done_at": time.Now(),
	}).Error
	if gorm.IsRecordNotFoundError(err) {
		replyMsg(bot.client, m, "You do not have an active help request.")
		return
	} else if err != nil {
		replyMsg(bot.client, m, "An unknown error occurred.")
		return
	}
	replyMsg(bot.client, m, "Your request was cancelled.")
}

func (bot *HelpBot) nextRequestCommand(m *disgord.MessageCreate) {
	// register assistant in DB
	var assistant Assistant
	err := bot.db.Where("user_id = ?", m.Message.Author.ID).First(&assistant).Error
	if gorm.IsRecordNotFoundError(err) {
		err := bot.db.Create(&Assistant{
			UserID:  m.Message.Author.ID,
			Waiting: false,
		}).Error
		if err != nil {
			bot.log.Errorln("Failed to create assistant record:", err)
		}
	} else if err != nil {
		bot.log.Errorln("Failed to store assistant in DB:", err)
	}

	tx := bot.db.Begin()
	defer tx.RollbackUnlessCommitted()

	var req HelpRequest
	err = tx.Where("done = ?", false).Order("created_at asc").First(&req).Error
	if gorm.IsRecordNotFoundError(err) {
		assistant.UserID = m.Message.Author.ID
		assistant.Waiting = true
		err := tx.Model(&Assistant{}).Update(&assistant).Error
		if err != nil {
			bot.log.Errorln("Failed to update waiting state for assistant:", err)
			replyMsg(bot.client, m, "There are no more requests in the queue, but due to an error, you won't receive a notification when the next one arrives.")
			return
		}
		if !replyMsg(bot.client, m, "There are no more requests in the queue. You will receive a message when the next request arrives.") {
			return
		}
		tx.Commit()
		return
	} else if err != nil {
		bot.log.Errorln("Failed to get next user:", err)
		replyMsg(bot.client, m, "An unknown error occurred.")
		return
	}

	req.Assistant = assistant
	req.Assistant.LastRequest = time.Now()
	req.Done = true
	req.DoneAt = req.Assistant.LastRequest
	req.Reason = "assistantNext"

	err = tx.Model(&HelpRequest{}).Update(&req).Error
	if err != nil {
		bot.log.Errorln("Failed to update request:", err)
		replyMsg(bot.client, m, "An error occurred while updating request.")
		return
	}

	err = tx.Model(&Assistant{}).Update(&req.Assistant).Error
	if err != nil {
		bot.log.Errorln("Failed to update assistant:", err)
		replyMsg(bot.client, m, "An error occurred while updating assistant status.")
		return
	}

	student, err := bot.client.GetMember(m.Ctx, bot.cfg.Guild, req.StudentUserID)
	if err != nil {
		bot.log.Errorln("Failed to fetch user:", err)
		replyMsg(bot.client, m, "An unknown error occurred.")
		return
	}

	assistantMember, err := bot.client.GetMember(m.Ctx, bot.cfg.Guild, m.Message.Author.ID)
	if err != nil {
		bot.log.Errorln("Failed to fetch user:", err)
		replyMsg(bot.client, m, "An unknown error occurred.")
		return
	}

	if !replyMsg(bot.client, m, fmt.Sprintf("Next '%s' request is by %s.", req.Type, getMentionAndNick(student))) {
		return
	}

	sendMsg(m.Ctx, bot.client, student.User, fmt.Sprintf("You will now receive help from %s", getMentionAndNick(assistantMember)))

	tx.Commit()
}

func (bot *HelpBot) lengthCommand(m *disgord.MessageCreate) {
	var length int
	err := bot.db.Model(&HelpRequest{}).Where("done = ?", false).Count(&length).Error
	if err != nil {
		bot.log.Errorln("Failed to count number of open requests:", err)
		replyMsg(bot.client, m, "An error occurred.")
		return
	}

	var msg string
	if length == 1 {
		msg = "There is 1 student waiting for help."
	} else {
		msg = fmt.Sprintf("There are %d students waiting for help.", length)
	}
	replyMsg(bot.client, m, msg)
}

func (bot *HelpBot) listCommand(m *disgord.MessageCreate) {
	words := strings.Fields(m.Message.Content)

	num := 10
	var err error

	if len(words) >= 2 {
		num, err = strconv.Atoi(words[1])
		if err != nil {
			replyMsg(bot.client, m, fmt.Sprintf("'%s' is not a vaild number.", words[1]))
			return
		}
	}

	var sb strings.Builder
	var requests []HelpRequest
	err = bot.db.Where("done = ?", false).Order("created_at asc").Limit(num).Find(&requests).Error
	if err != nil {
		bot.log.Errorln("Failed to get requests:", err)
		replyMsg(bot.client, m, "Failed to get list of requests.")
		return
	}

	if len(requests) == 0 {
		replyMsg(bot.client, m, "There are no open requests.")
		return
	}

	fmt.Fprintf(&sb, "Showing the next %d requests:\n\n", len(requests))
	for i, req := range requests {
		user, err := bot.client.GetMember(m.Ctx, bot.cfg.Guild, req.StudentUserID)
		if err != nil {
			bot.log.Errorln("Failed to obtain user info:", err)
			replyMsg(bot.client, m, "An error occurred while sending the message")
			return
		}
		fmt.Fprintf(&sb, "%d. User: %s, Type: %s\n", i+1, getMentionAndNick(user), req.Type)
	}
	replyMsg(bot.client, m, sb.String())
}

func (bot *HelpBot) clearCommand(m *disgord.MessageCreate) {
	words := strings.Fields(m.Message.Content)
	if len(words) < 2 || words[1] != "YES" {
		replyMsg(bot.client, m, fmt.Sprintf(
			"This command will cancel all the requests in the queue. If you really want to do this, type `%sclear YES`",
			bot.cfg.Prefix,
		))
		return
	}

	// TODO: send a message to each student whose request was cleared.
	err := bot.db.Model(&HelpRequest{}).Where("done = ? ", false).Updates(map[string]interface{}{
		"done":              true,
		"done_at":           time.Now(),
		"assistant_user_id": m.Message.Author.ID,
		"reason":            "assistantClear",
	}).Error
	if err != nil {
		bot.log.Errorln("Failed to clear queue:", err)
		replyMsg(bot.client, m, "Clear failed due to an error.")
		return
	}

	replyMsg(bot.client, m, "The queue was cleared.")
}

func (bot *HelpBot) registerCommand(m *disgord.MessageCreate) {
	words := strings.Fields(m.Message.Content)
	if len(words) < 2 {
		replyMsg(bot.client, m, "You must include your github username in the command.")
		return
	}

	githubLogin := words[1]

	// only allow one user per github login
	var count int
	err := bot.db.Model(&Student{}).Where("github_login = ?", githubLogin).Count(&count).Error
	if err != nil {
		bot.log.Errorln("Failed to check for existing user:", err)
		replyMsg(bot.client, m, "An unknown error occurred.")
		return
	}

	if count > 0 {
		replyMsg(bot.client, m, "That github login has already been used to register! If you believe that this is a mistake, please contact a teaching assistant.")
		return
	}

	membership, _, err := bot.gh.Organizations.GetOrgMembership(m.Ctx, githubLogin, bot.cfg.GitHubOrg)
	if err != nil {
		bot.log.Infof("Failed to get org membership for user '%s': %v\n", githubLogin, err)
		replyMsg(bot.client, m,
			"We were unable to verify that you are a member of the course's GitHub organization")
		return
	}

	if membership.GetState() != "active" {
		replyMsg(bot.client, m, fmt.Sprintf(
			"Please make sure that you have accepted the invitation to join the '%s' organization on GitHub",
			bot.cfg.GitHubOrg))
		return
	}

	student := Student{
		UserID:      m.Message.Author.ID,
		GithubLogin: githubLogin,
	}

	// TODO: query autograder for real name, using github login for now
	if bot.cfg.Autograder {
		ctx, cancel := context.WithTimeout(m.Ctx, 1*time.Second)
		defer cancel()
		ctx = metadata.NewOutgoingContext(ctx, bot.ag.md)
		req := &agpb.CourseUserRequest{
			CourseCode: bot.cfg.CourseCode,
			CourseYear: bot.cfg.CourseYear,
			UserLogin:  githubLogin,
		}
		userInfo, err := bot.ag.GetUserByCourse(ctx, req)
		if err != nil {
			bot.log.Errorln("Failed to get info from autograder:", err)
			replyMsg(bot.client, m, "Failed to communicate with autograder")
			return
		}
		student.Name = userInfo.GetName()
		student.StudentID = userInfo.GetStudentID()
	}

	// assign roles to student
	gm, err := bot.client.GetMember(m.Ctx, bot.cfg.Guild, m.Message.Author.ID)
	if err != nil {
		bot.log.Errorln("Failed to get member info:", err)
		replyMsg(bot.client, m, "An unknown error occurred")
		return
	}

	err = bot.db.Create(&student).Error
	if err != nil {
		bot.log.Errorln("Failed to store student in database:", err)
		replyMsg(bot.client, m, "An uknown error occurred.")
		return
	}

	err = gm.UpdateNick(m.Ctx, bot.client, student.Name)
	if err != nil {
		bot.log.Errorln("Failed to set nick:", err)
		replyMsg(bot.client, m, "An uknown error occurred")
		return
	}

	err = bot.client.AddGuildMemberRole(m.Ctx, bot.cfg.Guild, m.Message.Author.ID, bot.cfg.StudentRole)
	if err != nil {
		bot.log.Errorln("Failed to add student role:", err)
		replyMsg(bot.client, m, "An uknown error occurred")
		return
	}

	replyMsg(bot.client, m, fmt.Sprintf(
		"Authentication was successful! You should now have more access to the server. Type %shelp to see available commands",
		bot.cfg.Prefix))
}

func (bot *HelpBot) whoIsCommand(m *disgord.MessageCreate) {
	if len(m.Message.Mentions) < 1 {
		replyMsg(bot.client, m, "You must `@mention` a user to get the real name of.")
		return
	}
	user := m.Message.Mentions[0]
	var student Student
	if !bot.cfg.Autograder {
		replyMsg(bot.client, m, "This bot is not linked with Autograder.")
		return
	}
	err := bot.db.Where("user_id = ?", user.ID).First(&student).Error
	if err != nil {
		bot.log.Errorln("Failed to retrieve real name from db:", err)
		replyMsg(bot.client, m, "An uknown error occurred")
		return
	}

	replyMsg(bot.client, m, fmt.Sprintf(
		"@%s is %s",
		user.Username, student.Name))

}

func (bot *HelpBot) unregisterCommand(m *disgord.MessageCreate) {
	if len(m.Message.Mentions) < 1 {
		replyMsg(bot.client, m, "You must `@mention` a user to unregister.")
		return
	}

	user := m.Message.Mentions[0]

	// permanent deletion from db
	err := bot.db.Unscoped().Delete(&Student{}, "user_id = ?", user.ID).Error
	if err != nil {
		replyMsg(bot.client, m, "Failed to delete user info.")
		bot.log.Errorln("Failed to delete student info:", err)
		return
	}

	gm, err := bot.client.GetMember(m.Ctx, bot.cfg.Guild, user.ID)
	if err != nil {
		replyMsg(bot.client, m, "Failed to get member info. You may have to remove role/nickname manually.")
		bot.log.Errorln("Failed to get member info:", err)
		return
	}

	// clear nick
	err = gm.UpdateNick(m.Ctx, bot.client, "")
	if err != nil {
		replyMsg(bot.client, m, "Failed to remove user nick. You may have to remove role/nickname manually.")
		bot.log.Errorln("Failed to remove user nick:", err)
		return
	}

	// unassign role
	err = bot.client.RemoveGuildMemberRole(m.Ctx, bot.cfg.Guild, user.ID, bot.cfg.StudentRole)
	if err != nil {
		replyMsg(bot.client, m, "Failed to remove user roles. You may have to remove role/nickname manually.")
		bot.log.Errorln("Failed to remove user roles:", err)
		return
	}

	replyMsg(bot.client, m, "User was unregistered.")
}

func (bot *HelpBot) assistantCancelCommand(m *disgord.MessageCreate) {
	tx := bot.db.Begin()
	defer tx.RollbackUnlessCommitted()

	var assistant Assistant
	err := tx.Where("user_id = ?", m.Message.Author.ID).First(&assistant).Error
	if err != nil {
		bot.log.Errorln("Failed to get assistant from DB:", err)
		replyMsg(bot.client, m, "An unknown error occurred")
		return
	}

	if !assistant.Waiting {
		replyMsg(bot.client, m, "You were not marked as waiting, so no action was taken.")
		return
	}

	err = tx.Model(&Assistant{}).Where("user_id = ?", m.Message.Author.ID).UpdateColumn("waiting", false).Error
	if err != nil {
		bot.log.Errorln("Failed to update status in DB:", err)
		replyMsg(bot.client, m, "An unknown error occurred when attempting to update waiting status.")
		return
	}

	tx.Commit()

	replyMsg(bot.client, m, fmt.Sprintf("Your waiting status was removed (you will have to use %snext again to get the next student)", bot.cfg.Prefix))
}

package helpbot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/bwmarrin/discordgo"
	"github.com/jinzhu/gorm"
	qfpb "github.com/quickfeed/quickfeed/qf"
	"google.golang.org/grpc/metadata"
)

type command func(m *discordgo.InteractionCreate)

type commandMap map[string]command

func (bot *HelpBot) initCommands() {
	bot.commands = commandMap{
		// base commands
		"help":      bot.helpCommand,
		"register":  bot.registerCommand,
		"configure": bot.configureCommand,

		// student commands
		"gethelp": bot.hasRole(func(m *discordgo.InteractionCreate) { bot.helpRequestCommand(m, "help") }, RoleStudent),
		"approve": bot.hasRole(func(m *discordgo.InteractionCreate) { bot.helpRequestCommand(m, "approve") }, RoleStudent),
		"cancel":  bot.hasRole(bot.cancelRequestCommand, RoleStudent),
		"status":  bot.hasRole(bot.studentStatusCommand, RoleStudent),

		// assistant commands
		"length":         bot.hasRole(bot.lengthCommand, RoleAssistant),
		"list":           bot.hasRole(bot.listCommand, RoleAssistant),
		"next":           bot.hasRole(bot.nextRequestCommand, RoleAssistant),
		"clear":          bot.hasRole(bot.clearCommand, RoleAssistant),
		"unregister":     bot.hasRole(bot.unregisterCommand, RoleAssistant),
		"cancel-waiting": bot.hasRole(bot.assistantCancelCommand, RoleAssistant),
	}
}

var baseHelp = createModal("Available commands",
	``+"```"+`
help:                       Shows this help text
register [course] [GitHub username]: Register your discord account as a student.
`+"```", true)

var studentHelp = createModal("Student Commands",
	``+"```"+`
help:    Shows this help text
gethelp: Request help from a teaching assistant
approve: Get your lab approved by a teaching assistant
cancel:  Cancels your help request and removes you from the queue
status:  Show your position in the queue
`+"```"+`
After requesting help, you can check the response message you got to see your position in the queue.
You will receive a message when you are next in queue.
`, true)

var assistant = createModal("Teaching Assistant Commands",
	``+"```"+`
help:               Shows this help text
length:             Returns the number of students waiting for help.
list <num>:         Lists the next <num> students in the queue.
next:               Removes and returns the first student from the queue.
clear:              Clears the queue!
unregister @mention Unregisters the mentioned user.
cancel              Cancels your 'waiting' status.
`+"```", true)

func (bot *HelpBot) helpCommand(m *discordgo.InteractionCreate) {
	// check if the user has the teaching assistant role
	if bot.hasRoles(m.GuildID, m.Member, RoleAssistant) {
		replyModal(bot.client, m, assistant)
		return
	}
	// check if the user has the student role
	if bot.hasRoles(m.GuildID, m.Member, RoleStudent) {
		replyModal(bot.client, m, studentHelp)
		return
	}
	// user has no roles, show base help
	replyModal(bot.client, m, baseHelp)
}

func (bot *HelpBot) helpRequestCommand(m *discordgo.InteractionCreate, requestType string) {
	// create a transaction such that getPos... and Create... are performed atomically
	tx := bot.db.Begin()
	defer tx.RollbackUnlessCommitted()

	// check if an open request already exists
	pos, err := getPosInQueue(tx, m.Member.User.ID)
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
		StudentUserID: m.Member.User.ID,
		Type:          requestType,
		Done:          false,
	}

	err = tx.Create(&req).Error
	if err != nil {
		bot.log.Errorln("helpRequest: failed to create new request:", err)
		replyMsg(bot.client, m, "An error occurred while creating your request.")
		return
	}

	if bot.assignToIdleAssistant(context.Background(), m.GuildID, tx, req) {
		tx.Commit()
		return
	}

	pos, err = getPosInQueue(tx, m.Member.User.ID)
	if err != nil {
		bot.log.Errorln("helpRequest: failed to get pos in queue after creating request:", err)
		replyMsg(bot.client, m, "An error occurred while creating your request.")
		return
	}
	tx.Commit()

	replyMsg(bot.client, m, fmt.Sprintf("A help request has been created, and you are at position %d in the queue.", pos))
}

func (bot *HelpBot) studentStatusCommand(m *discordgo.InteractionCreate) {
	pos, err := getPosInQueue(bot.db, m.Member.User.ID)
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
func (bot *HelpBot) assignToIdleAssistant(ctx context.Context, guildID string, db *gorm.DB, req HelpRequest) bool {
	err := db.Where("waiting = ?", true).Order("last_request ASC").First(&req.Assistant).Error
	if gorm.IsRecordNotFoundError(err) {
		return false
	} else if err != nil {
		bot.log.Errorln("Failed to query for idle assistants:", err)
		return false
	}

	studUser, err := bot.client.GuildMember(guildID, req.StudentUserID)
	if err != nil {
		bot.log.Errorln("Failed to retrieve user info for student:", err)
		return false
	}

	assistantUser, err := bot.client.GuildMember(guildID, req.Assistant.UserID)
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

	if !sendMsg(bot.client, assistantUser.User, fmt.Sprintf("Next '%s' request is by %s", req.Type,
		getMentionAndNick(studUser))) {
		return false
	}

	if !sendMsg(bot.client, studUser.User, fmt.Sprintf("You will now receive help from %s.",
		getMentionAndNick(assistantUser))) {
		return false
	}

	return true
}

func getPosInQueue(db *gorm.DB, userID string) (rowNumber int, err error) {
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

func (bot *HelpBot) cancelRequestCommand(m *discordgo.InteractionCreate) {
	err := bot.db.Model(&HelpRequest{}).Where("student_user_id = ?", m.Member.User.ID).Updates(map[string]interface{}{
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

func (bot *HelpBot) nextRequestCommand(m *discordgo.InteractionCreate) {
	// register assistant in DB
	var assistant Assistant
	err := bot.db.Where("user_id = ?", m.Member.User.ID).First(&assistant).Error
	if gorm.IsRecordNotFoundError(err) {
		err := bot.db.Create(&Assistant{
			UserID:  m.Member.User.ID,
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

	student, err := bot.client.GuildMember(m.GuildID, req.StudentUserID)
	if err != nil {
		bot.log.Errorln("Failed to fetch user:", err)
		replyMsg(bot.client, m, "An unknown error occurred.")
		return
	}

	if !replyMsg(bot.client, m, fmt.Sprintf("Next '%s' request is by %s.", req.Type, getMentionAndNick(student))) {
		return
	}

	sendMsg(bot.client, student.User, fmt.Sprintf("You will now receive help from %s", getMentionAndNick(m.Member)))

	tx.Commit()
}

func (bot *HelpBot) lengthCommand(m *discordgo.InteractionCreate) {
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

func (bot *HelpBot) listCommand(m *discordgo.InteractionCreate) {
	num := 10

	options := m.ApplicationCommandData().Options

	var err error

	if len(options) >= 1 {
		num = int(options[0].IntValue())
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
		user, err := bot.client.GuildMember(m.GuildID, req.StudentUserID)
		if err != nil {
			bot.log.Errorln("Failed to obtain user info:", err)
			replyMsg(bot.client, m, "An error occurred while sending the message")
			return
		}
		fmt.Fprintf(&sb, "%d. User: %s, Type: %s\n", i+1, getMentionAndNick(user), req.Type)
	}
	replyMsg(bot.client, m, sb.String())
}

func (bot *HelpBot) clearCommand(m *discordgo.InteractionCreate) {
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

func (bot *HelpBot) registerCommand(m *discordgo.InteractionCreate) {

	// Check if role exists
	studentRole := bot.GetRole(m.GuildID, RoleStudent)
	if len(studentRole) == 0 {
		fmt.Printf("Failed to find student role for register command. Message: %+v Data: %+v\n", m.Message, m.Data)
		replyMsg(bot.client, m, "Failed to find student role.")
		return
	}

	if len(m.ApplicationCommandData().Options) == 0 {
		fmt.Printf("No github login provided for register command. Message: %+v Data: %+v\n", m.Message, m.Data)
		replyMsg(bot.client, m, "You must include your github username in the command.")
		return
	}
	githubLogin, ok := m.ApplicationCommandData().Options[0].Value.(string)
	if !ok {
		fmt.Printf("Failed to parse github login for register command. Message: %+v Data: %+v\n", m.Message, m.Data)
		replyMsg(bot.client, m, "You must include your github username in the command.")
		return
	}

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

	membership, _, err := bot.gh.Organizations.GetOrgMembership(context.Background(), githubLogin, bot.cfg.GitHubOrg)
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
		UserID:      m.Member.User.ID,
		GithubLogin: githubLogin,
	}

	// TODO: query autograder for real name, using github login for now
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	ctx = metadata.NewOutgoingContext(ctx, bot.qf.md)
	req := &qfpb.EnrollmentRequest{
		FetchMode: &qfpb.EnrollmentRequest_CourseID{CourseID: 1},
	}
	enrollments, err := bot.qf.qf.GetEnrollments(ctx, connect.NewRequest(req))
	if err != nil {
		bot.log.Errorln("Failed to get info from autograder:", err)
		replyMsg(bot.client, m, "Failed to communicate with autograder")
		return
	}

	found := false
	for _, e := range enrollments.Msg.GetEnrollments() {
		fmt.Println(e.GetUser().GetLogin())
		if e.GetUser().GetLogin() == githubLogin {
			student.Name = e.GetUser().GetName()
			student.StudentID = e.GetUser().GetStudentID()
			found = true
			break
		}
	}

	if !found {
		replyMsg(bot.client, m, "Failed to find your enrollment in the course")
		return
	}

	// assign roles to student

	err = bot.db.Create(&student).Error
	if err != nil {
		bot.log.Errorln("Failed to store student in database:", err)
		replyMsg(bot.client, m, "An uknown error occurred.")
		return
	}

	if err := bot.client.GuildMemberNickname(m.GuildID, student.UserID, student.Name); err != nil {
		bot.log.Errorln("Failed to set nick:", err)
		replyMsg(bot.client, m, "An uknown error occurred")
		return
	}

	if err := bot.client.GuildMemberRoleAdd(m.GuildID, student.UserID, studentRole); err != nil {
		bot.log.Errorln("Failed to add student role:", err)
		replyMsg(bot.client, m, "An uknown error occurred")
		return
	}

	replyMsg(bot.client, m, fmt.Sprintf(
		"Authentication was successful! You should now have more access to the server. Type %shelp to see available commands",
		bot.cfg.Prefix))
}

func (bot *HelpBot) unregisterCommand(m *discordgo.InteractionCreate) {
	if m.Member == nil {
		replyMsg(bot.client, m, "You must be a member of the server to use this command.")
		return
	}

	user := m.Member.User

	// permanent deletion from db
	err := bot.db.Unscoped().Delete(&Student{}, "user_id = ?", user.ID).Error
	if err != nil {
		replyMsg(bot.client, m, "Failed to delete user info.")
		bot.log.Errorln("Failed to delete student info:", err)
		return
	}

	// remove nickname
	if err := bot.client.GuildMemberNickname(m.GuildID, user.ID, ""); err != nil {
		replyMsg(bot.client, m, "Failed to remove user nick. You may have to remove role/nickname manually.")
		bot.log.Errorln("Failed to remove user nick:", err)
		return
	}

	// unassign role
	if err := bot.client.GuildMemberRoleRemove(m.GuildID, user.ID, bot.cfg.StudentRole); err != nil {
		replyMsg(bot.client, m, "Failed to remove user roles. You may have to remove role/nickname manually.")
		bot.log.Errorln("Failed to remove user roles:", err)
		return
	}
	replyMsg(bot.client, m, "User was unregistered.")
}

func (bot *HelpBot) assistantCancelCommand(m *discordgo.InteractionCreate) {
	tx := bot.db.Begin()
	defer tx.RollbackUnlessCommitted()

	var assistant Assistant
	err := tx.Where("user_id = ?", m.Member.User.ID).First(&assistant).Error
	if err != nil {
		bot.log.Errorln("Failed to get assistant from DB:", err)
		replyMsg(bot.client, m, "An unknown error occurred")
		return
	}

// hasRole returns a function that checks if the user has the specified role, and then calls the original function.
// If the user does not have the role, a message is sent to the user saying that they do not have permission to use the command.
// This function is used to wrap the command functions to check if the user has the required role.
func (bot *HelpBot) hasRole(f func(*discordgo.InteractionCreate), roles ...string) func(*discordgo.InteractionCreate) {
	return func(m *discordgo.InteractionCreate) {
		if !bot.hasRoles(m.GuildID, m.Member, roles...) {
			replyMsg(bot.client, m, "You do not have permission to use this command.")
			return
		}
		f(m)
	}
}

	if !assistant.Waiting {
		replyMsg(bot.client, m, "You were not marked as waiting, so no action was taken.")
		return
	}

	err = tx.Model(&Assistant{}).Where("user_id = ?", m.Member.User.ID).UpdateColumn("waiting", false).Error
	if err != nil {
		bot.log.Errorln("Failed to update status in DB:", err)
		replyMsg(bot.client, m, "An unknown error occurred when attempting to update waiting status.")
		return
	}

	tx.Commit()

	replyMsg(bot.client, m, fmt.Sprintf("Your waiting status was removed (you will have to use %snext again to get the next student)", bot.cfg.Prefix))
}

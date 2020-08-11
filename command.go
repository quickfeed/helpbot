package main

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
	"github.com/jinzhu/gorm"
)

type command func(s disgord.Session, m *disgord.MessageCreate)

type commandMap map[string]command

var (
	baseCommands = commandMap{
		"help":     baseHelpCommand,
		"register": registerCommand,
	}
	studentCommands = commandMap{
		"help":    studentHelpCommand,
		"gethelp": func(s disgord.Session, m *disgord.MessageCreate) { helpRequestCommand(s, m, "help") },
		"approve": func(s disgord.Session, m *disgord.MessageCreate) { helpRequestCommand(s, m, "approve") },
		"cancel":  cancelRequestCommand,
	}
	assistantCommands = commandMap{
		"help":   assistantHelpCommand,
		"length": lengthCommand,
		"list":   listCommand,
		"next":   nextRequestCommand,
		"clear":  clearCommand,
	}
)

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
{{.Prefix}}approve:  Get your lab approved by a teaching assistant
{{.Prefix}}cancel:  Cancels your help request and removes you from the queue
`+"```"+`
After requesting help, you can check the response message you got to see your position in the queue.
You will receive a message when you are next in queue.
Before you can be contacted by a teaching assistant, you must connect to the {{ch .LobbyChannel}} channel.
`+privacy)

var assistant = createTemplate("assistantHelp", `Teaching Assistant commands:
`+"```"+`
{{.Prefix}}help:       Shows this help text
{{.Prefix}}length:     Returns the number of students waiting for help.
{{.Prefix}}list <num>: Lists the next <num> students in the queue.
{{.Prefix}}next:       Removes and returns the first student from the queue.
{{.Prefix}}clear:      Clears the queue!
`+"```"+privacy)

func helpCommand(s disgord.Session, m *disgord.MessageCreate, helpTmpl *template.Template) error {
	buf := new(strings.Builder)
	err := helpTmpl.Execute(buf, cfg)
	if err != nil {
		return fmt.Errorf("helpCommand: failed to execute template: %w", err)
	}
	replyMsg(s, m, buf.String())
	return nil
}

func baseHelpCommand(s disgord.Session, m *disgord.MessageCreate) {
	err := helpCommand(s, m, baseHelp)
	if err != nil {
		log.Error(err)
	}
}

func studentHelpCommand(s disgord.Session, m *disgord.MessageCreate) {
	err := helpCommand(s, m, studentHelp)
	if err != nil {
		log.Error(err)
	}
}

func assistantHelpCommand(s disgord.Session, m *disgord.MessageCreate) {
	err := helpCommand(s, m, assistant)
	if err != nil {
		log.Error(err)
	}
}

func helpRequestCommand(s disgord.Session, m *disgord.MessageCreate, requestType string) {
	// create a transaction such that getPos... and Create... are performed atomically
	tx := db.Begin()
	defer tx.RollbackUnlessCommitted()

	// check if an open request already exists
	pos, err := getPosInQueue(tx, m.Message.Author.ID)
	if err != nil {
		log.Errorln("helpRequest: failed to get user pos in queue")
		replyMsg(s, m, "An error occurred while creating your request.")
		return
	}

	// already in the queue, no need to do anything.
	if pos > 0 {
		replyMsg(s, m, fmt.Sprintf("You are already at postition %d in the queue", pos))
		return
	}

	req := HelpRequest{
		StudentUserID: m.Message.Author.ID,
		Type:          requestType,
		Done:          false,
	}

	err = tx.Create(&req).Error
	if err != nil {
		log.Errorln("helpRequest: failed to create new request:", err)
		replyMsg(s, m, "An error occurred while creating your request.")
		return
	}

	if assignToIdleAssistant(m.Ctx, s, tx, req) {
		tx.Commit()
		return
	}

	pos, err = getPosInQueue(tx, m.Message.Author.ID)
	if err != nil {
		log.Errorln("helpRequest: failed to get pos in queue after creating request:", err)
		replyMsg(s, m, "An error occurred while creating your request.")
		return
	}
	tx.Commit()

	replyMsg(s, m, fmt.Sprintf("A help request has been created, and you are at position %d in the queue.", pos))
}

// assignToIdleAssistant will check if any assistants are waiting for a request and pick one of them to handle req.
// db must be a transaction.
func assignToIdleAssistant(ctx context.Context, s disgord.Session, db *gorm.DB, req HelpRequest) bool {
	err := db.Where("waiting = ?", true).Take(&req.Assistant).Error
	if gorm.IsRecordNotFoundError(err) {
		return false
	} else if err != nil {
		log.Errorln("Failed to query for idle assistants:", err)
		return false
	}

	studUser, err := s.GetMember(ctx, cfg.Guild, req.StudentUserID)
	if err != nil {
		log.Errorln("Failed to retrieve user info for student:", err)
		return false
	}

	assistantUser, err := s.GetMember(ctx, cfg.Guild, req.Assistant.UserID)
	if err != nil {
		log.Errorln("Failed to retrieve user info for assistant:", err)
		return false
	}

	req.Assistant.Waiting = false
	req.Done = true
	req.DoneAt = time.Now()
	req.Reason = "assistantNext"

	// need to do this update manually, as zero-valued struct fields are ignored
	err = db.Model(&Assistant{}).Update("waiting", false).Error
	if err != nil {
		log.Errorln("Failed to update assistant status:", err)
		return false
	}

	err = db.Model(&HelpRequest{}).Update(&req).Error
	if err != nil {
		log.Errorln("Failed to update request:", err)
		return false
	}

	if !sendMsg(ctx, s, assistantUser.User, fmt.Sprintf("Next '%s' request is by '%s'.", req.Type,
		getMemberName(studUser))) {
		return false
	}

	if !sendMsg(ctx, s, studUser.User, fmt.Sprintf("You will now receive help from %s.",
		getMemberName(assistantUser))) {
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

func cancelRequestCommand(s disgord.Session, m *disgord.MessageCreate) {
	err := db.Model(&HelpRequest{}).Where("user_id = ?", m.Message.Author.ID).Updates(map[string]interface{}{
		"done":    true,
		"reason":  "userCancel",
		"done_at": time.Now(),
	}).Error
	if gorm.IsRecordNotFoundError(err) {
		replyMsg(s, m, "You do not have an active help request.")
		return
	} else if err != nil {
		replyMsg(s, m, "An unknown error occurred.")
		return
	}
	replyMsg(s, m, "Your request was cancelled.")
}

func nextRequestCommand(s disgord.Session, m *disgord.MessageCreate) {
	// register assistant in DB
	var assistant Assistant
	err := db.Where("user_id = ?", m.Message.Author.ID).First(&assistant).Error
	if gorm.IsRecordNotFoundError(err) {
		err := db.Create(&Assistant{
			UserID:  m.Message.Author.ID,
			Waiting: false,
		}).Error
		if err != nil {
			log.Errorln("Failed to create assistant record:", err)
		}
	} else if err != nil {
		log.Errorln("Failed to store assistant in DB:", err)
	}

	tx := db.Begin()
	defer tx.RollbackUnlessCommitted()

	var req HelpRequest
	err = tx.Where("done = ?", false).Order("created_at asc").First(&req).Error
	if gorm.IsRecordNotFoundError(err) {
		assistant.UserID = m.Message.Author.ID
		assistant.Waiting = true
		err := tx.Model(&Assistant{}).Update(&assistant).Error
		if err != nil {
			log.Errorln("Failed to update waiting state for assistant:", err)
			replyMsg(s, m, "There are no more requests in the queue, but due to an error, you won't receive a notification when the next one arrives.")
			return
		}
		if !replyMsg(s, m, "There are no more requests in the queue. You will receive a message when the next request arrives.") {
			return
		}
		tx.Commit()
		return
	} else if err != nil {
		log.Errorln("Failed to get next user:", err)
		replyMsg(s, m, "An unknown error occurred.")
		return
	}

	req.Assistant = assistant
	req.Done = true
	req.DoneAt = time.Now()
	req.Reason = "assistantNext"

	err = tx.Model(&HelpRequest{}).Update(&req).Error
	if err != nil {
		log.Errorln("Failed to update request:", err)
		replyMsg(s, m, "An error occurred while updating request.")
		return
	}

	student, err := s.GetMember(m.Ctx, cfg.Guild, req.StudentUserID)
	if err != nil {
		log.Errorln("Failed to fetch user:", err)
		replyMsg(s, m, "An unknown error occurred.")
		return
	}

	assistantMember, err := s.GetMember(m.Ctx, cfg.Guild, m.Message.Author.ID)
	if err != nil {
		log.Errorln("Failed to fetch user:", err)
		replyMsg(s, m, "An unknown error occurred.")
		return
	}

	if !replyMsg(s, m, fmt.Sprintf("Next '%s' request is by '%s'.", req.Type, getMemberName(student))) {
		return
	}

	// TODO: getMemberName(handle)names
	sendMsg(m.Ctx, s, student.User, fmt.Sprintf("You will now receive help from %s", getMemberName(assistantMember)))

	tx.Commit()
}

func lengthCommand(s disgord.Session, m *disgord.MessageCreate) {
	var length int
	err := db.Model(&HelpRequest{}).Where("done = ?", false).Count(&length).Error
	if err != nil {
		log.Errorln("Failed to count number of open requests:", err)
		replyMsg(s, m, "An error occurred.")
		return
	}

	var msg string
	if length == 1 {
		msg = "There is 1 student waiting for help."
	} else {
		msg = fmt.Sprintf("There are %d students waiting for help.", length)
	}
	replyMsg(s, m, msg)
}

func listCommand(s disgord.Session, m *disgord.MessageCreate) {
	words := strings.Fields(m.Message.Content)
	if len(words) < 2 {
		replyMsg(s, m, "You must specify a number of requests to list.")
		return
	}

	num, err := strconv.Atoi(words[1])
	if err != nil {
		replyMsg(s, m, fmt.Sprintf("'%s' is not a vaild number.", words[1]))
		return
	}

	var sb strings.Builder
	var requests []HelpRequest
	err = db.Where("done = ?", false).Order("created_at asc").Limit(num).Find(&requests).Error
	if err != nil {
		log.Errorln("Failed to get requests:", err)
		replyMsg(s, m, "Failed to get list of requests.")
		return
	}

	if len(requests) == 0 {
		replyMsg(s, m, "There are no open requests.")
		return
	}

	fmt.Fprintf(&sb, "Showing the next %d requests:\n", len(requests))
	sb.WriteString("```\n")
	for i, req := range requests {
		user, err := s.GetMember(m.Ctx, cfg.Guild, req.StudentUserID)
		if err != nil {
			log.Errorln("Failed to obtain user info:", err)
			replyMsg(s, m, "An error occurred while sending the message")
			return
		}
		fmt.Fprintf(&sb, "%d. User: %s, Type: %s\n", i+1, getMemberName(user), req.Type)
	}
	sb.WriteString("```")
	replyMsg(s, m, sb.String())
}

func clearCommand(s disgord.Session, m *disgord.MessageCreate) {
	words := strings.Fields(m.Message.Content)
	if len(words) < 2 || words[1] != "YES" {
		replyMsg(s, m, fmt.Sprintf(
			"This command will cancel all the requests in the queue. If you really want to do this, type `%sclear YES`",
			cfg.Prefix,
		))
		return
	}

	// TODO: send a message to each student whose request was cleared.
	err := db.Model(&HelpRequest{}).Where("done = ? ", false).Updates(map[string]interface{}{
		"done":              true,
		"done_at":           time.Now(),
		"assistant_user_id": m.Message.Author.ID,
		"reason":            "assistantClear",
	}).Error
	if err != nil {
		log.Errorln("Failed to clear queue:", err)
		replyMsg(s, m, "Clear failed due to an error.")
		return
	}

	replyMsg(s, m, "The queue was cleared.")
}

func registerCommand(s disgord.Session, m *disgord.MessageCreate) {
	words := strings.Fields(m.Message.Content)
	if len(words) < 2 {
		replyMsg(s, m, "You must include your github username in the command.")
		return
	}

	githubLogin := words[1]

	membership, _, err := gh.Organizations.GetOrgMembership(m.Ctx, githubLogin, cfg.GitHubOrg)
	if err != nil {
		log.Infof("Failed to get org membership for user '%s': %v\n", githubLogin, err)
		replyMsg(s, m,
			"We were unable to verify that you are a member of the course's GitHub organization")
		return
	}

	if membership.GetState() != "active" {
		replyMsg(s, m, fmt.Sprintf(
			"Please make sure that you have accepted the invitation to join the '%s' organization on GitHub",
			cfg.GitHubOrg))
		return
	}

	// TODO: query autograder for real name, using github login for now

	// assign roles to student
	gm, err := s.GetMember(m.Ctx, cfg.Guild, m.Message.Author.ID)
	if err != nil {
		log.Errorln("Failed to get member info:", err)
		replyMsg(s, m, "An unknown error occurred")
		return
	}

	student := Student{
		UserID:      m.Message.Author.ID,
		GithubLogin: githubLogin,
		Name:        "", // TODO
		StudentID:   "", // TODO
	}

	err = db.Create(&student).Error
	if err != nil {
		log.Errorln("Failed to store student in database:", err)
		replyMsg(s, m, "An uknown error occurred.")
		return
	}

	err = gm.UpdateNick(m.Ctx, s, membership.GetUser().GetName())
	if err != nil {
		log.Errorln("Failed to set nick:", err)
		replyMsg(s, m, "An uknown error occurred")
		return
	}

	err = s.AddGuildMemberRole(m.Ctx, cfg.Guild, m.Message.Author.ID, cfg.StudentRole)
	if err != nil {
		log.Errorln("Failed to add student role:", err)
		replyMsg(s, m, "An uknown error occurred")
		return
	}

	replyMsg(s, m, fmt.Sprintf(
		"Authentication was successful! You should now have more access to the server. Type %shelp to see available commands",
		cfg.Prefix))
}

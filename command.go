package main

import (
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

func replyMsg(s disgord.Session, m *disgord.MessageCreate, msg string) {
	_, _, err := m.Message.Author.SendMsgString(m.Ctx, s, msg)
	if err != nil {
		log.Errorln("Sending message failed:", err)
	}
}

var studentHelp = createTemplate("studentHelp", `Available commands:
`+"```"+`
{{.Prefix}}help:    Shows this help text
{{.Prefix}}gethelp: Request help from a teaching assistant
{{.Prefix}}approve:  Get your lab approved by a teaching assistant
{{.Prefix}}cancel:  Cancels your help request and removes you from the queue
`+"```"+`
After requesting help, you can check the response message you got to see your position in the queue.
You will receive a message when you are next in queue.
Before you can be contacted by a teaching assitant, you must connect to the {{ch .LobbyChannel}} channel.
`)

var assitantHelp = createTemplate("assistantHelp", `Teaching Assistant commands:
`+"```"+`
{{.Prefix}}help:       Shows this help text
{{.Prefix}}lenght:     Returns the number of students waiting for help.
{{.Prefix}}list <num>: Lists the next <num> students in the queue.
{{.Prefix}}next:       Removes and returns the first student from the queue.
{{.Prefix}}clear:      Clears the queue!
`+"```")

func helpCommand(s disgord.Session, m *disgord.MessageCreate, helpTmpl *template.Template) error {
	buf := new(strings.Builder)
	err := helpTmpl.Execute(buf, cfg)
	if err != nil {
		return fmt.Errorf("helpCommand: failed to execute template: %w", err)
	}
	replyMsg(s, m, buf.String())
	return nil
}

func studentHelpCommand(s disgord.Session, m *disgord.MessageCreate) {
	err := helpCommand(s, m, studentHelp)
	if err != nil {
		log.Error(err)
	}
}

func assistantHelpCommand(s disgord.Session, m *disgord.MessageCreate) {
	err := helpCommand(s, m, assitantHelp)
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

	req := &HelpRequest{
		UserID: m.Message.Author.ID,
		Type:   requestType,
		Done:   false,
	}

	err = tx.Create(req).Error
	if err != nil {
		log.Errorln("helpRequest: failed to create new request:", err)
		replyMsg(s, m, "An error occurred while creating your request.")
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

func getPosInQueue(db *gorm.DB, userID disgord.Snowflake) (rowNumber int, err error) {
	err = db.Raw(`
		select row_number from (
			select
				row_number () over (
					order by created_at asc
				) row_number,
				user_id
			from
				help_requests
			where
				done = false
		) t
		where
			user_id = ?`,
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
	tx := db.Begin()
	defer tx.RollbackUnlessCommitted()

	var req HelpRequest
	err := tx.Where("done = ?", false).Order("created_at asc").First(&req).Error
	if gorm.IsRecordNotFoundError(err) {
		// TODO: set assitant in an idle state and notify when a new request arrives
		replyMsg(s, m, "There are no more requests in the queue.")
		return
	} else if err != nil {
		log.Errorln("Failed to get next user:", err)
		replyMsg(s, m, "An unknown error occurred.")
		return
	}

	req.AssistantID = m.Message.Author.ID
	req.Done = true
	req.DoneAt = time.Now()
	req.Reason = "assistantNext"

	err = tx.Update(&req).Error
	if err != nil {
		log.Errorln("Failed to update request:", err)
		replyMsg(s, m, "An error occurred while updating request.")
		return
	}

	student, err := s.GetUser(m.Ctx, req.UserID)
	if err != nil {
		log.Errorln("Failed to fetch user:", err)
		replyMsg(s, m, "An unknown error occurred.")
		return
	}
	replyMsg(s, m, fmt.Sprintf("Next '%s' request is by '%s'.", req.Type, student.Tag()))
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
		user, err := s.GetUser(m.Ctx, req.UserID)
		if err != nil {
			log.Errorln("Failed to obtain user info:", err)
			replyMsg(s, m, "An error occurred while sending the message")
			return
		}
		fmt.Fprintf(&sb, "%d. User: %s, Type: %s\n", i+1, user.Tag(), req.Type)
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
		"done":         true,
		"done_at":      time.Now(),
		"assistant_id": m.Message.Author.ID,
		"reason":       "assistantClear",
	}).Error
	if err != nil {
		log.Errorln("Failed to clear queue:", err)
		replyMsg(s, m, "Clear failed due to an error.")
		return
	}

	replyMsg(s, m, "The queue was cleared.")
}

package main

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
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
		"help": assistantHelpCommand,
	}
)

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
	buf := new(bytes.Buffer)
	err := helpTmpl.Execute(buf, cfg)
	if err != nil {
		return fmt.Errorf("helpCommand: failed to execute template: %w", err)
	}
	_, _, err = m.Message.Author.SendMsgString(m.Ctx, s, buf.String())
	if err != nil {
		return fmt.Errorf("helpCommand: failed to send help message: %w", err)
	}
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
		_, _, err := m.Message.Author.SendMsgString(m.Ctx, s, "An error occurred while creating your request.")
		if err != nil {
			log.Errorln("helpRequest: failed to send error message:", err)
		}
		return
	}

	// already in the queue, no need to do anything.
	if pos > 0 {
		_, _, err := m.Message.Author.SendMsgString(m.Ctx, s, fmt.Sprintf("You are already at postition %d in the queue", pos))
		if err != nil {
			log.Errorln("helpRequest: failed to send message:", err)
		}
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
		_, _, err := m.Message.Author.SendMsgString(m.Ctx, s, "An error occurred while creating your request.")
		if err != nil {
			log.Errorln("helpRequest: failed to send error message:", err)
		}
		return
	}

	pos, err = getPosInQueue(tx, m.Message.Author.ID)
	if err != nil {
		log.Errorln("helpRequest: failed to get pos in queue after creating request")
		_, _, err := m.Message.Author.SendMsgString(m.Ctx, s, "An error occurred while creating your request.")
		if err != nil {
			log.Errorln("helpReqest: failed to send error message:", err)
		}
		return
	}
	tx.Commit()

	_, _, err = m.Message.Author.SendMsgString(m.Ctx, s, fmt.Sprintf("A help request has been created, and you are at position %d in the queue.", pos))
	if err != nil {
		log.Errorln("helpRequest: failed to send response:", err)
	}
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
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			_, _, err := m.Message.Author.SendMsgString(m.Ctx, s, "You do not have an active help request.")
			if err != nil {
				log.Errorln("Failed to send error message:", err)
			}
		}
		return
	}
	_, _, err = m.Message.Author.SendMsgString(m.Ctx, s, "Your request was cancelled.")
	if err != nil {
		log.Errorln("Failed to send message:", err)
	}
}

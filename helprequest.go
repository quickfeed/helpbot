package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/andersfylling/disgord"
	"github.com/jinzhu/gorm"
)

func helpRequestCommand(s disgord.Session, m *disgord.MessageCreate) {
	words := strings.Fields(m.Message.Content)
	if len(words) < 2 {
		return
	}

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
		Type:   words[1],
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

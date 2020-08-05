package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/jinzhu/gorm"
)

func helpRequestCommand(s *discordgo.Session, m *discordgo.Message) {
	words := strings.Fields(m.Content)
	if len(words) < 2 {
		return
	}

	stud, err := userIsStudent(s, m.Author.ID)
	if err != nil {
		log.Println("helpRequest: failed to check user role:", err)
		return
	}
	if !stud {
		return
	}

	// get the channel to send response to
	ch, err := s.UserChannelCreate(m.Author.ID)
	if err != nil {
		log.Println("helpRequest: failed to create DM channel:", err)
		return
	}

	// create a transaction such that getPos... and Create... are performed atomically
	tx := db.Begin()
	defer tx.RollbackUnlessCommitted()

	// check if an open request already exists
	pos, err := getPosInQueue(tx, m.Author.ID)
	if err != nil {
		log.Println("helpRequest: failed to get user pos in queue")
		_, err := s.ChannelMessageSend(ch.ID, "An error occurred while creating your request.")
		if err != nil {
			log.Println("helpRequest: failed to send error message:", err)
		}
		return
	}

	// already in the queue, no need to do anything.
	if pos > 0 {
		_, err := s.ChannelMessageSend(ch.ID, fmt.Sprintf("You are already at postition %d in the queue", pos))
		if err != nil {
			log.Println("helpRequest: failed to send message:", err)
		}
		return
	}

	req := &HelpRequest{
		UserID: m.Author.ID,
		Type:   words[1],
		Done:   false,
	}

	err = tx.Create(req).Error
	if err != nil {
		log.Println("helpRequest: failed to create new request:", err)
		_, err := s.ChannelMessageSend(ch.ID, "An error occurred while creating your request.")
		if err != nil {
			log.Println("helpRequest: failed to send error message:", err)
		}
		return
	}

	pos, err = getPosInQueue(tx, m.Author.ID)
	if err != nil {
		log.Println("helpRequest: failed to get pos in queue after creating request")
		_, err := s.ChannelMessageSend(ch.ID, "An error occurred while creating your request.")
		if err != nil {
			log.Println("helpReqest: failed to send error message:", err)
		}
		return
	}
	tx.Commit()

	_, err = s.ChannelMessageSend(ch.ID, fmt.Sprintf("A help request has been created, and you are at position %d in the queue.", pos))
	if err != nil {
		log.Println("helpRequest: failed to send response:", err)
	}
}

func getPosInQueue(db *gorm.DB, userID string) (rowNumber int, err error) {
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

package database

import (
	"errors"
	"fmt"
	"time"

	"github.com/Raytar/helpbot/models"
	"gorm.io/gorm"
)

func (db *Database) ClearHelpRequests(assistantID, guildID string) error {
	return db.conn.Model(&models.HelpRequest{}).Where("done = ? AND guild_id = ?", false, guildID).Updates(map[string]any{
		"done":              true,
		"done_at":           time.Now(),
		"assistant_user_id": assistantID,
		"reason":            "assistantClear",
	}).Error
}

// GetWaitingRequests returns the oldest num requests that are not done. If num is 0, it returns all waiting requests.
//
//	db.GetWaitingRequests(0) // returns all waiting requests
//	db.GetWaitingRequests(5) // returns the 5 oldest waiting requests
func (db *Database) GetWaitingRequests(guildID string, num int) (requests []*models.HelpRequest, err error) {
	query := db.conn.Where("done = ? AND guild_id = ?", false, guildID).Order("created_at asc")
	if num > 0 {
		query = query.Limit(num)
	}
	err = query.Find(&requests).Error
	if err != nil {
		db.log.Errorln("Failed to get waiting requests from DB:", err)
	}
	return
}

func (db *Database) CancelHelpRequest(guildID, studentID string) error {
	if err := db.conn.Model(&models.HelpRequest{}).Where("student_user_id = ? and guild_id = ?", studentID, guildID).Updates(map[string]interface{}{
		"done":    true,
		"reason":  "userCancel",
		"done_at": time.Now(),
	}).Error; err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("you do not have an active help request")
	} else if err != nil {
		return fmt.Errorf("an unknown error occurred when attempting to cancel your help request")
	}
	db.log.Infoln("Canceled help request for", studentID)
	return nil
}

func (db *Database) CreateHelpRequest(request *models.HelpRequest) error {
	// Check if the user already has a help request
	var exists int64
	if err := db.conn.Model(&models.HelpRequest{}).Where("student_user_id = ? AND guild_id = ? AND done = ?", request.StudentUserID, request.GuildID, false).Count(&exists).Error; err != nil {
		db.log.Errorln("Failed to check if user has existing help request:", err)
		return err
	}

	if exists > 0 {
		return fmt.Errorf("you already have an active help request")
	}

	if err := db.conn.Create(request).Error; err != nil {
		db.log.Errorln("Failed to create help request:", err)
		return err
	}
	return nil
}

func (db *Database) GetHelpRequest(request *models.HelpRequest) (r *models.HelpRequest, err error) {
	if err := db.conn.Model(request).Where("student_user_id = ? AND guild_id = ?", request.StudentUserID, request.GuildID).First(&request).Error; err != nil {
		db.log.Errorln("Failed to get help request from DB:", err)
		return nil, err
	}
	return request, nil
}

func (db *Database) GetQueuePosition(guildID, userID string) (rowNumber int, err error) {
	rows, err := db.conn.Model(&models.HelpRequest{}).Select("student_user_id").Where("done = ? AND guild_id = ?", false, guildID).Order("created_at asc").Rows()
	defer rows.Close()
	if err != nil {
		return -1, fmt.Errorf("getPosInQueue error: %w", err)
	}

	found := false
	for rows.Next() {
		rowNumber++
		var studentUserID string
		if err := rows.Scan(&studentUserID); err != nil {
			return -1, fmt.Errorf("getPosInQueue error: %w", err)
		}
		if studentUserID == userID {
			found = true
			break
		}
	}
	if !found {
		return 0, nil
	}
	return
}

func (db *Database) AssignNextRequest(assistantID, guildID string) (*models.HelpRequest, error) {
	var req *models.HelpRequest
	err := db.conn.Transaction(func(tx *gorm.DB) error {
		assistant := &models.Assistant{UserID: assistantID, GuildID: guildID}
		if err := tx.Model(assistant).Where("user_id = ? AND guild_id = ?", assistant.UserID, assistant.GuildID).FirstOrCreate(assistant).Error; err != nil {
			db.log.Errorln("Failed to get assistant from DB:", err)
			return err
		}

		var requests []*models.HelpRequest
		// Get the oldest waiting request
		err := tx.Where("done = ? AND guild_id = ?", false, guildID).Order("created_at asc").Limit(1).Find(&requests).Error
		if err != nil {
			db.log.Errorln("Failed to get waiting requests from DB:", err)
			return err
		}

		if err == gorm.ErrRecordNotFound {
			assistant.Waiting = true
			err := tx.Model(&models.Assistant{}).Save(&assistant).Error
			if err != nil {
				return fmt.Errorf("there are no more requests in the queue, but due to an error, you won't receive a notification when the next one arrives")
			}
		} else if err != nil {
			return fmt.Errorf("an error occurred while fetching the next request")
		}

		if len(requests) == 0 {
			return fmt.Errorf("there are no more requests in the queue")
		}

		request := requests[0]
		request.Assistant = *assistant
		request.Assistant.LastRequest = time.Now()
		err = tx.Model(&models.HelpRequest{}).Where("id = ?", request.ID).Updates(map[string]interface{}{
			"assistant_user_id": assistantID,
			"done":              true,
			"done_at":           time.Now(),
			"reason":            "assistantNext",
		}).Error
		if err != nil {
			db.log.Errorln("Failed to update help request:", err)
			return fmt.Errorf("an error occurred while fetching the next request")
		}

		if err := tx.Model(assistant).Update("last_request", time.Now()).Error; err != nil {
			db.log.Errorln("Failed to update assistant:", err)
			return err
		}
		req = request
		return nil
	})
	return req, err
}

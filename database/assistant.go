package database

import (
	"fmt"

	"github.com/Raytar/helpbot/models"
	"gorm.io/gorm"
)

func (db *Database) CancelWaitingAssistant(assistantID string) error {
	return db.conn.Transaction(func(tx *gorm.DB) error {
		var assistant models.Assistant
		err := tx.Model(assistant).Where("user_id = ?", assistantID).First(&assistant).Error
		if err != nil {
			db.log.Errorln("Failed to get assistant from DB:", err)
			return fmt.Errorf("an unknown error occurred when attempting to get assistant from DB")
		}

		if !assistant.Waiting {
			return fmt.Errorf("you were not marked as waiting, so no action was taken")
		}

		err = tx.Model(assistant).Where("user_id = ?", assistantID).UpdateColumn("waiting", false).Error
		if err != nil {
			db.log.Errorln("Failed to update status in DB:", err)
			return fmt.Errorf("an unknown error occurred when attempting to update waiting status")
		}
		return nil
	})
}

func (db *Database) GetOrCreateAssistant(assistant *models.Assistant) (a *models.Assistant, err error) {
	if err := db.conn.Model(assistant).Where("user_id = ? AND guild_id = ?", assistant.UserID, assistant.GuildID).FirstOrCreate(&assistant).Error; err != nil {
		db.log.Errorln("Failed to get assistant from DB:", err)
		return nil, err
	}
	return assistant, nil
}

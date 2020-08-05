package main

import (
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/spf13/viper"
)

type HelpRequest struct {
	gorm.Model
	UserID      string `gorm:"INDEX"`
	AssistantID string
	Type        string `gorm:"INDEX"`
	Done        bool
	Reason      string
	DoneAt      time.Time
}

func initDB() (err error) {
	dbPath := viper.GetString("db-path")
	db, err = gorm.Open("sqlite3", dbPath)
	db.AutoMigrate(&HelpRequest{})
	return
}

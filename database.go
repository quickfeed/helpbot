package main

import (
	"time"

	"github.com/andersfylling/disgord"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/spf13/viper"
)

var db *gorm.DB

func initDB() (err error) {
	dbPath := viper.GetString("db-path")
	db, err = gorm.Open("sqlite3", dbPath)
	db.AutoMigrate(&Student{}, &Assistant{}, &HelpRequest{})
	return
}

type HelpRequest struct {
	gorm.Model
	StudentUserID   disgord.Snowflake `gorm:"index"`
	Student         Student
	AssistantUserID disgord.Snowflake
	Assistant       Assistant
	Type            string `gorm:"index"`
	Done            bool
	Reason          string
	DoneAt          time.Time
}

type Assistant struct {
	UserID  disgord.Snowflake `gorm:"primary_key"`
	Waiting bool
}

type Student struct {
	UserID      disgord.Snowflake `gorm:"primary_key"`
	GithubLogin string
	Name        string
	StudentID   string
}

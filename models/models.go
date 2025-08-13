package models

import (
	"time"

	"gorm.io/gorm"
)

type HelpRequest struct {
	gorm.Model
	StudentUserID   string `gorm:"index"`
	Student         Student
	AssistantUserID string
	Assistant       Assistant
	GuildID         string `gorm:"index"`
	Type            string `gorm:"index"`
	Done            bool
	Reason          string
	DoneAt          time.Time
}

type Assistant struct {
	gorm.Model
	UserID      string `gorm:"primary_key"`
	GuildID     string `gorm:"primary_key"`
	Waiting     bool
	LastRequest time.Time
}

type Student struct {
	gorm.Model
	// User ID and Guild ID are the primary key
	UserID      string `gorm:"primary_key"`
	GuildID     string `gorm:"primary_key"`
	GithubLogin string
	Name        string
	StudentID   string
}

type Course struct {
	CourseID int64 `gorm:"primary_key"`
	Name     string
	GuildID  string
	Year     uint32
}

package helpbot

import (
	"time"

	"github.com/andersfylling/disgord"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite" // for sqlite support
)

func OpenDatabase(path string) (db *gorm.DB, err error) {
	db, err = gorm.Open("sqlite3", path)
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
	UserID      disgord.Snowflake `gorm:"primary_key"`
	Waiting     bool
	LastRequest time.Time
}

type Student struct {
	UserID      disgord.Snowflake `gorm:"primary_key"`
	GithubLogin string
	Name        string
	StudentID   string
}

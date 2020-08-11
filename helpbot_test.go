package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/andersfylling/disgord"
	"github.com/spf13/viper"
)

func TestMain(m *testing.M) {
	viper.Set("db-path", "file::memory:?cache=shared")
	// viper.Set("db-path", "test.db")
	os.Exit(m.Run())
}

func TestCreateAndRetrieveHelpRequests(t *testing.T) {
	err := initDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to init DB:", err)
		os.Exit(1)
	}
	defer db.Close()

	db.Create(&HelpRequest{StudentUserID: 1, Student: Student{}, Done: true})
	var req HelpRequest
	db.Find(&req, "student_user_id = ?", 1)
	if req.StudentUserID != 1 {
		t.Fatalf("Failed to create and retrieve users")
	}
}

func TestGetPosInQueue(t *testing.T) {
	err := initDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to init DB:", err)
		os.Exit(1)
	}
	defer db.Close()

	db.Create(&HelpRequest{StudentUserID: 1, Student: Student{}})
	db.Create(&HelpRequest{StudentUserID: 2, Student: Student{}})
	db.Create(&HelpRequest{StudentUserID: 3, Student: Student{}, Done: true})
	db.Create(&HelpRequest{StudentUserID: 4, Student: Student{}})

	check := func(name disgord.Snowflake, want int) {
		if pos, err := getPosInQueue(db, name); err != nil {
			t.Errorf("getPosInQueue(%d): %v", name, err)
		} else if pos != want {
			t.Errorf("getPosInQueue(%d): got %d, want %d", name, pos, want)
		}
	}

	check(1, 1)
	check(2, 2)
	check(3, 0)
	check(4, 3)
}

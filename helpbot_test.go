package main

import (
	"fmt"
	"os"
	"testing"

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

	db.Create(&HelpRequest{UserID: "test", Done: true})
	var req HelpRequest
	db.Find(&req, "user_id = ?", "test")
	if req.UserID != "test" {
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

	db.Create(&HelpRequest{UserID: "test1"})
	db.Create(&HelpRequest{UserID: "test2"})
	db.Create(&HelpRequest{UserID: "test3", Done: true})
	db.Create(&HelpRequest{UserID: "test4"})

	check := func(name string, want int) {
		if pos, err := getPosInQueue(db, name); err != nil {
			t.Errorf("getPosInQueue(\"%s\"): %v", name, err)
		} else if pos != want {
			t.Errorf("getPosInQueue(\"%s\"): got %d, want %d", name, pos, want)
		}
	}

	check("test1", 1)
	check("test2", 2)
	check("test3", 0)
	check("test4", 3)
}

package helpbot

import (
	"fmt"
	"os"
	"testing"

	"github.com/Raytar/helpbot/database"
	"github.com/Raytar/helpbot/models"
	"github.com/sirupsen/logrus"
)

var db *database.Database

func TestMain(m *testing.M) {
	var err error
	db, err = database.OpenDatabase("file::memory:?cache=shared", logrus.StandardLogger())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ret := m.Run()
	db.Close()
	os.Exit(ret)
}

func TestCreateAndRetrieveHelpRequests(t *testing.T) {
	db.CreateHelpRequest(&models.HelpRequest{StudentUserID: "1", Student: models.Student{}, Done: true})
	req := &models.HelpRequest{
		StudentUserID: "1",
	}
	db.GetHelpRequest(req)
	if req.StudentUserID != "1" {
		t.Fatalf("Failed to create and retrieve users")
	}
}

func TestGetPosInQueue(t *testing.T) {
	db.CreateHelpRequest(&models.HelpRequest{StudentUserID: "4", Student: models.Student{}, GuildID: "2"})
	db.CreateHelpRequest(&models.HelpRequest{StudentUserID: "1", Student: models.Student{}, GuildID: "1"})
	db.CreateHelpRequest(&models.HelpRequest{StudentUserID: "2", Student: models.Student{}, GuildID: "1"})
	db.CreateHelpRequest(&models.HelpRequest{StudentUserID: "3", Student: models.Student{}, GuildID: "1", Done: true})
	db.CreateHelpRequest(&models.HelpRequest{StudentUserID: "4", Student: models.Student{}, GuildID: "1"})

	check := func(userID string, want int) {
		if pos, err := db.GetQueuePosition("1", userID); err != nil {
			t.Errorf("getPosInQueue(%s): %v", userID, err)
		} else if pos != want {
			t.Errorf("getPosInQueue(%s): got %d, want %d", userID, pos, want)
		}
	}

	check("1", 1)
	check("2", 2)
	check("3", 0)
	check("4", 3)
}

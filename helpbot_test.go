package helpbot

import (
	"testing"

	"github.com/Raytar/helpbot/database"
	"github.com/Raytar/helpbot/models"
)

func TestCreateAndRetrieveHelpRequests(t *testing.T) {
	db := setupTestDatabase(t)
	defer db.Close()

	db.CreateHelpRequest(&models.HelpRequest{StudentUserID: "1", GuildID: "1", Student: models.Student{}, Done: true})
	req := &models.HelpRequest{StudentUserID: "1", GuildID: "1"}
	got, err := db.GetHelpRequest(req)
	if err != nil {
		t.Fatalf("GetHelpRequest failed: %v", err)
	}
	if got.StudentUserID != "1" || got.GuildID != "1" || got.Done != true {
		t.Errorf("GetHelpRequest returned wrong data: got %+v, want %+v", got, req)
	}
}

func TestGetPosInQueue(t *testing.T) {
	db := setupTestDatabase(t)
	defer db.Close()
	db.CreateHelpRequest(&models.HelpRequest{StudentUserID: "1", GuildID: "1", Student: models.Student{}})
	db.CreateHelpRequest(&models.HelpRequest{StudentUserID: "2", GuildID: "1", Student: models.Student{}})
	db.CreateHelpRequest(&models.HelpRequest{StudentUserID: "3", GuildID: "1", Student: models.Student{}, Done: true})
	db.CreateHelpRequest(&models.HelpRequest{StudentUserID: "4", GuildID: "1", Student: models.Student{}})

	check := func(studentID, guildID string, want int) {
		if pos, err := db.GetQueuePosition(guildID, studentID); err != nil {
			t.Errorf("getPosInQueue(%s, %s): %v", studentID, guildID, err)
		} else if pos != want {
			t.Errorf("getPosInQueue(%s, %s): got %d, want %d", studentID, guildID, pos, want)
		}
	}

	// Check positions in the queue
	check("1", "1", 1)
	check("2", "1", 2)
	check("3", "1", 0)
	check("4", "1", 3)
}

func setupTestDatabase(t *testing.T) *database.Database {
	db, err := database.OpenDatabase("file::memory:?cache=shared", nil)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	return db
}

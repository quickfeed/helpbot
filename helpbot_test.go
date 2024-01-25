package helpbot

import (
	"fmt"
	"os"
	"testing"

	"github.com/jinzhu/gorm"
)

var db *gorm.DB

func TestMain(m *testing.M) {
	var err error
	db, err = OpenDatabase("file::memory:?cache=shared")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// viper.Set("db-path", "test.db")
	ret := m.Run()
	db.Close()
	os.Exit(ret)
}

func TestCreateAndRetrieveHelpRequests(t *testing.T) {
	db.Create(&HelpRequest{StudentUserID: "1", Student: Student{}, Done: true})
	var req HelpRequest
	db.Find(&req, "student_user_id = ?", 1)
	if req.StudentUserID != "1" {
		t.Fatalf("Failed to create and retrieve users")
	}
}

func TestGetPosInQueue(t *testing.T) {
	db.Create(&HelpRequest{StudentUserID: "1", Student: Student{}})
	db.Create(&HelpRequest{StudentUserID: "2", Student: Student{}})
	db.Create(&HelpRequest{StudentUserID: "3", Student: Student{}, Done: true})
	db.Create(&HelpRequest{StudentUserID: "4", Student: Student{}})

	check := func(name string, want int) {
		if pos, err := getPosInQueue(db, name); err != nil {
			t.Errorf("getPosInQueue(%s): %v", name, err)
		} else if pos != want {
			t.Errorf("getPosInQueue(%s): got %d, want %d", name, pos, want)
		}
	}

	check("1", 1)
	check("2", 2)
	check("3", 0)
	check("4", 3)
}

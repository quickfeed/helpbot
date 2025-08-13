package database

import (
	"github.com/Raytar/helpbot/models"
	"gorm.io/gorm"
)

func (db *Database) DeleteStudent(student *models.Student) error {
	return db.conn.Delete(student).Error
}

func (db *Database) CreateStudent(student *models.Student) error {
	if err := db.conn.Create(student).Error; err != nil {
		db.log.Errorln("Failed to create student:", err)
		return err
	}
	return nil
}

func (db *Database) GetStudent(s *models.Student) (student *models.Student, err error) {
	if err := db.conn.Model(student).Where("user_id = ? AND github_login = ?", s.UserID, s.GithubLogin).First(&student).Error; err != nil {
		db.log.Errorln("Failed to get student from DB:", err)
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return
}

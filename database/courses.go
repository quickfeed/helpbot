package database

import (
	"github.com/Raytar/helpbot/models"
	"github.com/quickfeed/quickfeed/qf"
	"gorm.io/gorm/clause"
)

func (db *Database) GetCourse(query *models.Course) (*models.Course, error) {
	var course models.Course
	if err := db.conn.Model(course).Where(query).First(&course).Error; err != nil {
		db.log.Errorln("Failed to get course from DB:", err)
		return nil, err
	}
	return &course, nil
}

func (db *Database) CreateCourse(course *models.Course) error {
	return db.conn.Create(course).Error
}

func (db *Database) UpdateCourse(course *models.Course) error {
	return db.conn.Save(course).Error
}

func (db *Database) UpdateCourses(courses []*qf.Course) error {
	for _, course := range courses {
		if err := db.conn.Model(&models.Course{}).Clauses(clause.OnConflict{DoNothing: true}).Create(&models.Course{
			CourseID: int64(course.ID),
			Name:     course.Name,
			GuildID:  "",
		}).Error; err != nil {
			db.log.Errorln("Failed to update courses:", err)
			return err
		}
	}
	return nil
}

func (db *Database) GetCourses() ([]*models.Course, error) {
	var courses []*models.Course
	if err := db.conn.Model(&models.Course{}).Find(&courses).Error; err != nil {
		db.log.Errorln("Failed to get courses from DB:", err)
		return nil, err
	}
	return courses, nil
}

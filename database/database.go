package database

import (
	"fmt"

	"github.com/Raytar/helpbot/models"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Database struct {
	conn *gorm.DB
	log  *logrus.Logger
}

func OpenDatabase(path string, logger *logrus.Logger) (*Database, error) {
	lgr, err := Zap()
	if err != nil {
		fmt.Println(err)
	}
	defer func() { _ = lgr.Sync() }()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger:                 NewGORMLogger(lgr),
		SkipDefaultTransaction: false,
	})
	if err != nil {
		panic(err)
	}
	db.AutoMigrate(
		&models.Student{},
		&models.Assistant{},
		&models.HelpRequest{},
		&models.Course{},
	)
	return &Database{db, logger}, nil
}

func (db *Database) Close() error {
	conn, err := db.conn.DB()
	if err != nil {
		return err
	}
	return conn.Close()
}

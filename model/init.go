package model

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var db *gorm.DB

func init() {
	var err error

	db, err = gorm.Open(sqlite.Open("D:/Workspace/Github/mediago/app.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

}

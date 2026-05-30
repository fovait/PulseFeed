package db

import (
	"PulseFeed/internal/config"
	"fmt"

	"PulseFeed/internal/errs"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func NewDB(dbcfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbcfg.User, dbcfg.Password, dbcfg.Host, dbcfg.Port, dbcfg.DBName)

	if db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		TranslateError: true,
	}); err != nil {
		return nil, err
	} else {
		return db, nil
	}
}

func AutoMigrate(db *gorm.DB) error {
	return errs.NotImplemented
}

func CloseDB(db *gorm.DB) error {
	if sqlDB, err := db.DB(); err != nil {
		return err
	} else {
		sqlDB.Close()
		return nil
	}
}

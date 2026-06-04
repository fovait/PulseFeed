package db

import (
	"PulseFeed/internal/account"
	"PulseFeed/internal/config"
	"PulseFeed/internal/event"
	"PulseFeed/internal/message"
	"PulseFeed/internal/moderation"
	"PulseFeed/internal/recommend"
	"PulseFeed/internal/social"
	"PulseFeed/internal/video"
	"PulseFeed/internal/worker"
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func NewDB(dbcfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbcfg.User, dbcfg.Password, dbcfg.Host, dbcfg.Port, dbcfg.DBName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		TranslateError: true,
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

// AutoMigrate 启动时迁移核心业务表（各模块路由内还会迁移专属表，重复调用安全）。
func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	return db.AutoMigrate(
		&account.Account{},
		&video.Video{},
		&video.Like{},
		&video.Comment{},
		&video.Tag{},
		&video.VideoTag{},
		&video.OutboxMsg{},
		&social.Social{},
		&message.Message{},
		&event.UserEvent{},
		&event.VideoMetrics{},
		&moderation.ContentReport{},
		&recommend.RecommendExposure{},
		&worker.Notification{},
	)
}

func CloseDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

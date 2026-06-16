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
	"time"

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

	// 连接池上限：database/sql 默认无限连接，高并发(尤其缓存被绕过时)会瞬间开出
	// 几百个连接打爆 MySQL 的 max_connections(默认 151)，引发级联失败。
	// 这里把上限压在 MySQL 容量之下，给 worker 等其它客户端留余量；
	// 超过上限的请求改为排队等连接(延迟有界)，而非直接报错雪崩。
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	return db, nil
}

// AutoMigrate 启动时迁移核心业务表（各模块路由内还会迁移专属表，重复调用安全）。
func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if err := db.AutoMigrate(
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
	); err != nil {
		return err
	}

	return db.Exec(`
		UPDATE videos v
		LEFT JOIN (
			SELECT video_id, COUNT(*) AS cnt
			FROM comments
			GROUP BY video_id
		) c ON c.video_id = v.id
		SET v.comments_count = COALESCE(c.cnt, 0)
	`).Error
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

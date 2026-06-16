// Command seed 为压测灌入基础数据：批量账号 + 视频。
// 直接复用项目的 config/db/account/video 包，保证密码哈希、表结构与线上完全一致。
//
//	go run ./cmd/seed -accounts=100 -videos=2000
//
// 账号按用户名幂等创建，可重复执行；视频为追加插入。
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"PulseFeed/internal/account"
	"PulseFeed/internal/config"
	"PulseFeed/internal/db"
	"PulseFeed/internal/video"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func main() {
	numAccounts := flag.Int("accounts", 100, "要确保存在的压测账号数")
	numVideos := flag.Int("videos", 2000, "要插入的压测视频数")
	password := flag.String("password", "bench123", "所有压测账号的统一密码")
	prefix := flag.String("prefix", "bench", "压测账号用户名前缀")
	spreadDays := flag.Int("spread-days", 30, "把视频 create_time 打散到最近 N 天")
	flag.Parse()

	if *numAccounts < 1 {
		log.Fatalf("accounts must be >= 1")
	}

	cfg, _, err := config.LoadConfig(config.ResolveConfigPath())
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	gdb, err := db.NewDB(cfg.Database)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	if err := db.AutoMigrate(gdb); err != nil {
		log.Fatalf("automigrate: %v", err)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 1) 账号：按用户名幂等创建，密码统一 bcrypt 哈希，保证压测能登录。
	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}
	accounts := make([]account.Account, 0, *numAccounts)
	for i := 1; i <= *numAccounts; i++ {
		uname := fmt.Sprintf("%s_%04d", *prefix, i)
		var acc account.Account
		queryErr := gdb.Where("username = ?", uname).First(&acc).Error
		switch {
		case queryErr == nil:
			// 已存在，复用
		case errors.Is(queryErr, gorm.ErrRecordNotFound):
			acc = account.Account{Username: uname, Password: string(hash)}
			if err := gdb.Create(&acc).Error; err != nil {
				log.Fatalf("create account %s: %v", uname, err)
			}
		default:
			log.Fatalf("query account %s: %v", uname, queryErr)
		}
		accounts = append(accounts, acc)
	}
	log.Printf("accounts ready: %d (login: username=%s_0001 password=%s)", len(accounts), *prefix, *password)

	// 2) 视频：批量插入；create_time / likes_count / popularity 打散，
	//    让最新流、点赞榜、热门流（MySQL 兜底）都有可分页、可排序的信号。
	now := time.Now()
	window := int64(*spreadDays) * 24 * int64(time.Hour)
	videos := make([]video.Video, 0, *numVideos)
	for i := 1; i <= *numVideos; i++ {
		author := accounts[rng.Intn(len(accounts))]
		videos = append(videos, video.Video{
			AuthorID:   author.ID,
			Username:   author.Username,
			Title:      fmt.Sprintf("bench video #%d", i),
			PlayURL:    fmt.Sprintf("/static/videos/bench/%d.mp4", i),
			CoverURL:   fmt.Sprintf("/static/covers/bench/%d.jpg", i),
			CreateTime: now.Add(-time.Duration(rng.Int63n(window))),
			LikesCount: rng.Int63n(10000),
			Popularity: rng.Int63n(100000),
		})
	}
	if err := gdb.CreateInBatches(&videos, 200).Error; err != nil {
		log.Fatalf("insert videos: %v", err)
	}

	var minID, maxID uint
	gdb.Model(&video.Video{}).Select("COALESCE(MIN(id),0)").Scan(&minID)
	gdb.Model(&video.Video{}).Select("COALESCE(MAX(id),0)").Scan(&maxID)
	log.Printf("videos inserted: %d  | table id range: %d..%d", len(videos), minID, maxID)
	log.Printf("k6 getDetail/like 用 MINID=%d MAXID=%d", minID, maxID)
}

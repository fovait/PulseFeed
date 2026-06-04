package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"PulseFeed/internal/config"
	"PulseFeed/internal/db"
	rabbitmq "PulseFeed/internal/middleware/rabbitmq"
	rediscache "PulseFeed/internal/middleware/redis"
	"PulseFeed/internal/observability"
	"PulseFeed/internal/video"
	"PulseFeed/internal/worker"

	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

const (
	// These topology names must match the publisher side in internal/middleware/rabbitmq.
	// Keep them explicit here because the current publisher constants are package-private.
	socialExchange   = "social.events"
	socialQueue      = "social.events"
	socialBindingKey = "social.*"

	likeExchange   = "like.events"
	likeQueue      = "like.events"
	likeBindingKey = "like.*"

	commentExchange   = "comment.events"
	commentQueue      = "comment.events"
	commentBindingKey = "comment.*"

	popularityExchange   = "video.popularity.exchange"
	popularityQueue      = "video.popularity.queue"
	popularityBindingKey = "video.popularity.*"

	workerShutdownTimeout = 10 * time.Second
)

func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 6 {
		attempt = 6
	}
	wait := time.Duration(1<<uint(attempt-1)) * time.Second
	if wait > 30*time.Second {
		return 30 * time.Second
	}
	return wait
}

func sleepOrDone(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func connectWithRetry(name string, maxRetries int, fn func() error) {
	for i := 0; i < maxRetries; i++ {
		if err := fn(); err == nil {
			return
		}
		wait := retryDelay(i + 1)
		log.Printf("%s 不可用，%v 后重试 (%d/%d)...", name, wait, i+1, maxRetries)
		time.Sleep(wait)
	}
	log.Fatalf("%s: 超过最大重试次数", name)
}

// runWorkerWithRetry gives each worker its own RabbitMQ connection.
// A closed AMQP connection cannot be repaired by only creating a new channel, so this loop
// redials the broker, redeclares the queue topology, then starts consuming again.
func runWorkerWithRetry(
	ctx context.Context,
	name string,
	amqpURL string,
	declare func(*amqp.Channel) error,
	run func(*amqp.Channel) error,
) {
	attempt := 1
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := amqp.Dial(amqpURL)
		if err != nil {
			wait := retryDelay(attempt)
			log.Printf("%s: RabbitMQ 连接失败: %v, %v 后重试", name, err, wait)
			attempt++
			if !sleepOrDone(ctx, wait) {
				return
			}
			continue
		}
		attempt = 1

		ch, err := conn.Channel()
		if err != nil {
			_ = conn.Close()
			log.Printf("%s: 创建 Channel 失败: %v, 5秒后重试", name, err)
			if !sleepOrDone(ctx, 5*time.Second) {
				return
			}
			continue
		}

		// Declare on the same connection used for consuming. This lets the worker recover
		// after a broker restart even if the API process has not recreated the queues yet.
		if declare != nil {
			if err := declare(ch); err != nil {
				_ = ch.Close()
				_ = conn.Close()
				log.Printf("%s: 声明 RabbitMQ 拓扑失败: %v, 5秒后重试", name, err)
				if !sleepOrDone(ctx, 5*time.Second) {
					return
				}
				continue
			}
		}

		log.Printf("%s started, consuming", name)
		if err := run(ch); err != nil {
			if ctx.Err() != nil {
				_ = ch.Close()
				_ = conn.Close()
				return
			}
			log.Printf("%s: %v, 5秒后重连...", name, err)
		}
		_ = ch.Close()
		_ = conn.Close()
		if !sleepOrDone(ctx, 5*time.Second) {
			return
		}
	}
}

func main() {
	// 加载 .env（本地开发）
	if err := godotenv.Load(); err != nil {
		log.Println(".env not found; continuing")
	}
	// 与 API 进程保持同一套配置解析规则：CONFIG_PATH > configs/config.yaml > config.yaml。
	configPath := config.ResolveConfigPath()
	log.Printf("Loading config from %s", configPath)
	cfg, usedDefault, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if usedDefault {
		log.Printf("Config File %s not found, using default local config", configPath)
	} else {
		log.Printf("Config loaded from file: %s", configPath)
	}
	// 连接数据库（带重试）
	var sqlDB *gorm.DB
	connectWithRetry("MySQL", 10, func() error {
		var err error
		sqlDB, err = db.NewDB(cfg.Database)
		return err
	})
	defer db.CloseDB(sqlDB)

	// 连接 Redis（用于流行度更新）
	cache, err := rediscache.NewFromConfig(&cfg.Redis)
	if err != nil {
		log.Printf("Redis config error (popularity worker disabled): %v", err)
		cache = nil
	} else {
		pingCtx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		if err := cache.Ping(pingCtx); err != nil {
			log.Printf("Redis not available (popularity worker disabled): %v", err)
			_ = cache.Close()
			cache = nil
		} else {
			defer cache.Close()
			log.Printf("Redis connected (popularity worker enabled)")
		}
	}
	amqpURL := "amqp://" + cfg.RabbitMQ.Username + ":" + cfg.RabbitMQ.Password + "@" + cfg.RabbitMQ.Host + ":" + strconv.Itoa(cfg.RabbitMQ.Port) + "/"

	// 准备 repo
	videoRepo := video.NewVideoRepository(sqlDB)
	likeRepo := video.NewLikeRepository(sqlDB)
	commentRepo := video.NewCommentRepository(sqlDB)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pprofServer, err := observability.NewPprofServer(
		"Worker",
		cfg.ObservabilityConfig.Pprof.Enabled,
		cfg.ObservabilityConfig.Pprof.WorkerAddr,
	)
	if err != nil {
		log.Printf("Failed to start worker pprof server: %v", err)
	}
	if pprofServer != nil {
		defer pprofServer.Close()
	}

	var wg sync.WaitGroup
	startWorker := func(name string, declare func(*amqp.Channel) error, run func(*amqp.Channel) error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runWorkerWithRetry(ctx, name, amqpURL, declare, run)
		}()
	}

	// Each worker consumes its own queue. Starting them independently prevents a single
	// failed channel from stopping unrelated queues.
	startWorker("SocialWorker", declareSocialTopology, func(ch *amqp.Channel) error {
		return worker.NewSocialWorker(ch, socialQueue).Run(ctx)
	})
	startWorker("LikeWorker", declareLikeTopology, func(ch *amqp.Channel) error {
		return worker.NewLikeWorker(ch, likeRepo, videoRepo, likeQueue).Run(ctx)
	})
	startWorker("CommentWorker", declareCommentTopology, func(ch *amqp.Channel) error {
		return worker.NewCommentWorker(ch, commentRepo, videoRepo, commentQueue).Run(ctx)
	})
	if cache != nil {
		startWorker("PopularityWorker", declarePopularityTopology, func(ch *amqp.Channel) error {
			return worker.NewPopularityWorker(ch, cache, popularityQueue).Run(ctx)
		})
	}

	// 等待退出信号
	<-ctx.Done()
	log.Printf("Worker shutting down...")

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("Worker stopped")
	case <-time.After(workerShutdownTimeout):
		log.Printf("Worker shutdown timed out after %v", workerShutdownTimeout)
	}
}

func declareSocialTopology(ch *amqp.Channel) error {
	return rabbitmq.DeclareTopic(ch, socialExchange, socialQueue, socialBindingKey)
}

func declarePopularityTopology(ch *amqp.Channel) error {
	return rabbitmq.DeclareTopic(ch, popularityExchange, popularityQueue, popularityBindingKey)
}

func declareLikeTopology(ch *amqp.Channel) error {
	return rabbitmq.DeclareTopic(ch, likeExchange, likeQueue, likeBindingKey)
}

func declareCommentTopology(ch *amqp.Channel) error {
	return rabbitmq.DeclareTopic(ch, commentExchange, commentQueue, commentBindingKey)
}

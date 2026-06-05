package router

import (
	"context"
	"log"
	"sync"
	"time"

	"PulseFeed/internal/middleware/rabbitmq"
	rediscache "PulseFeed/internal/middleware/redis"
	"PulseFeed/internal/worker"

	"gorm.io/gorm"
)

// BackgroundWorkers 管理 SetRouter 启动的后台 goroutine，配合 main 的信号取消做优雅退出。
type BackgroundWorkers struct {
	db         *gorm.DB
	cache      *rediscache.Client
	rmq        *rabbitmq.RabbitMQ
	timelineMQ *rabbitmq.TimelineMQ
	eventMQ    *rabbitmq.EventMQ
	sseHub     *worker.SSEHub

	wg sync.WaitGroup
}

func newBackgroundWorkers(
	db *gorm.DB,
	cache *rediscache.Client,
	rmq *rabbitmq.RabbitMQ,
	timelineMQ *rabbitmq.TimelineMQ,
	eventMQ *rabbitmq.EventMQ,
	sseHub *worker.SSEHub,
) *BackgroundWorkers {
	return &BackgroundWorkers{
		db:         db,
		cache:      cache,
		rmq:        rmq,
		timelineMQ: timelineMQ,
		eventMQ:    eventMQ,
		sseHub:     sseHub,
	}
}

// Start 在独立 goroutine 中运行 outbox / timeline / event / notification 消费者。
func (b *BackgroundWorkers) Start(ctx context.Context) {
	if b == nil {
		return
	}
	worker.RunOutboxPoller(ctx, &b.wg, b.db, b.timelineMQ)
	worker.RunTimelineConsumer(ctx, &b.wg, b.timelineMQ, b.cache, b.rmq)
	if b.rmq != nil && b.eventMQ != nil {
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			runEventMetricsWorker(ctx, b.rmq, b.cache)
		}()
	}
	if b.rmq != nil && b.sseHub != nil {
		runNotificationWorkers(ctx, &b.wg, b.rmq, b.db, b.sseHub)
	}
}

// Stop 等待所有后台 worker 退出，超时 10 秒后强制返回。
func (b *BackgroundWorkers) Stop() {
	if b == nil {
		return
	}
	log.Println("waiting for background workers to stop...")
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		log.Println("background workers stopped")
	case <-time.After(10 * time.Second):
		log.Println("background workers stop timed out, forcing exit")
	}
}

func runEventMetricsWorker(ctx context.Context, rmq *rabbitmq.RabbitMQ, cache *rediscache.Client) {
	queue := rabbitmq.EventMetricsQueue
	for {
		if ctx.Err() != nil {
			return
		}
		ch, err := rmq.NewChannel()
		if err != nil {
			log.Printf("event-metrics: create channel failed: %v, retry in 5s", err)
			if !sleepOrDone(ctx, 5*time.Second) {
				return
			}
			continue
		}
		w := worker.NewEventWorker(ch, cache, queue)
		if err := w.Run(ctx); err != nil {
			log.Printf("event-metrics: %v", err)
		}
		_ = ch.Close()
		if ctx.Err() != nil {
			return
		}
		if !sleepOrDone(ctx, 5*time.Second) {
			return
		}
	}
}

func runNotificationWorkers(ctx context.Context, wg *sync.WaitGroup, rmq *rabbitmq.RabbitMQ, db *gorm.DB, hub *worker.SSEHub) {
	for _, q := range []string{"notification.like", "notification.comment", "notification.social"} {
		wg.Add(1)
		go func(queue string) {
			defer wg.Done()
			for {
				if ctx.Err() != nil {
					return
				}
				ch, err := rmq.NewChannel()
				if err != nil {
					log.Printf("notification-%s: create channel failed: %v, retry in 5s", queue, err)
					if !sleepOrDone(ctx, 5*time.Second) {
						return
					}
					continue
				}
				w := worker.NewNotificationWorker(ch, db, queue, hub)
				if err := w.Run(ctx); err != nil {
					log.Printf("notification-%s: %v", queue, err)
				}
				_ = ch.Close()
				if ctx.Err() != nil {
					return
				}
				if !sleepOrDone(ctx, 5*time.Second) {
					return
				}
			}
		}(q)
	}
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

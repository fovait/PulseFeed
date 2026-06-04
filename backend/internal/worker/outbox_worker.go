package worker

import (
	"PulseFeed/internal/middleware/rabbitmq"
	rediscache "PulseFeed/internal/middleware/redis"
	"PulseFeed/internal/video"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// RunOutboxPoller 轮询 outbox 表并投递 timeline MQ；ctx 取消时退出。
func RunOutboxPoller(ctx context.Context, wg *sync.WaitGroup, db *gorm.DB, tmq *rabbitmq.TimelineMQ) {
	if db == nil || tmq == nil {
		log.Printf("Outbox poller disabled: timeline mq is not initialized")
		return
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if ctx.Err() != nil {
				return
			}

			var messages []video.OutboxMsg
			err := db.WithContext(ctx).Where("status = ?", "pending").Order("create_time ASC").Limit(100).Find(&messages).Error

			if err != nil || len(messages) == 0 {
				if !sleepOrDone(ctx, time.Second) {
					return
				}
				continue
			}

			for _, msg := range messages {
				if ctx.Err() != nil {
					return
				}
				err := tmq.PublishVideo(ctx, msg.VideoID, msg.CreateTime)
				if err == nil {
					if err := db.WithContext(ctx).Delete(&msg).Error; err != nil {
						log.Printf("删除 outbox 消息失败: id=%d, err=%v", msg.ID, err)
					}
				} else {
					log.Printf("投递MQ失败: VideoID: %d, err: %v", msg.VideoID, err)
				}
			}
		}
	}()
}

// RunTimelineConsumer 消费 timeline 队列并写入 Redis ZSET；ctx 取消时退出。
func RunTimelineConsumer(ctx context.Context, wg *sync.WaitGroup, tmq *rabbitmq.TimelineMQ, redisClient *rediscache.Client, rmq *rabbitmq.RabbitMQ) {
	if tmq == nil {
		log.Printf("Timeline consumer disabled: timeline mq is not initialized")
		return
	}
	if rmq == nil || rmq.Conn == nil {
		log.Printf("Timeline consumer disabled: rabbitmq is not initialized")
		return
	}
	if redisClient == nil {
		log.Printf("Timeline consumer disabled: redis is not initialized")
		return
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if ctx.Err() != nil {
				return
			}

			ch, err := rmq.NewChannel()
			if err != nil {
				log.Printf("Timeline consumer: create channel failed: %v, retry after 5s", err)
				if !sleepOrDone(ctx, 5*time.Second) {
					return
				}
				continue
			}

			if err := tmq.DeclareOn(ch); err != nil {
				log.Printf("Timeline consumer: declare timeline queue failed: %v, retry after 5s", err)
				_ = ch.Close()
				if !sleepOrDone(ctx, 5*time.Second) {
					return
				}
				continue
			}

			if err := ch.Qos(10, 0, false); err != nil {
				log.Printf("Timeline consumer: set qos failed %v", err)
			}

			queueName := tmq.QueueName()
			msgs, err := ch.Consume(queueName, "", false, false, false, false, nil)
			if err != nil {
				log.Printf("Timeline consumer: consume failed: %v, retry after 5s", err)
				_ = ch.Close()
				if !sleepOrDone(ctx, 5*time.Second) {
					return
				}
				continue
			}

			log.Printf("Timeline consumer started, queue=%s", queueName)

		consumeLoop:
			for {
				select {
				case <-ctx.Done():
					_ = ch.Close()
					return
				case d, ok := <-msgs:
					if !ok {
						break consumeLoop
					}
					var event rabbitmq.TimelineEvent
					if err := json.Unmarshal(d.Body, &event); err != nil {
						log.Printf("Timeline consumer: invalid message json: %v", err)
						_ = d.Ack(false)
						continue
					}
					if event.VideoID == 0 || event.CreateTime <= 0 {
						log.Printf("Timeline consumer: invalid event: %+v", event)
						_ = d.Nack(false, false)
						continue
					}
					if err := handleTimelineEvent(redisClient, event); err != nil {
						log.Printf("Timeline consumer: handle event failed: event_id=%s video_id=%d err=%v",
							event.EventID, event.VideoID, err)
						_ = d.Nack(false, true)
						continue
					}
					if err := d.Ack(false); err != nil {
						log.Printf("Timeline consumer: ack failed: %v", err)
					}
				}
			}

			_ = ch.Close()
			if ctx.Err() != nil {
				return
			}
			log.Printf("Timeline consumer: channel closed, reconnect after 5s")
			if !sleepOrDone(ctx, 5*time.Second) {
				return
			}
		}
	}()
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

// handleTimelineEvent 处理一条 timeline 消息。
func handleTimelineEvent(redisClient *rediscache.Client, event rabbitmq.TimelineEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	timelineKey := redisClient.Key("feed:global_timeline")

	if err := redisClient.ZAdd(ctx, timelineKey, redis.Z{
		Score:  float64(event.CreateTime),
		Member: fmt.Sprintf("%d", event.VideoID),
	}); err != nil {
		return err
	}

	if err := redisClient.ZRemRangeByRank(ctx, timelineKey, 0, -1001); err != nil {
		log.Printf("Timeline consumer: ZRemRangeByRank failed: %v", err)
	}

	return nil
}

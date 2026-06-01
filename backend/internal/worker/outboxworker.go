package worker

import (
	"PulseFeed/internal/middleware/rabbitmq"
	rediscache "PulseFeed/internal/middleware/redis"
	"PulseFeed/internal/video"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func StartOutboxPoller(db *gorm.DB, tmq *rabbitmq.TimelineMQ) {
	if db == nil || tmq == nil {
		log.Printf("Outbox poller disabled: timeline mq is not initialized")
		return
	}

	go func() {
		for {
			var messages []video.OutboxMsg

			err := db.Where("status = ?", "pending").Order("create_time ASC").Limit(100).Find(&messages).Error

			if err != nil || len(messages) == 0 {
				time.Sleep(1 * time.Second)
				continue
			}

			for _, msg := range messages {
				err := tmq.PublishVideo(context.Background(), msg.VideoID, msg.CreateTime)

				if err == nil {
					if err := db.Delete(&msg).Error; err != nil {
						log.Printf("删除 outbox 消息失败: id=%d, err=%v", msg.ID, err)
					}
				} else {
					log.Printf("投递MQ失败: VideoID: %d, err: %v", msg.VideoID, err)
				}
			}
		}
	}()
}

// StartTimelineConsumer 启动 timeline 消费者。
// 它从 RabbitMQ 的 timeline 队列中消费 TimelineEvent，
// 然后把 videoID 写入 Redis ZSET，形成全局时间线。
func StartTimelineConsumer(tmq *rabbitmq.TimelineMQ, redisClient *rediscache.Client, rmq *rabbitmq.RabbitMQ) {
	// 1. 基础依赖检查
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

	// 2.后台 goroutine 持续运行消费者
	go func() {
		for {
			// 每次重连都新建一个channel
			// 不要和 timelineMQ 内部发布消息用的 channel 共用
			ch, err := rmq.NewChannel()
			if err != nil {
				log.Printf("Timeline consumer: create channel failed: %v, retry after 5s", err)
				time.Sleep(5 * time.Second)
				continue
			}

			// 确保当前的 channel 上 timeline exchange / queue / binding 没问题
			if err := tmq.DeclareOn(ch); err != nil {
				log.Printf("Timeline consumer: declare timeline queue failed: %v, retry after 5s", err)
				_ = ch.Close()
				time.Sleep(5 * time.Second)
				continue
			}

			// Qos 限制消费者一次最多拿 10 条未确认消息
			// 防止消费者处理太慢时，RabbitMQ 一次性推太多消息到本地
			if err := ch.Qos(10, 0, false); err != nil {
				log.Printf("Timeline consumer: set qos failed %v", err)
			}

			// 获取队列名
			queueName := tmq.QueueName()

			// autoAck=false 表示要手动处理消息
			// 处理成功要 Ack 处理失败可以 Nack
			msgs, err := ch.Consume(queueName, "", false, false, false, false, nil)
			if err != nil {
				log.Printf("Timeline consumer: consume failed: %v, retry after 5s", err)
				_ = ch.Close()
				time.Sleep(5 * time.Second)
				continue
			}

			log.Printf("Timeline consumer started, queue=%s", queueName)

			// 3. 持续消费消息
			for msg := range msgs {
				var event rabbitmq.TimelineEvent

				// 反序列化 RabbitMQ 消息体
				if err := json.Unmarshal(msg.Body, &event); err != nil {
					log.Printf("Timeline consumer: invalid message json: %v", err)
					_ = msg.Ack(false)
					continue
				}

				// 参数校验
				if event.VideoID == 0 || event.CreateTime <= 0 {
					log.Printf("Timeline consumer: invalid event: %+v", event)
					_ = msg.Nack(false, false)
					continue
				}

				if err := handleTimelineEvent(redisClient, event); err != nil {
					log.Printf("Timeline consumer: handle event failed: event_id=%s video_id=%d err=%v",
						event.EventID, event.VideoID, err)
					_ = msg.Nack(false, true)
					continue
				}

				if err := msg.Ack(false); err != nil {
					log.Printf("Timeline consumer: ack failed: %v", err)
				}
			}
			_ = ch.Close()
			log.Printf("Timeline consumer: channel closed, reconnect after 5s")
			time.Sleep(5 * time.Second)
		}
	}()
}

// handleTimelineEvent 处理一条 timeline 消息。
// 这里把 videoID 写入 Redis ZSET：feed:global_timeline。
// score 使用视频创建时间，member 使用 videoID。
func handleTimelineEvent(redisClient *rediscache.Client, event rabbitmq.TimelineEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	timelineKey := redisClient.Key("feed:global_timeline")

	// Redis ZSET:
	// member = videoID
	// score = createTime
	// 这样可以按照发布时间排序，构建全局最新视频流
	if err := redisClient.ZAdd(ctx, timelineKey, redis.Z{
		Score:  float64(event.CreateTime),
		Member: fmt.Sprintf("%d", event.VideoID),
	}); err != nil {
		return err
	}

	// 只保留最新 1000 条。
	// ZSET 默认按 score 从小到大排列：
	// rank 越小越旧，rank 越大越新。
	// 删除 rank 0 到 -1001，基本就是删除旧数据，只保留最新 1000 条。
	// 裁剪失败不中断流程：数据已经 ZAdd 写入成功，裁剪是清理操作，
	// 失败了最多 ZSET 稍微大一点，不应触发消息重试。
	if err := redisClient.ZRemRangeByRank(ctx, timelineKey, 0, -1001); err != nil {
		log.Printf("Timeline consumer: ZRemRangeByRank failed: %v", err)
	}

	return nil
}

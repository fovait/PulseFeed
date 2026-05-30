package redis

import (
	"PulseFeed/internal/config"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Client struct {
	rdb       *goredis.Client
	keyPrefix string
}

const defaultKeyPrefix = "v1:"

func NewClient(rdb *goredis.Client, keyPrefix string) *Client {
	return &Client{rdb: rdb, keyPrefix: keyPrefix}
}

func NewFromConfig(cfg *config.RedisConfig) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("redis config is nil")
	}
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Host + ":" + strconv.Itoa(cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	return &Client{rdb: rdb, keyPrefix: defaultKeyPrefix}, nil
}

func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return errors.New("redis client not initialized")
	}
	return c.rdb.Ping(ctx).Err()
}

func IsMiss(err error) bool {
	return err == goredis.Nil
}

func (c *Client) Key(format string, args ...any) string {
	prefix := ""
	if c != nil {
		prefix = c.keyPrefix
	}
	return prefix + fmt.Sprintf(format, args...)
}

func randToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (c *Client) Lock(ctx context.Context, key string, ttl time.Duration) (token string, ok bool, err error) {
	if c == nil || c.rdb == nil {
		return "", false, errors.New("redis client not initialized")
	}

	token, err = randToken(16)
	if err != nil {
		return "", false, err
	}

	if ttl <= 0 {
		return "", false, errors.New("lock ttl must be positive")
	}

	ok, err = c.rdb.SetNX(ctx, key, token, ttl).Result()
	return token, ok, err
}

var unlockScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
else
  return 0
end
`)

var incrementWithExpireScript = goredis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return count
`)

func (c *Client) Unlock(ctx context.Context, key string, token string) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	_, err := unlockScript.Run(ctx, c.rdb, []string{key}, token).Result()
	return err
}

func (c *Client) IncrementWithExpire(ctx context.Context, key string, expire time.Duration) (int64, error) {
	if c == nil || c.rdb == nil {
		return 0, errors.New("redis client not initialized")
	}

	if expire <= 0 {
		return 0, errors.New("expire time must be positive")
	}

	return incrementWithExpireScript.Run(
		ctx,
		c.rdb,
		[]string{key},
		expire.Milliseconds(),
	).Int64()
}

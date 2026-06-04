package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server              ServerConfig        `yaml:"server"`
	Database            DatabaseConfig      `yaml:"database"`
	Redis               RedisConfig         `yaml:"redis"`
	JWT                 JWTConfig           `yaml:"jwt"`
	RabbitMQ            RabbitMQConfig      `yaml:"rabbitmq"`
	Moderation          ModerationConfig    `yaml:"moderation"`
	ObservabilityConfig ObservabilityConfig `yaml:"observability"`
}

// ModerationConfig 内容审核相关配置。
type ModerationConfig struct {
	// AdminAccountIDs 允许调用 /moderation/review 的账号 ID 白名单。
	AdminAccountIDs []uint `yaml:"admin_account_ids"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type JWTConfig struct {
	Secret string        `yaml:"secret"`
	Issuer string        `yaml:"issuer"`
	TTL    time.Duration `yaml:"ttl"`
}

type RabbitMQConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type ObservabilityConfig struct {
	Pprof PprofConfig `yaml:"pprof"`
}
type PprofConfig struct {
	Enabled    bool   `yaml:"enabled"`
	ApiAddr    string `yaml:"api_addr"`
	WorkerAddr string `yaml:"worker_addr"`
}

func load(filename string) (Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return Config{}, fmt.Errorf("failed to load config file %s : %w", filename, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", filename, err)
	}

	applyEnvOverrides(&cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}

	if v := os.Getenv("SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}

	if v := os.Getenv("MYSQL_HOST"); v != "" {
		cfg.Database.Host = v
	}

	if v := os.Getenv("MYSQL_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Database.Port = port
		}
	}

	if v := os.Getenv("MYSQL_USER"); v != "" {
		cfg.Database.User = v
	}

	if v := os.Getenv("MYSQL_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}

	if v := os.Getenv("MYSQL_DATABASE"); v != "" {
		cfg.Database.DBName = v
	}

	if v := os.Getenv("REDIS_HOST"); v != "" {
		cfg.Redis.Host = v
	}

	if v := os.Getenv("REDIS_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Redis.Port = port
		}
	}

	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}

	if v := os.Getenv("REDIS_DB"); v != "" {
		if db, err := strconv.Atoi(v); err == nil {
			cfg.Redis.DB = db
		}
	}

	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}

	if v := os.Getenv("JWT_ISSUER"); v != "" {
		cfg.JWT.Issuer = v
	}

	if v := os.Getenv("JWT_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.JWT.TTL = d
		}
	}

	if v := os.Getenv("RABBITMQ_HOST"); v != "" {
		cfg.RabbitMQ.Host = v
	}

	if v := os.Getenv("RABBITMQ_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.RabbitMQ.Port = port
		}
	}

	if v := os.Getenv("RABBITMQ_USER"); v != "" {
		cfg.RabbitMQ.Username = v
	}

	if v := os.Getenv("RABBITMQ_PASS"); v != "" {
		cfg.RabbitMQ.Password = v
	}

	// MODERATION_ADMIN_IDS=1,2 覆盖 yaml 中的 admin_account_ids（便于部署注入）。
	if v := os.Getenv("MODERATION_ADMIN_IDS"); v != "" {
		cfg.Moderation.AdminAccountIDs = parseUintList(v)
	}
}

// parseUintList 解析逗号分隔的无符号整数列表。
func parseUintList(raw string) []uint {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]uint, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if u, err := strconv.ParseUint(p, 10, 64); err == nil && u > 0 {
			out = append(out, uint(u))
		}
	}
	return out
}

// ResolveConfigPath 优先 CONFIG_PATH，其次 configs/config.yaml，最后 config.yaml。
func ResolveConfigPath() string {
	if p := os.Getenv("CONFIG_PATH"); p != "" {
		return p
	}
	if _, err := os.Stat("configs/config.yaml"); err == nil {
		return "configs/config.yaml"
	}
	return "config.yaml"
}

func LoadConfig(filename string) (Config, bool, error) {
	if cfg, err := load(filename); err == nil {
		return cfg, false, nil
	} else {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultLocalConfig(), true, nil
		} else {
			return Config{}, false, err
		}
	}
}

func DefaultLocalConfig() Config {
	cfg := Config{
		Server: ServerConfig{
			Port: 8080,
		},
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     3306,
			User:     "root",
			Password: "123456",
			DBName:   "feedsystem",
		},
		Redis: RedisConfig{
			Host:     "localhost",
			Port:     6379,
			Password: "123456",
			DB:       0,
		},
		JWT: JWTConfig{
			Secret: "dev-secret-change-in-production",
			Issuer: "pulsefeed",
			TTL:    24 * time.Hour,
		},
		RabbitMQ: RabbitMQConfig{
			Host:     "localhost",
			Port:     5672,
			Username: "admin",
			Password: "password123",
		},
		Moderation: ModerationConfig{
			// 本地默认账号 ID=1 为审核员，请按实际库中管理员账号修改。
			AdminAccountIDs: []uint{1},
		},

		ObservabilityConfig: ObservabilityConfig{
			Pprof: PprofConfig{
				Enabled:    true,
				ApiAddr:    "localhost:6060",
				WorkerAddr: "localhost:6061",
			},
		},
	}
	applyEnvOverrides(&cfg)
	return cfg
}

package config

import (
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	JWT      JWTConfig      `yaml:"jwt"`
}

type ServerConfig struct {
	Addr string
	Mode string
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret string
	Issuer string
	TTL    time.Duration
}

func Load() Config {
	_ = godotenv.Load()

	return Config{}
}

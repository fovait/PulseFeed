package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"PulseFeed/internal/config"
	"PulseFeed/internal/db"
	apphttp "PulseFeed/internal/http"
	"PulseFeed/internal/middleware/rabbitmq"
	rediscache "PulseFeed/internal/middleware/redis"
	"PulseFeed/internal/observability"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

const shutdownTimeout = 15 * time.Second

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println(".env not found; continuing")
	}

	configPath := config.ResolveConfigPath()
	log.Printf("Loading config from %s", configPath)
	cfg, usedDefault, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if usedDefault {
		log.Printf("Config file %s not found, using default local config", configPath)
	} else {
		log.Printf("Config loaded from file: %s", configPath)
	}

	if cfg.Server.Mode != "" {
		gin.SetMode(cfg.Server.Mode)
	}

	sqlDB, err := db.NewDB(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}
	if err := db.AutoMigrate(sqlDB); err != nil {
		log.Fatalf("Failed to auto migrate database: %v", err)
	}
	defer db.CloseDB(sqlDB)

	cache, err := rediscache.NewFromConfig(&cfg.Redis)
	if err != nil {
		log.Printf("Redis config error (cache disabled): %v", err)
		cache = nil
	} else {
		pingCtx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		pingErr := cache.Ping(pingCtx)
		cancel()
		if pingErr != nil {
			log.Printf("Redis not available (cache disabled): %v", pingErr)
			_ = cache.Close()
			cache = nil
		} else {
			defer cache.Close()
			log.Printf("Redis connected (cache enabled)")
		}
	}

	rmq, err := rabbitmq.NewRabbitMQ(&cfg.RabbitMQ)
	if err != nil {
		log.Printf("RabbitMQ config error (disabled): %v", err)
		rmq = nil
	} else {
		defer rmq.Close()
		log.Printf("RabbitMQ connected")
	}

	pprofServer, err := observability.NewPprofServer(
		"API",
		cfg.ObservabilityConfig.Pprof.Enabled,
		cfg.ObservabilityConfig.Pprof.ApiAddr,
	)
	if err != nil {
		log.Printf("Failed to start API pprof server: %v", err)
	}
	if pprofServer != nil {
		defer pprofServer.Close()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	r, bg := apphttp.SetRouter(sqlDB, cache, rmq)
	bg.Start(ctx)

	addr := ":" + strconv.Itoa(cfg.Server.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("Server is running on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Failed to run server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutdown signal received, stopping HTTP server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown: %v", err)
	}

	bg.Stop()
	log.Println("shutdown complete")
}

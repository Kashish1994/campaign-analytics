package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/viper"
	"github.com/zocket/campaign-analytics/internal/api/routes"
	"github.com/zocket/campaign-analytics/internal/config"
	"github.com/zocket/campaign-analytics/internal/infrastructure/database"
	"github.com/zocket/campaign-analytics/internal/infrastructure/platforms"
	"github.com/zocket/campaign-analytics/internal/infrastructure/redis"
	"go.uber.org/zap"
)

func main() {
	// Parse command line flags
	initSchema := flag.Bool("init-schema", false, "Initialize database schema")
	flag.Parse()

	// Initialize configuration
	if err := config.Init(); err != nil {
		log.Fatalf("Failed to initialize configuration: %v", err)
	}

	// Initialize logger
	var logger *zap.Logger
	var err error
	if viper.GetBool("logging.development") {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	// Initialize dependencies
	ctx := context.Background()
	redisClient, err := redis.NewClient(ctx)
	if err != nil {
		logger.Fatal("Failed to initialize Redis client", zap.Error(err))
	}

	clickhouseClient, err := database.NewClickHouseClient()
	if err != nil {
		logger.Fatal("Failed to initialize ClickHouse client", zap.Error(err))
	}

	postgresClient, err := database.NewPostgresClient()
	if err != nil {
		logger.Fatal("Failed to initialize Postgres client", zap.Error(err))
	}

	// Initialize schemas if requested
	if *initSchema {
		logger.Info("Initializing database schemas")
		if err := clickhouseClient.InitSchema(ctx); err != nil {
			logger.Fatal("Failed to initialize ClickHouse schema", zap.Error(err))
		}
		if err := postgresClient.InitSchema(ctx); err != nil {
			logger.Fatal("Failed to initialize Postgres schema", zap.Error(err))
		}
		logger.Info("Database schemas initialized successfully")
		return
	}

	// Platform clients
	platformClients := platforms.NewPlatformClients()

	// Initialize router
	router := routes.SetupRouter(
		clickhouseClient,
		postgresClient,
		redisClient,
		platformClients,
		logger,
	)

	// Configure server
	serverPort := viper.GetString("server.port")
	if serverPort == "" {
		serverPort = "8080"
	}

	server := &http.Server{
		Addr:         ":" + serverPort,
		Handler:      router,
		ReadTimeout:  viper.GetDuration("server.read_timeout"),
		WriteTimeout: viper.GetDuration("server.write_timeout"),
		IdleTimeout:  viper.GetDuration("server.idle_timeout"),
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Starting API server", zap.String("address", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown server
	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exiting")
}

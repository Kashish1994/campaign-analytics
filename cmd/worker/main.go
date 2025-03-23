package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zocket/campaign-analytics/internal/config"
	"github.com/zocket/campaign-analytics/internal/domain/services"
	"github.com/zocket/campaign-analytics/internal/infrastructure/database"
	"github.com/zocket/campaign-analytics/internal/infrastructure/kafka"
	"github.com/zocket/campaign-analytics/internal/infrastructure/redis"
	"github.com/zocket/campaign-analytics/internal/version"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func main() {
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	redisClient, err := redis.NewClient(ctx)
	if err != nil {
		logger.Fatal("Failed to initialize Redis client", zap.Error(err))
	}

	clickhouseClient, err := database.NewClickHouseClient()
	if err != nil {
		logger.Fatal("Failed to initialize ClickHouse client", zap.Error(err))
	}

	// Initialize Kafka consumer
	// We're passing an array with a single topic, which the updated NewConsumer will use correctly
	consumer, err := kafka.NewConsumer([]string{"campaign_events"})
	if err != nil {
		logger.Fatal("Failed to initialize Kafka consumer", zap.Error(err))
	}

	// Initialize processors
	eventProcessor := services.NewEventProcessor(clickhouseClient, redisClient, logger)
	aggregationService := services.NewAggregationService(clickhouseClient, redisClient, logger)

	// Start worker
	worker := services.NewWorker(consumer, eventProcessor, aggregationService, logger)
	go worker.Start(ctx)

	// Setup health check HTTP server
	router := gin.New()
	router.Use(gin.Recovery())
	
	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "worker",
			"version": version.GetVersion(),
		})
	})
	
	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	
	// Start HTTP server for health checks
	server := &http.Server{
		Addr:    ":8081", // Different port from API
		Handler: router,
	}
	
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Failed to start health check server", zap.Error(err))
		}
	}()
	
	logger.Info("Worker service started", zap.String("health_endpoint", "http://localhost:8081/health"))

	// Wait for interrupt signal to gracefully shut down the worker
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down worker...")
	
	// Shutdown HTTP server
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
	}
	
	// Stop the worker
	cancel()

	logger.Info("Worker exiting")
}

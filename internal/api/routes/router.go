package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"github.com/zocket/campaign-analytics/internal/api/handlers"
	"github.com/zocket/campaign-analytics/internal/api/middlewares"
	"github.com/zocket/campaign-analytics/internal/domain/services"
	"github.com/zocket/campaign-analytics/internal/infrastructure/database"
	"github.com/zocket/campaign-analytics/internal/infrastructure/platforms"
	"github.com/zocket/campaign-analytics/internal/infrastructure/redis"
	"github.com/zocket/campaign-analytics/internal/version"
	"go.uber.org/zap"
)

// SetupRouter sets up the HTTP router
func SetupRouter(
	clickhouseDB *database.ClickHouseClient,
	postgresDB *database.PostgresClient,
	redisClient *redis.Client,
	platformClients *platforms.PlatformClients,
	logger *zap.Logger,
) *gin.Engine {
	// Create the Gin router
	router := gin.New()

	// Get configuration
	jwtKey := viper.GetString("jwt.key")
	if jwtKey == "" {
		jwtKey = "default-secret-key" // Default key for development
	}

	// Create middlewares
	loggerMiddleware := middlewares.NewLoggerMiddleware(logger)
	authMiddleware := middlewares.NewAuthMiddleware(logger, jwtKey)
	rateLimiter := middlewares.NewRateLimiterMiddleware(redisClient.GetClient(), logger)

	// Apply global middlewares
	router.Use(gin.Recovery())
	router.Use(loggerMiddleware.Logger())
	router.Use(loggerMiddleware.ErrorLogger())

	// Apply rate limiting with defaults if config not set
	rateLimit := viper.GetInt("rate_limiting.default_rate")
	if rateLimit <= 0 {
		rateLimit = 100
	}
	router.Use(rateLimiter.RateLimit(rateLimit, 60)) // Default: 100 requests per minute

	// Create service instances
	campaignService, err := services.NewCampaignService(
		postgresDB,
		platformClients,
		logger,
	)
	if err != nil {
		logger.Fatal("Failed to create campaign service", zap.Error(err))
	}

	aggregationService := services.NewAggregationService(
		clickhouseDB,
		redisClient,
		logger,
	)

	// Create handlers
	authHandler := handlers.NewAuthHandler(
		postgresDB,
		logger,
		jwtKey,
	)

	campaignHandler := handlers.NewCampaignHandler(
		campaignService,
		aggregationService,
		logger,
	)

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"version": version.GetVersion(),
		})
	})

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Authentication routes
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
		}

		// Campaign routes (protected)
		campaigns := v1.Group("/campaigns")
		campaigns.Use(authMiddleware.AuthRequired())
		{
			campaigns.GET("", campaignHandler.ListCampaigns)
			campaigns.POST("", campaignHandler.CreateCampaign)
			campaigns.GET("/:id", campaignHandler.GetCampaign)
			campaigns.PUT("/:id", campaignHandler.UpdateCampaign)
			campaigns.POST("/:id/fetch-data", campaignHandler.FetchCampaignData)
			campaigns.GET("/:id/insights", campaignHandler.GetCampaignInsights)
			campaigns.POST("/:id/reaggregate", campaignHandler.TriggerInsightsReaggregation)
		}

		// Admin routes (protected + role requirement)
		admin := v1.Group("/admin")
		admin.Use(authMiddleware.AuthRequired())
		admin.Use(authMiddleware.RoleRequired("admin"))
		{
			// Admin-only endpoints here
		}
	}

	return router
}

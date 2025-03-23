package middlewares

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// RateLimiterMiddleware implements rate limiting using Redis
type RateLimiterMiddleware struct {
	redisClient *redis.Client
	logger      *zap.Logger
}

// NewRateLimiterMiddleware creates a new rate limiter middleware
func NewRateLimiterMiddleware(redisClient *redis.Client, logger *zap.Logger) *RateLimiterMiddleware {
	return &RateLimiterMiddleware{
		redisClient: redisClient,
		logger:      logger.With(zap.String("component", "rate_limiter_middleware")),
	}
}

// RateLimit limits requests based on IP address or user ID
// rate is the number of requests allowed in the period
// period is the time frame in seconds
func (m *RateLimiterMiddleware) RateLimit(rate int, period int) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the key for rate limiting (user ID if authenticated, IP otherwise)
		var key string
		if userID, exists := c.Get("user_id"); exists {
			key = fmt.Sprintf("rate_limit:%s", userID)
		} else {
			key = fmt.Sprintf("rate_limit:%s", c.ClientIP())
		}

		// Get the current count
		ctx := context.Background()
		count, err := m.redisClient.Get(ctx, key).Int()
		if err != nil && err != redis.Nil {
			m.logger.Error("Failed to get rate limit count", zap.Error(err), zap.String("key", key))
			// Allow the request to proceed if Redis is having issues
			c.Next()
			return
		}

		// Check if the rate limit is exceeded
		if count >= rate {
			c.Header("X-RateLimit-Limit", strconv.Itoa(rate))
			c.Header("X-RateLimit-Remaining", "0")
			retryAfter := m.redisClient.TTL(ctx, key).Val()
			c.Header("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))

			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
				"limit": rate,
				"retry_after": retryAfter.Seconds(),
			})
			return
		}

		// Increment the counter
		if count == 0 {
			// First request in the period, set the key with expiration
			err = m.redisClient.Set(ctx, key, 1, time.Duration(period)*time.Second).Err()
		} else {
			// Increment the existing key
			err = m.redisClient.Incr(ctx, key).Err()
		}

		if err != nil {
			m.logger.Error("Failed to increment rate limit counter", zap.Error(err), zap.String("key", key))
			// Allow the request to proceed if Redis is having issues
			c.Next()
			return
		}

		// Set response headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(rate))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(rate-count-1))

		c.Next()
	}
}

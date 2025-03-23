package middlewares

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// LoggerMiddleware is a middleware that logs HTTP requests
type LoggerMiddleware struct {
	logger *zap.Logger
}

// NewLoggerMiddleware creates a new logger middleware
func NewLoggerMiddleware(logger *zap.Logger) *LoggerMiddleware {
	return &LoggerMiddleware{
		logger: logger.With(zap.String("component", "logger_middleware")),
	}
}

// Logger is a middleware that logs HTTP requests
func (m *LoggerMiddleware) Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		method := c.Request.Method
		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()

		// Process request
		c.Next()

		// Get request result
		latency := time.Since(start)
		statusCode := c.Writer.Status()
		bodySize := c.Writer.Size()

		// Get user ID if available
		var userID string
		if id, exists := c.Get("user_id"); exists {
			userID = id.(string)
		}

		// Log the request
		m.logger.Info("HTTP request",
			zap.String("method", method),
			zap.String("path", path),
			zap.String("query", query),
			zap.Int("status", statusCode),
			zap.Int("size", bodySize),
			zap.Duration("latency", latency),
			zap.String("client_ip", clientIP),
			zap.String("user_agent", userAgent),
			zap.String("user_id", userID),
		)
	}
}

// ErrorLogger logs errors that occur during request processing
func (m *LoggerMiddleware) ErrorLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check if there are any errors
		if len(c.Errors) > 0 {
			for _, e := range c.Errors {
				m.logger.Error("Request error",
					zap.String("error", e.Error()),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
				)
			}
		}
	}
}

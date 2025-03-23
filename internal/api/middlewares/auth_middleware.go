package middlewares

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AuthMiddleware is a middleware that checks for a valid JWT token
type AuthMiddleware struct {
	logger *zap.Logger
	jwtKey []byte
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(logger *zap.Logger, jwtKey string) *AuthMiddleware {
	return &AuthMiddleware{
		logger: logger.With(zap.String("component", "auth_middleware")),
		jwtKey: []byte(jwtKey),
	}
}

// AuthRequired checks for a valid JWT token and sets the user ID in the context
func (m *AuthMiddleware) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the token from the Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			return
		}

		// Extract the token from the "Bearer" prefix
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenStr == authHeader {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
			return
		}

		// Parse the JWT token
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			// Validate the signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return m.jwtKey, nil
		})

		if err != nil {
			m.logger.Info("Invalid JWT token", zap.Error(err))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		// Check if the token is valid
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}

		// Extract user ID from token
		userIDStr, ok := claims["user_id"].(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token: missing user ID"})
			return
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			m.logger.Error("Failed to parse user ID from token", zap.Error(err), zap.String("user_id", userIDStr))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token: invalid user ID"})
			return
		}

		// Extract role from token
		role, ok := claims["role"].(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token: missing role"})
			return
		}

		// Set user ID and role in the context
		c.Set("user_id", userID)
		c.Set("role", role)
		c.Set("email", claims["email"].(string))

		c.Next()
	}
}

// RoleRequired checks if the user has the required role
func (m *AuthMiddleware) RoleRequired(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if user is authenticated first
		userRole, exists := c.Get("role")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			return
		}

		// Check if user has one of the required roles
		hasRole := false
		for _, role := range roles {
			if userRole == role {
				hasRole = true
				break
			}
		}

		if !hasRole {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			return
		}

		c.Next()
	}
}

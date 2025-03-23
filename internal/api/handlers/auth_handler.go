package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/zocket/campaign-analytics/internal/domain/models"
	"github.com/zocket/campaign-analytics/internal/infrastructure/database"
	"golang.org/x/crypto/bcrypt"
	"go.uber.org/zap"
)

// AuthHandler handles authentication requests
type AuthHandler struct {
	db     *database.PostgresClient
	logger *zap.Logger
	jwtKey []byte
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(
	db *database.PostgresClient,
	logger *zap.Logger,
	jwtKey string,
) *AuthHandler {
	return &AuthHandler{
		db:     db,
		logger: logger.With(zap.String("component", "auth_handler")),
		jwtKey: []byte(jwtKey),
	}
}

// Register handles POST /auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var creds models.UserCredentials
	if err := c.ShouldBindJSON(&creds); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user already exists
	var existingUser models.User
	err := h.db.GetDB().Get(&existingUser, "SELECT * FROM users WHERE email = $1", creds.Email)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		h.logger.Error("Failed to check existing user", zap.Error(err), zap.String("email", creds.Email))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(creds.Password), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("Failed to hash password", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Create a new user
	user := models.User{
		ID:        uuid.New(),
		Email:     creds.Email,
		Name:      c.PostForm("name"),
		Password:  string(hashedPassword),
		Role:      "user",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Insert the user
	_, err = h.db.GetDB().Exec(
		"INSERT INTO users (id, email, name, password, role, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		user.ID, user.Email, user.Name, user.Password, user.Role, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		h.logger.Error("Failed to create user", zap.Error(err), zap.String("email", user.Email))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Generate JWT token
	token, err := h.generateToken(user)
	if err != nil {
		h.logger.Error("Failed to generate JWT token", zap.Error(err), zap.String("user_id", user.ID.String()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate authentication token"})
		return
	}

	// Clear password before sending response
	user.Password = ""

	c.JSON(http.StatusCreated, models.AuthResponse{
		Token: token,
		User:  user,
	})
}

// Login handles POST /auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var creds models.UserCredentials
	if err := c.ShouldBindJSON(&creds); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get the user from the database
	var user models.User
	err := h.db.GetDB().Get(&user, "SELECT * FROM users WHERE email = $1", creds.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}
		h.logger.Error("Failed to get user", zap.Error(err), zap.String("email", creds.Email))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Check password
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(creds.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Generate JWT token
	token, err := h.generateToken(user)
	if err != nil {
		h.logger.Error("Failed to generate JWT token", zap.Error(err), zap.String("user_id", user.ID.String()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate authentication token"})
		return
	}

	// Clear password before sending response
	user.Password = ""

	c.JSON(http.StatusOK, models.AuthResponse{
		Token: token,
		User:  user,
	})
}

// generateToken generates a JWT token for a user
func (h *AuthHandler) generateToken(user models.User) (string, error) {
	// Set token expiration to 24 hours
	expirationTime := time.Now().Add(24 * time.Hour)

	// Create claims
	claims := jwt.MapClaims{
		"user_id": user.ID.String(),
		"email":   user.Email,
		"role":    user.Role,
		"exp":     expirationTime.Unix(),
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with our secret key
	tokenString, err := token.SignedString(h.jwtKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

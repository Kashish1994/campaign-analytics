package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Campaign represents a campaign model
type Campaign struct {
	ID     string  `db:"id" json:"id"`
	Budget float64 `db:"budget" json:"budget"`
	Spend  float64 `db:"spend" json:"spend"`
}

// App encapsulates application dependencies
type App struct {
	db                *sqlx.DB
	campaignSpendsMap sync.Map // Thread-safe map for in-memory caching
	logger            *log.Logger
}

// NewApp creates a new application instance
func NewApp() (*App, error) {
	logger := log.New(os.Stdout, "[CAMPAIGN-API] ", log.LstdFlags)

	// ISSUE 1: Hard-coded credentials in the code
	// FIXED: Use environment variables for database configuration
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "admin")
	dbPassword := getEnv("DB_PASSWORD", "")
	dbName := getEnv("DB_NAME", "zocket")
	dbSSLMode := getEnv("DB_SSL_MODE", "disable")

	// Construct connection string with proper error handling
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		dbHost, dbPort, dbUser, dbPassword, dbName, dbSSLMode,
	)

	// ISSUE 2: No connection pooling configuration
	// FIXED: Configure connection pool settings
	db, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("database connection failed: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	logger.Println("Database connected successfully")

	return &App{
		db:                db,
		campaignSpendsMap: sync.Map{},
		logger:            logger,
	}, nil
}

// Close properly shuts down the application
func (app *App) Close() {
	if app.db != nil {
		app.logger.Println("Closing database connection")
		err := app.db.Close()
		if err != nil {
			app.logger.Printf("Error closing database connection: %v", err)
		}
	}
}

// UpdateSpendRequest represents the request body for updating spend
type UpdateSpendRequest struct {
	Spend float64 `json:"spend" binding:"required"`
}

// ISSUE 3: Race condition in map access and lack of atomicity in database updates
// FIXED: Use a thread-safe map and database transactions
func (app *App) UpdateSpend(c *gin.Context) {
	campaignID := c.Param("campaign_id")
	
	// Input validation
	if campaignID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Campaign ID is required"})
		return
	}

	var request UpdateSpendRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: " + err.Error()})
		return
	}

	// ISSUE 4: No validation of input values
	// FIXED: Validate that spend is positive
	if request.Spend < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Spend amount cannot be negative"})
		return
	}

	// Start a transaction to ensure atomicity
	tx, err := app.db.Beginx()
	if err != nil {
		app.logger.Printf("Error starting transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process update"})
		return
	}
	
	// Defer a rollback in case anything fails
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			app.logger.Printf("Transaction rolled back due to panic: %v", r)
		}
	}()

	// ISSUE 5: No checking for record existence before update
	// FIXED: First check if the campaign exists
	var campaign Campaign
	err = tx.Get(&campaign, "SELECT id, budget, spend FROM campaigns WHERE id = $1 FOR UPDATE", campaignID)
	if err != nil {
		if err == sql.ErrNoRows {
			tx.Rollback()
			c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		} else {
			tx.Rollback()
			app.logger.Printf("Error fetching campaign: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch campaign data"})
		}
		return
	}

	// Update the database within the transaction
	_, err = tx.Exec("UPDATE campaigns SET spend = spend + $1 WHERE id = $2", request.Spend, campaignID)
	if err != nil {
		tx.Rollback()
		app.logger.Printf("Error updating campaign spend: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database update failed"})
		return
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		app.logger.Printf("Error committing transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit update"})
		return
	}

	// Update the cache after successful database update
	// Using LoadOrStore to safely handle concurrent operations
	var currentSpend float64
	if val, ok := app.campaignSpendsMap.Load(campaignID); ok {
		currentSpend = val.(float64)
	}
	app.campaignSpendsMap.Store(campaignID, currentSpend+request.Spend)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Spend updated successfully",
		"campaign_id": campaignID,
		"new_spend":   currentSpend + request.Spend,
	})
}

// BudgetStatusResponse represents the response for budget status API
type BudgetStatusResponse struct {
	CampaignID string  `json:"campaign_id"`
	Budget     float64 `json:"budget"`
	Spend      float64 `json:"spend"`
	Remaining  float64 `json:"remaining"`
	Status     string  `json:"status"`
}

func (app *App) GetBudgetStatus(c *gin.Context) {
	campaignID := c.Param("campaign_id")
	
	// Input validation
	if campaignID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Campaign ID is required"})
		return
	}

	// ISSUE 6: Inefficient query that selects all fields
	// FIXED: Only select the necessary fields and add proper error handling
	var campaign Campaign
	err := app.db.Get(&campaign, "SELECT id, budget, spend FROM campaigns WHERE id = $1", campaignID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		} else {
			app.logger.Printf("Error fetching campaign budget: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch campaign data"})
		}
		return
	}

	// Calculate remaining budget and status
	remaining := campaign.Budget - campaign.Spend
	status := "Active"
	if remaining <= 0 {
		status = "Overspent"
	}

	response := BudgetStatusResponse{
		CampaignID: campaignID,
		Budget:     campaign.Budget,
		Spend:      campaign.Spend,
		Remaining:  remaining,
		Status:     status,
	}

	c.JSON(http.StatusOK, response)
}

// Health check endpoint
func (app *App) HealthCheck(c *gin.Context) {
	err := app.db.Ping()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

// getEnv gets an environment variable or returns the fallback value
func getEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	return value
}

func main() {
	// Use a more secure gin mode for production
	if os.Getenv("GIN_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create and initialize the application
	app, err := NewApp()
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}
	defer app.Close()

	// Create a new gin router with proper middleware
	r := gin.New()

	// ISSUE 7: Missing middleware for logging, recovery, and CORS
	// FIXED: Add necessary middleware
	r.Use(gin.Recovery())
	r.Use(cors.Default()) // Add CORS support

	// Add routes
	r.POST("/campaigns/:campaign_id/spend", app.UpdateSpend)
	r.GET("/campaigns/:campaign_id/budget-status", app.GetBudgetStatus)
	r.GET("/health", app.HealthCheck)

	// Create a server with proper timeouts
	server := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start the server in a goroutine
	go func() {
		app.logger.Println("Starting server on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			app.logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	app.logger.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		app.logger.Fatalf("Server shutdown failed: %v", err)
	}
	app.logger.Println("Server gracefully stopped")
}

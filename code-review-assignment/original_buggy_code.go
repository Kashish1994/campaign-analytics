package main

import (
	"fmt"
	"net/http"
	"sync"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var db *sqlx.DB
var campaignSpends = make(map[string]float64)
var mu sync.Mutex

func initDB() {
	var err error
	db, err = sqlx.Connect("postgres", "host=localhost port=5432 user=admin dbname=zocket sslmode=disable")
	if err != nil {
		panic(err)
	}
	fmt.Println("Database connected")
}

// API to update campaign spend
func updateSpend(c *gin.Context) {
	campaignID := c.Param("campaign_id")
	var request struct {
		Spend float64 `json:"spend"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// Race condition here due to concurrent map writes
	campaignSpends[campaignID] += request.Spend

	// Inefficient: No transactions, no concurrency safety
	_, err := db.Exec("UPDATE campaigns SET spend = spend + $1 WHERE id = $2", request.Spend, campaignID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database update failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Spend updated"})
}

// API to get campaign budget status
func getBudgetStatus(c *gin.Context) {
	campaignID := c.Param("campaign_id")

	// Inefficient Query: Should only fetch required fields
	var budget, spend float64
	err := db.QueryRow("SELECT budget, spend FROM campaigns WHERE id = $1", campaignID).Scan(&budget, &spend)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch campaign data"})
		return
	}

	remaining := budget - spend
	status := "Active"
	if remaining <= 0 {
		status = "Overspent"
	}

	c.JSON(http.StatusOK, gin.H{
		"campaign_id": campaignID,
		"budget":      budget,
		"spend":       spend,
		"remaining":   remaining,
		"status":      status,
	})
}

func main() {
	initDB()
	r := gin.Default()
	r.POST("/campaigns/:campaign_id/spend", updateSpend)
	r.GET("/campaigns/:campaign_id/budget-status", getBudgetStatus)
	r.Run(":8080")
}

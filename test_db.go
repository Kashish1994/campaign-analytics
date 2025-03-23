package main

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	// Try explicit connection string
	connStr := "host=localhost port=5432 dbname=campaign_analytics user=postgres password=postgres sslmode=disable"
	
	fmt.Println("Attempting to connect to PostgreSQL...")
	db, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		fmt.Printf("Connection failed: %v\n", err)
		return
	}
	defer db.Close()
	
	fmt.Println("Successfully connected to PostgreSQL!")
	
	// Test query
	var result int
	err = db.Get(&result, "SELECT 1")
	if err != nil {
		fmt.Printf("Query failed: %v\n", err)
		return
	}
	
	fmt.Println("Query successful!")
}

package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
)

// PostgresClient wraps the sqlx DB with additional methods
type PostgresClient struct {
	db *sqlx.DB
}

// NewPostgresClient creates a new Postgres client
func NewPostgresClient() (*PostgresClient, error) {
	// Get configuration from environment or config file
	host := viper.GetString("postgres.host")
	port := viper.GetInt("postgres.port")
	database := viper.GetString("postgres.database")
	username := viper.GetString("postgres.username")
	password := viper.GetString("postgres.password")
	sslMode := viper.GetString("postgres.sslmode")

	// Use defaults if not provided
	if host == "" {
		host = "localhost"
	}
	if port == 0 {
		port = 5432
	}
	if database == "" {
		database = "campaign_analytics"
	}
	if sslMode == "" {
		sslMode = "disable"
	}

	// Create connection string
	connStr := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		host, port, database, username, password, sslMode,
	)

	// Connect to the database
	db, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Minute * 5)

	return &PostgresClient{db: db}, nil
}

// InitSchema initializes the database schema if it doesn't exist
func (c *PostgresClient) InitSchema(ctx context.Context) error {
	// Create users table
	if _, err := c.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			name VARCHAR(255) NOT NULL,
			password VARCHAR(255) NOT NULL,
			role VARCHAR(50) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return err
	}

	// Create campaigns table
	if _, err := c.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS campaigns (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users(id),
			name VARCHAR(255) NOT NULL,
			platform VARCHAR(50) NOT NULL,
			budget DECIMAL(12,2) NOT NULL,
			start_date TIMESTAMP WITH TIME ZONE NOT NULL,
			end_date TIMESTAMP WITH TIME ZONE NOT NULL,
			status VARCHAR(50) NOT NULL,
			external_id VARCHAR(255),
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return err
	}

	// Create platform_credentials table
	if _, err := c.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS platform_credentials (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users(id),
			platform VARCHAR(50) NOT NULL,
			credentials JSONB NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			UNIQUE(user_id, platform)
		)
	`); err != nil {
		return err
	}

	return nil
}

// Close closes the Postgres connection
func (c *PostgresClient) Close() error {
	return c.db.Close()
}

// GetDB returns the underlying sqlx DB
func (c *PostgresClient) GetDB() *sqlx.DB {
	return c.db
}

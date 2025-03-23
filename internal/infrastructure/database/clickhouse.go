package database

import (
	"context"
	"strconv"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/spf13/viper"
)

// ClickHouseClient wraps the ClickHouse client with additional methods
type ClickHouseClient struct {
	conn driver.Conn
}

// NewClickHouseClient creates a new ClickHouse client
func NewClickHouseClient() (*ClickHouseClient, error) {
	// Get configuration from environment or config file
	host := viper.GetString("clickhouse.host")
	port := viper.GetInt("clickhouse.port")
	database := viper.GetString("clickhouse.database")
	username := viper.GetString("clickhouse.username")
	password := viper.GetString("clickhouse.password")

	// Use defaults if not provided
	if host == "" {
		host = "localhost"
	}
	if port == 0 {
		port = 9000
	}
	if database == "" {
		database = "campaign_analytics"
	}

	// Create a connection to ClickHouse
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{host + ":" + strconv.Itoa(port)},
		Auth: clickhouse.Auth{
			Database: database,
			Username: username,
			Password: password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout:     time.Second * 10,
		MaxOpenConns:    50,
		MaxIdleConns:    10,
		ConnMaxLifetime: time.Hour,
	})
	if err != nil {
		return nil, err
	}

	// Check the connection
	if err := conn.Ping(context.Background()); err != nil {
		return nil, err
	}

	return &ClickHouseClient{conn: conn}, nil
}

// InitSchema initializes the database schema if it doesn't exist
func (c *ClickHouseClient) InitSchema(ctx context.Context) error {
	// Create campaign_events table to store raw events
	if err := c.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS campaign_events (
			id UUID,
			campaign_id UUID,
			platform String,
			event_type String,
			impressions Int64,
			clicks Int64,
			conversions Int64,
			spend Float64,
			revenue Float64,
			event_time DateTime,
			region String,
			currency String,
			deduplication_key String,
			received_at DateTime,
			processed_at DateTime,
			PRIMARY KEY (campaign_id, event_time, deduplication_key)
		) ENGINE = ReplacingMergeTree(processed_at)
		PARTITION BY toYYYYMM(event_time)
		ORDER BY (campaign_id, event_time, deduplication_key)
	`); err != nil {
		return err
	}

	// Create campaign_insights table for aggregated metrics
	if err := c.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS campaign_insights (
			campaign_id UUID,
			date Date,
			platform String,
			region String,
			impressions Int64,
			clicks Int64,
			conversions Int64,
			spend Float64,
			revenue Float64,
			ctr Float64,
			cpc Float64,
			cpa Float64,
			roas Float64,
			conversion_rate Float64,
			updated_at DateTime,
			PRIMARY KEY (campaign_id, date, platform, region)
		) ENGINE = ReplacingMergeTree(updated_at)
		PARTITION BY toYYYYMM(date)
		ORDER BY (campaign_id, date, platform, region)
	`); err != nil {
		return err
	}

	// Create a materialized view to automatically aggregate daily insights
	if err := c.conn.Exec(ctx, `
		CREATE MATERIALIZED VIEW IF NOT EXISTS mv_campaign_daily_aggregation
		TO campaign_insights
		AS SELECT
			campaign_id,
			toDate(event_time) as date,
			platform,
			region,
			sum(impressions) as impressions,
			sum(clicks) as clicks,
			sum(conversions) as conversions,
			sum(spend) as spend,
			sum(revenue) as revenue,
			if(sum(impressions) > 0, sum(clicks) / sum(impressions), 0) as ctr,
			if(sum(clicks) > 0, sum(spend) / sum(clicks), 0) as cpc,
			if(sum(conversions) > 0, sum(spend) / sum(conversions), 0) as cpa,
			if(sum(spend) > 0, sum(revenue) / sum(spend), 0) as roas,
			if(sum(clicks) > 0, sum(conversions) / sum(clicks), 0) as conversion_rate,
			now() as updated_at
		FROM campaign_events
		GROUP BY campaign_id, toDate(event_time), platform, region
	`); err != nil {
		return err
	}

	return nil
}

// Close closes the ClickHouse connection
func (c *ClickHouseClient) Close() error {
	return c.conn.Close()
}

// GetConn returns the underlying ClickHouse connection
func (c *ClickHouseClient) GetConn() driver.Conn {
	return c.conn
}

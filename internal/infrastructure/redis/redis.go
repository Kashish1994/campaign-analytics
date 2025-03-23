package redis

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
)

// Client wraps the Redis client with additional methods
type Client struct {
	client *redis.Client
}

// NewClient creates a new Redis client
func NewClient(ctx context.Context) (*Client, error) {
	// Get configuration from environment or config file
	addr := viper.GetString("redis.addr")
	password := viper.GetString("redis.password")
	db := viper.GetInt("redis.db")

	// Use defaults if not provided
	if addr == "" {
		addr = "localhost:6379"
	}

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 5,
	})

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &Client{client: client}, nil
}

// Set sets a key with value and expiration
func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	var data []byte
	var err error

	switch v := value.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		data, err = json.Marshal(value)
		if err != nil {
			return err
		}
	}

	return c.client.Set(ctx, key, data, expiration).Err()
}

// Get gets a value by key
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

// GetObject gets a value by key and unmarshals it into the provided object
func (c *Client) GetObject(ctx context.Context, key string, obj interface{}) error {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}

	return json.Unmarshal(data, obj)
}

// Delete deletes a key
func (c *Client) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// IsDeduplicationKeyProcessed checks if a deduplication key has been processed
func (c *Client) IsDeduplicationKeyProcessed(ctx context.Context, key string) (bool, error) {
	exists, err := c.client.Exists(ctx, "dedup:"+key).Result()
	return exists > 0, err
}

// MarkDeduplicationKeyProcessed marks a deduplication key as processed
func (c *Client) MarkDeduplicationKeyProcessed(ctx context.Context, key string, expiration time.Duration) error {
	return c.client.Set(ctx, "dedup:"+key, "1", expiration).Err()
}

// Close closes the Redis client
func (c *Client) Close() error {
	return c.client.Close()
}

// GetClient returns the underlying Redis client
func (c *Client) GetClient() *redis.Client {
	return c.client
}

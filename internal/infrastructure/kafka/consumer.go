package kafka

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
)

// Consumer wraps the Kafka reader with additional methods
type Consumer struct {
	reader *kafka.Reader
}

// NewConsumer creates a new Kafka consumer
func NewConsumer(topics []string) (*Consumer, error) {
	// Get configuration from environment or config file
	brokers := viper.GetStringSlice("kafka.brokers")
	groupID := viper.GetString("kafka.consumer.group_id")

	// Use defaults if not provided
	if len(brokers) == 0 {
		brokers = []string{"localhost:9092"}
	}
	if groupID == "" {
		groupID = "campaign-analytics-consumer"
	}

	// Create Kafka reader
	// Note: kafka-go only supports a single topic per reader as of the current version
	// If multiple topics are needed, create multiple readers or use a different approach
	if len(topics) == 0 {
		// Fallback to a default topic
		topics = []string{"campaign_events"}
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		GroupID:        groupID,
		Topic:          topics[0], // Use the first topic from the list
		MinBytes:       10e3,    // 10KB
		MaxBytes:       10e6,    // 10MB
		MaxWait:        1 * time.Second,
		StartOffset:    kafka.FirstOffset,
		CommitInterval: 1 * time.Second,
		RetentionTime:  7 * 24 * time.Hour, // 1 week
	})

	return &Consumer{reader: reader}, nil
}

// ReadMessage reads a message from Kafka
func (c *Consumer) ReadMessage(ctx context.Context) (kafka.Message, error) {
	return c.reader.ReadMessage(ctx)
}

// CommitMessages commits messages up to the provided message
func (c *Consumer) CommitMessages(ctx context.Context, msg kafka.Message) error {
	return c.reader.CommitMessages(ctx, msg)
}

// Close closes the Kafka consumer
func (c *Consumer) Close() error {
	return c.reader.Close()
}

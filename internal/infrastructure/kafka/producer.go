package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
)

// Producer wraps the Kafka writer with additional methods
type Producer struct {
	writer *kafka.Writer
}

// NewProducer creates a new Kafka producer
func NewProducer(topic string) (*Producer, error) {
	// Get configuration from environment or config file
	brokers := viper.GetStringSlice("kafka.brokers")

	// Use defaults if not provided
	if len(brokers) == 0 {
		brokers = []string{"localhost:9092"}
	}

	// Create Kafka writer
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		// Ensure at-least-once delivery
		RequiredAcks: kafka.RequireAll,
		// Retry delivery up to 10 times
		MaxAttempts: 10,
	}

	return &Producer{writer: writer}, nil
}

// SendMessage sends a message to Kafka
func (p *Producer) SendMessage(ctx context.Context, key string, value interface{}) error {
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

	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: data,
		Time:  time.Now(),
	})
}

// Close closes the Kafka producer
func (p *Producer) Close() error {
	return p.writer.Close()
}

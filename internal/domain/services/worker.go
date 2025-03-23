package services

import (
	"context"
	"time"

	"github.com/zocket/campaign-analytics/internal/infrastructure/kafka"
	"go.uber.org/zap"
)

// Worker processes messages from Kafka
type Worker struct {
	consumer           *kafka.Consumer
	eventProcessor     *EventProcessor
	aggregationService *AggregationService
	logger             *zap.Logger
}

// NewWorker creates a new worker
func NewWorker(
	consumer *kafka.Consumer,
	eventProcessor *EventProcessor,
	aggregationService *AggregationService,
	logger *zap.Logger,
) *Worker {
	return &Worker{
		consumer:           consumer,
		eventProcessor:     eventProcessor,
		aggregationService: aggregationService,
		logger:             logger.With(zap.String("component", "worker")),
	}
}

// Start starts the worker
func (w *Worker) Start(ctx context.Context) {
	w.logger.Info("Starting worker")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Worker shutting down")
			return
		default:
			// Read a message from Kafka
			msg, err := w.consumer.ReadMessage(ctx)
			if err != nil {
				w.logger.Error("Error reading message from Kafka", zap.Error(err))
				time.Sleep(1 * time.Second) // Backoff before retrying
				continue
			}

			// Process the message
			w.logger.Debug("Processing message",
				zap.String("topic", msg.Topic),
				zap.String("key", string(msg.Key)),
			)

			if err := w.processMessage(ctx, msg); err != nil {
				w.logger.Error("Error processing message",
					zap.Error(err),
					zap.String("key", string(msg.Key)),
				)
				// Do not commit the message so it will be reprocessed
				continue
			}

			// Commit the message
			if err := w.consumer.CommitMessages(ctx, msg); err != nil {
				w.logger.Error("Error committing message", zap.Error(err))
				// Continue processing other messages even if commit fails
			}
		}
	}
}

// processMessage processes a single message
func (w *Worker) processMessage(ctx context.Context, msg kafka.Message) error {
	// Process the event
	if err := w.eventProcessor.ProcessEvent(ctx, msg); err != nil {
		return err
	}

	// We could implement additional processing here,
	// like triggering aggregations or notifications

	return nil
}

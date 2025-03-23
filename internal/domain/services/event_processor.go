package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/zocket/campaign-analytics/internal/domain/models"
	"github.com/zocket/campaign-analytics/internal/infrastructure/database"
	"github.com/zocket/campaign-analytics/internal/infrastructure/kafka"
	"github.com/zocket/campaign-analytics/internal/infrastructure/redis"
	"go.uber.org/zap"
)

// EventProcessor processes campaign events
type EventProcessor struct {
	db     *database.ClickHouseClient
	redis  *redis.Client
	logger *zap.Logger
}

// NewEventProcessor creates a new event processor
func NewEventProcessor(
	db *database.ClickHouseClient,
	redis *redis.Client,
	logger *zap.Logger,
) *EventProcessor {
	return &EventProcessor{
		db:     db,
		redis:  redis,
		logger: logger.With(zap.String("component", "event_processor")),
	}
}

// ProcessEvent processes a single event
func (p *EventProcessor) ProcessEvent(ctx context.Context, msg kafka.Message) error {
	// Parse the event from the message
	var event models.CampaignEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		p.logger.Error("Failed to unmarshal event", zap.Error(err))
		return err
	}

	// Check if this event has already been processed (idempotence)
	isProcessed, err := p.redis.IsDeduplicationKeyProcessed(ctx, event.DeduplicationKey)
	if err != nil {
		p.logger.Error("Failed to check deduplication key", zap.Error(err), zap.String("key", event.DeduplicationKey))
		// Continue processing, worst case we process the same event twice
	}

	if isProcessed {
		p.logger.Debug("Skipping already processed event", zap.String("key", event.DeduplicationKey))
		return nil
	}

	// Validate the event
	if err := p.validateEvent(&event); err != nil {
		p.logger.Error("Event validation failed", zap.Error(err), zap.String("event_id", event.ID.String()))
		return err
	}

	// Set processed time
	event.ProcessedAt = time.Now()

	// Store the event in ClickHouse
	if err := p.storeEvent(ctx, &event); err != nil {
		p.logger.Error("Failed to store event", zap.Error(err), zap.String("event_id", event.ID.String()))
		return err
	}

	// Mark the event as processed in Redis to prevent duplicate processing
	if err := p.redis.MarkDeduplicationKeyProcessed(ctx, event.DeduplicationKey, 7*24*time.Hour); err != nil {
		p.logger.Error("Failed to mark event as processed", zap.Error(err), zap.String("key", event.DeduplicationKey))
		// Continue, as the event has been stored successfully
	}

	p.logger.Info("Event processed successfully", 
		zap.String("event_id", event.ID.String()),
		zap.String("campaign_id", event.CampaignID.String()),
		zap.String("platform", string(event.Platform)),
	)

	return nil
}

// validateEvent validates a campaign event
func (p *EventProcessor) validateEvent(event *models.CampaignEvent) error {
	// Ensure required fields are present
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if event.CampaignID == uuid.Nil {
		return ErrMissingCampaignID
	}
	if event.Platform == "" {
		return ErrMissingPlatform
	}
	if event.EventTime.IsZero() {
		return ErrMissingEventTime
	}
	if event.DeduplicationKey == "" {
		return ErrMissingDeduplicationKey
	}

	// Ensure non-negative metrics
	if event.Impressions < 0 {
		event.Impressions = 0
	}
	if event.Clicks < 0 {
		event.Clicks = 0
	}
	if event.Conversions < 0 {
		event.Conversions = 0
	}
	if event.Spend < 0 {
		event.Spend = 0
	}
	if event.Revenue < 0 {
		event.Revenue = 0
	}

	// Set defaults for optional fields
	if event.EventType == "" {
		event.EventType = "stats"
	}
	if event.Region == "" {
		event.Region = "all"
	}
	if event.Currency == "" {
		event.Currency = "USD"
	}
	if event.ReceivedAt.IsZero() {
		event.ReceivedAt = time.Now()
	}

	return nil
}

// storeEvent stores an event in ClickHouse
func (p *EventProcessor) storeEvent(ctx context.Context, event *models.CampaignEvent) error {
	// Insert the event into ClickHouse
	query := `
		INSERT INTO campaign_events (
			id, campaign_id, platform, event_type, impressions, clicks, conversions,
			spend, revenue, event_time, region, currency, deduplication_key,
			received_at, processed_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
	`

	conn := p.db.GetConn()
	
	err := conn.Exec(ctx, query,
		event.ID.String(),
		event.CampaignID.String(),
		event.Platform,
		event.EventType,
		event.Impressions,
		event.Clicks,
		event.Conversions,
		event.Spend,
		event.Revenue,
		event.EventTime,
		event.Region,
		event.Currency,
		event.DeduplicationKey,
		event.ReceivedAt,
		event.ProcessedAt,
	)

	return err
}

// Error definitions
var (
	ErrMissingCampaignID        = NewError("missing campaign ID")
	ErrMissingPlatform          = NewError("missing platform")
	ErrMissingEventTime         = NewError("missing event time")
	ErrMissingDeduplicationKey  = NewError("missing deduplication key")
)

// Error wraps errors with additional context
type Error struct {
	Message string
}

// NewError creates a new error
func NewError(message string) *Error {
	return &Error{Message: message}
}

func (e *Error) Error() string {
	return e.Message
}

package services

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/zocket/campaign-analytics/internal/domain/models"
	"github.com/zocket/campaign-analytics/internal/infrastructure/database"
	"github.com/zocket/campaign-analytics/internal/infrastructure/kafka"
	"github.com/zocket/campaign-analytics/internal/infrastructure/platforms"
	"go.uber.org/zap"
)

// CampaignService handles campaign-related operations
type CampaignService struct {
	db              *database.PostgresClient
	platformClients *platforms.PlatformClients
	producer        *kafka.Producer
	logger          *zap.Logger
}

// NewCampaignService creates a new campaign service
func NewCampaignService(
	db *database.PostgresClient,
	platformClients *platforms.PlatformClients,
	logger *zap.Logger,
) (*CampaignService, error) {
	// Create a Kafka producer for the campaign events topic
	producer, err := kafka.NewProducer("campaign_events")
	if err != nil {
		return nil, err
	}

	return &CampaignService{
		db:              db,
		platformClients: platformClients,
		producer:        producer,
		logger:          logger.With(zap.String("component", "campaign_service")),
	}, nil
}

// GetCampaign retrieves a campaign by ID
func (s *CampaignService) GetCampaign(ctx context.Context, id uuid.UUID) (*models.Campaign, error) {
	query := `
		SELECT * FROM campaigns 
		WHERE id = $1
	`

	var campaign models.Campaign
	err := s.db.GetDB().GetContext(ctx, &campaign, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("campaign not found")
		}
		s.logger.Error("Failed to get campaign", zap.Error(err), zap.String("campaign_id", id.String()))
		return nil, err
	}

	return &campaign, nil
}

// CreateCampaign creates a new campaign
func (s *CampaignService) CreateCampaign(ctx context.Context, campaign *models.Campaign) error {
	// Generate a new UUID if not provided
	if campaign.ID == uuid.Nil {
		campaign.ID = uuid.New()
	}

	// Set timestamps
	now := time.Now()
	campaign.CreatedAt = now
	campaign.UpdatedAt = now

	// Execute the insert
	query := `
		INSERT INTO campaigns (
			id, user_id, name, platform, budget, start_date, end_date, 
			status, external_id, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
	`

	_, err := s.db.GetDB().ExecContext(ctx, query,
		campaign.ID,
		campaign.UserID,
		campaign.Name,
		campaign.Platform,
		campaign.Budget,
		campaign.StartDate,
		campaign.EndDate,
		campaign.Status,
		campaign.ExternalID,
		campaign.CreatedAt,
		campaign.UpdatedAt,
	)
	if err != nil {
		s.logger.Error("Failed to create campaign",
			zap.Error(err),
			zap.String("campaign_id", campaign.ID.String()),
		)
		return err
	}

	s.logger.Info("Campaign created successfully", zap.String("campaign_id", campaign.ID.String()))
	return nil
}

// UpdateCampaign updates an existing campaign
func (s *CampaignService) UpdateCampaign(ctx context.Context, campaign *models.Campaign) error {
	// Update the timestamp
	campaign.UpdatedAt = time.Now()

	// Execute the update
	query := `
		UPDATE campaigns SET
			name = $1,
			platform = $2,
			budget = $3,
			start_date = $4,
			end_date = $5,
			status = $6,
			external_id = $7,
			updated_at = $8
		WHERE id = $9
	`

	result, err := s.db.GetDB().ExecContext(ctx, query,
		campaign.Name,
		campaign.Platform,
		campaign.Budget,
		campaign.StartDate,
		campaign.EndDate,
		campaign.Status,
		campaign.ExternalID,
		campaign.UpdatedAt,
		campaign.ID,
	)
	if err != nil {
		s.logger.Error("Failed to update campaign",
			zap.Error(err),
			zap.String("campaign_id", campaign.ID.String()),
		)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("campaign not found")
	}

	s.logger.Info("Campaign updated successfully", zap.String("campaign_id", campaign.ID.String()))
	return nil
}

// ListCampaigns lists campaigns with optional filters
func (s *CampaignService) ListCampaigns(ctx context.Context, userID uuid.UUID, platform *models.Platform, status *string) ([]models.Campaign, error) {
	// Build the query with filters
	query := "SELECT * FROM campaigns WHERE user_id = $1"
	args := []interface{}{userID}

	if platform != nil {
		query += " AND platform = $" + string(len(args)+1)
		args = append(args, string(*platform))
	}

	if status != nil {
		query += " AND status = $" + string(len(args)+1)
		args = append(args, *status)
	}

	query += " ORDER BY created_at DESC"

	// Execute the query
	var campaigns []models.Campaign
	err := s.db.GetDB().SelectContext(ctx, &campaigns, query, args...)
	if err != nil {
		s.logger.Error("Failed to list campaigns", zap.Error(err), zap.String("user_id", userID.String()))
		return nil, err
	}

	return campaigns, nil
}

// FetchCampaignData fetches the latest campaign data from the external platform
func (s *CampaignService) FetchCampaignData(ctx context.Context, campaignID uuid.UUID) error {
	// Get the campaign
	campaign, err := s.GetCampaign(ctx, campaignID)
	if err != nil {
		return err
	}

	// Get the platform client
	client, err := s.platformClients.GetClient(campaign.Platform)
	if err != nil {
		s.logger.Error("Platform client not found",
			zap.Error(err),
			zap.String("platform", string(campaign.Platform)),
		)
		return err
	}

	// Calculate time range (last 30 days by default)
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -30)
	if campaign.StartDate.After(startTime) {
		startTime = campaign.StartDate
	}

	// Fetch data from the platform
	events, err := client.FetchData(ctx, campaign.ExternalID, startTime, endTime)
	if err != nil {
		s.logger.Error("Failed to fetch campaign data",
			zap.Error(err),
			zap.String("campaign_id", campaignID.String()),
			zap.String("platform", string(campaign.Platform)),
		)
		return err
	}

	// Publish events to Kafka for processing
	for _, event := range events {
		// Ensure the campaign ID is set correctly
		event.CampaignID = campaignID

		// Send the event to Kafka
		err := s.producer.SendMessage(ctx, campaignID.String(), event)
		if err != nil {
			s.logger.Error("Failed to publish event to Kafka",
				zap.Error(err),
				zap.String("campaign_id", campaignID.String()),
				zap.String("event_id", event.ID.String()),
			)
			// Continue processing other events even if one fails
		}
	}

	s.logger.Info("Campaign data fetched and published",
		zap.String("campaign_id", campaignID.String()),
		zap.String("platform", string(campaign.Platform)),
		zap.Int("event_count", len(events)),
	)

	return nil
}

// Close closes the campaign service and releases resources
func (s *CampaignService) Close() error {
	return s.producer.Close()
}

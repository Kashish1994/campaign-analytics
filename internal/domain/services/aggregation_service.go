package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/zocket/campaign-analytics/internal/domain/models"
	"github.com/zocket/campaign-analytics/internal/infrastructure/database"
	"github.com/zocket/campaign-analytics/internal/infrastructure/redis"
	"go.uber.org/zap"
)

// AggregationService handles metric aggregation
type AggregationService struct {
	db     *database.ClickHouseClient
	redis  *redis.Client
	logger *zap.Logger
}

// NewAggregationService creates a new aggregation service
func NewAggregationService(
	db *database.ClickHouseClient,
	redis *redis.Client,
	logger *zap.Logger,
) *AggregationService {
	return &AggregationService{
		db:     db,
		redis:  redis,
		logger: logger.With(zap.String("component", "aggregation_service")),
	}
}

// GetCampaignInsights retrieves campaign insights based on filters
func (s *AggregationService) GetCampaignInsights(ctx context.Context, params models.CampaignInsightsParams) ([]models.CampaignInsights, error) {
	// Try to get from cache first
	cacheKey := s.getCacheKey(params)
	var insights []models.CampaignInsights

	err := s.redis.GetObject(ctx, cacheKey, &insights)
	if err == nil && len(insights) > 0 {
		s.logger.Debug("Retrieved campaign insights from cache", zap.String("cache_key", cacheKey))
		return insights, nil
	}

	// Cache miss, query from database
	s.logger.Debug("Cache miss, querying from database", zap.String("cache_key", cacheKey))

	// Build the query based on parameters
	query, args := s.buildInsightsQuery(params)

	// Execute the query
	conn := s.db.GetConn()
	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		s.logger.Error("Failed to query insights", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	// Parse the results
	insights = []models.CampaignInsights{}
	for rows.Next() {
		var insight models.CampaignInsights
		var campaignIDStr, platformStr string

		err := rows.Scan(
			&campaignIDStr,
			&insight.Date,
			&platformStr,
			&insight.Region,
			&insight.Impressions,
			&insight.Clicks,
			&insight.Conversions,
			&insight.Spend,
			&insight.Revenue,
			&insight.CTR,
			&insight.CPC,
			&insight.CPA,
			&insight.ROAS,
			&insight.ConversionRate,
			&insight.UpdatedAt,
		)
		if err != nil {
			s.logger.Error("Failed to scan insight row", zap.Error(err))
			return nil, err
		}

		// Convert string IDs to UUID
		campaignID, err := uuid.Parse(campaignIDStr)
		if err != nil {
			s.logger.Error("Failed to parse campaign ID", zap.Error(err), zap.String("campaign_id", campaignIDStr))
			return nil, err
		}
		insight.CampaignID = campaignID
		insight.Platform = models.Platform(platformStr)

		insights = append(insights, insight)
	}

	if err := rows.Err(); err != nil {
		s.logger.Error("Error iterating over insight rows", zap.Error(err))
		return nil, err
	}

	// Cache the results (only if there are results to cache)
	if len(insights) > 0 {
		// Cache for 5 minutes
		cacheExpiration := 5 * time.Minute
		if err := s.redis.Set(ctx, cacheKey, insights, cacheExpiration); err != nil {
			s.logger.Warn("Failed to cache insights", zap.Error(err), zap.String("cache_key", cacheKey))
			// Continue even if caching fails
		}
	}

	return insights, nil
}

// buildInsightsQuery builds the SQL query for retrieving insights
func (s *AggregationService) buildInsightsQuery(params models.CampaignInsightsParams) (string, []interface{}) {
	// Start with the base query
	query := `
		SELECT
			campaign_id,
			date,
			platform,
			region,
			impressions,
			clicks,
			conversions,
			spend,
			revenue,
			ctr,
			cpc,
			cpa,
			roas,
			conversion_rate,
			updated_at
		FROM campaign_insights
		WHERE 1=1
	`

	var args []interface{}

	// Add filters
	if params.CampaignID != uuid.Nil {
		query += " AND campaign_id = ?"
		args = append(args, params.CampaignID.String())
	}

	if !params.StartDate.IsZero() {
		query += " AND date >= ?"
		args = append(args, params.StartDate)
	}

	if !params.EndDate.IsZero() {
		query += " AND date <= ?"
		args = append(args, params.EndDate)
	}

	if params.Platform != nil {
		query += " AND platform = ?"
		args = append(args, string(*params.Platform))
	}

	if params.Region != nil && *params.Region != "" {
		query += " AND region = ?"
		args = append(args, *params.Region)
	}

	// Add ordering
	query += " ORDER BY date ASC"

	return query, args
}

// getCacheKey generates a cache key for the insights query
func (s *AggregationService) getCacheKey(params models.CampaignInsightsParams) string {
	// Build a cache key based on the query parameters
	cacheKey := fmt.Sprintf("insights:%s:", params.CampaignID.String())

	if !params.StartDate.IsZero() {
		cacheKey += fmt.Sprintf("start:%s:", params.StartDate.Format("2006-01-02"))
	}

	if !params.EndDate.IsZero() {
		cacheKey += fmt.Sprintf("end:%s:", params.EndDate.Format("2006-01-02"))
	}

	if params.Platform != nil {
		cacheKey += fmt.Sprintf("platform:%s:", *params.Platform)
	}

	if params.Region != nil {
		cacheKey += fmt.Sprintf("region:%s:", *params.Region)
	}

	if params.Granularity != "" {
		cacheKey += fmt.Sprintf("granularity:%s:", params.Granularity)
	}

	return cacheKey
}

// TriggerReaggregation triggers re-aggregation of metrics for a campaign
func (s *AggregationService) TriggerReaggregation(ctx context.Context, campaignID uuid.UUID, startDate, endDate time.Time) error {
	// Execute a query to re-aggregate the metrics
	query := `
		INSERT INTO campaign_insights
		SELECT
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
		WHERE campaign_id = ? AND event_time >= ? AND event_time <= ?
		GROUP BY campaign_id, toDate(event_time), platform, region
	`

	conn := s.db.GetConn()
	err := conn.Exec(ctx, query, campaignID.String(), startDate, endDate)
	if err != nil {
		s.logger.Error("Failed to re-aggregate insights",
			zap.Error(err),
			zap.String("campaign_id", campaignID.String()),
			zap.Time("start_date", startDate),
			zap.Time("end_date", endDate),
		)
		return err
	}

	s.logger.Info("Successfully re-aggregated insights",
		zap.String("campaign_id", campaignID.String()),
		zap.Time("start_date", startDate),
		zap.Time("end_date", endDate),
	)

	// Invalidate cache
	cachePattern := fmt.Sprintf("insights:%s:*", campaignID.String())
	keys, err := s.redis.GetClient().Keys(ctx, cachePattern).Result()
	if err != nil {
		s.logger.Warn("Failed to get cache keys for invalidation", zap.Error(err), zap.String("pattern", cachePattern))
		// Continue even if cache invalidation fails
	} else {
		if len(keys) > 0 {
			if err := s.redis.GetClient().Del(ctx, keys...).Err(); err != nil {
				s.logger.Warn("Failed to invalidate cache", zap.Error(err), zap.Strings("keys", keys))
			} else {
				s.logger.Debug("Invalidated cache", zap.Int("key_count", len(keys)))
			}
		}
	}

	return nil
}

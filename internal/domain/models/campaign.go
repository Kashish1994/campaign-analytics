package models

import (
	"time"

	"github.com/google/uuid"
)

// Platform represents an advertising platform (Meta, Google, etc.)
type Platform string

const (
	PlatformMeta     Platform = "meta"
	PlatformGoogle   Platform = "google"
	PlatformLinkedIn Platform = "linkedin"
	PlatformTikTok   Platform = "tiktok"
)

// Campaign represents a marketing campaign
type Campaign struct {
	ID          uuid.UUID `json:"id" db:"id"`
	UserID      uuid.UUID `json:"user_id" db:"user_id"`
	Name        string    `json:"name" db:"name"`
	Platform    Platform  `json:"platform" db:"platform"`
	Budget      float64   `json:"budget" db:"budget"`
	StartDate   time.Time `json:"start_date" db:"start_date"`
	EndDate     time.Time `json:"end_date" db:"end_date"`
	Status      string    `json:"status" db:"status"`
	ExternalID  string    `json:"external_id" db:"external_id"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// CampaignEvent represents raw event data received from ad platforms
type CampaignEvent struct {
	ID            uuid.UUID `json:"id" db:"id"`
	CampaignID    uuid.UUID `json:"campaign_id" db:"campaign_id"`
	Platform      Platform  `json:"platform" db:"platform"`
	EventType     string    `json:"event_type" db:"event_type"`
	Impressions   int64     `json:"impressions" db:"impressions"`
	Clicks        int64     `json:"clicks" db:"clicks"`
	Conversions   int64     `json:"conversions" db:"conversions"`
	Spend         float64   `json:"spend" db:"spend"`
	Revenue       float64   `json:"revenue" db:"revenue"`
	EventTime     time.Time `json:"event_time" db:"event_time"`
	Region        string    `json:"region" db:"region"`
	Currency      string    `json:"currency" db:"currency"`
	DeduplicationKey string `json:"deduplication_key" db:"deduplication_key"`
	ReceivedAt    time.Time `json:"received_at" db:"received_at"`
	ProcessedAt   time.Time `json:"processed_at" db:"processed_at"`
}

// CampaignInsights represents aggregated campaign metrics
type CampaignInsights struct {
	CampaignID    uuid.UUID `json:"campaign_id" db:"campaign_id"`
	Date          time.Time `json:"date" db:"date"`
	Platform      Platform  `json:"platform" db:"platform"`
	Region        string    `json:"region" db:"region"`
	Impressions   int64     `json:"impressions" db:"impressions"`
	Clicks        int64     `json:"clicks" db:"clicks"`
	Conversions   int64     `json:"conversions" db:"conversions"`
	Spend         float64   `json:"spend" db:"spend"`
	Revenue       float64   `json:"revenue" db:"revenue"`
	CTR           float64   `json:"ctr" db:"ctr"`                     // Click-through rate (clicks / impressions)
	CPC           float64   `json:"cpc" db:"cpc"`                     // Cost per click (spend / clicks)
	CPA           float64   `json:"cpa" db:"cpa"`                     // Cost per acquisition (spend / conversions)
	ROAS          float64   `json:"roas" db:"roas"`                   // Return on ad spend (revenue / spend)
	ConversionRate float64  `json:"conversion_rate" db:"conversion_rate"` // Conversion rate (conversions / clicks)
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// CampaignInsightsParams represents parameters for querying campaign insights
type CampaignInsightsParams struct {
	CampaignID  uuid.UUID  `json:"campaign_id" form:"campaign_id"`
	StartDate   time.Time  `json:"start_date" form:"start_date"`
	EndDate     time.Time  `json:"end_date" form:"end_date"`
	Platform    *Platform  `json:"platform" form:"platform"`
	Region      *string    `json:"region" form:"region"`
	Granularity string     `json:"granularity" form:"granularity"` // daily, weekly, monthly
}

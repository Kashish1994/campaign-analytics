package platforms

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/zocket/campaign-analytics/internal/domain/models"
)

// MetaClient implements the PlatformClient interface for Meta (Facebook) ads
type MetaClient struct {
	apiURL     string
	httpClient *http.Client
}

// NewMetaClient creates a new Meta client
func NewMetaClient() *MetaClient {
	return &MetaClient{
		apiURL: "https://graph.facebook.com/v16.0",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetName returns the platform name
func (c *MetaClient) GetName() models.Platform {
	return models.PlatformMeta
}

// MetaInsightsResponse represents the response from the Meta Insights API
type MetaInsightsResponse struct {
	Data []struct {
		DateStart   string  `json:"date_start"`
		DateStop    string  `json:"date_stop"`
		Impressions int64   `json:"impressions"`
		Clicks      int64   `json:"clicks"`
		Conversions int64   `json:"actions"`
		Spend       float64 `json:"spend"`
		Revenue     float64 `json:"action_values"`
	} `json:"data"`
	Paging struct {
		Cursors struct {
			Before string `json:"before"`
			After  string `json:"after"`
		} `json:"cursors"`
		Next string `json:"next,omitempty"`
	} `json:"paging"`
}

// FetchData fetches ad performance data from Meta
func (c *MetaClient) FetchData(ctx context.Context, campaignID string, startTime, endTime time.Time) ([]models.CampaignEvent, error) {
	// Format the time range for the API request
	startDate := startTime.Format("2006-01-02")
	endDate := endTime.Format("2006-01-02")

	// Build the API URL
	url := fmt.Sprintf(
		"%s/act_%s/insights?time_range={'since':'%s','until':'%s'}&level=campaign&fields=campaign_id,impressions,clicks,actions,spend,action_values",
		c.apiURL, campaignID, startDate, endDate,
	)

	// Create a new request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add authentication headers (in a real implementation, you would get these from a secret store)
	req.Header.Add("Authorization", "Bearer YOUR_ACCESS_TOKEN")

	// Execute the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("meta API returned non-200 status: %d", resp.StatusCode)
	}

	// Parse the response
	var insightsResp MetaInsightsResponse
	if err := json.NewDecoder(resp.Body).Decode(&insightsResp); err != nil {
		return nil, err
	}

	// Convert the response to CampaignEvent objects
	var events []models.CampaignEvent
	for _, item := range insightsResp.Data {
		eventTime, err := time.Parse("2006-01-02", item.DateStart)
		if err != nil {
			return nil, err
		}

		campaignUUID, err := uuid.Parse(campaignID)
		if err != nil {
			return nil, err
		}

		// Create a deduplication key to prevent duplicate processing
		dedupKey := fmt.Sprintf("meta:%s:%s", campaignID, item.DateStart)

		events = append(events, models.CampaignEvent{
			ID:              uuid.New(),
			CampaignID:      campaignUUID,
			Platform:        models.PlatformMeta,
			EventType:       "daily_stats",
			Impressions:     item.Impressions,
			Clicks:          item.Clicks,
			Conversions:     item.Conversions,
			Spend:           item.Spend,
			Revenue:         item.Revenue,
			EventTime:       eventTime,
			Region:          "all", // Meta doesn't provide region in this API call
			Currency:        "USD", // Assuming USD, would be configurable in a real system
			DeduplicationKey: dedupKey,
			ReceivedAt:      time.Now(),
		})
	}

	return events, nil
}

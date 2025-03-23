package platforms

import (
	"context"
	"errors"
	"time"

	"github.com/zocket/campaign-analytics/internal/domain/models"
)

// PlatformClient defines the interface for ad platform clients
type PlatformClient interface {
	// FetchData fetches ad performance data for a campaign
	FetchData(ctx context.Context, campaignID string, startTime, endTime time.Time) ([]models.CampaignEvent, error)
	
	// GetName returns the platform name
	GetName() models.Platform
}

// PlatformClients holds all platform clients
type PlatformClients struct {
	clients map[models.Platform]PlatformClient
}

// NewPlatformClients creates a new instance of platform clients
func NewPlatformClients() *PlatformClients {
	return &PlatformClients{
		clients: map[models.Platform]PlatformClient{
			models.PlatformMeta:     NewMetaClient(),
			models.PlatformGoogle:   NewGoogleClient(),
			models.PlatformLinkedIn: NewLinkedInClient(),
			models.PlatformTikTok:   NewTikTokClient(),
		},
	}
}

// GetClient returns a platform client by name
func (pc *PlatformClients) GetClient(platform models.Platform) (PlatformClient, error) {
	client, exists := pc.clients[platform]
	if !exists {
		return nil, errors.New("platform client not found")
	}
	return client, nil
}

// GetAllClients returns all platform clients
func (pc *PlatformClients) GetAllClients() map[models.Platform]PlatformClient {
	return pc.clients
}

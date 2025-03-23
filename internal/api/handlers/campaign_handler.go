package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zocket/campaign-analytics/internal/domain/models"
	"github.com/zocket/campaign-analytics/internal/domain/services"
	"go.uber.org/zap"
)

// CampaignHandler handles HTTP requests related to campaigns
type CampaignHandler struct {
	campaignService    *services.CampaignService
	aggregationService *services.AggregationService
	logger             *zap.Logger
}

// NewCampaignHandler creates a new campaign handler
func NewCampaignHandler(
	campaignService *services.CampaignService,
	aggregationService *services.AggregationService,
	logger *zap.Logger,
) *CampaignHandler {
	return &CampaignHandler{
		campaignService:    campaignService,
		aggregationService: aggregationService,
		logger:             logger.With(zap.String("component", "campaign_handler")),
	}
}

// GetCampaign handles GET /campaigns/:id
func (h *CampaignHandler) GetCampaign(c *gin.Context) {
	campaignID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign ID"})
		return
	}

	campaign, err := h.campaignService.GetCampaign(c.Request.Context(), campaignID)
	if err != nil {
		h.logger.Error("Failed to get campaign", zap.Error(err), zap.String("campaign_id", campaignID.String()))
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}

	c.JSON(http.StatusOK, campaign)
}

// CreateCampaign handles POST /campaigns
func (h *CampaignHandler) CreateCampaign(c *gin.Context) {
	var campaign models.Campaign
	if err := c.ShouldBindJSON(&campaign); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set the user ID from the authenticated user
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	campaign.UserID = userID.(uuid.UUID)

	if err := h.campaignService.CreateCampaign(c.Request.Context(), &campaign); err != nil {
		h.logger.Error("Failed to create campaign", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create campaign"})
		return
	}

	c.JSON(http.StatusCreated, campaign)
}

// UpdateCampaign handles PUT /campaigns/:id
func (h *CampaignHandler) UpdateCampaign(c *gin.Context) {
	campaignID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign ID"})
		return
	}

	var campaign models.Campaign
	if err := c.ShouldBindJSON(&campaign); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Ensure the ID in the URL matches the ID in the body
	campaign.ID = campaignID

	// Verify the user owns this campaign
	userID, _ := c.Get("user_id")
	existingCampaign, err := h.campaignService.GetCampaign(c.Request.Context(), campaignID)
	if err != nil {
		h.logger.Error("Failed to get campaign for update", zap.Error(err), zap.String("campaign_id", campaignID.String()))
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}

	if existingCampaign.UserID != userID.(uuid.UUID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to update this campaign"})
		return
	}

	if err := h.campaignService.UpdateCampaign(c.Request.Context(), &campaign); err != nil {
		h.logger.Error("Failed to update campaign", zap.Error(err), zap.String("campaign_id", campaignID.String()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update campaign"})
		return
	}

	c.JSON(http.StatusOK, campaign)
}

// ListCampaigns handles GET /campaigns
func (h *CampaignHandler) ListCampaigns(c *gin.Context) {
	// Get the user ID from the authenticated user
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Parse optional query parameters
	var platform *models.Platform
	platformStr := c.Query("platform")
	if platformStr != "" {
		p := models.Platform(platformStr)
		platform = &p
	}

	var status *string
	statusStr := c.Query("status")
	if statusStr != "" {
		status = &statusStr
	}

	campaigns, err := h.campaignService.ListCampaigns(c.Request.Context(), userID.(uuid.UUID), platform, status)
	if err != nil {
		h.logger.Error("Failed to list campaigns", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list campaigns"})
		return
	}

	c.JSON(http.StatusOK, campaigns)
}

// FetchCampaignData handles POST /campaigns/:id/fetch-data
func (h *CampaignHandler) FetchCampaignData(c *gin.Context) {
	campaignID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign ID"})
		return
	}

	// Verify the user owns this campaign
	userID, _ := c.Get("user_id")
	existingCampaign, err := h.campaignService.GetCampaign(c.Request.Context(), campaignID)
	if err != nil {
		h.logger.Error("Failed to get campaign for fetch", zap.Error(err), zap.String("campaign_id", campaignID.String()))
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}

	if existingCampaign.UserID != userID.(uuid.UUID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to fetch data for this campaign"})
		return
	}

	// Start data fetching in a goroutine to avoid blocking the API
	go func() {
		ctx := c.Request.Context()
		if err := h.campaignService.FetchCampaignData(ctx, campaignID); err != nil {
			h.logger.Error("Failed to fetch campaign data", zap.Error(err), zap.String("campaign_id", campaignID.String()))
		}
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Data fetch initiated"})
}

// GetCampaignInsights handles GET /campaigns/:id/insights
func (h *CampaignHandler) GetCampaignInsights(c *gin.Context) {
	campaignID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign ID"})
		return
	}

	// Verify the user has access to this campaign
	userID, _ := c.Get("user_id")
	existingCampaign, err := h.campaignService.GetCampaign(c.Request.Context(), campaignID)
	if err != nil {
		h.logger.Error("Failed to get campaign for insights", zap.Error(err), zap.String("campaign_id", campaignID.String()))
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}

	if existingCampaign.UserID != userID.(uuid.UUID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to view insights for this campaign"})
		return
	}

	// Parse query parameters
	var params models.CampaignInsightsParams
	params.CampaignID = campaignID

	// Parse start_date and end_date
	startDateStr := c.Query("start_date")
	if startDateStr != "" {
		startDate, err := time.Parse("2006-01-02", startDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format (use YYYY-MM-DD)"})
			return
		}
		params.StartDate = startDate
	} else {
		// Default to last 30 days
		params.StartDate = time.Now().AddDate(0, 0, -30)
	}

	endDateStr := c.Query("end_date")
	if endDateStr != "" {
		endDate, err := time.Parse("2006-01-02", endDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format (use YYYY-MM-DD)"})
			return
		}
		params.EndDate = endDate
	} else {
		// Default to today
		params.EndDate = time.Now()
	}

	// Parse platform filter
	platformStr := c.Query("platform")
	if platformStr != "" {
		platform := models.Platform(platformStr)
		params.Platform = &platform
	}

	// Parse region filter
	regionStr := c.Query("region")
	if regionStr != "" {
		params.Region = &regionStr
	}

	// Parse granularity
	granularity := c.Query("granularity")
	if granularity == "" {
		granularity = "daily" // Default granularity
	}
	params.Granularity = granularity

	// Get insights
	insights, err := h.aggregationService.GetCampaignInsights(c.Request.Context(), params)
	if err != nil {
		h.logger.Error("Failed to get campaign insights", 
			zap.Error(err), 
			zap.String("campaign_id", campaignID.String()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get campaign insights"})
		return
	}

	c.JSON(http.StatusOK, insights)
}

// TriggerInsightsReaggregation handles POST /campaigns/:id/reaggregate
func (h *CampaignHandler) TriggerInsightsReaggregation(c *gin.Context) {
	campaignID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign ID"})
		return
	}

	// Verify the user has access to this campaign
	userID, _ := c.Get("user_id")
	existingCampaign, err := h.campaignService.GetCampaign(c.Request.Context(), campaignID)
	if err != nil {
		h.logger.Error("Failed to get campaign for reaggregation", zap.Error(err), zap.String("campaign_id", campaignID.String()))
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}

	if existingCampaign.UserID != userID.(uuid.UUID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to reaggregate insights for this campaign"})
		return
	}

	// Parse date range
	var startDate, endDate time.Time
	startDateStr := c.Query("start_date")
	if startDateStr != "" {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format (use YYYY-MM-DD)"})
			return
		}
	} else {
		// Default to last 30 days
		startDate = time.Now().AddDate(0, 0, -30)
	}

	endDateStr := c.Query("end_date")
	if endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format (use YYYY-MM-DD)"})
			return
		}
	} else {
		// Default to today
		endDate = time.Now()
	}

	// Trigger reaggregation in a goroutine
	go func() {
		ctx := c.Request.Context()
		if err := h.aggregationService.TriggerReaggregation(ctx, campaignID, startDate, endDate); err != nil {
			h.logger.Error("Failed to reaggregate insights", 
				zap.Error(err), 
				zap.String("campaign_id", campaignID.String()),
			)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Reaggregation initiated"})
}

package campaigns

import (
	"context"
	"time"

	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
)

type Service struct{ repo *Repository }

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

var (
	validFormat  = map[string]bool{"banner": true, "popunder": true, "native": true, "push": true}
	validPricing = map[string]bool{"cpm": true, "cpc": true}
	validTraffic = map[string]bool{"mainstream": true, "adult": true, "mixed": true}
	validStatus  = map[string]bool{"active": true, "paused": true, "draft": true, "completed": true, "moderation": true}
)

func (s *Service) List(ctx context.Context, userID string) ([]models.Campaign, error) {
	return s.repo.List(ctx, userID)
}
func (s *Service) Get(ctx context.Context, userID, campaignID string) (models.Campaign, error) {
	return s.repo.Get(ctx, userID, campaignID)
}
func (s *Service) GetFormat(ctx context.Context, userID, campaignID string) (string, error) {
	return s.repo.GetFormat(ctx, userID, campaignID)
}

func (s *Service) Create(ctx context.Context, userID string, req UpsertCampaignRequest) (models.Campaign, error) {
	status := valueOr(req.Status, "draft")
	c := models.Campaign{
		UserID:             userID,
		CampaignName:       req.CampaignName,
		QualityType:        req.QualityType,
		FormatType:         req.FormatType,
		BrandName:          req.BrandName,
		H:                  req.H,
		W:                  req.W,
		Status:             status,
		TrafficType:        req.TrafficType,
		Vertical:           nonNilSlice(req.Vertical),
		PricingModel:       req.PricingModel,
		BasePriceCPM:       req.BasePriceCPM,
		BasePriceCPC:       req.BasePriceCPC,
		EvennessBySlotMode: req.EvennessBySlotMode,
		GoalTotalDollars:   req.GoalTotalDollars,
		CumDoneDollars:     0,
		StartTS:            req.StartTS,
		EndTS:              req.EndTS,
		ActiveIntervals:    nonNilIntervals(req.ActiveIntervals),
		Country:            nonNilMap(req.Country),
		Language:           nonNilMap(req.Language),
		DeviceType:         nonNilMap(req.DeviceType),
		OS:                 nonNilMap(req.OS),
		Browser:            nonNilMap(req.Browser),
		SiteID:             nonNilMap(req.SiteID),
		IP:                 nonNilMap(req.IP),
	}
	if c.StartTS.IsZero() {
		c.StartTS = time.Now()
	}
	if c.EndTS.IsZero() {
		c.EndTS = c.StartTS.AddDate(0, 1, 0)
	}
	if err := validateCampaign(c); err != nil {
		return models.Campaign{}, err
	}
	return s.repo.Create(ctx, c)
}

func (s *Service) Patch(ctx context.Context, userID, campaignID string, req PatchCampaignRequest) (models.Campaign, error) {
	current, err := s.repo.Get(ctx, userID, campaignID)
	if err != nil {
		return models.Campaign{}, err
	}
	if req.CampaignName != nil {
		current.CampaignName = *req.CampaignName
	}
	if req.FormatType != nil {
		current.FormatType = *req.FormatType
	}
	if req.QualityType != nil {
		current.QualityType = *req.QualityType
	}
	if req.BrandNameSet {
		current.BrandName = req.BrandName
	}
	if req.HSet {
		current.H = req.H
	}
	if req.WSet {
		current.W = req.W
	}
	if req.Status != nil {
		current.Status = *req.Status
	}
	if req.TrafficType != nil {
		current.TrafficType = *req.TrafficType
	}
	if req.Vertical != nil {
		current.Vertical = *req.Vertical
	}
	if req.PricingModel != nil {
		current.PricingModel = *req.PricingModel
	}
	if req.BasePriceCPM != nil {
		current.BasePriceCPM = *req.BasePriceCPM
	}
	if req.BasePriceCPC != nil {
		current.BasePriceCPC = *req.BasePriceCPC
	}
	if req.EvennessBySlotMode != nil {
		current.EvennessBySlotMode = *req.EvennessBySlotMode
	}
	if req.GoalTotalDollars != nil {
		current.GoalTotalDollars = *req.GoalTotalDollars
	}
	if req.CumDoneDollars != nil {
		current.CumDoneDollars = *req.CumDoneDollars
	}
	if req.StartTS != nil {
		current.StartTS = *req.StartTS
	}
	if req.EndTS != nil {
		current.EndTS = *req.EndTS
	}
	if req.ActiveIntervals != nil {
		current.ActiveIntervals = *req.ActiveIntervals
	}
	if req.Country != nil {
		current.Country = *req.Country
	}
	if req.Language != nil {
		current.Language = *req.Language
	}
	if req.DeviceType != nil {
		current.DeviceType = *req.DeviceType
	}
	if req.OS != nil {
		current.OS = *req.OS
	}
	if req.Browser != nil {
		current.Browser = *req.Browser
	}
	if req.SiteID != nil {
		current.SiteID = *req.SiteID
	}
	if req.IP != nil {
		current.IP = *req.IP
	}
	if err := validateCampaign(current); err != nil {
		return models.Campaign{}, err
	}
	return s.repo.Update(ctx, current)
}

func (s *Service) Delete(ctx context.Context, userID, campaignID string) error {
	return s.repo.Delete(ctx, userID, campaignID)
}

func validateCampaign(c models.Campaign) error {
	if c.CampaignName == "" {
		return httpx.BadRequest("campaign_name is required")
	}
	if c.QualityType == "" {
		return httpx.BadRequest("quality_type is required")
	}
	if !validFormat[c.FormatType] {
		return httpx.BadRequest("invalid format_type")
	}
	if !validPricing[c.PricingModel] {
		return httpx.BadRequest("invalid pricing_model")
	}
	if !validTraffic[c.TrafficType] {
		return httpx.BadRequest("invalid traffic_type")
	}
	if !validStatus[c.Status] {
		return httpx.BadRequest("invalid status")
	}
	if c.EndTS.Before(c.StartTS) {
		return httpx.BadRequest("end_ts must be after start_ts")
	}
	return nil
}

func valueOr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
func nonNilMap(v models.TargetingMap) models.TargetingMap {
	if v == nil {
		return models.TargetingMap{}
	}
	return v
}
func nonNilSlice(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}
func nonNilIntervals(v []models.ScheduleInterval) []models.ScheduleInterval {
	if v == nil {
		return []models.ScheduleInterval{}
	}
	return v
}

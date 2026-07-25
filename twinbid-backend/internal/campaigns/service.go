package campaigns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"twinbid-backend/internal/bot"
	"twinbid-backend/internal/config"
	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/mailer"
	"twinbid-backend/internal/models"
	"twinbid-backend/internal/notifications"
	"twinbid-backend/internal/profile"
	"twinbid-backend/internal/storage"
)

type Service struct {
	repo         *Repository
	creativeRepo interface {
		ListByCampaign(ctx context.Context, userID, campaignID string) ([]models.Creative, error)
		ListImagesByCampaign(ctx context.Context, userID, campaignID string) ([]models.CreativeImage, error)
		DeleteImageRecord(ctx context.Context, imageID string) error
	}
	profileRepo *profile.Repository
	notifySvc   *notifications.Service
	smtpCfg     config.SMTPConfig
	botCfg      config.BotConfig
	s3          *storage.S3Storage
}

func NewService(repo *Repository, creativeRepo interface {
	ListByCampaign(ctx context.Context, userID, campaignID string) ([]models.Creative, error)
	ListImagesByCampaign(ctx context.Context, userID, campaignID string) ([]models.CreativeImage, error)
	DeleteImageRecord(ctx context.Context, imageID string) error
}, profileRepo *profile.Repository, notifySvc *notifications.Service, smtpCfg config.SMTPConfig, botCfg config.BotConfig, s3 *storage.S3Storage) *Service {
	return &Service{
		repo:         repo,
		creativeRepo: creativeRepo,
		profileRepo:  profileRepo,
		notifySvc:    notifySvc,
		smtpCfg:      smtpCfg,
		botCfg:       botCfg,
		s3:           s3,
	}
}

var (
	validFormat  = map[string]bool{"banner": true, "popunder": true, "native": true, "push": true}
	validPricing = map[string]bool{"cpm": true, "cpc": true}
	validTraffic = map[string]bool{"mainstream": true, "adult": true, "mixed": true}
	validStatus  = map[string]bool{"active": true, "paused": true, "waiting": true, "draft": true, "completed": true, "moderation": true, "no_budget": true}
)

func (s *Service) List(ctx context.Context, userID string) ([]models.Campaign, error) {
	return s.repo.List(ctx, userID)
}
func (s *Service) Get(ctx context.Context, campaignID string) (models.Campaign, error) {
	return s.repo.Get(ctx, campaignID)
}
func (s *Service) GetFormat(ctx context.Context, campaignID string) (string, error) {
	return s.repo.GetFormat(ctx, campaignID)
}

func (s *Service) Create(ctx context.Context, userID string, req UpsertCampaignRequest) (models.Campaign, error) {
	status := valueOr(req.Status, "draft")
	c := models.Campaign{
		UserID:       userID,
		CampaignName: req.CampaignName,
		QualityType:  req.QualityType,
		TrafficResetVersion: func() int64 {
			if status == "active" {
				return 1
			}
			return 0
		}(),
		FormatType:         req.FormatType,
		BrandName:          req.BrandName,
		H:                  req.H,
		W:                  req.W,
		Status:             status,
		TrafficType:        req.TrafficType,
		Vertical:           req.Vertical,
		PricingModel:       req.PricingModel,
		BasePrice:          req.BasePrice,
		EvennessBySlotMode: req.EvennessBySlotMode,
		GoalTotalDollars:   req.GoalTotalDollars,
		CumDoneDollars:     0,
		StartTS:            req.StartTS,
		EndTS:              req.EndTS,
		ActiveIntervals:    nonNilIntervals(req.ActiveIntervals),
		Country:            models.NormalizeTargetingFilter(req.Country),
		Language:           models.NormalizeTargetingFilter(req.Language),
		DeviceType:         models.NormalizeTargetingFilter(req.DeviceType),
		OS:                 models.NormalizeTargetingFilter(req.OS),
		Browser:            models.NormalizeTargetingFilter(req.Browser),
		SiteID:             models.NormalizeTargetingFilter(req.SiteID),
		IP:                 models.NormalizeTargetingFilter(req.IP),
	}
	if c.StartTS.IsZero() {
		c.StartTS = time.Now().UTC()
	}
	if c.EndTS.IsZero() {
		c.EndTS = c.StartTS.AddDate(0, 1, 0)
	}

	c.StartTS = c.StartTS.UTC()
	c.EndTS = c.EndTS.UTC()

	if err := validateCampaign(c); err != nil {
		return models.Campaign{}, err
	}

	campaign, err := s.repo.Create(ctx, c)
	if err != nil {
		return models.Campaign{}, fmt.Errorf("create campaign: %w", err)
	}

	return campaign, nil
}

func (s *Service) Patch(ctx context.Context, campaignID string, req PatchCampaignRequest) (models.Campaign, error) {
	var oldStatus string
	campaign, err := s.repo.UpdateLocked(ctx, campaignID, func(current *models.Campaign) error {
		before := cloneCampaignForComparison(*current)
		oldStatus = before.Status
		applyPatchRequest(current, req)
		if err := validateCampaign(*current); err != nil {
			return err
		}
		if requiresTrafficReset(before, *current) {
			current.TrafficResetVersion++
		}
		return nil
	})
	if err != nil {
		return models.Campaign{}, fmt.Errorf("cannot patch campaign: %w", err)
	}

	if req.Status != nil && oldStatus != *req.Status {
		if err := s.notifyCampaignStatusChangeIfNeeded(ctx, campaign, oldStatus, *req.Status); err != nil {
			return models.Campaign{}, err
		}
	}

	///////
	if campaign.Status == "moderation" {
		fmt.Println("MODERATION CASE")
		user, err := s.profileRepo.Get(ctx, campaign.UserID)
		if err != nil {
			return models.Campaign{}, fmt.Errorf("get profile: %w", err)
		}
		userTelegram := ""
		if user.Telegram != nil {
			userTelegram = *user.Telegram
		}
		creativeItems, err := s.creativeRepo.ListByCampaign(ctx, campaign.UserID, campaign.CampaignID)
		if err != nil {
			return models.Campaign{}, fmt.Errorf("list creatives: %w", err)
		}
		creativesPayload := make([]bot.CreativePayload, 0, len(creativeItems))
		for _, cr := range creativeItems {
			macros := ""
			if len(cr.TrackersMacros) > 0 {
				b, _ := json.Marshal(cr.TrackersMacros)
				macros = string(b)
			}
			title := ""
			if cr.Title != nil {
				title = *cr.Title
			}
			description := ""
			if cr.Description != nil {
				description = *cr.Description
			}

			imageURL := ""
			if cr.ImageURL != nil {
				imageURL = *cr.ImageURL
			}

			creativesPayload = append(creativesPayload, bot.CreativePayload{
				CreativeName: cr.CreativeName,
				ADM:          cr.ADM,
				Macros:       macros,
				ImageURL:     imageURL,
				Title:        title,
				Description:  description,
			})
		}
		bannerSize := ""
		if campaign.W != nil && campaign.H != nil {
			bannerSize = fmt.Sprintf("%dx%d", *campaign.W, *campaign.H)
		}
		brandName := ""
		if campaign.BrandName != nil {
			brandName = *campaign.BrandName
		}

		botClient := bot.NewBotClient(s.botCfg.BaseURL, s.botCfg.InternalSecret)
		if err := botClient.SendCampaignModeration(ctx, bot.CampaignModerationRequest{
			CampaignID:   campaign.CampaignID,
			FormatType:   campaign.FormatType,
			TrafficType:  campaign.TrafficType,
			CampaignName: campaign.CampaignName,
			BannerSize:   bannerSize,
			BrandName:    brandName,
			UserID:       campaign.UserID,
			UserEmail:    user.Mail,
			UserTelegram: userTelegram,
			Creatives:    creativesPayload,
		}); err != nil {
			fmt.Println("GOT ERROR BOT")
			return models.Campaign{}, fmt.Errorf("send campaign moderation: %w", err)
		}
		fmt.Println("SUCCESS BOT")
	}

	return campaign, nil
}

func (s *Service) notifyCampaignStatusChangeIfNeeded(ctx context.Context, campaign models.Campaign, oldStatus, newStatus string) error {
	if !((oldStatus == "moderation" && newStatus == "waiting") ||
		(oldStatus == "waiting" && newStatus == "active") ||
		(oldStatus == "active" && newStatus == "no_budget") ||
		(oldStatus == "active" && newStatus == "completed")) {
		return nil
	}

	body := fmt.Sprintf(
		"Статус вашей кампании %s был изменен с %s на %s.",
		campaign.CampaignName,
		oldStatus,
		newStatus,
	)

	if _, err := s.notifySvc.Create(ctx, campaign.UserID, notifications.CreateNotificationRequest{
		CampaignID: &campaign.CampaignID,
		Text:       body,
		Type:       "campaign_status",
	}); err != nil {
		return fmt.Errorf("create campaign status notification: %w", err)
	}

	user, err := s.profileRepo.Get(ctx, campaign.UserID)
	if err != nil {
		return fmt.Errorf("get user notification settings: %w", err)
	}

	if !user.CampaignStatusNotifications || user.Mail == "" {
		return nil
	}

	if err := mailer.SendEmail(s.smtpCfg, user.Mail, "Изменение статуса кампании", body); err != nil {
		return fmt.Errorf("send campaign status email: %w", err)
	}

	return nil
}

func (s *Service) Delete(ctx context.Context, userID, campaignID string) error {
	images, err := s.creativeRepo.ListImagesByCampaign(ctx, userID, campaignID)
	if err != nil {
		return fmt.Errorf("list campaign images: %w", err)
	}
	if err := s.repo.Delete(ctx, userID, campaignID); err != nil {
		return err
	}

	var cleanupErrors []error
	for _, image := range images {
		if err := s.s3.Delete(ctx, image.S3Key); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("delete campaign image %s from s3: %w", image.ID, err))
			continue
		}
		if err := s.creativeRepo.DeleteImageRecord(ctx, image.ID); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("delete campaign image record %s: %w", image.ID, err))
		}
	}
	return errors.Join(cleanupErrors...)
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
func nonNilMap(v *models.TargetingFilter) *models.TargetingFilter {
	if v == nil {
		return &models.TargetingFilter{}
	}
	return v
}
func nonNilIntervals(v []models.ScheduleInterval) []models.ScheduleInterval {
	if v == nil {
		return []models.ScheduleInterval{}
	}
	return v
}

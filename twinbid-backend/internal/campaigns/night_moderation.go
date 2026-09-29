package campaigns

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
)

const (
	nightModerationStartHourUTC = 18
	nightModerationEndHourUTC   = 6
)

// NightModerationWindowStart returns the UTC start of the current automatic
// moderation window. The production window is 18:00 UTC through 06:00 UTC,
// which corresponds to 21:00-09:00 Moscow time (UTC+3).
func NightModerationWindowStart(now time.Time) (time.Time, bool) {
	now = now.UTC()
	year, month, day := now.Date()
	startToday := time.Date(year, month, day, nightModerationStartHourUTC, 0, 0, 0, time.UTC)

	if now.Hour() >= nightModerationStartHourUTC {
		return startToday, true
	}
	if now.Hour() < nightModerationEndHourUTC {
		return startToday.AddDate(0, 0, -1), true
	}
	return time.Time{}, false
}

// NextNightModerationWindowStart returns the next 18:00 UTC boundary strictly
// after now. It is used by the background worker so the existing moderation
// queue is auto-approved once when a new night window begins.
func NextNightModerationWindowStart(now time.Time) time.Time {
	now = now.UTC()
	year, month, day := now.Date()
	startToday := time.Date(year, month, day, nightModerationStartHourUTC, 0, 0, 0, time.UTC)
	if now.Before(startToday) {
		return startToday
	}
	return startToday.AddDate(0, 0, 1)
}

// AutoApprovePendingNightModeration is used on startup during an active night
// window and at every new 18:00 UTC boundary. It rotates the persistent list,
// then moves every campaign that is still in moderation to waiting while
// preserving its campaign ID for the later Telegram decision.
func (s *Service) AutoApprovePendingNightModeration(ctx context.Context, now time.Time) (int, error) {
	windowStart, active := NightModerationWindowStart(now)
	if !active {
		return 0, nil
	}

	if err := s.repo.RotateNightModerationWindow(ctx, windowStart); err != nil {
		return 0, fmt.Errorf("rotate night moderation window: %w", err)
	}

	campaignIDs, err := s.repo.ListModerationCampaignIDs(ctx)
	if err != nil {
		return 0, fmt.Errorf("list campaigns pending night moderation: %w", err)
	}

	approved := 0
	var errs []error
	for _, campaignID := range campaignIDs {
		campaign, changed, err := s.repo.AutoApproveNightCampaign(ctx, campaignID, windowStart)
		if err != nil {
			errs = append(errs, fmt.Errorf("campaign_id=%s: %w", campaignID, err))
			continue
		}
		if !changed {
			continue
		}
		approved++
		if err := s.notifyCampaignStatusChangeIfNeeded(ctx, campaign, "moderation", "waiting"); err != nil {
			errs = append(errs, fmt.Errorf("campaign_id=%s notification: %w", campaignID, err))
		}
	}
	return approved, errors.Join(errs...)
}

// autoApproveNightCampaignIfNeeded performs the hidden automatic approval for
// a campaign whose moderation message has already been delivered to Telegram.
// The Telegram keyboard is intentionally untouched: the bot can make one final
// manual decision later in the day.
func (s *Service) autoApproveNightCampaignIfNeeded(ctx context.Context, campaign models.Campaign, now time.Time) (models.Campaign, error) {
	windowStart, active := NightModerationWindowStart(now)
	if !active || strings.ToLower(strings.TrimSpace(campaign.Status)) != "moderation" {
		return campaign, nil
	}

	updated, changed, err := s.repo.AutoApproveNightCampaign(ctx, campaign.CampaignID, windowStart)
	if err != nil {
		return models.Campaign{}, fmt.Errorf("night auto-approve campaign: %w", err)
	}
	if changed {
		if err := s.notifyCampaignStatusChangeIfNeeded(ctx, updated, "moderation", "waiting"); err != nil {
			return models.Campaign{}, err
		}
	}
	return updated, nil
}

func applyNightModerationDecision(current *models.Campaign, state *NightModerationState, decision string, now time.Time) error {
	decision = strings.ToLower(strings.TrimSpace(decision))
	if decision != moderationDecisionApprove && decision != moderationDecisionReject {
		return httpx.BadRequest("decision must be approve or reject")
	}
	if state == nil {
		return fmt.Errorf("night moderation state is nil")
	}

	if state.ManualDecision != "" {
		if state.ManualDecision == decision {
			// Telegram may retry a callback. The same decision is idempotent.
			return nil
		}
		return httpx.Conflict("Решение по ночной кампании уже принято")
	}

	if decision == moderationDecisionReject {
		current.Status = "draft"
	}
	// Approve is deliberately state-preserving: the campaign was already
	// auto-approved and may now be waiting, active, no_budget, completed, etc.
	state.ManualDecision = decision
	decidedAt := now.UTC()
	state.ManualDecidedAt = &decidedAt
	return nil
}

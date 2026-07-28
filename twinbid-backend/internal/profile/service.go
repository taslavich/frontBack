package profile

import (
	"context"
	"database/sql"

	"twinbid-backend/internal/models"
)

type Service struct{ repo *Repository }

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

func (s *Service) Get(ctx context.Context, userID string) (models.User, error) {
	return s.repo.Get(ctx, userID)
}

func (s *Service) Patch(ctx context.Context, userID string, patch PatchProfileRequest) (models.User, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return models.User{}, err
	}
	defer tx.Rollback()

	u, err := s.patch(ctx, tx, userID, patch)
	if err != nil {
		return models.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.User{}, err
	}
	return u, nil
}

func (s *Service) PatchTx(ctx context.Context, tx *sql.Tx, userID string, patch PatchProfileRequest) (models.User, error) {
	return s.patch(ctx, tx, userID, patch)
}

func (s *Service) patch(ctx context.Context, tx *sql.Tx, userID string, patch PatchProfileRequest) (models.User, error) {
	u, err := s.repo.GetTx(ctx, tx, userID)
	if err != nil {
		return models.User{}, err
	}

	if patch.Login != nil {
		u.Login = *patch.Login
	}
	if patch.Mail != nil {
		u.Mail = *patch.Mail
	}
	if patch.Name != nil {
		u.Name = *patch.Name
	}
	if patch.TelegramSet {
		u.Telegram = patch.Telegram
	}
	if patch.ManagerTelegram != nil {
		u.ManagerTelegram = *patch.ManagerTelegram
	}
	if patch.BalanceDelta != nil {
		// Backward-compatible API semantics: "balance" is an additive adjustment.
		// After splitting the accounting fields, adjustments/topups increase only
		// goal_total_dollars. cum_done_dollars is written by the spending pipeline.
		u.GoalTotalDollars += *patch.BalanceDelta
		u.Balance = u.GoalTotalDollars - u.CumDoneDollars

		if u.Balance >= u.BalanceTreshold {
			u.LowBalanceNotified = false
		}
	}
	if patch.Timezone != nil {
		u.Timezone = *patch.Timezone
	}
	if patch.EmailNotifications != nil {
		u.EmailNotifications = *patch.EmailNotifications
	}
	if patch.CampaignStatusNotifications != nil {
		u.CampaignStatusNotifications = *patch.CampaignStatusNotifications
	}
	if patch.LowBalanceNotifications != nil {
		u.LowBalanceNotifications = *patch.LowBalanceNotifications
	}
	if patch.CampaignBalanseNotifications != nil {
		u.CampaignBalanseNotifications = *patch.CampaignBalanseNotifications
	}
	if patch.BalanceTreshold != nil {
		u.BalanceTreshold = *patch.BalanceTreshold

		if u.Balance >= u.BalanceTreshold {
			u.LowBalanceNotified = false
		}
	}

	updated, err := s.repo.UpdateTx(ctx, tx, userID, u)
	if err != nil {
		return models.User{}, err
	}
	if patch.BalanceDelta != nil && *patch.BalanceDelta > 0 {
		if err := s.repo.ClearAntiPerekrutBlockedTx(ctx, tx, userID); err != nil {
			return models.User{}, err
		}
	}
	return updated, nil
}

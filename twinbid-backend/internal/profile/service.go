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
	return s.patch(ctx, nil, userID, patch)
}

func (s *Service) PatchTx(ctx context.Context, tx *sql.Tx, userID string, patch PatchProfileRequest) (models.User, error) {
	return s.patch(ctx, tx, userID, patch)
}

func (s *Service) patch(ctx context.Context, tx *sql.Tx, userID string, patch PatchProfileRequest) (models.User, error) {
	var (
		u   models.User
		err error
	)

	if tx != nil {
		u, err = s.repo.GetTx(ctx, tx, userID)
	} else {
		u, err = s.repo.Get(ctx, userID)
	}
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
	if patch.Balance != nil {
		u.Balance = u.Balance + *patch.Balance

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

	if tx != nil {
		return s.repo.UpdateTx(ctx, tx, userID, u)
	}

	return s.repo.Update(ctx, userID, u)
}

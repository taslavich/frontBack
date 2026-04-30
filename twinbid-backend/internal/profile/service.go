package profile

import (
	"context"

	"twinbid-backend/internal/models"
)

type Service struct{ repo *Repository }

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

func (s *Service) Get(ctx context.Context, userID string) (models.User, error) {
	return s.repo.Get(ctx, userID)
}

func (s *Service) Patch(ctx context.Context, userID string, patch PatchProfileRequest) (models.User, error) {
	u, err := s.repo.Get(ctx, userID)
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
		u.Balance = *patch.Balance
		if u.Balance > u.BalanceTreshold {
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
		if u.Balance > u.BalanceTreshold {
			u.LowBalanceNotified = false
		}
	}
	return s.repo.Update(ctx, userID, u)
}

package topups

import (
	"context"
	"fmt"

	"twinbid-backend/internal/bot"
	"twinbid-backend/internal/config"
	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
	"twinbid-backend/internal/promocodes"
)

type Service struct {
	repo      *Repository
	promoSvc  *promocodes.Service
	promoRepo *promocodes.Repository
	botCfg    config.BotConfig
}

func NewService(repo *Repository, promoSvc *promocodes.Service, promoRepo *promocodes.Repository, botCfg config.BotConfig) *Service {
	return &Service{repo: repo, promoSvc: promoSvc, promoRepo: promoRepo, botCfg: botCfg}
}

func (s *Service) List(ctx context.Context, userID string) ([]models.UserTransaction, error) {
	return s.repo.List(ctx, userID)
}

func (s *Service) Create(ctx context.Context, userID string, req CreateTopupRequest) (models.UserTransaction, error) {
	if req.PaymentMethod == "" {
		return models.UserTransaction{}, httpx.BadRequest("payment_method is required")
	}
	if req.DepositAmount <= 0 {
		return models.UserTransaction{}, httpx.BadRequest("deposit_amount must be positive")
	}
	currency := req.Currency
	if currency == "" {
		currency = "USD"
	}
	status := req.Status
	if status == "" {
		status = string(models.TopupPending)
	}
	if !validTopupStatus[status] {
		return models.UserTransaction{}, httpx.BadRequest("invalid status")
	}
	bonus := req.BonusAmount
	var promocodeID *string
	if req.PromocodeID != nil && *req.PromocodeID != "" {
		p, err := s.promoSvc.GetByCode(ctx, *req.PromocodeID)
		if err != nil {
			return models.UserTransaction{}, err
		}
		used, err := s.repo.UserUsedPromocode(ctx, userID, p.ID)
		if err != nil {
			return models.UserTransaction{}, err
		}
		if used {
			return models.UserTransaction{}, httpx.BadRequest("promocode already used by this user")
		}
		bonus = p.BonusPercent
		promocodeID = &p.ID
	}
	total := req.DepositAmount * (1 + bonus/100)
	t := models.UserTransaction{
		UserID: userID, TransactionID: NewTransactionID(), PaymentMethod: req.PaymentMethod,
		BonusAmount: bonus, PromocodeID: promocodeID, TransactionHash: req.TransactionHash,
		DepositAmount: req.DepositAmount, TotalBalanceIncrease: total, Status: models.TopupStatus(status), Currency: currency,
	}

	ut, err := s.repo.Create(ctx, t)
	if err != nil {
		return models.UserTransaction{}, fmt.Errorf("create transaction: %w", err)
	}

	botClient := bot.NewBotClient(s.botCfg.BaseURL, s.botCfg.InternalSecret)
	if err := botClient.SendPaymentModeration(ctx, bot.PaymentModerationRequest{}); err != nil {
		return models.UserTransaction{}, fmt.Errorf("send payment moderation: %w", err)
	}

	return ut, nil
}

func (s *Service) Cancel(ctx context.Context, userID, id string) (models.UserTransaction, error) {
	return s.repo.Cancel(ctx, userID, id)
}

// Approve is backend-side business action: after approval it increases user balance and increments promocode usage_count.
func (s *Service) Approve(ctx context.Context, userID, id string) (models.UserTransaction, error) {
	return s.repo.Approve(ctx, userID, id, s.promoRepo)
}

var validTopupStatus = map[string]bool{"draft": true, "pending": true, "approved": true, "rejected": true, "cancelled": true}

package topups

import (
	"context"
	"fmt"

	"twinbid-backend/internal/bot"
	"twinbid-backend/internal/config"
	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
	"twinbid-backend/internal/profile"
	"twinbid-backend/internal/promocodes"
)

type Service struct {
	repo      *Repository
	promoSvc  *promocodes.Service
	promoRepo *promocodes.Repository
	profile   *profile.Repository
	botCfg    config.BotConfig
}

func NewService(repo *Repository, promoSvc *promocodes.Service, promoRepo *promocodes.Repository, profileRepo *profile.Repository, botCfg config.BotConfig) *Service {
	return &Service{repo: repo, promoSvc: promoSvc, promoRepo: promoRepo, profile: profileRepo, botCfg: botCfg}
}

func (s *Service) List(ctx context.Context, userID string) ([]models.UserTransaction, error) {
	return s.repo.List(ctx, userID)
}

func (s *Service) Get(ctx context.Context, userID, id string) (models.UserTransaction, error) {
	return s.repo.Get(ctx, userID, id)
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

	return ut, nil
}

func (s *Service) Cancel(ctx context.Context, userID, id string) (models.UserTransaction, error) {
	return s.repo.Cancel(ctx, userID, id)
}

func (s *Service) Patch(ctx context.Context, userID, id string, req PatchTopupRequest) (models.UserTransaction, error) {
	current, err := s.Get(ctx, userID, id)
	if err != nil {
		return models.UserTransaction{}, err
	}

	if req.TransactionID != nil {
		current.TransactionID = *req.TransactionID
	}
	if req.PaymentMethod != nil {
		current.PaymentMethod = *req.PaymentMethod
	}
	if req.BonusAmount != nil {
		current.BonusAmount = *req.BonusAmount
	}
	if req.PromocodeIDSet {
		current.PromocodeID = req.PromocodeID
	}
	if req.TransactionHashSet {
		current.TransactionHash = req.TransactionHash
	}
	if req.DepositAmount != nil {
		current.DepositAmount = *req.DepositAmount
	}
	if req.Status != nil {
		current.Status = models.TopupStatus(*req.Status)
	}
	if req.Currency != nil {
		current.Currency = *req.Currency
	}
	if req.TotalBalanceIncrease != nil {
		current.TotalBalanceIncrease = *req.TotalBalanceIncrease
	} else if req.DepositAmount != nil || req.BonusAmount != nil {
		current.TotalBalanceIncrease = current.DepositAmount * (1 + current.BonusAmount/100)
	}

	if req.DepositAmount != nil && *req.DepositAmount <= 0 {
		return models.UserTransaction{}, httpx.BadRequest("deposit_amount must be positive")
	}
	if req.Status != nil && !validTopupStatus[*req.Status] {
		return models.UserTransaction{}, httpx.BadRequest("invalid status")
	}

	//////////////////////////
	ut, err := s.repo.Update(ctx, current)
	if err != nil {
		return models.UserTransaction{}, fmt.Errorf("create transaction: %w", err)
	}
	////////////////

	user, err := s.profile.Get(ctx, userID)
	if err != nil {
		return models.UserTransaction{}, fmt.Errorf("get profile: %w", err)
	}
	userTelegram := ""
	if user.Telegram != nil {
		userTelegram = *user.Telegram
	}
	promocodeIDStr := ""
	if ut.PromocodeID != nil {
		promocodeIDStr = *ut.PromocodeID
	}
	transactionHash := ""
	if ut.TransactionHash != nil {
		transactionHash = *ut.TransactionHash
	}

	botClient := bot.NewBotClient(s.botCfg.BaseURL, s.botCfg.InternalSecret)
	if err := botClient.SendPaymentModeration(ctx, bot.PaymentModerationRequest{
		ID:                   ut.ID,
		TransactionID:        ut.TransactionID,
		UserID:               ut.UserID,
		UserEmail:            user.Mail,
		UserTelegram:         userTelegram,
		PaymentMethod:        ut.PaymentMethod,
		DepositAmount:        ut.DepositAmount,
		BonusAmount:          ut.BonusAmount,
		TotalBalanceIncrease: ut.TotalBalanceIncrease,
		Currency:             ut.Currency,
		PromocodeID:          promocodeIDStr,
		TransactionHash:      transactionHash,
	}); err != nil {
		return models.UserTransaction{}, fmt.Errorf("send payment moderation: %w", err)
	}
	return ut, nil
}

// Approve is backend-side business action: after approval it increases user balance and increments promocode usage_count.
func (s *Service) Approve(ctx context.Context, userID, id string) (models.UserTransaction, error) {
	return s.repo.Approve(ctx, userID, id, s.promoRepo)
}

var validTopupStatus = map[string]bool{"draft": true, "pending": true, "approved": true, "rejected": true, "cancelled": true}

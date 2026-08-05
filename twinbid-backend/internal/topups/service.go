package topups

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"

	"twinbid-backend/internal/bot"
	"twinbid-backend/internal/config"
	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
	"twinbid-backend/internal/passimpay"
	"twinbid-backend/internal/profile"
	"twinbid-backend/internal/promocodes"
)

type Service struct {
	repo       *Repository
	promoSvc   *promocodes.Service
	promoRepo  *promocodes.Repository
	profile    *profile.Repository
	botCfg     config.BotConfig
	profileSvc *profile.Service
	passimPay  *passimpay.Client
}

func NewService(
	repo *Repository,
	promoSvc *promocodes.Service,
	promoRepo *promocodes.Repository,
	profileRepo *profile.Repository,
	profileSvc *profile.Service,
	botCfg config.BotConfig,
	passimPay *passimpay.Client,
) *Service {
	return &Service{
		repo:       repo,
		promoSvc:   promoSvc,
		promoRepo:  promoRepo,
		profile:    profileRepo,
		profileSvc: profileSvc,
		botCfg:     botCfg,
		passimPay:  passimPay,
	}
}

func (s *Service) List(ctx context.Context, userID string) ([]models.UserTransaction, error) {
	return s.repo.List(ctx, userID)
}

func (s *Service) Get(ctx context.Context, userID, id string) (models.UserTransaction, error) {
	return s.repo.Get(ctx, userID, id)
}

func (s *Service) Create(ctx context.Context, userID string, req CreateTopupRequest) (models.UserTransaction, error) {
	channel := strings.TrimSpace(req.PaymentChannel)
	if channel == "" {
		channel = PaymentChannelStaticWallet
	}
	if channel != PaymentChannelStaticWallet && channel != PaymentChannelPassimPayInvoice {
		return models.UserTransaction{}, httpx.BadRequest("invalid payment_channel")
	}
	if req.DepositAmount <= 0 || math.IsNaN(req.DepositAmount) || math.IsInf(req.DepositAmount, 0) {
		return models.UserTransaction{}, httpx.BadRequest("deposit_amount must be positive")
	}
	if channel == PaymentChannelPassimPayInvoice && (s.passimPay == nil || !s.passimPay.Enabled()) {
		return models.UserTransaction{}, httpx.ServiceUnavailable("PassimPay is not configured")
	}

	paymentMethod := strings.TrimSpace(req.PaymentMethod)
	if paymentMethod == "" {
		if channel == PaymentChannelStaticWallet {
			return models.UserTransaction{}, httpx.BadRequest("payment_method is required")
		}
		paymentMethod = "passimpay"
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "USD"
	}

	bonus, promocodeID, err := s.resolvePromocode(ctx, userID, req.PromocodeID)
	if err != nil {
		return models.UserTransaction{}, err
	}
	total := req.DepositAmount * (1 + bonus/100)

	status := models.TopupPending
	var transactionHash *string
	if channel == PaymentChannelPassimPayInvoice {
		status = models.TopupDraft
	} else {
		switch strings.TrimSpace(req.Status) {
		case "", string(models.TopupPending):
			status = models.TopupPending
		case string(models.TopupDraft):
			// Backward-compatible with the existing frontend draft -> PATCH flow.
			status = models.TopupDraft
		default:
			return models.UserTransaction{}, httpx.BadRequest("static-wallet topup can only be created as draft or pending")
		}
		if req.TransactionHash != nil && strings.TrimSpace(*req.TransactionHash) != "" {
			value := strings.TrimSpace(*req.TransactionHash)
			transactionHash = &value
		}
	}

	topup := models.UserTransaction{
		UserID:               userID,
		TransactionID:        NewTransactionID(),
		PaymentChannel:       channel,
		PaymentMethod:        paymentMethod,
		BonusAmount:          bonus,
		PromocodeID:          promocodeID,
		TransactionHash:      transactionHash,
		DepositAmount:        req.DepositAmount,
		TotalBalanceIncrease: total,
		Status:               status,
		Currency:             currency,
	}

	created, err := s.repo.Create(ctx, topup)
	if err != nil {
		return models.UserTransaction{}, fmt.Errorf("create transaction: %w", err)
	}
	if channel == PaymentChannelStaticWallet {
		if created.Status == models.TopupPending && created.TransactionHash != nil && strings.TrimSpace(*created.TransactionHash) != "" {
			if err := s.sendPaymentModeration(ctx, created); err != nil {
				return models.UserTransaction{}, err
			}
		}
		return created, nil
	}

	invoice, err := s.passimPay.CreateInvoice(ctx, created.TransactionID, created.DepositAmount)
	if err != nil {
		payload, _ := json.Marshal(map[string]string{"error": err.Error()})
		if markErr := s.repo.MarkInvoiceCreationFailed(ctx, userID, created.ID, payload); markErr != nil {
			return models.UserTransaction{}, fmt.Errorf("create PassimPay invoice: %v; mark failed: %w", err, markErr)
		}
		return models.UserTransaction{}, fmt.Errorf("create PassimPay invoice: %w", err)
	}

	updated, err := s.repo.UpdateInvoiceCreated(
		ctx,
		userID,
		created.ID,
		invoice.PaymentURL,
		invoice.ProviderPaymentID,
		invoice.ProviderTransactionID,
		invoice.ProviderStatus,
		invoice.Raw,
	)
	if err != nil {
		return models.UserTransaction{}, fmt.Errorf("save PassimPay invoice: %w", err)
	}
	return updated, nil
}

func (s *Service) Cancel(ctx context.Context, userID, id string) (models.UserTransaction, error) {
	return s.repo.Cancel(ctx, userID, id)
}

func (s *Service) Patch(ctx context.Context, userID, id string, req PatchTopupRequest) (models.UserTransaction, error) {
	current, err := s.Get(ctx, userID, id)
	if err != nil {
		return models.UserTransaction{}, err
	}
	if current.PaymentChannel != PaymentChannelStaticWallet {
		return models.UserTransaction{}, httpx.BadRequest("PassimPay invoice cannot be changed from frontend")
	}
	if !req.TransactionHashSet || req.TransactionHash == nil || strings.TrimSpace(*req.TransactionHash) == "" {
		return models.UserTransaction{}, httpx.BadRequest("transaction_hash is required")
	}

	txHash := strings.TrimSpace(*req.TransactionHash)
	wasSubmitted := current.TransactionHash != nil && strings.TrimSpace(*current.TransactionHash) != ""
	updated, err := s.repo.SubmitStaticHash(ctx, userID, id, txHash)
	if err != nil {
		return models.UserTransaction{}, err
	}
	if wasSubmitted {
		return updated, nil
	}
	if err := s.sendPaymentModeration(ctx, updated); err != nil {
		return models.UserTransaction{}, err
	}
	return updated, nil
}

func (s *Service) Approve(ctx context.Context, userID, id string) (models.UserTransaction, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return models.UserTransaction{}, err
	}
	defer tx.Rollback()

	current, err := s.repo.LockByUserAndIDTx(ctx, tx, userID, id)
	if err != nil {
		return models.UserTransaction{}, err
	}
	if current.PaymentChannel != PaymentChannelStaticWallet {
		return models.UserTransaction{}, httpx.BadRequest("only static-wallet topups can be approved manually")
	}
	if current.Status != models.TopupPending || current.TransactionHash == nil || strings.TrimSpace(*current.TransactionHash) == "" {
		return models.UserTransaction{}, httpx.Conflict("topup is not ready for approval")
	}

	credited, err := s.creditLockedTopup(ctx, tx, current, ProviderState{})
	if err != nil {
		return models.UserTransaction{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.UserTransaction{}, err
	}
	return credited, nil
}

func (s *Service) HandlePassimPayWebhook(ctx context.Context, raw []byte, receivedSignature string) error {
	if s.passimPay == nil || !s.passimPay.Enabled() {
		return httpx.ServiceUnavailable("PassimPay is not configured")
	}
	if err := s.passimPay.VerifyPayload(raw, receivedSignature); err != nil {
		return httpx.BadRequest("invalid PassimPay signature")
	}

	webhookStatus, orderID, err := s.passimPay.ParseWebhook(raw)
	if err != nil {
		return httpx.BadRequest(err.Error())
	}

	// The deposit webhook can arrive for a partial invoice payment. Always ask
	// the invoice-status endpoint for the authoritative paid/waiting/error state
	// before changing the user's balance.
	checkedStatus, err := s.passimPay.CheckInvoice(ctx, orderID)
	if err != nil {
		state := providerState(webhookStatus)
		message := "cannot verify PassimPay invoice status: " + err.Error()
		_ = s.repo.InsertWebhookEvent(ctx, orderID, receivedSignature, state, message)
		return httpx.ServiceUnavailable("cannot verify PassimPay invoice status")
	}
	state := providerState(mergeInvoiceStatus(checkedStatus, webhookStatus))
	if err := s.applyProviderState(ctx, orderID, state); err != nil {
		_ = s.repo.InsertWebhookEvent(ctx, orderID, receivedSignature, state, err.Error())
		return err
	}
	if err := s.repo.InsertWebhookEvent(ctx, orderID, receivedSignature, state, ""); err != nil {
		return fmt.Errorf("save PassimPay webhook audit: %w", err)
	}
	return nil
}

func (s *Service) ReconcilePendingInvoices(ctx context.Context) error {
	if s.passimPay == nil || !s.passimPay.Enabled() {
		return nil
	}
	items, err := s.repo.ListPendingInvoices(ctx, 100)
	if err != nil {
		return err
	}
	var firstErr error
	for _, item := range items {
		status, checkErr := s.passimPay.CheckInvoice(ctx, item.TransactionID)
		if checkErr != nil {
			log.Printf("PassimPay status check failed: order_id=%s error=%v", item.TransactionID, checkErr)
			if firstErr == nil {
				firstErr = checkErr
			}
			continue
		}
		if applyErr := s.applyProviderState(ctx, item.TransactionID, providerState(status)); applyErr != nil {
			log.Printf("PassimPay reconciliation apply failed: order_id=%s error=%v", item.TransactionID, applyErr)
			if firstErr == nil {
				firstErr = applyErr
			}
		}
	}
	return firstErr
}

func (s *Service) applyProviderState(ctx context.Context, orderID string, state ProviderState) error {
	switch state.Status {
	case "paid":
		_, err := s.creditInvoice(ctx, orderID, state)
		return err
	case "error":
		status := models.TopupRejected
		_, err := s.repo.UpdateProviderState(ctx, orderID, state, &status)
		return err
	default:
		_, err := s.repo.UpdateProviderState(ctx, orderID, state, nil)
		return err
	}
}

func (s *Service) creditInvoice(ctx context.Context, orderID string, state ProviderState) (models.UserTransaction, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return models.UserTransaction{}, err
	}
	defer tx.Rollback()

	current, err := s.repo.LockByOrderIDTx(ctx, tx, orderID)
	if err != nil {
		return models.UserTransaction{}, err
	}
	if current.CreditedAt != nil {
		return current, nil
	}
	credited, err := s.creditLockedTopup(ctx, tx, current, state)
	if err != nil {
		return models.UserTransaction{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.UserTransaction{}, err
	}
	return credited, nil
}

func (s *Service) creditLockedTopup(ctx context.Context, tx *sql.Tx, current models.UserTransaction, state ProviderState) (models.UserTransaction, error) {
	if current.CreditedAt != nil {
		return current, nil
	}

	if current.PromocodeID != nil && *current.PromocodeID != "" && !current.PromocodeUsageApplied {
		if err := s.promoRepo.IncrementUsageTx(ctx, tx, *current.PromocodeID); err != nil {
			return models.UserTransaction{}, fmt.Errorf("increment promocode usage: %w", err)
		}
		if err := s.repo.MarkPromocodeUsageAppliedTx(ctx, tx, current.ID); err != nil {
			return models.UserTransaction{}, fmt.Errorf("mark promocode usage: %w", err)
		}
	}

	credited, err := s.repo.MarkCreditedTx(ctx, tx, current.ID, state)
	if err != nil {
		return models.UserTransaction{}, fmt.Errorf("mark topup credited: %w", err)
	}
	if _, err := s.profileSvc.PatchTx(ctx, tx, current.UserID, profile.PatchProfileRequest{
		BalanceDelta: floatPtr(current.TotalBalanceIncrease),
	}); err != nil {
		return models.UserTransaction{}, fmt.Errorf("increase user goal_total_dollars: %w", err)
	}
	return credited, nil
}

func (s *Service) resolvePromocode(ctx context.Context, userID string, code *string) (float64, *string, error) {
	if code == nil || strings.TrimSpace(*code) == "" {
		return 0, nil, nil
	}
	promo, err := s.promoSvc.GetByCode(ctx, strings.TrimSpace(*code))
	if err != nil {
		return 0, nil, err
	}
	used, err := s.repo.UserUsedPromocode(ctx, userID, promo.ID)
	if err != nil {
		return 0, nil, err
	}
	if used {
		return 0, nil, httpx.BadRequest("promocode already used by this user")
	}
	return promo.BonusPercent, &promo.ID, nil
}

func (s *Service) sendPaymentModeration(ctx context.Context, topup models.UserTransaction) error {
	user, err := s.profile.Get(ctx, topup.UserID)
	if err != nil {
		return fmt.Errorf("get profile: %w", err)
	}
	userTelegram := ""
	if user.Telegram != nil {
		userTelegram = *user.Telegram
	}
	promocodeID := ""
	if topup.PromocodeID != nil {
		promocodeID = *topup.PromocodeID
	}
	txHash := ""
	if topup.TransactionHash != nil {
		txHash = *topup.TransactionHash
	}

	botClient := bot.NewBotClient(s.botCfg.BaseURL, s.botCfg.InternalSecret)
	if err := botClient.SendPaymentModeration(ctx, bot.PaymentModerationRequest{
		ID:                   topup.ID,
		TransactionID:        topup.TransactionID,
		UserID:               topup.UserID,
		UserEmail:            user.Mail,
		UserTelegram:         userTelegram,
		PaymentMethod:        topup.PaymentMethod,
		DepositAmount:        topup.DepositAmount,
		BonusAmount:          topup.BonusAmount,
		TotalBalanceIncrease: topup.TotalBalanceIncrease,
		Currency:             topup.Currency,
		PromocodeID:          promocodeID,
		TransactionHash:      txHash,
	}); err != nil {
		return fmt.Errorf("send payment moderation: %w", err)
	}
	return nil
}

func mergeInvoiceStatus(authoritative, callback passimpay.InvoiceStatus) passimpay.InvoiceStatus {
	merged := authoritative
	if merged.ProviderPaymentID == "" {
		merged.ProviderPaymentID = callback.ProviderPaymentID
	}
	if merged.ProviderTransactionID == "" {
		merged.ProviderTransactionID = callback.ProviderTransactionID
	}
	if merged.TransactionHash == "" {
		merged.TransactionHash = callback.TransactionHash
	}
	if merged.AmountPaid == nil {
		merged.AmountPaid = callback.AmountPaid
	}
	if merged.AmountCredited == nil {
		merged.AmountCredited = callback.AmountCredited
	}
	if merged.FeeService == nil {
		merged.FeeService = callback.FeeService
	}
	if merged.FeeNetwork == nil {
		merged.FeeNetwork = callback.FeeNetwork
	}
	// Keep the callback body in the transaction audit fields because it contains
	// blockchain details that may be absent from the invoice-status response.
	if len(callback.Raw) > 0 {
		merged.Raw = callback.Raw
	}
	return merged
}

func providerState(status passimpay.InvoiceStatus) ProviderState {
	return ProviderState{
		Status:                status.Status,
		ProviderPaymentID:     status.ProviderPaymentID,
		ProviderTransactionID: status.ProviderTransactionID,
		TransactionHash:       status.TransactionHash,
		AmountPaid:            status.AmountPaid,
		AmountCredited:        status.AmountCredited,
		FeeService:            status.FeeService,
		FeeNetwork:            status.FeeNetwork,
		Raw:                   status.Raw,
	}
}

func floatPtr(v float64) *float64 { return &v }

package topups

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"twinbid-backend/internal/bot"
	"twinbid-backend/internal/config"
	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
	"twinbid-backend/internal/payments"
	"twinbid-backend/internal/profile"
	"twinbid-backend/internal/promocodes"
)

const invoiceLifetime = time.Hour

type Service struct {
	repo       *Repository
	promoSvc   *promocodes.Service
	promoRepo  *promocodes.Repository
	profile    *profile.Repository
	botCfg     config.BotConfig
	profileSvc *profile.Service
	providers  map[string]payments.InvoiceProvider
}

func NewService(
	repo *Repository,
	promoSvc *promocodes.Service,
	promoRepo *promocodes.Repository,
	profileRepo *profile.Repository,
	profileSvc *profile.Service,
	botCfg config.BotConfig,
	providers ...payments.InvoiceProvider,
) *Service {
	providerMap := make(map[string]payments.InvoiceProvider, len(providers))
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		providerMap[strings.ToLower(strings.TrimSpace(provider.Name()))] = provider
	}
	return &Service{
		repo:       repo,
		promoSvc:   promoSvc,
		promoRepo:  promoRepo,
		profile:    profileRepo,
		profileSvc: profileSvc,
		botCfg:     botCfg,
		providers:  providerMap,
	}
}

func (s *Service) List(ctx context.Context, userID string) ([]models.UserTransaction, error) {
	if _, err := s.ExpirePendingInvoices(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, userID)
}

func (s *Service) Get(ctx context.Context, userID, id string) (models.UserTransaction, error) {
	if _, err := s.ExpirePendingInvoices(ctx); err != nil {
		return models.UserTransaction{}, err
	}
	return s.repo.Get(ctx, userID, id)
}

func (s *Service) Create(ctx context.Context, userID string, req CreateTopupRequest) (models.UserTransaction, error) {
	channel, provider, err := s.resolvePaymentSelection(req)
	if err != nil {
		return models.UserTransaction{}, err
	}

	depositAmount, err := normalizeMoney(req.DepositAmount)
	if err != nil {
		return models.UserTransaction{}, err
	}
	if provider != nil && !provider.Enabled() {
		return models.UserTransaction{}, httpx.ServiceUnavailable(provider.Name() + " is not configured")
	}

	paymentMethod := strings.TrimSpace(req.PaymentMethod)
	if paymentMethod == "" {
		if provider == nil {
			return models.UserTransaction{}, httpx.BadRequest("payment_method is required")
		}
		paymentMethod = provider.Name()
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "USD"
	}
	if provider != nil && currency != "USD" {
		return models.UserTransaction{}, httpx.BadRequest("invoice currency must be USD")
	}

	status := models.TopupPending
	var transactionHash *string
	if provider != nil {
		status = models.TopupDraft
	} else {
		switch strings.TrimSpace(req.Status) {
		case "", string(models.TopupPending):
			status = models.TopupPending
		case string(models.TopupDraft):
			status = models.TopupDraft
		default:
			return models.UserTransaction{}, httpx.BadRequest("static-wallet topup can only be created as draft or pending")
		}
		if req.TransactionHash != nil && strings.TrimSpace(*req.TransactionHash) != "" {
			value := strings.TrimSpace(*req.TransactionHash)
			transactionHash = &value
		}
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return models.UserTransaction{}, err
	}
	defer tx.Rollback()

	bonus, promocodeID, reserved, err := s.resolvePromocodeTx(ctx, tx, userID, req.PromocodeID)
	if err != nil {
		return models.UserTransaction{}, err
	}
	total := roundMoney(depositAmount * (1 + bonus/100))
	topup := models.UserTransaction{
		UserID:                userID,
		TransactionID:         NewTransactionID(),
		PaymentChannel:        channel,
		PaymentMethod:         paymentMethod,
		BonusAmount:           bonus,
		PromocodeID:           promocodeID,
		PromocodeUsageApplied: reserved,
		TransactionHash:       transactionHash,
		DepositAmount:         depositAmount,
		TotalBalanceIncrease:  total,
		Status:                status,
		Currency:              currency,
	}
	if provider != nil {
		expiresAt := time.Now().UTC().Add(invoiceLifetime)
		topup.InvoiceExpiresAt = &expiresAt
	}

	created, err := s.repo.CreateTx(ctx, tx, topup)
	if err != nil {
		return models.UserTransaction{}, fmt.Errorf("create transaction: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return models.UserTransaction{}, err
	}

	if provider == nil {
		if created.Status == models.TopupPending && created.TransactionHash != nil && strings.TrimSpace(*created.TransactionHash) != "" {
			if err := s.sendPaymentModeration(ctx, created); err != nil {
				return models.UserTransaction{}, err
			}
		}
		return created, nil
	}

	invoice, err := provider.CreateInvoice(ctx, payments.CreateInvoiceRequest{
		OrderID:  created.TransactionID,
		Amount:   created.DepositAmount,
		Currency: created.Currency,
		Lifetime: invoiceLifetime,
	})
	if err != nil {
		payload, _ := json.Marshal(map[string]string{"error": err.Error()})
		nextCheckAt := time.Now().UTC().Add(time.Minute)
		if markErr := s.repo.MarkInvoiceCreationUnknown(ctx, userID, created.ID, channel, payload, nextCheckAt); markErr != nil {
			return models.UserTransaction{}, fmt.Errorf("create %s invoice: %v; mark unknown: %w", provider.Name(), err, markErr)
		}
		// A timeout does not prove the provider did not create the invoice. Both
		// supported providers can resolve the invoice later by our order ID.
		return s.repo.Get(ctx, userID, created.ID)
	}

	updated, err := s.repo.UpdateInvoiceCreated(
		ctx,
		userID,
		created.ID,
		channel,
		invoice.PaymentURL,
		invoice.ProviderPaymentID,
		invoice.ProviderTransactionID,
		invoice.ProviderStatus,
		invoice.Raw,
	)
	if err != nil {
		return models.UserTransaction{}, fmt.Errorf("save %s invoice: %w", provider.Name(), err)
	}
	return updated, nil
}

func (s *Service) Cancel(ctx context.Context, userID, id string) (models.UserTransaction, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return models.UserTransaction{}, err
	}
	defer tx.Rollback()

	current, err := s.repo.LockByUserAndIDTx(ctx, tx, userID, id)
	if err != nil {
		return models.UserTransaction{}, err
	}
	if current.CreditedAt != nil || (current.Status != models.TopupDraft && current.Status != models.TopupPending) {
		return models.UserTransaction{}, httpx.Conflict("topup cannot be cancelled")
	}
	cancelled, err := s.repo.CancelLockedTx(ctx, tx, current.ID)
	if err != nil {
		return models.UserTransaction{}, err
	}
	// An external invoice may still be paid after local cancellation, so its promo
	// reservation is retained until the provider reports a terminal error.
	if current.PaymentChannel == PaymentChannelStaticWallet {
		if err := s.releasePromocodeReservationTx(ctx, tx, current); err != nil {
			return models.UserTransaction{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return models.UserTransaction{}, err
	}
	return cancelled, nil
}

func (s *Service) Patch(ctx context.Context, userID, id string, req PatchTopupRequest) (models.UserTransaction, error) {
	current, err := s.Get(ctx, userID, id)
	if err != nil {
		return models.UserTransaction{}, err
	}
	if current.PaymentChannel != PaymentChannelStaticWallet {
		return models.UserTransaction{}, httpx.BadRequest("invoice topup cannot be changed from frontend")
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

func (s *Service) HandleProviderWebhook(ctx context.Context, providerName string, raw []byte, headers http.Header) error {
	provider, err := s.provider(providerName)
	if err != nil {
		return err
	}
	if !provider.Enabled() {
		return httpx.ServiceUnavailable(provider.Name() + " is not configured")
	}

	event, err := provider.ParseAndVerifyWebhook(raw, headers)
	if err != nil {
		return httpx.BadRequest("invalid " + provider.Name() + " webhook: " + err.Error())
	}

	// Webhooks are authenticated notifications, not the authoritative source for
	// balance credit. Re-read the invoice from the provider before changing money.
	checkedStatus, err := provider.CheckInvoice(ctx, event.OrderID)
	if err != nil {
		state := event.Status
		message := "cannot verify " + provider.Name() + " invoice status: " + err.Error()
		_ = s.repo.InsertWebhookEvent(ctx, provider.Name(), event.OrderID, event.Signature, state, message)
		return httpx.ServiceUnavailable("cannot verify " + provider.Name() + " invoice status")
	}
	state := mergeInvoiceStatus(checkedStatus, event.Status)
	if err := s.applyProviderState(ctx, provider.PaymentChannel(), event.OrderID, state); err != nil {
		_ = s.repo.InsertWebhookEvent(ctx, provider.Name(), event.OrderID, event.Signature, state, err.Error())
		return err
	}
	if err := s.repo.InsertWebhookEvent(ctx, provider.Name(), event.OrderID, event.Signature, state, ""); err != nil {
		return fmt.Errorf("save %s webhook audit: %w", provider.Name(), err)
	}
	return nil
}

func (s *Service) ExpirePendingInvoices(ctx context.Context) (int64, error) {
	return s.repo.ExpirePendingInvoices(ctx, time.Now().UTC())
}

func (s *Service) ReconcilePendingInvoices(ctx context.Context, providerName string, limit int, requestDelay, retryDelay time.Duration) error {
	provider, err := s.provider(providerName)
	if err != nil {
		return err
	}
	if !provider.Enabled() {
		return nil
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if requestDelay < 0 {
		requestDelay = 0
	}
	if retryDelay <= 0 {
		retryDelay = 5 * time.Minute
	}

	items, err := s.repo.ListPendingInvoices(ctx, provider.PaymentChannel(), limit)
	if err != nil {
		return err
	}
	var firstErr error
	for index, item := range items {
		if index > 0 && requestDelay > 0 {
			timer := time.NewTimer(requestDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}

		status, checkErr := provider.CheckInvoice(ctx, item.TransactionID)
		if checkErr != nil {
			log.Printf("%s status check failed: order_id=%s error=%v", provider.Name(), item.TransactionID, checkErr)
			nextCheckAt := time.Now().UTC().Add(retryDelay)
			if markErr := s.repo.MarkReconcileFailure(ctx, item.TransactionID, provider.PaymentChannel(), checkErr.Error(), nextCheckAt); markErr != nil {
				log.Printf("%s reconciliation failure state error: order_id=%s error=%v", provider.Name(), item.TransactionID, markErr)
			}
			if firstErr == nil {
				firstErr = checkErr
			}
			continue
		}
		if applyErr := s.applyProviderState(ctx, provider.PaymentChannel(), item.TransactionID, status); applyErr != nil {
			log.Printf("%s reconciliation apply failed: order_id=%s error=%v", provider.Name(), item.TransactionID, applyErr)
			if firstErr == nil {
				firstErr = applyErr
			}
		}
	}
	return firstErr
}

func (s *Service) applyProviderState(ctx context.Context, channel, orderID string, state ProviderState) error {
	switch state.Status {
	case "paid":
		_, err := s.creditInvoice(ctx, channel, orderID, state)
		return err
	case "error":
		return s.rejectInvoice(ctx, channel, orderID, state)
	default:
		_, err := s.repo.UpdateProviderState(ctx, orderID, channel, state, nil)
		return err
	}
}

func (s *Service) rejectInvoice(ctx context.Context, channel, orderID string, state ProviderState) error {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	current, err := s.repo.LockByOrderIDTx(ctx, tx, orderID, channel)
	if err != nil {
		return err
	}
	if current.CreditedAt != nil {
		return nil
	}
	status := models.TopupRejected
	if _, err := s.repo.UpdateProviderStateTx(ctx, tx, orderID, channel, state, &status); err != nil {
		return err
	}
	if err := s.releasePromocodeReservationTx(ctx, tx, current); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) creditInvoice(ctx context.Context, channel, orderID string, state ProviderState) (models.UserTransaction, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return models.UserTransaction{}, err
	}
	defer tx.Rollback()

	current, err := s.repo.LockByOrderIDTx(ctx, tx, orderID, channel)
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
		if err := s.reserveExistingPromocodeTx(ctx, tx, current); err != nil {
			return models.UserTransaction{}, err
		}
	}

	credited, err := s.repo.MarkCreditedTx(ctx, tx, current.ID, state)
	if err != nil {
		return models.UserTransaction{}, fmt.Errorf("mark topup credited: %w", err)
	}
	promoAmount := 0.0
	if current.BonusAmount > 0 {
		promoAmount = current.TotalBalanceIncrease
	}
	if _, err := s.profileSvc.IncreaseBalanceWithPromoTx(ctx, tx, current.UserID, current.TotalBalanceIncrease, promoAmount); err != nil {
		return models.UserTransaction{}, fmt.Errorf("increase user balance: %w", err)
	}
	return credited, nil
}

func (s *Service) resolvePromocodeTx(ctx context.Context, tx *sql.Tx, userID string, code *string) (float64, *string, bool, error) {
	if code == nil || strings.TrimSpace(*code) == "" {
		return 0, nil, false, nil
	}
	promo, err := s.promoRepo.GetByCodeForUpdateTx(ctx, tx, strings.TrimSpace(*code))
	if err != nil {
		return 0, nil, false, err
	}
	if err := validatePromocode(promo); err != nil {
		return 0, nil, false, err
	}
	if err := s.repo.LockPromocodeClaimTx(ctx, tx, userID, promo.ID); err != nil {
		return 0, nil, false, err
	}
	used, err := s.repo.UserUsedPromocodeTx(ctx, tx, userID, promo.ID, "")
	if err != nil {
		return 0, nil, false, err
	}
	if used {
		return 0, nil, false, httpx.BadRequest("promocode already used by this user")
	}
	if err := s.promoRepo.ReserveUsageTx(ctx, tx, promo.ID); err != nil {
		return 0, nil, false, err
	}
	return promo.BonusPercent, &promo.ID, true, nil
}

func (s *Service) reserveExistingPromocodeTx(ctx context.Context, tx *sql.Tx, current models.UserTransaction) error {
	promoID := strings.TrimSpace(*current.PromocodeID)
	promo, err := s.promoRepo.GetByIDForUpdateTx(ctx, tx, promoID)
	if err != nil {
		return err
	}
	if err := validatePromocode(promo); err != nil {
		return err
	}
	if err := s.repo.LockPromocodeClaimTx(ctx, tx, current.UserID, promoID); err != nil {
		return err
	}
	used, err := s.repo.UserUsedPromocodeTx(ctx, tx, current.UserID, promoID, current.ID)
	if err != nil {
		return err
	}
	if used {
		return httpx.BadRequest("promocode already used by this user")
	}
	if err := s.promoRepo.ReserveUsageTx(ctx, tx, promoID); err != nil {
		return err
	}
	return s.repo.MarkPromocodeUsageAppliedTx(ctx, tx, current.ID)
}

func (s *Service) releasePromocodeReservationTx(ctx context.Context, tx *sql.Tx, current models.UserTransaction) error {
	if current.PromocodeID == nil || strings.TrimSpace(*current.PromocodeID) == "" || !current.PromocodeUsageApplied {
		return nil
	}
	if err := s.promoRepo.ReleaseUsageTx(ctx, tx, *current.PromocodeID); err != nil {
		return fmt.Errorf("release promocode usage: %w", err)
	}
	if err := s.repo.MarkPromocodeUsageReleasedTx(ctx, tx, current.ID); err != nil {
		return fmt.Errorf("mark promocode usage released: %w", err)
	}
	return nil
}

func validatePromocode(promo models.Promocode) error {
	now := time.Now().UTC()
	if promo.ValidFrom != nil && now.Before(*promo.ValidFrom) {
		return httpx.BadRequest("promocode is not active yet")
	}
	if promo.ValidTo != nil && now.After(*promo.ValidTo) {
		return httpx.BadRequest("promocode expired")
	}
	if promo.UsageLimit != nil && promo.UsageCount >= *promo.UsageLimit {
		return httpx.BadRequest("promocode usage limit exceeded")
	}
	return nil
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

func mergeInvoiceStatus(authoritative, callback payments.InvoiceStatus) payments.InvoiceStatus {
	merged := authoritative
	if merged.PaymentURL == "" {
		merged.PaymentURL = callback.PaymentURL
	}
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
	// Keep the callback body in the audit payload: it often contains blockchain
	// details that are absent from the provider's status endpoint response.
	if len(callback.Raw) > 0 {
		merged.Raw = callback.Raw
	}
	return merged
}

func (s *Service) provider(name string) (payments.InvoiceProvider, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	provider := s.providers[name]
	if provider == nil {
		return nil, httpx.BadRequest("unsupported payment provider")
	}
	return provider, nil
}

func (s *Service) resolvePaymentSelection(req CreateTopupRequest) (string, payments.InvoiceProvider, error) {
	channel := strings.TrimSpace(req.PaymentChannel)
	providerName := strings.ToLower(strings.TrimSpace(req.Provider))

	// Keep the legacy manual wallet flow, but require it to be selected
	// explicitly. Missing both fields must never silently choose a provider.
	if channel == PaymentChannelStaticWallet {
		if providerName != "" {
			return "", nil, httpx.BadRequest("provider must not be set for static_wallet")
		}
		return PaymentChannelStaticWallet, nil, nil
	}
	if providerName == "" {
		return "", nil, httpx.BadRequest("payment provider is required")
	}
	provider, err := s.provider(providerName)
	if err != nil {
		return "", nil, err
	}
	expectedChannel := provider.PaymentChannel()
	if channel != "" && channel != expectedChannel {
		return "", nil, httpx.BadRequest("payment_channel does not match provider")
	}
	return expectedChannel, provider, nil
}

func normalizeMoney(value float64) (float64, error) {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, httpx.BadRequest("deposit_amount must be positive")
	}
	rounded := roundMoney(value)
	if math.Abs(value-rounded) > 0.0000001 {
		return 0, httpx.BadRequest("deposit_amount must have at most two decimal places")
	}
	return rounded, nil
}

func roundMoney(value float64) float64 {
	return math.Round(value*100) / 100
}

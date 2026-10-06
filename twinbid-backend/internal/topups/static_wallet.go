package topups

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/big"
	"strings"
	"time"

	"twinbid-backend/internal/models"
	"twinbid-backend/internal/payments"
	"twinbid-backend/internal/tronscan"
)

type StaticWalletVerifier interface {
	Enabled() bool
	VerifyUSDTTransfer(ctx context.Context, txHash string, expectedAmountMicro *big.Int) (tronscan.Verification, error)
}

type StaticWalletReconcileResult struct {
	Checked  int
	Approved int
	Pending  int
	Invalid  int
	Errors   int
}

func (s *Service) ConfigureStaticWalletAutoApproval(verifier StaticWalletVerifier, retryDelay time.Duration) {
	s.staticWalletVerifier = verifier
	if retryDelay <= 0 {
		retryDelay = 15 * time.Second
	}
	s.staticWalletRetryDelay = retryDelay
}

func (s *Service) StaticWalletAutoApprovalEnabled() bool {
	return s != nil && s.staticWalletVerifier != nil && s.staticWalletVerifier.Enabled()
}

func (s *Service) ReconcilePendingStaticWallets(ctx context.Context, limit int, requestDelay, retryDelay time.Duration) (StaticWalletReconcileResult, error) {
	var result StaticWalletReconcileResult
	if !s.StaticWalletAutoApprovalEnabled() {
		return result, nil
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if requestDelay < 0 {
		requestDelay = 0
	}
	if retryDelay <= 0 {
		retryDelay = s.staticWalletRetryDelay
		if retryDelay <= 0 {
			retryDelay = 15 * time.Second
		}
	}

	items, err := s.repo.ListPendingStaticWallets(ctx, limit)
	if err != nil {
		return result, err
	}

	var firstErr error
	for index, item := range items {
		if index > 0 && requestDelay > 0 {
			timer := time.NewTimer(requestDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return result, ctx.Err()
			case <-timer.C:
			}
		}

		result.Checked++
		updated, state, checkErr := s.verifyAndMaybeCreditStaticWallet(ctx, item, retryDelay)
		if checkErr != nil {
			result.Errors++
			log.Printf("TronScan static-wallet verification error: topup_id=%s hash=%s error=%v", item.ID, maskedHash(item.TransactionHash), checkErr)
			if firstErr == nil {
				firstErr = checkErr
			}
			continue
		}
		switch state {
		case tronscan.VerificationVerified:
			if updated.Status == models.TopupApproved {
				result.Approved++
			}
		case tronscan.VerificationPending:
			result.Pending++
		case tronscan.VerificationInvalid:
			result.Invalid++
		}
	}
	return result, firstErr
}

func (s *Service) verifyAndMaybeCreditStaticWallet(ctx context.Context, item models.UserTransaction, retryDelay time.Duration) (models.UserTransaction, tronscan.VerificationState, error) {
	if !s.StaticWalletAutoApprovalEnabled() {
		return item, tronscan.VerificationPending, nil
	}
	if item.PaymentChannel != PaymentChannelStaticWallet || item.Status != models.TopupPending || item.CreditedAt != nil || item.TransactionHash == nil {
		return item, tronscan.VerificationPending, nil
	}
	if !isUSDTTRC20PaymentMethod(item.PaymentMethod) {
		return item, tronscan.VerificationPending, nil
	}
	hash := strings.TrimSpace(*item.TransactionHash)
	if hash == "" {
		return item, tronscan.VerificationPending, nil
	}

	expectedMicro, err := usdtMicroAmount(item.DepositAmount)
	if err != nil {
		return item, tronscan.VerificationInvalid, err
	}
	verification, err := s.staticWalletVerifier.VerifyUSDTTransfer(ctx, hash, expectedMicro)
	if err != nil {
		next := time.Now().UTC().Add(retryDelay)
		markErr := s.repo.MarkStaticWalletVerification(ctx, item.ID, "tronscan_retry", nil, err.Error(), &next)
		if markErr != nil {
			return item, tronscan.VerificationPending, fmt.Errorf("TronScan lookup failed: %v; save retry state: %w", err, markErr)
		}
		return item, tronscan.VerificationPending, err
	}

	switch verification.State {
	case tronscan.VerificationPending:
		next := time.Now().UTC().Add(retryDelay)
		if err := s.repo.MarkStaticWalletVerification(ctx, item.ID, "tronscan_pending", verification.Raw, verification.Reason, &next); err != nil {
			return item, verification.State, err
		}
		return item, verification.State, nil
	case tronscan.VerificationInvalid:
		if err := s.repo.MarkStaticWalletVerification(ctx, item.ID, "tronscan_invalid", verification.Raw, verification.Reason, nil); err != nil {
			return item, verification.State, err
		}
		log.Printf("TronScan static-wallet verification rejected: topup_id=%s hash=%s reason=%s", item.ID, maskedHash(item.TransactionHash), verification.Reason)
		return item, verification.State, nil
	case tronscan.VerificationVerified:
		credited, err := s.creditVerifiedStaticWallet(ctx, item, verification)
		if err != nil {
			return item, verification.State, err
		}
		log.Printf("TronScan static-wallet auto-approved: topup_id=%s hash=%s amount=%.2f confirmations=%d", credited.ID, maskedHash(credited.TransactionHash), credited.DepositAmount, verification.Confirmations)
		return credited, verification.State, nil
	default:
		return item, verification.State, fmt.Errorf("unknown TronScan verification state %q", verification.State)
	}
}

func (s *Service) creditVerifiedStaticWallet(ctx context.Context, checked models.UserTransaction, verification tronscan.Verification) (models.UserTransaction, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return models.UserTransaction{}, err
	}
	defer tx.Rollback()

	current, err := s.repo.LockByUserAndIDTx(ctx, tx, checked.UserID, checked.ID)
	if err != nil {
		return models.UserTransaction{}, err
	}
	if current.CreditedAt != nil || current.Status == models.TopupApproved {
		return current, nil
	}
	if current.PaymentChannel != PaymentChannelStaticWallet || current.Status != models.TopupPending || current.TransactionHash == nil {
		return models.UserTransaction{}, fmt.Errorf("static-wallet topup changed before automatic approval")
	}
	currentHash := strings.TrimSpace(*current.TransactionHash)
	checkedHash := ""
	if checked.TransactionHash != nil {
		checkedHash = strings.TrimSpace(*checked.TransactionHash)
	}
	if currentHash == "" || currentHash != checkedHash {
		return models.UserTransaction{}, fmt.Errorf("static-wallet transaction hash changed before automatic approval")
	}
	if err := s.repo.LockStaticWalletHashTx(ctx, tx, currentHash); err != nil {
		return models.UserTransaction{}, fmt.Errorf("lock static-wallet transaction hash: %w", err)
	}
	duplicate, err := s.repo.HasOtherTopupWithHashTx(ctx, tx, current.ID, currentHash)
	if err != nil {
		return models.UserTransaction{}, fmt.Errorf("check duplicate transaction hash: %w", err)
	}
	if duplicate {
		if err := tx.Rollback(); err != nil {
			return models.UserTransaction{}, err
		}
		if err := s.repo.MarkStaticWalletVerification(ctx, current.ID, "tronscan_duplicate", verification.Raw, "transaction hash is already used by another topup", nil); err != nil {
			return models.UserTransaction{}, err
		}
		return current, fmt.Errorf("transaction hash is already used by another topup")
	}

	expectedMicro, err := usdtMicroAmount(current.DepositAmount)
	if err != nil {
		return models.UserTransaction{}, err
	}
	if verification.AmountMicro == nil || verification.AmountMicro.Cmp(expectedMicro) != 0 {
		return models.UserTransaction{}, fmt.Errorf("static-wallet amount changed before automatic approval")
	}
	amountPaid := current.DepositAmount
	state := payments.InvoiceStatus{
		Status:          "tronscan_verified",
		TransactionHash: currentHash,
		AmountPaid:      &amountPaid,
		AmountCredited:  &amountPaid,
		Raw:             append(json.RawMessage(nil), verification.Raw...),
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

func isUSDTTRC20PaymentMethod(value string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	return normalized == "USDT TRC20" || normalized == "USDT_TRC20"
}

func usdtMicroAmount(amount float64) (*big.Int, error) {
	if amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return nil, fmt.Errorf("deposit amount must be positive")
	}
	microFloat := amount * 1_000_000
	if microFloat > math.MaxInt64 {
		return nil, fmt.Errorf("deposit amount is too large")
	}
	micro := int64(math.Round(microFloat))
	if math.Abs(microFloat-float64(micro)) > 0.0001 {
		return nil, fmt.Errorf("deposit amount cannot be represented as USDT micro-units")
	}
	return big.NewInt(micro), nil
}

func maskedHash(hash *string) string {
	if hash == nil {
		return ""
	}
	value := strings.TrimSpace(*hash)
	if len(value) <= 12 {
		return value
	}
	return value[:6] + "..." + value[len(value)-6:]
}

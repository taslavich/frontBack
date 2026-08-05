package topups

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"

	"github.com/google/uuid"
)

type Repository struct{ db *sql.DB }

func NewRepository(dbConn *sql.DB) *Repository { return &Repository{db: dbConn} }

func (r *Repository) List(ctx context.Context, userID string) ([]models.UserTransaction, error) {
	rows, err := r.db.QueryContext(ctx, selectTx+` WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.UserTransaction
	for rows.Next() {
		t, err := scanTx(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository) Get(ctx context.Context, userID, id string) (models.UserTransaction, error) {
	row := r.db.QueryRowContext(ctx, selectTx+` WHERE id=$2 AND user_id=$1`, userID, id)
	item, err := scanTx(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.UserTransaction{}, httpx.NotFound("topup not found")
	}
	return item, err
}

func (r *Repository) Create(ctx context.Context, t models.UserTransaction) (models.UserTransaction, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO user_transactions (
			user_id, transaction_id, payment_channel, payment_method, bonus_amount,
			promocode_id, promocode_usage_applied, transaction_hash, deposit_amount,
			total_balance_increase, status, currency, payment_url, provider_status,
			provider_payment_id, provider_transaction_id, amount_paid, amount_credited,
			fee_service, fee_network, credited_at, provider_payload
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
		RETURNING `+returningTx,
		t.UserID, t.TransactionID, t.PaymentChannel, t.PaymentMethod, t.BonusAmount,
		t.PromocodeID, t.PromocodeUsageApplied, t.TransactionHash, t.DepositAmount,
		t.TotalBalanceIncrease, t.Status, t.Currency, t.PaymentURL, t.ProviderStatus,
		t.ProviderPaymentID, t.ProviderTransactionID, t.AmountPaid, t.AmountCredited,
		t.FeeService, t.FeeNetwork, t.CreditedAt, nullableJSON(t.ProviderPayload),
	)
	return scanTx(row)
}

func (r *Repository) CreateTx(ctx context.Context, tx *sql.Tx, t models.UserTransaction) (models.UserTransaction, error) {
	row := tx.QueryRowContext(ctx, `
		INSERT INTO user_transactions (
			user_id, transaction_id, payment_channel, payment_method, bonus_amount,
			promocode_id, promocode_usage_applied, transaction_hash, deposit_amount,
			total_balance_increase, status, currency, payment_url, provider_status,
			provider_payment_id, provider_transaction_id, amount_paid, amount_credited,
			fee_service, fee_network, credited_at, provider_payload
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
		RETURNING `+returningTx,
		t.UserID, t.TransactionID, t.PaymentChannel, t.PaymentMethod, t.BonusAmount,
		t.PromocodeID, t.PromocodeUsageApplied, t.TransactionHash, t.DepositAmount,
		t.TotalBalanceIncrease, t.Status, t.Currency, t.PaymentURL, t.ProviderStatus,
		t.ProviderPaymentID, t.ProviderTransactionID, t.AmountPaid, t.AmountCredited,
		t.FeeService, t.FeeNetwork, t.CreditedAt, nullableJSON(t.ProviderPayload),
	)
	return scanTx(row)
}

func (r *Repository) Cancel(ctx context.Context, userID, id string) (models.UserTransaction, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE user_transactions SET status='cancelled', updated_at=NOW()
		WHERE id=$2 AND user_id=$1 AND status IN ('draft','pending') AND credited_at IS NULL
		RETURNING `+returningTx, userID, id)
	t, err := scanTx(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.UserTransaction{}, httpx.NotFound("topup not found or cannot be cancelled")
	}
	return t, err
}

func (r *Repository) SubmitStaticHash(ctx context.Context, userID, id, txHash string) (models.UserTransaction, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE user_transactions
		SET transaction_hash=$3, status='pending', updated_at=NOW()
		WHERE id=$2 AND user_id=$1
		  AND payment_channel='static_wallet'
		  AND status IN ('draft','pending')
		  AND credited_at IS NULL
		  AND (transaction_hash IS NULL OR transaction_hash='' OR transaction_hash=$3)
		  AND NOT EXISTS (
			SELECT 1 FROM user_transactions other
			WHERE other.transaction_hash=$3 AND other.id<>$2
		  )
		RETURNING `+returningTx, userID, id, txHash)
	item, err := scanTx(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.UserTransaction{}, httpx.Conflict("topup cannot accept transaction hash")
	}
	return item, err
}

func (r *Repository) UpdateInvoiceCreated(ctx context.Context, userID, id string, paymentURL, providerPaymentID, providerTransactionID, providerStatus string, payload json.RawMessage) (models.UserTransaction, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE user_transactions
		SET payment_url=$3,
			provider_payment_id=NULLIF($4,''),
			provider_transaction_id=NULLIF($5,''),
			provider_status=COALESCE(NULLIF($6,''),'waiting'),
			provider_payload=$7,
			status='pending',
			updated_at=NOW()
		WHERE id=$2 AND user_id=$1 AND payment_channel='passimpay_invoice' AND credited_at IS NULL
		RETURNING `+returningTx, userID, id, paymentURL, providerPaymentID, providerTransactionID, providerStatus, nullableJSON(payload))
	item, err := scanTx(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.UserTransaction{}, httpx.NotFound("invoice topup not found")
	}
	return item, err
}

func (r *Repository) MarkInvoiceCreationUnknown(ctx context.Context, userID, id string, payload json.RawMessage, nextCheckAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE user_transactions
		SET provider_status='create_unknown', status='pending', provider_payload=$3,
			provider_check_attempts=0, provider_next_check_at=$4,
			provider_last_error='invoice creation result is unknown', updated_at=NOW()
		WHERE id=$2 AND user_id=$1 AND payment_channel='passimpay_invoice' AND credited_at IS NULL
	`, userID, id, nullableJSON(payload), nextCheckAt)
	return err
}

func (r *Repository) UpdateProviderState(ctx context.Context, orderID string, state ProviderState, internalStatus *models.TopupStatus) (models.UserTransaction, error) {
	statusValue := any(nil)
	if internalStatus != nil {
		statusValue = string(*internalStatus)
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE user_transactions
		SET payment_url=COALESCE(NULLIF($2,''), payment_url),
			provider_status=COALESCE(NULLIF($3,''), provider_status),
			provider_payment_id=COALESCE(NULLIF($4,''), provider_payment_id),
			provider_transaction_id=COALESCE(NULLIF($5,''), provider_transaction_id),
			transaction_hash=COALESCE(NULLIF($6,''), transaction_hash),
			amount_paid=COALESCE($7, amount_paid),
			amount_credited=COALESCE($8, amount_credited),
			fee_service=COALESCE($9, fee_service),
			fee_network=COALESCE($10, fee_network),
			provider_payload=COALESCE($11, provider_payload),
			status=CASE
				WHEN credited_at IS NULL THEN COALESCE($12, status)
				ELSE status
			END,
			provider_check_attempts=0, provider_next_check_at=NULL, provider_last_error=NULL,
			updated_at=NOW()
		WHERE transaction_id=$1 AND payment_channel='passimpay_invoice'
		RETURNING `+returningTx,
		orderID, state.PaymentURL, state.Status, state.ProviderPaymentID, state.ProviderTransactionID, state.TransactionHash,
		state.AmountPaid, state.AmountCredited, state.FeeService, state.FeeNetwork,
		nullableJSON(state.Raw), statusValue,
	)
	item, err := scanTx(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.UserTransaction{}, httpx.NotFound("invoice topup not found")
	}
	return item, err
}

func (r *Repository) UpdateProviderStateTx(ctx context.Context, tx *sql.Tx, orderID string, state ProviderState, internalStatus *models.TopupStatus) (models.UserTransaction, error) {
	statusValue := any(nil)
	if internalStatus != nil {
		statusValue = string(*internalStatus)
	}
	row := tx.QueryRowContext(ctx, `
		UPDATE user_transactions
		SET payment_url=COALESCE(NULLIF($2,''), payment_url),
			provider_status=COALESCE(NULLIF($3,''), provider_status),
			provider_payment_id=COALESCE(NULLIF($4,''), provider_payment_id),
			provider_transaction_id=COALESCE(NULLIF($5,''), provider_transaction_id),
			transaction_hash=COALESCE(NULLIF($6,''), transaction_hash),
			amount_paid=COALESCE($7, amount_paid), amount_credited=COALESCE($8, amount_credited),
			fee_service=COALESCE($9, fee_service), fee_network=COALESCE($10, fee_network),
			provider_payload=COALESCE($11, provider_payload),
			status=CASE WHEN credited_at IS NULL THEN COALESCE($12, status) ELSE status END,
			provider_check_attempts=0, provider_next_check_at=NULL, provider_last_error=NULL,
			updated_at=NOW()
		WHERE transaction_id=$1 AND payment_channel='passimpay_invoice'
		RETURNING `+returningTx,
		orderID, state.PaymentURL, state.Status, state.ProviderPaymentID, state.ProviderTransactionID,
		state.TransactionHash, state.AmountPaid, state.AmountCredited, state.FeeService, state.FeeNetwork,
		nullableJSON(state.Raw), statusValue)
	item, err := scanTx(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.UserTransaction{}, httpx.NotFound("invoice topup not found")
	}
	return item, err
}

func (r *Repository) MarkReconcileFailure(ctx context.Context, orderID, message string, nextCheckAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE user_transactions
		SET provider_check_attempts=provider_check_attempts+1,
			provider_next_check_at=$2, provider_last_error=$3, updated_at=NOW()
		WHERE transaction_id=$1 AND payment_channel='passimpay_invoice' AND credited_at IS NULL
	`, orderID, nextCheckAt, message)
	return err
}

func (r *Repository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}

func (r *Repository) LockByUserAndIDTx(ctx context.Context, tx *sql.Tx, userID, id string) (models.UserTransaction, error) {
	row := tx.QueryRowContext(ctx, selectTx+` WHERE id=$2 AND user_id=$1 FOR UPDATE`, userID, id)
	item, err := scanTx(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.UserTransaction{}, httpx.NotFound("topup not found")
	}
	return item, err
}

func (r *Repository) CancelLockedTx(ctx context.Context, tx *sql.Tx, topupID string) (models.UserTransaction, error) {
	row := tx.QueryRowContext(ctx, `
		UPDATE user_transactions
		SET status='cancelled', updated_at=NOW()
		WHERE id=$1 AND status IN ('draft','pending') AND credited_at IS NULL
		RETURNING `+returningTx, topupID)
	item, err := scanTx(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.UserTransaction{}, httpx.Conflict("topup cannot be cancelled")
	}
	return item, err
}

func (r *Repository) LockPromocodeClaimTx(ctx context.Context, tx *sql.Tx, userID, promocodeID string) error {
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, userID+":"+promocodeID)
	return err
}

func (r *Repository) UserUsedPromocodeTx(ctx context.Context, tx *sql.Tx, userID, promocodeID, excludeTopupID string) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_transactions
			WHERE user_id=$1 AND promocode_id=$2
			  AND (status IN ('draft','pending','approved') OR promocode_usage_applied=true)
			  AND ($3='' OR id::text<>$3)
		)
	`, userID, promocodeID, excludeTopupID).Scan(&exists)
	return exists, err
}

func (r *Repository) MarkPromocodeUsageReleasedTx(ctx context.Context, tx *sql.Tx, topupID string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE user_transactions
		SET promocode_usage_applied=false, updated_at=NOW()
		WHERE id=$1 AND promocode_usage_applied=true
	`, topupID)
	return err
}

func (r *Repository) LockByOrderIDTx(ctx context.Context, tx *sql.Tx, orderID string) (models.UserTransaction, error) {
	row := tx.QueryRowContext(ctx, selectTx+` WHERE transaction_id=$1 AND payment_channel='passimpay_invoice' FOR UPDATE`, orderID)
	item, err := scanTx(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.UserTransaction{}, httpx.NotFound("invoice topup not found")
	}
	return item, err
}

func (r *Repository) MarkCreditedTx(ctx context.Context, tx *sql.Tx, topupID string, state ProviderState) (models.UserTransaction, error) {
	row := tx.QueryRowContext(ctx, `
		UPDATE user_transactions
		SET status='approved',
			payment_url=COALESCE(NULLIF($2,''), payment_url),
			provider_status=COALESCE(NULLIF($3,''), provider_status),
			provider_payment_id=COALESCE(NULLIF($4,''), provider_payment_id),
			provider_transaction_id=COALESCE(NULLIF($5,''), provider_transaction_id),
			transaction_hash=COALESCE(NULLIF($6,''), transaction_hash),
			amount_paid=COALESCE($7, amount_paid),
			amount_credited=COALESCE($8, amount_credited),
			fee_service=COALESCE($9, fee_service),
			fee_network=COALESCE($10, fee_network),
			provider_payload=COALESCE($11, provider_payload),
			credited_at=COALESCE(credited_at, NOW()),
			provider_check_attempts=0, provider_next_check_at=NULL, provider_last_error=NULL,
			updated_at=NOW()
		WHERE id=$1 AND credited_at IS NULL
		RETURNING `+returningTx,
		topupID, state.PaymentURL, state.Status, state.ProviderPaymentID, state.ProviderTransactionID, state.TransactionHash,
		state.AmountPaid, state.AmountCredited, state.FeeService, state.FeeNetwork,
		nullableJSON(state.Raw),
	)
	item, err := scanTx(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.UserTransaction{}, httpx.Conflict("topup already credited")
	}
	return item, err
}

func (r *Repository) MarkPromocodeUsageAppliedTx(ctx context.Context, tx *sql.Tx, topupID string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE user_transactions
		SET promocode_usage_applied=true, updated_at=NOW()
		WHERE id=$1
	`, topupID)
	return err
}

func (r *Repository) ListPendingInvoices(ctx context.Context, limit int) ([]models.UserTransaction, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, selectTx+`
		WHERE payment_channel='passimpay_invoice'
		  AND credited_at IS NULL
		  AND status IN ('draft','pending','cancelled')
		  AND COALESCE(provider_status,'') NOT IN ('paid','error')
		  AND (provider_next_check_at IS NULL OR provider_next_check_at <= NOW())
		ORDER BY COALESCE(provider_next_check_at, updated_at) ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.UserTransaction
	for rows.Next() {
		item, err := scanTx(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) InsertWebhookEvent(ctx context.Context, orderID, signature string, state ProviderState, processingError string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO payment_webhook_events (
			provider, order_id, transaction_hash, signature, provider_status,
			payload, processing_error, processed_at
		) VALUES ('passimpay',$1,NULLIF($2,''),$3,NULLIF($4,''),$5,NULLIF($6,''),NOW())
	`, orderID, state.TransactionHash, signature, state.Status, nullableJSON(state.Raw), processingError)
	return err
}

func (r *Repository) UserUsedPromocode(ctx context.Context, userID, promocodeID string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_transactions
			WHERE user_id=$1 AND promocode_id=$2
			  AND (status IN ('draft','pending','approved') OR promocode_usage_applied=true)
		)
	`, userID, promocodeID).Scan(&exists)
	return exists, err
}

const returningTx = `id, user_id, transaction_time, transaction_id, payment_channel, payment_method,
	bonus_amount, promocode_id, promocode_usage_applied, transaction_hash, deposit_amount,
	total_balance_increase, status, currency, payment_url, provider_status,
	provider_payment_id, provider_transaction_id, amount_paid, amount_credited,
	fee_service, fee_network, credited_at, provider_payload, provider_check_attempts, provider_next_check_at, provider_last_error, created_at, updated_at`

const selectTx = `SELECT ` + returningTx + ` FROM user_transactions`

type scanner interface{ Scan(dest ...any) error }

func scanTx(s scanner) (models.UserTransaction, error) {
	var t models.UserTransaction
	var promo, hash, paymentURL, providerStatus, providerPaymentID, providerTransactionID sql.NullString
	var amountPaid, amountCredited, feeService, feeNetwork sql.NullFloat64
	var creditedAt, providerNextCheckAt sql.NullTime
	var providerLastError sql.NullString
	var providerPayload []byte
	var status string
	err := s.Scan(
		&t.ID, &t.UserID, &t.TransactionTime, &t.TransactionID, &t.PaymentChannel, &t.PaymentMethod,
		&t.BonusAmount, &promo, &t.PromocodeUsageApplied, &hash, &t.DepositAmount,
		&t.TotalBalanceIncrease, &status, &t.Currency, &paymentURL, &providerStatus,
		&providerPaymentID, &providerTransactionID, &amountPaid, &amountCredited,
		&feeService, &feeNetwork, &creditedAt, &providerPayload, &t.ProviderCheckAttempts, &providerNextCheckAt, &providerLastError, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return models.UserTransaction{}, err
	}
	if promo.Valid {
		t.PromocodeID = &promo.String
	}
	if hash.Valid {
		t.TransactionHash = &hash.String
	}
	if paymentURL.Valid {
		t.PaymentURL = &paymentURL.String
	}
	if providerStatus.Valid {
		t.ProviderStatus = &providerStatus.String
	}
	if providerPaymentID.Valid {
		t.ProviderPaymentID = &providerPaymentID.String
	}
	if providerTransactionID.Valid {
		t.ProviderTransactionID = &providerTransactionID.String
	}
	if amountPaid.Valid {
		t.AmountPaid = &amountPaid.Float64
	}
	if amountCredited.Valid {
		t.AmountCredited = &amountCredited.Float64
	}
	if feeService.Valid {
		t.FeeService = &feeService.Float64
	}
	if feeNetwork.Valid {
		t.FeeNetwork = &feeNetwork.Float64
	}
	if creditedAt.Valid {
		t.CreditedAt = &creditedAt.Time
	}
	if len(providerPayload) > 0 {
		t.ProviderPayload = append(json.RawMessage(nil), providerPayload...)
	}
	if providerNextCheckAt.Valid {
		t.ProviderNextCheckAt = &providerNextCheckAt.Time
	}
	if providerLastError.Valid {
		t.ProviderLastError = &providerLastError.String
	}
	if strings.TrimSpace(t.PaymentChannel) == "" {
		t.PaymentChannel = PaymentChannelStaticWallet
	}
	t.Status = models.TopupStatus(status)
	return t, nil
}

func NewTransactionID() string { return uuid.NewString() }

func nullableJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}

type ProviderState struct {
	PaymentURL            string
	Status                string
	ProviderPaymentID     string
	ProviderTransactionID string
	TransactionHash       string
	AmountPaid            *float64
	AmountCredited        *float64
	FeeService            *float64
	FeeNetwork            *float64
	Raw                   json.RawMessage
}

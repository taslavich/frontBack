package percenterbilling

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type Repository struct {
	db *sql.DB
}

var ErrEventPayloadConflict = errors.New("promo spend event payload conflict")

type ApplyRequest struct {
	EventID         string  `json:"event_id"`
	UserID          string  `json:"user_id"`
	CampaignID      string  `json:"campaign_id"`
	PromoGeneration int64   `json:"promo_generation"`
	SpendDelta      float64 `json:"spend_delta"`
}

type PromoState struct {
	Remaining  float64 `json:"remaining"`
	Revision   int64   `json:"revision"`
	Generation int64   `json:"generation"`
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// ApplyPromoSpend is the authoritative, idempotent promo mutation. It is owned
// by the cabinet backend because users.promo_spend_remaining/promo_revision/
// promo_generation are cabinet PostgreSQL state. The caller must pass the promo
// generation captured by ADV at pricing time. A delayed callback from an older
// generation is recorded idempotently but cannot consume a later promo grant.
func (r *Repository) ApplyPromoSpend(ctx context.Context, req ApplyRequest) (PromoState, error) {
	if r == nil || r.db == nil {
		return PromoState{}, errors.New("percenter billing PostgreSQL is nil")
	}
	if err := validateApplyRequest(req); err != nil {
		return PromoState{}, err
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return PromoState{}, fmt.Errorf("begin promo spend transaction: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO adv_promo_spend_events(event_id, user_id, campaign_id, promo_generation, spend_delta)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (event_id) DO NOTHING
	`, req.EventID, req.UserID, req.CampaignID, req.PromoGeneration, req.SpendDelta)
	if err != nil {
		return PromoState{}, fmt.Errorf("insert promo spend marker: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return PromoState{}, fmt.Errorf("read promo spend marker result: %w", err)
	}

	if inserted == 0 {
		var marker int
		if err := tx.QueryRowContext(ctx, `
			SELECT 1
			FROM adv_promo_spend_events
			WHERE event_id = $1
			  AND user_id = $2
			  AND campaign_id = $3
			  AND promo_generation = $4
			  AND spend_delta = $5
		`, req.EventID, req.UserID, req.CampaignID, req.PromoGeneration, req.SpendDelta).Scan(&marker); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return PromoState{}, fmt.Errorf("%w: event_id %q already exists with different user/campaign/generation/spend", ErrEventPayloadConflict, req.EventID)
			}
			return PromoState{}, fmt.Errorf("verify existing promo spend marker: %w", err)
		}
		state, err := readPromoState(ctx, tx, req.UserID, false)
		if err != nil {
			return PromoState{}, err
		}
		if err := tx.Commit(); err != nil {
			return PromoState{}, fmt.Errorf("commit replayed promo spend transaction: %w", err)
		}
		return state, nil
	}

	state, err := readPromoState(ctx, tx, req.UserID, true)
	if err != nil {
		return PromoState{}, err
	}

	// Generation mismatch is an intentional no-op. The event is still committed
	// to the ledger so every retry of the same stable event_id returns the same
	// idempotent class of result and can never start consuming a later grant.
	if state.Generation == req.PromoGeneration && state.Remaining > 0 {
		var rawRemaining string
		var revision, generation int64
		if err := tx.QueryRowContext(ctx, `
			UPDATE users
			SET promo_spend_remaining = GREATEST(0, promo_spend_remaining - $2),
			    promo_revision = promo_revision + 1,
			    updated_at = NOW()
			WHERE id = $1::uuid
			RETURNING promo_spend_remaining::text, promo_revision, promo_generation
		`, req.UserID, req.SpendDelta).Scan(&rawRemaining, &revision, &generation); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return PromoState{}, fmt.Errorf("promo user %q not found", req.UserID)
			}
			return PromoState{}, fmt.Errorf("decrement promo state: %w", err)
		}
		state, err = parsePromoState(rawRemaining, revision, generation)
		if err != nil {
			return PromoState{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return PromoState{}, fmt.Errorf("commit promo spend transaction: %w", err)
	}
	return state, nil
}

func readPromoState(ctx context.Context, tx *sql.Tx, userID string, forUpdate bool) (PromoState, error) {
	query := `
		SELECT COALESCE(promo_spend_remaining, 0)::text, promo_revision, promo_generation
		FROM users
		WHERE id = $1::uuid`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var rawRemaining string
	var revision, generation int64
	if err := tx.QueryRowContext(ctx, query, userID).Scan(&rawRemaining, &revision, &generation); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PromoState{}, fmt.Errorf("promo user %q not found", userID)
		}
		return PromoState{}, fmt.Errorf("read promo state: %w", err)
	}
	return parsePromoState(rawRemaining, revision, generation)
}

func parsePromoState(rawRemaining string, revision, generation int64) (PromoState, error) {
	remaining, err := strconv.ParseFloat(strings.TrimSpace(rawRemaining), 64)
	if err != nil || remaining < 0 || math.IsNaN(remaining) || math.IsInf(remaining, 0) {
		return PromoState{}, fmt.Errorf("invalid promo_spend_remaining %q", rawRemaining)
	}
	if revision < 0 {
		return PromoState{}, fmt.Errorf("invalid promo_revision %d", revision)
	}
	if generation < 0 {
		return PromoState{}, fmt.Errorf("invalid promo_generation %d", generation)
	}
	return PromoState{Remaining: remaining, Revision: revision, Generation: generation}, nil
}

// CleanupRetiredPromoSpendEvents removes only ledger rows that are provably
// unable to debit promo again. Rows from older generations are stale forever;
// rows from the current generation are safe once that generation is exhausted.
// Active-current-generation rows are deliberately retained regardless of age,
// preserving idempotency for arbitrarily late retries.
func (r *Repository) CleanupRetiredPromoSpendEvents(ctx context.Context, olderThan time.Duration, limit int) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("percenter billing PostgreSQL is nil")
	}
	if olderThan < 0 {
		return 0, errors.New("promo spend cleanup olderThan must be non-negative")
	}
	if limit <= 0 {
		return 0, errors.New("promo spend cleanup limit must be positive")
	}
	cutoff := time.Now().Add(-olderThan)
	result, err := r.db.ExecContext(ctx, `
		WITH retired AS (
			SELECT e.ctid
			FROM adv_promo_spend_events e
			JOIN users u ON e.user_id = u.id::text
			WHERE e.applied_at < $1
			  AND (
				e.promo_generation < u.promo_generation
				OR (
					e.promo_generation = u.promo_generation
					AND u.promo_spend_remaining <= 0
				)
			  )
			ORDER BY e.applied_at
			LIMIT $2
		)
		DELETE FROM adv_promo_spend_events e
		USING retired r
		WHERE e.ctid = r.ctid
	`, cutoff, limit)
	if err != nil {
		return 0, fmt.Errorf("cleanup retired promo spend events: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read promo spend cleanup result: %w", err)
	}
	return deleted, nil
}

func validateApplyRequest(req ApplyRequest) error {
	if strings.TrimSpace(req.EventID) == "" {
		return errors.New("event_id is required")
	}
	if strings.TrimSpace(req.UserID) == "" {
		return errors.New("user_id is required")
	}
	if strings.TrimSpace(req.CampaignID) == "" {
		return errors.New("campaign_id is required")
	}
	if req.PromoGeneration < 0 {
		return errors.New("promo_generation must be non-negative")
	}
	if req.SpendDelta <= 0 || math.IsNaN(req.SpendDelta) || math.IsInf(req.SpendDelta, 0) {
		return errors.New("spend_delta must be a finite positive number")
	}
	return nil
}

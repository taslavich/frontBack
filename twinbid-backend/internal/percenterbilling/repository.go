package percenterbilling

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Repository struct {
	db *sql.DB
}

var ErrEventPayloadConflict = errors.New("promo spend event payload conflict")

type ApplyRequest struct {
	EventID    string  `json:"event_id"`
	UserID     string  `json:"user_id"`
	CampaignID string  `json:"campaign_id"`
	SpendDelta float64 `json:"spend_delta"`
}

type PromoState struct {
	Remaining float64 `json:"remaining"`
	Revision  int64   `json:"revision"`
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// ApplyPromoSpend is the authoritative, idempotent promo mutation. It is owned
// by the cabinet backend because users.promo_spend_remaining/promo_revision are
// cabinet PostgreSQL state. Replays of the same event_id return the current
// authoritative state without decrementing promo twice.
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
		INSERT INTO adv_promo_spend_events(event_id, user_id, campaign_id, spend_delta)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (event_id) DO NOTHING
	`, req.EventID, req.UserID, req.CampaignID, req.SpendDelta)
	if err != nil {
		return PromoState{}, fmt.Errorf("insert promo spend marker: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return PromoState{}, fmt.Errorf("read promo spend marker result: %w", err)
	}

	var rawRemaining string
	var revision int64
	if inserted == 0 {
		var marker int
		if err := tx.QueryRowContext(ctx, `
			SELECT 1
			FROM adv_promo_spend_events
			WHERE event_id = $1
			  AND user_id = $2
			  AND campaign_id = $3
			  AND spend_delta = $4
		`, req.EventID, req.UserID, req.CampaignID, req.SpendDelta).Scan(&marker); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return PromoState{}, fmt.Errorf("%w: event_id %q already exists with different user/campaign/spend", ErrEventPayloadConflict, req.EventID)
			}
			return PromoState{}, fmt.Errorf("verify existing promo spend marker: %w", err)
		}
		if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(promo_spend_remaining, 0)::text, promo_revision
			FROM users
			WHERE id = $1::uuid
		`, req.UserID).Scan(&rawRemaining, &revision); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return PromoState{}, fmt.Errorf("promo user %q not found", req.UserID)
			}
			return PromoState{}, fmt.Errorf("read existing promo state: %w", err)
		}
	} else {
		if err := tx.QueryRowContext(ctx, `
			UPDATE users
			SET promo_spend_remaining = GREATEST(0, promo_spend_remaining - $2),
			    promo_revision = promo_revision + CASE
			        WHEN promo_spend_remaining > 0 THEN 1
			        ELSE 0
			    END,
			    updated_at = NOW()
			WHERE id = $1::uuid
			RETURNING promo_spend_remaining::text, promo_revision
		`, req.UserID, req.SpendDelta).Scan(&rawRemaining, &revision); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return PromoState{}, fmt.Errorf("promo user %q not found", req.UserID)
			}
			return PromoState{}, fmt.Errorf("decrement promo state: %w", err)
		}
	}

	remaining, err := strconv.ParseFloat(strings.TrimSpace(rawRemaining), 64)
	if err != nil || remaining < 0 || math.IsNaN(remaining) || math.IsInf(remaining, 0) {
		return PromoState{}, fmt.Errorf("invalid promo_spend_remaining %q", rawRemaining)
	}
	if revision < 0 {
		return PromoState{}, fmt.Errorf("invalid promo_revision %d", revision)
	}
	if err := tx.Commit(); err != nil {
		return PromoState{}, fmt.Errorf("commit promo spend transaction: %w", err)
	}
	return PromoState{Remaining: remaining, Revision: revision}, nil
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
	if req.SpendDelta <= 0 || math.IsNaN(req.SpendDelta) || math.IsInf(req.SpendDelta, 0) {
		return errors.New("spend_delta must be a finite positive number")
	}
	return nil
}

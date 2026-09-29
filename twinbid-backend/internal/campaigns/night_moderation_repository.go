package campaigns

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
)

type NightModerationState struct {
	WindowStart     time.Time
	ManualDecision  string
	ManualDecidedAt *time.Time
}

func (r *Repository) RotateNightModerationWindow(ctx context.Context, windowStart time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM night_autoapproved_campaigns
		WHERE window_start < $1
	`, windowStart.UTC())
	return err
}

func (r *Repository) ListModerationCampaignIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT campaign_id
		FROM campaigns
		WHERE status = 'moderation'
		ORDER BY updated_at, campaign_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// AutoApproveNightCampaign atomically remembers the campaign for this night
// window and changes moderation -> waiting. Repeated/concurrent calls are safe.
func (r *Repository) AutoApproveNightCampaign(ctx context.Context, campaignID string, windowStart time.Time) (models.Campaign, bool, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return models.Campaign{}, false, fmt.Errorf("begin night auto-approve transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := scanCampaign(tx.QueryRowContext(ctx, baseCampaignSelect+` WHERE campaign_id = $1 FOR UPDATE`, campaignID))
	if errors.Is(err, sql.ErrNoRows) {
		return models.Campaign{}, false, httpx.NotFound("campaign not found")
	}
	if err != nil {
		return models.Campaign{}, false, fmt.Errorf("lock campaign for night auto-approve: %w", err)
	}
	if current.Status != "moderation" {
		if err := tx.Commit(); err != nil {
			return models.Campaign{}, false, fmt.Errorf("commit night auto-approve no-op: %w", err)
		}
		return current, false, nil
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO night_autoapproved_campaigns (
			campaign_id, window_start, autoapproved_at, manual_decision, manual_decided_at
		) VALUES ($1, $2, NOW(), NULL, NULL)
		ON CONFLICT (campaign_id) DO UPDATE SET
			window_start = EXCLUDED.window_start,
			autoapproved_at = NOW(),
			manual_decision = NULL,
			manual_decided_at = NULL
	`, campaignID, windowStart.UTC()); err != nil {
		return models.Campaign{}, false, fmt.Errorf("remember night auto-approved campaign: %w", err)
	}

	current.Status = "waiting"
	updated, err := updateCampaignRow(ctx, tx, current)
	if err != nil {
		return models.Campaign{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return models.Campaign{}, false, fmt.Errorf("commit night auto-approve: %w", err)
	}
	return updated, true, nil
}

// UpdateLockedWithNightModeration locks campaign first and then its optional
// night record. Keeping that lock order aligned with AutoApproveNightCampaign
// prevents campaign/night-row deadlocks under concurrent Telegram callbacks.
func (r *Repository) UpdateLockedWithNightModeration(
	ctx context.Context,
	campaignID string,
	apply func(current *models.Campaign, night *NightModerationState) error,
) (models.Campaign, error) {
	if apply == nil {
		return models.Campaign{}, errors.New("campaign moderation callback is nil")
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return models.Campaign{}, fmt.Errorf("begin campaign moderation transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := scanCampaign(tx.QueryRowContext(ctx, baseCampaignSelect+` WHERE campaign_id = $1 FOR UPDATE`, campaignID))
	if errors.Is(err, sql.ErrNoRows) {
		return models.Campaign{}, httpx.NotFound("campaign not found")
	}
	if err != nil {
		return models.Campaign{}, fmt.Errorf("lock campaign for moderation: %w", err)
	}

	var night *NightModerationState
	var windowStart time.Time
	var manualDecision sql.NullString
	var manualDecidedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT window_start, manual_decision, manual_decided_at
		FROM night_autoapproved_campaigns
		WHERE campaign_id = $1
		FOR UPDATE
	`, campaignID).Scan(&windowStart, &manualDecision, &manualDecidedAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		night = nil
	case err != nil:
		return models.Campaign{}, fmt.Errorf("lock night moderation state: %w", err)
	default:
		night = &NightModerationState{WindowStart: windowStart.UTC()}
		if manualDecision.Valid {
			night.ManualDecision = manualDecision.String
		}
		if manualDecidedAt.Valid {
			t := manualDecidedAt.Time.UTC()
			night.ManualDecidedAt = &t
		}
	}

	if err := apply(&current, night); err != nil {
		return models.Campaign{}, err
	}

	updated, err := updateCampaignRow(ctx, tx, current)
	if err != nil {
		return models.Campaign{}, err
	}
	if night != nil {
		if _, err := tx.ExecContext(ctx, `
			UPDATE night_autoapproved_campaigns
			SET manual_decision = NULLIF($2, ''), manual_decided_at = $3
			WHERE campaign_id = $1
		`, campaignID, night.ManualDecision, night.ManualDecidedAt); err != nil {
			return models.Campaign{}, fmt.Errorf("persist night moderation decision: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return models.Campaign{}, fmt.Errorf("commit campaign moderation: %w", err)
	}
	return updated, nil
}

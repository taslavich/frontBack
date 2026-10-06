package poprecovery

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

type Adjustment struct {
	ID         string
	UserID     string
	CampaignID string
	Delta      string
	DeltaFloat float64
}

func (a Adjustment) Validate() error {
	if strings.TrimSpace(a.ID) == "" || strings.TrimSpace(a.UserID) == "" || strings.TrimSpace(a.CampaignID) == "" {
		return errors.New("invalid POP recovery adjustment identity")
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(a.Delta), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return fmt.Errorf("invalid POP recovery adjustment delta %q", a.Delta)
	}
	if math.Abs(value-a.DeltaFloat) > math.Max(1, math.Abs(value))*1e-12 {
		return fmt.Errorf("POP recovery adjustment delta mismatch text=%q float=%.17g", a.Delta, a.DeltaFloat)
	}
	return nil
}

type applier interface {
	Apply(ctx context.Context, adjustment Adjustment) (alreadyApplied bool, err error)
}

type Health struct {
	BootstrapCompleted     bool
	EventCursorInitialized bool
	EventCursorAtMS        int64
	EventCursorSourceKey   string
	PendingAdjustments     int64
	LastError              string
	LastErrorAt            sql.NullTime
}

type RunResult struct {
	Health         Health
	PendingLoaded  int
	Applied        int
	AlreadyApplied int
}

type Reconciler struct {
	postgres  *sql.DB
	applier   applier
	batchSize int
}

func NewReconciler(postgres *sql.DB, applier applier, batchSize int) *Reconciler {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &Reconciler{postgres: postgres, applier: applier, batchSize: batchSize}
}

func (r *Reconciler) Health(ctx context.Context) (Health, error) {
	if r == nil || r.postgres == nil {
		return Health{}, errors.New("POP recovery PostgreSQL is nil")
	}
	var health Health
	if err := r.postgres.QueryRowContext(ctx, `
SELECT
    s.bootstrap_completed,
    s.event_cursor_initialized,
    s.last_event_recovery_at_ms,
    s.last_event_source_key,
    (SELECT count(*) FROM pop_recovery_adjustments WHERE status = 'pending'),
    COALESCE(s.last_error, ''),
    s.last_error_at
FROM pop_recovery_reconciliation_state AS s
WHERE s.id = 1`).Scan(
		&health.BootstrapCompleted,
		&health.EventCursorInitialized,
		&health.EventCursorAtMS,
		&health.EventCursorSourceKey,
		&health.PendingAdjustments,
		&health.LastError,
		&health.LastErrorAt,
	); err != nil {
		return Health{}, fmt.Errorf("read POP recovery health: %w", err)
	}
	return health, nil
}

func (r *Reconciler) RunOnce(ctx context.Context) (RunResult, error) {
	if r == nil || r.postgres == nil {
		return RunResult{}, errors.New("POP recovery PostgreSQL is nil")
	}
	if r.applier == nil {
		return RunResult{}, errors.New("POP recovery Redis applier is nil")
	}

	health, err := r.Health(ctx)
	if err != nil {
		return RunResult{}, err
	}
	result := RunResult{Health: health}
	if !health.BootstrapCompleted || !health.EventCursorInitialized {
		return result, nil
	}

	adjustments, err := r.pending(ctx)
	if err != nil {
		_ = r.recordStateError(ctx, err)
		return result, err
	}
	result.PendingLoaded = len(adjustments)

	for _, adjustment := range adjustments {
		alreadyApplied, applyErr := r.applier.Apply(ctx, adjustment)
		if applyErr != nil {
			_ = r.recordAdjustmentError(ctx, adjustment.ID, applyErr)
			_ = r.recordStateError(ctx, applyErr)
			return result, fmt.Errorf("apply POP recovery adjustment %s: %w", adjustment.ID, applyErr)
		}

		if err := r.markApplied(ctx, adjustment.ID); err != nil {
			// Redis may already have committed both spend increments and the marker.
			// Do not attempt compensation. A later run observes the persistent marker
			// and only completes this PostgreSQL status transition.
			_ = r.recordStateError(ctx, err)
			return result, fmt.Errorf("mark POP recovery adjustment %s applied: %w", adjustment.ID, err)
		}
		result.Applied++
		if alreadyApplied {
			result.AlreadyApplied++
		}
	}

	if len(adjustments) > 0 {
		health, err := r.Health(ctx)
		if err != nil {
			_ = r.recordStateError(ctx, err)
			return result, err
		}
		result.Health = health
	}
	return result, nil
}

func (r *Reconciler) pending(ctx context.Context) ([]Adjustment, error) {
	rows, err := r.postgres.QueryContext(ctx, `
SELECT adjustment_id::text, user_id::text, campaign_id::text, delta::text
FROM pop_recovery_adjustments
WHERE status = 'pending'
ORDER BY created_at, adjustment_id
LIMIT $1`, r.batchSize)
	if err != nil {
		return nil, fmt.Errorf("query pending POP recovery adjustments: %w", err)
	}
	defer rows.Close()

	out := make([]Adjustment, 0, r.batchSize)
	for rows.Next() {
		var adjustment Adjustment
		if err := rows.Scan(&adjustment.ID, &adjustment.UserID, &adjustment.CampaignID, &adjustment.Delta); err != nil {
			return nil, fmt.Errorf("scan pending POP recovery adjustment: %w", err)
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(adjustment.Delta), 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			return nil, fmt.Errorf("invalid pending POP recovery adjustment %s delta %q", adjustment.ID, adjustment.Delta)
		}
		adjustment.DeltaFloat = value
		out = append(out, adjustment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending POP recovery adjustments: %w", err)
	}
	return out, nil
}

func (r *Reconciler) markApplied(ctx context.Context, adjustmentID string) error {
	result, err := r.postgres.ExecContext(ctx, `
UPDATE pop_recovery_adjustments
SET status = 'applied',
    applied_at = COALESCE(applied_at, NOW()),
    last_error = NULL,
    last_attempt_at = NOW(),
    attempt_count = attempt_count + 1
WHERE adjustment_id = $1
  AND status = 'pending'`, adjustmentID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows > 1 {
		return fmt.Errorf("unexpected applied row count %d", rows)
	}
	return nil
}

func (r *Reconciler) recordAdjustmentError(ctx context.Context, adjustmentID string, applyErr error) error {
	if applyErr == nil {
		return nil
	}
	_, err := r.postgres.ExecContext(ctx, `
UPDATE pop_recovery_adjustments
SET attempt_count = attempt_count + 1,
    last_attempt_at = NOW(),
    last_error = $2
WHERE adjustment_id = $1
  AND status = 'pending'`, adjustmentID, applyErr.Error())
	return err
}

func (r *Reconciler) recordStateError(ctx context.Context, reconcileErr error) error {
	if reconcileErr == nil {
		return nil
	}
	_, err := r.postgres.ExecContext(ctx, `
UPDATE pop_recovery_reconciliation_state
SET last_error = $1,
    last_error_at = NOW(),
    updated_at = NOW()
WHERE id = 1`, reconcileErr.Error())
	return err
}

func FormatHealthLog(health Health) string {
	lastErrorAt := ""
	if health.LastErrorAt.Valid {
		lastErrorAt = health.LastErrorAt.Time.UTC().Format(time.RFC3339)
	}
	return fmt.Sprintf(
		"bootstrap_completed=%t event_cursor_initialized=%t event_cursor_at_ms=%d event_cursor_source_key=%q pending_adjustments=%d last_error=%q last_error_at=%q",
		health.BootstrapCompleted,
		health.EventCursorInitialized,
		health.EventCursorAtMS,
		health.EventCursorSourceKey,
		health.PendingAdjustments,
		health.LastError,
		lastErrorAt,
	)
}

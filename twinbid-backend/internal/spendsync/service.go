package spendsync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"twinbid-backend/internal/stats"
)

type source interface {
	CumulativeSpend(ctx context.Context) ([]stats.CumulativeSpendTotal, error)
	POPRecoveryEventsAfter(ctx context.Context, cursor stats.POPRecoveryCursor, limit int) ([]stats.POPRecoveryEvent, error)
}

type Service struct {
	postgres *sql.DB
	source   source
}

type Result struct {
	SourceRows                     int
	UserTotals                     int
	CampaignTotals                 int
	UpdatedUsers                   int64
	UpdatedCampaigns               int64
	StoppedCampaigns               int64
	SkippedInvalidEntityRows       int
	InvalidEntityIDSamples         []string
	RecoverySourceRows             int
	RecoveryAdjustmentsCreated     int64
	RecoveryBootstrapCompleted     bool
	RecoveryEventCursorInitialized bool
	RecoveryAnomalies              []RecoveryAnomaly
	ClickHouseQueryDuration        time.Duration
}

type RecoveryAnomaly struct {
	UserID      string
	CampaignID  string
	CursorTotal string
	SourceTotal string
	Reason      string
}

type normalizedRecoveryTotal struct {
	UserID     string
	CampaignID string
	Amount     string
	value      *big.Rat
}

type recoveryCursor struct {
	UserID string
	Amount string
	value  *big.Rat
}

// eventRecoveryState is the one global ordered marker for the immutable
// ClickHouse pop_recovery_events stream. It replaces the old per-campaign
// cumulative money cursors for all new reconciliation work.
type eventRecoveryState struct {
	BootstrapCompleted     bool
	EventCursorInitialized bool
	RecoveryAtMS           int64
	SourceKey              string
}

type normalizedRecoveryEventGroup struct {
	UserID     string
	CampaignID string
	Delta      string
	value      *big.Rat
}

const recoveryEventFetchLimit = 5000

const updateUsersCumulativeSpendSQL = `
WITH incoming(id, cum_done_dollars) AS (
    SELECT * FROM unnest($1::uuid[], $2::numeric[])
)
UPDATE users AS u
SET promo_spend_remaining = GREATEST(
        0,
        u.promo_spend_remaining - GREATEST(incoming.cum_done_dollars - u.promo_spend_synced, 0)
    ),
    promo_spend_synced = GREATEST(u.promo_spend_synced, incoming.cum_done_dollars),
    cum_done_dollars = incoming.cum_done_dollars
FROM incoming
WHERE u.id = incoming.id`

const updateCampaignsCumulativeSpendSQL = `
WITH incoming(id, cum_done_dollars) AS (
    SELECT * FROM unnest($1::uuid[], $2::numeric[])
)
UPDATE campaigns AS c
SET cum_done_dollars = incoming.cum_done_dollars
FROM incoming
WHERE c.campaign_id = incoming.id`

// markNoBudgetCampaignsSQL intentionally runs in the same PostgreSQL transaction
// as the ClickHouse -> PostgreSQL cumulative-spend reconciliation. Once fresh spend
// has been written to campaigns.cum_done_dollars, there is no second polling window
// before the campaign status becomes no_budget.
//
// no_budget_notified is deliberately left false here. Notifications are handled by
// a separate non-critical worker so SMTP/network stalls can never delay the status.
const markNoBudgetCampaignsSQL = `
UPDATE campaigns AS c
SET status = 'no_budget', updated_at = NOW()
WHERE c.goal_total_dollars > 0
  AND c.status = 'active'
  AND (c.goal_total_dollars - c.cum_done_dollars) <
      CASE
          WHEN LOWER(TRIM(c.pricing_model)) = 'cpm'
              THEN c.base_price / 1000
          WHEN LOWER(TRIM(c.pricing_model)) = 'cpc'
               AND LOWER(TRIM(c.format_type)) = 'popunder'
              THEN c.base_price / 1000
          WHEN LOWER(TRIM(c.pricing_model)) = 'cpc'
              THEN c.base_price
          ELSE NULL
      END`

func NewService(postgres *sql.DB, source source) *Service {
	return &Service{postgres: postgres, source: source}
}

func (s *Service) Sync(ctx context.Context) (Result, error) {
	if s == nil || s.postgres == nil {
		return Result{}, errors.New("spend sync PostgreSQL is nil")
	}
	if s.source == nil {
		return Result{}, errors.New("spend sync ClickHouse source is nil")
	}

	var result Result
	ordinaryQueryStartedAt := time.Now()
	totals, err := s.source.CumulativeSpend(ctx)
	result.ClickHouseQueryDuration += time.Since(ordinaryQueryStartedAt)
	for _, total := range totals {
		if !total.DiagnosticOnly {
			result.SourceRows++
		}
	}
	if err != nil {
		return result, fmt.Errorf("query ClickHouse cumulative spend: %w", err)
	}

	userIDs, userAmounts, campaignIDs, campaignAmounts, skippedInvalidEntityRows, invalidEntityIDSamples, err := splitTotals(totals)
	if err != nil {
		return result, err
	}
	result.SkippedInvalidEntityRows = skippedInvalidEntityRows
	result.InvalidEntityIDSamples = invalidEntityIDSamples
	if skippedInvalidEntityRows > 0 {
		log.Printf(
			"[SPEND_SYNC][INVALID_NON_INTERNAL_ENTITY] skipped_rows=%d samples=%v",
			skippedInvalidEntityRows,
			invalidEntityIDSamples,
		)
	}
	result.UserTotals = len(userIDs)
	result.CampaignTotals = len(campaignIDs)

	// Keep the pre-existing cumulative-spend / no-budget transaction independent
	// from the new POP-recovery reconciliation. A failure of the additive recovery
	// path must not roll back or delay ordinary billing synchronization.
	if len(userIDs) > 0 || len(campaignIDs) > 0 {
		if err := s.syncOrdinaryTotalsTx(ctx, userIDs, userAmounts, campaignIDs, campaignAmounts, &result); err != nil {
			return result, err
		}
	}

	state, err := s.readEventRecoveryState(ctx)
	if err != nil {
		return result, err
	}
	result.RecoveryBootstrapCompleted = state.BootstrapCompleted
	result.RecoveryEventCursorInitialized = state.EventCursorInitialized
	if !state.BootstrapCompleted || !state.EventCursorInitialized {
		return result, nil
	}

	recoveryQueryStartedAt := time.Now()
	events, err := s.source.POPRecoveryEventsAfter(ctx, stats.POPRecoveryCursor{
		RecoveryAtMS: state.RecoveryAtMS,
		SourceKey:    state.SourceKey,
	}, recoveryEventFetchLimit)
	result.ClickHouseQueryDuration += time.Since(recoveryQueryStartedAt)
	result.RecoverySourceRows = len(events)
	if err != nil {
		return result, fmt.Errorf("query ClickHouse POP recovery events: %w", err)
	}
	if len(events) == 0 {
		return result, nil
	}

	groups, endCursor, err := normalizeRecoveryEvents(
		events,
		stats.POPRecoveryCursor{RecoveryAtMS: state.RecoveryAtMS, SourceKey: state.SourceKey},
	)
	if err != nil {
		return result, err
	}
	if err := s.syncRecoveryEventsTx(ctx, state, groups, endCursor, &result); err != nil {
		return result, err
	}
	return result, nil
}

func (s *Service) syncOrdinaryTotalsTx(
	ctx context.Context,
	userIDs, userAmounts, campaignIDs, campaignAmounts []string,
	result *Result,
) error {
	tx, err := s.postgres.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ordinary spend sync transaction: %w", err)
	}
	defer tx.Rollback()

	if len(userIDs) > 0 {
		// ClickHouse cumulative spend is the sole spend source for PostgreSQL promo
		// consumption. promo_spend_synced is a monotonic per-user baseline, so each
		// cumulative increment is applied to promo_spend_remaining exactly once.
		// A delayed ClickHouse increment is intentionally charged whenever it
		// becomes visible, even if a new promo grant was credited in the meantime.
		execResult, err := tx.ExecContext(ctx, updateUsersCumulativeSpendSQL, pq.Array(userIDs), pq.Array(userAmounts))
		if err != nil {
			return fmt.Errorf("bulk update users cumulative spend: %w", err)
		}
		result.UpdatedUsers, err = execResult.RowsAffected()
		if err != nil {
			return fmt.Errorf("users cumulative spend rows affected: %w", err)
		}
	}

	if len(campaignIDs) > 0 {
		execResult, err := tx.ExecContext(ctx, updateCampaignsCumulativeSpendSQL, pq.Array(campaignIDs), pq.Array(campaignAmounts))
		if err != nil {
			return fmt.Errorf("bulk update campaigns cumulative spend: %w", err)
		}
		result.UpdatedCampaigns, err = execResult.RowsAffected()
		if err != nil {
			return fmt.Errorf("campaigns cumulative spend rows affected: %w", err)
		}
	}

	// Status transition remains in the same transaction as the ordinary spend
	// update, exactly as before Patch 2.
	statusResult, err := tx.ExecContext(ctx, markNoBudgetCampaignsSQL)
	if err != nil {
		return fmt.Errorf("mark no-budget campaigns: %w", err)
	}
	result.StoppedCampaigns, err = statusResult.RowsAffected()
	if err != nil {
		return fmt.Errorf("no-budget campaigns rows affected: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit ordinary spend sync transaction: %w", err)
	}
	return nil
}

func (s *Service) syncRecoveryTotalsTx(ctx context.Context, totals []normalizedRecoveryTotal, result *Result) error {
	tx, err := s.postgres.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin POP recovery transaction: %w", err)
	}
	defer tx.Rollback()

	if err := processPOPRecoveryTotalsTx(ctx, tx, totals, result); err != nil {
		return fmt.Errorf("process POP recovery deltas: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit POP recovery transaction: %w", err)
	}
	return nil
}

func canonicalEntityUUID(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if len(value) != 36 {
		return "", false
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return "", false
	}
	canonical := parsed.String()
	if value != canonical {
		return "", false
	}
	return canonical, true
}

func splitTotals(totals []stats.CumulativeSpendTotal) ([]string, []string, []string, []string, int, []string, error) {
	userAmountsByID := make(map[string]string)
	campaignAmountsByID := make(map[string]string)
	skippedInvalidEntityRows := 0
	invalidEntityIDSamples := make([]string, 0, 5)

	for i, total := range totals {
		entityType := strings.ToLower(strings.TrimSpace(total.EntityType))
		entityID := strings.TrimSpace(total.EntityID)
		amount := strings.TrimSpace(total.Amount)

		if total.DiagnosticOnly {
			skippedInvalidEntityRows++
			if len(invalidEntityIDSamples) < 5 {
				invalidEntityIDSamples = append(
					invalidEntityIDSamples,
					fmt.Sprintf("filtered_by_strict_ch_uuid_guard row=%d type=%q entity_id=%q", i, total.EntityType, total.EntityID),
				)
			}
			continue
		}

		canonicalEntityID, ok := canonicalEntityUUID(entityID)
		if !ok {
			skippedInvalidEntityRows++
			if len(invalidEntityIDSamples) < 5 {
				invalidEntityIDSamples = append(
					invalidEntityIDSamples,
					fmt.Sprintf("row=%d type=%q entity_id=%q", i, total.EntityType, total.EntityID),
				)
			}
			continue
		}
		entityID = canonicalEntityID
		value, err := strconv.ParseFloat(amount, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return nil, nil, nil, nil, skippedInvalidEntityRows, invalidEntityIDSamples, fmt.Errorf("invalid ClickHouse cumulative spend amount at row %d: %q", i, amount)
		}

		switch entityType {
		case "user":
			userAmountsByID[entityID] = amount
		case "campaign":
			campaignAmountsByID[entityID] = amount
		default:
			return nil, nil, nil, nil, skippedInvalidEntityRows, invalidEntityIDSamples, fmt.Errorf("invalid ClickHouse cumulative spend entity_type at row %d: %q", i, total.EntityType)
		}
	}

	userIDs, userAmounts := sortedTotals(userAmountsByID)
	campaignIDs, campaignAmounts := sortedTotals(campaignAmountsByID)
	return userIDs, userAmounts, campaignIDs, campaignAmounts, skippedInvalidEntityRows, invalidEntityIDSamples, nil
}

func sortedTotals(amountsByID map[string]string) ([]string, []string) {
	ids := make([]string, 0, len(amountsByID))
	for id := range amountsByID {
		ids = append(ids, id)
	}
	// Stable ordering makes logs, tests and PostgreSQL array arguments deterministic.
	sort.Strings(ids)

	amounts := make([]string, 0, len(ids))
	for _, id := range ids {
		amounts = append(amounts, amountsByID[id])
	}
	return ids, amounts
}

func (s *Service) readEventRecoveryState(ctx context.Context) (eventRecoveryState, error) {
	var state eventRecoveryState
	if err := s.postgres.QueryRowContext(ctx, `
SELECT bootstrap_completed,
       event_cursor_initialized,
       last_event_recovery_at_ms,
       last_event_source_key
FROM pop_recovery_reconciliation_state
WHERE id = 1`).Scan(
		&state.BootstrapCompleted,
		&state.EventCursorInitialized,
		&state.RecoveryAtMS,
		&state.SourceKey,
	); err != nil {
		return eventRecoveryState{}, fmt.Errorf("read POP recovery event cursor: %w", err)
	}
	return state, nil
}

func recoveryCursorLess(aAt int64, aKey string, bAt int64, bKey string) bool {
	return aAt < bAt || (aAt == bAt && aKey < bKey)
}

func normalizeRecoveryEvents(events []stats.POPRecoveryEvent, start stats.POPRecoveryCursor) ([]normalizedRecoveryEventGroup, stats.POPRecoveryCursor, error) {
	if len(events) == 0 {
		return nil, start, nil
	}

	type aggregate struct {
		UserID string
		value  *big.Rat
	}
	byCampaign := make(map[string]aggregate)
	previous := start
	end := start

	for i, event := range events {
		key := strings.TrimSpace(event.SourceKey)
		if key == "" || !recoveryCursorLess(previous.RecoveryAtMS, previous.SourceKey, event.RecoveryAtMS, key) {
			return nil, start, fmt.Errorf(
				"POP recovery events are not strictly ordered after cursor at row %d: previous=(%d,%q) current=(%d,%q)",
				i, previous.RecoveryAtMS, previous.SourceKey, event.RecoveryAtMS, key,
			)
		}
		previous = stats.POPRecoveryCursor{RecoveryAtMS: event.RecoveryAtMS, SourceKey: key}
		end = previous

		userID, userOK := canonicalEntityUUID(event.UserID)
		campaignID, campaignOK := canonicalEntityUUID(event.CampaignID)
		if !userOK || !campaignOK {
			// ClickHouse applies the same canonical UUID filter. Keep this Go guard as
			// defense in depth, but advance the global source marker past a raw
			// external/malformed event so it can never poison future scans.
			log.Printf(
				"[POP_RECOVERY][INVALID_EVENT_ENTITY] row=%d marker=(%d,%q) user_id=%q campaign_id=%q",
				i, event.RecoveryAtMS, key, event.UserID, event.CampaignID,
			)
			continue
		}
		value, ok := new(big.Rat).SetString(strings.TrimSpace(event.Amount))
		if !ok || value.Sign() <= 0 {
			return nil, start, fmt.Errorf("invalid ClickHouse POP recovery event amount at row %d: %q", i, event.Amount)
		}

		current, exists := byCampaign[campaignID]
		if exists && current.UserID != userID {
			return nil, start, fmt.Errorf(
				"POP recovery campaign %s changed user inside one source batch: %s -> %s",
				campaignID, current.UserID, userID,
			)
		}
		if !exists {
			current = aggregate{UserID: userID, value: new(big.Rat)}
		}
		current.value.Add(current.value, value)
		byCampaign[campaignID] = current
	}

	campaignIDs := make([]string, 0, len(byCampaign))
	for campaignID := range byCampaign {
		campaignIDs = append(campaignIDs, campaignID)
	}
	sort.Strings(campaignIDs)
	groups := make([]normalizedRecoveryEventGroup, 0, len(campaignIDs))
	for _, campaignID := range campaignIDs {
		agg := byCampaign[campaignID]
		groups = append(groups, normalizedRecoveryEventGroup{
			UserID:     agg.UserID,
			CampaignID: campaignID,
			Delta:      decimalString(agg.value),
			value:      agg.value,
		})
	}
	return groups, end, nil
}

func (s *Service) syncRecoveryEventsTx(
	ctx context.Context,
	expected eventRecoveryState,
	groups []normalizedRecoveryEventGroup,
	end stats.POPRecoveryCursor,
	result *Result,
) error {
	tx, err := s.postgres.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin POP recovery event transaction: %w", err)
	}
	defer tx.Rollback()

	var current eventRecoveryState
	if err := tx.QueryRowContext(ctx, `
SELECT bootstrap_completed,
       event_cursor_initialized,
       last_event_recovery_at_ms,
       last_event_source_key
FROM pop_recovery_reconciliation_state
WHERE id = 1
FOR UPDATE`).Scan(
		&current.BootstrapCompleted,
		&current.EventCursorInitialized,
		&current.RecoveryAtMS,
		&current.SourceKey,
	); err != nil {
		return fmt.Errorf("lock POP recovery event cursor: %w", err)
	}
	if !current.BootstrapCompleted || !current.EventCursorInitialized {
		return nil
	}
	if current.RecoveryAtMS != expected.RecoveryAtMS || current.SourceKey != expected.SourceKey {
		// Another sync invocation advanced the source marker while this cycle was
		// reading ClickHouse. Do not create duplicate adjustments; the next cycle
		// will query again from the newer marker.
		return nil
	}
	if !recoveryCursorLess(current.RecoveryAtMS, current.SourceKey, end.RecoveryAtMS, end.SourceKey) {
		return fmt.Errorf(
			"POP recovery end cursor did not advance: current=(%d,%q) end=(%d,%q)",
			current.RecoveryAtMS, current.SourceKey, end.RecoveryAtMS, end.SourceKey,
		)
	}

	if len(groups) > 0 {
		adjustmentIDs := make([]string, 0, len(groups))
		userIDs := make([]string, 0, len(groups))
		campaignIDs := make([]string, 0, len(groups))
		deltas := make([]string, 0, len(groups))
		markerAt := make([]int64, 0, len(groups))
		markerKey := make([]string, 0, len(groups))
		zeros := make([]string, 0, len(groups))

		for _, group := range groups {
			if group.value == nil || group.value.Sign() <= 0 {
				continue
			}
			adjustmentIDs = append(adjustmentIDs, uuid.NewString())
			userIDs = append(userIDs, group.UserID)
			campaignIDs = append(campaignIDs, group.CampaignID)
			deltas = append(deltas, group.Delta)
			markerAt = append(markerAt, end.RecoveryAtMS)
			markerKey = append(markerKey, end.SourceKey)
			zeros = append(zeros, "0")
		}

		if len(adjustmentIDs) > 0 {
			insertResult, err := tx.ExecContext(ctx, `
WITH incoming(adjustment_id, user_id, campaign_id, delta, source_before, source_after, source_marker_at_ms, source_marker_key) AS (
    SELECT * FROM unnest(
        $1::uuid[], $2::uuid[], $3::uuid[], $4::numeric[], $5::numeric[], $6::numeric[], $7::bigint[], $8::text[]
    )
)
INSERT INTO pop_recovery_adjustments (
    adjustment_id, user_id, campaign_id, delta,
    source_before, source_after,
    source_marker_at_ms, source_marker_key,
    status
)
SELECT adjustment_id, user_id, campaign_id, delta,
       source_before, source_after,
       source_marker_at_ms, source_marker_key,
       'pending'
FROM incoming
ON CONFLICT (campaign_id, source_marker_at_ms, source_marker_key) DO NOTHING`,
				pq.Array(adjustmentIDs), pq.Array(userIDs), pq.Array(campaignIDs), pq.Array(deltas),
				pq.Array(zeros), pq.Array(deltas), pq.Array(markerAt), pq.Array(markerKey),
			)
			if err != nil {
				return fmt.Errorf("insert POP recovery event adjustments: %w", err)
			}
			created, err := insertResult.RowsAffected()
			if err != nil {
				return fmt.Errorf("POP recovery event adjustments rows affected: %w", err)
			}
			result.RecoveryAdjustmentsCreated = created
		}
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE pop_recovery_reconciliation_state
SET last_event_recovery_at_ms = $1,
    last_event_source_key = $2,
    last_error = NULL,
    updated_at = NOW()
WHERE id = 1`, end.RecoveryAtMS, end.SourceKey); err != nil {
		return fmt.Errorf("advance POP recovery event cursor: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit POP recovery event transaction: %w", err)
	}
	return nil
}

func normalizeRecoveryTotals(totals []stats.POPRecoveredSpendTotal) ([]normalizedRecoveryTotal, error) {
	out := make([]normalizedRecoveryTotal, 0, len(totals))
	seenCampaigns := make(map[string]struct{}, len(totals))
	for i, total := range totals {
		userID, userOK := canonicalEntityUUID(total.UserID)
		campaignID, campaignOK := canonicalEntityUUID(total.CampaignID)
		amount := strings.TrimSpace(total.Amount)
		if !userOK || !campaignOK {
			log.Printf(
				"[POP_RECOVERY][INVALID_NON_INTERNAL_ENTITY] row=%d user_id=%q campaign_id=%q",
				i, total.UserID, total.CampaignID,
			)
			continue
		}
		if _, exists := seenCampaigns[campaignID]; exists {
			return nil, fmt.Errorf("duplicate ClickHouse POP recovery campaign_id at row %d: %s", i, campaignID)
		}
		value, ok := new(big.Rat).SetString(amount)
		if !ok || value.Sign() < 0 {
			return nil, fmt.Errorf("invalid ClickHouse POP recovery amount at row %d: %q", i, total.Amount)
		}
		seenCampaigns[campaignID] = struct{}{}
		out = append(out, normalizedRecoveryTotal{
			UserID: userID, CampaignID: campaignID, Amount: amount, value: value,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CampaignID < out[j].CampaignID })
	return out, nil
}

func processPOPRecoveryTotalsTx(ctx context.Context, tx *sql.Tx, totals []normalizedRecoveryTotal, result *Result) error {
	if tx == nil {
		return errors.New("POP recovery transaction is nil")
	}
	if result == nil {
		return errors.New("POP recovery result is nil")
	}

	var bootstrapCompleted bool
	if err := tx.QueryRowContext(ctx, `
SELECT bootstrap_completed
FROM pop_recovery_reconciliation_state
WHERE id = 1
FOR SHARE`).Scan(&bootstrapCompleted); err != nil {
		return fmt.Errorf("read POP recovery bootstrap state: %w", err)
	}
	result.RecoveryBootstrapCompleted = bootstrapCompleted
	if !bootstrapCompleted {
		return nil
	}

	campaignIDs := make([]string, 0, len(totals))
	userIDs := make([]string, 0, len(totals))
	for _, total := range totals {
		campaignIDs = append(campaignIDs, total.CampaignID)
		userIDs = append(userIDs, total.UserID)
	}

	// Seed missing cursor rows at zero only after bootstrap. During the initial
	// deploy bootstrap=false prevents historical recovery from turning into
	// pending adjustments before the operator has synchronized old CH-vs-Redis
	// drift and established the initial cursor.
	if len(campaignIDs) > 0 {
		if _, err := tx.ExecContext(ctx, `
WITH incoming(campaign_id, user_id) AS (
    SELECT * FROM unnest($1::uuid[], $2::uuid[])
)
INSERT INTO pop_recovery_cursors (campaign_id, user_id, source_total)
SELECT campaign_id, user_id, 0
FROM incoming
ON CONFLICT (campaign_id) DO NOTHING`, pq.Array(campaignIDs), pq.Array(userIDs)); err != nil {
			return fmt.Errorf("seed POP recovery cursors: %w", err)
		}
	}

	cursors := make(map[string]recoveryCursor, len(totals))
	if len(campaignIDs) > 0 {
		rows, err := tx.QueryContext(ctx, `
SELECT campaign_id::text, user_id::text, source_total::text
FROM pop_recovery_cursors
WHERE campaign_id = ANY($1::uuid[])
ORDER BY campaign_id
FOR UPDATE`, pq.Array(campaignIDs))
		if err != nil {
			return fmt.Errorf("lock POP recovery cursors: %w", err)
		}
		for rows.Next() {
			var campaignID, userID, amount string
			if err := rows.Scan(&campaignID, &userID, &amount); err != nil {
				_ = rows.Close()
				return fmt.Errorf("scan POP recovery cursor: %w", err)
			}
			value, ok := new(big.Rat).SetString(strings.TrimSpace(amount))
			if !ok || value.Sign() < 0 {
				_ = rows.Close()
				return fmt.Errorf("invalid POP recovery cursor for campaign %s: %q", campaignID, amount)
			}
			cursors[campaignID] = recoveryCursor{UserID: userID, Amount: amount, value: value}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return fmt.Errorf("iterate POP recovery cursors: %w", err)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("close POP recovery cursor rows: %w", err)
		}
	}

	// A cursor that was positive but disappeared from the current ClickHouse
	// cumulative result is equivalent to source_total=0. Treat that exactly like
	// any other source regression: alert and never subtract from Redis/cursor.
	missingRows, err := tx.QueryContext(ctx, `
SELECT campaign_id::text, user_id::text, source_total::text
FROM pop_recovery_cursors
WHERE source_total > 0
  AND NOT (campaign_id = ANY($1::uuid[]))
ORDER BY campaign_id
FOR UPDATE`, pq.Array(campaignIDs))
	if err != nil {
		return fmt.Errorf("lock missing POP recovery cursors: %w", err)
	}
	for missingRows.Next() {
		var campaignID, userID, amount string
		if err := missingRows.Scan(&campaignID, &userID, &amount); err != nil {
			_ = missingRows.Close()
			return fmt.Errorf("scan missing POP recovery cursor: %w", err)
		}
		result.RecoveryAnomalies = append(result.RecoveryAnomalies, RecoveryAnomaly{
			UserID: userID, CampaignID: campaignID, CursorTotal: amount, SourceTotal: "0",
			Reason: "ClickHouse recovered total disappeared below the stored cursor",
		})
	}
	if err := missingRows.Err(); err != nil {
		_ = missingRows.Close()
		return fmt.Errorf("iterate missing POP recovery cursors: %w", err)
	}
	if err := missingRows.Close(); err != nil {
		return fmt.Errorf("close missing POP recovery cursor rows: %w", err)
	}

	adjustmentIDs := make([]string, 0)
	adjustmentUserIDs := make([]string, 0)
	adjustmentCampaignIDs := make([]string, 0)
	deltas := make([]string, 0)
	sourceBefore := make([]string, 0)
	sourceAfter := make([]string, 0)
	advanceCampaignIDs := make([]string, 0)
	advanceUserIDs := make([]string, 0)
	advanceTotals := make([]string, 0)

	for _, total := range totals {
		cursor, ok := cursors[total.CampaignID]
		if !ok {
			return fmt.Errorf("POP recovery cursor disappeared for campaign %s", total.CampaignID)
		}
		if strings.TrimSpace(cursor.UserID) != total.UserID {
			result.RecoveryAnomalies = append(result.RecoveryAnomalies, RecoveryAnomaly{
				UserID: total.UserID, CampaignID: total.CampaignID, CursorTotal: cursor.Amount, SourceTotal: total.Amount,
				Reason: fmt.Sprintf("cursor user_id=%s differs from ClickHouse user_id=%s", cursor.UserID, total.UserID),
			})
			continue
		}

		cmp := total.value.Cmp(cursor.value)
		if cmp < 0 {
			result.RecoveryAnomalies = append(result.RecoveryAnomalies, RecoveryAnomaly{
				UserID: total.UserID, CampaignID: total.CampaignID, CursorTotal: cursor.Amount, SourceTotal: total.Amount,
				Reason: "ClickHouse recovered total is lower than the stored cursor",
			})
			continue
		}
		if cmp == 0 {
			continue
		}

		delta := new(big.Rat).Sub(total.value, cursor.value)
		adjustmentIDs = append(adjustmentIDs, uuid.NewString())
		adjustmentUserIDs = append(adjustmentUserIDs, total.UserID)
		adjustmentCampaignIDs = append(adjustmentCampaignIDs, total.CampaignID)
		deltas = append(deltas, decimalString(delta))
		sourceBefore = append(sourceBefore, cursor.Amount)
		sourceAfter = append(sourceAfter, total.Amount)
		advanceCampaignIDs = append(advanceCampaignIDs, total.CampaignID)
		advanceUserIDs = append(advanceUserIDs, total.UserID)
		advanceTotals = append(advanceTotals, total.Amount)
	}

	if len(adjustmentIDs) > 0 {
		insertResult, err := tx.ExecContext(ctx, `
WITH incoming(adjustment_id, user_id, campaign_id, delta, source_before, source_after) AS (
    SELECT * FROM unnest(
        $1::uuid[], $2::uuid[], $3::uuid[], $4::numeric[], $5::numeric[], $6::numeric[]
    )
)
INSERT INTO pop_recovery_adjustments (
    adjustment_id, user_id, campaign_id, delta, source_before, source_after, status
)
SELECT adjustment_id, user_id, campaign_id, delta, source_before, source_after, 'pending'
FROM incoming
ON CONFLICT (campaign_id, source_after) DO NOTHING`,
			pq.Array(adjustmentIDs), pq.Array(adjustmentUserIDs), pq.Array(adjustmentCampaignIDs),
			pq.Array(deltas), pq.Array(sourceBefore), pq.Array(sourceAfter),
		)
		if err != nil {
			return fmt.Errorf("insert POP recovery adjustments: %w", err)
		}
		created, err := insertResult.RowsAffected()
		if err != nil {
			return fmt.Errorf("POP recovery adjustments rows affected: %w", err)
		}
		result.RecoveryAdjustmentsCreated = created

		if _, err := tx.ExecContext(ctx, `
WITH incoming(campaign_id, user_id, source_total) AS (
    SELECT * FROM unnest($1::uuid[], $2::uuid[], $3::numeric[])
)
UPDATE pop_recovery_cursors AS c
SET user_id = incoming.user_id,
    source_total = incoming.source_total,
    updated_at = NOW()
FROM incoming
WHERE c.campaign_id = incoming.campaign_id`,
			pq.Array(advanceCampaignIDs), pq.Array(advanceUserIDs), pq.Array(advanceTotals),
		); err != nil {
			return fmt.Errorf("advance POP recovery cursors: %w", err)
		}
	}

	if len(result.RecoveryAnomalies) > 0 {
		message := formatRecoveryAnomaly(result.RecoveryAnomalies[0])
		if len(result.RecoveryAnomalies) > 1 {
			message = fmt.Sprintf("%s; additional_anomalies=%d", message, len(result.RecoveryAnomalies)-1)
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE pop_recovery_reconciliation_state
SET last_error = $1, last_error_at = NOW(), updated_at = NOW()
WHERE id = 1`, message); err != nil {
			return fmt.Errorf("record POP recovery anomaly: %w", err)
		}
	}

	return nil
}

func decimalString(value *big.Rat) string {
	if value == nil {
		return "0"
	}
	return strings.TrimRight(strings.TrimRight(value.FloatString(12), "0"), ".")
}

func formatRecoveryAnomaly(anomaly RecoveryAnomaly) string {
	return fmt.Sprintf(
		"POP recovery anomaly user_id=%s campaign_id=%s source_total=%s cursor_total=%s reason=%s",
		anomaly.UserID, anomaly.CampaignID, anomaly.SourceTotal, anomaly.CursorTotal, anomaly.Reason,
	)
}

package publisher

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"log"
	"sort"
	"time"
)

type RevenueTotal struct{ UserID, Amount string }
type revenueSource interface {
	PublisherCumulativeRevenue(context.Context) ([]RevenueTotal, error)
}

const aggregateRevenuePrefix = "agg_stats:pop:"

// The global transaction lock is acquired BEFORE reading CH. Therefore two
// instances cannot apply an older snapshot after a newer one. Each snapshot,
// cumulative checkpoint, delta ledger and available balance commit together.
func (s *Service) SyncRevenue(ctx context.Context) (result error) {
	source, ok := s.Reports.(revenueSource)
	if !ok {
		return errors.New("publisher aggregate revenue source unavailable")
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(871922135)`); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `SAVEPOINT publisher_revenue_snapshot`); e != nil {
		return e
	}
	defer func() {
		if result != nil && ctx.Err() == nil {
			if _, err := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT publisher_revenue_snapshot`); err == nil {
				if _, err = tx.ExecContext(ctx, `UPDATE publisher_revenue_source_state SET last_error=$1 WHERE id=1`, result.Error()); err == nil {
					_ = tx.Commit()
				}
			}
		}
	}()
	totals, e := source.PublisherCumulativeRevenue(ctx)
	if e != nil {
		return e
	}
	amounts := map[string]decimal.Decimal{}
	for _, v := range totals {
		id, err := uuid.Parse(v.UserID)
		if err != nil || id == uuid.Nil || id.String() != v.UserID {
			return fmt.Errorf("invalid publisher aggregate owner %q", v.UserID)
		}
		value, err := decimal.NewFromString(v.Amount)
		if err != nil {
			return fmt.Errorf("invalid aggregate amount for %s: %w", v.UserID, err)
		}
		if _, duplicate := amounts[v.UserID]; duplicate {
			return fmt.Errorf("duplicate aggregate owner %s", v.UserID)
		}
		amounts[v.UserID] = value.Round(8)
	}
	// A successfully read empty/missing owner is cumulative zero (correction),
	// while CH errors never enter this branch or overwrite checkpoints.
	rows, e := tx.QueryContext(ctx, `SELECT user_id::text FROM publisher_revenue_checkpoints ORDER BY user_id`)
	if e != nil {
		return e
	}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		if _, found := amounts[id]; !found {
			amounts[id] = decimal.Zero
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	// Deterministic account locks prevent lock-order inversions with payout code.
	ids := sortedRevenueOwners(amounts)
	for _, id := range ids {
		if e = applyRevenueSnapshot(ctx, tx, id, amounts[id]); e != nil {
			return e
		}
	}
	if _, e = tx.ExecContext(ctx, `UPDATE publisher_revenue_source_state SET last_success=now(),last_error='' WHERE id=1`); e != nil {
		return e
	}
	return tx.Commit()
}
func applyRevenueSnapshot(ctx context.Context, tx *sql.Tx, user string, current decimal.Decimal) error {
	// Existing external credits must be resolved explicitly before switching the
	// same account to the automatic authority; never silently add both sources.
	var legacy int
	if e := tx.QueryRowContext(ctx, `SELECT count(*) FROM publisher_revenue WHERE user_id=$1 AND source_id<>$2`, user, aggregateRevenuePrefix+user).Scan(&legacy); e != nil {
		return e
	}
	if legacy != 0 {
		return fmt.Errorf("publisher %s has legacy external revenue; archive/reconcile prior credits explicitly before automatic sync", user)
	}
	if _, e := tx.ExecContext(ctx, `INSERT INTO publisher_accounts(user_id) VALUES($1) ON CONFLICT DO NOTHING`, user); e != nil {
		return e
	}
	var locked string
	if e := tx.QueryRowContext(ctx, `SELECT user_id::text FROM publisher_accounts WHERE user_id=$1 FOR UPDATE`, user).Scan(&locked); e != nil {
		return e
	}
	var before string
	var revision int64
	e := tx.QueryRowContext(ctx, `SELECT cumulative::text,revision FROM publisher_revenue_checkpoints WHERE user_id=$1 FOR UPDATE`, user).Scan(&before, &revision)
	if errors.Is(e, sql.ErrNoRows) {
		before = "0"
	} else if e != nil {
		return e
	}
	old, e := decimal.NewFromString(before)
	if e != nil {
		return e
	}
	delta := current.Sub(old)
	if !delta.IsZero() {
		revision++
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO publisher_revenue_checkpoints(user_id,cumulative,revision) VALUES($1,$2,$3) ON CONFLICT(user_id) DO UPDATE SET cumulative=EXCLUDED.cumulative,revision=EXCLUDED.revision,updated_at=now()`, user, current.StringFixed(8), revision); e != nil {
		return e
	}
	if delta.IsZero() {
		return nil
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO publisher_revenue(source_id,user_id,ad_code_id,site_id,zone_id,amount,event_time,revision) VALUES($1,$2,NULL,'','',$3,now(),$4) ON CONFLICT(source_id) DO UPDATE SET amount=EXCLUDED.amount,revision=EXCLUDED.revision`, aggregateRevenuePrefix+user, user, current.StringFixed(8), revision); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO publisher_revenue_deltas(user_id,revision,cumulative,delta) VALUES($1,$2,$3,$4)`, user, revision, current.StringFixed(8), delta.StringFixed(8)); e != nil {
		return e
	}
	// Negative corrections are kept as debt. Reservation is never erased; create
	// and paid transition refuse uncovered withdrawals until debt is resolved.
	_, e = tx.ExecContext(ctx, `UPDATE publisher_accounts SET balance=balance+$2 WHERE user_id=$1`, user, delta.StringFixed(8))
	return e
}
func (s *Service) RevenueStatus(ctx context.Context) string {
	var success sql.NullTime
	var lastError string
	if e := s.DB.QueryRowContext(ctx, `SELECT last_success,last_error FROM publisher_revenue_source_state WHERE id=1`).Scan(&success, &lastError); e != nil {
		return "agg_stats_unavailable"
	}
	if lastError != "" {
		return "agg_stats_unavailable"
	}
	interval := s.Config.RevenueSyncInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if !success.Valid {
		return "agg_stats_pending"
	}
	if time.Since(success.Time) > 2*interval+10*time.Second {
		return "agg_stats_stale"
	}
	return "agg_stats_ready"
}
func (s *Service) RunRevenueSync(ctx context.Context) {
	interval := s.Config.RevenueSyncInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	timeout := s.Config.RevenueSyncTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		work, cancel := context.WithTimeout(ctx, timeout)
		if e := s.SyncRevenue(work); e != nil {
			log.Printf("publisher aggregate revenue sync: %v", e)
		}
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func sortedRevenueOwners(amounts map[string]decimal.Decimal) []string {
	ids := make([]string, 0, len(amounts))
	for id := range amounts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

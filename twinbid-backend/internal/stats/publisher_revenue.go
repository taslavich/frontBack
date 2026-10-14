package stats

import (
	"context"
	"fmt"
	"strings"
	"time"
	"twinbid-backend/internal/publisher"
)

func publisherCompanionTable(table, name string) (string, error) {
	table, e := normalizeTable(table)
	if e != nil {
		return "", e
	}
	if i := strings.LastIndex(table, "."); i >= 0 {
		name = table[:i+1] + name
	}
	return normalizeTable(name)
}
func (s *Service) PublisherCumulativeRevenue(ctx context.Context) ([]publisher.RevenueTotal, error) {
	r, ok := s.repo.(*ClickHouseRepository)
	if !ok {
		return nil, fmt.Errorf("publisher aggregate unavailable")
	}
	return r.PublisherCumulativeRevenue(ctx)
}
func (r *ClickHouseRepository) PublisherCumulativeRevenue(ctx context.Context) ([]publisher.RevenueTotal, error) {
	if e := r.publisherRevenueSchemaReady(ctx); e != nil {
		return nil, e
	}
	query, e := publisherCumulativeQuery(r.table)
	if e != nil {
		return nil, e
	}
	rows, e := r.db.QueryContext(ctx, query)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []publisher.RevenueTotal{}
	for rows.Next() {
		var v publisher.RevenueTotal
		if e = rows.Scan(&v.UserID, &v.Amount); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func publisherCumulativeQuery(table string) (string, error) {
	table, e := normalizeTable(table)
	if e != nil {
		return "", e
	}
	// Do not silently discard invalid non-empty ownership. The consumer strictly
	// validates every returned UUID/amount before any checkpoint or money changes.
	return `SELECT pub_user_id,toString(round(sum(ssp_revenue_views),8)) FROM ` + table + ` WHERE format='POP' AND pub_user_id!='' GROUP BY pub_user_id ORDER BY pub_user_id`, nil
}
func (s *Service) PublisherTopSites(ctx context.Context, user string, from, to time.Time) ([]string, error) {
	r, ok := s.repo.(*ClickHouseRepository)
	if !ok {
		return nil, fmt.Errorf("publisher aggregate unavailable")
	}
	table, e := normalizeTable(r.table)
	if e != nil {
		return nil, e
	}
	rows, e := r.db.QueryContext(ctx, `SELECT site_id FROM `+table+` WHERE pub_user_id=? AND format='POP' AND event_hour>=? AND event_hour<? GROUP BY site_id ORDER BY sum(ssp_revenue_views) DESC,site_id LIMIT 5`, user, from.UTC().Truncate(time.Hour), to.UTC().Truncate(time.Hour).Add(time.Hour))
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return nil, e
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// A partial DDL rollout must not be called a ready financial source merely
// because ADD COLUMN succeeded. Validate merging key and all four writer views.
func (r *ClickHouseRepository) publisherRevenueSchemaReady(ctx context.Context) error {
	table, e := normalizeTable(r.table)
	if e != nil {
		return e
	}
	database, name := "", table
	if parts := strings.Split(table, "."); len(parts) == 2 {
		database, name = parts[0], parts[1]
	}
	var engine, key string
	e = r.db.QueryRowContext(ctx, `SELECT engine,sorting_key FROM system.tables WHERE database=if(?='',currentDatabase(),?) AND name=?`, database, database, name).Scan(&engine, &key)
	if e != nil {
		return e
	}
	key = strings.NewReplacer(" ", "", "`", "", "\n", "").Replace(key)
	key = strings.Trim(key, "()")
	if !strings.HasSuffix(engine, "SummingMergeTree") || !strings.HasSuffix(key, "pub_user_id,ad_code_id,zone_id") {
		return fmt.Errorf("publisher aggregate sorting key is not migrated: %s", table)
	}
	var views uint64
	e = r.db.QueryRowContext(ctx, `SELECT count() FROM system.tables WHERE database=if(?='',currentDatabase(),?) AND name IN ('mv_fact_impressions_to_agg_stats','mv_fact_clicks_to_agg_stats','mv_fact_clicks_wins_to_agg_stats','mv_fact_conversions_to_agg_stats') AND engine='MaterializedView' AND position(create_table_query,'ssp_revenue_views')>0 AND position(create_table_query,'pub_user_id')>0 AND position(create_table_query,'ad_code_id')>0 AND position(create_table_query,'zone_id')>0`, database, database).Scan(&views)
	if e != nil {
		return e
	}
	if views != 4 {
		return fmt.Errorf("publisher aggregate writer migration incomplete: %d/4", views)
	}
	return nil
}

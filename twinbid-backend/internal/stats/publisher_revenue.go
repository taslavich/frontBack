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
	// Only popunder is supported by publisher Code.Validate / runtime today.
	// BAN/NAT/IPP/VID grouping is analytics, not an authorization to pay formats
	// that cannot be created or served. PublisherQuery uses this same restriction.
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

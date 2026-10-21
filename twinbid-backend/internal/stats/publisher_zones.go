package stats

import (
	"context"
	"fmt"
	"sort"
	"time"
	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/publisher"
)

func (s *Service) PublisherZoneDiagnostics(ctx context.Context, user string, codes []string, limit, offset int, asof time.Time) (publisher.ZoneDiagnostics, error) {
	r, ok := s.repo.(*ClickHouseRepository)
	if !ok {
		return publisher.ZoneDiagnostics{}, fmt.Errorf("zone diagnostics unavailable")
	}
	return r.PublisherZoneDiagnostics(ctx, user, codes, limit, offset, asof)
}

// Coverage heartbeats acknowledge a previous successful refresh, never their
// own possibly partial APPEND. A source
// TTL of one hour cannot repair a long outage. Fail closed on stale/gapped data.
func zoneCoverageReady(stamps []time.Time, bootstrap, from, to time.Time) bool {
	sort.Slice(stamps, func(i, j int) bool { return stamps[i].Before(stamps[j]) })
	cursor := from
	if !bootstrap.IsZero() && !bootstrap.Before(from) && !bootstrap.After(to) {
		cursor = bootstrap
	}
	for _, t := range stamps {
		if t.Before(cursor) {
			continue
		}
		if t.Sub(cursor) > 10*time.Minute {
			return false
		}
		cursor = t
	}
	return to.Sub(cursor) <= 10*time.Minute
}

func (r *ClickHouseRepository) PublisherZoneDiagnostics(ctx context.Context, user string, codes []string, limit, offset int, asof time.Time) (publisher.ZoneDiagnostics, error) {
	to := asof.UTC().Truncate(time.Second)
	from := to.Add(-time.Hour)
	out := publisher.ZoneDiagnostics{Rows: []publisher.ZoneDiagnostic{}, UndiscoveredCodes: []publisher.UndiscoveredCode{}, Limit: limit, Offset: offset, WindowFrom: from, WindowTo: to}
	if user == "" || limit < 1 || limit > 500 || offset < 0 || offset > 1000000 {
		return out, httpx.BadRequest("invalid zone diagnostics request")
	}
	if len(codes) == 0 {
		return out, nil
	}
	table, e := publisherCompanionTable(r.table, "publisher_zone_hourly")
	if e != nil {
		return out, e
	}
	registry, e := publisherCompanionTable(r.table, "publisher_inventory")
	if e != nil {
		return out, e
	}
	// The supplied IDs come from the owner-bound active PG registry. Durable
	// discovery decides whether a code has ANY known zone, independently of time.
	discovered, e := r.db.QueryContext(ctx, "SELECT DISTINCT ad_code_id FROM "+registry+" WHERE pub_user_id=? AND ad_code_id IN ? AND site_id!='' AND zone_id!=''", user, codes)
	if e != nil {
		return out, e
	}
	known := map[string]bool{}
	for discovered.Next() {
		var id string
		if e = discovered.Scan(&id); e != nil {
			discovered.Close()
			return out, e
		}
		known[id] = true
	}
	e = discovered.Err()
	discovered.Close()
	if e != nil {
		return out, e
	}
	missing := []string{}
	seen := map[string]bool{}
	for _, id := range codes {
		if !known[id] && !seen[id] {
			missing = append(missing, id)
			seen[id] = true
		}
	}
	sort.Strings(missing)
	out.CodesWithoutZones = uint64(len(missing))
	for i := offset; i < len(missing) && i < offset+limit; i++ {
		out.UndiscoveredCodes = append(out.UndiscoveredCodes, publisher.UndiscoveredCode{CodeID: missing[i], Status: "no_zones_detected"})
	}
	beats, e := r.db.QueryContext(ctx, `SELECT is_meta,refreshed_at FROM `+table+` FINAL WHERE is_meta>0 AND expires_at>now() AND refreshed_at>=? AND refreshed_at<=? ORDER BY refreshed_at`, from.Add(-10*time.Minute), to)
	if e != nil {
		return out, e
	}
	stamps := []time.Time{}
	var boot time.Time
	for beats.Next() {
		var kind uint8
		var stamp time.Time
		if e = beats.Scan(&kind, &stamp); e != nil {
			beats.Close()
			return out, e
		}
		if kind == 2 {
			boot = stamp
		} else {
			stamps = append(stamps, stamp)
		}
		if stamp.After(out.RefreshedAt) {
			out.RefreshedAt = stamp
		}
	}
	e = beats.Err()
	beats.Close()
	if e != nil {
		return out, e
	}
	if !zoneCoverageReady(stamps, boot, from, to) {
		return out, httpx.ServiceUnavailable("zone diagnostics refresh is stale or incomplete")
	}
	base := `WITH inventory AS (
 SELECT ad_code_id,zone_id,site_id,argMax(hostname,last_request_at) AS hostname,max(last_request_at) AS known_last_request_at
 FROM ` + registry + ` WHERE pub_user_id=? AND ad_code_id IN ? AND site_id!='' AND zone_id!=''
 GROUP BY ad_code_id,zone_id,site_id
 ), counts AS (
 SELECT ad_code_id,zone_id,site_id,sum(s.n200) AS n200,sum(s.n204) AS n204,sum(s.n4xx) AS n4xx,sum(s.n5xx) AS n5xx
 FROM ` + table + ` FINAL ARRAY JOIN seconds AS s
 WHERE is_meta=0 AND pub_user_id=? AND ad_code_id IN ? AND expires_at>now()
 AND event_bucket>=toStartOfFiveMinutes(?) AND event_bucket<? AND s.second>=? AND s.second<?
 GROUP BY ad_code_id,zone_id,site_id
 ), zones AS (
 SELECT i.ad_code_id AS ad_code_id,i.zone_id AS zone_id,i.site_id AS site_id,i.hostname AS hostname,
 ifNull(c.n200,0) AS n200,ifNull(c.n204,0) AS n204,ifNull(c.n4xx,0) AS n4xx,ifNull(c.n5xx,0) AS n5xx,i.known_last_request_at AS last_seen
 FROM inventory i LEFT JOIN counts c ON i.ad_code_id=c.ad_code_id AND i.zone_id=c.zone_id AND i.site_id=c.site_id
 ) `
	args := []any{user, codes, user, codes, from, to, from, to}
	if e = r.db.QueryRowContext(ctx, base+`SELECT countIf(n200>0 OR n204>0),countIf(n200=0 AND n204=0) FROM zones`, args...).Scan(&out.ActiveZones, &out.ProblemZones); e != nil {
		return out, e
	}
	rowArgs := append(append([]any{}, args...), limit, offset)
	rows, e := r.db.QueryContext(ctx, base+`SELECT ad_code_id,zone_id,site_id,hostname,n200,n204,n4xx,n5xx,last_seen FROM zones ORDER BY ad_code_id,zone_id,site_id LIMIT ? OFFSET ?`, rowArgs...)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var z publisher.ZoneDiagnostic
		if e = rows.Scan(&z.CodeID, &z.ZoneID, &z.SiteID, &z.Hostname, &z.Requests200, &z.Requests204, &z.Requests4xx, &z.Requests5xx, &z.LastRequestAt); e != nil {
			return out, e
		}
		z.Status = publisher.ZoneStatus(z.Requests200, z.Requests204)
		out.Rows = append(out.Rows, z)
	}
	return out, rows.Err()
}

// Compatibility counters remain in the API contract. Financial metrics are
// still selected exclusively from agg_stats by the unchanged UNION branch.
func publisherRequestSnapshotSQL(table, where string) string {
	return `SELECT toDateTime64(last_request_at,3,'UTC') AS event_time,site_id,zone_id,ad_code_id,hostname,geo,device,os,browser,format,
 requests_200+requests_204+requests_4xx+requests_5xx+requests_other AS requests,
 requests_4xx+requests_5xx AS errors,requests_200 AS fills,requests_204 AS no_fills,
 toUInt64(0) AS impressions,toUInt64(0) AS clicks,toFloat64(0) AS revenue FROM ` + table + ` FINAL WHERE is_meta=0 AND expires_at>now() AND ` + where
}

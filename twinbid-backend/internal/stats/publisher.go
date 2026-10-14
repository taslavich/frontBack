package stats

import (
	"context"
	"fmt"
	"strings"
	"time"
	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/publisher"
)

func (s *Service) PublisherQuery(ctx context.Context, user string, q publisher.StatsRequest) (publisher.Report, error) {
	r, ok := s.repo.(*ClickHouseRepository)
	if !ok {
		return publisher.Report{}, fmt.Errorf("publisher reports unavailable")
	}
	return r.PublisherQuery(ctx, user, q)
}
func (r *ClickHouseRepository) PublisherQuery(ctx context.Context, user string, q publisher.StatsRequest) (publisher.Report, error) {
	var out publisher.Report
	out.Rows = []publisher.Row{}
	out.Limit = q.Limit
	out.Offset = q.Offset
	if q.Limit < 1 || q.Limit > 500 || q.Offset < 0 || q.Offset > 1000000 || q.From.IsZero() || !q.To.After(q.From) || (!q.LifetimeRevenueOnly && q.To.Sub(q.From) > 366*24*time.Hour) {
		return out, httpx.BadRequest("valid UTC from/to, period <=366d, limit 1..500 required")
	}
	tz, e := publisher.NormalizeTimezone(q.Timezone)
	if e != nil {
		return out, httpx.BadRequest("invalid timezone")
	}
	q.Timezone = tz
	group := map[string]string{"date": "toString(toDate(event_time,?))", "hour": "formatDateTime(event_time,'%Y-%m-%d %H:%i:00',?)", "country": "geo", "device": "device", "os": "os", "browser": "browser", "site": "site_id", "zone": "zone_id", "ad_code": "ad_code_id"}[q.GroupBy]
	if group == "" {
		return out, httpx.BadRequest("invalid group_by")
	}
	table, e := normalizeTable(r.table)
	if e != nil {
		return out, e
	}
	requestsTable, e := publisherCompanionTable(table, "publisher_requests_logical")
	if e != nil {
		return out, e
	}
	// The source aggregate has hourly resolution. Requests use the same expanded
	// hour range; no fabricated sub-hour impression/revenue precision is exposed.
	q.From = q.From.UTC().Truncate(time.Hour)
	if rounded := q.To.UTC().Truncate(time.Hour); !rounded.Equal(q.To) {
		q.To = rounded.Add(time.Hour)
	}
	out.EffectiveFrom = q.From
	out.EffectiveTo = q.To
	out.TimeResolution = "1h_utc"
	where := `pub_user_id=? AND event_time>=? AND event_time<? AND (?='' OR site_id=?) AND (?='' OR zone_id=?) AND (?='' OR ad_code_id=?)`
	argsFor := func() []any {
		return []any{user, q.From, q.To, q.SiteID, q.SiteID, q.ZoneID, q.ZoneID, q.CodeID, q.CodeID}
	}
	base := `WITH data AS (
 SELECT event_time,site_id,zone_id,ad_code_id,ifNull(site_domain,'') AS hostname,ifNull(geo,'') AS geo,ifNull(device,'') AS device,ifNull(os,'') AS os,ifNull(browser,'') AS browser,toUInt64(1) AS requests,toUInt64(request_error) AS errors,toUInt64(code=200 AND request_error=0) AS fills,toUInt64(code=204 AND request_error=0) AS no_fills,toUInt64(0) AS impressions,toUInt64(0) AS clicks,toFloat64(0) AS revenue FROM ` + requestsTable + ` WHERE ` + where + `
 UNION ALL
 SELECT toDateTime64(event_hour,3,'UTC') AS event_time,site_id,zone_id,ad_code_id,'' AS hostname,geo,device_type AS device,os,browser,toUInt64(0),toUInt64(0),toUInt64(0),toUInt64(0),impressions,clicks,ssp_revenue_views AS revenue FROM ` + table + ` WHERE format='POP' AND ` + strings.ReplaceAll(where, "event_time", "event_hour") + `
 ) `
	args := append(argsFor(), argsFor()...)
	if q.LifetimeRevenueOnly {
		if q.GroupBy != "site" && q.GroupBy != "zone" && q.GroupBy != "ad_code" {
			return out, httpx.BadRequest("identity group required for lifetime earnings")
		}
		base = `WITH data AS (SELECT toDateTime64(event_hour,3,'UTC') AS event_time,site_id,zone_id,ad_code_id,'' AS hostname,geo,device_type AS device,os,browser,toUInt64(0) AS requests,toUInt64(0) AS errors,toUInt64(0) AS fills,toUInt64(0) AS no_fills,toUInt64(0) AS impressions,toUInt64(0) AS clicks,ssp_revenue_views AS revenue FROM ` + table + ` WHERE format='POP' AND pub_user_id=? AND (?='' OR site_id=?) AND (?='' OR zone_id=?) AND (?='' OR ad_code_id=?)) `
		args = []any{user, q.SiteID, q.SiteID, q.ZoneID, q.ZoneID, q.CodeID, q.CodeID}
	}
	filterSQL := "1=1"
	if q.DiscoveredOnly {
		filterSQL += " AND site_id!='' AND zone_id!=''"
	}
	if len(q.BucketIDs) > 0 {
		column := map[string]string{"site": "site_id", "zone": "zone_id", "ad_code": "ad_code_id"}[q.GroupBy]
		if column == "" || len(q.BucketIDs) > 500 {
			return out, httpx.BadRequest("invalid inventory buckets")
		}
		placeholders := []string{}
		for _, id := range q.BucketIDs {
			args = append(args, id)
			placeholders = append(placeholders, "?")
		}
		filterSQL += " AND " + column + " IN (" + strings.Join(placeholders, ",") + ")"
	}
	for _, filter := range []struct {
		col    string
		values []string
	}{{"geo", q.Countries}, {"device", q.Devices}, {"os", q.OSes}, {"browser", q.Browsers}} {
		if len(filter.values) > 64 {
			return out, httpx.BadRequest("too many filter values")
		}
		if len(filter.values) == 0 {
			continue
		}
		placeholders := []string{}
		for _, value := range filter.values {
			if len(value) > 100 {
				return out, httpx.BadRequest("filter value too long")
			}
			args = append(args, strings.ToLower(value))
			placeholders = append(placeholders, "?")
		}
		filterSQL += " AND lowerUTF8(" + filter.col + ") IN (" + strings.Join(placeholders, ",") + ")"
	}
	base += ",filtered AS (SELECT * FROM data WHERE " + filterSQL + ") "
	rowArgs := append([]any{}, args...)
	if q.GroupBy == "date" || q.GroupBy == "hour" {
		rowArgs = append(rowArgs, q.Timezone)
	}
	rowArgs = append(rowArgs, q.Limit, q.Offset)
	query := base + `SELECT ` + group + ` AS bucket,any(site_id),any(zone_id),any(ad_code_id),any(hostname),maxIfOrNull(event_time,requests=1),sum(requests),sum(errors),sum(fills),sum(no_fills),sum(impressions),sum(clicks),toString(round(sum(revenue),8)) FROM filtered GROUP BY bucket ORDER BY bucket LIMIT ? OFFSET ?`
	rows, e := r.db.QueryContext(ctx, query, rowArgs...)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var v publisher.Row
		v.Revenue = "0"
		if e = rows.Scan(&v.Bucket, &v.SiteID, &v.ZoneID, &v.CodeID, &v.Hostname, &v.LastRequestAt, &v.Requests, &v.Errors, &v.Fills, &v.NoFills, &v.Impressions, &v.Clicks, &v.Revenue); e != nil {
			return out, e
		}
		if v.Impressions > 0 {
			v.CTR = 100 * float64(v.Clicks) / float64(v.Impressions)
		}
		out.Rows = append(out.Rows, v)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	e = r.db.QueryRowContext(ctx, base+`SELECT sum(requests),sum(errors),sum(fills),sum(no_fills),sum(impressions),sum(clicks),toString(round(sum(revenue),8)) FROM filtered`, args...).Scan(&out.Totals.Requests, &out.Totals.Errors, &out.Totals.Fills, &out.Totals.NoFills, &out.Totals.Impressions, &out.Totals.Clicks, &out.Totals.Revenue)
	if out.Totals.Impressions > 0 {
		out.Totals.CTR = 100 * float64(out.Totals.Clicks) / float64(out.Totals.Impressions)
	}
	return out, e
}
func (s *Service) PublisherDiagnosis(ctx context.Context, user, code string) ([]publisher.Diagnostic, error) {
	r, ok := s.repo.(*ClickHouseRepository)
	if !ok {
		return nil, fmt.Errorf("diagnostics unavailable")
	}
	requestsTable, e := publisherCompanionTable(r.table, "publisher_requests_logical")
	if e != nil {
		return nil, e
	}
	rows, e := r.db.QueryContext(ctx, `SELECT request_id,ifNull(site_id,''),zone_id,ifNull(site_domain,''),event_time,code,request_error,error_reason FROM `+requestsTable+` WHERE pub_user_id=? AND ad_code_id=? ORDER BY event_time DESC LIMIT 50`, user, code)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []publisher.Diagnostic{}
	for rows.Next() {
		var d publisher.Diagnostic
		var flag uint8
		if e = rows.Scan(&d.RequestID, &d.SiteID, &d.ZoneID, &d.Hostname, &d.EventTime, &d.Code, &flag, &d.Reason); e != nil {
			return nil, e
		}
		d.Error = flag != 0
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Service) PublisherInventory(ctx context.Context, user, group, site, code string, limit, offset int) ([]publisher.Row, error) {
	r, ok := s.repo.(*ClickHouseRepository)
	if !ok {
		return nil, fmt.Errorf("inventory unavailable")
	}
	bucket := "zone_id"
	if group == "site" {
		bucket = "site_id"
	}
	inventoryTable, e := publisherCompanionTable(r.table, "publisher_inventory")
	if e != nil {
		return nil, e
	}
	query := `SELECT ` + bucket + `,any(site_id),any(zone_id),any(ad_code_id),any(hostname),min(first_request_at),max(last_request_at) FROM ` + inventoryTable + ` WHERE pub_user_id=? AND (?='' OR site_id=?) AND (?='' OR ad_code_id=?) GROUP BY ` + bucket + ` ORDER BY ` + bucket + ` LIMIT ? OFFSET ?`
	rows, e := r.db.QueryContext(ctx, query, user, site, site, code, code, limit, offset)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []publisher.Row{}
	for rows.Next() {
		var v publisher.Row
		if e = rows.Scan(&v.Bucket, &v.SiteID, &v.ZoneID, &v.CodeID, &v.Hostname, &v.FirstRequestAt, &v.LastRequestAt); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

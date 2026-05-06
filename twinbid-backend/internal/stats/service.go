package stats

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"

	"twinbid-backend/internal/config"
	"twinbid-backend/internal/httpx"
)

type Service struct {
	db    *sql.DB
	table string
}

type groupSpec struct {
	selectExpr string
	groupExpr  string
	orderExpr  string
}

func NewService(ctx context.Context, cfg config.ClickHouseConfig) (*Service, error) {
	db, err := sql.Open("clickhouse", buildDSN(cfg))
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	table := strings.TrimSpace(cfg.Table)
	if table == "" {
		table = "agg_stats"
	}

	return &Service{db: db, table: table}, nil
}

func (s *Service) Close() error { return s.db.Close() }

var groupColumns = map[GroupBy]groupSpec{
	GroupByDate: {
		selectExpr: "toString(event_date)",
		groupExpr:  "event_date",
		orderExpr:  "event_date",
	},
	GroupByHour: {
		selectExpr: "formatDateTime(toStartOfHour(event_hour), '%F %H:00', 'UTC')",
		groupExpr:  "toStartOfHour(event_hour)",
		orderExpr:  "toStartOfHour(event_hour)",
	},
	GroupByCountry: {
		selectExpr: "geo",
		groupExpr:  "geo",
	},
	GroupByOS: {
		selectExpr: "os",
		groupExpr:  "os",
	},
	GroupByBrowser: {
		selectExpr: "browser",
		groupExpr:  "browser",
	},
	GroupByDeviceType: {
		selectExpr: "device_type",
		groupExpr:  "device_type",
	},
	GroupBySiteID: {
		selectExpr: "site_id",
		groupExpr:  "site_id",
	},
	GroupByCampaign: {
		selectExpr: "toString(campaign_id)",
		groupExpr:  "campaign_id",
	},
}

var filterColumns = map[string]string{
	string(FilterByCountry):    "geo",
	string(FilterByOS):         "os",
	string(FilterByBrowser):    "browser",
	string(FilterByDeviceType): "device_type",
}

// One common spent formula for the single stats endpoint.
// In your current MV spend_views_table is already stored as win_dsp_price / 1000,
// and spend_clicks_table is stored as win_dsp_price, so we do not divide it here again.
const spentExpression = "round(sum(spend_views_table) + sum(spend_clicks_table), 2)"

func (s *Service) Query(ctx context.Context, userID string, req QueryRequest) (QueryResponse, error) {
	if req.GroupBy == "" {
		req.GroupBy = GroupByCampaign
	}

	spec, ok := groupColumns[req.GroupBy]
	if !ok {
		return QueryResponse{}, httpx.BadRequest("invalid group_by: " + string(req.GroupBy))
	}

	from, to, err := normalizeDates(req.From, req.To)
	if err != nil {
		return QueryResponse{}, err
	}

	where, args, err := s.where(req, userID, from, to)
	if err != nil {
		return QueryResponse{}, err
	}

	query := fmt.Sprintf(`
SELECT
    %s AS bucket,
    toUInt64(sum(impressions)) AS impressions,
    toUInt64(sum(clicks)) AS clicks,
    %s AS spent,
    round(if(sum(impressions) = 0, 0, sum(clicks) * 100.0 / sum(impressions)), 2) AS ctr
FROM %s
WHERE %s
GROUP BY %s
%s`, spec.selectExpr, spentExpression, s.table, where, spec.groupExpr, buildOrderBy(spec))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return QueryResponse{}, err
	}
	defer rows.Close()

	out := make(map[string]Summary)
	for rows.Next() {
		var bucket string
		var summary Summary
		if err := rows.Scan(&bucket, &summary.Impressions, &summary.Clicks, &summary.Spent, &summary.CTR); err != nil {
			return QueryResponse{}, err
		}
		out[bucket] = summary
	}
	if err := rows.Err(); err != nil {
		return QueryResponse{}, err
	}

	totals, err := s.totals(ctx, req, userID, from, to)
	if err != nil {
		return QueryResponse{}, err
	}

	return QueryResponse{Rows: out, Totals: totals}, nil
}

func (s *Service) totals(ctx context.Context, req QueryRequest, userID, from, to string) (Summary, error) {
	where, args, err := s.where(req, userID, from, to)
	if err != nil {
		return Summary{}, err
	}

	query := fmt.Sprintf(`
SELECT
    toUInt64(sum(impressions)) AS impressions,
    toUInt64(sum(clicks)) AS clicks,
    %s AS spent,
    round(if(sum(impressions) = 0, 0, sum(clicks) * 100.0 / sum(impressions)), 2) AS ctr
FROM %s
WHERE %s`, spentExpression, s.table, where)

	var res Summary
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&res.Impressions, &res.Clicks, &res.Spent, &res.CTR); err != nil {
		return Summary{}, err
	}
	return res, nil
}

func (s *Service) where(req QueryRequest, userID, from, to string) (string, []any, error) {
	if err := validateUUID(userID, "user_id"); err != nil {
		return "", nil, err
	}
	if err := validateUUIDList(req.CampaignIDs, "campaign_ids"); err != nil {
		return "", nil, err
	}
	if err := validateUUIDList(req.CreativeIDs, "creative_ids"); err != nil {
		return "", nil, err
	}

	parts := []string{"user_id = toUUID(?)", "event_date BETWEEN toDate(?) AND toDate(?)"}
	args := []any{userID, from, to}

	if len(req.CampaignIDs) > 0 {
		parts = append(parts, "campaign_id IN ("+uuidPlaceholders(len(req.CampaignIDs))+")")
		for _, id := range req.CampaignIDs {
			args = append(args, id)
		}
	}

	if len(req.CreativeIDs) > 0 {
		parts = append(parts, "creative_id IN ("+uuidPlaceholders(len(req.CreativeIDs))+")")
		for _, id := range req.CreativeIDs {
			args = append(args, id)
		}
	}

	filterKeys := make([]string, 0, len(req.Filters))
	for key := range req.Filters {
		filterKeys = append(filterKeys, key)
	}
	sort.Strings(filterKeys)

	for _, key := range filterKeys {
		values := req.Filters[key]
		if len(values) == 0 {
			continue
		}
		col, ok := filterColumns[key]
		if !ok {
			return "", nil, httpx.BadRequest("invalid filter: " + key)
		}
		parts = append(parts, fmt.Sprintf("%s IN (%s)", col, valuePlaceholders(len(values))))
		for _, value := range values {
			args = append(args, value)
		}
	}

	return strings.Join(parts, " AND "), args, nil
}

func normalizeDates(from, to string) (string, string, error) {
	if from == "" {
		from = time.Now().UTC().AddDate(0, -1, 0).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().UTC().Format("2006-01-02")
	}

	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		return "", "", httpx.BadRequest("invalid from date, expected YYYY-MM-DD")
	}
	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		return "", "", httpx.BadRequest("invalid to date, expected YYYY-MM-DD")
	}
	if fromDate.After(toDate) {
		return "", "", httpx.BadRequest("from must be <= to")
	}

	return from, to, nil
}

func validateUUID(value, field string) error {
	if _, err := uuid.Parse(value); err != nil {
		return httpx.BadRequest("invalid " + field)
	}
	return nil
}

func validateUUIDList(values []string, field string) error {
	for _, value := range values {
		if err := validateUUID(value, field); err != nil {
			return err
		}
	}
	return nil
}

func uuidPlaceholders(n int) string {
	items := make([]string, n)
	for i := range items {
		items[i] = "toUUID(?)"
	}
	return strings.Join(items, ",")
}

func valuePlaceholders(n int) string {
	items := make([]string, n)
	for i := range items {
		items[i] = "?"
	}
	return strings.Join(items, ",")
}

func buildOrderBy(spec groupSpec) string {
	if spec.orderExpr == "" {
		return ""
	}
	return "ORDER BY " + spec.orderExpr
}

func buildDSN(cfg config.ClickHouseConfig) string {
	u := url.URL{Scheme: "clickhouse", Host: cfg.Addr, Path: cfg.Database}
	if cfg.Username != "" {
		u.User = url.UserPassword(cfg.Username, cfg.Password)
	}

	q := u.Query()
	q.Set("secure", fmt.Sprintf("%t", cfg.Secure))
	u.RawQuery = q.Encode()

	return u.String()
}

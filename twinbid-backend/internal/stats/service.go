package stats

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"twinbid-backend/internal/config"
	"twinbid-backend/internal/httpx"
)

type Service struct {
	db    *sql.DB
	table string
}

func NewService(ctx context.Context, cfg config.ClickHouseConfig) (*Service, error) {
	dsn := buildDSN(cfg)
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	return &Service{db: db, table: cfg.Table}, nil
}

func (s *Service) Close() error { return s.db.Close() }

var groupColumns = map[string]string{
	"date":        "toString(date)",
	"hour":        "formatDateTime(hour, '%Y-%m-%d %H:00')",
	"campaign":    "campaign_id",
	"country":     "country",
	"format":      "format_type",
	"creative":    "creative_id",
	"os":          "os",
	"browser":     "browser",
	"device_type": "device_type",
	"language":    "language",
	"site_id":     "site_id",
}

var filterColumns = map[string]string{
	"campaign":    "campaign_id",
	"country":     "country",
	"format":      "format_type",
	"creative":    "creative_id",
	"os":          "os",
	"browser":     "browser",
	"device_type": "device_type",
	"language":    "language",
	"site_id":     "site_id",
}

func (s *Service) Query(ctx context.Context, userID string, req QueryRequest) (QueryResponse, error) {
	if len(req.GroupBy) == 0 {
		req.GroupBy = []string{"campaign"}
	}
	for _, g := range req.GroupBy {
		if _, ok := groupColumns[g]; !ok {
			return QueryResponse{}, httpx.BadRequest("invalid group_by: " + g)
		}
	}
	from, to := normalizeDates(req.From, req.To)

	selectParts := make([]string, 0, len(req.GroupBy)+4)
	groupExprs := make([]string, 0, len(req.GroupBy))
	for _, g := range req.GroupBy {
		expr := groupColumns[g]
		selectParts = append(selectParts, fmt.Sprintf("%s AS %s", expr, g))
		groupExprs = append(groupExprs, expr)
	}
	selectParts = append(selectParts, "sum(impressions) AS impressions", "sum(clicks) AS clicks", "sum(spent) AS spent", "if(sum(impressions)=0, 0, round(sum(clicks)/sum(impressions)*100, 2)) AS ctr")

	where, args, err := s.where(req, userID, from, to)
	if err != nil {
		return QueryResponse{}, err
	}
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s GROUP BY %s ORDER BY impressions DESC LIMIT 1000", strings.Join(selectParts, ", "), s.table, where, strings.Join(groupExprs, ", "))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return QueryResponse{}, err
	}
	defer rows.Close()

	var out []Row
	for rows.Next() {
		groupValues := make([]sql.NullString, len(req.GroupBy))
		dest := make([]any, 0, len(req.GroupBy)+4)
		for i := range groupValues {
			dest = append(dest, &groupValues[i])
		}
		var impressions, clicks uint64
		var spent, ctr float64
		dest = append(dest, &impressions, &clicks, &spent, &ctr)
		if err := rows.Scan(dest...); err != nil {
			return QueryResponse{}, err
		}
		m := Row{}
		for i, g := range req.GroupBy {
			if groupValues[i].Valid {
				m[g] = groupValues[i].String
			} else {
				m[g] = ""
			}
		}
		m["impressions"] = impressions
		m["clicks"] = clicks
		m["spent"] = spent
		m["ctr"] = ctr
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return QueryResponse{}, err
	}
	totals, err := s.Totals(ctx, userID, req, from, to)
	if err != nil {
		return QueryResponse{}, err
	}
	return QueryResponse{Rows: out, Totals: totals}, nil
}

func (s *Service) Overview(ctx context.Context, userID string) (Summary, error) {
	from := time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	to := time.Now().Format("2006-01-02")
	return s.Totals(ctx, userID, QueryRequest{}, from, to)
}

func (s *Service) CampaignSummary(ctx context.Context, userID, campaignID string) (Summary, error) {
	from := time.Now().AddDate(0, -12, 0).Format("2006-01-02")
	to := time.Now().Format("2006-01-02")
	return s.Totals(ctx, userID, QueryRequest{CampaignIDs: []string{campaignID}}, from, to)
}

func (s *Service) Totals(ctx context.Context, userID string, req QueryRequest, from, to string) (Summary, error) {
	where, args, err := s.where(req, userID, from, to)
	if err != nil {
		return Summary{}, err
	}
	query := fmt.Sprintf("SELECT sum(impressions), sum(clicks), sum(spent), if(sum(impressions)=0, 0, round(sum(clicks)/sum(impressions)*100, 2)) FROM %s WHERE %s", s.table, where)
	var res Summary
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&res.Impressions, &res.Clicks, &res.Spent, &res.CTR); err != nil {
		return Summary{}, err
	}
	return res, nil
}

func (s *Service) where(req QueryRequest, userID, from, to string) (string, []any, error) {
	parts := []string{"user_id = ?", "date >= toDate(?)", "date <= toDate(?)"}
	args := []any{userID, from, to}
	if len(req.CampaignIDs) > 0 {
		parts = append(parts, "campaign_id IN ?")
		args = append(args, req.CampaignIDs)
	}
	for key, values := range req.Filters {
		if len(values) == 0 {
			continue
		}
		col, ok := filterColumns[key]
		if !ok {
			return "", nil, httpx.BadRequest("invalid filter: " + key)
		}
		parts = append(parts, fmt.Sprintf("%s IN ?", col))
		args = append(args, values)
	}
	return strings.Join(parts, " AND "), args, nil
}

func normalizeDates(from, to string) (string, string) {
	if from == "" {
		from = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02")
	}
	return from, to
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

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
)

type ClickHouseRepository struct {
	db           *sql.DB
	table        string
	trafficTable string
}

func NewClickHouseRepository(ctx context.Context, cfg config.ClickHouseConfig) (*ClickHouseRepository, error) {
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

	trafficTable := strings.TrimSpace(cfg.TrafficTable)
	if trafficTable == "" {
		trafficTable = "traffic_volume_hourly"
	}

	return &ClickHouseRepository{
		db:           db,
		table:        table,
		trafficTable: trafficTable,
	}, nil
}

func (r *ClickHouseRepository) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *ClickHouseRepository) Query(ctx context.Context, userID string, req QueryRequest) (QueryResponse, error) {
	rowsPlan, totalsPlan, err := buildStatsQueries(userID, req, r.table)
	if err != nil {
		return QueryResponse{}, err
	}

	rows, err := r.db.QueryContext(ctx, rowsPlan.SQL, rowsPlan.Args...)
	if err != nil {
		return QueryResponse{}, err
	}
	defer rows.Close()

	out := make(map[string]Summary)
	for rows.Next() {
		var bucket string
		var summary Summary
		if err := rows.Scan(
			&bucket,
			&summary.Impressions,
			&summary.Clicks,
			&summary.Conversions,
			&summary.Spent,
			&summary.Income,
			&summary.ConversionsApproved,
			&summary.IncomeApproved,
			&summary.CTR,
		); err != nil {
			return QueryResponse{}, err
		}
		out[bucket] = summary
	}
	if err := rows.Err(); err != nil {
		return QueryResponse{}, err
	}

	var totals Summary
	if err := r.db.QueryRowContext(
		ctx,
		totalsPlan.SQL,
		totalsPlan.Args...,
	).Scan(
		&totals.Impressions,
		&totals.Clicks,
		&totals.Conversions,
		&totals.Spent,
		&totals.Income,
		&totals.ConversionsApproved,
		&totals.IncomeApproved,
		&totals.CTR,
	); err != nil {
		return QueryResponse{}, err
	}

	return QueryResponse{Rows: out, Totals: totals}, nil
}

func (r *ClickHouseRepository) Calculator(
	ctx context.Context,
	req TrafficSegmentRequest,
) (CalculatorResponse, error) {
	plan, err := buildCalculatorPlan(req, r.trafficTable)
	if err != nil {
		return CalculatorResponse{}, err
	}

	var out CalculatorResponse
	if err := r.db.QueryRowContext(ctx, plan.SQL, plan.Args...).Scan(
		&out.PotentialImpressions,
	); err != nil {
		return CalculatorResponse{}, err
	}

	return out, nil
}

func (r *ClickHouseRepository) RecommendBid(
	ctx context.Context,
	req TrafficSegmentRequest,
) (RecommendBidResponse, error) {
	plan, err := buildRecommendBidPlan(req, r.trafficTable)
	if err != nil {
		return RecommendBidResponse{}, err
	}

	var out RecommendBidResponse
	if err := r.db.QueryRowContext(ctx, plan.SQL, plan.Args...).Scan(
		&out.AverageBid,
	); err != nil {
		return RecommendBidResponse{}, err
	}

	return out, nil
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

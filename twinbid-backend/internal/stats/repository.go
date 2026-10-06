package stats

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"

	"twinbid-backend/internal/config"
)

type ClickHouseRepository struct {
	db           *sql.DB
	table        string
	trafficTable string

	invalidEntityProbeMu             sync.Mutex
	lastInvalidEntityProbe           time.Time
	filteredInvalidCampaignIDSamples []string
}

const filteredInvalidEntityProbeInterval = 10 * time.Minute

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

func (r *ClickHouseRepository) CumulativeSpend(ctx context.Context) ([]CumulativeSpendTotal, error) {
	query, err := buildCumulativeSpendQuery(r.table)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	totals := make([]CumulativeSpendTotal, 0)
	for rows.Next() {
		var total CumulativeSpendTotal
		if err := rows.Scan(&total.EntityType, &total.EntityID, &total.Amount); err != nil {
			return nil, err
		}
		totals = append(totals, total)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return r.appendFilteredNonCanonicalUUIDLikeCampaignSamples(ctx, totals), nil
}

func (r *ClickHouseRepository) appendFilteredNonCanonicalUUIDLikeCampaignSamples(
	ctx context.Context,
	totals []CumulativeSpendTotal,
) []CumulativeSpendTotal {
	if r == nil || r.db == nil {
		return totals
	}

	r.invalidEntityProbeMu.Lock()
	defer r.invalidEntityProbeMu.Unlock()

	now := time.Now()
	if r.lastInvalidEntityProbe.IsZero() || now.Sub(r.lastInvalidEntityProbe) >= filteredInvalidEntityProbeInterval {
		// Advance the probe clock before issuing the query so a failing diagnostic
		// query cannot hammer ClickHouse every spend-sync cycle. The last known
		// samples remain cached until a later successful probe replaces them.
		r.lastInvalidEntityProbe = now

		query, err := buildFilteredNonCanonicalUUIDLikeCampaignQuery(r.table)
		if err != nil {
			log.Printf("[SPEND_SYNC][INVALID_ENTITY_DIAGNOSTIC_ERROR] build query: %v", err)
		} else {
			rows, queryErr := r.db.QueryContext(ctx, query)
			if queryErr != nil {
				log.Printf("[SPEND_SYNC][INVALID_ENTITY_DIAGNOSTIC_ERROR] query ClickHouse: %v", queryErr)
			} else {
				samples := make([]string, 0, 5)
				for rows.Next() {
					var entityID string
					if scanErr := rows.Scan(&entityID); scanErr != nil {
						log.Printf("[SPEND_SYNC][INVALID_ENTITY_DIAGNOSTIC_ERROR] scan ClickHouse: %v", scanErr)
						break
					}
					samples = append(samples, strings.TrimSpace(entityID))
				}
				if rowsErr := rows.Err(); rowsErr != nil {
					log.Printf("[SPEND_SYNC][INVALID_ENTITY_DIAGNOSTIC_ERROR] iterate ClickHouse: %v", rowsErr)
				} else {
					r.filteredInvalidCampaignIDSamples = samples
				}
				_ = rows.Close()
			}
		}
	}

	for _, entityID := range r.filteredInvalidCampaignIDSamples {
		if entityID == "" {
			continue
		}
		totals = append(totals, CumulativeSpendTotal{
			EntityType:     "campaign",
			EntityID:       entityID,
			Amount:         "0",
			DiagnosticOnly: true,
		})
	}
	return totals
}

func (r *ClickHouseRepository) POPRecoveryEventsAfter(ctx context.Context, cursor POPRecoveryCursor, limit int) ([]POPRecoveryEvent, error) {
	if limit <= 0 {
		limit = 5000
	}
	query, err := buildPOPRecoveryEventsAfterQuery(popRecoveryEventsTable)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, query, cursor.RecoveryAtMS, cursor.RecoveryAtMS, cursor.SourceKey, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]POPRecoveryEvent, 0)
	for rows.Next() {
		var event POPRecoveryEvent
		if err := rows.Scan(&event.RecoveryAtMS, &event.SourceKey, &event.UserID, &event.CampaignID, &event.Amount); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (r *ClickHouseRepository) CumulativePOPRecoveredSpend(ctx context.Context) ([]POPRecoveredSpendTotal, error) {
	query, err := buildPOPRecoveredSpendQuery(r.table)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	totals := make([]POPRecoveredSpendTotal, 0)
	for rows.Next() {
		var total POPRecoveredSpendTotal
		if err := rows.Scan(&total.UserID, &total.CampaignID, &total.Amount); err != nil {
			return nil, err
		}
		totals = append(totals, total)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return totals, nil
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

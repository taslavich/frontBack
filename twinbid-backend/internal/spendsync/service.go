package spendsync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"twinbid-backend/internal/stats"
)

type source interface {
	CumulativeSpend(ctx context.Context) ([]stats.CumulativeSpendTotal, error)
}

type Service struct {
	postgres *sql.DB
	source   source
}

type Result struct {
	SourceRows       int
	UserTotals       int
	CampaignTotals   int
	UpdatedUsers     int64
	UpdatedCampaigns int64
}

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

	totals, err := s.source.CumulativeSpend(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("query ClickHouse cumulative spend: %w", err)
	}

	userIDs, userAmounts, campaignIDs, campaignAmounts, err := splitTotals(totals)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		SourceRows:     len(totals),
		UserTotals:     len(userIDs),
		CampaignTotals: len(campaignIDs),
	}
	if len(userIDs) == 0 && len(campaignIDs) == 0 {
		return result, nil
	}

	tx, err := s.postgres.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, fmt.Errorf("begin spend sync transaction: %w", err)
	}
	defer tx.Rollback()

	if len(userIDs) > 0 {
		const updateUsers = `
WITH incoming(id, cum_done_dollars) AS (
    SELECT * FROM unnest($1::uuid[], $2::numeric[])
)
UPDATE users AS u
SET promo_spend_remaining = GREATEST(
        0,
        u.promo_spend_remaining - GREATEST(incoming.cum_done_dollars - u.cum_done_dollars, 0)
    ),
    cum_done_dollars = incoming.cum_done_dollars
FROM incoming
WHERE u.id = incoming.id`
		execResult, err := tx.ExecContext(ctx, updateUsers, pq.Array(userIDs), pq.Array(userAmounts))
		if err != nil {
			return Result{}, fmt.Errorf("bulk update users cumulative spend: %w", err)
		}
		result.UpdatedUsers, err = execResult.RowsAffected()
		if err != nil {
			return Result{}, fmt.Errorf("users cumulative spend rows affected: %w", err)
		}
	}

	if len(campaignIDs) > 0 {
		const updateCampaigns = `
WITH incoming(id, cum_done_dollars) AS (
    SELECT * FROM unnest($1::uuid[], $2::numeric[])
)
UPDATE campaigns AS c
SET cum_done_dollars = incoming.cum_done_dollars
FROM incoming
WHERE c.campaign_id = incoming.id`
		execResult, err := tx.ExecContext(ctx, updateCampaigns, pq.Array(campaignIDs), pq.Array(campaignAmounts))
		if err != nil {
			return Result{}, fmt.Errorf("bulk update campaigns cumulative spend: %w", err)
		}
		result.UpdatedCampaigns, err = execResult.RowsAffected()
		if err != nil {
			return Result{}, fmt.Errorf("campaigns cumulative spend rows affected: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("commit spend sync transaction: %w", err)
	}
	return result, nil
}

func splitTotals(totals []stats.CumulativeSpendTotal) ([]string, []string, []string, []string, error) {
	userAmountsByID := make(map[string]string)
	campaignAmountsByID := make(map[string]string)

	for i, total := range totals {
		entityType := strings.ToLower(strings.TrimSpace(total.EntityType))
		entityID := strings.TrimSpace(total.EntityID)
		amount := strings.TrimSpace(total.Amount)

		if _, err := uuid.Parse(entityID); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("invalid ClickHouse cumulative spend entity_id at row %d: %w", i, err)
		}
		value, err := strconv.ParseFloat(amount, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return nil, nil, nil, nil, fmt.Errorf("invalid ClickHouse cumulative spend amount at row %d: %q", i, amount)
		}

		switch entityType {
		case "user":
			userAmountsByID[entityID] = amount
		case "campaign":
			campaignAmountsByID[entityID] = amount
		default:
			return nil, nil, nil, nil, fmt.Errorf("invalid ClickHouse cumulative spend entity_type at row %d: %q", i, total.EntityType)
		}
	}

	userIDs, userAmounts := sortedTotals(userAmountsByID)
	campaignIDs, campaignAmounts := sortedTotals(campaignAmountsByID)
	return userIDs, userAmounts, campaignIDs, campaignAmounts, nil
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

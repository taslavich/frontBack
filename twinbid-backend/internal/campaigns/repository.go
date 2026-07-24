package campaigns

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"twinbid-backend/internal/db"
	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
)

type Repository struct{ db *sql.DB }

func NewRepository(dbConn *sql.DB) *Repository { return &Repository{db: dbConn} }

func (r *Repository) List(ctx context.Context, userID string) ([]models.Campaign, error) {
	rows, err := r.db.QueryContext(ctx, baseCampaignSelect+` WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Campaign
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) Get(ctx context.Context, campaignID string) (models.Campaign, error) {
	row := r.db.QueryRowContext(ctx, baseCampaignSelect+` WHERE campaign_id = $1`, campaignID)
	c, err := scanCampaign(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Campaign{}, httpx.NotFound("campaign not found")
	}
	return c, err
}

func (r *Repository) GetFormat(ctx context.Context, campaignID string) (string, error) {
	var f string
	err := r.db.QueryRowContext(ctx, `SELECT format_type FROM campaigns WHERE campaign_id = $1`, campaignID).Scan(&f)
	if errors.Is(err, sql.ErrNoRows) {
		return "", httpx.NotFound("campaign not found")
	}
	return f, err
}

func (r *Repository) Create(ctx context.Context, c models.Campaign) (models.Campaign, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO campaigns (
			user_id, campaign_name, format_type, brand_name, h, w, status, traffic_type, vertical, pricing_model,
			base_price, evenness_by_slot_mode, goal_total_dollars, cum_done_dollars,
			no_budget_notified, start_ts, end_ts, active_intervals, country, language, device_type, os, browser, site_id, ip,
			quality_type, traffic_reset_version
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27
		)
		RETURNING `+campaignSelectColumns+`
	`, c.UserID, c.CampaignName, c.FormatType, c.BrandName, c.H, c.W, c.Status, c.TrafficType, jsonArg(c.Vertical), c.PricingModel,
		c.BasePrice, c.EvennessBySlotMode, c.GoalTotalDollars, c.CumDoneDollars, c.NoBudgetNotified, c.StartTS, c.EndTS,
		jsonArg(c.ActiveIntervals), jsonArg(c.Country), jsonArg(c.Language), jsonArg(c.DeviceType), jsonArg(c.OS), jsonArg(c.Browser),
		jsonArg(c.SiteID), jsonArg(c.IP), c.QualityType, c.TrafficResetVersion)
	return scanCampaign(row)
}

// UpdateLocked locks the current campaign row, lets the service apply and validate
// a complete new state, and persists the campaign together with its reset version
// in one transaction. This prevents lost updates and lost reset events.
func (r *Repository) UpdateLocked(
	ctx context.Context,
	campaignID string,
	apply func(current *models.Campaign) error,
) (models.Campaign, error) {
	if apply == nil {
		return models.Campaign{}, errors.New("campaign update callback is nil")
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return models.Campaign{}, fmt.Errorf("begin campaign update transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := scanCampaign(tx.QueryRowContext(ctx, baseCampaignSelect+` WHERE campaign_id = $1 FOR UPDATE`, campaignID))
	if errors.Is(err, sql.ErrNoRows) {
		return models.Campaign{}, httpx.NotFound("campaign not found")
	}
	if err != nil {
		return models.Campaign{}, fmt.Errorf("lock campaign: %w", err)
	}
	if err := apply(&current); err != nil {
		return models.Campaign{}, err
	}

	updated, err := updateCampaignRow(ctx, tx, current)
	if err != nil {
		return models.Campaign{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.Campaign{}, fmt.Errorf("commit campaign update: %w", err)
	}
	return updated, nil
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func updateCampaignRow(ctx context.Context, q queryRower, c models.Campaign) (models.Campaign, error) {
	row := q.QueryRowContext(ctx, `
		UPDATE campaigns SET
			campaign_name=$2, format_type=$3, brand_name=$4, h=$5, w=$6, status=$7, traffic_type=$8, vertical=$9,
			pricing_model=$10, base_price=$11, evenness_by_slot_mode=$12,
			goal_total_dollars=$13, cum_done_dollars=$14, no_budget_notified=$15, start_ts=$16, end_ts=$17, active_intervals=$18,
			country=$19, language=$20, device_type=$21, os=$22, browser=$23, site_id=$24, ip=$25, quality_type=$26,
			traffic_reset_version=$27, updated_at=NOW()
		WHERE campaign_id=$1
		RETURNING `+campaignSelectColumns+`
	`, c.CampaignID, c.CampaignName, c.FormatType, c.BrandName, c.H, c.W, c.Status, c.TrafficType, jsonArg(c.Vertical),
		c.PricingModel, c.BasePrice, c.EvennessBySlotMode, c.GoalTotalDollars, c.CumDoneDollars,
		c.NoBudgetNotified, c.StartTS, c.EndTS, jsonArg(c.ActiveIntervals), jsonArg(c.Country), jsonArg(c.Language),
		jsonArg(c.DeviceType), jsonArg(c.OS), jsonArg(c.Browser), jsonArg(c.SiteID), jsonArg(c.IP), c.QualityType,
		c.TrafficResetVersion)
	out, err := scanCampaign(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Campaign{}, httpx.NotFound("campaign not found")
	}
	if err != nil {
		return models.Campaign{}, fmt.Errorf("update campaign: %w", err)
	}
	return out, nil
}

func (r *Repository) Delete(ctx context.Context, userID, campaignID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM campaigns WHERE user_id=$1 AND campaign_id=$2`, userID, campaignID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return httpx.NotFound("campaign not found")
	}
	return nil
}

const campaignSelectColumns = `campaign_id, user_id, campaign_name, format_type, brand_name, h, w, status, traffic_type, vertical,
	pricing_model, base_price, evenness_by_slot_mode, goal_total_dollars, cum_done_dollars, no_budget_notified,
	start_ts, end_ts, active_intervals, country, language, device_type, os, browser, site_id, ip, quality_type,
	traffic_reset_version, updated_at`

const baseCampaignSelect = `SELECT ` + campaignSelectColumns + ` FROM campaigns`

type scanner interface{ Scan(dest ...any) error }

func scanCampaign(s scanner) (models.Campaign, error) {
	var c models.Campaign
	var brand sql.NullString
	var h, w sql.NullInt64
	var updatedAt sql.NullTime
	var verticalRaw, activeRaw, countryRaw, languageRaw, deviceRaw, osRaw, browserRaw, siteRaw, ipRaw []byte
	err := s.Scan(&c.CampaignID, &c.UserID, &c.CampaignName, &c.FormatType, &brand, &h, &w, &c.Status, &c.TrafficType, &verticalRaw,
		&c.PricingModel, &c.BasePrice, &c.EvennessBySlotMode, &c.GoalTotalDollars, &c.CumDoneDollars, &c.NoBudgetNotified,
		&c.StartTS, &c.EndTS, &activeRaw, &countryRaw, &languageRaw, &deviceRaw, &osRaw, &browserRaw, &siteRaw, &ipRaw,
		&c.QualityType, &c.TrafficResetVersion, &updatedAt)
	if err != nil {
		return models.Campaign{}, err
	}
	if updatedAt.Valid {
		c.UpdatedAt = updatedAt.Time.UTC()
	}
	if brand.Valid {
		c.BrandName = &brand.String
	}
	if h.Valid {
		v := int(h.Int64)
		c.H = &v
	}
	if w.Valid {
		v := int(w.Int64)
		c.W = &v
	}
	c.Vertical, err = db.UnmarshalTargeting(verticalRaw)
	if err != nil {
		return models.Campaign{}, err
	}
	c.ActiveIntervals, err = db.UnmarshalIntervals(activeRaw)
	if err != nil {
		return models.Campaign{}, err
	}
	if c.Country, err = db.UnmarshalTargetingFilter(countryRaw); err != nil {
		return models.Campaign{}, err
	}
	if c.Language, err = db.UnmarshalTargetingFilter(languageRaw); err != nil {
		return models.Campaign{}, err
	}
	if c.DeviceType, err = db.UnmarshalTargetingFilter(deviceRaw); err != nil {
		return models.Campaign{}, err
	}
	if c.OS, err = db.UnmarshalTargetingFilter(osRaw); err != nil {
		return models.Campaign{}, err
	}
	if c.Browser, err = db.UnmarshalTargetingFilter(browserRaw); err != nil {
		return models.Campaign{}, err
	}
	if c.SiteID, err = db.UnmarshalTargetingFilter(siteRaw); err != nil {
		return models.Campaign{}, err
	}
	if c.IP, err = db.UnmarshalTargetingFilter(ipRaw); err != nil {
		return models.Campaign{}, err
	}
	return c, nil
}

func jsonArg(v any) any {
	b, _ := json.Marshal(v)
	return string(b)
}

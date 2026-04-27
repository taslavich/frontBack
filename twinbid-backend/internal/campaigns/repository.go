package campaigns

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

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

func (r *Repository) Get(ctx context.Context, userID, campaignID string) (models.Campaign, error) {
	row := r.db.QueryRowContext(ctx, baseCampaignSelect+` WHERE user_id = $1 AND campaign_id = $2`, userID, campaignID)
	c, err := scanCampaign(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Campaign{}, httpx.NotFound("campaign not found")
	}
	return c, err
}

func (r *Repository) GetFormat(ctx context.Context, userID, campaignID string) (string, error) {
	var f string
	err := r.db.QueryRowContext(ctx, `SELECT format_type FROM campaigns WHERE user_id = $1 AND campaign_id = $2`, userID, campaignID).Scan(&f)
	if errors.Is(err, sql.ErrNoRows) {
		return "", httpx.NotFound("campaign not found")
	}
	return f, err
}

func (r *Repository) Create(ctx context.Context, c models.Campaign) (models.Campaign, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO campaigns (
			user_id, campaign_name, format_type, brand_name, h, w, status, traffic_type, vertical, pricing_model,
			base_price_cpm, base_price_cpc, evenness_by_slot_mode, goal_total_dollars, cum_done_dollars,
			start_ts, end_ts, active_intervals, country, language, device_type, os, browser, site_id, ip, quality_type
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26
		)
		RETURNING campaign_id, user_id, campaign_name, format_type, brand_name, h, w, status, traffic_type, vertical,
			pricing_model, base_price_cpm, base_price_cpc, evenness_by_slot_mode, goal_total_dollars, cum_done_dollars,
			start_ts, end_ts, active_intervals, country, language, device_type, os, browser, site_id, ip, quality_type
	`, c.UserID, c.CampaignName, c.FormatType, c.BrandName, c.H, c.W, c.Status, c.TrafficType, jsonArg(c.Vertical), c.PricingModel,
		c.BasePriceCPM, c.BasePriceCPC, c.EvennessBySlotMode, c.GoalTotalDollars, c.CumDoneDollars, c.StartTS, c.EndTS,
		jsonArg(c.ActiveIntervals), jsonArg(c.Country), jsonArg(c.Language), jsonArg(c.DeviceType), jsonArg(c.OS), jsonArg(c.Browser), jsonArg(c.SiteID), jsonArg(c.IP), c.QualityType)
	return scanCampaign(row)
}

func (r *Repository) Update(ctx context.Context, c models.Campaign) (models.Campaign, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE campaigns SET
			campaign_name=$3, format_type=$4, brand_name=$5, h=$6, w=$7, status=$8, traffic_type=$9, vertical=$10,
			pricing_model=$11, base_price_cpm=$12, base_price_cpc=$13, evenness_by_slot_mode=$14,
			goal_total_dollars=$15, cum_done_dollars=$16, start_ts=$17, end_ts=$18, active_intervals=$19,
			country=$20, language=$21, device_type=$22, os=$23, browser=$24, site_id=$25, ip=$26, quality_type=$27, updated_at=NOW()
		WHERE user_id=$1 AND campaign_id=$2
		RETURNING campaign_id, user_id, campaign_name, format_type, brand_name, h, w, status, traffic_type, vertical,
			pricing_model, base_price_cpm, base_price_cpc, evenness_by_slot_mode, goal_total_dollars, cum_done_dollars,
			start_ts, end_ts, active_intervals, country, language, device_type, os, browser, site_id, ip, quality_type
	`, c.UserID, c.CampaignID, c.CampaignName, c.FormatType, c.BrandName, c.H, c.W, c.Status, c.TrafficType, jsonArg(c.Vertical),
		c.PricingModel, c.BasePriceCPM, c.BasePriceCPC, c.EvennessBySlotMode, c.GoalTotalDollars, c.CumDoneDollars,
		c.StartTS, c.EndTS, jsonArg(c.ActiveIntervals), jsonArg(c.Country), jsonArg(c.Language), jsonArg(c.DeviceType), jsonArg(c.OS), jsonArg(c.Browser), jsonArg(c.SiteID), jsonArg(c.IP), c.QualityType)
	out, err := scanCampaign(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Campaign{}, httpx.NotFound("campaign not found")
	}
	return out, err
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

const baseCampaignSelect = `SELECT campaign_id, user_id, campaign_name, format_type, brand_name, h, w, status, traffic_type, vertical,
	pricing_model, base_price_cpm, base_price_cpc, evenness_by_slot_mode, goal_total_dollars, cum_done_dollars,
	start_ts, end_ts, active_intervals, country, language, device_type, os, browser, site_id, ip, quality_type FROM campaigns`

type scanner interface{ Scan(dest ...any) error }

func scanCampaign(s scanner) (models.Campaign, error) {
	var c models.Campaign
	var brand sql.NullString
	var h, w sql.NullInt64
	var verticalRaw, activeRaw, countryRaw, languageRaw, deviceRaw, osRaw, browserRaw, siteRaw, ipRaw []byte
	err := s.Scan(&c.CampaignID, &c.UserID, &c.CampaignName, &c.FormatType, &brand, &h, &w, &c.Status, &c.TrafficType, &verticalRaw,
		&c.PricingModel, &c.BasePriceCPM, &c.BasePriceCPC, &c.EvennessBySlotMode, &c.GoalTotalDollars, &c.CumDoneDollars,
		&c.StartTS, &c.EndTS, &activeRaw, &countryRaw, &languageRaw, &deviceRaw, &osRaw, &browserRaw, &siteRaw, &ipRaw, &c.QualityType)
	if err != nil {
		return models.Campaign{}, err
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
	c.Country, err = db.UnmarshalTargeting(countryRaw)
	if err != nil {
		return models.Campaign{}, err
	}
	c.Language, err = db.UnmarshalTargeting(languageRaw)
	if err != nil {
		return models.Campaign{}, err
	}
	c.DeviceType, err = db.UnmarshalTargeting(deviceRaw)
	if err != nil {
		return models.Campaign{}, err
	}
	c.OS, err = db.UnmarshalTargeting(osRaw)
	if err != nil {
		return models.Campaign{}, err
	}
	c.Browser, err = db.UnmarshalTargeting(browserRaw)
	if err != nil {
		return models.Campaign{}, err
	}
	c.SiteID, err = db.UnmarshalTargeting(siteRaw)
	if err != nil {
		return models.Campaign{}, err
	}
	c.IP, err = db.UnmarshalTargeting(ipRaw)
	if err != nil {
		return models.Campaign{}, err
	}
	return c, nil
}

func jsonArg(v any) any {
	b, _ := json.Marshal(v)
	return string(b)
}

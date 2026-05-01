package campaigns

import (
	"encoding/json"
	"time"

	"twinbid-backend/internal/models"
)

type UpsertCampaignRequest struct {
	CampaignName       string                    `json:"campaign_name"`
	FormatType         string                    `json:"format_type"`
	BrandName          *string                   `json:"brand_name"`
	H                  *int                      `json:"h"`
	W                  *int                      `json:"w"`
	Status             string                    `json:"status"`
	TrafficType        string                    `json:"traffic_type"`
	Vertical           models.TargetingMap       `json:"vertical"`
	PricingModel       string                    `json:"pricing_model"`
	BasePriceCPM       float64                   `json:"base_price_cpm"`
	BasePriceCPC       float64                   `json:"base_price_cpc"`
	EvennessBySlotMode bool                      `json:"evenness_by_slot_mode"`
	GoalTotalDollars   float64                   `json:"goal_total_dollars"`
	StartTS            time.Time                 `json:"start_ts"`
	EndTS              time.Time                 `json:"end_ts"`
	ActiveIntervals    []models.ScheduleInterval `json:"active_intervals"`
	Country            models.TargetingMap       `json:"country"`
	Language           models.TargetingMap       `json:"language"`
	DeviceType         models.TargetingMap       `json:"device_type"`
	OS                 models.TargetingMap       `json:"os"`
	Browser            models.TargetingMap       `json:"browser"`
	SiteID             models.TargetingMap       `json:"site_id"`
	IP                 models.TargetingMap       `json:"ip"`
	QualityType        string                    `json:"quality_type"`
}

type PatchCampaignRequest struct {
	CampaignName       *string                    `json:"campaign_name"`
	FormatType         *string                    `json:"format_type"`
	BrandName          *string                    `json:"-"`
	BrandNameSet       bool                       `json:"-"`
	H                  *int                       `json:"-"`
	HSet               bool                       `json:"-"`
	W                  *int                       `json:"-"`
	WSet               bool                       `json:"-"`
	Status             *string                    `json:"status"`
	TrafficType        *string                    `json:"traffic_type"`
	Vertical           *models.TargetingMap       `json:"vertical"`
	PricingModel       *string                    `json:"pricing_model"`
	BasePriceCPM       *float64                   `json:"base_price_cpm"`
	BasePriceCPC       *float64                   `json:"base_price_cpc"`
	EvennessBySlotMode *bool                      `json:"evenness_by_slot_mode"`
	GoalTotalDollars   *float64                   `json:"goal_total_dollars"`
	CumDoneDollars     *float64                   `json:"cum_done_dollars"`
	StartTS            *time.Time                 `json:"start_ts"`
	EndTS              *time.Time                 `json:"end_ts"`
	ActiveIntervals    *[]models.ScheduleInterval `json:"active_intervals"`
	Country            *models.TargetingMap       `json:"country"`
	Language           *models.TargetingMap       `json:"language"`
	DeviceType         *models.TargetingMap       `json:"device_type"`
	OS                 *models.TargetingMap       `json:"os"`
	Browser            *models.TargetingMap       `json:"browser"`
	SiteID             *models.TargetingMap       `json:"site_id"`
	IP                 *models.TargetingMap       `json:"ip"`
	QualityType        *string                    `json:"quality_type"`
	NoBudgetNotified   *bool                      `json:"no_budget_notified"`
}

func (p *PatchCampaignRequest) UnmarshalJSON(data []byte) error {
	type alias PatchCampaignRequest
	var aux alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["brand_name"]; ok {
		aux.BrandNameSet = true
		if string(v) != "null" {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return err
			}
			aux.BrandName = &s
		}
	}
	if v, ok := raw["h"]; ok {
		aux.HSet = true
		if string(v) != "null" {
			var n int
			if err := json.Unmarshal(v, &n); err != nil {
				return err
			}
			aux.H = &n
		}
	}
	if v, ok := raw["w"]; ok {
		aux.WSet = true
		if string(v) != "null" {
			var n int
			if err := json.Unmarshal(v, &n); err != nil {
				return err
			}
			aux.W = &n
		}
	}
	*p = PatchCampaignRequest(aux)
	return nil
}

package models

import "time"

type Campaign struct {
	CampaignID         string         `json:"campaing_id"`
	UserID             string         `json:"user_id"`
	CampaignName       string         `json:"campaign_name"`
	FormatType         string         `json:"format_type"`
	BrandName          *string        `json:"brand_name,omitempty"`
	H                  *int           `json:"h,omitempty"`
	W                  *int           `json:"w,omitempty"`
	Status             string         `json:"status"`
	TrafficType        string         `json:"traffic_type"`
	Vertical           []string       `json:"vertical"`
	PricingModel       string         `json:"pricing_model"`
	BasePriceCPM       float64        `json:"base_price_cpm"`
	BasePriceCPC       float64        `json:"base_price_cpc"`
	EvennessBySlotMode bool           `json:"evenness_by_slot_mode"`
	GoalTotalDollars   float64        `json:"goal_total_dollars"`
	CumDoneDollars     float64        `json:"cum_done_dollars"`
	StartTS            time.Time      `json:"start_ts"`
	EndTS              time.Time      `json:"end_ts"`
	ActiveIntervals    [][2]string    `json:"active_intervals"`
	Country            map[string]int `json:"country"`
	Language           map[string]int `json:"language"`
	DeviceType         map[string]int `json:"device_type"`
	OS                 map[string]int `json:"os"`
	Browser            map[string]int `json:"browser"`
	SiteID             map[string]int `json:"site_id"`
	IP                 map[string]int `json:"ip"`
}

type Creative struct {
	ID             string         `json:"id"`
	CampaignID     string         `json:"campaign_id"`
	CreativeName   string         `json:"creative_name"`
	Link           string         `json:"link"`
	TrackersMacros map[string]int `json:"trackers_macros"`
	W              *int           `json:"w,omitempty"`
	H              *int           `json:"h,omitempty"`
	S3FilePath     *string        `json:"s3_file_path,omitempty"`
	FileFormat     *string        `json:"file_format,omitempty"`
	Title          *string        `json:"title,omitempty"`
	Description    *string        `json:"description,omitempty"`
	CreativeURL    *string        `json:"creative_url,omitempty"`
}

type UserTransaction struct {
	ID                   string    `json:"id"`
	UserID               string    `json:"user_id"`
	TransactionTime      time.Time `json:"transaction_time"`
	TransactionID        string    `json:"transaction_id"`
	PaymentMethod        string    `json:"payment_method"`
	BonusAmount          float64   `json:"bonus_amount"`
	PromocodeID          *string   `json:"promocode_id"`
	TransactionHash      *string   `json:"transaction_hash"`
	DepositAmount        float64   `json:"deposit_amount"`
	TotalBalanceIncrease float64   `json:"total_balance_increase"`
	Status               string    `json:"status"`
	Currency             string    `json:"currency"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type Promocode struct {
	ID           string     `json:"id"`
	PromocodeTxt string     `json:"promocode_text"`
	BonusPercent float64    `json:"bonus_percent"`
	UsageCount   int        `json:"usage_count"`
	UsageLimit   *int       `json:"usage_limit"`
	ValidFrom    *time.Time `json:"valid_from"`
	ValidTo      *time.Time `json:"valid_to"`
}

type Notification struct {
	ID            string   `json:"id"`
	UserID        string   `json:"user_id"`
	TransactionID *string  `json:"transaction_id"`
	CampaignID    *string  `json:"campaign_id"`
	DepositAmount *float64 `json:"deposit_amount"`
	Status        string   `json:"status"`
	Text          string   `json:"text"`
	Type          string   `json:"type"`
}

package models

import (
	"encoding/json"
	"time"
)

type TargetingMap map[string]int
type MacroMap map[string]string

type TargetingFilter struct {
	IsWhiteList bool     `json:"isWhiteList"`
	Objects     []string `json:"objects"`
}

func NormalizeTargetingFilter(v TargetingFilter) TargetingFilter {
	if v.Objects == nil {
		v.Objects = []string{}
	}
	return v
}

type ScheduleInterval [2]string

type User struct {
	ID                           string  `json:"-"`
	Login                        string  `json:"login"`
	Mail                         string  `json:"mail"`
	Name                         string  `json:"name"`
	Verified                     bool    `json:"-"`
	Telegram                     *string `json:"telegram"`
	ManagerTelegram              string  `json:"manager_telegram"`
	GoalTotalDollars             float64 `json:"-"`
	CumDoneDollars               float64 `json:"-"`
	Balance                      float64 `json:"balance"`
	Timezone                     string  `json:"timezone"`
	EmailNotifications           bool    `json:"email_notifications"`
	CampaignStatusNotifications  bool    `json:"campaign_status_notifications"`
	LowBalanceNotifications      bool    `json:"low_balance_notifications"`
	CampaignBalanseNotifications bool    `json:"campaign_balanse_notifications"`
	BalanceTreshold              float64 `json:"balance_treshold"`
	LowBalanceNotified           bool    `json:"low_balance_notified"`
	PartnerID                    string  `json:"partner_id"`
	Partner                      *string `json:"partner"`
}

type Campaign struct {
	CampaignID          string             `json:"campaign_id"`
	UserID              string             `json:"user_id"`
	CampaignName        string             `json:"campaign_name"`
	FormatType          string             `json:"format_type"`
	BrandName           *string            `json:"brand_name"`
	H                   *int               `json:"h"`
	W                   *int               `json:"w"`
	Status              string             `json:"status"`
	TrafficType         string             `json:"traffic_type"`
	Vertical            TargetingMap       `json:"vertical"`
	PricingModel        string             `json:"pricing_model"`
	BasePrice           float64            `json:"base_price"`
	TypeModel           int                `json:"type_model"`
	EvennessBySlotMode  bool               `json:"evenness_by_slot_mode"`
	BlockVPN            bool               `json:"block_vpn"`
	GoalTotalDollars    float64            `json:"goal_total_dollars"`
	CumDoneDollars      float64            `json:"cum_done_dollars"`
	NoBudgetNotified    bool               `json:"no_budget_notified"`
	StartTS             time.Time          `json:"start_ts"`
	EndTS               time.Time          `json:"end_ts"`
	ActiveIntervals     []ScheduleInterval `json:"active_intervals"`
	Country             TargetingFilter    `json:"country"`
	Language            TargetingFilter    `json:"language"`
	DeviceType          TargetingFilter    `json:"device_type"`
	OS                  TargetingFilter    `json:"os"`
	Browser             TargetingFilter    `json:"browser"`
	SiteID              TargetingFilter    `json:"site_id"`
	IP                  TargetingFilter    `json:"ip"`
	QualityType         string             `json:"quality_type"`
	TrafficResetVersion int64              `json:"-"`
	UpdatedAt           time.Time          `json:"-"`
}

type Creative struct {
	ID             string   `json:"id"`
	CampaignID     string   `json:"campaign_id"`
	CreativeName   string   `json:"creative_name"`
	ADM            string   `json:"adm"`
	BannerType     *string  `json:"banner_type,omitempty"`
	TrackersMacros MacroMap `json:"trackers_macros"`
	Macros         MacroMap `json:"macros"`
	W              *int     `json:"w,omitempty"`
	H              *int     `json:"h,omitempty"`
	Title          *string  `json:"title,omitempty"`
	Description    *string  `json:"description,omitempty"`
	ImageID        *string  `json:"image_id,omitempty"`
	ImageURL       *string  `json:"image_url,omitempty"`
	ImageName      *string  `json:"image_name,omitempty"`
	S3Key          *string  `json:"-"`
	ImageMimeType  *string  `json:"-"`
	ImageFormat    *string  `json:"-"`
	FormatType     string   `json:"-"`
}

type CreativeImage struct {
	ID           string    `json:"image_id"`
	UserID       string    `json:"-"`
	CampaignID   string    `json:"campaign_id"`
	CreativeID   *string   `json:"creative_id,omitempty"`
	S3Key        string    `json:"-"`
	WebURL       string    `json:"image_url"`
	OriginalName string    `json:"filename"`
	MimeType     string    `json:"mime_type"`
	FileFormat   string    `json:"file_format"`
	SizeBytes    int64     `json:"size_bytes"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type TopupStatus string

const (
	TopupDraft     TopupStatus = "draft"
	TopupPending   TopupStatus = "pending"
	TopupApproved  TopupStatus = "approved"
	TopupRejected  TopupStatus = "rejected"
	TopupCancelled TopupStatus = "cancelled"
)

type UserTransaction struct {
	ID                    string          `json:"id"`
	UserID                string          `json:"user_id"`
	TransactionTime       time.Time       `json:"transaction_time"`
	TransactionID         string          `json:"transaction_id"`
	PaymentChannel        string          `json:"payment_channel"`
	PaymentMethod         string          `json:"payment_method"`
	BonusAmount           float64         `json:"bonus_amount"`
	PromocodeID           *string         `json:"promocode_id"`
	PromocodeUsageApplied bool            `json:"-"`
	TransactionHash       *string         `json:"transaction_hash"`
	DepositAmount         float64         `json:"deposit_amount"`
	TotalBalanceIncrease  float64         `json:"total_balance_increase"`
	Status                TopupStatus     `json:"status"`
	Currency              string          `json:"currency"`
	PaymentURL            *string         `json:"payment_url,omitempty"`
	ProviderStatus        *string         `json:"provider_status,omitempty"`
	ProviderPaymentID     *string         `json:"provider_payment_id,omitempty"`
	ProviderTransactionID *string         `json:"provider_transaction_id,omitempty"`
	AmountPaid            *float64        `json:"amount_paid,omitempty"`
	AmountCredited        *float64        `json:"amount_credited,omitempty"`
	FeeService            *float64        `json:"fee_service,omitempty"`
	FeeNetwork            *float64        `json:"fee_network,omitempty"`
	CreditedAt            *time.Time      `json:"credited_at,omitempty"`
	InvoiceExpiresAt      *time.Time      `json:"invoice_expires_at,omitempty"`
	ProviderPayload       json.RawMessage `json:"-"`
	ProviderCheckAttempts int             `json:"-"`
	ProviderNextCheckAt   *time.Time      `json:"-"`
	ProviderLastError     *string         `json:"-"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

type Promocode struct {
	ID            string     `json:"id"`
	PromocodeText string     `json:"promocode_text"`
	BonusPercent  float64    `json:"bonus_percent"`
	UsageCount    int        `json:"usage_count"`
	UsageLimit    *int       `json:"usage_limit"`
	ValidFrom     *time.Time `json:"valid_from"`
	ValidTo       *time.Time `json:"valid_to"`
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

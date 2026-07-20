package stats

// FilterMode controls whether the selected values are included or excluded.
type FilterMode string

const (
	FilterModeInclude FilterMode = "include"
	FilterModeExclude FilterMode = "exclude"
)

// TrafficSegmentRequest matches TrafficSegmentRequest in the frontend.
type TrafficSegmentRequest struct {
	FormatType  string `json:"format_type"`
	TrafficType string `json:"traffic_type"`

	Country     []string   `json:"country,omitempty"`
	CountryMode FilterMode `json:"country_mode,omitempty"`

	Language     []string   `json:"language,omitempty"`
	LanguageMode FilterMode `json:"language_mode,omitempty"`

	DeviceType     []string   `json:"device_type,omitempty"`
	DeviceTypeMode FilterMode `json:"device_type_mode,omitempty"`

	OS     []string   `json:"os,omitempty"`
	OSMode FilterMode `json:"os_mode,omitempty"`

	Browser     []string   `json:"browser,omitempty"`
	BrowserMode FilterMode `json:"browser_mode,omitempty"`
}

type CalculatorResponse struct {
	PotentialImpressions uint64 `json:"potential_impressions"`
}

type RecommendBidResponse struct {
	AverageBid float64 `json:"average_bid"`
}

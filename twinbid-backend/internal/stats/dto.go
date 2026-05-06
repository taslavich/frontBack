package stats

// GroupBy is a single grouping selected by frontend.
type GroupBy string

const (
	GroupByDate       GroupBy = "date"
	GroupByHour       GroupBy = "hour"
	GroupByCountry    GroupBy = "country"
	GroupByOS         GroupBy = "os"
	GroupByBrowser    GroupBy = "browser"
	GroupByDeviceType GroupBy = "device_type"
	GroupBySiteID     GroupBy = "site_id"
	GroupByCampaign   GroupBy = "campaign"
)

// FilterBy is intentionally narrower than GroupBy: it matches frontend StatsFilterBy.
type FilterBy string

const (
	FilterByCountry    FilterBy = "country"
	FilterByOS         FilterBy = "os"
	FilterByBrowser    FilterBy = "browser"
	FilterByDeviceType FilterBy = "device_type"
)

type QueryRequest struct {
	From        string              `json:"from"`
	To          string              `json:"to"`
	CampaignIDs []string            `json:"campaign_ids,omitempty"`
	CreativeIDs []string            `json:"creative_ids,omitempty"`
	GroupBy     GroupBy             `json:"group_by"`
	Filters     map[string][]string `json:"filters,omitempty"`
}

type Summary struct {
	Impressions uint64  `json:"impressions"`
	Clicks      uint64  `json:"clicks"`
	Spent       float64 `json:"spent"`
	CTR         float64 `json:"ctr"`
}

type QueryResponse struct {
	Rows   map[string]Summary `json:"rows"`
	Totals Summary            `json:"totals"`
}

package stats

type QueryRequest struct {
	From        string              `json:"from"`
	To          string              `json:"to"`
	CampaignIDs []string            `json:"campaign_ids"`
	GroupBy     []string            `json:"group_by"`
	Filters     map[string][]string `json:"filters"`
}

type Row map[string]any

type QueryResponse struct {
	Rows   []Row   `json:"rows"`
	Totals Summary `json:"totals"`
}

type Summary struct {
	Impressions uint64  `json:"impressions"`
	Clicks      uint64  `json:"clicks"`
	Spent       float64 `json:"spent"`
	CTR         float64 `json:"ctr"`
}

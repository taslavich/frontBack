package stats

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"twinbid-backend/internal/httpx"
)

type groupSpec struct {
	selectExpr string
	groupExpr  string
	orderExpr  string
}

type sqlPlan struct {
	SQL  string
	Args []any
}

var safeIdentifierRE = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)?$`)

var groupColumns = map[GroupBy]groupSpec{
	GroupByDate: {
		selectExpr: "toString(event_date)",
		groupExpr:  "event_date",
		orderExpr:  "event_date",
	},
	GroupByHour: {
		selectExpr: "formatDateTime(toStartOfHour(event_hour), '%F %H:00', 'UTC')",
		groupExpr:  "toStartOfHour(event_hour)",
		orderExpr:  "toStartOfHour(event_hour)",
	},
	GroupByCountry: {
		selectExpr: "geo",
		groupExpr:  "geo",
	},
	GroupByOS: {
		selectExpr: "os",
		groupExpr:  "os",
	},
	GroupByBrowser: {
		selectExpr: "browser",
		groupExpr:  "browser",
	},
	GroupByDeviceType: {
		selectExpr: "device_type",
		groupExpr:  "device_type",
	},
	GroupBySiteID: {
		selectExpr: "site_id",
		groupExpr:  "site_id",
	},
	GroupByCampaign: {
		selectExpr: "toString(campaign_id)",
		groupExpr:  "campaign_id",
	},
}

var filterColumns = map[string]string{
	string(FilterByCountry):    "geo",
	string(FilterByOS):         "os",
	string(FilterByBrowser):    "browser",
	string(FilterByDeviceType): "device_type",
}

// Spent formula for the single endpoint /api/stats/query.
// It assumes that ads.agg_stats has the future `format` column and that the MV groups by it.
const spentExpression = `round(
    sum(
        multiIf(
            lowerUTF8(ifNull(format, '')) IN ('banner', 'native'), spend_views_table / 1000,
            lowerUTF8(ifNull(format, '')) = 'ipp', spend_clicks_table,
            lowerUTF8(ifNull(format, '')) = 'popunder', spend_clicks_table / 1000,
            0
        )
    ),
    2
)`

func buildStatsQueries(userID string, req QueryRequest, table string) (sqlPlan, sqlPlan, error) {
	if req.GroupBy == "" {
		req.GroupBy = GroupByCampaign
	}

	spec, ok := groupColumns[req.GroupBy]
	if !ok {
		return sqlPlan{}, sqlPlan{}, httpx.BadRequest("invalid group_by: " + string(req.GroupBy))
	}

	from, to, err := normalizeDates(req.From, req.To)
	if err != nil {
		return sqlPlan{}, sqlPlan{}, err
	}

	table, err = normalizeTable(table)
	if err != nil {
		return sqlPlan{}, sqlPlan{}, err
	}

	where, args, err := buildWhere(req, userID, from, to)
	if err != nil {
		return sqlPlan{}, sqlPlan{}, err
	}

	rowsSQL := fmt.Sprintf(`
SELECT
    %s AS bucket,
    toUInt64(sum(impressions)) AS impressions,
    toUInt64(sum(clicks)) AS clicks,
    %s AS spent,
    round(if(sum(impressions) = 0, 0, sum(clicks) * 100.0 / sum(impressions)), 2) AS ctr
FROM %s
WHERE %s
GROUP BY %s
%s`, spec.selectExpr, spentExpression, table, where, spec.groupExpr, buildOrderBy(spec))

	totalsSQL := fmt.Sprintf(`
SELECT
    toUInt64(sum(impressions)) AS impressions,
    toUInt64(sum(clicks)) AS clicks,
    %s AS spent,
    round(if(sum(impressions) = 0, 0, sum(clicks) * 100.0 / sum(impressions)), 2) AS ctr
FROM %s
WHERE %s`, spentExpression, table, where)

	return sqlPlan{SQL: rowsSQL, Args: args}, sqlPlan{SQL: totalsSQL, Args: append([]any(nil), args...)}, nil
}

func buildWhere(req QueryRequest, userID, from, to string) (string, []any, error) {
	if err := validateUUID(userID, "user_id"); err != nil {
		return "", nil, err
	}
	if err := validateUUIDList(req.CampaignIDs, "campaign_ids"); err != nil {
		return "", nil, err
	}
	if err := validateUUIDList(req.CreativeIDs, "creative_ids"); err != nil {
		return "", nil, err
	}

	parts := []string{"user_id = toUUID(?)", "event_date BETWEEN toDate(?) AND toDate(?)"}
	args := []any{userID, from, to}

	if len(req.CampaignIDs) > 0 {
		parts = append(parts, "campaign_id IN ("+uuidPlaceholders(len(req.CampaignIDs))+")")
		for _, id := range req.CampaignIDs {
			args = append(args, id)
		}
	}

	if len(req.CreativeIDs) > 0 {
		parts = append(parts, "creative_id IN ("+uuidPlaceholders(len(req.CreativeIDs))+")")
		for _, id := range req.CreativeIDs {
			args = append(args, id)
		}
	}

	for key, values := range req.Filters {
		if len(values) == 0 {
			continue
		}

		col, ok := filterColumns[key]
		if !ok {
			return "", nil, httpx.BadRequest("invalid filter: " + key)
		}

		parts = append(parts, fmt.Sprintf("%s IN (%s)", col, valuePlaceholders(len(values))))
		for _, value := range values {
			args = append(args, value)
		}
	}

	return strings.Join(parts, " AND "), args, nil
}

func normalizeDates(from, to string) (string, string, error) {
	if from == "" {
		from = time.Now().UTC().AddDate(0, -1, 0).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().UTC().Format("2006-01-02")
	}

	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		return "", "", httpx.BadRequest("invalid from date, expected YYYY-MM-DD")
	}
	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		return "", "", httpx.BadRequest("invalid to date, expected YYYY-MM-DD")
	}
	if fromDate.After(toDate) {
		return "", "", httpx.BadRequest("from must be <= to")
	}

	return from, to, nil
}

func normalizeTable(table string) (string, error) {
	table = strings.TrimSpace(table)
	if table == "" {
		table = "agg_stats"
	}
	if !safeIdentifierRE.MatchString(table) {
		return "", httpx.BadRequest("invalid ClickHouse stats table name")
	}
	return table, nil
}

func validateUUID(value, field string) error {
	if _, err := uuid.Parse(value); err != nil {
		return httpx.BadRequest("invalid " + field)
	}
	return nil
}

func validateUUIDList(values []string, field string) error {
	for _, value := range values {
		if err := validateUUID(value, field); err != nil {
			return err
		}
	}
	return nil
}

func uuidPlaceholders(n int) string {
	items := make([]string, n)
	for i := range items {
		items[i] = "toUUID(?)"
	}
	return strings.Join(items, ",")
}

func valuePlaceholders(n int) string {
	items := make([]string, n)
	for i := range items {
		items[i] = "?"
	}
	return strings.Join(items, ",")
}

func buildOrderBy(spec groupSpec) string {
	if spec.orderExpr == "" {
		return ""
	}
	return "ORDER BY " + spec.orderExpr
}

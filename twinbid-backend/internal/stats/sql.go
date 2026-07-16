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
		selectExpr: "win_cid",
		groupExpr:  "win_cid",
	},
}

var filterColumns = map[string]string{
	string(FilterByCountry):    "geo",
	string(FilterByOS):         "os",
	string(FilterByBrowser):    "browser",
	string(FilterByDeviceType): "lowerUTF8(device_type)",
}

const impressionsExpression = "toUInt64(ifNull(sum(impressions), 0))"
const clicksExpression = `toUInt64(
    ifNull(
        sum(
            multiIf(
                lowerUTF8(ifNull(format, '')) = 'pop',
                impressions,
                clicks
            )
        ),
        0
    )
)`

const conversionsExpression = "toUInt64(ifNull(sum(conversions), 0))"
const incomeExpression = "round(ifNull(sum(payout), 0), 4)"

const conversionsApprovedExpression = "toUInt64(ifNull(sum(conversions_approved), 0))"
const incomeApprovedExpression = "round(ifNull(sum(payout_approved), 0), 4)"

// Spent formula for the single endpoint /api/stats/query.
// It assumes that ads.agg_stats has the future `format` column and that the MV groups by it.
const spentExpression = `round(
    ifNull(
        sum(
            multiIf(
                lowerUTF8(ifNull(format, '')) IN ('ban', 'nat', 'pop'), spend_views_table,
                lowerUTF8(ifNull(format, '')) = 'ipp', spend_clicks_table,
                0
            )
        ),
        0
    ),
    4
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
    bucket,
    impressions_value AS impressions,
    clicks_value AS clicks,
    conversions_value AS conversions,
    spent_value AS spent,
    income_value AS income,
    conversions_approved_value AS conversions_approved,
    income_approved_value AS income_approved,
    round(
        if(
            impressions_value = 0,
            0,
            clicks_value * 100.0 / impressions_value
        ),
        2
    ) AS ctr
FROM
(
    SELECT
        %s AS bucket,
        %s AS impressions_value,
        %s AS clicks_value,
        %s AS conversions_value,
        %s AS spent_value,
        %s AS income_value,
        %s AS conversions_approved_value,
        %s AS income_approved_value
    FROM %s
    WHERE %s
    GROUP BY %s
)
%s`,
		spec.selectExpr,
		impressionsExpression,
		clicksExpression,
		conversionsExpression,
		spentExpression,
		incomeExpression,
		conversionsApprovedExpression,
		incomeApprovedExpression,
		table,
		where,
		spec.groupExpr,
		buildBucketOrderBy(spec),
	)

	totalsSQL := fmt.Sprintf(`
SELECT
    impressions_value AS impressions,
    clicks_value AS clicks,
    conversions_value AS conversions,
    spent_value AS spent,
    income_value AS income,
    conversions_approved_value AS conversions_approved,
    income_approved_value AS income_approved,
    round(
        if(
            impressions_value = 0,
            0,
            clicks_value * 100.0 / impressions_value
        ),
        2
    ) AS ctr
FROM
(
    SELECT
        %s AS impressions_value,
        %s AS clicks_value,
        %s AS conversions_value,
        %s AS spent_value,
        %s AS income_value,
        %s AS conversions_approved_value,
        %s AS income_approved_value
    FROM %s
    WHERE %s
)`,
		impressionsExpression,
		clicksExpression,
		conversionsExpression,
		spentExpression,
		incomeExpression,
		conversionsApprovedExpression,
		incomeApprovedExpression,
		table,
		where,
	)

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

	parts := []string{
		"win_user_id = ?",
	}
	args := []any{userID}

	switch {
	case from != "" && to != "":
		parts = append(
			parts,
			"event_date BETWEEN toDate(?) AND toDate(?)",
		)
		args = append(args, from, to)

	case from != "":
		parts = append(
			parts,
			"event_date >= toDate(?)",
		)
		args = append(args, from)

	case to != "":
		parts = append(
			parts,
			"event_date <= toDate(?)",
		)
		args = append(args, to)
	}

	if len(req.CampaignIDs) > 0 {
		parts = append(
			parts,
			"win_cid IN ("+valuePlaceholders(len(req.CampaignIDs))+")",
		)

		for _, id := range req.CampaignIDs {
			args = append(args, id)
		}
	}

	if len(req.CreativeIDs) > 0 {
		parts = append(
			parts,
			"win_crid IN ("+valuePlaceholders(len(req.CreativeIDs))+")",
		)

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
			if key == string(FilterByDeviceType) {
				value = strings.ToLower(value)
			}
			args = append(args, value)
		}
	}

	return strings.Join(parts, " AND "), args, nil
}

func normalizeDates(from, to string) (string, string, error) {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)

	var fromDate time.Time
	var toDate time.Time
	var err error

	if from != "" {
		fromDate, err = time.Parse("2006-01-02", from)
		if err != nil {
			return "", "", httpx.BadRequest(
				"invalid from date, expected YYYY-MM-DD",
			)
		}
	}

	if to != "" {
		toDate, err = time.Parse("2006-01-02", to)
		if err != nil {
			return "", "", httpx.BadRequest(
				"invalid to date, expected YYYY-MM-DD",
			)
		}
	}

	if from != "" && to != "" && fromDate.After(toDate) {
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

func valuePlaceholders(n int) string {
	items := make([]string, n)
	for i := range items {
		items[i] = "?"
	}
	return strings.Join(items, ",")
}

func buildBucketOrderBy(spec groupSpec) string {
	if spec.orderExpr == "" {
		return ""
	}
	return "ORDER BY bucket"
}

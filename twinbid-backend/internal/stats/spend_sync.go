package stats

import (
	"fmt"
)

// CumulativeSpendTotal is an all-time cumulative spend value calculated from
// ClickHouse statistics. Amount is kept as decimal text until PostgreSQL casts
// it to NUMERIC, avoiding another float conversion in the synchronization path.
type CumulativeSpendTotal struct {
	EntityType string
	EntityID   string
	Amount     string
}

// POPRecoveredSpendTotal is the all-time cumulative POP recovery spend for a
// concrete campaign and its owning advertiser. It is deliberately separate
// from CumulativeSpendTotal: ordinary spend already contains recovered rows,
// while this value is used only to derive the one-time Redis reconciliation
// delta introduced by POP recovery.
type POPRecoveredSpendTotal struct {
	UserID     string
	CampaignID string
	Amount     string
}

const canonicalUUIDClickHouseRegexp = `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`

func buildCumulativeSpendQuery(table string) (string, error) {
	table, err := normalizeTable(table)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`
SELECT
    if(grouping(win_cid) = 1, 'user', 'campaign') AS entity_type,
    trimBoth(if(grouping(win_cid) = 1, win_user_id, win_cid)) AS entity_id,
    toString(
        round(
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
            12
        )
    ) AS cum_done_dollars
FROM %s
WHERE notEmpty(trimBoth(win_user_id))
   OR notEmpty(trimBoth(win_cid))
GROUP BY GROUPING SETS
(
    (win_user_id),
    (win_cid)
)
HAVING notEmpty(entity_id)
   AND length(entity_id) = 36
   AND match(entity_id, '%s')
ORDER BY entity_type, entity_id`, table, canonicalUUIDClickHouseRegexp), nil
}

func buildPOPRecoveredSpendQuery(table string) (string, error) {
	table, err := normalizeTable(table)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`
SELECT
    trimBoth(win_user_id) AS user_id,
    trimBoth(win_cid) AS campaign_id,
    toString(round(ifNull(sum(pop_recovered_spend), 0), 12)) AS pop_recovered_spend
FROM %s
WHERE notEmpty(trimBoth(win_user_id))
  AND notEmpty(trimBoth(win_cid))
  AND length(trimBoth(win_user_id)) = 36
  AND match(trimBoth(win_user_id), '%s')
  AND length(trimBoth(win_cid)) = 36
  AND match(trimBoth(win_cid), '%s')
  AND pop_recovered_spend > 0
GROUP BY
    win_user_id,
    win_cid
ORDER BY
    user_id,
    campaign_id`, table, canonicalUUIDClickHouseRegexp, canonicalUUIDClickHouseRegexp), nil
}

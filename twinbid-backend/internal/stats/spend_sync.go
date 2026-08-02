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
   AND isNotNull(toUUIDOrNull(entity_id))
ORDER BY entity_type, entity_id`, table), nil
}

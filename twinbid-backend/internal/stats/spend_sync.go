package stats

import (
	"fmt"
)

// CumulativeSpendTotal is an all-time cumulative spend value calculated from
// ClickHouse statistics. Amount is kept as decimal text until PostgreSQL casts
// it to NUMERIC, avoiding another float conversion in the synchronization path.
type CumulativeSpendTotal struct {
	EntityType     string
	EntityID       string
	Amount         string
	DiagnosticOnly bool
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

// POPRecoveryCursor is the global ordered marker for the immutable
// pop_recovery_events stream. RecoveryAtMS is DateTime64(3) in Unix ms and
// SourceKey breaks ties inside one recovery batch.
type POPRecoveryCursor struct {
	RecoveryAtMS int64
	SourceKey    string
}

// POPRecoveryEvent is one newly recovered POP impression emitted by the
// ClickHouse loader. Amount is decimal text to avoid another float round-trip
// before PostgreSQL NUMERIC/Redis adjustment creation.
type POPRecoveryEvent struct {
	RecoveryAtMS int64
	SourceKey    string
	UserID       string
	CampaignID   string
	Amount       string
}

const popRecoveryEventsTable = "pop_recovery_events"

func buildPOPRecoveryEventsAfterQuery(table string) (string, error) {
	table, err := normalizeTable(table)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`
SELECT
    toUnixTimestamp64Milli(recovery_at) AS recovery_at_ms,
    recovery_source_key,
    trimBoth(user_id) AS user_id,
    trimBoth(campaign_id) AS campaign_id,
    toString(round(spend, 12)) AS recovered_spend
FROM %s
WHERE
    (
        toUnixTimestamp64Milli(recovery_at) > ?
        OR (
            toUnixTimestamp64Milli(recovery_at) = ?
            AND recovery_source_key > ?
        )
    )
  AND notEmpty(recovery_source_key)
  AND notEmpty(trimBoth(user_id))
  AND notEmpty(trimBoth(campaign_id))
  AND length(trimBoth(user_id)) = 36
  AND match(trimBoth(user_id), '%s')
  AND length(trimBoth(campaign_id)) = 36
  AND match(trimBoth(campaign_id), '%s')
  AND spend > 0
ORDER BY recovery_at, recovery_source_key
LIMIT ?`, table, canonicalUUIDClickHouseRegexp, canonicalUUIDClickHouseRegexp), nil
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
   AND length(entity_id) = 36
   AND match(entity_id, '%s')
ORDER BY entity_type, entity_id`, table, canonicalUUIDClickHouseRegexp), nil
}

func buildFilteredNonCanonicalUUIDLikeCampaignQuery(table string) (string, error) {
	table, err := normalizeTable(table)
	if err != nil {
		return "", err
	}

	// This query is diagnostic only. The authoritative spend query above still
	// accepts entities exclusively through the strict canonical UUID predicate.
	// Here toUUIDOrNull is intentionally used to surface the narrow class of raw
	// DSP win_cid values that ClickHouse would have permissively coerced to UUIDs
	// in the old implementation (the production poison-row failure mode).
	return fmt.Sprintf(`
SELECT trimBoth(win_cid) AS entity_id
FROM %s
WHERE notEmpty(trimBoth(win_cid))
  AND NOT (
      length(trimBoth(win_cid)) = 36
      AND match(trimBoth(win_cid), '%s')
  )
  AND isNotNull(toUUIDOrNull(trimBoth(win_cid)))
GROUP BY win_cid
ORDER BY entity_id
LIMIT 5`, table, canonicalUUIDClickHouseRegexp), nil
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

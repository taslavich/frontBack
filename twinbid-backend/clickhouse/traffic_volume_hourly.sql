-- Hourly traffic aggregates for:
--   POST /api/calculator
--   POST /api/recommend_bid
--
-- The materialized view is incremental: every INSERT into ads.ortb contributes
-- only its new block to this compact SummingMergeTree table. Late events are
-- attributed to their original event_hour automatically.

CREATE TABLE IF NOT EXISTS ads.traffic_volume_hourly
(
    event_hour DateTime('UTC'),

    format LowCardinality(String),
    typic LowCardinality(String),
    geo LowCardinality(String),
    lang LowCardinality(String),
    device LowCardinality(String),
    os LowCardinality(String),
    browser LowCardinality(String),

    requests UInt64,
    nonzero_win_dsp_price_sum Float64,
    nonzero_win_dsp_price_count UInt64
)
ENGINE = SummingMergeTree((
    requests,
    nonzero_win_dsp_price_sum,
    nonzero_win_dsp_price_count
))
PARTITION BY toYYYYMM(event_hour)
ORDER BY
(
    event_hour,
    format,
    typic,
    geo,
    lang,
    device,
    os,
    browser
)
TTL event_hour + INTERVAL 10 DAY DELETE
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS ads.mv_ortb_traffic_hourly
TO ads.traffic_volume_hourly
AS
SELECT
    toStartOfHour(toDateTime(event_time, 'UTC')) AS event_hour,

    multiIf(
        lowerUTF8(ifNull(format, '')) IN ('ban', 'banner'), 'BAN',
        lowerUTF8(ifNull(format, '')) IN ('nat', 'native'), 'NAT',
        lowerUTF8(ifNull(format, '')) IN ('pop', 'popunder'), 'POP',
        lowerUTF8(ifNull(format, '')) IN ('ipp', 'push'), 'IPP',
        upperUTF8(ifNull(format, ''))
    ) AS format,

    multiIf(
        lowerUTF8(ifNull(typic, '')) IN ('mainstream', 'main', 'mc'), 'MAINSTREAM',
        lowerUTF8(ifNull(typic, '')) IN ('adult', 'adl'), 'ADULT',
        upperUTF8(ifNull(typic, ''))
    ) AS typic,

    upperUTF8(ifNull(geo, '')) AS geo,
    lowerUTF8(ifNull(lang, '')) AS lang,
    lowerUTF8(ifNull(device, '')) AS device,
    ifNull(os, '') AS os,
    ifNull(browser, '') AS browser,

    count() AS requests,

    sumIf(
        toFloat64(ifNull(win_dsp_price, 0)),
        ifNull(win_dsp_price, 0) > 0
    ) AS nonzero_win_dsp_price_sum,

    countIf(
        ifNull(win_dsp_price, 0) > 0
    ) AS nonzero_win_dsp_price_count
FROM ads.ortb
GROUP BY
    event_hour,
    format,
    typic,
    geo,
    lang,
    device,
    os,
    browser;

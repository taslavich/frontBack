-- OPTIONAL ONE-TIME BACKFILL.
--
-- Run this only once if ads.ortb already contains historical data and the API
-- must work immediately. For an exact cutover, briefly pause writes to ads.ortb,
-- execute this INSERT, create/enable the MV, and resume writes. Do not execute
-- the same backfill twice: SummingMergeTree would add the rows again.

INSERT INTO ads.traffic_volume_hourly
(
    event_hour,
    format,
    typic,
    geo,
    lang,
    device,
    os,
    browser,
    requests,
    nonzero_win_dsp_price_sum,
    nonzero_win_dsp_price_count
)
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
WHERE event_time >= toStartOfDay(now('UTC')) - INTERVAL 10 DAY
  AND event_time < toStartOfHour(now('UTC'))
GROUP BY
    event_hour,
    format,
    typic,
    geo,
    lang,
    device,
    os,
    browser;

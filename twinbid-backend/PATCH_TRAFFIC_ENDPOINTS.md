# Traffic calculator and bid recommendation patch

## Added API endpoints

Both endpoints are inside the existing JWT-protected router group:

- `POST /api/calculator`
- `POST /api/recommend_bid`

They accept the frontend `TrafficSegmentRequest` fields:

- `format_type`
- `traffic_type`
- `country`, `country_mode`
- `language`, `language_mode`
- `device_type`, `device_type_mode`
- `os`, `os_mode`
- `browser`, `browser_mode`

An empty array disables the corresponding filter. `include` produces `IN` and
`exclude` produces `NOT IN`.

## Responses

`POST /api/calculator`:

```json
{
  "success": true,
  "errorMsg": "",
  "data": {
    "potential_impressions": 128400
  }
}
```

`POST /api/recommend_bid`:

```json
{
  "success": true,
  "errorMsg": "",
  "data": {
    "average_bid": 1.24
  }
}
```

The frontend API layer unwraps `data`, so page components receive exactly
`{ potential_impressions }` and `{ average_bid }`.

## Reporting period

Both endpoints query the previous complete UTC day:

```text
[toStartOfDay(now('UTC')) - 1 day, toStartOfDay(now('UTC')))
```

This matches the frontend's `previousCompleteUtcDate()` comparison request.

## ClickHouse files

- `clickhouse/traffic_volume_hourly.sql` creates the target table and MV.
- `clickhouse/traffic_volume_hourly_backfill.sql` is an optional one-time
  historical backfill. Do not execute it twice.

The bid recommendation is calculated as a weighted average:

```text
sum(nonzero winning prices) / count(nonzero winning prices)
```

It is not an average of hourly averages.

## Configuration

The patch adds:

```env
CLICKHOUSE_TRAFFIC_TABLE=traffic_volume_hourly
```

The configured ClickHouse database remains `ads`, so the default resolved table
is `ads.traffic_volume_hourly`.

# TwinBid backend

Go backend for the TypeScript API contract.

## Run locally

```bash
cp .env.example .env
docker compose up -d postgres clickhouse minio
source .env
go run ./cmd/api
```

The app runs migrations in Postgres on startup.

## Creative images and private MinIO

MinIO stays private. The backend connects to it through the local S3 endpoint and
serves permanent public image URLs itself:

```env
S3_ENDPOINT=http://127.0.0.1:9000
S3_BUCKET=creatives
S3_USE_PATH_STYLE=true
PUBLIC_API_BASE_URL=https://twinbid.io
```

New images are uploaded with:

```text
POST /api/campaigns/{campaignID}/creative-images
```

The backend stores objects under keys such as:

```text
images/{campaign_id}/{image_uuid}.png
```

The returned permanent URL has no signature or TTL:

```text
https://twinbid.io/api/media/{image_uuid}
```

`GET` and `HEAD /api/media/{image_uuid}` read the private MinIO object through
the backend. Creative create/update requests are JSON and reference the uploaded
object with `image_id`.

## ClickHouse stats table expected by default

The stats service expects `CLICKHOUSE_STATS_TABLE` with at least these columns:

```sql
CREATE TABLE IF NOT EXISTS campaign_stats (
    date Date,
    hour DateTime,
    user_id String,
    campaign_id String,
    creative_id String,
    country String,
    format_type String,
    os String,
    browser String,
    device_type String,
    language String,
    site_id String,
    impressions UInt64,
    clicks UInt64,
    spent Float64
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(date)
ORDER BY (user_id, date, campaign_id, creative_id);
```

## Traffic calculator and bid recommendation

The backend also expects `CLICKHOUSE_TRAFFIC_TABLE` (default: `traffic_volume_hourly`)
for `POST /api/calculator` and `POST /api/recommend_bid`. Create the table and
materialized view with:

```bash
clickhouse-client --multiquery < clickhouse/traffic_volume_hourly.sql
```

See `PATCH_TRAFFIC_ENDPOINTS.md` for the request/response contract and optional
historical backfill instructions.


## Internal campaign moderation

The Telegram moderation bot applies a decision through the secret-protected endpoint:

```http
POST /internal/campaigns/{campaign_id}/moderation
X-Bot-Secret: <BOT_INTERNAL_SECRET>
Content-Type: application/json

{"decision":"approve"}
```

Supported decisions are `approve` (`moderation` -> `waiting`) and `reject`
(`moderation` -> `draft`). The campaign row is locked in the same transaction as
the status change. If the current status is already `draft`, the endpoint returns
`409 Conflict` with `Модерация уже отменена пользователем`.

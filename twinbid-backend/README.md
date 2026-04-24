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

## S3

Creatives are stored in S3 using keys:

```text
creatives/{campaign_id}/{creative_id}/{filename}
```

For read operations backend returns `name` and `presigned_s3_url`.

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

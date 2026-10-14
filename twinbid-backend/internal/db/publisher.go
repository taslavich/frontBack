package db

import (
	"context"
	"database/sql"
)

// EnsurePublisherSchema is additive. Existing advertiser/recovery tables are untouched.
func EnsurePublisherSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(871922130)`); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
 CREATE TABLE IF NOT EXISTS ad_codes (
 ad_code_id UUID PRIMARY KEY, user_id UUID NOT NULL REFERENCES users(id), short_code TEXT UNIQUE NOT NULL,
 name TEXT NOT NULL, format TEXT NOT NULL DEFAULT 'popunder' CHECK(format='popunder'),
 traffic_type TEXT NOT NULL CONSTRAINT ad_codes_traffic_type_check CHECK(traffic_type IN ('adult','mainstream')),
 device TEXT NOT NULL DEFAULT 'all' CHECK(device IN ('all','desktop','mobile')),
 is_active BOOLEAN NOT NULL DEFAULT true, archived_at TIMESTAMPTZ,
 trigger_click INT NOT NULL DEFAULT 1 CHECK(trigger_click IN(1,2)),
 frequency_minutes DOUBLE PRECISION NOT NULL DEFAULT 10 CHECK(frequency_minutes>=0.1 AND frequency_minutes<=10080),
 max_per_session INT CHECK(max_per_session>=1 AND max_per_session<=100000),
 version BIGINT NOT NULL DEFAULT 1, synced_version BIGINT NOT NULL DEFAULT 0, synced_epoch TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
 ALTER TABLE ad_codes ALTER COLUMN traffic_type DROP DEFAULT;
 ALTER TABLE ad_codes DROP CONSTRAINT IF EXISTS ad_codes_traffic_type_check;
 ALTER TABLE ad_codes ADD CONSTRAINT ad_codes_traffic_type_check CHECK(traffic_type IN ('adult','mainstream')) NOT VALID;
 DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM ad_codes WHERE traffic_type NOT IN ('adult','mainstream')) THEN ALTER TABLE ad_codes VALIDATE CONSTRAINT ad_codes_traffic_type_check; END IF; END $$;
 CREATE INDEX IF NOT EXISTS ad_codes_owner ON ad_codes(user_id,created_at);
 CREATE INDEX IF NOT EXISTS ad_codes_delta ON ad_codes(updated_at) WHERE version>synced_version;
 CREATE TABLE IF NOT EXISTS publisher_domain_blocks(hostname TEXT PRIMARY KEY, blocked BOOLEAN NOT NULL, reason TEXT NOT NULL DEFAULT '', version BIGINT NOT NULL DEFAULT 1, synced_version BIGINT NOT NULL DEFAULT 0, synced_epoch TEXT NOT NULL DEFAULT '', updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
 CREATE TABLE IF NOT EXISTS publisher_control(id INT PRIMARY KEY CHECK(id=1), epoch TEXT NOT NULL DEFAULT '', state TEXT NOT NULL DEFAULT 'blocked');
 INSERT INTO publisher_control(id) VALUES(1) ON CONFLICT DO NOTHING;
 CREATE TABLE IF NOT EXISTS publisher_accounts(user_id UUID PRIMARY KEY REFERENCES users(id), balance NUMERIC(24,8) NOT NULL DEFAULT 0, reserved NUMERIC(24,8) NOT NULL DEFAULT 0 CHECK(reserved>=0), preferences JSONB NOT NULL DEFAULT '{"codeAlerts":true,"payoutMethod":"usdt_trc20","wallet":""}');
 CREATE TABLE IF NOT EXISTS publisher_revenue(source_id TEXT PRIMARY KEY, user_id UUID NOT NULL REFERENCES users(id), ad_code_id UUID REFERENCES ad_codes(ad_code_id), site_id TEXT NOT NULL, zone_id TEXT NOT NULL, amount NUMERIC(24,8) NOT NULL, geo TEXT NOT NULL DEFAULT '', device TEXT NOT NULL DEFAULT '', os TEXT NOT NULL DEFAULT '', browser TEXT NOT NULL DEFAULT '', event_time TIMESTAMPTZ NOT NULL, revision BIGINT NOT NULL CHECK(revision>=1));
 ALTER TABLE publisher_revenue ALTER COLUMN ad_code_id DROP NOT NULL;
 CREATE TABLE IF NOT EXISTS publisher_revenue_source_state(id INT PRIMARY KEY CHECK(id=1), last_success TIMESTAMPTZ, last_error TEXT NOT NULL DEFAULT '');
 INSERT INTO publisher_revenue_source_state(id) VALUES(1) ON CONFLICT DO NOTHING;
 CREATE TABLE IF NOT EXISTS publisher_revenue_checkpoints(user_id UUID PRIMARY KEY REFERENCES users(id), cumulative NUMERIC(24,8) NOT NULL DEFAULT 0, revision BIGINT NOT NULL DEFAULT 0, updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
 CREATE TABLE IF NOT EXISTS publisher_revenue_deltas(user_id UUID NOT NULL REFERENCES users(id), revision BIGINT NOT NULL, cumulative NUMERIC(24,8) NOT NULL, delta NUMERIC(24,8) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY(user_id,revision));
 CREATE INDEX IF NOT EXISTS publisher_revenue_owner_time ON publisher_revenue(user_id,event_time);
 CREATE TABLE IF NOT EXISTS publisher_payouts(id UUID PRIMARY KEY,user_id UUID NOT NULL REFERENCES users(id), request_id UUID NOT NULL, amount NUMERIC(24,8) NOT NULL CHECK(amount>=50), method TEXT NOT NULL, wallet TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','paid','canceled')), transfer_reference TEXT NOT NULL DEFAULT '', notified_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), UNIQUE(user_id,request_id));
 CREATE TABLE IF NOT EXISTS publisher_attention(ad_code_id UUID PRIMARY KEY REFERENCES ad_codes(ad_code_id), missing JSONB NOT NULL, notified BOOLEAN NOT NULL DEFAULT false, updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
 CREATE OR REPLACE FUNCTION publisher_code_owner_immutable() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.ad_code_id<>OLD.ad_code_id OR NEW.user_id<>OLD.user_id OR NEW.short_code<>OLD.short_code THEN RAISE EXCEPTION 'Ad Code identity is immutable'; END IF; RETURN NEW; END $$;
 DROP TRIGGER IF EXISTS publisher_code_identity ON ad_codes;
 CREATE TRIGGER publisher_code_identity BEFORE UPDATE ON ad_codes FOR EACH ROW EXECUTE FUNCTION publisher_code_owner_immutable();
 `)
	if err != nil {
		return err
	}
	return tx.Commit()
}

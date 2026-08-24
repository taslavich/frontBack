package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"
)

func NewPostgres(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(30)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB, publicAPIBaseURL string) error {
	queries := []string{
		`CREATE EXTENSION IF NOT EXISTS pgcrypto;`,
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			login TEXT UNIQUE NOT NULL,
			mail TEXT NOT NULL,
			name TEXT NOT NULL,
			telegram TEXT,
			manager_telegram TEXT NOT NULL,
			goal_total_dollars DECIMAL NOT NULL DEFAULT 0,
			cum_done_dollars DECIMAL NOT NULL DEFAULT 0,
			timezone TEXT NOT NULL DEFAULT 'utc_3',
			email_notifications BOOLEAN NOT NULL DEFAULT true,
			campaign_status_notifications BOOLEAN NOT NULL DEFAULT true,
			low_balance_notifications BOOLEAN NOT NULL DEFAULT true,
			campaign_balance_notifications BOOLEAN NOT NULL DEFAULT true,
			balance_treshold DECIMAL NOT NULL DEFAULT 100,
			low_balance_notified BOOLEAN NOT NULL DEFAULT true,
			antiperekrut_blocked BOOLEAN NOT NULL DEFAULT false,
			verified BOOLEAN NOT NULL DEFAULT false,
			password TEXT NOT NULL,
			utm_source TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'balance'
			) AND NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'goal_total_dollars'
			) THEN
				ALTER TABLE users RENAME COLUMN balance TO goal_total_dollars;
			END IF;
		END $$;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS goal_total_dollars DECIMAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS cum_done_dollars DECIMAL NOT NULL DEFAULT 0;`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS antiperekrut_blocked BOOLEAN NOT NULL DEFAULT false;`,
		`CREATE TABLE IF NOT EXISTS refresh_tokens (
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token TEXT PRIMARY KEY,
			expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS registrate_tokens (
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token TEXT PRIMARY KEY,
			expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS campaigns (
			campaign_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			campaign_name TEXT NOT NULL,
			format_type TEXT NOT NULL,
			brand_name TEXT,
			h INT,
			w INT,
			status TEXT NOT NULL DEFAULT 'draft',
			traffic_type TEXT NOT NULL,
			vertical JSONB NOT NULL DEFAULT '{}'::jsonb,
			pricing_model TEXT NOT NULL,
			base_price DECIMAL NOT NULL DEFAULT 0,
			evenness_by_slot_mode BOOLEAN NOT NULL DEFAULT false,
			block_vpn BOOLEAN NOT NULL DEFAULT false,
			goal_total_dollars DECIMAL NOT NULL DEFAULT 0,
			cum_done_dollars DECIMAL NOT NULL DEFAULT 0,
			no_budget_notified BOOLEAN NOT NULL DEFAULT false,
			start_ts TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			end_ts TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			active_intervals JSONB NOT NULL DEFAULT '[]'::jsonb,
			country JSONB NOT NULL DEFAULT '{}'::jsonb,
			language JSONB NOT NULL DEFAULT '{}'::jsonb,
			device_type JSONB NOT NULL DEFAULT '{}'::jsonb,
			os JSONB NOT NULL DEFAULT '{}'::jsonb,
			browser JSONB NOT NULL DEFAULT '{}'::jsonb,
			site_id JSONB NOT NULL DEFAULT '{}'::jsonb,
			ip JSONB NOT NULL DEFAULT '{}'::jsonb,
			quality_type TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS block_vpn BOOLEAN NOT NULL DEFAULT false;`,
		`ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS traffic_reset_version BIGINT NOT NULL DEFAULT 0;`,
		`CREATE TABLE IF NOT EXISTS antiperekrut_control_state (
			id SMALLINT PRIMARY KEY,
			global_reset_generation BIGINT NOT NULL DEFAULT 0,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			CONSTRAINT antiperekrut_control_state_singleton CHECK (id = 1)
		);`,
		`INSERT INTO antiperekrut_control_state (id, global_reset_generation)
		 VALUES (1, 0) ON CONFLICT (id) DO NOTHING;`,
		`CREATE TABLE IF NOT EXISTS antiperekrut_restart_events (
			event_id UUID PRIMARY KEY,
			source_service TEXT NOT NULL,
			source_instance TEXT NOT NULL,
			reason TEXT NOT NULL DEFAULT 'startup',
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_antiperekrut_restart_events_created_at
		 ON antiperekrut_restart_events(created_at DESC);`,
		`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 
				FROM information_schema.table_constraints 
				WHERE constraint_name = 'check_campaign_dates' 
				AND table_name = 'campaigns'
			) THEN
				ALTER TABLE campaigns 
				ADD CONSTRAINT check_campaign_dates 
				CHECK (start_ts <= end_ts);
			END IF;
		END $$;`,
		`CREATE TABLE IF NOT EXISTS creatives (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			campaign_id UUID NOT NULL REFERENCES campaigns(campaign_id) ON DELETE CASCADE,
			creative_name TEXT NOT NULL,
			adm TEXT NOT NULL,
			banner_type TEXT,
			trackers_macros JSONB NOT NULL DEFAULT '{}'::jsonb,
			w INT,
			h INT,
			name TEXT,
			s3_file_path TEXT,
			file_format TEXT,
			title TEXT,
			description TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,

		`DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema='public' AND table_name='creatives' AND column_name='link'
			) AND NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema='public' AND table_name='creatives' AND column_name='adm'
			) THEN
				ALTER TABLE creatives RENAME COLUMN link TO adm;
			END IF;
		END $$;`,
		`ALTER TABLE creatives ADD COLUMN IF NOT EXISTS adm TEXT;`,
		`UPDATE creatives SET adm='' WHERE adm IS NULL;`,
		`ALTER TABLE creatives ALTER COLUMN adm SET NOT NULL;`,
		`ALTER TABLE creatives ADD COLUMN IF NOT EXISTS banner_type TEXT;`,
		`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.table_constraints
				WHERE table_schema='public' AND table_name='creatives'
				  AND constraint_name='check_creatives_banner_type'
			) THEN
				ALTER TABLE creatives ADD CONSTRAINT check_creatives_banner_type
				CHECK (banner_type IS NULL OR banner_type IN ('img', 'iframe'));
			END IF;
		END $$;`,
		`UPDATE creatives cr
		 SET banner_type=CASE
			WHEN ltrim(cr.adm) LIKE '<%' THEN 'iframe'
			ELSE 'img'
		 END
		 FROM campaigns c
		 WHERE c.campaign_id=cr.campaign_id AND c.format_type='banner' AND cr.banner_type IS NULL;`,
		`UPDATE creatives cr
		 SET banner_type=NULL
		 FROM campaigns c
		 WHERE c.campaign_id=cr.campaign_id AND c.format_type<>'banner' AND cr.banner_type IS NOT NULL;`,
		`ALTER TABLE creatives ADD COLUMN IF NOT EXISTS trackers_macros JSONB NOT NULL DEFAULT '{}'::jsonb;`,
		`WITH normalized AS (
			SELECT c.id,
				COALESCE((
					SELECT jsonb_object_agg(
						item.key,
						CASE jsonb_typeof(item.value)
							WHEN 'boolean' THEN to_jsonb(item.key)
							WHEN 'number' THEN to_jsonb(item.key)
							WHEN 'string' THEN to_jsonb(btrim(item.value #>> '{}'))
						END
					)
					FROM jsonb_each(COALESCE(c.trackers_macros, '{}'::jsonb)) AS item
					WHERE (jsonb_typeof(item.value) = 'boolean' AND item.value = 'true'::jsonb)
					   OR (jsonb_typeof(item.value) = 'number' AND (item.value #>> '{}')::numeric <> 0)
					   OR (jsonb_typeof(item.value) = 'string' AND btrim(item.value #>> '{}') <> '')
				), '{}'::jsonb) AS macros
			FROM creatives c
		), migrated AS (
			SELECT normalized.id,
				CASE
					WHEN c.format_type = 'banner' AND cr.banner_type = 'iframe'
						THEN normalized.macros - 'click_id'
					ELSE normalized.macros || jsonb_build_object(
						'click_id',
						COALESCE(NULLIF(btrim(normalized.macros ->> 'click_id'), ''), 'click_id')
					)
				END AS macros
			FROM normalized
			JOIN creatives cr ON cr.id = normalized.id
			JOIN campaigns c ON c.campaign_id = cr.campaign_id
		)
		UPDATE creatives c
		SET trackers_macros = migrated.macros
		FROM migrated
		WHERE c.id = migrated.id
		  AND c.trackers_macros IS DISTINCT FROM migrated.macros;`,
		`CREATE TABLE IF NOT EXISTS creative_images (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			campaign_id UUID REFERENCES campaigns(campaign_id) ON DELETE SET NULL,
			creative_id UUID UNIQUE REFERENCES creatives(id) ON DELETE SET NULL,
			s3_key TEXT NOT NULL UNIQUE,
			web_url TEXT NOT NULL UNIQUE,
			original_name TEXT NOT NULL,
			mime_type TEXT NOT NULL,
			file_format TEXT NOT NULL,
			size_bytes BIGINT NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);`,
		`UPDATE creatives cr
		 SET banner_type='iframe', updated_at=NOW()
		 FROM campaigns c
		 WHERE c.campaign_id=cr.campaign_id
		   AND c.format_type='banner'
		   AND cr.banner_type='img'
		   AND cr.s3_file_path IS NOT NULL
		   AND cr.s3_file_path<>''
		   AND ltrim(cr.adm) LIKE '<%'
		   AND cr.adm NOT LIKE '%/api/media/%';`,
		`UPDATE creative_images ci
		 SET creative_id=NULL, updated_at=NOW()
		 FROM creatives cr
		 JOIN campaigns c ON c.campaign_id=cr.campaign_id
		 WHERE ci.creative_id=cr.id
		   AND c.format_type='banner'
		   AND cr.banner_type='iframe';`,
		`UPDATE creative_images ci
		 SET creative_id=cr.id, campaign_id=cr.campaign_id, updated_at=NOW()
		 FROM creatives cr
		 JOIN campaigns c ON c.campaign_id=cr.campaign_id
		 WHERE ci.s3_key=cr.s3_file_path
		   AND ci.creative_id IS NULL
		   AND cr.s3_file_path IS NOT NULL
		   AND cr.s3_file_path<>''
		   AND (c.format_type<>'banner' OR cr.banner_type='img')
		   AND NOT EXISTS (
			SELECT 1 FROM creative_images occupied WHERE occupied.creative_id=cr.id
		   );`,
		`CREATE TABLE IF NOT EXISTS promocodes (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			promocode_text TEXT UNIQUE NOT NULL,
			bonus_percent DECIMAL NOT NULL DEFAULT 0,
			usage_count INT NOT NULL DEFAULT 0,
			usage_limit INT,
			valid_from TIMESTAMP WITH TIME ZONE,
			valid_to TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS user_transactions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			transaction_time TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			transaction_id TEXT NOT NULL UNIQUE,
			payment_method TEXT NOT NULL,
			bonus_amount DECIMAL NOT NULL DEFAULT 0,
			promocode_id UUID REFERENCES promocodes(id) ON DELETE SET NULL,
			transaction_hash TEXT,
			deposit_amount DECIMAL NOT NULL DEFAULT 0,
			total_balance_increase DECIMAL NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'pending',
			currency TEXT NOT NULL DEFAULT 'USD',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`ALTER TABLE user_transactions ADD COLUMN IF NOT EXISTS payment_channel TEXT NOT NULL DEFAULT 'static_wallet';`,
		`ALTER TABLE user_transactions ADD COLUMN IF NOT EXISTS promocode_usage_applied BOOLEAN NOT NULL DEFAULT false;`,
		`ALTER TABLE user_transactions ADD COLUMN IF NOT EXISTS payment_url TEXT;`,
		`ALTER TABLE user_transactions ADD COLUMN IF NOT EXISTS provider_status TEXT;`,
		`ALTER TABLE user_transactions ADD COLUMN IF NOT EXISTS provider_payment_id TEXT;`,
		`ALTER TABLE user_transactions ADD COLUMN IF NOT EXISTS provider_transaction_id TEXT;`,
		`ALTER TABLE user_transactions ADD COLUMN IF NOT EXISTS amount_paid DECIMAL;`,
		`ALTER TABLE user_transactions ADD COLUMN IF NOT EXISTS amount_credited DECIMAL;`,
		`ALTER TABLE user_transactions ADD COLUMN IF NOT EXISTS fee_service DECIMAL;`,
		`ALTER TABLE user_transactions ADD COLUMN IF NOT EXISTS fee_network DECIMAL;`,
		`ALTER TABLE user_transactions ADD COLUMN IF NOT EXISTS credited_at TIMESTAMP WITH TIME ZONE;`,
		`ALTER TABLE user_transactions ADD COLUMN IF NOT EXISTS provider_payload JSONB;`,
		`ALTER TABLE user_transactions ADD COLUMN IF NOT EXISTS provider_check_attempts INTEGER NOT NULL DEFAULT 0;`,
		`ALTER TABLE user_transactions ADD COLUMN IF NOT EXISTS provider_next_check_at TIMESTAMP WITH TIME ZONE;`,
		`ALTER TABLE user_transactions ADD COLUMN IF NOT EXISTS provider_last_error TEXT;`,
		`ALTER TABLE user_transactions ADD COLUMN IF NOT EXISTS invoice_expires_at TIMESTAMP WITH TIME ZONE;`,
		`UPDATE user_transactions
		 SET credited_at=COALESCE(updated_at, transaction_time, NOW())
		 WHERE status='approved' AND credited_at IS NULL;`,
		`UPDATE user_transactions SET payment_channel='static_wallet' WHERE payment_channel IS NULL OR payment_channel='';`,
		`UPDATE user_transactions
		 SET invoice_expires_at=created_at + INTERVAL '60 minutes'
		 WHERE payment_channel IN ('passimpay_invoice','cryptomus_invoice')
		   AND invoice_expires_at IS NULL;`,
		`UPDATE user_transactions
		 SET promocode_usage_applied=true
		 WHERE promocode_id IS NOT NULL
		   AND status='approved'
		   AND promocode_usage_applied=false;`,
		`CREATE TABLE IF NOT EXISTS app_schema_migrations (
			migration_key TEXT PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);`,
		`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM app_schema_migrations
				WHERE migration_key='normalize_promocode_reservations_v1'
			) THEN
				-- The previous migration marked pending/rejected rows as applied even
				-- though the old service only incremented usage on final approval.
				UPDATE user_transactions
				SET promocode_usage_applied=false
				WHERE promocode_id IS NOT NULL AND status<>'approved';
				INSERT INTO app_schema_migrations (migration_key)
				VALUES ('normalize_promocode_reservations_v1');
			END IF;
		END $$;`,
		`DO $$
		DECLARE
			constraint_def TEXT;
		BEGIN
			SELECT pg_get_constraintdef(c.oid)
			INTO constraint_def
			FROM pg_constraint c
			JOIN pg_class t ON t.oid=c.conrelid
			JOIN pg_namespace n ON n.oid=t.relnamespace
			WHERE n.nspname='public'
			  AND t.relname='user_transactions'
			  AND c.conname='check_user_transactions_payment_channel';

			IF constraint_def IS NULL OR POSITION('cryptomus_invoice' IN constraint_def)=0 THEN
				ALTER TABLE user_transactions DROP CONSTRAINT IF EXISTS check_user_transactions_payment_channel;
				ALTER TABLE user_transactions ADD CONSTRAINT check_user_transactions_payment_channel
				CHECK (payment_channel IN ('static_wallet','passimpay_invoice','cryptomus_invoice'));
			END IF;
		END $$;`,
		`CREATE TABLE IF NOT EXISTS payment_webhook_events (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			provider TEXT NOT NULL,
			order_id TEXT NOT NULL,
			transaction_hash TEXT,
			signature TEXT NOT NULL,
			provider_status TEXT,
			payload JSONB NOT NULL,
			processing_error TEXT,
			processed_at TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS notifications (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			transaction_id UUID REFERENCES user_transactions(id) ON DELETE SET NULL,
			campaign_id UUID REFERENCES campaigns(campaign_id) ON DELETE SET NULL,
			deposit_amount DECIMAL,
			status TEXT NOT NULL DEFAULT 'active',
			text TEXT NOT NULL,
			type TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_campaigns_campaign_id ON campaigns(campaign_id);`,
		`CREATE INDEX IF NOT EXISTS idx_campaigns_user_id ON campaigns(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_campaigns_user_created_at_desc ON campaigns(user_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_campaigns_user_status ON campaigns(user_id, status);`,
		`CREATE INDEX IF NOT EXISTS idx_registrate_tokens_expires_at ON registrate_tokens(expires_at);`,
		`CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_expires_at ON refresh_tokens(user_id, expires_at);`,
		`CREATE INDEX IF NOT EXISTS idx_creatives_campaign_id ON creatives(campaign_id);`,
		`CREATE INDEX IF NOT EXISTS idx_creatives_campaign_created_at_desc ON creatives(campaign_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_creative_images_user_id ON creative_images(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_creative_images_campaign_id ON creative_images(campaign_id);`,
		`CREATE INDEX IF NOT EXISTS idx_creative_images_creative_id ON creative_images(creative_id);`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON user_transactions(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_user_created_at_desc ON user_transactions(user_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_user_promocode_status ON user_transactions(user_id, promocode_id, status);`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_channel_status ON user_transactions(payment_channel, status, updated_at);`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_passimpay_reconcile ON user_transactions(provider_next_check_at, updated_at) WHERE payment_channel='passimpay_invoice' AND credited_at IS NULL;`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_cryptomus_reconcile ON user_transactions(provider_next_check_at, updated_at) WHERE payment_channel='cryptomus_invoice' AND credited_at IS NULL;`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_invoice_expiry ON user_transactions(invoice_expires_at) WHERE payment_channel IN ('passimpay_invoice','cryptomus_invoice') AND credited_at IS NULL AND status IN ('draft','pending');`,
		`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT transaction_hash
				FROM user_transactions
				WHERE transaction_hash IS NOT NULL AND transaction_hash <> ''
				GROUP BY transaction_hash HAVING COUNT(*) > 1
			) THEN
				CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_hash_unique
				ON user_transactions(transaction_hash)
				WHERE transaction_hash IS NOT NULL AND transaction_hash <> '';
			ELSE
				CREATE INDEX IF NOT EXISTS idx_transactions_hash
				ON user_transactions(transaction_hash);
			END IF;
		END $$;`,
		`CREATE INDEX IF NOT EXISTS idx_payment_webhook_events_order_created ON payment_webhook_events(order_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_campaign_id ON notifications(campaign_id);`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_status ON notifications(user_id, status);`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_created_at_desc ON notifications(user_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_campaign_type_status ON notifications(user_id, campaign_id, type, status);`,
		`CREATE INDEX IF NOT EXISTS idx_users_mail ON users(mail);`,
		`CREATE INDEX IF NOT EXISTS idx_promocodes_text_upper ON promocodes(UPPER(promocode_text));`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS low_balance_notified BOOLEAN NOT NULL DEFAULT false;`,
		`ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS no_budget_notified BOOLEAN NOT NULL DEFAULT false;`,
		`INSERT INTO promocodes (promocode_text, bonus_percent, usage_limit) VALUES
			('TWINBID25', 25, NULL),
			('WELCOME10', 10, NULL)
		ON CONFLICT (promocode_text) DO NOTHING;`,
		`ALTER TABLE users ALTER COLUMN low_balance_notified SET DEFAULT true;`,
		`ALTER DATABASE twinbid SET timezone TO 'UTC';`,
	}
	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q); err != nil {
			log.Printf("migration error: %v", err)
			return err
		}
	}

	baseURL := strings.TrimRight(strings.TrimSpace(publicAPIBaseURL), "/")
	if baseURL == "" {
		return fmt.Errorf("PUBLIC_API_BASE_URL is required for creative image migration")
	}
	_, err := db.ExecContext(ctx, `
		WITH legacy AS (
			SELECT gen_random_uuid() AS image_id,
				c.user_id,
				cr.campaign_id,
				cr.id AS creative_id,
				cr.s3_file_path AS s3_key,
				COALESCE(NULLIF(cr.name, ''), regexp_replace(cr.s3_file_path, '^.*/', ''), 'legacy-image') AS original_name,
				COALESCE(NULLIF(lower(cr.file_format), ''), 'bin') AS file_format
			FROM creatives cr
			JOIN campaigns c ON c.campaign_id=cr.campaign_id
			LEFT JOIN creative_images ci ON ci.creative_id=cr.id OR ci.s3_key=cr.s3_file_path
			WHERE cr.s3_file_path IS NOT NULL
			  AND cr.s3_file_path<>''
			  AND ci.id IS NULL
			  AND (c.format_type<>'banner' OR cr.banner_type='img')
		)
		INSERT INTO creative_images (
			id, user_id, campaign_id, creative_id, s3_key, web_url, original_name,
			mime_type, file_format, size_bytes
		)
		SELECT image_id, user_id, campaign_id, creative_id, s3_key,
			$1 || '/api/media/' || image_id::text,
			original_name,
			CASE file_format
				WHEN 'jpg' THEN 'image/jpg'
				WHEN 'jpeg' THEN 'image/jpg'
				WHEN 'png' THEN 'image/png'
				WHEN 'gif' THEN 'image/gif'
				WHEN 'mp4' THEN 'video/mp4'
				ELSE 'application/octet-stream'
			END,
			file_format,
			0
		FROM legacy
		ON CONFLICT DO NOTHING
	`, baseURL)
	if err != nil {
		log.Printf("creative image backfill error: %v", err)
		return err
	}
	if err := migrateLegacyBannerADM(ctx, db); err != nil {
		log.Printf("legacy banner ADM migration error: %v", err)
		return err
	}
	return nil
}

// InitDBAndMigrate создаёт БД при необходимости и запускает миграцию
func InitDBAndMigrate(ctx context.Context, dsn, publicAPIBaseURL string) (*sql.DB, error) {
	// Подключаемся к системной БД postgres
	sysDSN := removeDatabaseFromDSN(dsn)

	sysDB, err := sql.Open("postgres", sysDSN)
	if err != nil {
		return nil, err
	}
	defer sysDB.Close()

	if err := sysDB.PingContext(ctx); err != nil {
		return nil, err
	}

	// Извлекаем имя БД
	dbName := extractDatabaseName(dsn)

	// Проверяем существует ли БД
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`
	err = sysDB.QueryRowContext(ctx, query, dbName).Scan(&exists)
	if err != nil {
		return nil, err
	}

	// Создаём БД если не существует
	if !exists {
		_, err = sysDB.ExecContext(ctx, "CREATE DATABASE "+dbName)
		if err != nil {
			return nil, err
		}
		log.Printf("✅ Database '%s' created successfully", dbName)
	}

	// Подключаемся к нашей БД и запускаем миграцию
	db, err := NewPostgres(ctx, dsn)
	if err != nil {
		return nil, err
	}

	if err := Migrate(ctx, db, publicAPIBaseURL); err != nil {
		db.Close()
		return nil, err
	}

	log.Println("✅ Migration completed successfully")
	return db, nil
}

// Вспомогательная функция: извлекает имя БД из DSN
func extractDatabaseName(dsn string) string {
	if u, err := url.Parse(dsn); err == nil {
		if dbName := strings.TrimPrefix(u.Path, "/"); dbName != "" {
			return dbName
		}
	}
	// Ищем /имя_бд? или /имя_бд
	for i := len(dsn) - 1; i >= 0; i-- {
		if dsn[i] == '/' {
			start := i + 1
			end := start
			for end < len(dsn) && dsn[end] != '?' && dsn[end] != '&' {
				end++
			}
			return dsn[start:end]
		}
	}
	return "twinbid"
}

// Вспомогательная функция: убирает имя БД из DSN
func removeDatabaseFromDSN(dsn string) string {
	if u, err := url.Parse(dsn); err == nil {
		u.Path = "/postgres"
		return u.String()
	}
	// Заменяем /имя_бд на /postgres
	for i := len(dsn) - 1; i >= 0; i-- {
		if dsn[i] == '/' {
			if qIdx := strings.Index(dsn[i:], "?"); qIdx >= 0 {
				return dsn[:i+1] + "postgres" + dsn[i+qIdx:]
			}
			return dsn[:i+1] + "postgres"
		}
	}
	return dsn
}

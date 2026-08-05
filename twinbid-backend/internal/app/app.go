package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"twinbid-backend/internal/auth"
	"twinbid-backend/internal/campaigns"
	"twinbid-backend/internal/config"
	"twinbid-backend/internal/creatives"
	"twinbid-backend/internal/db"
	"twinbid-backend/internal/notifications"
	"twinbid-backend/internal/passimpay"
	"twinbid-backend/internal/profile"
	"twinbid-backend/internal/promocodes"
	"twinbid-backend/internal/spendsync"
	"twinbid-backend/internal/stats"
	"twinbid-backend/internal/storage"
	"twinbid-backend/internal/topups"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type App struct {
	Cfg      config.Config
	Postgres *sql.DB
	Stats    *stats.Service
	Router   http.Handler
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	pg, err := db.InitDBAndMigrate(ctx, cfg.Postgres.DSN, cfg.PublicAPIBaseURL)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	log.Println("✅ Connected to Postgres")

	s3, err := storage.NewS3(ctx, cfg.S3)
	if err != nil {
		return nil, fmt.Errorf("s3: %w", err)
	}
	statsSvc, err := stats.NewService(ctx, cfg.ClickHouse)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: %w", err)
	}
	log.Println("✅ Connected to ClickHouse")

	authRepo := auth.NewRepository(pg)
	authSvc := auth.NewService(authRepo, cfg.JWT, cfg.SMTP)
	authHandler := auth.NewHandler(authSvc)
	go func() {
		t := time.NewTicker(cfg.JWT.RegistrationCleanupIn)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := authSvc.CleanupExpiredRegistrationTokens(ctx); err != nil {
					log.Printf("registration token cleanup error: %v", err)
				}
			}
		}
	}()

	profileRepo := profile.NewRepository(pg)
	profileSvc := profile.NewService(profileRepo)
	profileHandler := profile.NewHandler(profileSvc)

	notificationRepo := notifications.NewRepository(pg)
	notificationSvc := notifications.NewService(notificationRepo)
	notificationHandler := notifications.NewHandler(notificationSvc)

	creativeRepo := creatives.NewRepository(pg)
	campaignSvc := campaigns.NewService(
		campaigns.NewRepository(pg),
		creativeRepo,
		profileRepo,
		notificationSvc,
		cfg.SMTP,
		cfg.Bot,
		s3,
	)
	campaignHandler := campaigns.NewHandler(campaignSvc, cfg.Bot.InternalSecret)

	creativeSvc := creatives.NewService(creativeRepo, campaignSvc, s3, cfg.PublicAPIBaseURL)
	creativeHandler := creatives.NewHandler(creativeSvc)

	promoRepo := promocodes.NewRepository(pg)
	promoSvc := promocodes.NewService(promoRepo)
	promoHandler := promocodes.NewHandler(promoSvc)
	passimPayClient := passimpay.NewClient(passimpay.Config{
		BaseURL:           cfg.PassimPay.BaseURL,
		PlatformID:        cfg.PassimPay.PlatformID,
		APIKey:            cfg.PassimPay.APIKey,
		CreateInvoicePath: cfg.PassimPay.CreateInvoicePath,
		CheckInvoicePath:  cfg.PassimPay.CheckInvoicePath,
		InvoiceType:       cfg.PassimPay.InvoiceType,
		CurrencyIDs:       cfg.PassimPay.CurrencyIDs,
		Timeout:           cfg.PassimPay.Timeout,
	})

	topupSvc := topups.NewService(
		topups.NewRepository(pg),
		promoSvc,
		promoRepo,
		profileRepo,
		profileSvc,
		cfg.Bot,
		passimPayClient,
	)
	topupHandler := topups.NewHandler(topupSvc, cfg.Bot.InternalSecret, cfg.Bot.AdminUserID)

	statsHandler := stats.NewHandler(statsSvc)
	spendSyncSvc := spendsync.NewService(pg, statsSvc)
	go runStatsSpendSyncTicker(ctx, cfg, spendSyncSvc)
	go runPassimPayReconcileTicker(ctx, cfg.PassimPay, topupSvc)
	go runNoBudgetTicker(ctx, pg, cfg, campaignSvc)
	go runCampaignCompletedTicker(ctx, pg, cfg, campaignSvc)
	go runWaitingCampaignStartTicker(ctx, pg, campaignSvc)

	r := buildRouter(authSvc, authHandler, profileHandler, campaignHandler, creativeHandler, promoHandler, topupHandler, notificationHandler, statsHandler)
	return &App{Cfg: cfg, Postgres: pg, Stats: statsSvc, Router: r}, nil
}

func runStatsSpendSyncTicker(ctx context.Context, cfg config.Config, service *spendsync.Service) {
	interval := cfg.SpendSync.Interval
	if interval <= 0 {
		log.Printf("stats spend sync disabled: invalid interval %s", interval)
		return
	}
	timeout := cfg.SpendSync.Timeout
	if timeout <= 0 {
		timeout = interval
	}

	run := func() {
		startedAt := time.Now()
		syncCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		result, err := service.Sync(syncCtx)
		duration := time.Since(startedAt)
		if err != nil {
			log.Printf("stats spend sync error after %s: %v", duration, err)
			return
		}
		log.Printf(
			"stats spend sync completed: duration=%s source_rows=%d user_totals=%d campaign_totals=%d updated_users=%d updated_campaigns=%d",
			duration,
			result.SourceRows,
			result.UserTotals,
			result.CampaignTotals,
			result.UpdatedUsers,
			result.UpdatedCampaigns,
		)
	}

	// Synchronize immediately after startup, then continue at the configured interval.
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func runNoBudgetTicker(ctx context.Context, pg *sql.DB, cfg config.Config, campaignSvc *campaigns.Service) {
	t := time.NewTicker(cfg.Notifications.NoBudgetCheckInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			rows, err := pg.QueryContext(ctx, `
				SELECT c.user_id, c.campaign_id
				FROM campaigns c
				WHERE c.goal_total_dollars > 0
				  AND c.status = 'active'
				  AND (c.goal_total_dollars - c.cum_done_dollars) <
					  CASE
						  WHEN LOWER(TRIM(c.pricing_model)) = 'cpm'
							  THEN c.base_price / 1000
						  WHEN LOWER(TRIM(c.pricing_model)) = 'cpc'
							   AND LOWER(TRIM(c.format_type)) = 'popunder'
							  THEN c.base_price / 1000
						  WHEN LOWER(TRIM(c.pricing_model)) = 'cpc'
							  THEN c.base_price
						  ELSE NULL
					  END
				  AND c.no_budget_notified = false
			`)
			if err != nil {
				log.Printf("no budget ticker query error: %v", err)
				continue
			}
			for rows.Next() {
				var userID, campaignID string
				if err := rows.Scan(&userID, &campaignID); err != nil {
					log.Printf("no budget ticker scan error: %v", err)
					continue
				}
				if _, err := campaignSvc.Patch(ctx, campaignID, campaigns.PatchCampaignRequest{Status: strPtr("no_budget"), NoBudgetNotified: booleanPtr(true)}); err != nil {
					log.Printf("no budget ticker patch status error: %v", err)
					continue
				}

			}
			if err := rows.Err(); err != nil {
				log.Printf("no budget ticker rows iteration error: %v", err)
			}
			_ = rows.Close()
		}
	}
}

func runWaitingCampaignStartTicker(ctx context.Context, pg *sql.DB, campaignSvc *campaigns.Service) {
	t := time.NewTicker(1 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			rows, err := pg.QueryContext(ctx, `
				SELECT c.campaign_id
				FROM campaigns c
				WHERE c.status = 'waiting'
				  AND c.start_ts <= NOW()
			`)
			if err != nil {
				log.Printf("campaign waiting ticker query error: %v", err)
				continue
			}
			for rows.Next() {
				var campaignID string
				if err := rows.Scan(&campaignID); err != nil {
					log.Printf("campaign waiting ticker scan error: %v", err)
					continue
				}
				if _, err := campaignSvc.Patch(ctx, campaignID, campaigns.PatchCampaignRequest{Status: strPtr("active")}); err != nil {
					log.Printf("campaign waiting ticker patch status error: %v", err)
					continue
				}
			}
			if err := rows.Err(); err != nil {
				log.Printf("campaign waiting ticker rows iteration error: %v", err)
			}
			_ = rows.Close()
		}
	}
}

func runPassimPayReconcileTicker(ctx context.Context, cfg config.PassimPayConfig, svc *topups.Service) {
	interval := cfg.ReconcileInterval
	if interval <= 0 {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := svc.ReconcilePendingInvoices(ctx, cfg.ReconcileBatchSize, cfg.ReconcileRequestDelay, cfg.ReconcileRetryDelay); err != nil {
				log.Printf("PassimPay reconciliation error: %v", err)
			}
		}
	}
}

func runCampaignCompletedTicker(ctx context.Context, pg *sql.DB, cfg config.Config, campaignSvc *campaigns.Service) {
	t := time.NewTicker(cfg.Notifications.CampaignCompletedCheckInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			rows, err := pg.QueryContext(ctx, `
				SELECT c.campaign_id
				FROM campaigns c
				WHERE c.end_ts < NOW()
				  AND c.status <> 'completed'
			`)
			if err != nil {
				log.Printf("campaign completed ticker query error: %v", err)
				continue
			}
			for rows.Next() {
				fmt.Println("GOT")
				var campaignID string
				if err := rows.Scan(&campaignID); err != nil {
					log.Printf("campaign completed ticker scan error: %v", err)
					continue
				}
				if _, err := campaignSvc.Patch(ctx, campaignID, campaigns.PatchCampaignRequest{Status: strPtr("completed")}); err != nil {
					log.Printf("campaign completed ticker patch status error: %v", err)
					continue
				}
			}
			if err := rows.Err(); err != nil {
				log.Printf("campaign completed ticker rows iteration error: %v", err)
			}
			_ = rows.Close()
		}
	}
}

func (a *App) Close() error {
	if a.Stats != nil {
		_ = a.Stats.Close()
	}
	if a.Postgres != nil {
		return a.Postgres.Close()
	}
	return nil
}

func (a *App) Server() *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf("%s:%d", a.Cfg.HTTP.Host, a.Cfg.HTTP.Port),
		Handler:           a.Router,
		ReadHeaderTimeout: 10 * time.Second,
	}
}

func buildRouter(
	authSvc *auth.Service,
	authHandler *auth.Handler,
	profileHandler *profile.Handler,
	campaignHandler *campaigns.Handler,
	creativeHandler *creatives.Handler,
	promoHandler *promocodes.Handler,
	topupHandler *topups.Handler,
	notificationHandler *notifications.Handler,
	statsHandler *stats.Handler,
) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors)

	r.Get("/api/media/{imageID}", creativeHandler.Media)
	r.Head("/api/media/{imageID}", creativeHandler.Media)
	r.Post("/api/internal/campaigns/{id}/moderation", campaignHandler.Moderate)
	r.Post("/api/webhooks/passimpay", topupHandler.PassimPayWebhook)

	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/signup", authHandler.Signup)
		r.Post("/login", authHandler.Login)
		r.Post("/refresh", authHandler.Refresh)
		r.Patch("/verify", authHandler.VerifyEmail)
		r.Get("/session", authHandler.Session)
		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(authSvc))
			r.Post("/logout", authHandler.Logout)
			r.Post("/password", authHandler.ChangePassword)
		})
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(authSvc))
		r.Get("/api/profile", profileHandler.Get)
		r.Patch("/api/profile", profileHandler.Patch)

		r.Get("/api/campaigns", campaignHandler.List)
		r.Post("/api/campaigns", campaignHandler.Create)
		r.Get("/api/campaigns/{id}", campaignHandler.Get)
		r.Patch("/api/campaigns/{id}", campaignHandler.Patch)
		r.Delete("/api/campaigns/{id}", campaignHandler.Delete)

		r.Get("/api/campaigns/{campaignID}/creatives", creativeHandler.ListByCampaign)
		r.Post("/api/campaigns/{campaignID}/creative-images", creativeHandler.UploadImage)
		r.Post("/api/campaigns/{campaignID}/creatives", creativeHandler.Create)
		r.Patch("/api/creatives/{id}", creativeHandler.Patch)
		r.Delete("/api/creatives/{id}", creativeHandler.Delete)

		r.Get("/api/transactions", topupHandler.List)
		r.Post("/api/transactions", topupHandler.Create)
		r.Get("/api/transactions/{id}", topupHandler.Get)
		r.Patch("/api/transactions/{id}", topupHandler.Patch)
		r.Post("/api/transactions/{id}/cancel", topupHandler.Cancel)

		// Payment-bot callbacks require both a valid backend JWT (this group)
		// and the shared X-Bot-Secret checked by the handler.
		r.Post("/api/transactions/{id}/cancel_admin", topupHandler.CancelAdmin)
		r.Post("/api/transactions/{id}/approve_admin", topupHandler.ApproveAdmin)

		r.Get("/api/promocodes/{code}", promoHandler.GetByCode)

		r.Get("/api/notifications", notificationHandler.List)
		r.Post("/api/notifications", notificationHandler.Create)
		r.Patch("/api/notifications/{id}", notificationHandler.Patch)

		r.Post("/api/stats/query", statsHandler.Query)
		r.Post("/api/calculator", statsHandler.Calculator)
		r.Post("/api/recommend_bid", statsHandler.RecommendBid)
	})

	return r
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,Accept,X-Bot-Secret")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func strPtr(v string) *string { return &v }
func booleanPtr(v bool) *bool { return &v }

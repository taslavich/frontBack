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
	"twinbid-backend/internal/profile"
	"twinbid-backend/internal/promocodes"
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
	pg, err := db.InitDBAndMigrate(ctx, cfg.Postgres.DSN)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	log.Println("✅ Connected to Postgres")

	s3, err := storage.NewS3(ctx, cfg.S3)
	if err != nil {
		return nil, fmt.Errorf("s3: %w", err)
	}
	/*statsSvc, err := stats.NewService(ctx, cfg.ClickHouse)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: %w", err)
	}*/

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

	profileSvc := profile.NewService(profile.NewRepository(pg))
	profileHandler := profile.NewHandler(profileSvc)

	notificationRepo := notifications.NewRepository(pg)
	notificationSvc := notifications.NewService(notificationRepo)
	notificationHandler := notifications.NewHandler(notificationSvc)

	creativeRepo := creatives.NewRepository(pg)
	campaignSvc := campaigns.NewService(
		campaigns.NewRepository(pg),
		creativeRepo,
		profile.NewRepository(pg),
		notificationSvc,
		cfg.SMTP,
		cfg.Bot,
		s3,
	)
	campaignHandler := campaigns.NewHandler(campaignSvc)

	creativeSvc := creatives.NewService(creativeRepo, campaignSvc, s3)
	creativeHandler := creatives.NewHandler(creativeSvc)

	promoRepo := promocodes.NewRepository(pg)
	promoSvc := promocodes.NewService(promoRepo)
	promoHandler := promocodes.NewHandler(promoSvc)

	topupSvc := topups.NewService(topups.NewRepository(pg), promoSvc, promoRepo, profile.NewRepository(pg), cfg.Bot)
	topupHandler := topups.NewHandler(topupSvc)

	/*statsHandler := stats.NewHandler(statsSvc)*/
	go runNoBudgetTicker(ctx, pg, cfg, campaignSvc)

	r := buildRouter(authSvc, authHandler, profileHandler, campaignHandler, creativeHandler, promoHandler, topupHandler, notificationHandler, nil /*statsHandler*/)
	return &App{Cfg: cfg, Postgres: pg /*Stats: statsSvc,*/, Router: r}, nil
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
				  AND c.cum_done_dollars >= c.goal_total_dollars
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
		r.Patch("/api/profile_admin", profileHandler.PatchAdmin)

		r.Get("/api/campaigns", campaignHandler.List)
		r.Post("/api/campaigns", campaignHandler.Create)
		r.Get("/api/campaigns/{id}", campaignHandler.Get)
		r.Patch("/api/campaigns/{id}", campaignHandler.Patch)
		r.Delete("/api/campaigns/{id}", campaignHandler.Delete)

		r.Get("/api/campaigns/{campaignID}/creatives", creativeHandler.ListByCampaign)
		r.Post("/api/campaigns/{campaignID}/creatives", creativeHandler.Create)
		r.Patch("/api/creatives/{id}", creativeHandler.Patch)
		r.Delete("/api/creatives/{id}", creativeHandler.Delete)

		r.Get("/api/transactions", topupHandler.List)
		r.Post("/api/transactions", topupHandler.Create)
		r.Post("/api/transactions/{id}/cancel", topupHandler.Cancel)
		r.Post("/api/transactions/{id}/cancel_admin", topupHandler.CancelAdmin)
		// Backend-only action. Frontend may ignore it; PATCH /api/transactions/{id} intentionally is not implemented.
		r.Post("/api/transactions/{id}/approve", topupHandler.Approve)
		r.Post("/api/transactions/{id}/approve_admin", topupHandler.ApproveAdmin)

		r.Get("/api/promocodes/{code}", promoHandler.GetByCode)

		r.Get("/api/notifications", notificationHandler.List)
		r.Post("/api/notifications", notificationHandler.Create)
		r.Patch("/api/notifications/{id}", notificationHandler.Patch)

		/*r.Post("/api/stats/query", statsHandler.Query)
		r.Get("/api/stats/campaign/{id}/summary", statsHandler.CampaignSummary)
		r.Get("/api/stats/overview", statsHandler.Overview)*/
	})

	return r
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,Accept")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func strPtr(v string) *string { return &v }
func booleanPtr(v bool) *bool { return &v }

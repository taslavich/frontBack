package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"github.com/taslavich/frontBack/config"
	"github.com/taslavich/frontBack/db"
	"github.com/taslavich/frontBack/handlers"
	"github.com/taslavich/frontBack/middleware"
	"github.com/taslavich/frontBack/storage"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()
	pgDB, err := db.NewPostgres(cfg.PostgresDSN)
	if err != nil {
		log.Fatal("Postgres connection error:", err)
	}
	defer pgDB.Close()

	if err := db.Migrate(pgDB); err != nil {
		log.Fatal("Migration error:", err)
	}

	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.FrontendURL},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	authHandler := handlers.NewAuthHandler(pgDB, cfg)
	profileHandler := handlers.NewProfileHandler(pgDB)
	var creativeStorage *storage.S3Storage
	if cfg.S3.Bucket != "" {
		creativeStorage, err = storage.NewS3Storage(
			ctx,
			cfg.S3.Region,
			cfg.S3.Bucket,
			cfg.S3.AccessKeyID,
			cfg.S3.SecretAccessKey,
			cfg.S3.SessionToken,
			cfg.S3.UploadURLTTL,
			cfg.S3.DownloadURLTTL,
		)
		if err != nil {
			log.Fatal("S3 init error:", err)
		}
	}

	marketingHandler := handlers.NewMarketingHandler(pgDB, creativeStorage, cfg.S3.SkipObjectHeadCheck)

	// Публичные маршруты
	r.Post("/api/auth/signup", authHandler.Signup)
	r.Post("/api/auth/login", authHandler.Login)
	r.Post("/api/auth/refresh", authHandler.Refresh)

	// Защищённые маршруты
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(cfg.JWTSecret))
		r.Post("/api/auth/logout", authHandler.Logout)
		r.Get("/api/auth/session", authHandler.GetSession)
		r.Post("/api/auth/password", authHandler.ChangePassword)
		r.Get("/api/profile", profileHandler.GetProfile)
		r.Patch("/api/profile", profileHandler.PatchProfile)
		r.Get("/api/campaigns", marketingHandler.ListCampaigns)
		r.Get("/api/campaigns/{id}", marketingHandler.GetCampaign)
		r.Post("/api/campaigns", marketingHandler.CreateCampaign)
		r.Patch("/api/campaigns/{id}", marketingHandler.PatchCampaign)
		r.Delete("/api/campaigns/{id}", marketingHandler.DeleteCampaign)

		r.Get("/api/campaigns/{cid}/creatives", marketingHandler.ListCreatives)
		r.Post("/api/campaigns/{cid}/creatives", marketingHandler.CreateCreative)
		r.Patch("/api/creatives/{id}", marketingHandler.PatchCreative)
		r.Delete("/api/creatives/{id}", marketingHandler.DeleteCreative)
		r.Post("/api/creatives/upload-url", marketingHandler.GetUploadURL)

		r.Get("/api/topups", marketingHandler.ListTopups)
		r.Post("/api/topups", marketingHandler.CreateTopup)
		r.Patch("/api/topups/{id}", marketingHandler.PatchTopup)
		r.Post("/api/topups/{id}/cancel", marketingHandler.CancelTopup)

		r.Get("/api/promocodes/{code}", marketingHandler.GetPromocode)

		r.Get("/api/notifications", marketingHandler.ListNotifications)
		r.Post("/api/notifications", marketingHandler.CreateNotification)
		r.Patch("/api/notifications/{id}", marketingHandler.PatchNotification)

	})

	log.Printf("Server starting on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatal(err)
	}
}

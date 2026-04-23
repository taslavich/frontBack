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
	"github.com/taslavich/frontBack/utils"
)

func main() {
	cfg := config.Load()
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

	s3Service, err := utils.NewS3Service(context.Background(), utils.S3InitParams{
		Region:             cfg.S3Region,
		Bucket:             cfg.S3Bucket,
		AccessKey:          cfg.S3AccessKey,
		SecretKey:          cfg.S3SecretKey,
		Endpoint:           cfg.S3Endpoint,
		UsePathStyle:       cfg.S3UsePathStyle,
		UploadTTLSeconds:   cfg.S3UploadTTLSeconds,
		DownloadTTLSeconds: cfg.S3DownloadTTLSeconds,
	})
	if err != nil {
		log.Fatal("S3 init error:", err)
	}

	authHandler := handlers.NewAuthHandler(pgDB, cfg)
	profileHandler := handlers.NewProfileHandler(pgDB)
	marketingHandler := handlers.NewMarketingHandler(pgDB, s3Service)

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

package main

import (
	"context"
	"fmt"
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
	ctx := context.Background()

	cfg, err := config.LoadConfig[config.ApiConfig](ctx)
	if err != nil {
		log.Fatalf("Cannot load config: %v", err)
	}
	log.Println("Config initialized!")

	// PostgreSQL
	pgDB, err := db.NewPostgres(cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("Postgres connection error: %v", err)
	}
	defer pgDB.Close()

	if err := db.Migrate(pgDB); err != nil {
		log.Fatalf("Migration error: %v", err)
	}
	log.Println("✅ Connected to PostgreSQL")

	// HTTP роутер
	router := chi.NewRouter()

	// CORS middleware
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.FrontendURL},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	// Инициализация хендлеров
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
			log.Fatalf("S3 initialization error: %v", err)
		}
	}
	marketingHandler := handlers.NewMarketingHandler(pgDB, creativeStorage)

	// Публичные маршруты
	router.Post("/api/auth/signup", authHandler.Signup)
	router.Post("/api/auth/login", authHandler.Login)
	router.Post("/api/auth/refresh", authHandler.Refresh)

	// Защищённые маршруты
	router.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(cfg.JWTSecret))
		r.Post("/api/auth/logout", authHandler.Logout)
		r.Get("/api/auth/session", authHandler.GetSession)
		r.Post("/api/auth/password", authHandler.ChangePassword)
		r.Get("/api/profile", profileHandler.GetProfile)
		r.Patch("/api/profile", profileHandler.PatchProfile)
		r.Get("/api/campaigns/{cid}/creatives", marketingHandler.ListCreatives)
		r.Post("/api/campaigns/{cid}/creatives", marketingHandler.CreateCreative)
		r.Patch("/api/creatives/{id}", marketingHandler.PatchCreative)
		r.Delete("/api/creatives/{id}", marketingHandler.DeleteCreative)
		r.Post("/api/creatives/upload-url", marketingHandler.GetUploadURL)
	})

	// Запуск сервера
	addr := fmt.Sprintf("%s:%d", cfg.HttpServer.Host, cfg.HttpServer.Port)
	log.Printf("🚀 Server starting on %s", addr)

	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"github.com/taslavich/frontBack/config"
	"github.com/taslavich/frontBack/db"
	"github.com/taslavich/frontBack/handlers"
	"github.com/taslavich/frontBack/middleware"
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

	authHandler := handlers.NewAuthHandler(pgDB, cfg)
	profileHandler := handlers.NewProfileHandler(pgDB)

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
	})

	log.Printf("Server starting on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatal(err)
	}
}

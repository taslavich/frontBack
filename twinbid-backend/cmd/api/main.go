package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"twinbid-backend/internal/app"
	"twinbid-backend/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	application, err := app.New(ctx, cfg)
	if err != nil {
		log.Fatalf("app init: %v", err)
	}
	defer application.Close()

	srv := application.Server()
	go func() {
		log.Printf("HTTP server started on %s", srv.Addr)
		if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
			log.Printf("https enabled")
			if err := srv.ListenAndServeTLS(
				cfg.TLSCertFile,
				cfg.TLSKeyFile,
			); err != nil {
				log.Fatal(err)
			}
		} else {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("server: %v", err)
			}
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

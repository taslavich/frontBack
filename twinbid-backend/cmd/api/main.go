package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"twinbid-backend/internal/app"
	"twinbid-backend/internal/config"
	"twinbid-backend/internal/mailer"
	"twinbid-backend/internal/models"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(ctx)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	application, err := app.New(ctx, *cfg)
	if err != nil {
		log.Fatalf("app init: %v", err)
	}
	defer application.Close()

	go runLowBalanceNotifications(ctx, application.Postgres, cfg.Notifications.LowBalanceCheckInterval, cfg.SMTP)

	srv := application.Server()
	go func() {
		log.Printf("HTTP server started on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func runLowBalanceNotifications(ctx context.Context, db *sql.DB, interval time.Duration, smtpCfg config.SMTPConfig) {
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
			users, err := getLowBalanceUsers(ctx, db)
			if err != nil {
				log.Printf("low balance check error: %v", err)
				continue
			}
			for _, user := range users {
				body := fmt.Sprintf("Ваш баланс %.2f меньше чем %.2f.", user.Balance, user.BalanceTreshold)
				if err := mailer.SendEmail(smtpCfg, user.Mail, "Низкий баланс", body); err != nil {
					log.Printf("low balance email error for user %s: %v", user.ID, err)
				}
			}
		}
	}
}

func getLowBalanceUsers(ctx context.Context, db *sql.DB) ([]models.User, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, mail, balance, balance_treshold
		FROM users
		WHERE low_balance_notifications = true
		  AND balance < balance_treshold
		  AND mail <> ''
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.User
	for rows.Next() {
		var item models.User
		if err := rows.Scan(&item.ID, &item.Mail, &item.Balance, &item.BalanceTreshold); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

package publisher

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"twinbid-backend/internal/bot"
	"twinbid-backend/internal/httpx"
	c "twinbid-backend/internal/pubcontract"
)

type SiteReviewCandidate struct {
	SiteID         string
	Hostname       string
	FirstRequestAt time.Time
}

type SiteReviewState struct {
	Decided        string
	BlockRequested bool
}

type siteReviewRepository interface {
	PublisherUnnotifiedSites(context.Context, int) ([]SiteReviewCandidate, error)
	PublisherSiteAwaitingReview(context.Context, string) (bool, error)
	PublisherSiteReviewState(context.Context, string) (SiteReviewState, error)
	AppendPublisherSiteReview(context.Context, string, string, int64) error
}

type siteBlockWriter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func writePublisherSiteBlock(ctx context.Context, db siteBlockWriter, siteID string, blocked bool, reason string) error {
	_, err := db.ExecContext(ctx, `INSERT INTO publisher_site_blocks(site_id,blocked,reason) VALUES($1,$2,$3) ON CONFLICT(site_id) DO UPDATE SET blocked=EXCLUDED.blocked,reason=EXCLUDED.reason,version=publisher_site_blocks.version+1,updated_at=now()`, siteID, blocked, reason)
	return err
}

const (
	siteReviewInterval     = 5 * time.Minute
	siteReviewBatchSize    = 25
	siteReviewNoticeLock   = int64(871922136)
	siteReviewDecisionLock = int64(871922137)
)

// The existing PostgreSQL connection supplies a transient cross-instance lock
// only. Discovery and the entire review/notification history live in ClickHouse.
func (s *Service) withSiteReviewLock(ctx context.Context, key int64, fn func(*sql.Conn) error) error {
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var locked bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&locked); err != nil {
		// The server might have acquired the lock before cancellation.
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		return err
	}
	if !locked {
		return errSiteReviewBusy
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		var unlocked bool
		if err := conn.QueryRowContext(cleanup, "SELECT pg_advisory_unlock($1)", key).Scan(&unlocked); err != nil || !unlocked {
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()
	return fn(conn)
}

var errSiteReviewBusy = errors.New("site review lock is held")

func (s *Service) RunSiteReviewNotices(ctx context.Context) {
	repo, ok := s.Reports.(siteReviewRepository)
	if !ok || s.DB == nil || s.Bot == nil || s.Bot.BaseURL == "" || s.botSecret == "" || s.AdminID == "" {
		return
	}
	ticker := time.NewTicker(siteReviewInterval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		batch, cancel := context.WithTimeout(ctx, 3*time.Minute)
		err := s.withSiteReviewLock(batch, siteReviewNoticeLock, func(_ *sql.Conn) error {
			return dispatchSiteReviewNotices(batch, repo, s.Bot.SendPublisherSiteReview)
		})
		cancel()
		if err != nil && !errors.Is(err, errSiteReviewBusy) && ctx.Err() == nil {
			log.Printf("publisher site review notices: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func dispatchSiteReviewNotices(ctx context.Context, repo siteReviewRepository, send func(context.Context, bot.PublisherSiteReviewRequest) error) error {
	sites, err := repo.PublisherUnnotifiedSites(ctx, siteReviewBatchSize)
	if err != nil {
		return err
	}
	for _, site := range sites {
		if err := send(ctx, bot.PublisherSiteReviewRequest{
			SiteID: site.SiteID, Hostname: site.Hostname, FirstRequestAt: site.FirstRequestAt,
		}); err != nil {
			return fmt.Errorf("send site %s: %w", site.SiteID, err)
		}
		// Finish recording a successful send even if the worker is shutting down.
		markCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 8*time.Second)
		err := repo.AppendPublisherSiteReview(markCtx, site.SiteID, "notified", 0)
		cancel()
		if err != nil {
			return fmt.Errorf("mark site %s notified: %w", site.SiteID, err)
		}
	}
	return nil
}

func (s *Service) siteReviewDecision(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) || s.botSecret == "" ||
		subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Bot-Secret")), []byte(s.botSecret)) != 1 {
		reply(w, nil, httpx.Forbidden("site review requires the admin bot"))
		return
	}
	siteID := chi.URLParam(r, "siteID")
	var req struct {
		Decision    string `json:"decision"`
		ModeratorID int64  `json:"moderator_id"`
	}
	if err := decode(w, r, &req); err != nil {
		reply(w, nil, err)
		return
	}
	if !c.ValidShortID(siteID) || req.ModeratorID <= 0 || (req.Decision != "allowed" && req.Decision != "blocked") {
		reply(w, nil, httpx.BadRequest("invalid site review decision"))
		return
	}
	repo, ok := s.Reports.(siteReviewRepository)
	if !ok {
		reply(w, nil, httpx.ServiceUnavailable("site review unavailable"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	var result string
	err := s.withSiteReviewLock(ctx, siteReviewDecisionLock, func(conn *sql.Conn) error {
		var err error
		result, err = applySiteReview(ctx, repo, siteID, req.Decision, req.ModeratorID, func() error {
			var blocked bool
			err := conn.QueryRowContext(ctx, "SELECT blocked FROM publisher_site_blocks WHERE site_id=$1", siteID).Scan(&blocked)
			if err == nil && blocked {
				return nil // Retry after an already committed block; preserve its reason.
			}
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			return writePublisherSiteBlock(ctx, conn, siteID, true, fmt.Sprintf("Telegram site review; moderator %d", req.ModeratorID))
		})
		return err
	})
	if errors.Is(err, errSiteReviewBusy) {
		err = httpx.ServiceUnavailable("another moderation decision is being saved; retry")
	}
	reply(w, map[string]string{"site_id": siteID, "decision": result}, err)
}

func applySiteReview(ctx context.Context, repo siteReviewRepository, siteID, decision string, moderator int64, block func() error) (string, error) {
	state, err := repo.PublisherSiteReviewState(ctx, siteID)
	if err != nil || state.Decided != "" {
		return state.Decided, err
	}
	if state.BlockRequested && decision != "blocked" {
		return "", httpx.Conflict("a block was requested; retry the Block button")
	}
	if !state.BlockRequested {
		pending, err := repo.PublisherSiteAwaitingReview(ctx, siteID)
		if err != nil {
			return "", err
		}
		if !pending {
			return "", httpx.NotFound("site is not awaiting review")
		}
	}
	if decision == "blocked" {
		// Persist intent before the existing policy write. If either database
		// fails, Block can be retried without an Allow overwriting that intent.
		if !state.BlockRequested {
			if err := repo.AppendPublisherSiteReview(ctx, siteID, "block_requested", moderator); err != nil {
				return "", err
			}
		}
		if err := block(); err != nil {
			return "", err
		}
	}
	if err := repo.AppendPublisherSiteReview(ctx, siteID, decision, moderator); err != nil {
		return "", err
	}
	return decision, nil
}

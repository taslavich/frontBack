package stats

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"twinbid-backend/internal/httpx"
)

var advertiserAPITokenRE = regexp.MustCompile(`^adv_[0-9a-f]{64}$`)

type advertiserAPIRepository interface {
	ConsumeAdvertiserToken(ctx context.Context, token string) (string, error)
	CampaignBelongsToUser(ctx context.Context, userID, campaignID string) (bool, error)
}

type advertiserStatsQuerier interface {
	Query(ctx context.Context, userID string, req QueryRequest) (QueryResponse, error)
}

type AdvertiserAPIHandler struct {
	repo  advertiserAPIRepository
	stats advertiserStatsQuerier
}

func NewAdvertiserAPIHandler(db *sql.DB, stats advertiserStatsQuerier) *AdvertiserAPIHandler {
	return &AdvertiserAPIHandler{
		repo:  &postgresAdvertiserAPIRepository{db: db},
		stats: stats,
	}
}

func newAdvertiserAPIHandlerWithRepository(repo advertiserAPIRepository, stats advertiserStatsQuerier) *AdvertiserAPIHandler {
	return &AdvertiserAPIHandler{repo: repo, stats: stats}
}

// Query handles the public advertiser statistics API.
// Example:
// GET /api/advertiser/stats?token=adv_...&campaign_id=<uuid>&from=2026-09-01&to=2026-09-24&group_by=date
func (h *AdvertiserAPIHandler) Query(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	token := advertiserTokenFromRequest(r)
	campaignID := strings.TrimSpace(q.Get("campaign_id"))
	from := strings.TrimSpace(q.Get("from"))
	to := strings.TrimSpace(q.Get("to"))
	groupBy := GroupBy(strings.TrimSpace(q.Get("group_by")))

	if !advertiserAPITokenRE.MatchString(token) {
		httpx.Error(w, httpx.Unauthorized("invalid advertiser API token"))
		return
	}

	// Consume the token's one-second slot before request validation so malformed
	// requests cannot bypass the per-token anti-abuse limit.
	userID, err := h.repo.ConsumeAdvertiserToken(r.Context(), token)
	if err != nil {
		if isAdvertiserRateLimitError(err) {
			w.Header().Set("Retry-After", "1")
		}
		httpx.Error(w, err)
		return
	}

	if campaignID == "" || from == "" || to == "" || groupBy == "" {
		httpx.Error(w, httpx.BadRequest("campaign_id, from, to and group_by are required"))
		return
	}
	if err := validateUUID(campaignID, "campaign_id"); err != nil {
		httpx.Error(w, err)
		return
	}
	if _, ok := groupColumns[groupBy]; !ok {
		httpx.Error(w, httpx.BadRequest("invalid group_by: "+string(groupBy)))
		return
	}
	if _, _, err := normalizeDates(from, to); err != nil {
		httpx.Error(w, err)
		return
	}

	owned, err := h.repo.CampaignBelongsToUser(r.Context(), userID, campaignID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	if !owned {
		httpx.Error(w, httpx.NotFound("campaign not found"))
		return
	}

	res, err := h.stats.Query(r.Context(), userID, QueryRequest{
		From:        from,
		To:          to,
		CampaignIDs: []string{campaignID},
		GroupBy:     groupBy,
	})
	if err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.JSON(w, http.StatusOK, res)
}

type postgresAdvertiserAPIRepository struct {
	db *sql.DB
}

func (r *postgresAdvertiserAPIRepository) ConsumeAdvertiserToken(ctx context.Context, token string) (string, error) {
	var userID string
	err := r.db.QueryRowContext(ctx, `
		UPDATE users
		SET advertiser_api_last_request_at = NOW()
		WHERE advertiser_api_token = $1
		  AND (
			advertiser_api_last_request_at IS NULL
			OR advertiser_api_last_request_at <= NOW() - INTERVAL '1 second'
		  )
		RETURNING id
	`, token).Scan(&userID)
	if err == nil {
		return userID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	var exists bool
	if err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM users
			WHERE advertiser_api_token = $1
		)
	`, token).Scan(&exists); err != nil {
		return "", err
	}
	if !exists {
		return "", httpx.Unauthorized("invalid advertiser API token")
	}

	return "", httpx.HTTPError{
		Status:  http.StatusTooManyRequests,
		Code:    "rate_limited",
		Message: "advertiser API rate limit exceeded: maximum 1 request per second per token",
	}
}

func (r *postgresAdvertiserAPIRepository) CampaignBelongsToUser(ctx context.Context, userID, campaignID string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM campaigns
			WHERE campaign_id = $1 AND user_id = $2
		)
	`, campaignID, userID).Scan(&exists)
	return exists, err
}

func isAdvertiserRateLimitError(err error) bool {
	var httpErr httpx.HTTPError
	return errors.As(err, &httpErr) && httpErr.Status == http.StatusTooManyRequests
}

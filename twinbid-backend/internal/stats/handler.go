package stats

import (
	"context"
	"net/http"

	"twinbid-backend/internal/auth"
	"twinbid-backend/internal/httpx"
)

type handlerService interface {
	Query(ctx context.Context, userID string, req QueryRequest) (QueryResponse, error)
	Calculator(ctx context.Context, req TrafficSegmentRequest) (CalculatorResponse, error)
	RecommendBid(ctx context.Context, req TrafficSegmentRequest) (RecommendBidResponse, error)
}

type Handler struct {
	svc handlerService
}

func NewHandler(svc handlerService) *Handler {
	return &Handler{svc: svc}
}

// Query handles POST /api/stats/query.
func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	var req QueryRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	res, err := h.svc.Query(r.Context(), auth.UserID(r), req)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.JSON(w, http.StatusOK, res)
}

// Calculator returns the historical available impression volume for the
// latest fully closed UTC day matching the selected segment.
func (h *Handler) Calculator(w http.ResponseWriter, r *http.Request) {
	var req TrafficSegmentRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	res, err := h.svc.Calculator(r.Context(), req)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.JSON(w, http.StatusOK, res)
}

// RecommendBid returns the weighted average non-zero winning bid for the
// latest fully closed UTC day matching the selected segment.
func (h *Handler) RecommendBid(w http.ResponseWriter, r *http.Request) {
	var req TrafficSegmentRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	res, err := h.svc.RecommendBid(r.Context(), req)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	httpx.JSON(w, http.StatusOK, res)
}

package stats

import (
	"context"
	"net/http"

	"twinbid-backend/internal/auth"
	"twinbid-backend/internal/httpx"
)

type queryService interface {
	Query(ctx context.Context, userID string, req QueryRequest) (QueryResponse, error)
}

type Handler struct {
	svc queryService
}

func NewHandler(svc queryService) *Handler {
	return &Handler{svc: svc}
}

// Query handles the only stats endpoint: POST /api/stats/query.
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

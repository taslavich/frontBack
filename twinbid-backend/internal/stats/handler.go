package stats

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"twinbid-backend/internal/auth"
	"twinbid-backend/internal/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

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

func (h *Handler) CampaignSummary(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.CampaignSummary(r.Context(), auth.UserID(r), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.Overview(r.Context(), auth.UserID(r))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

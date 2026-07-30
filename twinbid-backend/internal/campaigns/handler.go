package campaigns

import (
	"crypto/subtle"
	"net/http"

	"github.com/go-chi/chi/v5"
	"twinbid-backend/internal/auth"
	"twinbid-backend/internal/httpx"
)

type Handler struct {
	svc       *Service
	botSecret string
}

func NewHandler(svc *Service, botSecret string) *Handler {
	return &Handler{svc: svc, botSecret: botSecret}
}

type listResponse struct {
	Items any `json:"items"`
	Total int `json:"total"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context(), auth.UserID(r))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, listResponse{Items: items, Total: len(items)})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	item, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req UpsertCampaignRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	item, err := h.svc.Create(r.Context(), auth.UserID(r), req)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, item)
}

func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	var req PatchCampaignRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	item, err := h.svc.Patch(r.Context(), chi.URLParam(r, "id"), req)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (h *Handler) Moderate(w http.ResponseWriter, r *http.Request) {
	if !h.validBotSecret(r.Header.Get("X-Bot-Secret")) {
		httpx.Error(w, httpx.Unauthorized("invalid X-Bot-Secret"))
		return
	}

	var req ModerateCampaignRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	item, err := h.svc.Moderate(r.Context(), chi.URLParam(r, "id"), req.Decision)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (h *Handler) validBotSecret(got string) bool {
	if h.botSecret == "" || len(got) != len(h.botSecret) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(h.botSecret)) == 1
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), auth.UserID(r), chi.URLParam(r, "id")); err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.NoContent(w)
}

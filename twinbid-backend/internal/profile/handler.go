package profile

import (
	"net/http"

	"twinbid-backend/internal/auth"
	"twinbid-backend/internal/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	u, err := h.svc.Get(r.Context(), auth.UserID(r))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}

func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	var req PatchProfileRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	u, err := h.svc.Patch(r.Context(), auth.UserID(r), req)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}

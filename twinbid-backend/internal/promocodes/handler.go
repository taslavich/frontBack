package promocodes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"twinbid-backend/internal/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) GetByCode(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetByCode(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

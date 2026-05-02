package topups

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"twinbid-backend/internal/auth"
	"twinbid-backend/internal/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

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
	item, err := h.svc.Get(r.Context(), auth.UserID(r), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateTopupRequest
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

func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	item, err := h.svc.Cancel(r.Context(), auth.UserID(r), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (h *Handler) Approve(w http.ResponseWriter, r *http.Request) {
	item, err := h.svc.Approve(r.Context(), auth.UserID(r), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	var req PatchTopupRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	item, err := h.svc.Patch(r.Context(), auth.UserID(r), chi.URLParam(r, "id"), req)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (h *Handler) CancelAdmin(w http.ResponseWriter, r *http.Request) {
	var req AdminTopupActionRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	if req.UserID == "" {
		httpx.Error(w, httpx.BadRequest("user_id is required"))
		return
	}

	item, err := h.svc.Cancel(r.Context(), req.UserID, chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (h *Handler) ApproveAdmin(w http.ResponseWriter, r *http.Request) {
	var req AdminTopupActionRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	if req.UserID == "" {
		httpx.Error(w, httpx.BadRequest("user_id is required"))
		return
	}

	item, err := h.svc.Approve(r.Context(), req.UserID, chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

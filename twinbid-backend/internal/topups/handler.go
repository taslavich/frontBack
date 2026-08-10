package topups

import (
	"crypto/subtle"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"twinbid-backend/internal/auth"
	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/payments"
)

const maxWebhookBody = 1 << 20

type Handler struct {
	svc        *Service
	botSecret  string
	botAdminID string
}

func NewHandler(svc *Service, botSecret, botAdminID string) *Handler {
	return &Handler{svc: svc, botSecret: botSecret, botAdminID: botAdminID}
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
	if !h.validBotRequest(r) {
		httpx.Error(w, httpx.Forbidden("invalid X-Bot-Secret"))
		return
	}

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
	if !h.validBotRequest(r) {
		httpx.Error(w, httpx.Forbidden("invalid X-Bot-Secret"))
		return
	}

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

func (h *Handler) PassimPayWebhook(w http.ResponseWriter, r *http.Request) {
	h.handleProviderWebhook(w, r, payments.ProviderPassimPay)
}

func (h *Handler) CryptomusWebhook(w http.ResponseWriter, r *http.Request) {
	h.handleProviderWebhook(w, r, payments.ProviderCryptomus)
}

func (h *Handler) handleProviderWebhook(w http.ResponseWriter, r *http.Request, provider string) {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody+1))
	if err != nil {
		httpx.Error(w, httpx.BadRequest("cannot read webhook body"))
		return
	}
	if len(body) > maxWebhookBody {
		httpx.Error(w, httpx.BadRequest("webhook body is too large"))
		return
	}
	if err := h.svc.HandleProviderWebhook(r.Context(), provider, body, r.Header); err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) validBotRequest(r *http.Request) bool {
	gotSecret := r.Header.Get("X-Bot-Secret")
	if h.botSecret == "" || h.botAdminID == "" || len(gotSecret) != len(h.botSecret) {
		return false
	}
	secretOK := subtle.ConstantTimeCompare([]byte(gotSecret), []byte(h.botSecret)) == 1
	adminIDOK := subtle.ConstantTimeCompare([]byte(auth.UserID(r)), []byte(h.botAdminID)) == 1
	return secretOK && adminIDOK
}

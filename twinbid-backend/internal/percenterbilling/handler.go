package percenterbilling

import (
	"context"
	"crypto/subtle"
	"net/http"

	"twinbid-backend/internal/httpx"
)

type promoApplier interface {
	ApplyPromoSpend(context.Context, ApplyRequest) (PromoState, error)
}

type Handler struct {
	repo           promoApplier
	internalSecret string
}

func NewHandler(repo promoApplier, internalSecret string) *Handler {
	return &Handler{repo: repo, internalSecret: internalSecret}
}

func (h *Handler) Apply(w http.ResponseWriter, r *http.Request) {
	if !h.validInternalSecret(r.Header.Get("X-Bot-Secret")) {
		httpx.Error(w, httpx.Unauthorized("invalid X-Bot-Secret"))
		return
	}
	var req ApplyRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	state, err := h.repo.ApplyPromoSpend(r.Context(), req)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, state)
}

func (h *Handler) validInternalSecret(got string) bool {
	if h.internalSecret == "" || len(got) != len(h.internalSecret) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(h.internalSecret)) == 1
}

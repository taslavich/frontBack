package auth

import (
	"net/http"

	"twinbid-backend/internal/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

type signupRequest struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	FullName        string `json:"full_name"`
	ManagerTelegram string `json:"manager_telegram"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}
type changePasswordRequest struct {
	NewPassword string `json:"new_password"`
}
type verifyEmailRequest struct {
	Token string `json:"token"`
}

type sessionResponse struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	res, err := h.svc.Signup(r.Context(), req.Email, req.Password, req.FullName, req.ManagerTelegram)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, res)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	res, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	res, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Logout(r.Context(), UserID(r)); err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) Session(w http.ResponseWriter, r *http.Request) {
	token := BearerToken(r)
	if token == "" {
		httpx.JSON(w, http.StatusOK, nil)
		return
	}
	userID, err := h.svc.VerifyAccessToken(token)
	if err != nil {
		httpx.JSON(w, http.StatusOK, nil)
		return
	}
	u, err := h.svc.Session(r.Context(), userID)
	if err != nil {
		httpx.JSON(w, http.StatusOK, nil)
		return
	}
	httpx.JSON(w, http.StatusOK, sessionResponse{UserID: u.ID, Email: u.Mail, FullName: u.Name})
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	if err := h.svc.ChangePassword(r.Context(), UserID(r), req.NewPassword); err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req verifyEmailRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	status, err := h.svc.VerifyEmail(r.Context(), req.Token)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	if status == http.StatusNotFound {
		httpx.Error(w, httpx.NotFound("verification token not found"))
		return
	}
	if status == http.StatusConflict {
		httpx.JSON(w, http.StatusConflict, map[string]any{"success": false, "errorMsg": "already verified"})
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"success": true, "errorMsg": ""})
}

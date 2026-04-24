package auth

import (
	"context"
	"net/http"
	"strings"

	"twinbid-backend/internal/httpx"
)

type ctxKey string

const userIDKey ctxKey = "user_id"

func Middleware(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" {
				httpx.Error(w, httpx.Unauthorized("missing bearer token"))
				return
			}
			userID, err := svc.VerifyAccessToken(token)
			if err != nil {
				httpx.Error(w, httpx.Unauthorized("invalid access token"))
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserID(r *http.Request) string {
	v, _ := r.Context().Value(userIDKey).(string)
	return v
}

func BearerToken(r *http.Request) string { return bearerToken(r) }

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

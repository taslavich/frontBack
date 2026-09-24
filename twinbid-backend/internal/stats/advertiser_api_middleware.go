package stats

import (
	"context"
	"net/http"
	"net/url"
)

type advertiserTokenContextKey struct{}

// RedactAdvertiserTokenQuery must be installed before request logging middleware.
// The public API intentionally receives its token in the URL, so this middleware
// removes the raw token from RequestURI/URL before it reaches the logger while
// preserving the original value only in request context for the API handler.
func RedactAdvertiserTokenQuery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL == nil || r.URL.Path != "/api/advertiser/stats" {
			next.ServeHTTP(w, r)
			return
		}

		q := r.URL.Query()
		token := q.Get("token")
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		clone := r.Clone(context.WithValue(r.Context(), advertiserTokenContextKey{}, token))
		clone.URL = cloneAdvertiserURL(r.URL)
		redacted := clone.URL.Query()
		redacted.Set("token", "[REDACTED]")
		clone.URL.RawQuery = redacted.Encode()
		clone.RequestURI = clone.URL.RequestURI()
		next.ServeHTTP(w, clone)
	})
}

func advertiserTokenFromRequest(r *http.Request) string {
	if token, ok := r.Context().Value(advertiserTokenContextKey{}).(string); ok {
		return token
	}
	return r.URL.Query().Get("token")
}

func cloneAdvertiserURL(src *url.URL) *url.URL {
	clone := *src
	return &clone
}

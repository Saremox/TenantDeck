package httpapi

import (
	"context"
	"net/http"

	"github.com/Saremox/TenantDeck/internal/auth"
	"github.com/Saremox/TenantDeck/internal/session"
)

// securityHeaders sets the response headers required by
// docs/spec/04-auth-session-browser-security.md regardless of which
// handler ends up serving the request: a strict CSP with no third-party
// script sources, anti-framing, content-type, and referrer protection, and
// no-store so nothing here is cached. HSTS is added separately in
// production (non-insecure) mode, since it actively breaks plain-http
// loopback development.
func securityHeaders(insecure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Security-Policy", "default-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Cache-Control", "no-store")
			if !insecure {
				h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

type sessionContextKey struct{}

func sessionFromContext(ctx context.Context) (session.Record, bool) {
	rec, ok := ctx.Value(sessionContextKey{}).(session.Record)
	return rec, ok
}

// requireSession fails closed: any problem reading the session cookie or
// looking it up in the store - missing, expired, malformed, or the store
// itself being unreachable - is the same 401, never a fallback to treating
// the caller as authenticated. See docs/spec/04-auth-session-browser-security.md
// ("Store unavailable means protected operations fail closed").
func requireSession(store *session.Store, insecure bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.SessionCookieName(insecure))
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		rec, err := store.GetSession(r.Context(), cookie.Value)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), sessionContextKey{}, rec)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

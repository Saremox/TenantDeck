// Package httpapi wires the BFF's HTTP routes: OIDC login/callback/session/
// logout (internal/auth) and the read-only API surface from
// docs/route-allowlist.md, behind the security headers and session
// middleware every response needs.
package httpapi

import (
	"net/http"

	"github.com/Saremox/TenantDeck/internal/auth"
	"github.com/Saremox/TenantDeck/internal/capsule"
	"github.com/Saremox/TenantDeck/internal/session"
)

// NewRouter builds the BFF's full route table. Routes not listed here
// don't exist - there is no catch-all API handler to accidentally expose
// something off docs/route-allowlist.md. frontend, if non-nil, serves the
// embedded built frontend for every path this mux doesn't otherwise claim
// (see internal/webassets); it's nil until that's wired up.
func NewRouter(authHandler *auth.Handler, store *session.Store, capsuleClient *capsule.Client, insecure bool, frontend http.Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /auth/login", authHandler.LoginHandler)
	mux.HandleFunc("GET /auth/callback", authHandler.CallbackHandler)
	mux.HandleFunc("GET /auth/session", authHandler.SessionHandler)
	mux.HandleFunc("POST /auth/logout", authHandler.LogoutHandler)

	mux.Handle("GET /api/namespaces", requireSession(store, insecure, namespacesHandler(capsuleClient)))

	if frontend != nil {
		mux.Handle("/", frontend)
	}

	return securityHeaders(insecure)(mux)
}

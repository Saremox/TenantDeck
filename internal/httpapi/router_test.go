package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Saremox/TenantDeck/internal/auth"
	"github.com/Saremox/TenantDeck/internal/capsule"
	"github.com/Saremox/TenantDeck/internal/config"
)

// newTestOPForRouter serves just enough OIDC discovery for auth.NewHandler
// to succeed - routing tests don't need a working token exchange, only a
// constructible Handler.
func newTestOPForRouter(t *testing.T) string {
	t.Helper()
	var issuer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"issuer": "` + issuer + `",
			"authorization_endpoint": "` + issuer + `/authorize",
			"token_endpoint": "` + issuer + `/token",
			"jwks_uri": "` + issuer + `/jwks",
			"id_token_signing_alg_values_supported": ["RS256"]
		}`))
	}))
	issuer = srv.URL
	t.Cleanup(srv.Close)
	return srv.URL
}

// This only checks that each path reaches the handler it's supposed to -
// the handlers' own behavior is covered by internal/auth's and this
// package's other tests.
func TestRouter_DispatchesEachRouteToItsHandler(t *testing.T) {
	cfg := &config.Config{
		ExternalURL: "https://tenantdeck.example.com",
		Insecure:    true,
		OIDC:        config.OIDCConfig{IssuerURL: newTestOPForRouter(t), ClientID: "client", Scopes: []string{"openid"}},
	}
	store := newTestStoreForHTTPAPI(t)
	authHandler, err := auth.NewHandler(context.Background(), cfg, store)
	if err != nil {
		t.Fatalf("auth.NewHandler: %v", err)
	}
	capsuleClient := capsule.NewClient("http://unused.invalid", nil)
	router := NewRouter(authHandler, store, capsuleClient, true, nil)

	// /auth/login: reaching LoginHandler means a 302 to the IdP.
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/login", nil))
	if rec.Code != http.StatusFound {
		t.Errorf("/auth/login status = %d, want %d", rec.Code, http.StatusFound)
	}

	// /auth/session: reaching SessionHandler means a 200 with a JSON body,
	// even with no session.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/session", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("/auth/session status = %d, want %d", rec.Code, http.StatusOK)
	}

	// /auth/logout: reaching LogoutHandler with no session cookie means 401
	// (not 404, which would mean the route isn't wired at all).
	rec = httptest.NewRecorder()
	logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	logoutReq.Header.Set("Origin", "https://tenantdeck.example.com")
	router.ServeHTTP(rec, logoutReq)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("/auth/logout status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	// /api/namespaces: reaching requireSession with no cookie means 401.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/namespaces", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("/api/namespaces status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	// Every response, including the 401s above, must carry the security
	// headers - prove it on one of them.
	if got := rec.Header().Get("Content-Security-Policy"); got == "" {
		t.Error("router response missing Content-Security-Policy")
	}
}

// All of Phase 3's new routes - proves each is actually mounted at the
// path docs/route-allowlist.md specifies (a 401, not a 404, means it's
// wired; a 404 here would mean the route isn't registered at all).
func TestRouter_DispatchesPhase3RoutesToTheirHandlers(t *testing.T) {
	cfg := &config.Config{
		ExternalURL: "https://tenantdeck.example.com",
		Insecure:    true,
		OIDC:        config.OIDCConfig{IssuerURL: newTestOPForRouter(t), ClientID: "client"},
	}
	store := newTestStoreForHTTPAPI(t)
	authHandler, err := auth.NewHandler(context.Background(), cfg, store)
	if err != nil {
		t.Fatalf("auth.NewHandler: %v", err)
	}
	router := NewRouter(authHandler, store, capsule.NewClient("http://unused.invalid", nil), true, nil)

	paths := []string{
		"/api/namespaces/ns/overview",
		"/api/namespaces/ns/deployments",
		"/api/namespaces/ns/deployments/api",
		"/api/namespaces/ns/statefulsets",
		"/api/namespaces/ns/statefulsets/db",
		"/api/namespaces/ns/daemonsets",
		"/api/namespaces/ns/daemonsets/agent",
		"/api/namespaces/ns/pods",
		"/api/namespaces/ns/pods/web-1",
		"/api/namespaces/ns/pods/web-1/logs",
		"/api/namespaces/ns/jobs",
		"/api/namespaces/ns/jobs/migrate",
		"/api/namespaces/ns/cronjobs",
		"/api/namespaces/ns/cronjobs/backup",
		"/api/namespaces/ns/services",
		"/api/namespaces/ns/services/web",
		"/api/namespaces/ns/ingresses",
		"/api/namespaces/ns/ingresses/web",
		"/api/namespaces/ns/persistentvolumeclaims",
		"/api/namespaces/ns/persistentvolumeclaims/data",
		"/api/namespaces/ns/events",
	}
	for _, path := range paths {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s status = %d, want %d (route not wired, or requireSession not applied)", path, rec.Code, http.StatusUnauthorized)
		}
	}
}

func TestRouter_ServesFrontendHandlerForUnmatchedPaths(t *testing.T) {
	cfg := &config.Config{
		ExternalURL: "https://tenantdeck.example.com",
		Insecure:    true,
		OIDC:        config.OIDCConfig{IssuerURL: newTestOPForRouter(t), ClientID: "client"},
	}
	store := newTestStoreForHTTPAPI(t)
	authHandler, err := auth.NewHandler(context.Background(), cfg, store)
	if err != nil {
		t.Fatalf("auth.NewHandler: %v", err)
	}

	frontendCalled := false
	frontend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { frontendCalled = true })
	router := NewRouter(authHandler, store, capsule.NewClient("http://unused.invalid", nil), true, frontend)

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/some/frontend/route", nil))
	if !frontendCalled {
		t.Error("frontend handler was not called for an unmatched path")
	}
}

func TestRouter_HasNoFrontendFallbackWhenFrontendIsNil(t *testing.T) {
	cfg := &config.Config{
		ExternalURL: "https://tenantdeck.example.com",
		Insecure:    true,
		OIDC:        config.OIDCConfig{IssuerURL: newTestOPForRouter(t), ClientID: "client"},
	}
	store := newTestStoreForHTTPAPI(t)
	authHandler, err := auth.NewHandler(context.Background(), cfg, store)
	if err != nil {
		t.Fatalf("auth.NewHandler: %v", err)
	}
	router := NewRouter(authHandler, store, capsule.NewClient("http://unused.invalid", nil), true, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/some/frontend/route", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d with no frontend handler registered", rec.Code, http.StatusNotFound)
	}
}

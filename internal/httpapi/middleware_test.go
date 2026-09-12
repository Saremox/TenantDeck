package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Saremox/TenantDeck/internal/session"
)

func ok(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

func TestSecurityHeaders_SetsCSPAndAntiFramingHeaders(t *testing.T) {
	h := securityHeaders(true)(http.HandlerFunc(ok))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Header().Get("Content-Security-Policy"); got == "" {
		t.Error("Content-Security-Policy is empty")
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want %q", got, "nosniff")
	}
	if got := rec.Header().Get("Referrer-Policy"); got == "" {
		t.Error("Referrer-Policy is empty")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
}

func TestSecurityHeaders_OmitsHSTSWhenInsecure(t *testing.T) {
	h := securityHeaders(true)(http.HandlerFunc(ok))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("Strict-Transport-Security = %q, want empty in loopback dev mode", got)
	}
}

func TestSecurityHeaders_SetsHSTSWhenNotInsecure(t *testing.T) {
	h := securityHeaders(false)(http.HandlerFunc(ok))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Header().Get("Strict-Transport-Security"); got == "" {
		t.Error("Strict-Transport-Security is empty, want it set in production mode")
	}
}

func TestRequireSession_RejectsMissingCookie(t *testing.T) {
	store := newTestStoreForHTTPAPI(t)
	h := requireSession(store, true, http.HandlerFunc(ok))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/namespaces", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireSession_RejectsUnknownSessionID(t *testing.T) {
	store := newTestStoreForHTTPAPI(t)
	h := requireSession(store, true, http.HandlerFunc(ok))

	req := httptest.NewRequest(http.MethodGet, "/api/namespaces", nil)
	req.AddCookie(&http.Cookie{Name: "td-session", Value: "never-issued"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireSession_CallsNextWithSessionInContextOnValidCookie(t *testing.T) {
	store := newTestStoreForHTTPAPI(t)
	id, err := store.CreateSession(context.Background(), session.Record{Subject: "alice", IDToken: "itok"}, time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	var gotSubject string
	var gotOK bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec, ok := sessionFromContext(r.Context())
		gotSubject, gotOK = rec.Subject, ok
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/namespaces", nil)
	req.AddCookie(&http.Cookie{Name: "td-session", Value: id})
	rec := httptest.NewRecorder()
	requireSession(store, true, next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !gotOK || gotSubject != "alice" {
		t.Errorf("sessionFromContext() = (%q, %v), want (%q, true)", gotSubject, gotOK, "alice")
	}
}

func TestRequireSession_RejectsWhenStoreUnavailable(t *testing.T) {
	backend := session.NewRedisBackend(session.RedisOptions{Addr: "127.0.0.1:1"})
	store, err := session.NewStore(backend, make([]byte, session.SessionKeySize))
	if err != nil {
		t.Fatalf("session.NewStore: %v", err)
	}
	h := requireSession(store, true, http.HandlerFunc(ok))

	req := httptest.NewRequest(http.MethodGet, "/api/namespaces", nil)
	req.AddCookie(&http.Cookie{Name: "td-session", Value: "anything"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

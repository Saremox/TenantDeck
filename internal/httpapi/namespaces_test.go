package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Saremox/TenantDeck/internal/capsule"
	"github.com/Saremox/TenantDeck/internal/session"
)

func requestWithSession(t *testing.T, rec session.Record) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/namespaces", nil)
	ctx := context.WithValue(req.Context(), sessionContextKey{}, rec)
	return req.WithContext(ctx)
}

func TestNamespacesHandler_ReturnsNamespacesFromUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items": [{"metadata": {"name": "tenant-a-dev"}}]}`))
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	namespacesHandler(client).ServeHTTP(rec, requestWithSession(t, session.Record{Subject: "alice", IDToken: "itok"}))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q, want %d", rec.Code, rec.Body.String(), http.StatusOK)
	}
	var resp namespacesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(resp.Namespaces) != 1 || resp.Namespaces[0].Name != "tenant-a-dev" {
		t.Errorf("Namespaces = %+v, want one namespace named tenant-a-dev", resp.Namespaces)
	}
}

func TestNamespacesHandler_UsesTheSessionsIDTokenAsBearerCredential(t *testing.T) {
	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"items": []}`))
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	namespacesHandler(client).ServeHTTP(rec, requestWithSession(t, session.Record{IDToken: "the-session-id-token"}))

	if gotAuth != "Bearer the-session-id-token" {
		t.Errorf("Authorization sent upstream = %q, want %q", gotAuth, "Bearer the-session-id-token")
	}
}

func TestNamespacesHandler_MapsUpstreamUnauthorizedTo401(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer upstream.Close()

	rec := httptest.NewRecorder()
	namespacesHandler(capsule.NewClient(upstream.URL, nil)).ServeHTTP(rec, requestWithSession(t, session.Record{IDToken: "itok"}))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestNamespacesHandler_MapsUpstreamForbiddenTo403(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer upstream.Close()

	rec := httptest.NewRecorder()
	namespacesHandler(capsule.NewClient(upstream.URL, nil)).ServeHTTP(rec, requestWithSession(t, session.Record{IDToken: "itok"}))

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestNamespacesHandler_MapsOtherUpstreamFailureTo502(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	rec := httptest.NewRecorder()
	namespacesHandler(capsule.NewClient(upstream.URL, nil)).ServeHTTP(rec, requestWithSession(t, session.Record{IDToken: "itok"}))

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
}

func TestNamespacesHandler_RejectsRequestWithNoSessionInContext(t *testing.T) {
	client := capsule.NewClient("http://unused.invalid", nil)
	rec := httptest.NewRecorder()
	namespacesHandler(client).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/namespaces", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

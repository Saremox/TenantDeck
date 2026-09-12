package capsule

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestListNamespaces_ReturnsParsedNamespacesOnSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"apiVersion": "v1",
			"kind": "NamespaceList",
			"items": [
				{"metadata": {"name": "tenant-a-dev"}},
				{"metadata": {"name": "tenant-a-prod"}}
			]
		}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, nil)
	got, err := client.ListNamespaces(context.Background(), "test-id-token")
	if err != nil {
		t.Fatalf("ListNamespaces: %v", err)
	}
	want := []Namespace{{Name: "tenant-a-dev"}, {Name: "tenant-a-prod"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("ListNamespaces() = %+v, want %+v", got, want)
	}
}

func TestListNamespaces_ReturnsEmptySliceForEmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items": []}`))
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, nil).ListNamespaces(context.Background(), "token")
	if err != nil {
		t.Fatalf("ListNamespaces: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ListNamespaces() = %+v, want an empty slice", got)
	}
}

func TestListNamespaces_RequestsExactlyTheAllowlistedPathAndMethod(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.Write([]byte(`{"items": []}`))
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL, nil).ListNamespaces(context.Background(), "token"); err != nil {
		t.Fatalf("ListNamespaces: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want %q", gotMethod, http.MethodGet)
	}
	if gotPath != "/api/v1/namespaces" {
		t.Errorf("path = %q, want %q", gotPath, "/api/v1/namespaces")
	}
}

// This is the upstream-boundary property that matters most: the only
// identity-bearing header reaching Capsule Proxy is the one TenantDeck
// itself builds from the stored ID token - see docs/route-allowlist.md.
func TestListNamespaces_SendsOnlyTheBearerIDTokenAsAuthorization(t *testing.T) {
	var gotAuth, gotAccept string
	var gotCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotCookie = r.Header.Get("Cookie")
		w.Write([]byte(`{"items": []}`))
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL, nil).ListNamespaces(context.Background(), "the-id-token"); err != nil {
		t.Fatalf("ListNamespaces: %v", err)
	}
	if gotAuth != "Bearer the-id-token" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer the-id-token")
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want %q", gotAccept, "application/json")
	}
	if gotCookie != "" {
		t.Errorf("Cookie = %q, want empty - nothing should ever set this header upstream", gotCookie)
	}
}

func TestListNamespaces_ReturnsErrUnauthorizedOn401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, nil).ListNamespaces(context.Background(), "token")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("err = %v, want ErrUnauthorized", err)
	}
}

func TestListNamespaces_ReturnsErrForbiddenOn403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, nil).ListNamespaces(context.Background(), "token")
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
}

func TestListNamespaces_ReturnsErrorOnUnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, nil).ListNamespaces(context.Background(), "token")
	if err == nil {
		t.Fatal("ListNamespaces() succeeded on a 500 response, want error")
	}
	if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrForbidden) {
		t.Errorf("err = %v, want neither ErrUnauthorized nor ErrForbidden for a 500", err)
	}
}

func TestListNamespaces_ReturnsErrorOnMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{not valid json`))
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL, nil).ListNamespaces(context.Background(), "token"); err == nil {
		t.Fatal("ListNamespaces() succeeded on malformed JSON, want error")
	}
}

func TestListNamespaces_ReturnsErrorWhenResponseExceedsSizeLimit(t *testing.T) {
	oversized := strings.Repeat("a", maxResponseBytes+1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Content doesn't need to be valid JSON: the size check runs
		// before JSON parsing and must reject this regardless of content.
		w.Write([]byte(oversized))
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL, nil).ListNamespaces(context.Background(), "token"); err == nil {
		t.Fatal("ListNamespaces() succeeded on an oversized response, want error")
	}
}

func TestListNamespaces_ReturnsErrorForInvalidBaseURL(t *testing.T) {
	client := NewClient("http://\x7f", nil) // control character: rejected when building the request
	if _, err := client.ListNamespaces(context.Background(), "token"); err == nil {
		t.Fatal("ListNamespaces() succeeded with an invalid base URL, want error")
	}
}

func TestListNamespaces_RespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Write([]byte(`{"items": []}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := NewClient(srv.URL, nil).ListNamespaces(ctx, "token")
	if err == nil {
		t.Fatal("ListNamespaces() succeeded despite context deadline, want error")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("err = %v, want it to mention the context deadline", err)
	}
}

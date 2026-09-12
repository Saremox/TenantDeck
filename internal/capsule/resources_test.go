package capsule

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

var deploymentsGVR = GroupVersionResource{APIPrefix: "apis/apps/v1", Resource: "deployments"}

func TestListNamespacedResource_RequestsExactlyTheExpectedPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"items": []}`))
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL, nil).ListNamespacedResource(context.Background(), "token", "tenant-a-dev", deploymentsGVR, 0); err != nil {
		t.Fatalf("ListNamespacedResource: %v", err)
	}
	if want := "/apis/apps/v1/namespaces/tenant-a-dev/deployments"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

func TestListNamespacedResource_ReturnsEachItemsRawJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items": [{"metadata": {"name": "api"}, "status": {"readyReplicas": 2}}]}`))
	}))
	defer srv.Close()

	items, err := NewClient(srv.URL, nil).ListNamespacedResource(context.Background(), "token", "ns", deploymentsGVR, 0)
	if err != nil {
		t.Fatalf("ListNamespacedResource: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	var decoded struct {
		Metadata struct{ Name string }       `json:"metadata"`
		Status   struct{ ReadyReplicas int } `json:"status"`
	}
	if err := json.Unmarshal(items[0], &decoded); err != nil {
		t.Fatalf("decoding item: %v", err)
	}
	if decoded.Metadata.Name != "api" || decoded.Status.ReadyReplicas != 2 {
		t.Errorf("decoded = %+v, want name=api readyReplicas=2", decoded)
	}
}

func TestListNamespacedResource_SetsLimitQueryParamWhenPositive(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("limit")
		w.Write([]byte(`{"items": []}`))
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL, nil).ListNamespacedResource(context.Background(), "token", "ns", deploymentsGVR, 200); err != nil {
		t.Fatalf("ListNamespacedResource: %v", err)
	}
	if gotQuery != "200" {
		t.Errorf("limit query param = %q, want %q", gotQuery, "200")
	}
}

func TestListNamespacedResource_OmitsLimitQueryParamWhenZero(t *testing.T) {
	var sawLimit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawLimit = r.URL.Query().Has("limit")
		w.Write([]byte(`{"items": []}`))
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL, nil).ListNamespacedResource(context.Background(), "token", "ns", deploymentsGVR, 0); err != nil {
		t.Fatalf("ListNamespacedResource: %v", err)
	}
	if sawLimit {
		t.Error("limit query param present, want absent when limit is 0")
	}
}

func TestListNamespacedResource_ReturnsErrForbiddenOn403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, nil).ListNamespacedResource(context.Background(), "token", "ns", deploymentsGVR, 0)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
}

func TestGetNamespacedResource_RequestsExactlyTheExpectedPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"metadata": {"name": "api"}}`))
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL, nil).GetNamespacedResource(context.Background(), "token", "tenant-a-dev", "api", deploymentsGVR); err != nil {
		t.Fatalf("GetNamespacedResource: %v", err)
	}
	if want := "/apis/apps/v1/namespaces/tenant-a-dev/deployments/api"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

func TestGetNamespacedResource_ReturnsErrUnauthorizedOn401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, nil).GetNamespacedResource(context.Background(), "token", "ns", "api", deploymentsGVR)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("err = %v, want ErrUnauthorized", err)
	}
}

func TestGetNamespacedResource_ReturnsErrorOnMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{not json`))
	}))
	defer srv.Close()

	// GetNamespacedResource itself doesn't decode - it returns the raw body
	// for the caller to decode - so this just proves the raw bytes round-trip
	// unmodified; decoding errors are the caller's to surface.
	raw, err := NewClient(srv.URL, nil).GetNamespacedResource(context.Background(), "token", "ns", "api", deploymentsGVR)
	if err != nil {
		t.Fatalf("GetNamespacedResource: %v", err)
	}
	if string(raw) != "{not json" {
		t.Errorf("raw = %q, want the exact upstream bytes", raw)
	}
}

func TestListNamespacedResource_ReturnsErrorOnMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{not json`))
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL, nil).ListNamespacedResource(context.Background(), "token", "ns", deploymentsGVR, 0); err == nil {
		t.Fatal("ListNamespacedResource() succeeded on malformed JSON, want error")
	}
}

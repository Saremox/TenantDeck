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

// These exercise the generic list/get handler factory in resources.go.
// deployments stands in for every resource built on the same factory
// (statefulsets, daemonsets, jobs, cronjobs, services, ingresses, pvcs) -
// their own tests only need to check their summarize function and that
// they're wired with the right GVR, not re-prove this shared behavior.

func requestWithSessionPath(t *testing.T, rec session.Record, method, path string, pathValues map[string]string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	ctx := context.WithValue(req.Context(), sessionContextKey{}, rec)
	req = req.WithContext(ctx)
	for k, v := range pathValues {
		req.SetPathValue(k, v)
	}
	return req
}

func TestListResourceHandler_ReturnsSummarizedItemsOnSuccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items": [{"metadata": {"name": "api"}, "status": {"readyReplicas": 2}}]}`))
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet, "/api/namespaces/ns/deployments", map[string]string{"ns": "ns"})
	listResourceHandler(client, deploymentsGVR, summarizeDeployment).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q, want %d", rec.Code, rec.Body.String(), http.StatusOK)
	}
	var resp struct {
		Items []deploymentSummary `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Name != "api" || resp.Items[0].ReadyReplicas != 2 {
		t.Errorf("Items = %+v, want one deployment named api with readyReplicas=2", resp.Items)
	}
}

func TestListResourceHandler_RejectsInvalidNamespace(t *testing.T) {
	client := capsule.NewClient("http://unused.invalid", nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet, "/api/namespaces/../deployments", map[string]string{"ns": "../secrets"})
	listResourceHandler(client, deploymentsGVR, summarizeDeployment).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestListResourceHandler_RejectsRequestWithNoSessionInContext(t *testing.T) {
	client := capsule.NewClient("http://unused.invalid", nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/namespaces/ns/deployments", nil)
	req.SetPathValue("ns", "ns")
	listResourceHandler(client, deploymentsGVR, summarizeDeployment).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestListResourceHandler_MapsUpstreamForbiddenTo403(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet, "/api/namespaces/ns/deployments", map[string]string{"ns": "ns"})
	listResourceHandler(client, deploymentsGVR, summarizeDeployment).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestListResourceHandler_MapsSummarizeErrorTo502(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items": ["not an object"]}`))
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet, "/api/namespaces/ns/deployments", map[string]string{"ns": "ns"})
	listResourceHandler(client, deploymentsGVR, summarizeDeployment).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
}

func TestGetResourceHandler_ReturnsSummarizedItemOnSuccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"metadata": {"name": "api"}, "status": {"readyReplicas": 3}}`))
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet, "/api/namespaces/ns/deployments/api", map[string]string{"ns": "ns", "name": "api"})
	getResourceHandler(client, deploymentsGVR, summarizeDeployment).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q, want %d", rec.Code, rec.Body.String(), http.StatusOK)
	}
	var got deploymentSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if got.Name != "api" || got.ReadyReplicas != 3 {
		t.Errorf("got = %+v, want name=api readyReplicas=3", got)
	}
}

func TestGetResourceHandler_RejectsInvalidResourceName(t *testing.T) {
	client := capsule.NewClient("http://unused.invalid", nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet, "/api/namespaces/ns/deployments/..", map[string]string{"ns": "ns", "name": ".."})
	getResourceHandler(client, deploymentsGVR, summarizeDeployment).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetResourceHandler_MapsSummarizeErrorTo502(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`"not an object"`))
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet, "/api/namespaces/ns/deployments/api", map[string]string{"ns": "ns", "name": "api"})
	getResourceHandler(client, deploymentsGVR, summarizeDeployment).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
}

func TestGetResourceHandler_MapsUpstreamUnauthorizedTo401(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet, "/api/namespaces/ns/deployments/api", map[string]string{"ns": "ns", "name": "api"})
	getResourceHandler(client, deploymentsGVR, summarizeDeployment).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

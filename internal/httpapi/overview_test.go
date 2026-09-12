package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Saremox/TenantDeck/internal/capsule"
	"github.com/Saremox/TenantDeck/internal/session"
)

func TestSummarizeResourceQuota_ParsesHardAndUsed(t *testing.T) {
	raw := []byte(`{
		"metadata": {"name": "compute-quota"},
		"status": {"hard": {"cpu": "4", "memory": "8Gi"}, "used": {"cpu": "1", "memory": "2Gi"}}
	}`)
	got, err := summarizeResourceQuota(raw)
	if err != nil {
		t.Fatalf("summarizeResourceQuota: %v", err)
	}
	if got.Name != "compute-quota" || got.Hard["cpu"] != "4" || got.Used["memory"] != "2Gi" {
		t.Errorf("summarizeResourceQuota() = %+v", got)
	}
}

func TestSummarizeResourceQuota_ErrorsOnMalformedJSON(t *testing.T) {
	if _, err := summarizeResourceQuota([]byte(`not json`)); err == nil {
		t.Fatal("summarizeResourceQuota() succeeded on malformed JSON, want error")
	}
}

func TestSummarizeLimitRange_ParsesLimitItems(t *testing.T) {
	raw := []byte(`{
		"metadata": {"name": "defaults"},
		"spec": {"limits": [{"type": "Container", "default": {"cpu": "500m"}, "defaultRequest": {"cpu": "250m"}}]}
	}`)
	got, err := summarizeLimitRange(raw)
	if err != nil {
		t.Fatalf("summarizeLimitRange: %v", err)
	}
	if got.Name != "defaults" || len(got.Limits) != 1 || got.Limits[0].Type != "Container" {
		t.Errorf("summarizeLimitRange() = %+v", got)
	}
	if got.Limits[0].Default["cpu"] != "500m" {
		t.Errorf("Default = %v", got.Limits[0].Default)
	}
}

func TestSummarizeLimitRange_ErrorsOnMalformedJSON(t *testing.T) {
	if _, err := summarizeLimitRange([]byte(`not json`)); err == nil {
		t.Fatal("summarizeLimitRange() succeeded on malformed JSON, want error")
	}
}

func TestOverviewHandler_CombinesQuotasAndLimitsOnSuccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/namespaces/ns/resourcequotas":
			w.Write([]byte(`{"items": [{"metadata": {"name": "compute"}, "status": {"hard": {"cpu": "4"}}}]}`))
		case r.URL.Path == "/api/v1/namespaces/ns/limitranges":
			w.Write([]byte(`{"items": [{"metadata": {"name": "defaults"}, "spec": {"limits": []}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet, "/api/namespaces/ns/overview", map[string]string{"ns": "ns"})
	overviewHandler(client).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q, want %d", rec.Code, rec.Body.String(), http.StatusOK)
	}
	var resp overviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(resp.ResourceQuotas) != 1 || resp.ResourceQuotas[0].Name != "compute" {
		t.Errorf("ResourceQuotas = %+v", resp.ResourceQuotas)
	}
	if len(resp.LimitRanges) != 1 || resp.LimitRanges[0].Name != "defaults" {
		t.Errorf("LimitRanges = %+v", resp.LimitRanges)
	}
}

func TestOverviewHandler_RejectsInvalidNamespace(t *testing.T) {
	client := capsule.NewClient("http://unused.invalid", nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet, "/api/namespaces/../overview", map[string]string{"ns": ".."})
	overviewHandler(client).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestOverviewHandler_MapsQuotaFetchFailureToUpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet, "/api/namespaces/ns/overview", map[string]string{"ns": "ns"})
	overviewHandler(client).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// The quota fetch can succeed while the limitranges fetch fails (or vice
// versa) - both must be checked, not just the first.
func TestOverviewHandler_MapsLimitRangeFetchFailureToUpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/namespaces/ns/resourcequotas" {
			w.Write([]byte(`{"items": []}`))
			return
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet, "/api/namespaces/ns/overview", map[string]string{"ns": "ns"})
	overviewHandler(client).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestOverviewHandler_MapsQuotaSummarizeErrorTo502(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/namespaces/ns/resourcequotas" {
			w.Write([]byte(`{"items": ["not an object"]}`))
			return
		}
		w.Write([]byte(`{"items": []}`))
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet, "/api/namespaces/ns/overview", map[string]string{"ns": "ns"})
	overviewHandler(client).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
}

func TestOverviewHandler_MapsLimitRangeSummarizeErrorTo502(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/namespaces/ns/limitranges" {
			w.Write([]byte(`{"items": ["not an object"]}`))
			return
		}
		w.Write([]byte(`{"items": []}`))
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet, "/api/namespaces/ns/overview", map[string]string{"ns": "ns"})
	overviewHandler(client).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
}

func TestOverviewHandler_RejectsRequestWithNoSessionInContext(t *testing.T) {
	client := capsule.NewClient("http://unused.invalid", nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/namespaces/ns/overview", nil)
	req.SetPathValue("ns", "ns")
	overviewHandler(client).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

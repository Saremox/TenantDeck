package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Saremox/TenantDeck/internal/capsule"
	"github.com/Saremox/TenantDeck/internal/session"
)

func TestSummarizeEvent_ParsesFields(t *testing.T) {
	raw := []byte(`{
		"message": "Pulled image \"app:v1\"",
		"reason": "Pulled",
		"type": "Normal",
		"count": 3,
		"lastTimestamp": "2026-09-12T10:00:00Z",
		"involvedObject": {"kind": "Pod", "name": "api-abc123"}
	}`)
	got, err := summarizeEvent(raw)
	if err != nil {
		t.Fatalf("summarizeEvent: %v", err)
	}
	ev := got.(eventSummary)
	if ev.Reason != "Pulled" || ev.Type != "Normal" || ev.Count != 3 {
		t.Errorf("summarizeEvent() = %+v", ev)
	}
	if ev.InvolvedObject != "Pod/api-abc123" {
		t.Errorf("InvolvedObject = %q, want %q", ev.InvolvedObject, "Pod/api-abc123")
	}
}

// The message is attacker-controlled (whatever produced the event, not
// Kubernetes or TenantDeck itself) - summarizeEvent must pass it through
// as a plain string field unmodified, leaving escaping to the frontend's
// rendering layer (Lens 6, tenantdeck-security-review) rather than trying
// to sanitize it here, which would risk mangling legitimate messages.
func TestSummarizeEvent_PassesHostileMessageThroughAsPlainText(t *testing.T) {
	hostile := `<img src=x onerror="alert(1)">`
	raw, err := json.Marshal(map[string]any{"message": hostile})
	if err != nil {
		t.Fatalf("marshaling fixture: %v", err)
	}
	got, err := summarizeEvent(raw)
	if err != nil {
		t.Fatalf("summarizeEvent: %v", err)
	}
	if got.(eventSummary).Message != hostile {
		t.Errorf("Message = %q, want the exact original string unmodified", got.(eventSummary).Message)
	}
}

func TestSummarizeEvent_ErrorsOnMalformedJSON(t *testing.T) {
	if _, err := summarizeEvent([]byte(`not json`)); err == nil {
		t.Fatal("summarizeEvent() succeeded on malformed JSON, want error")
	}
}

func TestEventsHandler_ReturnsSummarizedEventsOnSuccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items": [{"reason": "Scheduled", "type": "Normal"}]}`))
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet, "/api/namespaces/ns/events", map[string]string{"ns": "ns"})
	eventsHandler(client).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q, want %d", rec.Code, rec.Body.String(), http.StatusOK)
	}
	var resp struct {
		Items []eventSummary `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Reason != "Scheduled" {
		t.Errorf("Items = %+v", resp.Items)
	}
}

func TestEventsHandler_RequestsABoundedLimit(t *testing.T) {
	var gotLimit string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLimit = r.URL.Query().Get("limit")
		w.Write([]byte(`{"items": []}`))
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet, "/api/namespaces/ns/events", map[string]string{"ns": "ns"})
	eventsHandler(client).ServeHTTP(rec, req)

	if gotLimit == "" || gotLimit == "0" {
		t.Errorf("limit query param = %q, want a positive bound forwarded upstream", gotLimit)
	}
}

func TestEventsHandler_MapsSummarizeErrorTo502(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items": ["not an object"]}`))
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet, "/api/namespaces/ns/events", map[string]string{"ns": "ns"})
	eventsHandler(client).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
}

func TestEventsHandler_RejectsInvalidNamespace(t *testing.T) {
	client := capsule.NewClient("http://unused.invalid", nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet, "/api/namespaces/../events", map[string]string{"ns": ".."})
	eventsHandler(client).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestEventsHandler_MapsUpstreamForbiddenTo403(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet, "/api/namespaces/ns/events", map[string]string{"ns": "ns"})
	eventsHandler(client).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestEventsHandler_RejectsRequestWithNoSessionInContext(t *testing.T) {
	client := capsule.NewClient("http://unused.invalid", nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/namespaces/ns/events", nil)
	req.SetPathValue("ns", "ns")
	eventsHandler(client).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

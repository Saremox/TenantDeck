package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Saremox/TenantDeck/internal/capsule"
)

var eventsGVR = capsule.GroupVersionResource{APIPrefix: "api/v1", Resource: "events"}

// maxEventsListed bounds the events page size, per
// docs/spec/05-upstream-boundary-and-resilience.md ("bound pagination") -
// a busy namespace can accumulate far more events than any UI should try
// to render at once.
const maxEventsListed = 200

type eventSummary struct {
	Message        string `json:"message"`
	Reason         string `json:"reason"`
	Type           string `json:"type"`
	Count          int32  `json:"count"`
	LastTimestamp  string `json:"lastTimestamp"`
	InvolvedObject string `json:"involvedObject"`
}

func summarizeEvent(raw json.RawMessage) (any, error) {
	var obj struct {
		Message        string `json:"message"`
		Reason         string `json:"reason"`
		Type           string `json:"type"`
		Count          int32  `json:"count"`
		LastTimestamp  string `json:"lastTimestamp"`
		InvolvedObject struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		} `json:"involvedObject"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decoding event: %w", err)
	}
	return eventSummary{
		// Event Message/Reason text is hostile input - it comes from
		// whatever produced the event, not from TenantDeck or Kubernetes
		// itself. It's returned as plain JSON string fields here; the
		// frontend renders it as text, never as HTML (Lens 6,
		// tenantdeck-security-review).
		Message:        obj.Message,
		Reason:         obj.Reason,
		Type:           obj.Type,
		Count:          obj.Count,
		LastTimestamp:  obj.LastTimestamp,
		InvolvedObject: fmt.Sprintf("%s/%s", obj.InvolvedObject.Kind, obj.InvolvedObject.Name),
	}, nil
}

// eventsHandler implements GET /api/namespaces/{ns}/events (docs/route-allowlist.md).
func eventsHandler(client *capsule.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec, ok := sessionFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		namespace := r.PathValue("ns")
		if !validK8sName(namespace) {
			http.Error(w, "invalid namespace", http.StatusBadRequest)
			return
		}

		items, err := client.ListNamespacedResource(r.Context(), rec.IDToken, namespace, eventsGVR, maxEventsListed)
		if err != nil {
			writeUpstreamError(w, r, err)
			return
		}

		summaries := make([]any, 0, len(items))
		for _, raw := range items {
			s, err := summarizeEvent(raw)
			if err != nil {
				slog.ErrorContext(r.Context(), "decoding event", "error", err)
				http.Error(w, "upstream returned an unexpected shape", http.StatusBadGateway)
				return
			}
			summaries = append(summaries, s)
		}
		writeJSONItems(w, summaries)
	})
}

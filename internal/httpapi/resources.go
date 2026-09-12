package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Saremox/TenantDeck/internal/capsule"
)

// summarizeFunc converts one resource's raw upstream JSON into the small
// DTO TenantDeck actually exposes - see docs/route-allowlist.md. Kept
// separate from the upstream fetch itself (internal/capsule) so this
// client stays agnostic of any one resource kind's fields.
type summarizeFunc func(raw json.RawMessage) (any, error)

// listResourceHandler implements the "list" half of a namespaced resource
// route (e.g. GET /api/namespaces/{ns}/deployments). Must run behind
// requireSession - it reads the session from context, not a cookie
// directly.
func listResourceHandler(client *capsule.Client, gvr capsule.GroupVersionResource, summarize summarizeFunc) http.Handler {
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

		items, err := client.ListNamespacedResource(r.Context(), rec.IDToken, namespace, gvr, 0)
		if err != nil {
			writeUpstreamError(w, r, err)
			return
		}

		summaries := make([]any, 0, len(items))
		for _, raw := range items {
			s, err := summarize(raw)
			if err != nil {
				slog.ErrorContext(r.Context(), "decoding upstream resource", "resource", gvr.Resource, "error", err)
				http.Error(w, "upstream returned an unexpected shape", http.StatusBadGateway)
				return
			}
			summaries = append(summaries, s)
		}
		writeJSONItems(w, summaries)
	})
}

// getResourceHandler implements the "detail" half of a namespaced resource
// route (e.g. GET /api/namespaces/{ns}/deployments/{name}).
func getResourceHandler(client *capsule.Client, gvr capsule.GroupVersionResource, summarize summarizeFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec, ok := sessionFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		namespace, name := r.PathValue("ns"), r.PathValue("name")
		if !validK8sName(namespace) || !validK8sName(name) {
			http.Error(w, "invalid namespace or name", http.StatusBadRequest)
			return
		}

		raw, err := client.GetNamespacedResource(r.Context(), rec.IDToken, namespace, name, gvr)
		if err != nil {
			writeUpstreamError(w, r, err)
			return
		}
		summary, err := summarize(raw)
		if err != nil {
			slog.ErrorContext(r.Context(), "decoding upstream resource", "resource", gvr.Resource, "error", err)
			http.Error(w, "upstream returned an unexpected shape", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(summary)
	})
}

// writeJSONItems is the one response shape every list route in this file
// uses: {"items": [...]}. /api/namespaces predates this and keeps its own
// {"namespaces": [...]} shape rather than being changed to match.
func writeJSONItems(w http.ResponseWriter, items []any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Items []any `json:"items"`
	}{Items: items})
}

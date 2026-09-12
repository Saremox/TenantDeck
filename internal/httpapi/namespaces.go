package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Saremox/TenantDeck/internal/capsule"
)

type namespaceDTO struct {
	Name string `json:"name"`
}

type namespacesResponse struct {
	Namespaces []namespaceDTO `json:"namespaces"`
}

// namespacesHandler implements GET /api/namespaces from docs/route-allowlist.md.
// It must run behind requireSession - see router.go - so a session.Record
// is always present in the request context here.
func namespacesHandler(capsuleClient *capsule.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec, ok := sessionFromContext(r.Context())
		if !ok {
			// Defensive only: reaching this handler without a session means
			// it was wired up without requireSession, which is a routing
			// bug, not a caller-triggerable state.
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		namespaces, err := capsuleClient.ListNamespaces(r.Context(), rec.IDToken)
		if err != nil {
			writeUpstreamError(w, r, err)
			return
		}

		resp := namespacesResponse{Namespaces: make([]namespaceDTO, len(namespaces))}
		for i, ns := range namespaces {
			resp.Namespaces[i] = namespaceDTO{Name: ns.Name}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// writeUpstreamError preserves Capsule/Kubernetes authorization outcomes
// (docs/spec/05-upstream-boundary-and-resilience.md: "preserve Kubernetes
// authorization failures") rather than flattening every upstream problem
// into the same response.
func writeUpstreamError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, capsule.ErrUnauthorized):
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	case errors.Is(err, capsule.ErrForbidden):
		http.Error(w, "forbidden", http.StatusForbidden)
	default:
		slog.ErrorContext(r.Context(), "upstream request failed", "error", err)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}
}

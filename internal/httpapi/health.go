package httpapi

import (
	"context"
	"net/http"
	"time"
)

// pingTimeout bounds how long the readiness probe waits on the session
// store - a probe that can hang forever defeats its own purpose under
// docs/spec/06-container-and-kubernetes-deployment.md's "probes" requirement.
const pingTimeout = 2 * time.Second

// livezHandler answers GET /healthz: the process is up and serving HTTP.
// No session store check here - that's what readyz is for; a liveness
// probe that depends on a downstream service causes needless restarts
// when only the store, not the process, is unhealthy.
func livezHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// storePinger is the one method readyzHandler needs from *session.Store -
// kept as an interface so tests can exercise both outcomes without a real
// Redis.
type storePinger interface {
	Ping(ctx context.Context) error
}

// readyzHandler answers GET /readyz: whether the session store is
// reachable right now. Every protected route fails closed on a store
// outage anyway (requireSession) - this just lets Kubernetes stop sending
// traffic to a replica before a request does, rather than after.
func readyzHandler(store storePinger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
		defer cancel()

		if err := store.Ping(ctx); err != nil {
			http.Error(w, "session store unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

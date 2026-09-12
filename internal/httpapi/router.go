// Package httpapi wires the BFF's HTTP routes: OIDC login/callback/session/
// logout (internal/auth) and the read-only API surface from
// docs/route-allowlist.md, behind the security headers and session
// middleware every response needs.
package httpapi

import (
	"net/http"

	"github.com/Saremox/TenantDeck/internal/auth"
	"github.com/Saremox/TenantDeck/internal/capsule"
	"github.com/Saremox/TenantDeck/internal/session"
)

// NewRouter builds the BFF's full route table. Routes not listed here
// don't exist - there is no catch-all API handler to accidentally expose
// something off docs/route-allowlist.md. frontend, if non-nil, serves the
// embedded built frontend for every path this mux doesn't otherwise claim
// (see internal/webassets); it's nil until that's wired up.
func NewRouter(authHandler *auth.Handler, store *session.Store, capsuleClient *capsule.Client, insecure bool, frontend http.Handler) http.Handler {
	mux := http.NewServeMux()

	// Kubernetes probes (docs/spec/06-container-and-kubernetes-deployment.md)
	// - unauthenticated by convention, never session-gated.
	mux.Handle("GET /healthz", livezHandler())
	mux.Handle("GET /readyz", readyzHandler(store))

	mux.HandleFunc("GET /auth/login", authHandler.LoginHandler)
	mux.HandleFunc("GET /auth/callback", authHandler.CallbackHandler)
	mux.HandleFunc("GET /auth/session", authHandler.SessionHandler)
	mux.HandleFunc("POST /auth/logout", authHandler.LogoutHandler)

	mux.Handle("GET /api/namespaces", requireSession(store, insecure, namespacesHandler(capsuleClient)))
	mux.Handle("GET /api/namespaces/{ns}/overview", requireSession(store, insecure, overviewHandler(capsuleClient)))

	mux.Handle("GET /api/namespaces/{ns}/deployments", requireSession(store, insecure, listResourceHandler(capsuleClient, deploymentsGVR, summarizeDeployment)))
	mux.Handle("GET /api/namespaces/{ns}/deployments/{name}", requireSession(store, insecure, getResourceHandler(capsuleClient, deploymentsGVR, summarizeDeployment)))
	mux.Handle("GET /api/namespaces/{ns}/statefulsets", requireSession(store, insecure, listResourceHandler(capsuleClient, statefulSetsGVR, summarizeStatefulSet)))
	mux.Handle("GET /api/namespaces/{ns}/statefulsets/{name}", requireSession(store, insecure, getResourceHandler(capsuleClient, statefulSetsGVR, summarizeStatefulSet)))
	mux.Handle("GET /api/namespaces/{ns}/daemonsets", requireSession(store, insecure, listResourceHandler(capsuleClient, daemonSetsGVR, summarizeDaemonSet)))
	mux.Handle("GET /api/namespaces/{ns}/daemonsets/{name}", requireSession(store, insecure, getResourceHandler(capsuleClient, daemonSetsGVR, summarizeDaemonSet)))
	mux.Handle("GET /api/namespaces/{ns}/pods", requireSession(store, insecure, listResourceHandler(capsuleClient, podsGVR, summarizePod)))
	mux.Handle("GET /api/namespaces/{ns}/pods/{name}", requireSession(store, insecure, getResourceHandler(capsuleClient, podsGVR, summarizePod)))
	mux.Handle("GET /api/namespaces/{ns}/pods/{name}/logs", requireSession(store, insecure, podLogsHandler(capsuleClient)))
	mux.Handle("GET /api/namespaces/{ns}/jobs", requireSession(store, insecure, listResourceHandler(capsuleClient, jobsGVR, summarizeJob)))
	mux.Handle("GET /api/namespaces/{ns}/jobs/{name}", requireSession(store, insecure, getResourceHandler(capsuleClient, jobsGVR, summarizeJob)))
	mux.Handle("GET /api/namespaces/{ns}/cronjobs", requireSession(store, insecure, listResourceHandler(capsuleClient, cronJobsGVR, summarizeCronJob)))
	mux.Handle("GET /api/namespaces/{ns}/cronjobs/{name}", requireSession(store, insecure, getResourceHandler(capsuleClient, cronJobsGVR, summarizeCronJob)))

	mux.Handle("GET /api/namespaces/{ns}/services", requireSession(store, insecure, listResourceHandler(capsuleClient, servicesGVR, summarizeService)))
	mux.Handle("GET /api/namespaces/{ns}/services/{name}", requireSession(store, insecure, getResourceHandler(capsuleClient, servicesGVR, summarizeService)))
	mux.Handle("GET /api/namespaces/{ns}/ingresses", requireSession(store, insecure, listResourceHandler(capsuleClient, ingressesGVR, summarizeIngress)))
	mux.Handle("GET /api/namespaces/{ns}/ingresses/{name}", requireSession(store, insecure, getResourceHandler(capsuleClient, ingressesGVR, summarizeIngress)))
	mux.Handle("GET /api/namespaces/{ns}/persistentvolumeclaims", requireSession(store, insecure, listResourceHandler(capsuleClient, pvcGVR, summarizePersistentVolumeClaim)))
	mux.Handle("GET /api/namespaces/{ns}/persistentvolumeclaims/{name}", requireSession(store, insecure, getResourceHandler(capsuleClient, pvcGVR, summarizePersistentVolumeClaim)))

	mux.Handle("GET /api/namespaces/{ns}/events", requireSession(store, insecure, eventsHandler(capsuleClient)))

	if frontend != nil {
		mux.Handle("/", frontend)
	}

	return securityHeaders(insecure)(mux)
}

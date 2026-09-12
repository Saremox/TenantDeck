# Route allowlist

**Status: implemented as of Phase 3.** This is the concrete allowlist
`docs/spec/10-process-and-definition-of-done.md` step 1 calls for: every
BFF route, what it's allowed to do upstream, and nothing else. The router
must reject anything not on this list before it reaches the upstream
client — see `docs/spec/05-upstream-boundary-and-resilience.md`. Update
this file in the same change that adds or changes a route;
`tenantdeck-security-review` should treat a route not listed here as a
finding.

Every upstream entry below is namespace-scoped and **read-only** (`get`/
`list`/`watch` where noted) unless explicitly marked otherwise. Every route
in this file is wired in `internal/httpapi/router.go` and backed by real
handlers/tests as of Phase 3 (`internal/httpapi`, `internal/capsule`) — see
`docs/implementation-plan.md` Phase 3 and `docs/security-test-matrix.md`
for the test evidence. It has **not** been exercised against a real Capsule
Proxy (no Docker/Kubernetes in this session's environment — see
"Known blockers" in `docs/implementation-plan.md`).

## Session and auth (no Kubernetes upstream call)

| BFF route | Method | Purpose | Upstream call |
|---|---|---|---|
| `/auth/login` | GET | Start Authorization Code + PKCE flow, redirect to IdP | none (IdP authorization endpoint) |
| `/auth/callback` | GET | Exchange code, validate tokens, create session | none (IdP token/JWKS endpoints) |
| `/auth/logout` | POST | Delete server-side session (CSRF-protected, exact `Origin` check) | none |
| `/auth/session` | GET | Report current login state to the frontend | none (reads local session store only) |

## Namespace discovery

| BFF route | Method | Upstream call | Verb | Notes |
|---|---|---|---|---|
| `/api/namespaces` | GET | Capsule Proxy `core/v1 namespaces` (list) | list | Capsule Proxy itself filters to the caller's tenant; the BFF does not compute this list. |

## Namespace overview (quotas/limits)

| BFF route | Method | Upstream call | Verb | Notes |
|---|---|---|---|---|
| `/api/namespaces/{ns}/overview` | GET | `core/v1 namespaces/{ns}/resourcequotas` (list), `core/v1 namespaces/{ns}/limitranges` (list) | list | Aggregates both into one response; workload health counts are derived from the workload list calls below, not a separate upstream call. |

## Workloads (list + detail)

| BFF route | Method | Upstream call | Verb | Notes |
|---|---|---|---|---|
| `/api/namespaces/{ns}/deployments` | GET | `apps/v1 deployments` (list) | list | |
| `/api/namespaces/{ns}/deployments/{name}` | GET | `apps/v1 deployments/{name}` (get) | get | |
| `/api/namespaces/{ns}/statefulsets` | GET | `apps/v1 statefulsets` (list) | list | |
| `/api/namespaces/{ns}/statefulsets/{name}` | GET | `apps/v1 statefulsets/{name}` (get) | get | |
| `/api/namespaces/{ns}/daemonsets` | GET | `apps/v1 daemonsets` (list) | list | |
| `/api/namespaces/{ns}/daemonsets/{name}` | GET | `apps/v1 daemonsets/{name}` (get) | get | |
| `/api/namespaces/{ns}/pods` | GET | `core/v1 pods` (list) | list | |
| `/api/namespaces/{ns}/pods/{name}` | GET | `core/v1 pods/{name}` (get) | get | |
| `/api/namespaces/{ns}/pods/{name}/logs` | GET | `core/v1 pods/{name}/log` (get, streamed) | get | Only allowlisted subresource. Bounded `tailLines`/`limitBytes`/`sinceSeconds`; `follow=true` permitted with server-side cancellation on client disconnect — see `docs/spec/05-upstream-boundary-and-resilience.md`. |
| `/api/namespaces/{ns}/jobs` | GET | `batch/v1 jobs` (list) | list | |
| `/api/namespaces/{ns}/jobs/{name}` | GET | `batch/v1 jobs/{name}` (get) | get | |
| `/api/namespaces/{ns}/cronjobs` | GET | `batch/v1 cronjobs` (list) | list | |
| `/api/namespaces/{ns}/cronjobs/{name}` | GET | `batch/v1 cronjobs/{name}` (get) | get | |

## Services, Ingresses, PVC status

| BFF route | Method | Upstream call | Verb | Notes |
|---|---|---|---|---|
| `/api/namespaces/{ns}/services` | GET | `core/v1 services` (list) | list | |
| `/api/namespaces/{ns}/services/{name}` | GET | `core/v1 services/{name}` (get) | get | |
| `/api/namespaces/{ns}/ingresses` | GET | `networking.k8s.io/v1 ingresses` (list) | list | Any ingress-sourced host/URL rendered by the frontend goes through the URL-scheme validation in Lens 6 of `tenantdeck-security-review`. |
| `/api/namespaces/{ns}/ingresses/{name}` | GET | `networking.k8s.io/v1 ingresses/{name}` (get) | get | |
| `/api/namespaces/{ns}/persistentvolumeclaims` | GET | `core/v1 persistentvolumeclaims` (list) | list | Frontend renders status only (bound/pending/capacity) — no provisioning/resize UI, even though the full object is returned. |
| `/api/namespaces/{ns}/persistentvolumeclaims/{name}` | GET | `core/v1 persistentvolumeclaims/{name}` (get) | get | |

## Events

| BFF route | Method | Upstream call | Verb | Notes |
|---|---|---|---|---|
| `/api/namespaces/{ns}/events` | GET | `core/v1 events` (list) | list | Bounded pagination; event *message* text is hostile input (Lens 6). |

## Explicitly excluded — never allowlisted

Per `docs/spec/02-product-and-scope.md` and `docs/spec/05-upstream-boundary-and-resilience.md`.
These must be rejected by the router before reaching the upstream client,
not merely absent from the UI — enforced even for a caller with
Kubernetes RBAC broad enough to perform them directly:

- Any write verb: `create`, `update`, `patch`, `delete`, `deletecollection`.
- `secrets` and `configmaps` content (any verb) — not a TenantDeck resource,
  ever.
- Subresources: `exec`, `attach`, `portforward`, `proxy` (on pods or
  services), and any subresource not explicitly listed above (e.g.
  `scale`, `status` writes).
- Cluster-scoped resources not covered above: `nodes`, `persistentvolumes`,
  `storageclasses`, `ingressclasses`.
- RBAC objects: `roles`, `rolebindings`, `clusterroles`,
  `clusterrolebindings` — read or write.
- Capsule's own CRDs (e.g. `Tenant`) — namespace discovery goes through
  Capsule Proxy's existing namespace-listing behavior, not by TenantDeck
  querying Capsule's CRDs directly.
- Any YAML/raw-manifest view or edit endpoint.

## How this maps to tests

Every row above should eventually have: a positive test (the route returns
the right data for the right tenant) and a negative test (a disallowed
verb/subresource/resource on the same path is rejected before the upstream
client is invoked). Track both in `docs/security-test-matrix.md`.

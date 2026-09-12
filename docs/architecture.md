# Architecture

**Status: written against the system as built through Phase 4.** This
describes what actually exists and has been run, not the aspirational
shape in [`docs/spec/03-architecture.md`](spec/03-architecture.md) (still
the normative spec for anything this file doesn't cover or contradicts -
deviations are called out explicitly below).

## System diagram and request path

```
Browser  --same-origin-->  TenantDeck BFF (cmd/tenantdeck)  -->  Capsule Proxy  -->  kube-apiserver
                                  |
                                  v
                          Valkey/Redis (session store)
```

One browser origin, always: `cmd/tenantdeck` serves both the built React
frontend (`internal/webassets`, embedding `web/dist` via `web/embed.go`'s
`//go:embed`) and the JSON API on the same `http.ServeMux`
(`internal/httpapi/router.go`). There is no separate frontend deployment
or CDN to keep in sync.

Inside the BFF binary (`cmd/tenantdeck/main.go` wires these together):

- `internal/config` — loads and validates environment variables. Fails
  closed: a missing required value is a startup error, never a silently
  applied default (e.g. there is no default `TENANTDECK_CAPSULE_PROXY_URL`).
- `internal/session` — the session store. `Store` (store.go) handles
  sealing/unsealing records with AES-256-GCM (`encryptor`) before/after
  they touch the `backend` interface; `RedisBackend` is the only
  implementation that ships (Redis/Valkey are wire-compatible, same
  client). `Store.Ping` backs the `/readyz` probe.
- `internal/auth` — the OIDC relying-party side (`zitadel/oidc/v3`,
  RP-only, TenantDeck is never an OP), login/callback/session/logout
  handlers, server-side login-transaction state (not the library's own
  cookie handler — ADR-008).
- `internal/capsule` — the upstream HTTP client to Capsule Proxy. Every
  call builds its own request from an allowlisted path/verb
  (`GroupVersionResource`) and the caller's own ID token as
  `Authorization: Bearer` — it never forwards an incoming request's
  headers. Two `http.Client`s: one with a wall-clock `Timeout` for bounded
  list/get calls, a second with none for `StreamPodLogs` (bounded instead
  by `context.WithTimeout` + `io.LimitReader` at the `internal/httpapi`
  layer — see ADR in `docs/implementation-plan.md` on why a shared
  timeout would silently cut a `follow=true` log stream).
- `internal/httpapi` — the router (`docs/route-allowlist.md` made
  concrete), `securityHeaders`/`requireSession` middleware, and one
  handler per allowlisted route. `/healthz` and `/readyz` are the only
  unauthenticated routes; every `/api/*` route runs behind
  `requireSession`, which fails closed on any problem (missing cookie,
  unknown session, store unreachable) to the same 401.

## Identity and audience configuration actually used

`internal/auth/oidc.go` constructs the RP client with
`rp.NewRelyingPartyOIDC(ctx, issuerURL, clientID, clientSecret,
redirectURL, scopes, ...)` and `rp.WithVerifierOpts(rp.WithIssuedAtOffset(5s),
rp.WithNonce(expectedNonceFromContext))`. The audience check is the
library's own OIDC Core validation against `clientID` — TenantDeck does
not separately re-derive or widen the accepted audience. Signing
algorithms accepted are exactly what the IdP's own discovery document
advertises (`id_token_signing_alg_values_supported`), not a hardcoded
list — `cmd/mockoidc` and any real IdP both drive this through the same
discovery path. No multi-audience/`azp` handling exists: this is a
single-client-ID deployment, and nothing here has needed more yet.

The nonce is the one place the library's default behavior was
deliberately not used as-is (ADR-003 in `docs/implementation-plan.md`):
`rp.WithNonce` defaults to expecting an empty nonce unless wired through
explicitly, so `internal/auth` threads the expected nonce through a
request-context value (`expectedNonceContextKey`) set from the
server-side login transaction, rather than trusting a client-suppliable
value.

## Session lifecycle

A session (`session.Record`) holds: `Subject`, `IDToken` (the raw ID
token string — this is what gets replayed to Capsule Proxy on every
upstream call), `ExpiresAt`/`CreatedAt`, and a `CSRFToken`. The whole
record is JSON-marshaled, then sealed with AES-256-GCM
(`internal/session/crypto.go`'s `encryptor`) before being written to
Redis under `session:<id>` — compromising Redis alone does not hand over
session contents, the encryption key is supplied separately
(`TENANTDECK_SESSION_ENCRYPTION_KEY`, never derived from or stored
alongside the Redis connection itself). The key is a 32-byte value,
base64-encoded in the environment; see `docs/operations.md` for
provisioning and rotation.

Expiry is absolute only, tied to the ID token's own `exp` claim
(`session.CreateSession`'s `ttl` argument, computed by `internal/auth`
from the token it just validated) — there is no separate idle timeout,
and refresh-token handling is not implemented at all (see
`docs/implementation-plan.md` "Known blockers"; `session.Record` has no
field for one). The browser only ever holds an opaque session ID in a
`__Host-`-prefixed, `HttpOnly`, `Secure` (unless `TENANTDECK_INSECURE`),
`SameSite=Strict` cookie (`auth.SessionCookieName`) — it never sees the ID
token, access token, or encryption key.

## Deployment topology

`charts/tenantdeck` renders: a `Deployment` (configurable replica count,
`RollingUpdate` with `maxUnavailable: 0`), a `ClusterIP` `Service`, a
dedicated `ServiceAccount` with `automountServiceAccountToken: false`
hardcoded (not a values.yaml toggle — see `templates/serviceaccount.yaml`'s
own comment on why), a default-on `NetworkPolicy` with no default-allow
egress, and an optional `Ingress`. No `Role`/`ClusterRole`/`RoleBinding`/
`ClusterRoleBinding` exists anywhere in the chart — TenantDeck has no
Kubernetes API permissions of its own and loads no in-cluster credentials
(`cmd/tenantdeck` never imports `client-go` or any Kubernetes API client;
its only "upstream" is the plain `net/http` client in `internal/capsule`
talking to Capsule Proxy's HTTP(S) endpoint). Capsule's own operator and
capsule-proxy are separate Helm releases in their own namespace
(`capsule-system` in `e2e/up.sh`), installed and operated independently of
TenantDeck's own identity.

Valkey/Redis is never bundled — the chart only ever references an
existing `addr`/Secret (`values.yaml`'s `config.session.redis.*`); the E2E
stack's `e2e/manifests/valkey.yaml` is test infrastructure provisioning
its own disposable store, not a shape the production chart offers.

## Deliberate deviations from the spec

None identified yet beyond what's already recorded as ADRs in
`docs/implementation-plan.md` (e.g. ADR-008's server-side login
transactions instead of the OIDC library's cookie handler, ADR-009's
CSRF-token-in-JSON-body instead of a cookie). Nothing in Phase 4 diverged
from `docs/spec/06-container-and-kubernetes-deployment.md`'s chart
requirements or `docs/spec/03-architecture.md`'s request path.

# Mandatory automated testing

> Part of the [TenantDeck kickoff specification](README.md).

Create one documented command, e.g. `make verify`, for fast deterministic
checks, and `make e2e` for a disposable full-stack suite. CI must call the
same underlying commands.

## Unit/integration coverage

Must include OIDC validation failures, callback replay, CSRF, cookie flags,
unsafe redirects, malformed paths, header spoofing, forbidden
resource/subresource access, session fixation, expiry, refresh races across
instances, logout-refresh races, store failure, stream cancellation/limits,
token redaction, and no cross-user data leakage. Include table-driven and
fuzz tests where useful. Do not pursue a coverage percentage instead of
testing security boundaries.

## Mandatory E2E stack

1. Disposable kind cluster with explicit kubeconfig and compatible pinned Kubernetes version.
2. Real Capsule operator and Capsule Proxy using pinned supported charts/images.
3. Deterministic mock OIDC provider implementing discovery, JWKS, authorization-code flow, PKCE enforcement and refresh with real signed JWTs. Use a maintained mock if suitable; otherwise isolate minimal test-only code. Never compile a login bypass or mock identity mode into the production app.
4. Test-only CA and TLS configuration with matching issuer and trusted certificates across the HTTP test client, BFF and kube-apiserver. No production TLS verification bypass. Arrange issuer DNS/network reachability deliberately.
5. Valkey/Redis, the actual built TenantDeck image deployed with the actual chart, and at least two TenantDeck replicas for session/refresh tests.
6. Two independent tenants with multiple namespaces, distinct identifiable fixtures, a user in each tenant, and a valid authenticated user without tenant access. Test direct upstream permissions to prove fixtures are wired correctly before testing the BFF.

E2E must perform HTTP login redirects and callback exchange with a cookie jar,
then call TenantDeck APIs using only that session cookie. Do not insert a
session directly into the store or mint a token and call that an end-to-end
login test. UI browser automation is optional; if using only an HTTP client,
separately verify cookie attributes and document that browser
SameSite/CSP enforcement is not exercised.

## Required E2E assertions

- Successful login and namespace/workload/log/quota reads for the correct tenant; logout followed by denied reuse of the old cookie.
- Tenant A cannot retrieve Tenant B's named resources, list contents or stream logs, including direct URL manipulation; reverse direction too. Forbidden/not-found responses contain no other tenant's data.
- The non-tenant user sees no tenant resources. Unauthenticated and expired sessions cannot access protected endpoints.
- Invalid signature/issuer/audience/nonce/state and replayed authorization codes fail; refresh success/failure and expiry paths are exercised without long sleeps.
- Tenant owner privileges do not bypass TenantDeck's read-only allowlist. Writes, Secrets, impersonation headers, traversal, and excluded subresources are denied before reaching the upstream where appropriate.
- Tokens never appear in browser-visible responses, cookies, Location headers or captured application logs. Logout invalidates sessions across replicas.
- Runtime Pod/SA configuration is hardened, no SA token volume is mounted, and ordinary app functions work with read-only root filesystem and no runtime API RBAC grants. NetworkPolicy uses a CNI that actually enforces it; test allowed and disallowed connections. Do not mistake a rendered policy on a non-enforcing CNI for enforcement testing.
- Failure diagnostics are redacted; teardown occurs on success and failure, scoped only to the test resources.

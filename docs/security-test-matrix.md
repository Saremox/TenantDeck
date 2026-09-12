# Security test matrix

Living map from requirements in [`docs/spec/`](spec/README.md) to the tests
that actually verify them, with evidence locations (test file + name, or CI
run). Do not mark a row "done" without a real passing test backing it —
see `docs/spec/01-working-agreement.md` on not fabricating results. Status
is one of `missing`, `partial` (some real aspect of the requirement is
tested, but not the whole thing — say exactly what's left), `in progress`,
or `covered`.

**Status: Phase 2 vertical slice in progress.** Unit/integration tests now
back the rows below that the vertical slice (OIDC login, session, logout,
namespace list) actually touches; everything else (remaining v1 features,
the chart, CI, the mandatory multi-tenant/multi-replica E2E suite) is still
`missing` because it hasn't been built yet - see `docs/implementation-plan.md`.

| # | Requirement | Source | Test(s) | Evidence | Status |
|---|---|---|---|---|---|
| 1 | OIDC validation failures (signature/issuer/audience/nonce/state) | spec/04, spec/07 | `internal/auth`: `TestCallbackHandler_RejectsNonceMismatch`, `TestCallbackHandler_RejectsAlreadyExpiredToken`, `TestCallbackHandler_RejectsStateQueryMismatchingCookie`, `TestCallbackHandler_RejectsMissingStateCookie` | `go test ./internal/auth/...` (2026-09-12) | covered |
| 2 | Authorization code replay rejected | spec/07 | `internal/auth`: `TestCallbackHandler_RejectsReplayedState`; `internal/session`: `TestStore_TakeLoginTransactionIsSingleUse` | `go test ./internal/auth/... ./internal/session/...` | covered |
| 3 | CSRF protection + exact Origin validation on state-changing endpoints | spec/04 | `internal/auth`: `TestLogoutHandler_RejectsMissingOrigin`, `TestLogoutHandler_RejectsMismatchedOrigin`, `TestLogoutHandler_RejectsMissingCSRFToken`, `TestLogoutHandler_RejectsWrongCSRFToken` | `go test ./internal/auth/...` | covered |
| 4 | Cookie flags (Secure/HttpOnly/host-only/Path=/, `__Host-` prefix, SameSite) | spec/04 | `internal/auth`: `TestLoginHandler_SetsHostPrefixedSecureCookieWhenNotInsecure` | `go test ./internal/auth/...` | covered |
| 5 | Open-redirect / unsafe post-login destination rejected | spec/04 | N/A yet - `postLoginRedirect` is a hardcoded `"/"`, no request-derived redirect exists to test | - | missing (not yet applicable - no configurable redirect target exists) |
| 6 | Malformed/traversal paths and encoded separators rejected by upstream boundary | spec/05 | Only one route (`/api/namespaces`) exists with no path parameters yet; nothing to test this against until workload routes with `{name}`/`{ns}` params are built (Phase 3) | - | missing |
| 7 | Forwarding-header spoofing (Authorization/Cookie/Impersonate-*/X-Remote-*) stripped | spec/05 | `internal/capsule`: `TestListNamespaces_SendsOnlyTheBearerIDTokenAsAuthorization`; `internal/httpapi`: `TestNamespacesHandler_UsesTheSessionsIDTokenAsBearerCredential` | `go test ./internal/capsule/... ./internal/httpapi/...` | covered |
| 8 | Forbidden resource/subresource/verb access denied (read-only allowlist, incl. for a tenant owner) | spec/02, spec/05 | Only list-namespaces exists so far, which is itself read-only by construction (no write path in the client at all); no test yet proves a tenant owner's broader RBAC can't reach a write through TenantDeck, because no route accepts any verb but GET | - | missing |
| 9 | Session fixation protection | spec/04 | Partially: `internal/auth`: `TestCallbackHandler_CreatesSessionOnValidCallback` + `internal/session`: `TestStore_CreateSessionGeneratesDistinctIDs` show a session ID is only minted post-verification and is never client-supplied; no test yet tries to pre-seed a cookie before login and confirms it's ignored | `go test ./internal/auth/... ./internal/session/...` | partial |
| 10 | Session idle/absolute expiry enforced | spec/04 | `internal/session`: `TestRedisBackend_KeyExpiresAfterTTL` (absolute TTL tied to ID token expiry); no separate *idle* timeout exists yet - sessions only expire absolutely | `go test ./internal/session/...` | partial |
| 11 | Refresh races across replicas (atomic store/locking) | spec/04 | Refresh-token handling isn't implemented yet - see `docs/implementation-plan.md` "Known blockers" | - | missing |
| 12 | Logout-refresh race: refresh cannot resurrect a deleted session | spec/04 | Same - no refresh implementation yet | - | missing |
| 13 | Session store failure → fail closed (no fallback auth) | spec/04 | `internal/session`: `TestStore_GetSessionReturnsErrorWhenStoreUnavailable`; `internal/auth`: `TestLoginHandler_FailsClosedWhenStoreUnavailable`; `internal/httpapi`: `TestRequireSession_RejectsWhenStoreUnavailable` | `go test ./internal/session/... ./internal/auth/... ./internal/httpapi/...` | covered |
| 14 | Stream cancellation terminates upstream work; log/stream limits enforced | spec/05 | No streaming endpoint (pod logs) exists yet (Phase 3); `internal/capsule`'s `TestListNamespaces_RespectsContextCancellation` and `TestListNamespaces_ReturnsErrorWhenResponseExceedsSizeLimit` cover the same *pattern* for the one upstream call that exists | `go test ./internal/capsule/...` | partial |
| 15 | Token material never appears in responses/cookies/Location/logs (redaction) | spec/04, spec/07 | Not tested yet - `slog.ErrorContext` calls in `internal/auth`/`internal/httpapi` log `err.Error()` values, which don't currently include token material, but nothing asserts this holds as the code grows | - | missing |
| 16 | No cross-user / cross-tenant data leakage (identity contamination under concurrency) | spec/05, spec/07 | Only one upstream client per request, built fresh from that request's own session - no shared/pooled client carries another session's credential - but no concurrent-access test exists yet | - | missing |
| 17 | Tenant A cannot read/list/stream Tenant B's resources (incl. direct URL manipulation), both directions | spec/07 | Needs two real tenants against a real Capsule Proxy - this is Phase 4's mandatory E2E stack, not unit-testable against a fake upstream | - | missing |
| 18 | Non-tenant authenticated user sees no tenant resources | spec/07 | Same - needs the real E2E stack | - | missing |
| 19 | Unauthenticated / expired sessions denied on protected endpoints | spec/07 | `internal/httpapi`: `TestRequireSession_RejectsMissingCookie`, `TestRequireSession_RejectsUnknownSessionID` | `go test ./internal/httpapi/...` | covered |
| 20 | Logout invalidates sessions across replicas | spec/04, spec/07 | `internal/auth`: `TestLogoutHandler_OldCookieIsRejectedAfterLogout` proves it on one process/store; "across replicas" specifically needs the 2-replica E2E setup (Phase 4) | `go test ./internal/auth/...` | partial |
| 21 | Pod/SA hardening: no SA token mounted, works under `readOnlyRootFilesystem`, no runtime RBAC grants | spec/06, spec/07 | | | missing |
| 22 | NetworkPolicy enforced by an actually-enforcing CNI (allowed + disallowed connections) | spec/06, spec/07 | | | missing |
| 23 | Chart renders Restricted Pod Security Standard-compliant manifests | spec/06 | | | missing |
| 24 | Two replicas share sessions correctly (Valkey/Redis-backed) | spec/06, spec/07 | | | missing |
| 25 | CI: fork PRs run without secrets/write privileges | spec/08 | | | missing |
| 26 | CI: vulnerability scan policy fails on actionable high/critical findings | spec/08 | | | missing |
| 27 | Release workflow: SBOM/provenance + artifact signing, no publish from PRs | spec/08 | | | missing |
| 28 | CSP present and strict; no third-party scripts/CDNs loaded | spec/04 | `internal/httpapi`: `TestSecurityHeaders_SetsCSPAndAntiFramingHeaders` | `go test ./internal/httpapi/...` | covered |
| 29 | Hostile Kubernetes-sourced content (logs/annotations/labels/events) never rendered as executable HTML | spec/04 | `web/src/NamespaceList.test.tsx`: "renders a hostile namespace name as literal text, not as markup" (namespace names are the only Kubernetes-sourced content rendered so far; logs/annotations/events don't exist yet) | `cd web && npx vitest run` | partial |
| 30 | Rendered links validate URL scheme (no `javascript:`/`data:`) | spec/04 | No rendered links exist yet (ingress hosts aren't built - Phase 3) | - | missing |
| 31 | Anti-framing/content-type/referrer/HSTS headers present; `no-store` on sensitive responses | spec/04 | `internal/httpapi`: `TestSecurityHeaders_SetsCSPAndAntiFramingHeaders`, `TestSecurityHeaders_SetsHSTSWhenNotInsecure`, `TestSecurityHeaders_OmitsHSTSWhenInsecure` | `go test ./internal/httpapi/...` | covered |
| 32 | No service worker caches tenant/session data | spec/04 | No service worker exists in this codebase at all | - | covered (by absence - revisit if one is ever added) |
| 33 | Login/token-exchange attempts rate-limited; limiter state itself bounded | spec/05 | Not implemented yet | - | missing |
| 34 | Each required UI state (empty/loading/forbidden/expired-session/unavailable-upstream) renders distinctly for its condition | spec/02 | `web/src/NamespaceList.test.tsx` covers loading/empty/forbidden/unavailable for namespaces; `web/src/App.test.tsx` covers the logged-out (expired-session-equivalent) state. Per-workload-type empty/loading states don't exist yet (Phase 3) | `cd web && npx vitest run` | partial |
| 35 | Frontend API client maps 401/403/5xx/network failure to the correct UI state | spec/02 | `web/src/NamespaceList.test.tsx`: 403 → forbidden state, 502 → unavailable state | `cd web && npx vitest run` | covered (for the one API call that exists) |
| 36 | Automated accessibility checks (axe-core) pass with zero serious/critical violations on required views | spec/02 | `web/src/App.a11y.test.tsx` (logged-out and logged-in states) | `cd web && npx vitest run` | partial (only App's two states checked; NamespaceList's forbidden/unavailable/loaded states aren't separately axe-checked yet) |
| 37 | Every route in `docs/route-allowlist.md` rejects every verb/subresource/resource not listed for it, before the upstream client runs | route-allowlist | Only `/api/namespaces` (GET) is implemented; it has no wrong-verb/wrong-path test yet because Go's `http.ServeMux` 404s/405s anything else by construction, untested explicitly | - | missing |

Add rows as the spec is refined or new findings surface during the
adversarial review phase (`docs/spec/10-process-and-definition-of-done.md`,
step 5). Do not delete a row just because it's inconvenient — mark it
`missing` and explain the blocker instead.

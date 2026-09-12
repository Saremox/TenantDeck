# Implementation plan

Living progress tracker against the execution sequence in
[`docs/spec/10-process-and-definition-of-done.md`](spec/10-process-and-definition-of-done.md).
Update this file whenever a phase advances — do not let it drift from what is
actually merged.

**Status: Phase 2 (vertical slice) done for what a sandboxed, Docker-less
session could build and verify.** Real OIDC login → server-side session →
namespace list → UI works, with the BFF, session store, upstream client,
router, and a built React frontend all genuinely compiled, unit/integration
tested, and smoke-tested as a running binary (see "Known blockers" for what
that smoke test could and couldn't exercise). This is working software now,
not just decisions — but it has not been validated against a real Capsule
Proxy or a real IdP; see below.

## Phases

- [x] **1. Inspection and planning** — dependency set chosen and recorded as
      ADRs below; [`docs/route-allowlist.md`](route-allowlist.md) written;
      `docs/threat-model.md` assets/trust-boundaries filled in. Per-component
      threat analysis (OIDC login, session handling, upstream client,
      frontend) is deferred to when each component is actually built — you
      can't meaningfully threat-model code that doesn't exist yet.
- [x] **2. Vertical slice** — implemented: `internal/config`, `internal/session`
      (encrypted, Redis-backed), `internal/auth` (OIDC login/callback/session/
      logout), `internal/capsule` (namespace-list upstream client),
      `internal/httpapi` (router + security headers + session middleware),
      a real React/TS/Vite frontend (`web/`) embedded via `go:embed`
      (`web/embed.go`, `internal/webassets`), and `cmd/tenantdeck` wiring it
      all together. `make verify` runs clean (Go tests with `-race`,
      frontend typecheck/lint/test/build). The built binary was run for real
      against a real Redis and a minimal fake OIDC discovery endpoint and
      correctly served the frontend, reported logged-out session status,
      redirected to the IdP with real PKCE/state/nonce, and fail-closed
      401'd `/api/namespaces` without a session — see "Known blockers" for
      what wasn't exercised this way. `docs/local-development.md` is now
      written for real.
- [ ] **3. Remaining bounded v1 features** — overview/quotas, workload
      list/detail views, services/ingresses/PVC status, events, bounded pod
      logs with follow/cancel, UI states — each with tests alongside it.
- [ ] **4. Hardened image/chart + full-stack E2E** — Dockerfile, Helm chart
      per `docs/spec/06-container-and-kubernetes-deployment.md`, the mandatory
      E2E stack per `docs/spec/07-mandatory-automated-testing.md`, then
      GitHub Actions CI/release workflows per
      `docs/spec/08-github-actions-and-supply-chain.md`. Write
      `docs/operations.md` and `docs/architecture.md` for real once the chart
      and E2E stack actually exist and have been run.
- [ ] **5. Adversarial security review** — run the `tenantdeck-security-review`
      skill against `docs/threat-model.md`; fix findings with regression
      tests, not just writeups.
- [ ] **6. Clean-build reconciliation** — re-run `make verify` / `make e2e`
      from a clean checkout; reconcile `docs/security-test-matrix.md` with
      actual evidence before calling anything done. Confirm none of the five
      required docs are still sitting at "Status: not started" — and that
      `SECURITY.md`'s reporting path actually works, not just reads as
      plausible.

## Decisions / ADRs

Researched and recorded 2026-09-12 per
`docs/spec/01-working-agreement.md` ("verify current primary documentation
before choosing dependency versions... never invent an available
version"). **Re-verify before actually pinning these in `go.mod`/`package.json`
if much time has passed** — this list is only as fresh as the date above.

- **ADR-001 — Go toolchain: 1.26.x** (amended from the original 1.27.x
  research). `golang.org/x/oauth2` (pulled in by ADR-003) requires
  `go >= 1.26.0`, and `go.mod` now reads `go 1.26.0`. There was no reason
  to force 1.27.x once 1.26.x was already required by a real dependency.
- **ADR-002 — HTTP stack: standard library `net/http` only, no router
  dependency.** Confirmed as built: `internal/httpapi` uses Go's
  method+wildcard-pattern `http.ServeMux` (`"GET /auth/login"`, etc.) for
  every route in `docs/route-allowlist.md`. No router dependency needed.
- **ADR-003 — OIDC relying-party library: `github.com/zitadel/oidc/v3`**
  (amended from the original "v4" research). At implementation time,
  `zitadel/oidc/v4` only had `-next` pre-release tags (`v4.0.0-next.4`), not
  a stable release — using a pre-release in a security-sensitive auth path
  would have contradicted "prefer mature maintained libraries." `v3.49.6`
  is the real stable latest and is what's in `go.mod`. We use only its RP
  side (`pkg/client/rp`) — TenantDeck is never an OP. One non-obvious but
  important finding from building against it: its ID token verifier's
  nonce check is **opt-in** (`rp.WithNonce`, defaulting to expecting an
  empty nonce) rather than automatic — `internal/auth/oidc.go` wires the
  expected nonce through via a request-context value
  (`expectedNonceContextKey`) rather than hand-rolling a parallel nonce
  check, so the library's own OIDC Core validation is what actually
  enforces it.
- **ADR-004 — Session-store client: `github.com/redis/go-redis/v9`.**
  Confirmed as built and tested against a real `redis-server` process in
  every package's tests (not just a fake), per
  `docs/spec/07-mandatory-automated-testing.md`'s preference for real
  infrastructure.
- **ADR-005 — Frontend tooling: React 19 + TypeScript + Vite 8.** Confirmed
  as built, with two corrections to the original research: `@vitejs/plugin-react`
  needed `^6.0.0` (the `^4.x` line doesn't declare Vite 8 as a supported
  peer), and `vitest` needed `^5.0.0` rather than `^3.0.0` — partly for the
  same Vite 8 compatibility reason, and partly because `npm audit` flagged
  a moderate path-traversal advisory
  ([GHSA-82fw-gwwq-j7x9](https://github.com/advisories/GHSA-82fw-gwwq-j7x9))
  in `@vitest/mocker` versions pulled in by `vitest@2–4.x`; `vitest@5.0.0`
  resolves both. `npm audit` reports zero vulnerabilities against the
  versions actually in `web/package.json` now.
- **ADR-006 — Capsule compatibility set: Capsule operator v0.14.5 +
  capsule-proxy v0.14.1, target Kubernetes 1.36.x.** Unchanged from
  research — **not yet exercised**, since this sandboxed session has no
  Docker daemon (see "Known blockers"). Still the right target; just
  unverified against a real cluster so far.
- **ADR-007 — Disposable E2E cluster: kind v0.33.0**, whose default node
  image is Kubernetes 1.36.1 — matches ADR-006. Same caveat: unverified in
  this session.
- **ADR-008 — Login-transaction binding: server-side state, not the OIDC
  library's cookie handler.** `internal/auth` does its own state/nonce/PKCE
  generation and storage (`session.Store.CreateLoginTransaction`/
  `TakeLoginTransaction`, single-use via Redis `GETDEL`) rather than using
  `rp.AuthURLHandler`/`rp.CodeExchangeHandler`'s built-in `httphelper.CookieHandler`.
  This keeps the "cryptographically random one-use state... bounded
  login-transaction lifetime" properties from
  `docs/spec/04-auth-session-browser-security.md` fully auditable in our
  own code rather than trusting a black-box cookie mechanism, at the cost
  of a bit more code than the library's convenience wrappers would need.
- **ADR-009 — CSRF protection: synchronizer token delivered in the
  `/auth/session` JSON body, never a cookie.** A random `CSRFToken` is
  generated at login, stored in the session record, returned once in that
  same-origin JSON response, and must be echoed back as `X-CSRF-Token` on
  `POST /auth/logout` — alongside an exact `Origin` check, per
  `docs/spec/04-auth-session-browser-security.md`'s "Require CSRF
  protection **and** exact Origin validation."
- **ADR-010 — CSP: `default-src 'self'; object-src 'none'; base-uri 'none';
  frame-ancestors 'none'`**, set on every response by `internal/httpapi`'s
  `securityHeaders` middleware. No `unsafe-inline`/`unsafe-eval`, no
  third-party sources — revisit only if a real UI need (e.g. a font CDN)
  comes up, and then extend deliberately rather than loosening broadly.
- **ADR-011 — Frontend embedding: a sibling `web` Go package
  (`web/embed.go`, `//go:embed dist`), not a path inside `internal/`.**
  Go's `//go:embed` directive can't reference a parent directory (`..`),
  so the embed must live inside the frontend's own directory tree.
  `web/dist/.gitkeep` is committed (real build output is gitignored) so a
  fresh checkout's `go build` has something to embed before `npm run
  build` has ever run.
- **Deferred to Phase 4**: the E2E mock OIDC provider choice
  (`docs/spec/07-mandatory-automated-testing.md` asks for "a maintained
  mock if suitable") isn't decided here — it's an E2E-harness decision,
  not a production dependency, and belongs with the rest of that stack
  when it's actually built.

Record further ADRs here (or as separate files under `docs/adr/` if this
section grows unwieldy) as they're made.

## Known blockers

- **No Docker daemon in this session's execution environment** (confirmed:
  `docker info` fails to reach `/var/run/docker.sock`). This blocks running
  a disposable kind cluster, the real Capsule operator, or a real
  capsule-proxy — the things ADR-006/007 target. As a result:
  - `internal/capsule`'s client is tested against an `httptest.Server`
    standing in for Capsule Proxy's namespace-list response shape, proving
    the client builds the right request and handles the right
    statuses/sizes/timeouts — **not** that a real capsule-proxy accepts
    and answers it the same way.
  - The manual smoke test (recorded in this session) ran the real built
    binary against a real Redis and a minimal fake OIDC discovery endpoint
    (Python stdlib `http.server`, not a full OP) — enough to prove the
    binary boots, wires config/session/router/frontend correctly, and
    `/auth/login` redirects with real PKCE/state/nonce — but it did not
    complete a full login (no real signed token exchange) or reach Capsule
    Proxy at all.
  - The full login/nonce/PKCE/replay/expiry round trip **is** verified for
    real, with real RS256-signed tokens and real discovery/JWKS, by
    `internal/auth`'s test suite (`testop_test.go`'s minimal test-only OP) —
    that part doesn't depend on Docker.
  - **Before trusting this against a real deployment**, Phase 4's mandatory
    E2E stack (real kind + real Capsule + real capsule-proxy + a
    decided-on mock OIDC provider, per `docs/spec/07-mandatory-automated-testing.md`)
    must actually run, in an environment that has container/Kubernetes
    capability. Nothing here should be read as "verified against Capsule."
- **Refresh-token handling is not implemented.** `session.Record` has no
  refresh token field; a session's only expiry is the absolute TTL tied to
  the ID token's own `exp` claim. There is no idle timeout distinct from
  that, and no refresh-race/concurrent-refresh handling
  (`docs/spec/04-auth-session-browser-security.md` requires both) — rows
  10–12 in `docs/security-test-matrix.md` are `partial`/`missing`
  accordingly. This needs its own implementation pass before logout/expiry
  behavior can be called complete.
- **Login/token-exchange rate limiting is not implemented** (matrix row
  33) — `docs/spec/05-upstream-boundary-and-resilience.md`'s "bound...
  login abuse" requirement.
- **`postLoginRedirect` is a hardcoded `"/"`**, not a validated relative
  path from a request parameter — trivially safe (no redirect surface to
  exploit) but also not yet a general "allowlisted post-login destination"
  mechanism. Revisit if/when a real deep-link requirement shows up.

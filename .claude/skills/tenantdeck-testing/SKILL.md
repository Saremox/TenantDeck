---
name: tenantdeck-testing
description: "Use when writing or running tests for TenantDeck: unit/integration tests for security boundaries (OIDC validation, CSRF, sessions, upstream allowlisting), or the mandatory full-stack E2E suite (kind cluster + real Capsule + mock OIDC + Valkey + two replicas). Trigger on \"write tests\", \"add e2e\", \"test this change\", or when a feature from tenantdeck-development is missing its tests. Not for implementing the feature itself (tenantdeck-development) or for adversarial code review (tenantdeck-security-review), though a review finding should usually turn into a test here."
---

# TenantDeck testing

Full requirements: `docs/spec/07-mandatory-automated-testing.md`. Two
commands, both called identically by CI:

- `make verify` — fast, deterministic: fmt/vet/static analysis/race-enabled
  unit+integration tests (Go), lint/typecheck/tests/build (frontend),
  Helm lint/template/schema validation.
- `make e2e` — disposable full-stack suite described below.

**Do not pursue a coverage percentage instead of testing security
boundaries.** A boundary without a test that actually tries to cross it is
not verified, no matter how high coverage reads. That said: **target 100%
line coverage on the BFF** (the Go module — not the frontend, not generated
code). Coverage is a floor reached by meaningful tests, not a goal to pad:
if a line only gets covered by a test with no real assertion, that's not
progress toward the target, and if a line genuinely can't be exercised by a
meaningful test, that's usually a sign it's dead code or an unreachable
branch worth removing rather than a gap to paper over.

Prefer writing the test before or alongside the implementation (TDD),
especially for anything in the required coverage list below — it's the
cheapest way to be sure the boundary is actually being tested, not just
described. This isn't mandatory for every line of glue code, but default to
it for security-relevant logic.

Don't over-test: one well-placed test per behavior is enough. Don't add a
second test (at the same or a different layer) that re-asserts something an
existing test already proves, "just to be safe" — that's cost (slower
suite, more churn on every change) without a corresponding gain. If you're
tempted to add a redundant test, that's usually a sign the existing test is
in the wrong place (e.g. a unit-testable case currently only covered at the
E2E layer) — move it, don't duplicate it.

## Test size and naming

- **One test, one concern.** A test proves one specific behavior or
  boundary. If you're describing a test with "and" ("it logs in and checks
  the cookie and also rejects a bad nonce"), split it.
- No 200-line tests covering multiple aspects. If a test function is
  sprawling, that's a sign it's actually several tests glued together —
  split it into focused tests that can each fail for exactly one reason.
- The test name alone should tell a reader what's being verified —
  `TestLogout_DeletesSessionServerSide`, `TestCallback_RejectsReplayedCode`,
  not `TestLogin` or `TestSession2`. If the name genuinely can't carry the
  scenario (e.g. a subtle edge case), add a 1-2 line comment directly above
  the test stating what it verifies — don't leave the reader to infer it
  from the body. This is the one place a short *what* is welcome; see
  "Comments in tests" below for the *why* rule that still applies to
  everything else in the test.

## Unit/integration coverage (required, not optional)

Every item below needs a real test, table-driven or fuzzed where that fits
the input space better than enumerated cases:

- OIDC validation failures: bad signature, wrong issuer, wrong audience,
  expired token, bad/missing nonce, bad/missing state.
- Authorization-code replay (second exchange of the same code fails).
- CSRF rejected on state-changing endpoints (logout); exact `Origin`
  mismatch rejected.
- Cookie flags asserted directly (Secure/HttpOnly/host-only/Path/SameSite/
  `__Host-` prefix) — don't just assert login "works," assert the actual
  `Set-Cookie` header.
- Unsafe/open-redirect post-login destinations rejected.
- Malformed paths: encoded traversal, encoded separators, unsupported
  subresources — rejected before reaching the upstream client.
- Header spoofing: incoming `Authorization`/`Cookie`/`Impersonate-*`/
  `X-Remote-*`/`X-Forwarded-Client-Cert` must not appear in the outbound
  request — assert on the actual request the upstream client builds, not on
  the response.
- Forbidden resource/subresource/verb combinations denied by the allowlist.
- Session fixation: session ID changes across the login boundary.
- Idle and absolute session expiry enforced.
- Refresh races: concurrent refresh attempts across simulated replicas
  don't double-refresh or corrupt the session (this is about the
  store/locking strategy, not real multi-process concurrency).
- Logout-refresh race: refresh must fail after logout, not resurrect the
  session.
- Session store failure → requests fail closed, not open.
- Stream cancellation actually cancels the upstream request (assert the
  upstream saw cancellation, not just that the client connection closed).
- Log/stream/request size and rate limits enforced.
- Token redaction: serialize whatever the app logs and assert no token
  substring appears.
- No cross-user leakage: two concurrent sessions with different identities
  never observe each other's data (this is the unit-level version of the
  E2E tenant-isolation assertions below).

## Mandatory E2E stack

Build this for `make e2e`, matching
`docs/spec/07-mandatory-automated-testing.md` exactly:

1. Disposable **kind** cluster, explicit kubeconfig, pinned compatible
   Kubernetes version.
2. Real **Capsule operator + Capsule Proxy**, pinned chart/image versions
   (verify current versions — don't guess).
3. **Deterministic mock OIDC provider** — discovery, JWKS, real
   authorization-code flow with PKCE enforcement, refresh, real signed
   JWTs. Prefer a maintained mock over hand-rolled test code. This mock
   lives only in the test harness — never compile a login bypass or mock
   identity mode into the production binary.
4. **Matching test-only CA/TLS** across the HTTP test client, the BFF, and
   kube-apiserver, with the same issuer reachable from all three. No
   production TLS-verification bypass, ever, even in this harness.
5. **Valkey/Redis** + the actual built TenantDeck image deployed via the
   actual Helm chart, with **at least two replicas** — session/refresh
   tests must exercise cross-replica behavior for real.
6. **Two independent tenants**, each with multiple namespaces and distinct,
   identifiable fixtures; one user per tenant; one valid authenticated user
   with no tenant access. Before testing the BFF, test direct upstream
   (Capsule Proxy) permissions for these fixtures — if the fixtures aren't
   wired correctly at that layer, the BFF-level test proves nothing.

**How login must be tested:** real HTTP redirects through the mock IdP and
a real callback exchange, using a cookie jar, then call TenantDeck APIs
using *only* that session cookie. Inserting a session directly into the
store, or minting a token and calling that "E2E," does not satisfy this —
it skips exactly the code path most likely to have a bug. UI browser
automation is optional; if the suite is HTTP-client-only, add separate
explicit assertions on cookie attributes and document in the test file that
browser-enforced SameSite/CSP behavior is not exercised by this suite.

Avoid long `sleep`-based waits for expiry paths — drive time forward or use
short configured TTLs in the test environment instead.

## Required E2E assertions (checklist)

- [ ] Login succeeds; namespace/workload/log/quota reads succeed for the
      correct tenant.
- [ ] Logout, then reuse of the old cookie is denied.
- [ ] Tenant A cannot read/list/stream Tenant B's named resources via normal
      requests or direct URL manipulation — and the reverse direction too.
- [ ] Forbidden/not-found responses contain no other tenant's data in body
      or headers.
- [ ] The non-tenant authenticated user sees no tenant resources.
- [ ] Unauthenticated and expired-session requests are denied on protected
      endpoints.
- [ ] Invalid signature/issuer/audience/nonce/state rejected; replayed auth
      code rejected.
- [ ] Refresh success, refresh failure, and expiry paths all exercised.
- [ ] A tenant-owner-level identity still cannot perform a write, read a
      Secret, send an impersonation header, path-traverse, or reach an
      excluded subresource through TenantDeck.
- [ ] No token material appears in any captured response, cookie, Location
      header, or application log from the run.
- [ ] Logout invalidates the session on both replicas (hit each directly or
      force routing to confirm).
- [ ] Pod/SA hardening holds at runtime: no SA token volume mounted; app
      functions correctly with `readOnlyRootFilesystem: true`; no pods need
      runtime Kubernetes API RBAC.
- [ ] NetworkPolicy is tested against a CNI that actually enforces
      NetworkPolicy — verify both an allowed and a disallowed connection.
      A policy that merely renders correctly on a non-enforcing CNI is not
      a pass.
- [ ] Failure diagnostics captured on test failure are redacted (no tokens/
      secrets); teardown happens on both success and failure, and is scoped
      only to resources this test run created.

## Comments in tests (see `CLAUDE.md` "Code style")

Same rule as application code: the test name and the assertion should carry
the *what* whenever a name can express it —
`TestRefreshFailsClosedWhenStoreUnavailable`, not `TestRefresh` with a
comment explaining the case. When the name alone can't (see "Test size and
naming" above), a brief 1-2 line *what* comment above the test is the
accepted exception — better than a misleadingly generic name. Beyond that,
reserve comments for the *why*: why a fixture is shaped a particular way
(e.g. two tenants with overlapping resource names, specifically to catch an
isolation bug that distinct names would hide), why a wait uses a driven
clock instead of a `sleep`, or why a negative case is expected to fail in a
specific, non-obvious way. Don't caption each assertion with what it
asserts — that's what the assertion already says.

A test proving a security boundary is read far more often than it's changed
— prefer an explicit, slightly repetitive test over a clever generic harness
that hides which case is which. If a table-driven test's table gets hard to
read at a glance, that's a sign to split it, not to compress it further.

## After writing tests

- Update `docs/security-test-matrix.md`: fill in the `Test(s)` and
  `Evidence` columns for every row this work covers, and flip `Status` to
  `covered` only once the test actually passes locally (or in CI) — not on
  the strength of having written it.
- Check BFF coverage (`go test -cover`/`-coverprofile`) for the package you
  touched. Report the actual number — don't claim 100% without having run
  it.

---
name: tenantdeck-security-review
description: "Adversarial security review lenses for TenantDeck, derived from its threat model. Use when reviewing a PR/diff/new code in this repository against the project's security requirements — especially anything touching OIDC/session handling, the Capsule Proxy upstream client, Helm chart security context/RBAC, or CI workflow permissions. Trigger on \"review this\", \"security review\", \"check this PR/diff\", or before merging such changes. This is a review lens, not an implementation guide (use tenantdeck-development for that) and not a test-writing guide (use tenantdeck-testing for that), though findings here should become tests there."
---

# TenantDeck adversarial security review

Per `docs/spec/01-working-agreement.md`: implementation and review are
separate passes, and agreement between them does not prove correctness.
Review this diff as an attacker would, against the concrete requirements
below — not as a second implementer confirming the first one's choices.
Per `docs/spec/09-documentation-and-threat-model.md`: agent-generated code
and green CI are *not* independent security certification. Treat this skill
as that missing independent check.

For each finding: state the concrete exploit scenario (who, what request,
what they get), not just "this violates the spec." Findings should turn into
rows in `docs/security-test-matrix.md` and regression tests, not just prose
(per `docs/spec/10-process-and-definition-of-done.md` step 5 — "fix findings
and add regression tests; do not merely describe them").

## Lens 1 — Authentication and session (`docs/spec/04-auth-session-browser-security.md`)

- Is the Authorization Code flow using PKCE S256, a cryptographically
  random one-use `state`, a `nonce`, and a bounded login-transaction
  lifetime? Is session-fixation actually prevented (new session ID issued
  post-login)?
- Does token validation check issuer, signature/allowed algorithms,
  audience, expiry, and required claims via a maintained library — not
  hand-rolled JWT parsing? Is TLS verified, with custom CA bundle support?
- Can an attacker construct a redirect URI or post-login destination outside
  a fixed allowlist? Is the external URL/redirect URI derived from request
  Host/forwarding headers anywhere (it must not be)?
- Cookie attributes: Secure, HttpOnly, host-only, `Path=/`, ideally
  `__Host-` prefix. Is SameSite chosen deliberately (not blindly Strict,
  which can break the callback) and documented? Are loopback-only dev
  exceptions isolated from the production fail-closed path?
- Is there CSRF protection + exact `Origin` validation on state-changing
  local endpoints (e.g. logout)? Is the OIDC callback bound to its own login
  transaction rather than a blanket same-origin check? Is cross-origin API
  access disabled by default?
- Are token-bearing session records encrypted with authenticated encryption
  using a key stored *separately* from the record? Are session IDs random
  and opaque, with idle and absolute TTLs, and documented rotation support?
- Server-side refresh: does it handle refresh-token rotation and concurrent
  refresh across replicas atomically (no duplicate-refresh race)? Does it
  fail closed on invalid refresh, handle providers that omit a new ID token,
  and never reuse an expired token or silently substitute an access token?
- Does logout actually delete server-side state such that a stolen/cached
  cookie is rejected afterward, and does refresh fail to resurrect a deleted
  session? Is it documented that dashboard logout doesn't revoke Kubernetes
  JWTs or the IdP SSO session?
- If the session store is unavailable, do protected operations fail closed —
  no fallback to unverified cookies, in-memory sessions, or another
  credential?
- Is token material absent from every browser-visible surface: responses,
  URLs, Location headers, logs, traces, metrics, error pages?

## Lens 2 — Upstream boundary (`docs/spec/05-upstream-boundary-and-resilience.md`)

- Is the upstream origin fixed by trusted config — no caller-controlled
  host, full URL, redirect target, credentials, or kubeconfig path?
- Are outbound headers built from an explicit allowlist? Specifically
  check: is `Authorization`, `Cookie`, any `Impersonate-*`, `X-Remote-*`, or
  `X-Forwarded-Client-Cert` from the *incoming* request ever forwarded
  upstream? (It must never be.) Is only the backend-held ID token inserted?
- Does the router reject unexpected methods, encoded path
  traversal/separators, ambiguous query parameters, unsupported content
  types, and any subresource not on an explicit allowlist — *before*
  constructing the upstream request?
- Could a tenant owner with broad Kubernetes RBAC reach a write, Secret,
  exec/attach/port-forward, or proxy subresource through this code path?
  ("It's just a GET" is not a safe argument by itself.)
- Are pagination, log tail/bytes, request/header sizes, response sizes,
  timeouts, concurrent requests, and active streams all bounded? Does
  cancellation actually terminate the upstream request/stream, not just the
  HTTP response to the browser?
- Could one session/browser create unbounded watches or trigger a retry
  storm on repeated 401/403s?
- Is there any code path where one user's client/Authorization header could
  be reused for another session's request (shared client, cache, pool)?
- Are logs structured and redacted, with correlation IDs and bounded-
  cardinality metrics? Is there a reachable debug/pprof route? Do liveness
  checks depend on IdP/Kubernetes availability (they must not)?

## Lens 3 — Container and Kubernetes hardening (`docs/spec/06-container-and-kubernetes-deployment.md`)

- Does the chart render these at the correct Pod/container fields (not a
  single copy-pasted block that silently doesn't apply)?
  `automountServiceAccountToken: false`; Pod: `runAsNonRoot: true`,
  `runAsUser: 65532`, `runAsGroup: 65532`, `seccompProfile.type:
  RuntimeDefault`; container: `allowPrivilegeEscalation: false`,
  `readOnlyRootFilesystem: true`, `capabilities.drop: [ALL]`.
- Does the chart create any Role/ClusterRole/RoleBinding/ClusterRoleBinding,
  or does the app load in-cluster credentials anywhere? (It must not — zero
  Kubernetes API permissions for TenantDeck's own SA.)
- Any privileged container, hostNetwork/hostPID/hostIPC, hostPath, or
  writable volume not justified and size-limited (emptyDir)?
- Does the chart avoid bundling a production database, support
  external Valkey/Redis with auth + TLS/custom CA via existing Secret
  references, and work correctly with 2 replicas sharing sessions?
- Are OIDC client secret and session encryption key referenced from
  existing Secrets — zero plaintext secrets in values/ConfigMaps/NOTES, and
  no secret generated at template-render time?
- Is NetworkPolicy enabled by default with explicit (not allow-all) egress —
  ingress controller, DNS, IdP, Capsule Proxy, session store — and does a
  restricted-policy CI profile actually exist and pass?
- Does the chart render cleanly under the Restricted Pod Security Standard?

## Lens 4 — CI and supply chain (`docs/spec/08-github-actions-and-supply-chain.md`)

- Do fork PR workflows run without repository secrets or write
  permissions? Is default `permissions: contents: read`, with elevated
  permissions scoped only to the specific jobs that need them (e.g.
  release)?
- Are third-party Actions pinned to full commit SHAs (not a tag or branch)?
  Is checkout configured without persisted credentials?
- Any untrusted `pull_request_target` usage, or untrusted input
  (PR title/branch name/issue body) flowing into a shell command?
- Is there a vulnerability policy that actually fails the build on
  actionable high/critical findings, with no broad blanket suppressions?
- Does the release workflow gate publishing/signing behind a trusted tag,
  keep `id-token`/`write` scoped to only that job, and avoid any publish
  path reachable from a PR?

## Lens 5 — Cross-tenant isolation

This is the property the mandatory E2E suite exists to prove
(`docs/spec/07-mandatory-automated-testing.md`) — use it as a review lens
too, not just a test-writing checklist:

- For every new endpoint: can Tenant A reach Tenant B's named resource,
  list, or log stream by substituting an ID/namespace in the request, in
  either direction? Does a forbidden/not-found response leak any of the
  other tenant's data in its body or headers?
- Does an authenticated user with no tenant access see anything at all?
- Is tenant identification derived from the deployed Capsule model (e.g. a
  live lookup/Capsule Proxy-scoped call), not a client-supplied namespace
  name or a trusted JWT claim asserting a tenant name?

## Lens 6 — Comments (see `CLAUDE.md` "Code style")

- Flag comments that only restate the next line — they should be deleted,
  not left as noise in a security-sensitive diff.
- Flag a *missing* comment where a security-relevant why isn't obvious from
  the code alone — e.g. a header strip, a fail-closed branch, or a
  deliberate deviation from the spec's default. The reviewer should not have
  to reconstruct the rationale from the spec every time; a one-line why at
  the decision point is cheap insurance against a future "cleanup" removing
  it.
- A comment is never a substitute for the fix itself — "TODO: validate this"
  next to unvalidated input is a finding, not documentation.
- Flag code that is clever or dense enough to obscure a security-relevant
  check — an attacker-relevant bug hiding in code a reviewer can't quickly
  read is itself a finding. A micro-optimization around auth/session/
  upstream-boundary logic needs a concrete, stated reason (see `CLAUDE.md`
  "Code style" — readability is the default here); if it doesn't have one,
  that's worth raising even if the logic happens to be correct.

## After the review

List findings as concrete exploit scenarios, ranked by what an attacker
gains. For each: is it already covered by a row in
`docs/security-test-matrix.md`? If not, it's missing coverage — say so
explicitly rather than letting it pass because "the code looks fine."

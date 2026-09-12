# Threat model

**Status: acknowledgments, assets, trust boundaries, and attacker
archetypes written (Phase 1); per-component analysis and residual risks
now written against the real Phase 1-4 codebase.** The required
acknowledgments below come straight from
[`docs/spec/09-documentation-and-threat-model.md`](spec/09-documentation-and-threat-model.md)
and are already authoritative — don't soften or remove them as the system
gets built.

## Required acknowledgments (do not remove)

- A BFF reduces bearer-token extraction but active XSS can still read
  allowed data or act using the victim's session. (This is why
  `docs/spec/04-auth-session-browser-security.md`'s CSP/hostile-content
  requirements and Lens 6 of `tenantdeck-security-review` exist — they
  reduce the odds of XSS happening, they don't remove its consequence once
  it does.)
- Pod logs may contain sensitive information — TenantDeck displaying a log
  is not a TenantDeck vulnerability if the workload put secrets in its own
  logs, but it is a reason log content is treated as hostile/untrusted
  input rather than safe to render unescaped.
- Capsule and Kubernetes remain the actual tenant boundary. TenantDeck adds
  a read-only allowlist in front of that boundary; it does not replace it,
  and a Capsule/Kubernetes authorization bug is not fixed by anything in
  this codebase.
- Public Capsule Proxy access remains independently usable by authenticated
  kubectl clients, unchanged by TenantDeck's existence. TenantDeck is not a
  replacement for, or a hardening of, that existing access path.
- Agent-generated code and a green CI run are not independent security
  certification. The adversarial review pass
  (`tenantdeck-security-review`) is a structured check, not a substitute
  for an actual independent audit before anything security-sensitive is
  trusted in a real deployment.

## Assets

What's actually worth protecting, most sensitive first:

1. **The session encryption key.** Whoever has it can decrypt every session
   record in the store, recovering every active user's ID/refresh tokens.
   Stored separately from the records it protects (`docs/spec/04-auth-session-browser-security.md`);
   its rotation procedure belongs in `docs/operations.md`.
2. **Session records** (encrypted ID/refresh tokens, tied to a real
   identity at a real tenant) in Valkey/Redis.
3. **The BFF's forwarded ID token**, in flight to Capsule Proxy on each
   upstream call — it's what makes every upstream request authoritative as
   that specific user.
4. **Tenant workload data surfaced through the UI**: pod logs, annotations,
   event messages, resource names/specs across the required views in
   `docs/spec/02-product-and-scope.md`. Sensitive because a workload may
   have put secrets or PII into it, not because TenantDeck adds anything
   sensitive of its own.
5. **The OIDC client secret**, which lets someone impersonate TenantDeck to
   the IdP if stolen.

## Trust boundaries

| Boundary | What crosses it | What's validated crossing in | What's never trusted from the other side |
|---|---|---|---|
| Browser ↔ BFF | Session cookie (opaque ID only), page requests | Session ID → server-side lookup; CSRF/Origin check on state-changing routes; CSP constrains what the page can load/render | Any token material — the browser never holds one. No request parameter is treated as an authorization decision (e.g. a namespace name in a URL proves nothing by itself). |
| BFF ↔ Capsule Proxy | The BFF's held ID token (as `Authorization: Bearer`), an explicitly allowlisted request (`docs/route-allowlist.md`) | Request is built server-side from validated parameters only; response is relayed, not re-interpreted as more trusted than it is | Incoming `Authorization`/`Cookie`/`Impersonate-*`/`X-Forwarded-Client-Cert` headers — never forwarded (`docs/spec/05-upstream-boundary-and-resilience.md`). Upstream redirects/`Set-Cookie`/auth challenges aren't relayed indiscriminately. |
| BFF ↔ session store (Valkey/Redis) | Encrypted session records, over TLS with store auth | Store auth + TLS/custom CA (`docs/spec/06-container-and-kubernetes-deployment.md`) | A reachable-but-unauthenticated store is never treated as good enough — store failure fails closed, not open. |
| BFF ↔ IdP | Authorization code, tokens, JWKS | Issuer/signature/audience/expiry/nonce/state validation via the OIDC library (ADR-003); TLS verified, custom CA bundle supported | A token is never accepted solely because it would be accepted by Kubernetes — the BFF validates its own intended audience separately (`docs/spec/03-architecture.md`). |

## Attacker archetypes

- **An authenticated user of a different tenant** — the most realistic
  adversary TenantDeck faces day to day. Every route in
  `docs/route-allowlist.md` needs a negative test proving this archetype
  gets nothing from another tenant's namespace.
- **An attacker who achieves XSS** (via a hostile pod log/annotation/event,
  per the required acknowledgments above) — acts with the victim's live
  session. CSP and hostile-content handling (Lens 6,
  `docs/spec/04-auth-session-browser-security.md`) reduce how this
  happens; they don't remove what it grants once it does, which is why
  it's listed as an acknowledged limitation, not something "fixed."
- **A network attacker on the cluster** (e.g. a compromised pod in an
  unrelated namespace) — this is what NetworkPolicy
  (`docs/spec/06-container-and-kubernetes-deployment.md`) and TLS between
  the BFF and its dependencies are against.
- **Someone who compromises the session store** — mitigated by encrypting
  session records with a separately-held key (asset #1/#2 above); a store
  compromise alone shouldn't yield usable tokens.
- **An authenticated user with no tenant** — should see nothing; distinct
  from the cross-tenant case because there's no tenant data to leak at all,
  only the question of whether the UI/API correctly shows "nothing" instead
  of erroring into a default-allow state.

## Per-component analysis

- **OIDC login (`internal/auth`).** Scenarios considered: code/state
  replay (mitigated — single-use login transactions via Redis `GETDEL`,
  `TestCallbackHandler_RejectsReplayedState`), nonce substitution
  (mitigated — server-held expected nonce threaded through context, not
  client-suppliable), session fixation (partially mitigated — session IDs
  are only minted post-verification and never client-supplied, but no
  test yet pre-seeds a cookie before login to confirm it's ignored; see
  `docs/security-test-matrix.md` row 9), and a malicious/compromised IdP
  response (out of scope by design — `docs/spec/03` treats the configured
  issuer as trusted; a compromised IdP is a different, larger problem
  than TenantDeck can mitigate).
- **Session handling (`internal/session`).** Scenarios considered: Redis
  compromise alone (mitigated — AES-256-GCM with a separately-held key;
  asset #1/#2), store unavailability (mitigated — every operation fails
  closed, never falls back to treating the caller as authenticated),
  and stale/replayed sessions after logout (mitigated —
  `TestLogoutHandler_OldCookieIsRejectedAfterLogout`). Not mitigated:
  idle-timeout-independent absolute expiry only (accepted residual risk
  below), and no refresh-race protection since refresh isn't implemented
  at all yet.
- **Upstream client (`internal/capsule`).** Scenarios considered: header
  injection via a caller-controlled path segment (mitigated — every
  path parameter goes through `validK8sName`'s DNS-1123 validator before
  a request is built), forwarding-header spoofing (mitigated — the
  request is built from an explicit allowlist of headers, never copied
  from the incoming request), and an oversized/slow upstream response
  tying up a goroutine indefinitely (mitigated for bounded calls via
  `httpClient`'s `Timeout` + `readBounded`'s size cap; mitigated for the
  log-streaming path via `context.WithTimeout` + `io.LimitReader` instead,
  since a shared wall-clock timeout would cut a legitimate `follow=true`
  stream). Not mitigated, and can't be from this layer alone: Capsule
  Proxy itself returning data for the wrong tenant — that's Capsule's own
  boundary, not TenantDeck's to fix (required acknowledgment above).
- **Frontend (`web/`).** Scenarios considered: hostile Kubernetes-sourced
  content (pod logs, event messages, ingress hosts, resource names)
  becoming executable markup (mitigated — React's default text-node
  escaping plus an explicit `isSafeHostname` gate before anything becomes
  an `<a href>`; tested per-component, see `docs/security-test-matrix.md`
  row 29/30) and a strict CSP as a second layer if escaping ever has a
  gap (`default-src 'self'; object-src 'none'; base-uri 'none';
  frame-ancestors 'none'`, no `unsafe-inline`/`unsafe-eval`, no
  third-party sources). Not mitigated: an XSS that does occur still acts
  with the victim's live session (required acknowledgment above — this
  is a consequence-reduction design, not an XSS-proof one).
- **Container/chart/E2E (`Dockerfile`, `charts/tenantdeck`, `cmd/mockoidc`).**
  Scenarios considered: a compromised/malicious build dependency
  (mitigated for the shipped image — pinned base image digests, no
  shell/package manager in the final `distroless/static` stage, so even a
  build-time compromise has a minimal runtime surface to persist in); a
  Pod with excess privilege being used to pivot (mitigated — non-root
  UID 65532, `allowPrivilegeEscalation: false`, all capabilities dropped,
  `readOnlyRootFilesystem: true`, no SA token mount, no RBAC objects
  created by the chart at all — verified against the actually-running
  container, not just the rendered manifest, see
  `docs/implementation-plan.md` Phase 4); `cmd/mockoidc` itself becoming
  a production login bypass (mitigated by construction — it is a
  separate `cmd/`, never imported by `cmd/tenantdeck`, built by a
  separate Dockerfile (`e2e/mockoidc.Dockerfile`) never referenced by the
  production one). Not yet mitigated/verified: NetworkPolicy enforcement
  against a real enforcing CNI, and Capsule-tenant-boundary behavior
  under a real multi-tenant cluster — both blocked in this session for
  reasons with no bearing on TenantDeck's own code (see
  `docs/implementation-plan.md` Phase 4 "Known blockers").

## Residual risks accepted

- **Absolute-only session expiry, no idle timeout.** A session stays
  valid until the ID token's own `exp`, even if the user walked away — a
  shared/kiosk-browser risk `docs/spec/04-auth-session-browser-security.md`
  asks to be mitigated but isn't yet (matrix row 10, `partial`). Accepted
  for now because fixing it properly needs refresh-token handling first
  (an idle timeout without refresh just logs everyone out early).
- **No login/token-exchange rate limiting** (matrix row 33, `missing`).
  Accepted as a known gap, not a design decision — an IdP-side brute-force
  protection (most real IdPs already rate-limit token endpoints
  themselves) is the practical mitigation until this is built.
- **NetworkPolicy's CIDR-based egress allowlists go stale on IP rotation**
  (DNS rotation, a load balancer changing addresses) with no warning —
  documented in `docs/operations.md`, inherent to `networking.k8s.io/v1`
  NetworkPolicy (no FQDN matching), not fixable by TenantDeck's chart
  alone without requiring a specific CNI (e.g. Cilium) that not every
  installation will run.
- **No metrics endpoint and no request-correlation ID yet**
  (`docs/operations.md` "Logs and metrics") — an operational gap that
  doesn't weaken the security boundary itself, but does make detecting
  abuse or diagnosing an incident slower than it should be.

This file is read by `tenantdeck-security-review` reviewers as the
standing threat model — keep it reconciled with
`docs/security-test-matrix.md` rather than letting either drift ahead of
the other.

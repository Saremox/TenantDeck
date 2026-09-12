# Threat model

**Status: acknowledgments, assets, trust boundaries, and attacker
archetypes written (Phase 1). Per-component analysis and residual risks
still open** — those need real code to analyze, not the intended design.
The required acknowledgments below come straight from
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

## Still open (see implementation-plan.md Phase 1 note)

- **Per-component analysis** once components exist: for each of OIDC
  login, session handling, the upstream client, and the frontend, the
  concrete attack scenarios considered and how they're mitigated (or
  explicitly accepted as a residual risk, with the reason).
- **Residual risks actually accepted**, distinct from the required
  acknowledgments above — e.g. any NetworkPolicy limitation documented in
  `docs/operations.md`, or a CI exception recorded under
  `docs/spec/08-github-actions-and-supply-chain.md`'s vulnerability policy.

This file is read by `tenantdeck-security-review` reviewers as the
standing threat model — keep it reconciled with
`docs/security-test-matrix.md` rather than letting either drift ahead of
the other.

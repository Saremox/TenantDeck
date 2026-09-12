# Threat model

**Status: acknowledgments seeded from spec; full analysis not started.** The
required acknowledgments below come straight from
[`docs/spec/09-documentation-and-threat-model.md`](spec/09-documentation-and-threat-model.md)
and are already authoritative — don't soften or remove them as the system
gets built. Everything else here still needs to be written against the
actual implementation, not the intended one.

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

## What this must eventually also cover

- **Assets**: what's actually worth protecting (session records and their
  encryption key, the BFF's ID token forwarding path, tenant workload
  metadata/logs) and who the realistic attackers are (an authenticated
  user of a different tenant; an attacker who achieves XSS; a network
  attacker on the cluster; someone who compromises the session store).
- **Trust boundaries**: browser↔BFF, BFF↔Capsule Proxy, BFF↔session store —
  what's validated crossing each one, and what deliberately isn't trusted
  from the other side (see `docs/spec/05-upstream-boundary-and-resilience.md`
  for what the BFF↔upstream boundary must never forward).
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

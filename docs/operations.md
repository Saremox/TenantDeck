# Deployment and operations guide

**Status: not started.** No Helm chart exists yet. This is a skeleton so
the filename and required sections exist before implementation starts;
replace each section with real, tested instructions once the chart exists —
see
[`docs/spec/06-container-and-kubernetes-deployment.md`](spec/06-container-and-kubernetes-deployment.md)
for the chart requirements this must document.

## What this must eventually cover

- **Installing/upgrading the chart**: required values, how `values.schema.json`
  validation surfaces misconfiguration, and what a rolling upgrade does to
  existing sessions (two replicas, graceful shutdown).
- **External Valkey/Redis**: how to point the chart at an existing store
  with auth and TLS/custom CA, via existing Secret references — never a
  bundled production database (`docs/spec/06-container-and-kubernetes-deployment.md`).
- **OIDC client secret and session encryption key provisioning**: which
  existing Secrets the chart expects, and the **documented key rotation
  procedure** required by
  `docs/spec/04-auth-session-browser-security.md` — rotating the session
  encryption key without silently breaking or leaking existing sessions.
- **Session revocation procedure**: how an operator forcibly invalidates
  sessions (e.g. a compromised account) outside of normal user-initiated
  logout.
- **NetworkPolicy setup**: the ingress/egress allowances this environment
  needs (ingress controller, DNS, IdP, Capsule Proxy, session store), and
  the explicit limitations to document per spec — DNS rotation, CNI/NAT
  behavior, and an optional Cilium FQDN-based example, since standard
  NetworkPolicy can't express a hostname allowlist.
- **Compatibility and upgrade notes**: the tested Kubernetes/Capsule
  version matrix, and what changes between versions that operators need to
  know before upgrading.
- **Health/readiness semantics**: what liveness vs. readiness actually
  check, and confirmation neither depends on IdP/Kubernetes availability
  (`docs/spec/05-upstream-boundary-and-resilience.md`).
- **Logs and metrics an operator will actually see**: structured/redacted
  log format, correlation IDs, and what the bounded-cardinality metrics
  cover — written for someone operating this, not debugging the source.

Write this once the chart is real and has actually been installed/upgraded
against a cluster — an untested operations guide is worse than none, because
it reads as verified when it isn't.

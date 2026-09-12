# Security policy

**Status: not started.** TenantDeck has no released version yet — this file
is a skeleton so the filename and required sections exist before they're
needed, per
[`docs/spec/09-documentation-and-threat-model.md`](docs/spec/09-documentation-and-threat-model.md).
Fill it in with real content once there's a supported release to report
against.

## What this must eventually cover

- **Supported versions.** Which tagged releases receive security fixes,
  once releases exist (see `docs/spec/08-github-actions-and-supply-chain.md`
  for the release workflow that will produce them).
- **Reporting a vulnerability.** A concrete, working contact path (not a
  placeholder address) and what to expect after reporting (acknowledgement
  time, disclosure coordination).
- **Scope.** TenantDeck's own code, container image, and Helm chart are in
  scope. The underlying Capsule/Kubernetes platform, the configured IdP, and
  the external Valkey/Redis deployment are explicitly out of scope for this
  policy — link to [`docs/threat-model.md`](docs/threat-model.md) for why
  those remain the tenant boundary regardless of what TenantDeck does.
- **Known, accepted limitations.** Point at the threat model's acknowledged
  items (XSS can still act within a victim's session even with a BFF; pod
  logs may contain sensitive information leaked by workloads, not by
  TenantDeck) rather than restating them — this file shouldn't drift out of
  sync with that one.

Do not publish this with placeholder contact information or a vulnerability
process that doesn't actually work end to end — an unreachable reporting
path is worse than no policy at all.

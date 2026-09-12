# Documentation and threat model

> Part of the [TenantDeck kickoff specification](README.md).

Create README.md, CLAUDE.md, SECURITY.md, docs/architecture.md,
docs/threat-model.md, docs/security-test-matrix.md, brief ADRs, a
local-development guide, and deployment/operations instructions. Cover
configuration, OIDC audience/group mapping, mock versus real provider
limitations, custom CAs, external session-store ACL/TLS, key rotation,
session revocation, NetworkPolicy setup, compatibility versions and
upgrades. Include deterministic quickstart and test commands that work from
a clean checkout.

## Threat model must acknowledge

- A BFF reduces bearer-token extraction but active XSS can still read allowed data/use the victim's session.
- Pod logs may contain sensitive information.
- Capsule/Kubernetes remain the tenant boundary.
- Public Capsule access remains independently usable.
- Agent-generated code and successful CI are not independent security certification.

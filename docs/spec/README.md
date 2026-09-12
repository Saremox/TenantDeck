# TenantDeck kickoff specification

TenantDeck is an independent, security-first customer dashboard for an existing
Capsule multi-tenant Kubernetes platform.

Tagline: *Your slice of Kubernetes.*

This directory splits the original autonomous-implementation kickoff prompt into
topic files so future sessions and subagents can load only the section they
need instead of the whole document. The content below is preserved verbatim
from that prompt (reorganized, not rewritten) and is normative for how
TenantDeck must be built. It describes a **target v1**, not work already done —
check `docs/implementation-plan.md` for actual progress.

Read `CLAUDE.md` at the repository root first; it has the condensed,
always-relevant constraints. Come here for the full detail behind any one of
them.

## Index

1. [Working agreement](01-working-agreement.md) — how the implementing agent is expected to operate.
2. [Product and scope](02-product-and-scope.md) — what v1 is, the required UI, and what is explicitly out of scope.
3. [Architecture](03-architecture.md) — BFF/frontend split, request path, identity/audience handling, authorization model.
4. [Authentication, sessions, and browser security](04-auth-session-browser-security.md) — OIDC flow, cookies, CSRF, encryption, refresh, logout, CSP.
5. [Upstream boundary and resilience](05-upstream-boundary-and-resilience.md) — the Capsule Proxy boundary, header allowlisting, limits, isolation between users.
6. [Container and Kubernetes deployment](06-container-and-kubernetes-deployment.md) — Dockerfile, Pod/container security context, Helm chart requirements.
7. [Mandatory automated testing](07-mandatory-automated-testing.md) — required unit/integration coverage and the full-stack E2E stack and assertions.
8. [GitHub Actions and supply chain](08-github-actions-and-supply-chain.md) — CI requirements, least privilege, vulnerability policy, release workflow.
9. [Documentation and threat model](09-documentation-and-threat-model.md) — required docs and the threat model's acknowledged limits.
10. [Process and definition of done](10-process-and-definition-of-done.md) — execution sequence, definition of done, final handoff format.
11. [Primary references](11-primary-references.md) — external docs to verify against before pinning dependency/API versions.

## Related project docs

- `CLAUDE.md` — condensed durable constraints and project commands.
- `docs/implementation-plan.md` — living progress tracker against the execution sequence.
- `docs/security-test-matrix.md` — living map of requirements in this spec to actual tests.

## Skills

Three project skills under `.claude/skills/` operationalize this spec for agent
work in this repository:

- `tenantdeck-development` — implementing features against the architecture and security constraints.
- `tenantdeck-security-review` — adversarial review lenses derived from the threat model.
- `tenantdeck-testing` — unit/integration and mandatory E2E testing guidance.

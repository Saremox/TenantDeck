# TenantDeck

A Kubernetes customer dashboard designed, implemented, tested, and documented entirely by AI agents.

*Your slice of Kubernetes.*

TenantDeck is a security-first, read-only dashboard for an existing Capsule
multi-tenant Kubernetes platform. The vertical slice (OIDC login →
server-side session → namespace list → UI) is implemented and tested — see
`docs/implementation-plan.md` for what's verified vs. still open (notably:
not yet validated against a real Capsule Proxy or IdP).

- [`CLAUDE.md`](CLAUDE.md) — durable constraints, project commands, current status.
- [`docs/local-development.md`](docs/local-development.md) — how to build and run it locally.
- [`docs/spec/`](docs/spec/README.md) — full requirements, split by topic.
- [`docs/implementation-plan.md`](docs/implementation-plan.md) — progress tracker, dependency-set ADRs, known blockers.
- [`docs/route-allowlist.md`](docs/route-allowlist.md) — the concrete BFF route → upstream call allowlist.
- [`docs/security-test-matrix.md`](docs/security-test-matrix.md) — requirements-to-tests map.
- [`SECURITY.md`](SECURITY.md) — vulnerability reporting policy (skeleton).
- [`docs/architecture.md`](docs/architecture.md), [`docs/operations.md`](docs/operations.md) — required docs, still skeletons (written once the chart/Phase 4 exist).
- [`docs/threat-model.md`](docs/threat-model.md) — assets/trust-boundaries written; per-component analysis still open.

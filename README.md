# TenantDeck

A Kubernetes customer dashboard designed, implemented, tested, and documented entirely by AI agents.

*Your slice of Kubernetes.*

TenantDeck is a security-first, read-only dashboard for an existing Capsule
multi-tenant Kubernetes platform. This repository currently contains the
project's specification, agent guidance, and Phase 1 planning output
(dependency decisions, route allowlist, threat model); application code has
not been implemented yet.

- [`CLAUDE.md`](CLAUDE.md) — durable constraints and project commands.
- [`docs/spec/`](docs/spec/README.md) — full requirements, split by topic.
- [`docs/implementation-plan.md`](docs/implementation-plan.md) — progress tracker and dependency-set ADRs.
- [`docs/route-allowlist.md`](docs/route-allowlist.md) — the concrete BFF route → upstream call allowlist.
- [`docs/security-test-matrix.md`](docs/security-test-matrix.md) — requirements-to-tests map.
- [`SECURITY.md`](SECURITY.md) — vulnerability reporting policy (skeleton).
- [`docs/architecture.md`](docs/architecture.md), [`docs/local-development.md`](docs/local-development.md), [`docs/operations.md`](docs/operations.md) — required docs, still skeletons.
- [`docs/threat-model.md`](docs/threat-model.md) — assets/trust-boundaries written; per-component analysis still open.

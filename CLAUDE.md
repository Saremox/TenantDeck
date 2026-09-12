# CLAUDE.md

TenantDeck is a security-first, read-only customer dashboard for an existing
Capsule multi-tenant Kubernetes platform. Tagline: *Your slice of Kubernetes.*

**Status: Phase 1 (inspection and planning) done, no application code yet.**
The full requirements live in [`docs/spec/`](docs/spec/README.md), split by
topic from the original kickoff prompt — read that index before writing
code; it is normative, this file is just the condensed, always-relevant
summary. The dependency set is decided
([`docs/implementation-plan.md`](docs/implementation-plan.md) ADRs) and the
concrete route allowlist exists
([`docs/route-allowlist.md`](docs/route-allowlist.md)) — implement against
those rather than re-deciding them.

## Durable constraints (non-negotiable)

These hold regardless of which part of the app is being touched. Full detail
and rationale is in `docs/spec/`.

- **Read-only v1, enforced server-side.** No writes, YAML editing, Secrets,
  ConfigMap contents, exec/attach/port-forward, or node/RBAC administration —
  ever, even for a tenant owner with broad Kubernetes rights. See
  [`docs/spec/02-product-and-scope.md`](docs/spec/02-product-and-scope.md).
- **Architecture:** Browser → same-origin Go BFF (embeds the built
  React/TS frontend) → existing Capsule Proxy → Kubernetes API. The BFF holds
  server-side sessions in external Valkey/Redis; the browser only ever sees an
  opaque session ID, never ID/access/refresh tokens. See
  [`docs/spec/03-architecture.md`](docs/spec/03-architecture.md).
- **Upstream calls use explicit allowlists** (routes, resources, subresources,
  verbs) built from validated parameters, with outbound headers built from an
  explicit allowlist — never forward incoming `Authorization`, `Cookie`,
  `Impersonate-*`, or other forwarding headers. See
  [`docs/spec/05-upstream-boundary-and-resilience.md`](docs/spec/05-upstream-boundary-and-resilience.md)
  and the concrete allowlist in
  [`docs/route-allowlist.md`](docs/route-allowlist.md).
- **No in-cluster credentials for TenantDeck itself.** The Helm chart creates
  no Role/ClusterRole/RoleBinding/ClusterRoleBinding, disables SA token
  automount at both SA and Pod level, and runs as non-root (UID/GID 65532)
  with `readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false`,
  all capabilities dropped, and `seccompProfile: RuntimeDefault`. See
  [`docs/spec/06-container-and-kubernetes-deployment.md`](docs/spec/06-container-and-kubernetes-deployment.md).
- **Cluster-mutating commands only target disposable, explicitly created test
  infrastructure** (kind, test kubeconfig/context) — never a developer's
  default kubeconfig or production credentials.
- **Never bypass permissions, disable security tests to go green, fabricate
  results, or claim an unexecuted check passed.** A blocker gets reported
  explicitly, not hidden.
- **Verify current upstream docs** (Kubernetes, Capsule, OIDC, Actions — see
  [`docs/spec/11-primary-references.md`](docs/spec/11-primary-references.md))
  before pinning dependency/API versions. Never invent an available version.

## Code style

Readability and maintainability are the priority. A human should be able to
read a function top to bottom and know what it does; performance is a
secondary concern, not a default design constraint.

- Prefer the straightforward implementation over the clever or
  micro-optimized one. Don't hand-roll a faster version of something a
  standard library or maintained dependency already does clearly.
- Don't optimize for performance without a concrete, measured reason
  (a profiled hot path, a documented latency/throughput requirement). If
  there isn't one, write the version that's easiest to read.
- Favor short, single-purpose functions over one function doing several
  things for the sake of avoiding a call. A few extra lines that make the
  control flow obvious beat a dense one-liner that doesn't.
- This still has limits set elsewhere: upstream request/response size and
  concurrency bounds are security requirements
  (`docs/spec/05-upstream-boundary-and-resilience.md`), not a performance
  nice-to-have — don't drop them for simplicity.

Names carry meaning; comments carry context the code can't. Concretely:

- Name variables, functions, and types so the reader doesn't need a comment
  to know *what* they do. If a name needs a comment to explain its purpose,
  rename it instead of commenting it.
- Don't write comments that restate the code (`// increment counter` above
  `counter++`). If removing a comment loses no information, delete it.
- Write a comment only for the **why** when it isn't obvious from reading the
  code: a non-obvious constraint, a workaround for a specific upstream bug,
  a security-relevant invariant, a deliberate deviation from the "obvious"
  approach. Example worth a comment: *why* a header is stripped before
  forwarding upstream (security boundary) — not *that* it's stripped.
- No comments referencing the current task, a ticket, or "added for X" — that
  belongs in the commit message/PR description and rots as the code evolves.
- No doc blocks that restate a function's signature in prose. If a function's
  contract is genuinely non-obvious (e.g. partial failure behavior, what it
  does on a closed session store), document *that*, briefly.

## Testing philosophy

Full detail in the `tenantdeck-testing` skill; the durable rules:

- Target **100% line coverage on the BFF**, reached through real,
  meaningful tests — not padding. Prefer writing the test before or
  alongside the implementation (TDD), especially for security-boundary
  logic.
- Don't over-test: one well-placed test per behavior, not the same
  assertion repeated across layers "to be safe."
- **One test, one concern.** No 200-line tests covering several aspects —
  split them. The test name alone should say what's being verified; if it
  can't, add a 1-2 line comment above the test stating what it checks.
- **Frontend has no coverage percentage** — by design, not by omission.
  Named categories (API error mapping, session-expired handling, hostile-
  content rendering, each required UI state) are mandatory instead; a
  percentage over presentation code rewards the wrong thing.
- **Accessibility is an automated gate, not a judgment call.** axe-core (or
  equivalent) runs in `make verify`; zero serious/critical violations on
  required views, narrowly justified exceptions only.

## Skills for this repo

- `tenantdeck-development` — implementing BFF/frontend/chart/Dockerfile code against the architecture and security constraints.
- `tenantdeck-security-review` — adversarial review lenses derived from the threat model; run before merging anything touching auth, the upstream client, chart security context, or CI.
- `tenantdeck-testing` — required unit/integration coverage and the mandatory full-stack E2E stack (kind + Capsule + mock OIDC + Valkey + two replicas).

## Commands

Not yet defined — no build exists. Once implementation starts, this section
must be replaced with the real, working commands (per the working agreement
in [`docs/spec/01-working-agreement.md`](docs/spec/01-working-agreement.md)):

- `make verify` — fast deterministic checks (fmt, vet, lint, typecheck, unit/integration tests, automated accessibility checks, Helm lint/template, chart schema validation).
- `make e2e` — disposable full-stack E2E suite (kind + Capsule + mock OIDC + Valkey + built image/chart).

CI must call these same commands, not a separate path.

## Living docs to keep current

- [`docs/implementation-plan.md`](docs/implementation-plan.md) — progress against the execution sequence.
- [`docs/security-test-matrix.md`](docs/security-test-matrix.md) — requirements mapped to actual tests, with evidence.

Update both whenever you complete work, not just at the end of a session.

## Required docs not yet written

These are mandated by
[`docs/spec/09-documentation-and-threat-model.md`](docs/spec/09-documentation-and-threat-model.md)
and currently exist only as skeletons stating what must go in them —
replace each with real content at the point in the execution sequence
(`docs/spec/10-process-and-definition-of-done.md`) where it stops being
speculative:

- [`SECURITY.md`](SECURITY.md) — vulnerability reporting policy.
- [`docs/architecture.md`](docs/architecture.md) — the system as actually
  built, not a restatement of the spec.
- [`docs/threat-model.md`](docs/threat-model.md) — already carries the
  spec's required acknowledgments; the per-component analysis is still
  open. `tenantdeck-security-review` treats this as the living threat
  model.
- [`docs/local-development.md`](docs/local-development.md) — quickstart
  and dev workflow.
- [`docs/operations.md`](docs/operations.md) — deployment, key rotation,
  session revocation, NetworkPolicy setup.

Don't let a skeleton sit there looking finished — each one says "Status:
not started" for a reason; update that line the moment real content goes
in, and don't claim a quickstart or runbook works until it's actually been
run.

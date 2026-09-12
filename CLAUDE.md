# CLAUDE.md

TenantDeck is a security-first, read-only customer dashboard for an existing
Capsule multi-tenant Kubernetes platform. Tagline: *Your slice of Kubernetes.*

**Status: bootstrap only.** No application code exists yet — this session set
up the spec, docs, and skills an implementing agent needs. The full
requirements live in [`docs/spec/`](docs/spec/README.md), split by topic from
the original kickoff prompt. Read that index before writing code; it is
normative, this file is just the condensed, always-relevant summary.

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
  [`docs/spec/05-upstream-boundary-and-resilience.md`](docs/spec/05-upstream-boundary-and-resilience.md).
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

## Skills for this repo

- `tenantdeck-development` — implementing BFF/frontend/chart/Dockerfile code against the architecture and security constraints.
- `tenantdeck-security-review` — adversarial review lenses derived from the threat model; run before merging anything touching auth, the upstream client, chart security context, or CI.
- `tenantdeck-testing` — required unit/integration coverage and the mandatory full-stack E2E stack (kind + Capsule + mock OIDC + Valkey + two replicas).

## Commands

Not yet defined — no build exists. Once implementation starts, this section
must be replaced with the real, working commands (per the working agreement
in [`docs/spec/01-working-agreement.md`](docs/spec/01-working-agreement.md)):

- `make verify` — fast deterministic checks (fmt, vet, lint, typecheck, unit/integration tests, Helm lint/template, chart schema validation).
- `make e2e` — disposable full-stack E2E suite (kind + Capsule + mock OIDC + Valkey + built image/chart).

CI must call these same commands, not a separate path.

## Living docs to keep current

- [`docs/implementation-plan.md`](docs/implementation-plan.md) — progress against the execution sequence.
- [`docs/security-test-matrix.md`](docs/security-test-matrix.md) — requirements mapped to actual tests, with evidence.

Update both whenever you complete work, not just at the end of a session.

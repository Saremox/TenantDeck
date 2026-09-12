# Implementation plan

Living progress tracker against the execution sequence in
[`docs/spec/10-process-and-definition-of-done.md`](spec/10-process-and-definition-of-done.md).
Update this file whenever a phase advances — do not let it drift from what is
actually merged.

**Status: Phase 1 complete.** This repository contains the spec split,
`CLAUDE.md`, the project skills, skeleton stubs for the docs required by
`docs/spec/09-documentation-and-threat-model.md`, the dependency set decided
below, [`docs/route-allowlist.md`](route-allowlist.md), and the assets/trust-
boundary section of `docs/threat-model.md`. **No application code, chart, or
CI workflow exists yet** — nothing below has been built or run; these are
decisions, not working software.

## Phases

- [x] **1. Inspection and planning** — dependency set chosen and recorded as
      ADRs below; [`docs/route-allowlist.md`](route-allowlist.md) written;
      `docs/threat-model.md` assets/trust-boundaries filled in. Per-component
      threat analysis (OIDC login, session handling, upstream client,
      frontend) is deferred to when each component is actually built — you
      can't meaningfully threat-model code that doesn't exist yet.
- [ ] **2. Vertical slice** — real OIDC login → server-side session → real
      Capsule namespace list → UI, end to end. Write `docs/local-development.md`
      for real once this runs locally — don't leave it a skeleton once
      there's something to document.
- [ ] **3. Remaining bounded v1 features** — overview/quotas, workload
      list/detail views, services/ingresses/PVC status, events, bounded pod
      logs with follow/cancel, UI states — each with tests alongside it.
- [ ] **4. Hardened image/chart + full-stack E2E** — Dockerfile, Helm chart
      per `docs/spec/06-container-and-kubernetes-deployment.md`, the mandatory
      E2E stack per `docs/spec/07-mandatory-automated-testing.md`, then
      GitHub Actions CI/release workflows per
      `docs/spec/08-github-actions-and-supply-chain.md`. Write
      `docs/operations.md` and `docs/architecture.md` for real once the chart
      and E2E stack actually exist and have been run.
- [ ] **5. Adversarial security review** — run the `tenantdeck-security-review`
      skill against `docs/threat-model.md`; fix findings with regression
      tests, not just writeups.
- [ ] **6. Clean-build reconciliation** — re-run `make verify` / `make e2e`
      from a clean checkout; reconcile `docs/security-test-matrix.md` with
      actual evidence before calling anything done. Confirm none of the five
      required docs are still sitting at "Status: not started" — and that
      `SECURITY.md`'s reporting path actually works, not just reads as
      plausible.

## Decisions / ADRs

Researched and recorded 2026-09-12 per
`docs/spec/01-working-agreement.md` ("verify current primary documentation
before choosing dependency versions... never invent an available
version"). **Re-verify before actually pinning these in `go.mod`/`package.json`
if much time has passed** — this list is only as fresh as the date above.

- **ADR-001 — Go toolchain: 1.27.x.** Latest stable (1.27.1, released
  2026-09-01). No reason to pin older; TenantDeck has no legacy constraint.
- **ADR-002 — HTTP stack: standard library `net/http` only, no router
  dependency.** Go's built-in method+wildcard-pattern routing (since 1.22)
  is sufficient for the route set in `docs/route-allowlist.md`; this also
  matches the spec's explicit preference for the standard Go HTTP stack.
  Revisit only if routing logic genuinely outgrows it.
- **ADR-003 — OIDC relying-party library: `github.com/zitadel/oidc` (v4).**
  Chosen over `coreos/go-oidc`: go-oidc is RP-only and has seen
  comparatively little recent release activity; zitadel/oidc is actively
  maintained (v4 published 2026-07-30), OpenID Foundation-certified for
  the basic and config profiles, and supports what we need (Authorization
  Code + PKCE, discovery, JWKS validation, refresh). We use only its RP
  side — TenantDeck is never an OP.
- **ADR-004 — Session-store client: `github.com/redis/go-redis/v9`.**
  Valkey and Redis 8 share the RESP3 wire protocol, and go-redis connects
  to both without code changes. `valkey-io/valkey-go` was considered for
  its claimed throughput advantage, but go-redis is the more broadly
  adopted, longer-track-record library — the better "mature maintained
  library" choice per the working agreement, and this session store is not
  expected to be a performance bottleneck (see `CLAUDE.md` "Code style" on
  not optimizing without a measured reason).
- **ADR-005 — Frontend tooling: React 19 + TypeScript + Vite 8.** Current
  stable as of this writing (React 19.3.0, 2026-09-09; Vite 8.0.0,
  2026-03-12). Built as static assets and embedded into the Go binary via
  `go:embed`, per the architecture in `docs/spec/03-architecture.md`.
- **ADR-006 — Capsule compatibility set: Capsule operator v0.14.5 +
  capsule-proxy v0.14.1, target Kubernetes 1.36.x.** Capsule's operator
  states it "offers support only for the latest minor version of
  Kubernetes," which at time of writing is 1.36 (operator v0.14.5 requires
  >=1.36.0; capsule-proxy v0.14.1 requires >=1.35.0, so 1.36.x satisfies
  both). Don't pin to 1.37.x even though newer kind images offer it —
  Capsule's own compatibility statement hasn't caught up to it yet.
- **ADR-007 — Disposable E2E cluster: kind v0.33.0**, whose default node
  image is Kubernetes 1.36.1 — matches ADR-006 without needing a
  non-default `--image` override.
- **Deferred to Phase 4**: the E2E mock OIDC provider choice
  (`docs/spec/07-mandatory-automated-testing.md` asks for "a maintained
  mock if suitable") isn't decided here — it's an E2E-harness decision,
  not a production dependency, and belongs with the rest of that stack
  when it's actually built.

Record further ADRs here (or as separate files under `docs/adr/` if this
section grows unwieldy) as they're made.

## Known blockers

None yet — implementation has not started.

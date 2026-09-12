# Implementation plan

Living progress tracker against the execution sequence in
[`docs/spec/10-process-and-definition-of-done.md`](spec/10-process-and-definition-of-done.md).
Update this file whenever a phase advances — do not let it drift from what is
actually merged.

**Status: not started.** This repository currently contains only the spec
split, `CLAUDE.md`, and the project skills. No application code, chart, or
CI workflow exists yet.

## Phases

- [ ] **1. Inspection and planning** — choose a tested dependency set (Go
      HTTP stack, OIDC library, React/TS tooling, Valkey/Redis client, Capsule
      version); write the route allowlist; record ADRs for significant
      decisions.
- [ ] **2. Vertical slice** — real OIDC login → server-side session → real
      Capsule namespace list → UI, end to end.
- [ ] **3. Remaining bounded v1 features** — overview/quotas, workload
      list/detail views, services/ingresses/PVC status, events, bounded pod
      logs with follow/cancel, UI states — each with tests alongside it.
- [ ] **4. Hardened image/chart + full-stack E2E** — Dockerfile, Helm chart
      per `docs/spec/06-container-and-kubernetes-deployment.md`, the mandatory
      E2E stack per `docs/spec/07-mandatory-automated-testing.md`, then
      GitHub Actions CI/release workflows per
      `docs/spec/08-github-actions-and-supply-chain.md`.
- [ ] **5. Adversarial security review** — run the `tenantdeck-security-review`
      skill against the threat model; fix findings with regression tests, not
      just writeups.
- [ ] **6. Clean-build reconciliation** — re-run `make verify` / `make e2e`
      from a clean checkout; reconcile `docs/security-test-matrix.md` with
      actual evidence before calling anything done.

## Decisions / ADRs

None yet. Record short ADRs here (or as separate files under `docs/adr/` if
this section grows unwieldy) as they're made — dependency choices, Capsule
API version pinned, OIDC provider audience configuration, SameSite/cookie
choices, etc.

## Known blockers

None yet — implementation has not started.

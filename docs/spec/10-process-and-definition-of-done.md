# Process and definition of done

> Part of the [TenantDeck kickoff specification](README.md).

## Execution sequence

1. Inspect, choose a tested dependency set, write the concise plan/threat model and route allowlist.
2. Get a vertical slice working: actual OIDC login -> server-side session -> actual Capsule namespace list -> UI.
3. Implement remaining bounded v1 features with tests alongside them.
4. Complete hardened image/chart and full-stack E2E, then GitHub CI/release workflows.
5. Perform an adversarial review against the threat model. Fix findings and add regression tests; do not merely describe them.
6. Re-run checks from a clean build and reconcile the security matrix with actual test evidence.

## Definition of done

No placeholders in required v1 features; frontend builds; required local
checks and E2E pass in the available environment; actual image/chart
deployment works with hardening; workflows are complete and statically
validated; documentation matches behavior. Distinguish verified local results
from GitHub runs not yet executed. Any blocker or remaining security finding
must be explicitly reported, not labelled complete.

## Final handoff

Concise implementation summary, exact run/test/deploy commands, test outcomes
with evidence locations, remaining risks/blockers, and external setup still
needed. Begin now with repository inspection and proceed through
implementation without waiting for routine approvals.

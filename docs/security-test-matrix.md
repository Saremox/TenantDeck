# Security test matrix

Living map from requirements in [`docs/spec/`](spec/README.md) to the tests
that actually verify them, with evidence locations (test file + name, or CI
run). Do not mark a row "done" without a real passing test backing it —
see `docs/spec/01-working-agreement.md` on not fabricating results.

**Status: not started.** No tests exist yet. Rows below are the required
coverage extracted from the spec; fill in `Test(s)` and `Evidence` as they are
written, and keep `Status` honest (`missing` / `in progress` / `covered`).

| # | Requirement | Source | Test(s) | Evidence | Status |
|---|---|---|---|---|---|
| 1 | OIDC validation failures (signature/issuer/audience/nonce/state) | spec/04, spec/07 | | | missing |
| 2 | Authorization code replay rejected | spec/07 | | | missing |
| 3 | CSRF protection + exact Origin validation on state-changing endpoints | spec/04 | | | missing |
| 4 | Cookie flags (Secure/HttpOnly/host-only/Path=/, `__Host-` prefix, SameSite) | spec/04 | | | missing |
| 5 | Open-redirect / unsafe post-login destination rejected | spec/04 | | | missing |
| 6 | Malformed/traversal paths and encoded separators rejected by upstream boundary | spec/05 | | | missing |
| 7 | Forwarding-header spoofing (Authorization/Cookie/Impersonate-*/X-Remote-*) stripped | spec/05 | | | missing |
| 8 | Forbidden resource/subresource/verb access denied (read-only allowlist, incl. for a tenant owner) | spec/02, spec/05 | | | missing |
| 9 | Session fixation protection | spec/04 | | | missing |
| 10 | Session idle/absolute expiry enforced | spec/04 | | | missing |
| 11 | Refresh races across replicas (atomic store/locking) | spec/04 | | | missing |
| 12 | Logout-refresh race: refresh cannot resurrect a deleted session | spec/04 | | | missing |
| 13 | Session store failure → fail closed (no fallback auth) | spec/04 | | | missing |
| 14 | Stream cancellation terminates upstream work; log/stream limits enforced | spec/05 | | | missing |
| 15 | Token material never appears in responses/cookies/Location/logs (redaction) | spec/04, spec/07 | | | missing |
| 16 | No cross-user / cross-tenant data leakage (identity contamination under concurrency) | spec/05, spec/07 | | | missing |
| 17 | Tenant A cannot read/list/stream Tenant B's resources (incl. direct URL manipulation), both directions | spec/07 | | | missing |
| 18 | Non-tenant authenticated user sees no tenant resources | spec/07 | | | missing |
| 19 | Unauthenticated / expired sessions denied on protected endpoints | spec/07 | | | missing |
| 20 | Logout invalidates sessions across replicas | spec/04, spec/07 | | | missing |
| 21 | Pod/SA hardening: no SA token mounted, works under `readOnlyRootFilesystem`, no runtime RBAC grants | spec/06, spec/07 | | | missing |
| 22 | NetworkPolicy enforced by an actually-enforcing CNI (allowed + disallowed connections) | spec/06, spec/07 | | | missing |
| 23 | Chart renders Restricted Pod Security Standard-compliant manifests | spec/06 | | | missing |
| 24 | Two replicas share sessions correctly (Valkey/Redis-backed) | spec/06, spec/07 | | | missing |
| 25 | CI: fork PRs run without secrets/write privileges | spec/08 | | | missing |
| 26 | CI: vulnerability scan policy fails on actionable high/critical findings | spec/08 | | | missing |
| 27 | Release workflow: SBOM/provenance + artifact signing, no publish from PRs | spec/08 | | | missing |

Add rows as the spec is refined or new findings surface during the
adversarial review phase (`docs/spec/10-process-and-definition-of-done.md`,
step 5). Do not delete a row just because it's inconvenient — mark it
`missing` and explain the blocker instead.

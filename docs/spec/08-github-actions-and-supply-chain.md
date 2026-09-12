# GitHub Actions and supply chain

> Part of the [TenantDeck kickoff specification](README.md).

Provide PR/push CI with formatting, Go vet/static analysis/race tests,
frontend lint/typecheck/tests/build, Helm/schema validation, Docker build,
security scans, and mandatory full-stack E2E. Fork PRs must run without
repository secrets or write privileges.

Use least-privilege workflow/job permissions; default contents: read,
checkout without persisted credentials, full verified commit SHA pins for
third-party Actions with version comments, lockfiles and deterministic
installs, bounded job timeouts/concurrency, and no untrusted
pull_request_target execution. Prevent untrusted inputs from becoming shell
code. Keep release permissions and artifacts isolated from untrusted
PRs/caches. Include secret scanning and dependency/update automation; never
silently ignore scanner failures.

Provide a documented vulnerability policy: fail on actionable high/critical
findings, with narrowly scoped, justified, expiring exceptions if absolutely
necessary. Never add broad suppressions to reach green. Select a small
maintained toolset, such as govulncheck plus a container/IaC scanner and
secret scanner, rather than duplicating many tools.

Prepare a separate trusted-tag release workflow that verifies the release
source, builds and publishes the image and OCI Helm chart to GHCR, emits
SBOM/provenance, and signs/attests release artifacts using GitHub OIDC where
supported. Restrict write/id-token permissions to required release jobs.
Document verification commands and repository/environment protections that
require an owner's setup. No publishing from PRs. Test packaging locally; do
not claim remote publishing worked until an authorized run exists.

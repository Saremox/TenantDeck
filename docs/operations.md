# Deployment and operations guide

**Status: written against the real chart (`charts/tenantdeck`) and the
real image (`Dockerfile`).** `helm lint`/`helm template` and a real
`docker build`/`docker run` have been exercised against exactly what's
described here (see `docs/implementation-plan.md` Phase 4). Installing
the chart against a live cluster has **not** been run end-to-end yet -
see that same doc's "Known blockers" for precisely why, and don't read
anything below as "verified against a real cluster" until that changes.

## Installing/upgrading the chart

```sh
helm install tenantdeck charts/tenantdeck -n tenantdeck -f my-values.yaml
```

Start from [`charts/tenantdeck/ci/values-prod-example.yaml`](../charts/tenantdeck/ci/values-prod-example.yaml)
(production-shaped, every secret referenced by name only) or
[`values-dev.yaml`](../charts/tenantdeck/ci/values-dev.yaml) (loopback-relaxed,
disposable-cluster-only). `values.schema.json` is enforced on every
`helm install`/`upgrade`/`template`/`lint`: a missing required value
(`config.externalURL`, `config.oidc.issuerURL`/`clientID`/`existingSecret`,
`config.session.redis.addr`, `config.session.existingSecretEncryptionKey`,
`config.upstream.capsuleProxyURL`) fails immediately with the exact field
path, rather than deploying a Pod that crash-loops on a clearer error. The
schema also makes the hardening defaults (`securityContext.readOnlyRootFilesystem`,
`allowPrivilegeEscalation`, `capabilities.drop`, `podSecurityContext.runAsNonRoot`/
`runAsUser`/`runAsGroup`) `const`-pinned — weakening them isn't just
discouraged, it's a schema validation failure.

A rolling upgrade (`updateStrategy.rollingUpdate.maxUnavailable: 0`)
never drops below the configured replica count mid-rollout. Because
sessions live in the external Valkey/Redis, not in-process, an old
replica being terminated doesn't invalidate anyone's session — the next
request just lands on a different replica that reads the same encrypted
record. `terminationGracePeriodSeconds: 30` gives `cmd/tenantdeck`'s own
`server.Shutdown(15s timeout)` room to drain in-flight requests before
the process is killed.

## External Valkey/Redis

The chart never bundles a database - `config.session.redis.addr` must
point at an existing instance. Authentication: `config.session.redis.username`
(plaintext, non-secret) plus `config.session.redis.existingSecretPassword`
(name of an existing Secret; `existingSecretPasswordKey` defaults to
`password`) - leave the password field unset only for an unauthenticated
store, development only. TLS: `config.session.redis.tls.enabled: true`,
and for a private CA, `config.session.redis.tls.existingSecretCA` (an
existing Secret holding the CA bundle, key name configurable via
`existingSecretCAKey`, default `ca.crt`) - this is mounted read-only at
`/etc/tenantdeck/redis-ca/` and `internal/config.SessionConfig.BuildRedisTLSConfig`
reads it at startup, failing closed (not falling back to the system pool)
if the file is missing or contains no valid certificate.

## OIDC client secret and session encryption key provisioning

Both are existing-Secret references only - the chart never generates or
accepts a plaintext value in `values.yaml`, a ConfigMap, or its NOTES
output:

- `config.oidc.existingSecret` / `existingSecretKey` (default
  `client-secret`) - the OIDC client secret.
- `config.session.existingSecretEncryptionKey` /
  `existingSecretEncryptionKeyKey` (default `encryption-key`) - a
  32-byte AES-256-GCM key, base64-encoded. Generate one with:

  ```sh
  kubectl create secret generic tenantdeck-session \
    --from-literal=encryption-key="$(openssl rand -base64 32)"
  ```

**Key rotation procedure.** There is no in-place key-versioning in
`internal/session` today - rotating the encryption key invalidates every
existing session's ability to be *decrypted* (the old ciphertext can't be
unsealed with a new key). The only safe rotation procedure right now is:
generate a new key, update the Secret, roll the Deployment - every active
user is logged out (their session record becomes unreadable, so
`requireSession` treats it the same as a missing/expired one - fails
closed, not open) and must log in again. This is the cost of AES-256-GCM
with a single key rather than a key-ID-tagged scheme; revisit if rotation
without mass logout becomes a real requirement.

## Session revocation procedure

To forcibly invalidate a specific user's session (e.g. a compromised
account) without rotating the shared encryption key for everyone: delete
that session's key directly from Valkey/Redis -
`session:<the-opaque-session-id>` - which is exactly what
`session.Store.DeleteSession` does on a normal logout. There is currently
no admin-facing TenantDeck endpoint to look up which Redis key belongs to
a given subject (sessions are keyed by a random ID, not by subject) - an
operator needs either Redis-side tooling to scan session values (they're
encrypted; this only reveals *that* a session exists and its TTL, not its
subject, without the encryption key) or to revoke at the IdP
(de-authorizing the client/session there, which stops a *new* login, but
does not retroactively invalidate a TenantDeck session already issued -
revisit once refresh-token handling exists and a revocation check can be
added to `requireSession`).

## NetworkPolicy setup

`networkPolicy.enabled: true` by default, and an **empty** allow-list
means deny, not allow - `networkPolicy.ingress.from: []` (the chart
default) makes the Pod unreachable by anything until you add a peer; each
empty `networkPolicy.egress.*.to: []` (idp/capsuleProxy/sessionStore)
means that outbound path is blocked until configured. Only
`networkPolicy.egress.dns` is populated by default (CoreDNS, by its
standard `k8s-app: kube-dns` label) because every other egress rule needs
DNS to resolve *something* first.

Fill in, at minimum, for production:

```yaml
networkPolicy:
  ingress:
    from:
      - namespaceSelector:
          matchLabels:
            kubernetes.io/metadata.name: ingress-nginx
  egress:
    idp:
      to: [{ ipBlock: { cidr: <your IdP's CIDR>/32 } }]
    capsuleProxy:
      to: [{ ipBlock: { cidr: <capsule-proxy's CIDR>/32 } }]
    sessionStore:
      to: [{ ipBlock: { cidr: <your Valkey/Redis CIDR>/32 } }]
```

**Limitations to know before relying on this**: standard Kubernetes
NetworkPolicy matches IP addresses/selectors, never hostnames - there is
no way to write "allow egress to idp.example.com" directly. If your IdP,
Capsule Proxy, or session store's IP changes (DNS rotation, a cloud
load balancer rotating addresses), the `ipBlock` CIDRs above go stale
silently - NetworkPolicy will keep enforcing the *old* address, not the
new one, and nothing will warn you. Pin a CIDR as wide as your
infrastructure can tolerate, or use the node/cluster CNI's own
FQDN-aware policy extension if it has one (e.g. Cilium's
`toFQDNs` - not supported by the plain `networking.k8s.io/v1`
NetworkPolicy this chart renders, and not available unless your cluster
actually runs Cilium). Also: a *rendered* NetworkPolicy is not enforcement
- kindnet (kind's default CNI) does not enforce NetworkPolicy at all; see
`e2e/README.md` and `docs/implementation-plan.md` Phase 4 for why the
E2E stack installs Calico instead.

## Compatibility and upgrade notes

Targets Kubernetes 1.36.x and Capsule v0.14.5 / capsule-proxy v0.14.1
(ADR-006, `docs/implementation-plan.md`) - **unverified against a real
cluster in this session**, see that doc's "Known blockers." `e2e/up.sh`'s
`NODE_IMAGE` defaults to `kindest/node:v1.36.1` but is overridable; note
in its own comments that any Kubernetes 1.37+ kubelet hard-refuses to
start on a cgroup v1 Docker host at all (a real finding from this
session, not a hypothetical) - a cgroup v2 host (most current CI runners
and development machines) isn't affected.

## Health/readiness semantics

- `GET /healthz` (liveness) - always `200` if the process is serving
  HTTP at all. No downstream check - a liveness probe that depends on
  Valkey/Redis availability causes pointless restarts when only the
  store, not the process, is unhealthy.
- `GET /readyz` (readiness) - `200` if the session store responds to a
  `PING` within 2 seconds, `503` otherwise
  (`internal/httpapi/health.go`). Does not depend on the IdP or Capsule
  Proxy being reachable - per `docs/spec/05-upstream-boundary-and-resilience.md`,
  those failures are handled per-request (a 401/502/503 from the specific
  route that needed them), not by taking the whole replica out of
  rotation.
- Neither endpoint requires a session or touches `requireSession`.

## Logs and metrics an operator will actually see

Structured logging via the standard library's `log/slog`
(`slog.Info`/`slog.ErrorContext` throughout `internal/auth`,
`internal/httpapi`, `cmd/tenantdeck`) - JSON-shaped key/value fields
(`"error"`, `"addr"`, etc.), not formatted prose. No correlation/request
ID is generated or propagated yet - there is no `X-Request-ID`-style
middleware in `internal/httpapi` today; an operator correlates by
timestamp and the logged fields only. No metrics endpoint
(`/metrics` or otherwise) exists yet - this is a real gap against a
"production-ready" bar, not an oversight being hidden: revisit once
there's an actual operational need driving which metrics matter, rather
than exporting everything speculatively.

Token material is never logged: error paths log `err.Error()` values,
which as of this writing don't embed token contents, but nothing
currently *asserts* this holds as the code grows (tracked as `missing` in
`docs/security-test-matrix.md` row 15) - don't take "it doesn't today" as
a permanent guarantee without that test existing.

## Vulnerability policy

`verify.yml`'s `docker` job runs a Trivy image scan and its `helm` job
runs a Trivy IaC/config scan, both `exit-code: 1` on any `HIGH`/`CRITICAL`
finding - a failing scan fails the workflow, it is not advisory-only.
There is no ignore-file or suppression list in this repository yet
(`.trivyignore` doesn't exist) - per `docs/spec/08-github-actions-and-supply-chain.md`,
any exception must be narrowly scoped to the specific CVE/path, justified
in the exception itself, and time-bounded (re-evaluated, not left
indefinitely) - add it as `.trivyignore` entries with a comment citing the
CVE, the reason it doesn't apply or can't be fixed yet, and a revisit
date, never a blanket `--severity` downgrade to reach green.
`govulncheck` (the `go` job) has the same bar: a finding fails the job,
full stop - Go's module system makes "can't upgrade yet" rare enough that
there's no documented exception process for it at all yet.

## Repository protections (an owner's one-time setup)

These aren't set by any workflow file - a repository owner configures
them once in GitHub's settings:

- **Branch protection on `main`**: require `verify`'s jobs (and,
  eventually, `make e2e`'s job) to pass before merge; require PRs, no
  direct pushes.
- **`GITLEAKS_LICENSE` secret**: only needed if this repository is
  private - gitleaks-action is free for public repositories. Add it as
  an organization or repository secret if/when this repo goes private.
- **GHCR package visibility**: `release.yml` pushes to
  `ghcr.io/<owner>/tenantdeck` and `ghcr.io/<owner>/charts/tenantdeck` -
  an owner decides whether those packages are public or private
  independently of the source repository's own visibility (GHCR defaults
  new packages to private).
- **Environment protection for the `release` job** (optional but
  recommended once this matters for real): a GitHub Environment named
  e.g. `release` with required reviewers, so a `v*` tag push doesn't
  publish without a human's sign-off, even though the workflow already
  restricts `packages`/`id-token: write` to that one job.

## Releasing

`release.yml` triggers only on a `v*.*.*` tag push (never a PR, never a
plain branch push) and builds the image, pushes it plus an OCI Helm chart
to GHCR, generates an SPDX SBOM, and signs the image plus attests the SBOM
via keyless cosign (GitHub OIDC - no signing key stored anywhere). **This
has been authored and packaging-tested locally only** (`docker build`,
`helm package` both run for real in this session) - there has been no
authorized run of this workflow against a real tag in this repository.
Do not claim a release "worked" until one has actually happened.

To verify a published release once one exists:

```sh
cosign verify ghcr.io/<owner>/tenantdeck:<version> \
  --certificate-identity-regexp 'https://github.com/<owner>/TenantDeck/.github/workflows/release.yml@.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com

helm pull oci://ghcr.io/<owner>/charts/tenantdeck --version <version>
```

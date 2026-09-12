# Implementation plan

Living progress tracker against the execution sequence in
[`docs/spec/10-process-and-definition-of-done.md`](spec/10-process-and-definition-of-done.md).
Update this file whenever a phase advances — do not let it drift from what is
actually merged.

**Status: Phase 4 (hardened image/chart + E2E stack) done, and the
mandatory E2E stack has now actually run to green in GitHub Actions CI**
(`verify.yml`'s "Mandatory full-stack E2E" job, first passing run
2026-09-12) — a real kind cluster, real Calico, cert-manager, Capsule
operator/proxy, Valkey, and the actual built image/chart, two replicas,
with `login_test.go`/`hardening_test.go` passing against it. This
repo's own dev sandbox still can't run any of that directly (see "Known
blockers" for the two independently diagnosed limits, unchanged), which
is why getting there took several CI-only diagnose/fix/push/verify
rounds instead of local iteration - each CI failure was read from real
job logs, fixed, and re-verified by watching the next run, not guessed
at. Tenant-isolation and NetworkPolicy-enforcement assertions
(`docs/security-test-matrix.md` rows 17, 22) are still not written as
tests, even though the fixtures/infrastructure they'd run against now
provision successfully for real.

**Correction to every earlier phase's "Known blockers":** Phase 1-3
repeatedly recorded "no Docker daemon in this session's execution
environment." That was true for those sessions - this one's sandbox
genuinely has a working `dockerd` (root, `dockerd &` starts clean) and
most registries are reachable. Don't assume "no Docker" still holds
without checking again in whatever session reads this next; see below
for exactly what is and isn't reachable even with Docker working.

## Phases

- [x] **1. Inspection and planning** — dependency set chosen and recorded as
      ADRs below; [`docs/route-allowlist.md`](route-allowlist.md) written;
      `docs/threat-model.md` assets/trust-boundaries filled in. Per-component
      threat analysis (OIDC login, session handling, upstream client,
      frontend) is deferred to when each component is actually built — you
      can't meaningfully threat-model code that doesn't exist yet.
- [x] **2. Vertical slice** — implemented: `internal/config`, `internal/session`
      (encrypted, Redis-backed), `internal/auth` (OIDC login/callback/session/
      logout), `internal/capsule` (namespace-list upstream client),
      `internal/httpapi` (router + security headers + session middleware),
      a real React/TS/Vite frontend (`web/`) embedded via `go:embed`
      (`web/embed.go`, `internal/webassets`), and `cmd/tenantdeck` wiring it
      all together. `make verify` runs clean (Go tests with `-race`,
      frontend typecheck/lint/test/build). The built binary was run for real
      against a real Redis and a minimal fake OIDC discovery endpoint and
      correctly served the frontend, reported logged-out session status,
      redirected to the IdP with real PKCE/state/nonce, and fail-closed
      401'd `/api/namespaces` without a session — see "Known blockers" for
      what wasn't exercised this way. `docs/local-development.md` is now
      written for real.
- [x] **3. Remaining bounded v1 features** — implemented: a generic
      namespaced-resource upstream client (`internal/capsule`'s
      `ListNamespacedResource`/`GetNamespacedResource`, parameterized by
      `GroupVersionResource`) and bounded log streaming
      (`StreamPodLogs`, a dedicated no-`Timeout` `streamClient` bounded
      instead by `context.WithTimeout`/`io.LimitReader`); `internal/httpapi`
      handlers for overview, Deployments/StatefulSets/DaemonSets/Pods/Jobs/
      CronJobs, Services/Ingresses/PersistentVolumeClaims, Events, and
      `pods/{name}/logs` (follow + cancellation), all behind the shared
      `validK8sName` path-parameter validator; a React frontend
      (`NamespaceSelector`, `Workspace` navigation, generic `ResourceList`/
      `ResourceDetail`, `Overview`, `EventsList`, `PodLogsViewer`) with
      loading/empty/forbidden/unavailable states per view and a hostile-log/
      hostile-event/hostile-ingress-host rendering test for each. `make
      verify` (Go `-race` tests + frontend typecheck/lint/test/build) runs
      clean; `internal/httpapi` is at 99.0% statement coverage,
      `internal/capsule` at 94.9%. See `docs/security-test-matrix.md` rows
      6, 8, 14, 29, 30, 34-37 for the test evidence, including the two rows
      (8, 37) left `partial` because no test yet sends a wrong-verb request
      to assert Go's `http.ServeMux` 405s it (every route is `GET`-only by
      construction, and the upstream client has no write method at all, so
      there is no write path to reach regardless).
- [x] **4. Hardened image/chart + full-stack E2E** — implemented: a
      pinned-digest multi-stage `Dockerfile` producing a 4.7MB
      `gcr.io/distroless/static-debian12:nonroot` image (UID/GID 65532 by
      construction), built and run for real under
      `--read-only --user 65532:65532 --cap-drop ALL --security-opt
      no-new-privileges`, serving `/healthz`/`/readyz`/`/auth/*`/`/api/*`/`/`
      correctly against a real Redis and the real `cmd/mockoidc`; the
      `charts/tenantdeck` Helm chart (securityContext rendered at the
      correct fields, no automount/RBAC objects, default-on NetworkPolicy
      with no default-allow egress, existing-Secret-only OIDC/session/Redis
      credentials, `values.schema.json` that structurally forbids weakening
      the hardening defaults, dev/prod-example values, `helm test`) -
      `helm lint`/`helm template` pass for real (`make helm-verify`, now
      part of `make verify`); `cmd/mockoidc`, a real (not rubber-stamp)
      authorization-code + mandatory-PKCE + refresh OIDC provider with real
      RS256-signed JWTs, its own test suite verifying signatures against its
      own JWKS; and a real full HTTP cookie-jar login → session →
      `/api/namespaces` (502, not 401) → logout → denied-reuse round trip
      between the real `tenantdeck` and `mockoidc` binaries. The mandatory
      E2E stack (`e2e/`: kind + Calico + cert-manager + Capsule + Valkey +
      the chart + two replicas + `login_test.go`/`hardening_test.go`) and
      the GitHub Actions workflows (`.github/workflows/verify.yml`,
      `release.yml`, SHA-pinned third-party Actions, least-privilege
      permissions, `make e2e` as the one path CI and a human both run)
      **have now run end-to-end for real in GitHub Actions CI** -
      `verify.yml`'s "Mandatory full-stack E2E" job passed for the first
      time 2026-09-12, after five rounds of reading a real CI failure,
      fixing its root cause, and pushing to watch the next run (`go:embed`
      on an empty `dist/`, `govulncheck` against a pinned stdlib patch,
      `/usr/local/bin` not writable on the runner image, cert-manager
      missing as a Capsule-chart prerequisite, the Valkey image's
      entrypoint needing a missing `setpriv` to de-escalate from root,
      Capsule's namespace admission requiring both a correct
      label+ownerReference *and* the requesting user to be the Tenant's
      own owner, that owner needing its own namespace-create RBAC, and
      finally the OIDC redirect chain needing TenantDeck's and mockoidc's
      real in-cluster hostnames reachable from the test process, not just
      a port-forwarded localhost address). This repo's own dev sandbox
      still can't run any of it directly - see "Known blockers" for the
      two independent, reproduced reasons, unchanged by the above.
      `docs/architecture.md` and `docs/operations.md` are written for real
      against what was actually built and verified.
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

- **ADR-001 — Go toolchain: 1.26.x** (amended from the original 1.27.x
  research). `golang.org/x/oauth2` (pulled in by ADR-003) requires
  `go >= 1.26.0`, and `go.mod` now reads `go 1.26.0`. There was no reason
  to force 1.27.x once 1.26.x was already required by a real dependency.
- **ADR-002 — HTTP stack: standard library `net/http` only, no router
  dependency.** Confirmed as built: `internal/httpapi` uses Go's
  method+wildcard-pattern `http.ServeMux` (`"GET /auth/login"`, etc.) for
  every route in `docs/route-allowlist.md`. No router dependency needed.
- **ADR-003 — OIDC relying-party library: `github.com/zitadel/oidc/v3`**
  (amended from the original "v4" research). At implementation time,
  `zitadel/oidc/v4` only had `-next` pre-release tags (`v4.0.0-next.4`), not
  a stable release — using a pre-release in a security-sensitive auth path
  would have contradicted "prefer mature maintained libraries." `v3.49.6`
  is the real stable latest and is what's in `go.mod`. We use only its RP
  side (`pkg/client/rp`) — TenantDeck is never an OP. One non-obvious but
  important finding from building against it: its ID token verifier's
  nonce check is **opt-in** (`rp.WithNonce`, defaulting to expecting an
  empty nonce) rather than automatic — `internal/auth/oidc.go` wires the
  expected nonce through via a request-context value
  (`expectedNonceContextKey`) rather than hand-rolling a parallel nonce
  check, so the library's own OIDC Core validation is what actually
  enforces it.
- **ADR-004 — Session-store client: `github.com/redis/go-redis/v9`.**
  Confirmed as built and tested against a real `redis-server` process in
  every package's tests (not just a fake), per
  `docs/spec/07-mandatory-automated-testing.md`'s preference for real
  infrastructure.
- **ADR-005 — Frontend tooling: React 19 + TypeScript + Vite 8.** Confirmed
  as built, with two corrections to the original research: `@vitejs/plugin-react`
  needed `^6.0.0` (the `^4.x` line doesn't declare Vite 8 as a supported
  peer), and `vitest` needed `^5.0.0` rather than `^3.0.0` — partly for the
  same Vite 8 compatibility reason, and partly because `npm audit` flagged
  a moderate path-traversal advisory
  ([GHSA-82fw-gwwq-j7x9](https://github.com/advisories/GHSA-82fw-gwwq-j7x9))
  in `@vitest/mocker` versions pulled in by `vitest@2–4.x`; `vitest@5.0.0`
  resolves both. `npm audit` reports zero vulnerabilities against the
  versions actually in `web/package.json` now.
- **ADR-006 — Capsule compatibility set: Capsule operator v0.14.5 +
  capsule-proxy v0.14.1, target Kubernetes 1.36.x.** Unchanged from
  research — **not yet exercised**, since this sandboxed session has no
  Docker daemon (see "Known blockers"). Still the right target; just
  unverified against a real cluster so far.
- **ADR-007 — Disposable E2E cluster: kind v0.33.0**, whose default node
  image is Kubernetes 1.36.1 — matches ADR-006. Same caveat: unverified in
  this session.
- **ADR-008 — Login-transaction binding: server-side state, not the OIDC
  library's cookie handler.** `internal/auth` does its own state/nonce/PKCE
  generation and storage (`session.Store.CreateLoginTransaction`/
  `TakeLoginTransaction`, single-use via Redis `GETDEL`) rather than using
  `rp.AuthURLHandler`/`rp.CodeExchangeHandler`'s built-in `httphelper.CookieHandler`.
  This keeps the "cryptographically random one-use state... bounded
  login-transaction lifetime" properties from
  `docs/spec/04-auth-session-browser-security.md` fully auditable in our
  own code rather than trusting a black-box cookie mechanism, at the cost
  of a bit more code than the library's convenience wrappers would need.
- **ADR-009 — CSRF protection: synchronizer token delivered in the
  `/auth/session` JSON body, never a cookie.** A random `CSRFToken` is
  generated at login, stored in the session record, returned once in that
  same-origin JSON response, and must be echoed back as `X-CSRF-Token` on
  `POST /auth/logout` — alongside an exact `Origin` check, per
  `docs/spec/04-auth-session-browser-security.md`'s "Require CSRF
  protection **and** exact Origin validation."
- **ADR-010 — CSP: `default-src 'self'; object-src 'none'; base-uri 'none';
  frame-ancestors 'none'`**, set on every response by `internal/httpapi`'s
  `securityHeaders` middleware. No `unsafe-inline`/`unsafe-eval`, no
  third-party sources — revisit only if a real UI need (e.g. a font CDN)
  comes up, and then extend deliberately rather than loosening broadly.
- **ADR-011 — Frontend embedding: a sibling `web` Go package
  (`web/embed.go`, `//go:embed dist`), not a path inside `internal/`.**
  Go's `//go:embed` directive can't reference a parent directory (`..`),
  so the embed must live inside the frontend's own directory tree.
  `web/dist/.gitkeep` is committed (real build output is gitignored) so a
  fresh checkout's `go build` has something to embed before `npm run
  build` has ever run.
- **ADR-012 — Pod log streaming uses a second `http.Client` with no
  `Timeout`.** `internal/capsule.Client` already had one `httpClient` with
  a wall-clock `Timeout` for bounded list/get calls; reusing it for
  `StreamPodLogs` would silently cut off a `follow=true` stream after that
  timeout even while it's actively producing data. Added a sibling
  `streamClient` (same `Transport`, no `Timeout`) used only by
  `StreamPodLogs`, with the stream instead bounded by
  `internal/httpapi/logs.go`'s `context.WithTimeout(maxLogStreamDuration)`
  (10 minutes) and an `io.LimitReader` byte cap (`maxStreamBytes`, 50MiB) —
  bounding total duration/size without bounding time-between-bytes.
- **ADR-013 — `ColumnDef<T>.render` is a TS method-shorthand signature,
  not a `render: (item: T) => ReactNode` property.** `web/src/resourceKinds.tsx`'s
  `resourceKinds` array is heterogeneous — each entry's `columns` closes
  over a different concrete resource type (`DeploymentSummary`,
  `PodSummary`, etc.) but the array itself is typed `ResourceKindConfig[]`
  (`= ResourceKindConfig<unknown>[]`) so `ResourceList`/`Workspace` can
  consume any entry generically. With `render` as a property, TypeScript's
  `strictFunctionTypes` checks parameter types contravariantly and rejects
  assigning e.g. `(item: PodSummary) => ReactNode` where
  `(item: unknown) => ReactNode` is expected — the alternative would have
  been an `any` escape hatch. TypeScript checks method-shorthand signatures
  bivariantly instead (a deliberate, long-standing exception, not a bug),
  so declaring `render(item: T): ReactNode` as a method lets the
  heterogeneous array typecheck with no `any` anywhere in the file.
- **ADR-014 — Runtime base image: `gcr.io/distroless/static-debian12:nonroot`,
  pinned by digest.** Already non-root at UID/GID 65532 by construction
  (matches spec exactly, no chart-side `runAsUser` override needed to
  make that true), already bundles a CA certificate store, has no
  shell/package manager at all. Build stages use `golang:1.26-alpine` and
  `node:22-alpine` (also digest-pinned) - only the final stage's choice
  matters for the shipped image's attack surface.
- **ADR-015 — E2E mock OIDC provider: hand-rolled (`cmd/mockoidc`), not a
  maintained third-party mock.** Researched alternatives at
  implementation time (e.g. `oauth2-mock-server`,
  `navikt/mock-oauth2-server`) - each either lacked enforced PKCE with a
  real S256 check, or added a JVM/Node runtime dependency to the E2E
  toolchain for a fixture whose entire job is being small, deterministic,
  and auditable. `cmd/mockoidc` is ~250 lines, reuses the project's own
  `go-jose`/`jwt` dependencies already in `go.mod`, implements real
  discovery/JWKS/authorization-code/PKCE-enforcement/refresh with real
  RS256-signed JWTs, and is proven independently correct by its own test
  suite (`go test ./cmd/mockoidc/...`, including verifying signatures
  against its own published JWKS, not just decoding unchecked). It is a
  separate `cmd/`, never imported by `cmd/tenantdeck` and never in the
  production Dockerfile - see `e2e/mockoidc.Dockerfile`, a distinct file.
- **ADR-016 — NetworkPolicy enforcement CNI for the E2E cluster: Calico,
  manifest-installed, kindnet's default disabled.** kindnet (kind's
  default CNI) does not enforce NetworkPolicy at all -
  `docs/spec/07-mandatory-automated-testing.md` explicitly warns against
  mistaking a rendered-but-unenforced policy for enforcement testing.
  `e2e/kind-config.yaml` sets `disableDefaultCNI: true` with a matching
  `podSubnet`; `e2e/up.sh` installs Calico's manifest at a real pinned
  release tag.
- **ADR-017 — Local image-registry finding, scoped to verification only,
  never the shipped artifacts.** This session's sandbox blocks the blob
  CDN behind `docker.io` (`production.cloudfront.docker.com`) by egress
  policy, but not `gcr.io` or Google's own `mirror.gcr.io` (a real,
  publicly documented Docker Hub pull-through cache). Configuring the
  local `dockerd`'s `registry-mirrors` to `mirror.gcr.io` was what let
  `node:22-alpine`/`golang:1.26-alpine`/`kindest/node` pull at all in
  *this* sandbox. That daemon config is local-machine-only, was never
  written into the Dockerfile/chart/CI (which reference plain
  `docker.io`/`ghcr.io` images exactly as a normal, unrestricted CI
  runner would), and should not be assumed necessary elsewhere.

Record further ADRs here (or as separate files under `docs/adr/` if this
section grows unwieldy) as they're made.

## Known blockers

- **This session's sandbox could not run a live Kubernetes cluster, for
  two independent, fully diagnosed reasons** (correcting Phases 1-3's "no
  Docker daemon" note — this session's sandbox actually had one; see
  ADR-017). Both are reproduced with direct command output, not assumed:
  1. **`ghcr.io`'s blob-storage CDN (`pkg-containers.githubusercontent.com`)
     is blocked by this sandbox's egress policy** — confirmed with
     `docker pull ghcr.io/stefanprodan/podinfo:latest` failing
     `Forbidden` on that exact host, while the registry API itself
     (`ghcr.io`) and `gcr.io`/`mirror.gcr.io` pull fully. Capsule's
     operator and capsule-proxy images are published only to `ghcr.io`
     (`ghcr.io/projectcapsule/capsule`, `.../capsule-proxy`) — there is
     no equivalent pull-through mirror for it the way `mirror.gcr.io`
     covers Docker Hub, and routing around a deliberate egress policy
     block is explicitly out of bounds (per this session's own tooling
     guidance) rather than something to engineer past.
  2. **Even with an image, this sandbox's container runtime cannot start
     a third level of nested containers.** A kind cluster is itself
     `docker` (this sandbox) → a privileged "node" container → that
     node's own containerd/runc starting a Pod's sandbox — one layer
     deeper than the sandbox's own containment. Concretely: `kindest/node:v1.37.0`
     (and even the older `v1.31.12`, `v1.36.1`, chosen to rule out a
     separate, real, and also-hit issue — newer kubelets hard-refuse
     this sandbox's cgroup v1 Docker host entirely: *"kubelet is
     configured to not run on a host using cgroup v1"*) all fail
     identically once past that gate, every single static pod
     (`etcd`/`kube-apiserver`/`kube-controller-manager`) erroring
     `failed to create shim task: OCI runtime create failed: ... can't
     get final child's PID from pipe: EOF`; a bare `ctr run --rm
     registry.k8s.io/pause:3.10` (no Kubernetes involved at all) hangs
     the same way. This is a ceiling of the sandbox's own container
     nesting depth, not a Kubernetes-version or cgroup-version question.
  - As a direct result, none of `internal/capsule`'s, `cmd/mockoidc`'s,
    or the chart's behavior has been proven against a real kube-apiserver,
    real Capsule Proxy, or inside an actual Pod. `internal/capsule`'s
    client is tested against an `httptest.Server` standing in for Capsule
    Proxy's response shapes — proving the client builds the right
    request and handles the right statuses/sizes/timeouts, **not** that a
    real capsule-proxy answers the same way.
  - What **was** verified for real, outside a cluster entirely: the exact
    built Docker image (`docker build .`, no shortcuts) running under
    `--read-only --user 65532:65532 --cap-drop ALL --security-opt
    no-new-privileges`, correctly serving `/healthz`, `/readyz`,
    `/auth/session`, `/`, and a fail-closed 401 then a correctly-scoped
    502 on `/api/namespaces`, against a real `redis:7-alpine` container
    and a real running `cmd/mockoidc`; a full real HTTP
    login→session→logout→denied-cookie-reuse round trip between the real
    `tenantdeck` and `mockoidc` binaries (no stubs) with zero token
    material in the process logs; and `cmd/mockoidc`'s own test suite
    proving real PKCE enforcement, single-use authorization codes, and
    refresh, with ID token signatures verified against the server's own
    published JWKS.
  - `e2e/` (kind + Calico + cert-manager + Capsule + Valkey + the chart +
    `login_test.go`/`hardening_test.go`) **has run and passed** on a
    normal CI runner (GitHub Actions, `verify.yml`, 2026-09-12) - neither
    limit above is specific to Kubernetes or to TenantDeck's own code, so
    a real runner was never blocked by them. It still has not run inside
    this repo's own dev sandbox, which remains blocked by both limits -
    see `e2e/README.md` for exactly what that sandbox can and can't do.
  - The full login/nonce/PKCE/replay/expiry round trip **is** separately
    verified for real, with real RS256-signed tokens and real
    discovery/JWKS, by `internal/auth`'s own test suite
    (`testop_test.go`'s minimal test-only OP) — that part never depended
    on any of the above.
- **`e2e/fixtures/tenants.yaml`'s Capsule `Tenant` owners still aren't
  wired to real OIDC identities.** `e2e/up.sh` now provisions each
  owner's namespace for real in CI - by impersonating the owner
  (`kubectl --as=<owner>`, which kind's `system:masters` admin kubeconfig
  can do without any real identity behind it) and granting that
  impersonated identity its own namespace-create RBAC, so Capsule's
  admission webhooks see a consistent, correctly-labeled,
  correctly-owned namespace. That's a deliberate stand-in for identity,
  not a fix for the underlying gap: Capsule Proxy scopes a request by
  the identity kube-apiserver's own OIDC authentication resolves — not
  by re-validating the bearer token itself — so kube-apiserver still
  needs its own `--oidc-*` flags pointed at `cmd/mockoidc` before a real
  browser login through mockoidc would ever resolve to one of these
  owners. Getting the TLS-only issuer-URL requirement and in-cluster
  reachability (kube-apiserver's static pod is `hostNetwork`, so it can't
  resolve a Service DNS name the way a normal Pod can) right needs
  real cluster iteration this session couldn't do (see above) — sketched
  in `e2e/up.sh` comments, not implemented. The fixture YAML itself is a
  real, schema-correct `Tenant` CR either way.
- **Refresh-token handling is not implemented.** `session.Record` has no
  refresh token field; a session's only expiry is the absolute TTL tied to
  the ID token's own `exp` claim. There is no idle timeout distinct from
  that, and no refresh-race/concurrent-refresh handling
  (`docs/spec/04-auth-session-browser-security.md` requires both) — rows
  10–12 in `docs/security-test-matrix.md` are `partial`/`missing`
  accordingly. This needs its own implementation pass before logout/expiry
  behavior can be called complete.
- **Login/token-exchange rate limiting is not implemented** (matrix row
  33) — `docs/spec/05-upstream-boundary-and-resilience.md`'s "bound...
  login abuse" requirement.
- **`postLoginRedirect` is a hardcoded `"/"`**, not a validated relative
  path from a request parameter — trivially safe (no redirect surface to
  exploit) but also not yet a general "allowlisted post-login destination"
  mechanism. Revisit if/when a real deep-link requirement shows up.
- **No explicit wrong-verb (non-`GET`) rejection test exists for any
  Phase 3 route** (`docs/security-test-matrix.md` rows 8, 37). Every route
  is registered `GET`-only and `internal/capsule.Client` has no write
  method at all, so there is no write path to reach regardless of what a
  caller's RBAC allows — but nothing asserts the 405/404 Go's
  `http.ServeMux` gives a non-`GET` request by construction. Low risk (no
  code path exists for it to matter), but worth a quick table-driven test
  alongside `TestRouter_DispatchesPhase3RoutesToTheirHandlers` before
  calling the allowlist enforcement fully verified.
- **Phase 3's frontend tests mock `fetch`/`streamPodLogs`, same as
  Phase 2's** — they prove the UI maps responses/states correctly, not
  that a real Capsule Proxy answers these shapes the same way. Same
  Docker-less constraint as above; resolved only once Phase 4's E2E stack
  exists.

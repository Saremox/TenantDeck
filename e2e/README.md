# Mandatory E2E stack

What `make e2e` runs, per [`docs/spec/07-mandatory-automated-testing.md`](../docs/spec/07-mandatory-automated-testing.md).

## What this brings up

1. A disposable kind cluster (`kind-config.yaml`), with the default CNI
   disabled and replaced by a real Calico install (`up.sh`) - kindnet,
   kind's default, does not enforce NetworkPolicy at all, so it can't be
   used to test allowed/disallowed connections for real.
2. Real Capsule operator + capsule-proxy, pinned per ADR-006
   (`docs/implementation-plan.md`).
3. The deterministic mock OIDC provider (`../cmd/mockoidc`) - real
   discovery, JWKS, authorization-code + enforced PKCE, real RS256-signed
   JWTs, refresh support. Never linked into `cmd/tenantdeck`.
4. Valkey (test infrastructure provisioning its own store, per
   `docs/route-allowlist.md`).
5. The actual built TenantDeck image, deployed via the actual chart, two
   replicas, in a namespace labeled for the Restricted Pod Security
   Standard.
6. Two Capsule `Tenant` fixtures with distinct owners (`fixtures/tenants.yaml`),
   matching mockoidc's `login_hint`-selectable identities.

`run-tests.sh` port-forwards the TenantDeck Service and runs `go test
-tags e2e ./e2e/...` (`login_test.go`, `hardening_test.go`) against it -
a real HTTP cookie-jar login/logout round trip and a check of the
*actually running* Pod spec (no SA token volume, read-only root
filesystem), not just the chart's rendered YAML.

## What has and hasn't actually been run

**Authored and reasoned through carefully; not executed end-to-end in
this repository's own development session.** Two independent,
fully-diagnosed environment limits of that specific sandbox blocked it -
see `docs/implementation-plan.md` Phase 4 "Known blockers" for the exact
evidence:

1. This sandbox's egress policy blocks the blob-storage CDN behind
   `ghcr.io` (where Capsule's images are published) - confirmed directly,
   not assumed.
2. Even working around (1) wouldn't have been enough: this sandbox's
   container runtime cannot start a third level of nested containers
   (`docker` → kind's privileged node container → that node's own
   containerd/runc trying to start a Pod's sandbox) - confirmed with
   `runc create failed: ... can't get final child's PID from pipe: EOF`
   on a plain `ctr run`, independent of Kubernetes version or the
   cgroup v1/v2 kubelet requirement also hit along the way.

What *was* verified for real in that session, outside a Kubernetes
cluster entirely: the built Docker image running under
`--read-only --user 65532:65532 --cap-drop ALL --security-opt
no-new-privileges`, serving `/healthz`/`/readyz`/`/auth/session`/`/`/`/api/namespaces`
correctly; `cmd/mockoidc`'s own real PKCE-enforcing authorization-code +
refresh flow (`go test ./cmd/mockoidc/...`); and a full real HTTP
cookie-jar login → session → logout → denied-cookie-reuse round trip
between the real `tenantdeck` and `mockoidc` binaries and a real
`redis-server`, with zero token material appearing in the process logs.

Run this on a normal GitHub Actions runner or a developer machine with a
cgroup v2 Docker host (most are) and it should actually complete - that
combination doesn't hit either limit above. If it doesn't, fix forward
rather than assuming the harness itself is untrustworthy; file what broke
against the specific step in `up.sh`.

## Running it yourself

```sh
make e2e       # up.sh, then run-tests.sh, always down.sh after (success or failure)
make e2e-up    # just bring the stack up, for manual poking
make e2e-down  # tear it down
```

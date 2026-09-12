# Local development guide

**Status: real commands below, verified in this session** (2026-09-12) by
actually running them — `make verify` passing, the binary built and booted
against a real Redis and serving the real frontend, `/auth/login`
redirecting with real PKCE/state/nonce. **Not yet verified**: a full login
against a real IdP, and anything involving Capsule/Capsule Proxy/kind — see
"What you can't fully exercise locally yet" below and
`docs/implementation-plan.md`'s "Known blockers."

## Prerequisites

Versions actually used when this was last verified — re-check
`docs/implementation-plan.md`'s ADRs before assuming these are still
current if much time has passed.

- **Go** ≥ 1.26 (`go.mod` pins `go 1.26.0`; `go version` to check).
- **Node** ≥ 22, **npm** ≥ 10 (`web/package.json`'s devDependencies assume
  this; older Node may fail to run Vite 8/Vitest 5).
- **A Valkey- or Redis-compatible server** reachable from the BFF. Any real
  `redis-server`/`valkey-server` works for local dev — there's no bundled
  database and none should be added (`docs/spec/06-container-and-kubernetes-deployment.md`).
- **kind, Capsule, capsule-proxy, Docker**: only needed once you're working
  on Phase 4 (the mandatory E2E stack) or testing against a real upstream —
  not required for the steps below.

## Running the BFF locally

1. Build the frontend once (re-run after changing anything in `web/src`):

   ```sh
   cd web && npm install && npm run build && cd ..
   ```

   This produces `web/dist/`, which `web/embed.go` embeds into the Go
   binary via `go:embed`. Skipping this step leaves the committed
   `web/dist/.gitkeep` placeholder embedded instead, and the app will
   serve an empty shell.

2. Start a local Redis/Valkey:

   ```sh
   redis-server --port 6379 --daemonize yes
   ```

3. Set the required environment variables. There are **no insecure
   defaults** (`internal/config`) — every one of these is required, and
   `TENANTDECK_INSECURE=true` is what relaxes the `https://`/`__Host-`
   cookie requirements for loopback dev specifically:

   ```sh
   export TENANTDECK_LISTEN_ADDR=":8080"
   export TENANTDECK_EXTERNAL_URL="http://127.0.0.1:8080"
   export TENANTDECK_INSECURE="true"
   export TENANTDECK_OIDC_ISSUER_URL="http://127.0.0.1:9090"   # see below
   export TENANTDECK_OIDC_CLIENT_ID="tenantdeck-dev"
   export TENANTDECK_OIDC_CLIENT_SECRET="dev-secret"
   export TENANTDECK_REDIS_ADDR="127.0.0.1:6379"
   export TENANTDECK_CAPSULE_PROXY_URL="http://127.0.0.1:9091"  # see below
   export TENANTDECK_SESSION_ENCRYPTION_KEY="$(head -c32 /dev/urandom | base64)"
   ```

4. Build and run:

   ```sh
   make build   # frontend build + go build -o bin/tenantdeck ./cmd/tenantdeck
   ./bin/tenantdeck
   # or, skipping the separate build step:
   make run
   ```

5. `curl http://127.0.0.1:8080/auth/session` should return
   `{"authenticated":false}`. `curl -i http://127.0.0.1:8080/` should serve
   the built frontend with the security headers
   (`Content-Security-Policy`, `Cache-Control: no-store`, etc.) set by
   `internal/httpapi`'s `securityHeaders` middleware.

## What you can't fully exercise locally yet

- **A real login.** `TENANTDECK_OIDC_ISSUER_URL` needs a reachable OIDC
  discovery document (`/.well-known/openid-configuration`) for the BFF to
  even start (`auth.NewHandler` does discovery eagerly). There is currently
  **no standalone, reusable mock OIDC provider binary in this repo** — the
  one that exists (`internal/auth/testop_test.go`) is test-only Go code
  that spins up inside `go test`, not something you can run interactively.
  For a real manual login locally, point `TENANTDECK_OIDC_ISSUER_URL` at an
  actual OIDC provider you control (a dev realm on Keycloak/Dex/your org's
  IdP, etc.), with a client registered for
  `http://127.0.0.1:8080/auth/callback`. Deciding on a reusable mock OIDC
  provider is explicitly deferred to Phase 4
  (`docs/implementation-plan.md` ADR notes) — until then, the full
  login→session→logout path is verified by `internal/auth`'s test suite
  (real signature/discovery/JWKS, just not interactively from a browser).
- **A real namespace list.** `TENANTDECK_CAPSULE_PROXY_URL` needs a real
  Capsule Proxy (or at least something answering `GET /api/v1/namespaces`
  the way it does) behind it. Without Docker/kind available, this hasn't
  been exercised against the real thing in this repository yet — see
  `docs/implementation-plan.md`'s "Known blockers."

## `make verify` and friends

- **`make verify`** — `gofmt` check, `go vet`, `go test -race` (scoped to
  `./cmd/... ./internal/...` — see the Makefile comment on why not bare
  `./...`), then frontend typecheck + lint + `vitest run` + build. This is
  what's described in `CLAUDE.md`'s Commands section; CI must call the
  same target once CI exists (Phase 4).
- **`make build`** — frontend build, then `go build -o bin/tenantdeck
  ./cmd/tenantdeck`.
- **`make run`** — `go run ./cmd/tenantdeck` (needs the env vars above
  already exported).
- **`make e2e`** doesn't exist yet — it's the mandatory full-stack suite
  from `docs/spec/07-mandatory-automated-testing.md`, which needs the kind/
  Capsule/mock-OIDC stack Phase 4 builds.
- **Running one package while iterating**: `go test ./internal/auth/...
  -run TestCallbackHandler -v`. Most packages' tests spin up a real
  `redis-server` subprocess automatically (they skip with a clear message
  if `redis-server` isn't on `PATH` — install it, e.g. `apt-get install
  redis-server`, rather than relying on the fallback skip).
- **Frontend only**: `cd web && npm run typecheck && npm run lint && npx
  vitest run`.

## Troubleshooting

- **`dial tcp ...: connect: connection refused` from Go tests or the
  running binary** — Redis/Valkey isn't running, or
  `TENANTDECK_REDIS_ADDR`/the test's expectations don't match where you
  started it.
- **Tests print `redis-server not found on PATH, skipping...` and are
  silently skipped** — install `redis-server` (or `valkey-server`,
  redis-protocol-compatible) to actually run those tests instead of
  skipping them.
- **`auth.NewHandler` / the binary fails to start with a discovery error**
  — `TENANTDECK_OIDC_ISSUER_URL` isn't serving a discovery document at
  `<issuer>/.well-known/openid-configuration` that echoes back the exact
  same issuer string, or isn't reachable at all. Discovery happens once,
  synchronously, at startup.
- **The served frontend looks like the "not built yet" placeholder** — you
  haven't run `npm run build` in `web/` since the last `go build`;
  `go:embed` only sees whatever was in `web/dist/` at Go compile time.
- **A 401 on `/api/namespaces` you didn't expect** — `requireSession`
  fails closed on *any* problem (missing cookie, expired/unknown session,
  or the store being unreachable) — check `TENANTDECK_REDIS_ADDR`
  connectivity before assuming the session itself is the problem.

# Local development guide

**Status: not started.** No build exists yet. This is a skeleton so the
filename and required sections exist before implementation starts; replace
each section with real, tested commands as they're built — a quickstart
that doesn't actually work from a clean checkout fails the definition of
done in
[`docs/spec/10-process-and-definition-of-done.md`](spec/10-process-and-definition-of-done.md).

## What this must eventually cover

- **Prerequisites** and exact versions (Go, Node, Helm, kind, Docker/
  equivalent) — pinned, not "latest."
- **Running the BFF locally** serving the embedded built frontend, including
  how local-only loopback exceptions to cookie/TLS rules are enabled and why
  they're isolated from the production fail-closed path (see
  `docs/spec/04-auth-session-browser-security.md`).
- **A local mock OIDC provider** for interactive manual testing (distinct
  from the E2E suite's mock — or the same one, reused, documented either
  way) — and an explicit note on what it does *not* exercise versus a real
  IdP (see `docs/spec/07-mandatory-automated-testing.md` on mock vs. real
  provider limitations).
- **A local disposable kind cluster** with Capsule/Capsule Proxy installed,
  and the explicit test kubeconfig/context convention
  (`docs/spec/01-working-agreement.md` — never a developer's default
  kubeconfig).
- **`make verify` and `make e2e`**: what each actually runs locally, how
  long to expect, and how to run a narrower slice (e.g. one package) during
  iteration.
- **Troubleshooting** for the sharp edges that will come up repeatedly
  (issuer DNS reachability between the test client/BFF/kube-apiserver,
  Valkey/Redis connection issues, stale kind cluster state).

Write this once the vertical slice (login → session → namespace list → UI)
actually runs locally — document the real commands, not the intended ones.

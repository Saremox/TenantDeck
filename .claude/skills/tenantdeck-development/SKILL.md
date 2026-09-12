---
name: tenantdeck-development
description: "Use when implementing or modifying TenantDeck code: the Go BFF, the embedded React/TypeScript frontend, the Dockerfile, the Helm chart, or GitHub Actions workflows. Loads the project's architecture and security constraints (read-only boundary, server-side sessions, upstream allowlisting, hardened Pod/container security, no in-cluster RBAC) before writing code, and points to the exact spec section for the area being touched. Trigger on \"implement\", \"add a feature/view/endpoint\", \"build the BFF/frontend/chart\", or any code change in the TenantDeck repository. Not for reviewing already-written code (use tenantdeck-security-review) or writing tests in isolation (use tenantdeck-testing)."
---

# TenantDeck development

TenantDeck is a read-only, security-first dashboard for an existing Capsule
multi-tenant Kubernetes platform:

```
Browser -> same-origin TenantDeck BFF (Go, embeds built React/TS assets) -> existing Capsule Proxy -> Kubernetes API
```

The BFF owns OIDC login and server-side sessions (external Valkey/Redis); the
browser only ever holds an opaque session ID. One configured
cluster/Capsule Proxy per installation — no multi-cluster abstraction.

## Before writing code

1. Check `docs/implementation-plan.md` for current status — don't redo a
   phase that's already done, and don't skip ahead of a dependency.
2. Read the specific `docs/spec/*.md` file for the area you're touching
   (table below). It is normative; this file is just the index plus
   workflow.
3. If you're making a non-obvious, non-cosmetic engineering decision
   (library choice, audience/claim mapping, SameSite behavior, cookie
   prefix, a deviation from the default architecture), record it as a short
   ADR in `docs/implementation-plan.md` as you go — don't leave it implicit
   in the diff.

| Area | Spec file |
|---|---|
| Read-only scope, required UI, non-goals | `docs/spec/02-product-and-scope.md` |
| BFF/frontend split, request path, audience/claims, authorization model | `docs/spec/03-architecture.md` |
| OIDC flow, cookies, CSRF, session encryption, refresh, logout, CSP | `docs/spec/04-auth-session-browser-security.md` |
| Capsule Proxy client: header allowlisting, path/verb validation, limits, isolation | `docs/spec/05-upstream-boundary-and-resilience.md` |
| Dockerfile, Pod/container securityContext, Helm chart, NetworkPolicy | `docs/spec/06-container-and-kubernetes-deployment.md` |
| CI workflows, least privilege, vulnerability policy, release workflow | `docs/spec/08-github-actions-and-supply-chain.md` |

## Non-negotiables while implementing (see `CLAUDE.md` for the full list)

- **Read-only is enforced in the backend, in code**, not by omitting UI
  buttons. A tenant owner with broad Kubernetes RBAC must still be blocked by
  TenantDeck's own allowlist. If a feature would need a write, a YAML editor,
  Secrets/ConfigMap contents, exec/attach/port-forward, or node/RBAC admin —
  it's out of scope; don't build a "just this once" exception.
- **Never fetch privileged/cluster-wide data and filter client-side.** Every
  request must be scoped server-side by validated parameters before it
  reaches Capsule Proxy.
- **Never forward incoming `Authorization`, `Cookie`, `Impersonate-*`,
  `X-Remote-*`, `X-Forwarded-Client-Cert`, or other untrusted headers
  upstream.** Build the outbound request from an explicit allowlist; insert
  only the backend-held ID token.
- **Never put ID/access/refresh tokens in anything browser-visible** —
  responses, cookies, URLs, logs, traces, metrics, error pages.
- **Never give the TenantDeck ServiceAccount Kubernetes API permissions.**
  No Role/ClusterRole/RoleBinding/ClusterRoleBinding in the chart, no SA
  token automount, no in-cluster client config.
- **Never invent a version.** Verify current docs (Kubernetes, Capsule OIDC
  setup, the OIDC library, Actions SHAs — see
  `docs/spec/11-primary-references.md`) before pinning a dependency or
  API version.
- **Don't build abstractions this spec doesn't ask for** — no multi-cluster
  support, plugin framework, billing integration, or Prometheus integration
  in v1.
- Write tests alongside the feature, not after — see the
  `tenantdeck-testing` skill for required coverage. A feature isn't done
  without them.
- **Comments document why, not what — see `CLAUDE.md` "Code style."** Name
  things so the code reads without narration; reserve comments for a
  non-obvious constraint or rationale (e.g. why a header must never be
  forwarded), not a restatement of the next line.

## Workflow

1. Implement the smallest correct slice (see
   `docs/spec/10-process-and-definition-of-done.md` for the intended
   execution order — vertical slice first: login → session → namespace list
   → UI — before breadth).
2. Add the tests the `tenantdeck-testing` skill calls for, for this change.
3. Update `docs/implementation-plan.md` (status) and
   `docs/security-test-matrix.md` (which rows this change covers, with
   evidence) in the same change — don't let them drift.
4. Before merging, run the `tenantdeck-security-review` skill as an
   adversarial pass on your own diff if it touches auth, the upstream
   client, chart security context, or CI permissions.

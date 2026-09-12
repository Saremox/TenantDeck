# Architecture

**Status: not started.** This is a skeleton — the authoritative description
of the intended architecture right now is
[`docs/spec/03-architecture.md`](spec/03-architecture.md). Replace this file
with the *actual* built system as it exists, once it exists; don't let it
drift into a second copy of the spec.

## What this must eventually cover

- **System diagram and request path.** Browser → same-origin BFF → Capsule
  Proxy → Kubernetes API, with the actual package/module boundaries inside
  the BFF (router, session store client, upstream client, OIDC handler).
- **Identity and audience configuration actually used.** The concrete,
  tested OIDC audience/claim mapping this deployment relies on (see
  `docs/spec/03-architecture.md` on not accepting a token just because
  Kubernetes would accept it), including multi-audience/`azp` handling if
  applicable.
- **Session lifecycle.** Where sessions live (Valkey/Redis), what's
  encrypted vs. plaintext in a session record, and how the key used for
  that encryption is supplied and rotated (cross-reference
  `docs/operations.md`).
- **Deployment topology.** How the chart's components (Deployment, Service,
  NetworkPolicy, external store) fit together at runtime, and what's
  explicitly *not* part of TenantDeck's runtime identity (Capsule's own
  components, CI's cluster-setup identity — see
  `docs/spec/06-container-and-kubernetes-deployment.md`).
- **Deliberate deviations from the spec**, each with the reason — record
  these as they're made (see `docs/implementation-plan.md` "Decisions /
  ADRs"), then summarize the ones that matter for understanding the system
  here.

Write this once there's a real vertical slice to describe — a diagram of an
unbuilt system is exactly the kind of placeholder the working agreement
warns against.

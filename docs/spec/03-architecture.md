# Architecture

> Part of the [TenantDeck kickoff specification](README.md).

Default to Go backend + React/TypeScript frontend. Build static assets and
embed/serve them from the Go application, yielding one production application
container and one browser origin. Node is build-time only. Prefer the standard
Go HTTP stack and maintained OIDC/OAuth libraries. Document justified
deviations.

```
Browser -> same-origin TenantDeck BFF -> existing Capsule Proxy -> Kubernetes API.
```

The BFF owns OIDC login and server-side sessions backed by external
Valkey/Redis. The browser receives an opaque session ID, never ID/access/refresh
tokens. Do not substitute an encrypted token-containing cookie for server-side
token storage. The BFF forwards the authenticated user's Kubernetes-compatible
ID token, never its own ServiceAccount credentials or a shared administrator
identity.

The identity provider issues tokens with audiences/claims accepted by both the
BFF login validation and the cluster's OIDC configuration. Validate the BFF's
intended audience; do not accept any token simply because it is usable by
Kubernetes. Document a concrete tested audience configuration, including
multi-audience/azp validation when applicable. Do not conflate ID tokens and
access tokens or promise general-purpose token exchange without implementing
and testing it.

TenantDeck does not replace Capsule authorization. Configure explicit backend
routes and resource/subresource/verb allowlists, construct upstream requests
from validated parameters, and preserve Kubernetes authorization failures.
Never fetch privileged data and filter it client-side. Namespace listing uses
Capsule Proxy; other requests are namespace-scoped. Tenant identification must
follow the deployed Capsule model, not a hard-coded tenant-name JWT claim.
Namespace names from a caller are never authorization evidence.

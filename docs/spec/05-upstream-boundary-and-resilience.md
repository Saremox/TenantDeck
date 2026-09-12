# Upstream boundary and resilience

> Part of the [TenantDeck kickoff specification](README.md).

- Upstream origin is fixed by trusted configuration. No caller-controlled hosts, full URLs, redirect targets, credentials or kubeconfigs. Never follow upstream redirects in a way that discloses credentials; do not relay upstream Set-Cookie or authentication challenges indiscriminately.
- Build outbound headers from an explicit allowlist. Never forward incoming Authorization, Cookie, Impersonate-*, X-Remote-*, X-Forwarded-Client-Cert, or untrusted forwarding headers. Handle hop-by-hop headers correctly. Insert only the backend-held ID token. Set upstream Host/SNI correctly and verify its CA.
- Reject unexpected methods, paths, encoded traversal/separators, ambiguous query parameters, unsupported content types, and non-allowlisted subresources. Enforce read-only even for a tenant owner with broad Kubernetes rights. GET alone does not make an arbitrary Kubernetes endpoint safe.
- Bound pagination, log tail/bytes, request sizes, header sizes, ordinary upstream response sizes, timeouts, concurrent requests, and active streams. Cancellation must terminate upstream work. Use bounded streaming rather than buffering full logs/watches. Prevent long streams from outliving the dashboard session indefinitely; document reconnection and expiry behavior.
- Protect against per-session resource exhaustion and login abuse; bound rate-limit state. Document any per-replica limits. Avoid retry storms on 401/403 and upstream failures. Do not let one browser create unbounded watches.
- Avoid shared Kubernetes response caches in v1. Never reuse a client with one user's Authorization header for another session. Test concurrent users for identity contamination.
- Structured redacted logs, request correlation IDs, bounded-cardinality metrics, and no publicly exposed debug/pprof routes. Treat detailed operational metrics as internal. Health checks reveal no secrets; liveness must not depend on IdP/Kubernetes availability.

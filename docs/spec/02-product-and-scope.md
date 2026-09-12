# Product and scope

> Part of the [TenantDeck kickoff specification](README.md).

TenantDeck is an independent customer-facing dashboard for an existing Capsule
multi-tenant Kubernetes platform. Capsule Proxy is already publicly accessible
for authenticated kubectl clients. TenantDeck must not require hiding that
endpoint or changing its existing consumers.

Deliver a complete, deliberately bounded READ-ONLY v1. Read-only is a security
boundary enforced by the backend, not just missing UI buttons. Do not add
writes, a YAML editor, arbitrary API exploration, Secrets, ConfigMap contents,
node administration, RBAC administration, exec, attach, port-forward, or
service/pod proxy subresources. Record them as out of scope, not unfinished v1
features.

## Required UI

- OIDC login/logout and safe session-expired handling.
- Namespace selector showing only namespaces available through the user's Capsule identity.
- Overview with workload health and namespace ResourceQuota/LimitRange information.
- List/detail views for Deployments, StatefulSets, DaemonSets, Pods, Jobs, and CronJobs.
- Services, Ingresses, and PVC status views.
- Namespace events and bounded pod logs, including follow mode with cancellation/reconnection.
- Clear empty, loading, forbidden, expired-session, and unavailable-upstream states.
- A usable responsive interface with accessible controls. No fake production data or placeholder buttons. UI automation is optional; frontend typechecking/build and logic tests are mandatory.

## Deliberate non-goals

Assume one configured cluster/Capsule Proxy per installation. Avoid
multi-cluster abstraction, plugin frameworks, external billing integrations,
and Prometheus integration in v1. A thin coherent working application is more
important than feature breadth.

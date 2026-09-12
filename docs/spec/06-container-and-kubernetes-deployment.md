# Container and Kubernetes deployment

> Part of the [TenantDeck kickoff specification](README.md).

Provide a reproducible multi-stage Dockerfile, minimal non-root runtime with CA
certificates, no runtime shell/package manager where practical, and no
embedded credentials. Pin base image digests and provide a documented update
mechanism. Listen on an unprivileged port.

The Helm chart must set by default:

```yaml
automountServiceAccountToken: false
# Pod securityContext
runAsNonRoot: true
runAsUser: 65532
runAsGroup: 65532
seccompProfile:
  type: RuntimeDefault
# Container securityContext
allowPrivilegeEscalation: false
readOnlyRootFilesystem: true
capabilities:
  drop: [ALL]
```

Render these at the correct Kubernetes fields, not as one literal block. Use no
privileged containers, host network/PID/IPC, hostPath, or unnecessary writable
volumes. If temporary storage is required, use a size-limited emptyDir and
explain it. Set resource requests/limits including ephemeral storage where
appropriate. Apply the same standards to TenantDeck chart hooks/test pods.

Use a dedicated ServiceAccount with token automount disabled at both SA and
Pod level. TenantDeck's chart creates NO Role, ClusterRole, RoleBinding or
ClusterRoleBinding by default. It needs no Kubernetes API permissions of its
own and must not load in-cluster credentials. Mounted Secrets are supplied by
Kubernetes, not fetched via its SA. Capsule's separately installed privileged
components and CI's cluster setup identity are distinct from TenantDeck's
runtime identity.

## Chart requirements

- Deployment, ClusterIP Service, dedicated SA, config, probes, configurable replicas, graceful shutdown and rolling upgrades.
- External Valkey/Redis configuration with authentication, TLS/custom CA support and existing Secret references. No bundled production database dependency in v1. Test infrastructure provisions its own store. Test two replicas sharing sessions.
- Existing Secret references for OIDC client secret and session encryption key; no plaintext production secrets in values, ConfigMaps, NOTES, or generated examples. No randomly generated secrets during template rendering.
- Optional Ingress with TLS and documented trusted-proxy settings; app remains same-origin. No public NodePort/LoadBalancer by default.
- NetworkPolicy enabled by default with explicit ingress and egress allowances for the deployment environment: ingress controller, DNS, IdP, Capsule Proxy and session store. No default allow-all egress. Require users to configure external endpoint CIDRs/selectors; do not claim standard NetworkPolicy supports FQDNs. Document DNS rotation, CNI/NAT limitations, and optional Cilium examples separately. Include a working restricted-policy CI profile.
- values.schema.json, commented values, example development and production values, helm lint/template validation, chart tests, digest image support, and clear NOTES.
- Compatible with the Restricted Pod Security Standard. Test TenantDeck in a namespace enforcing Restricted. Install Capsule/operator dependencies in separate namespaces with their own documented requirements.

#!/usr/bin/env bash
# Brings up the mandatory E2E stack (docs/spec/07-mandatory-automated-testing.md):
# disposable kind cluster + real Calico (kindnet does not enforce
# NetworkPolicy) + real Capsule operator/proxy + Valkey + the mock OIDC
# provider + the actual built TenantDeck image via the actual chart, two
# replicas. Run `make e2e`, not this script directly, unless you're
# iterating on the stack itself.
#
# NOT executed end-to-end in this repository's own development session -
# see docs/implementation-plan.md Phase 4 "Known blockers" for the two
# independent, fully diagnosed reasons (a registry policy block and a
# nested-container-runtime ceiling specific to that sandbox). Written to
# run on a normal CI runner or developer machine with real Docker/kind.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLUSTER_NAME="${CLUSTER_NAME:-tenantdeck-e2e}"
KUBECONFIG_PATH="$HERE/.kubeconfig"
# ADR-007 (docs/implementation-plan.md): kind v0.33.0's own default node
# image is Kubernetes 1.36.1, matching ADR-006's Capsule compatibility
# target. Override for a runner whose Docker reports cgroup v1 (this
# version's kubelet still starts there; anything newer hard-refuses
# cgroup v1 - see the Phase 4 "Known blockers" note above).
NODE_IMAGE="${NODE_IMAGE:-kindest/node:v1.36.1}"
CALICO_VERSION="${CALICO_VERSION:-v3.32.2}"
CAPSULE_VERSION="${CAPSULE_VERSION:-0.14.5}"
CAPSULE_PROXY_VERSION="${CAPSULE_PROXY_VERSION:-0.14.1}"

echo "==> Creating kind cluster ($CLUSTER_NAME, node image $NODE_IMAGE)"
kind create cluster --name "$CLUSTER_NAME" --config "$HERE/kind-config.yaml" --image "$NODE_IMAGE" --kubeconfig "$KUBECONFIG_PATH"
export KUBECONFIG="$KUBECONFIG_PATH"

echo "==> Installing Calico (kindnet does not enforce NetworkPolicy)"
kubectl apply -f "https://raw.githubusercontent.com/projectcalico/calico/$CALICO_VERSION/manifests/calico.yaml"
kubectl -n kube-system rollout status daemonset/calico-node --timeout=180s
kubectl -n kube-system rollout status deployment/calico-kube-controllers --timeout=180s

echo "==> Installing Capsule operator + capsule-proxy (pinned per ADR-006)"
helm repo add projectcapsule https://projectcapsule.github.io/charts >/dev/null
helm repo update projectcapsule >/dev/null
helm upgrade --install capsule projectcapsule/capsule \
  --version "$CAPSULE_VERSION" --namespace capsule-system --create-namespace --wait
helm upgrade --install capsule-proxy projectcapsule/capsule-proxy \
  --version "$CAPSULE_PROXY_VERSION" --namespace capsule-system --wait

echo "==> Installing Valkey (test infrastructure provisions its own store - docs/route-allowlist.md)"
kubectl apply -n tenantdeck-e2e -f "$HERE/manifests/valkey.yaml" --validate=false 2>/dev/null || \
  kubectl create namespace tenantdeck-e2e --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -n tenantdeck-e2e -f "$HERE/manifests/valkey.yaml"
kubectl -n tenantdeck-e2e rollout status deployment/valkey --timeout=120s

echo "==> Building and loading the mockoidc and tenantdeck images"
docker build -t tenantdeck:e2e "$HERE/.."
docker build -t mockoidc:e2e -f "$HERE/mockoidc.Dockerfile" "$HERE/.."
kind load docker-image tenantdeck:e2e mockoidc:e2e --name "$CLUSTER_NAME"

echo "==> Deploying the mock OIDC provider"
kubectl apply -n tenantdeck-e2e -f "$HERE/manifests/mockoidc.yaml"
kubectl -n tenantdeck-e2e rollout status deployment/mockoidc --timeout=120s

echo "==> Applying two-tenant Capsule fixtures (docs/spec/07 item 6)"
# Known gap: kube-apiserver itself - not capsule-proxy - is what resolves
# the authenticated username Capsule matches against these Tenants'
# owners[].name (capsule-proxy scopes by that identity, it doesn't
# re-validate the bearer token itself). Wiring kube-apiserver's own
# --oidc-* flags at the mock OIDC provider for this disposable cluster is
# not done here yet - see docs/implementation-plan.md Phase 4 "Known
# blockers" for why (it needs real cluster iteration to get the
# TLS/in-cluster-reachability details right, which this sandbox can't do).
kubectl apply -f "$HERE/fixtures/tenants.yaml"

echo "==> Creating the existing-Secret references the chart requires"
kubectl -n tenantdeck-e2e create secret generic tenantdeck-oidc \
  --from-literal=client-secret=e2e-secret --dry-run=client -o yaml | kubectl apply -f -
kubectl -n tenantdeck-e2e create secret generic tenantdeck-session \
  --from-literal=encryption-key="$(openssl rand -base64 32)" --dry-run=client -o yaml | kubectl apply -f -

echo "==> Installing the TenantDeck chart (2 replicas, Restricted PSS namespace)"
kubectl label namespace tenantdeck-e2e pod-security.kubernetes.io/enforce=restricted --overwrite
helm upgrade --install tenantdeck "$HERE/../charts/tenantdeck" \
  --namespace tenantdeck-e2e \
  -f "$HERE/manifests/values-e2e.yaml" \
  --wait --timeout 120s

echo "==> Stack is up. KUBECONFIG=$KUBECONFIG_PATH"

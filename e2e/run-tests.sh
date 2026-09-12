#!/usr/bin/env bash
# Port-forwards the live tenantdeck and mockoidc Services so
# e2e/*_test.go can reach them over plain HTTP, runs the suite, and
# always cleans up (port-forwards, /etc/hosts) afterward regardless of
# test outcome.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export KUBECONFIG="$HERE/.kubeconfig"

# values-e2e.yaml configures TenantDeck's own externalURL/issuerURL as
# their real in-cluster Service DNS names, not localhost - the OIDC
# authorization-code redirects the test's http.Client follows
# (mockoidc's /authorize -> TenantDeck's /auth/callback) land on those
# hostnames literally, not wherever the test's first request happened to
# go. Alias both to 127.0.0.1 so the test process, not just TenantDeck's
# own in-cluster pod, can resolve them; kubectl port-forward reaches the
# pod directly (it doesn't go through the Service's ClusterIP), so this
# doesn't need to satisfy the NetworkPolicy that'd otherwise apply.
HOSTS_LINE="127.0.0.1 tenantdeck.tenantdeck-e2e.svc.cluster.local mockoidc.tenantdeck-e2e.svc.cluster.local"
HOSTS_LINE_ADDED=0
if ! grep -qF "$HOSTS_LINE" /etc/hosts; then
  echo "$HOSTS_LINE" | sudo tee -a /etc/hosts >/dev/null
  HOSTS_LINE_ADDED=1
fi

kubectl -n tenantdeck-e2e port-forward svc/tenantdeck 8080:8080 >/tmp/tenantdeck-e2e-port-forward.log 2>&1 &
TENANTDECK_PF_PID=$!
kubectl -n tenantdeck-e2e port-forward svc/mockoidc 9999:9999 >/tmp/mockoidc-e2e-port-forward.log 2>&1 &
MOCKOIDC_PF_PID=$!
cleanup() {
  kill "$TENANTDECK_PF_PID" "$MOCKOIDC_PF_PID" 2>/dev/null || true
  if [ "$HOSTS_LINE_ADDED" = 1 ]; then
    sudo sed -i "\#$HOSTS_LINE#d" /etc/hosts || true
  fi
}
trap cleanup EXIT

export TENANTDECK_E2E_BASE_URL="http://tenantdeck.tenantdeck-e2e.svc.cluster.local:8080"

for i in $(seq 1 30); do
  if curl -sf -o /dev/null "$TENANTDECK_E2E_BASE_URL/healthz"; then
    break
  fi
  sleep 1
done

go test -tags e2e -v ./e2e/...

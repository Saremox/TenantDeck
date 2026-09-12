#!/usr/bin/env bash
# Port-forwards the live tenantdeck Service so e2e/*_test.go can reach it
# over plain localhost HTTP, runs the suite, and always kills the
# port-forward afterward regardless of test outcome.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export KUBECONFIG="$HERE/.kubeconfig"

kubectl -n tenantdeck-e2e port-forward svc/tenantdeck 18080:8080 >/tmp/tenantdeck-e2e-port-forward.log 2>&1 &
PF_PID=$!
trap 'kill "$PF_PID" 2>/dev/null || true' EXIT

for i in $(seq 1 30); do
  if curl -sf -o /dev/null http://127.0.0.1:18080/healthz; then
    break
  fi
  sleep 1
done

go test -tags e2e -v ./e2e/...

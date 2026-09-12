#!/usr/bin/env bash
# Tears down the disposable E2E cluster, and only that cluster -
# docs/spec/07-mandatory-automated-testing.md "teardown occurs on
# success and failure, scoped only to the test resources."
set -euo pipefail
CLUSTER_NAME="${CLUSTER_NAME:-tenantdeck-e2e}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
kind delete cluster --name "$CLUSTER_NAME" --kubeconfig "$HERE/.kubeconfig" || true
rm -f "$HERE/.kubeconfig"

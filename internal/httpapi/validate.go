package httpapi

import "regexp"

// k8sNamePattern matches a DNS-1123 label: lowercase alphanumeric and '-',
// starting and ending with alphanumeric, max 63 characters - the format
// Kubernetes requires for namespace and most object names. Every namespace
// and resource-name path parameter is checked against this before it's
// used to build an upstream URL - see
// docs/spec/05-upstream-boundary-and-resilience.md ("malformed paths...
// rejected") and docs/route-allowlist.md. This rejects encoded traversal
// ("..", "%2e%2e"), embedded separators, and anything else that isn't a
// plain, valid Kubernetes name - well before the upstream client is ever
// called.
var k8sNamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

func validK8sName(s string) bool {
	return k8sNamePattern.MatchString(s)
}

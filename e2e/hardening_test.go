//go:build e2e

package e2e

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// kubectlJSON runs `kubectl get <args> -o json` against the live cluster
// and decodes the result - these tests check the actually-running Pod
// spec, not the chart's rendered YAML (that's templates/deployment.yaml's
// own helm-template tests).
func kubectlJSON(t *testing.T, v any, args ...string) {
	t.Helper()
	fullArgs := append([]string{"--kubeconfig", kubeconfigPath(), "-n", "tenantdeck-e2e", "get"}, args...)
	fullArgs = append(fullArgs, "-o", "json")
	out, err := exec.Command("kubectl", fullArgs...).Output()
	if err != nil {
		t.Fatalf("kubectl %v: %v", fullArgs, err)
	}
	if err := json.Unmarshal(out, v); err != nil {
		t.Fatalf("decoding kubectl JSON output: %v", err)
	}
}

type podList struct {
	Items []struct {
		Spec struct {
			AutomountServiceAccountToken *bool `json:"automountServiceAccountToken"`
			Containers                   []struct {
				Name            string `json:"name"`
				SecurityContext struct {
					ReadOnlyRootFilesystem   *bool `json:"readOnlyRootFilesystem"`
					AllowPrivilegeEscalation *bool `json:"allowPrivilegeEscalation"`
					Capabilities             struct {
						Drop []string `json:"drop"`
					} `json:"capabilities"`
				} `json:"securityContext"`
			} `json:"containers"`
			Volumes []struct {
				Name string `json:"name"`
			} `json:"volumes"`
		} `json:"spec"`
		Status struct {
			Phase string `json:"phase"`
		} `json:"status"`
	} `json:"items"`
}

// TestPodHardening_NoServiceAccountTokenAndReadOnlyRootFilesystem proves
// the Pod actually running in the cluster - not just the rendered
// manifest - has no SA token volume mounted and a read-only root
// filesystem, per docs/spec/07 "Runtime Pod/SA configuration is
// hardened, no SA token volume is mounted, and ordinary app functions
// work with read-only root filesystem."
func TestPodHardening_NoServiceAccountTokenAndReadOnlyRootFilesystem(t *testing.T) {
	var pods podList
	kubectlJSON(t, &pods, "pods", "-l", "app.kubernetes.io/name=tenantdeck")

	if len(pods.Items) == 0 {
		t.Fatal("no tenantdeck pods found")
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase != "Running" {
			t.Errorf("pod phase = %q, want Running", pod.Status.Phase)
			continue
		}
		if pod.Spec.AutomountServiceAccountToken == nil || *pod.Spec.AutomountServiceAccountToken {
			t.Error("automountServiceAccountToken is not explicitly false")
		}
		for _, vol := range pod.Spec.Volumes {
			if strings.HasPrefix(vol.Name, "kube-api-access") {
				t.Errorf("found a service account token volume: %s", vol.Name)
			}
		}
		for _, c := range pod.Spec.Containers {
			if c.SecurityContext.ReadOnlyRootFilesystem == nil || !*c.SecurityContext.ReadOnlyRootFilesystem {
				t.Errorf("container %s: readOnlyRootFilesystem is not true", c.Name)
			}
			if c.SecurityContext.AllowPrivilegeEscalation == nil || *c.SecurityContext.AllowPrivilegeEscalation {
				t.Errorf("container %s: allowPrivilegeEscalation is not false", c.Name)
			}
			dropsAll := false
			for _, d := range c.SecurityContext.Capabilities.Drop {
				if d == "ALL" {
					dropsAll = true
				}
			}
			if !dropsAll {
				t.Errorf("container %s: capabilities.drop does not include ALL", c.Name)
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

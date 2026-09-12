package httpapi

import (
	"encoding/json"
	"fmt"

	"github.com/Saremox/TenantDeck/internal/capsule"
)

var (
	deploymentsGVR  = capsule.GroupVersionResource{APIPrefix: "apis/apps/v1", Resource: "deployments"}
	statefulSetsGVR = capsule.GroupVersionResource{APIPrefix: "apis/apps/v1", Resource: "statefulsets"}
	daemonSetsGVR   = capsule.GroupVersionResource{APIPrefix: "apis/apps/v1", Resource: "daemonsets"}
	podsGVR         = capsule.GroupVersionResource{APIPrefix: "api/v1", Resource: "pods"}
	jobsGVR         = capsule.GroupVersionResource{APIPrefix: "apis/batch/v1", Resource: "jobs"}
	cronJobsGVR     = capsule.GroupVersionResource{APIPrefix: "apis/batch/v1", Resource: "cronjobs"}
)

type deploymentSummary struct {
	Name              string `json:"name"`
	Replicas          int32  `json:"replicas"`
	ReadyReplicas     int32  `json:"readyReplicas"`
	UpdatedReplicas   int32  `json:"updatedReplicas"`
	AvailableReplicas int32  `json:"availableReplicas"`
}

func summarizeDeployment(raw json.RawMessage) (any, error) {
	var obj struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			Replicas int32 `json:"replicas"`
		} `json:"spec"`
		Status struct {
			ReadyReplicas     int32 `json:"readyReplicas"`
			UpdatedReplicas   int32 `json:"updatedReplicas"`
			AvailableReplicas int32 `json:"availableReplicas"`
		} `json:"status"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decoding deployment: %w", err)
	}
	return deploymentSummary{
		Name:              obj.Metadata.Name,
		Replicas:          obj.Spec.Replicas,
		ReadyReplicas:     obj.Status.ReadyReplicas,
		UpdatedReplicas:   obj.Status.UpdatedReplicas,
		AvailableReplicas: obj.Status.AvailableReplicas,
	}, nil
}

type statefulSetSummary struct {
	Name            string `json:"name"`
	Replicas        int32  `json:"replicas"`
	ReadyReplicas   int32  `json:"readyReplicas"`
	CurrentReplicas int32  `json:"currentReplicas"`
}

func summarizeStatefulSet(raw json.RawMessage) (any, error) {
	var obj struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			Replicas int32 `json:"replicas"`
		} `json:"spec"`
		Status struct {
			ReadyReplicas   int32 `json:"readyReplicas"`
			CurrentReplicas int32 `json:"currentReplicas"`
		} `json:"status"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decoding statefulset: %w", err)
	}
	return statefulSetSummary{
		Name:            obj.Metadata.Name,
		Replicas:        obj.Spec.Replicas,
		ReadyReplicas:   obj.Status.ReadyReplicas,
		CurrentReplicas: obj.Status.CurrentReplicas,
	}, nil
}

type daemonSetSummary struct {
	Name                   string `json:"name"`
	DesiredNumberScheduled int32  `json:"desiredNumberScheduled"`
	NumberReady            int32  `json:"numberReady"`
	NumberAvailable        int32  `json:"numberAvailable"`
}

func summarizeDaemonSet(raw json.RawMessage) (any, error) {
	var obj struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Status struct {
			DesiredNumberScheduled int32 `json:"desiredNumberScheduled"`
			NumberReady            int32 `json:"numberReady"`
			NumberAvailable        int32 `json:"numberAvailable"`
		} `json:"status"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decoding daemonset: %w", err)
	}
	return daemonSetSummary{
		Name:                   obj.Metadata.Name,
		DesiredNumberScheduled: obj.Status.DesiredNumberScheduled,
		NumberReady:            obj.Status.NumberReady,
		NumberAvailable:        obj.Status.NumberAvailable,
	}, nil
}

type containerSummary struct {
	Name         string `json:"name"`
	Image        string `json:"image"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restartCount"`
}

type podSummary struct {
	Name       string             `json:"name"`
	Phase      string             `json:"phase"`
	Ready      bool               `json:"ready"`
	Containers []containerSummary `json:"containers"`
}

func summarizePod(raw json.RawMessage) (any, error) {
	var obj struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			Containers []struct {
				Name  string `json:"name"`
				Image string `json:"image"`
			} `json:"containers"`
		} `json:"spec"`
		Status struct {
			Phase             string `json:"phase"`
			ContainerStatuses []struct {
				Name         string `json:"name"`
				Image        string `json:"image"`
				Ready        bool   `json:"ready"`
				RestartCount int32  `json:"restartCount"`
			} `json:"containerStatuses"`
		} `json:"status"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decoding pod: %w", err)
	}

	containers := make([]containerSummary, 0, len(obj.Status.ContainerStatuses))
	allReady := len(obj.Status.ContainerStatuses) > 0
	for _, cs := range obj.Status.ContainerStatuses {
		containers = append(containers, containerSummary{
			Name: cs.Name, Image: cs.Image, Ready: cs.Ready, RestartCount: cs.RestartCount,
		})
		if !cs.Ready {
			allReady = false
		}
	}
	// Fall back to spec.containers (name/image only, no status) if the pod
	// hasn't reported any container statuses yet - e.g. still Pending.
	if len(containers) == 0 {
		for _, c := range obj.Spec.Containers {
			containers = append(containers, containerSummary{Name: c.Name, Image: c.Image})
		}
		allReady = false
	}

	return podSummary{
		Name:       obj.Metadata.Name,
		Phase:      obj.Status.Phase,
		Ready:      allReady,
		Containers: containers,
	}, nil
}

type jobSummary struct {
	Name      string `json:"name"`
	Active    int32  `json:"active"`
	Succeeded int32  `json:"succeeded"`
	Failed    int32  `json:"failed"`
}

func summarizeJob(raw json.RawMessage) (any, error) {
	var obj struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Status struct {
			Active    int32 `json:"active"`
			Succeeded int32 `json:"succeeded"`
			Failed    int32 `json:"failed"`
		} `json:"status"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decoding job: %w", err)
	}
	return jobSummary{
		Name:      obj.Metadata.Name,
		Active:    obj.Status.Active,
		Succeeded: obj.Status.Succeeded,
		Failed:    obj.Status.Failed,
	}, nil
}

type cronJobSummary struct {
	Name             string  `json:"name"`
	Schedule         string  `json:"schedule"`
	Suspend          bool    `json:"suspend"`
	LastScheduleTime *string `json:"lastScheduleTime,omitempty"`
}

func summarizeCronJob(raw json.RawMessage) (any, error) {
	var obj struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			Schedule string `json:"schedule"`
			Suspend  bool   `json:"suspend"`
		} `json:"spec"`
		Status struct {
			LastScheduleTime *string `json:"lastScheduleTime"`
		} `json:"status"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decoding cronjob: %w", err)
	}
	return cronJobSummary{
		Name:             obj.Metadata.Name,
		Schedule:         obj.Spec.Schedule,
		Suspend:          obj.Spec.Suspend,
		LastScheduleTime: obj.Status.LastScheduleTime,
	}, nil
}

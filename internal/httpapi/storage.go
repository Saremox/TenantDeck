package httpapi

import (
	"encoding/json"
	"fmt"

	"github.com/Saremox/TenantDeck/internal/capsule"
)

var pvcGVR = capsule.GroupVersionResource{APIPrefix: "api/v1", Resource: "persistentvolumeclaims"}

type persistentVolumeClaimSummary struct {
	Name         string   `json:"name"`
	Phase        string   `json:"phase"`
	Capacity     string   `json:"capacity,omitempty"`
	AccessModes  []string `json:"accessModes"`
	StorageClass *string  `json:"storageClassName,omitempty"`
}

func summarizePersistentVolumeClaim(raw json.RawMessage) (any, error) {
	var obj struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			AccessModes      []string `json:"accessModes"`
			StorageClassName *string  `json:"storageClassName"`
		} `json:"spec"`
		Status struct {
			Phase    string `json:"phase"`
			Capacity struct {
				Storage string `json:"storage"`
			} `json:"capacity"`
		} `json:"status"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decoding persistentvolumeclaim: %w", err)
	}
	return persistentVolumeClaimSummary{
		Name:         obj.Metadata.Name,
		Phase:        obj.Status.Phase,
		Capacity:     obj.Status.Capacity.Storage,
		AccessModes:  obj.Spec.AccessModes,
		StorageClass: obj.Spec.StorageClassName,
	}, nil
}

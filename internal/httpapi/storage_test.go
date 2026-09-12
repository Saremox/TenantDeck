package httpapi

import "testing"

func TestSummarizePersistentVolumeClaim_ParsesPhaseCapacityAndAccessModes(t *testing.T) {
	raw := []byte(`{
		"metadata": {"name": "data"},
		"spec": {"accessModes": ["ReadWriteOnce"], "storageClassName": "standard"},
		"status": {"phase": "Bound", "capacity": {"storage": "10Gi"}}
	}`)
	got, err := summarizePersistentVolumeClaim(raw)
	if err != nil {
		t.Fatalf("summarizePersistentVolumeClaim: %v", err)
	}
	pvc := got.(persistentVolumeClaimSummary)
	if pvc.Name != "data" || pvc.Phase != "Bound" || pvc.Capacity != "10Gi" {
		t.Errorf("summarizePersistentVolumeClaim() = %+v", pvc)
	}
	if len(pvc.AccessModes) != 1 || pvc.AccessModes[0] != "ReadWriteOnce" {
		t.Errorf("AccessModes = %v", pvc.AccessModes)
	}
	if pvc.StorageClass == nil || *pvc.StorageClass != "standard" {
		t.Errorf("StorageClass = %v, want a pointer to %q", pvc.StorageClass, "standard")
	}
}

func TestSummarizePersistentVolumeClaim_PendingHasNoCapacityYet(t *testing.T) {
	raw := []byte(`{"metadata": {"name": "data"}, "spec": {"accessModes": ["ReadWriteOnce"]}, "status": {"phase": "Pending"}}`)
	got, err := summarizePersistentVolumeClaim(raw)
	if err != nil {
		t.Fatalf("summarizePersistentVolumeClaim: %v", err)
	}
	if got.(persistentVolumeClaimSummary).Capacity != "" {
		t.Errorf("Capacity = %q, want empty for a Pending claim", got.(persistentVolumeClaimSummary).Capacity)
	}
}

func TestSummarizePersistentVolumeClaim_ErrorsOnMalformedJSON(t *testing.T) {
	if _, err := summarizePersistentVolumeClaim([]byte(`not json`)); err == nil {
		t.Fatal("summarizePersistentVolumeClaim() succeeded on malformed JSON, want error")
	}
}

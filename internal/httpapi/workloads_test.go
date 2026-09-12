package httpapi

import "testing"

func TestSummarizeDeployment_ParsesNameAndReplicaCounts(t *testing.T) {
	raw := []byte(`{
		"metadata": {"name": "api"},
		"spec": {"replicas": 3},
		"status": {"readyReplicas": 2, "updatedReplicas": 3, "availableReplicas": 2}
	}`)
	got, err := summarizeDeployment(raw)
	if err != nil {
		t.Fatalf("summarizeDeployment: %v", err)
	}
	want := deploymentSummary{Name: "api", Replicas: 3, ReadyReplicas: 2, UpdatedReplicas: 3, AvailableReplicas: 2}
	if got != want {
		t.Errorf("summarizeDeployment() = %+v, want %+v", got, want)
	}
}

func TestSummarizeDeployment_ErrorsOnMalformedJSON(t *testing.T) {
	if _, err := summarizeDeployment([]byte(`not json`)); err == nil {
		t.Fatal("summarizeDeployment() succeeded on malformed JSON, want error")
	}
}

func TestSummarizeStatefulSet_ParsesNameAndReplicaCounts(t *testing.T) {
	raw := []byte(`{
		"metadata": {"name": "db"},
		"spec": {"replicas": 3},
		"status": {"readyReplicas": 3, "currentReplicas": 3}
	}`)
	got, err := summarizeStatefulSet(raw)
	if err != nil {
		t.Fatalf("summarizeStatefulSet: %v", err)
	}
	want := statefulSetSummary{Name: "db", Replicas: 3, ReadyReplicas: 3, CurrentReplicas: 3}
	if got != want {
		t.Errorf("summarizeStatefulSet() = %+v, want %+v", got, want)
	}
}

func TestSummarizeStatefulSet_ErrorsOnMalformedJSON(t *testing.T) {
	if _, err := summarizeStatefulSet([]byte(`not json`)); err == nil {
		t.Fatal("summarizeStatefulSet() succeeded on malformed JSON, want error")
	}
}

func TestSummarizeDaemonSet_ParsesNameAndCounts(t *testing.T) {
	raw := []byte(`{
		"metadata": {"name": "node-agent"},
		"status": {"desiredNumberScheduled": 5, "numberReady": 5, "numberAvailable": 5}
	}`)
	got, err := summarizeDaemonSet(raw)
	if err != nil {
		t.Fatalf("summarizeDaemonSet: %v", err)
	}
	want := daemonSetSummary{Name: "node-agent", DesiredNumberScheduled: 5, NumberReady: 5, NumberAvailable: 5}
	if got != want {
		t.Errorf("summarizeDaemonSet() = %+v, want %+v", got, want)
	}
}

func TestSummarizeDaemonSet_ErrorsOnMalformedJSON(t *testing.T) {
	if _, err := summarizeDaemonSet([]byte(`not json`)); err == nil {
		t.Fatal("summarizeDaemonSet() succeeded on malformed JSON, want error")
	}
}

func TestSummarizePod_ParsesPhaseAndContainerStatuses(t *testing.T) {
	raw := []byte(`{
		"metadata": {"name": "api-abc123"},
		"spec": {"containers": [{"name": "app", "image": "app:v1"}]},
		"status": {
			"phase": "Running",
			"containerStatuses": [{"name": "app", "image": "app:v1", "ready": true, "restartCount": 0}]
		}
	}`)
	got, err := summarizePod(raw)
	if err != nil {
		t.Fatalf("summarizePod: %v", err)
	}
	want := podSummary{
		Name: "api-abc123", Phase: "Running", Ready: true,
		Containers: []containerSummary{{Name: "app", Image: "app:v1", Ready: true, RestartCount: 0}},
	}
	got2 := got.(podSummary)
	if got2.Name != want.Name || got2.Phase != want.Phase || got2.Ready != want.Ready {
		t.Errorf("summarizePod() = %+v, want %+v", got2, want)
	}
}

func TestSummarizePod_NotReadyWhenAnyContainerIsNotReady(t *testing.T) {
	raw := []byte(`{
		"metadata": {"name": "api-abc123"},
		"status": {
			"phase": "Running",
			"containerStatuses": [
				{"name": "app", "ready": true},
				{"name": "sidecar", "ready": false}
			]
		}
	}`)
	got, err := summarizePod(raw)
	if err != nil {
		t.Fatalf("summarizePod: %v", err)
	}
	if got.(podSummary).Ready {
		t.Error("Ready = true, want false when any container isn't ready")
	}
}

// A pod that hasn't been scheduled yet has no containerStatuses at all -
// this must fall back to spec.containers (name/image only) rather than
// reporting zero containers or erroring.
func TestSummarizePod_FallsBackToSpecContainersWhenNoStatusYet(t *testing.T) {
	raw := []byte(`{
		"metadata": {"name": "pending-pod"},
		"spec": {"containers": [{"name": "app", "image": "app:v1"}]},
		"status": {"phase": "Pending"}
	}`)
	got, err := summarizePod(raw)
	if err != nil {
		t.Fatalf("summarizePod: %v", err)
	}
	p := got.(podSummary)
	if p.Ready {
		t.Error("Ready = true, want false for a Pending pod with no container statuses")
	}
	if len(p.Containers) != 1 || p.Containers[0].Name != "app" {
		t.Errorf("Containers = %+v, want one container named app from spec", p.Containers)
	}
}

func TestSummarizePod_ErrorsOnMalformedJSON(t *testing.T) {
	if _, err := summarizePod([]byte(`not json`)); err == nil {
		t.Fatal("summarizePod() succeeded on malformed JSON, want error")
	}
}

func TestSummarizeJob_ParsesStatusCounts(t *testing.T) {
	raw := []byte(`{"metadata": {"name": "migrate"}, "status": {"active": 0, "succeeded": 1, "failed": 0}}`)
	got, err := summarizeJob(raw)
	if err != nil {
		t.Fatalf("summarizeJob: %v", err)
	}
	want := jobSummary{Name: "migrate", Active: 0, Succeeded: 1, Failed: 0}
	if got != want {
		t.Errorf("summarizeJob() = %+v, want %+v", got, want)
	}
}

func TestSummarizeJob_ErrorsOnMalformedJSON(t *testing.T) {
	if _, err := summarizeJob([]byte(`not json`)); err == nil {
		t.Fatal("summarizeJob() succeeded on malformed JSON, want error")
	}
}

func TestSummarizeCronJob_ParsesScheduleAndLastRun(t *testing.T) {
	raw := []byte(`{
		"metadata": {"name": "nightly-backup"},
		"spec": {"schedule": "0 2 * * *", "suspend": false},
		"status": {"lastScheduleTime": "2026-09-12T02:00:00Z"}
	}`)
	got, err := summarizeCronJob(raw)
	if err != nil {
		t.Fatalf("summarizeCronJob: %v", err)
	}
	cj := got.(cronJobSummary)
	if cj.Name != "nightly-backup" || cj.Schedule != "0 2 * * *" || cj.Suspend {
		t.Errorf("summarizeCronJob() = %+v", cj)
	}
	if cj.LastScheduleTime == nil || *cj.LastScheduleTime != "2026-09-12T02:00:00Z" {
		t.Errorf("LastScheduleTime = %v, want a pointer to the timestamp", cj.LastScheduleTime)
	}
}

func TestSummarizeCronJob_LastScheduleTimeNilWhenNeverRun(t *testing.T) {
	raw := []byte(`{"metadata": {"name": "never-run"}, "spec": {"schedule": "@weekly"}, "status": {}}`)
	got, err := summarizeCronJob(raw)
	if err != nil {
		t.Fatalf("summarizeCronJob: %v", err)
	}
	if got.(cronJobSummary).LastScheduleTime != nil {
		t.Error("LastScheduleTime != nil, want nil for a cronjob that has never run")
	}
}

func TestSummarizeCronJob_ErrorsOnMalformedJSON(t *testing.T) {
	if _, err := summarizeCronJob([]byte(`not json`)); err == nil {
		t.Fatal("summarizeCronJob() succeeded on malformed JSON, want error")
	}
}

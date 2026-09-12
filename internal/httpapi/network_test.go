package httpapi

import "testing"

func TestSummarizeService_ParsesTypeClusterIPAndPorts(t *testing.T) {
	raw := []byte(`{
		"metadata": {"name": "api"},
		"spec": {
			"type": "ClusterIP",
			"clusterIP": "10.0.0.5",
			"ports": [{"name": "http", "port": 80, "targetPort": 8080, "protocol": "TCP"}]
		}
	}`)
	got, err := summarizeService(raw)
	if err != nil {
		t.Fatalf("summarizeService: %v", err)
	}
	s := got.(serviceSummary)
	if s.Name != "api" || s.Type != "ClusterIP" || s.ClusterIP != "10.0.0.5" {
		t.Errorf("summarizeService() = %+v", s)
	}
	if len(s.Ports) != 1 || s.Ports[0].Port != 80 || s.Ports[0].Protocol != "TCP" {
		t.Errorf("Ports = %+v", s.Ports)
	}
}

// targetPort can be a number or a named port string in the real API -
// confirm both decode without error.
func TestSummarizeService_HandlesNamedTargetPort(t *testing.T) {
	raw := []byte(`{
		"metadata": {"name": "api"},
		"spec": {"type": "ClusterIP", "ports": [{"port": 80, "targetPort": "http", "protocol": "TCP"}]}
	}`)
	got, err := summarizeService(raw)
	if err != nil {
		t.Fatalf("summarizeService: %v", err)
	}
	if got.(serviceSummary).Ports[0].TargetPort != "http" {
		t.Errorf("TargetPort = %q, want %q", got.(serviceSummary).Ports[0].TargetPort, "http")
	}
}

func TestSummarizeService_ErrorsOnMalformedJSON(t *testing.T) {
	if _, err := summarizeService([]byte(`not json`)); err == nil {
		t.Fatal("summarizeService() succeeded on malformed JSON, want error")
	}
}

func TestSummarizeIngress_ParsesHostsFromRules(t *testing.T) {
	raw := []byte(`{
		"metadata": {"name": "web"},
		"spec": {
			"ingressClassName": "nginx",
			"rules": [{"host": "app.example.com"}, {"host": "api.example.com"}]
		}
	}`)
	got, err := summarizeIngress(raw)
	if err != nil {
		t.Fatalf("summarizeIngress: %v", err)
	}
	ing := got.(ingressSummary)
	if ing.Name != "web" || ing.IngressClassName == nil || *ing.IngressClassName != "nginx" {
		t.Errorf("summarizeIngress() = %+v", ing)
	}
	if len(ing.Hosts) != 2 || ing.Hosts[0] != "app.example.com" || ing.Hosts[1] != "api.example.com" {
		t.Errorf("Hosts = %v", ing.Hosts)
	}
}

func TestSummarizeIngress_OmitsEmptyHostRules(t *testing.T) {
	raw := []byte(`{"metadata": {"name": "catch-all"}, "spec": {"rules": [{"host": ""}, {"host": "real.example.com"}]}}`)
	got, err := summarizeIngress(raw)
	if err != nil {
		t.Fatalf("summarizeIngress: %v", err)
	}
	hosts := got.(ingressSummary).Hosts
	if len(hosts) != 1 || hosts[0] != "real.example.com" {
		t.Errorf("Hosts = %v, want only the non-empty host", hosts)
	}
}

func TestSummarizeIngress_ErrorsOnMalformedJSON(t *testing.T) {
	if _, err := summarizeIngress([]byte(`not json`)); err == nil {
		t.Fatal("summarizeIngress() succeeded on malformed JSON, want error")
	}
}

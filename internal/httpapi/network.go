package httpapi

import (
	"encoding/json"
	"fmt"

	"github.com/Saremox/TenantDeck/internal/capsule"
)

var (
	servicesGVR  = capsule.GroupVersionResource{APIPrefix: "api/v1", Resource: "services"}
	ingressesGVR = capsule.GroupVersionResource{APIPrefix: "apis/networking.k8s.io/v1", Resource: "ingresses"}
)

type servicePortSummary struct {
	Name       string `json:"name,omitempty"`
	Port       int32  `json:"port"`
	TargetPort string `json:"targetPort,omitempty"`
	Protocol   string `json:"protocol"`
}

type serviceSummary struct {
	Name      string               `json:"name"`
	Type      string               `json:"type"`
	ClusterIP string               `json:"clusterIP"`
	Ports     []servicePortSummary `json:"ports"`
}

func summarizeService(raw json.RawMessage) (any, error) {
	var obj struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			Type      string `json:"type"`
			ClusterIP string `json:"clusterIP"`
			Ports     []struct {
				Name       string `json:"name"`
				Port       int32  `json:"port"`
				TargetPort any    `json:"targetPort"`
				Protocol   string `json:"protocol"`
			} `json:"ports"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decoding service: %w", err)
	}

	ports := make([]servicePortSummary, 0, len(obj.Spec.Ports))
	for _, p := range obj.Spec.Ports {
		ports = append(ports, servicePortSummary{
			Name: p.Name, Port: p.Port, Protocol: p.Protocol,
			TargetPort: fmt.Sprint(p.TargetPort),
		})
	}
	return serviceSummary{
		Name:      obj.Metadata.Name,
		Type:      obj.Spec.Type,
		ClusterIP: obj.Spec.ClusterIP,
		Ports:     ports,
	}, nil
}

type ingressSummary struct {
	Name             string   `json:"name"`
	IngressClassName *string  `json:"ingressClassName,omitempty"`
	Hosts            []string `json:"hosts"`
}

func summarizeIngress(raw json.RawMessage) (any, error) {
	var obj struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			IngressClassName *string `json:"ingressClassName"`
			Rules            []struct {
				Host string `json:"host"`
			} `json:"rules"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decoding ingress: %w", err)
	}

	hosts := make([]string, 0, len(obj.Spec.Rules))
	for _, rule := range obj.Spec.Rules {
		if rule.Host != "" {
			hosts = append(hosts, rule.Host)
		}
	}
	return ingressSummary{
		Name:             obj.Metadata.Name,
		IngressClassName: obj.Spec.IngressClassName,
		Hosts:            hosts,
	}, nil
}

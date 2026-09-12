package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Saremox/TenantDeck/internal/capsule"
)

var (
	resourceQuotasGVR = capsule.GroupVersionResource{APIPrefix: "api/v1", Resource: "resourcequotas"}
	limitRangesGVR    = capsule.GroupVersionResource{APIPrefix: "api/v1", Resource: "limitranges"}
)

type resourceQuotaSummary struct {
	Name string            `json:"name"`
	Hard map[string]string `json:"hard"`
	Used map[string]string `json:"used"`
}

func summarizeResourceQuota(raw json.RawMessage) (resourceQuotaSummary, error) {
	var obj struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Status struct {
			Hard map[string]string `json:"hard"`
			Used map[string]string `json:"used"`
		} `json:"status"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return resourceQuotaSummary{}, fmt.Errorf("decoding resourcequota: %w", err)
	}
	return resourceQuotaSummary{Name: obj.Metadata.Name, Hard: obj.Status.Hard, Used: obj.Status.Used}, nil
}

type limitRangeItemSummary struct {
	Type           string            `json:"type"`
	Default        map[string]string `json:"default,omitempty"`
	DefaultRequest map[string]string `json:"defaultRequest,omitempty"`
	Max            map[string]string `json:"max,omitempty"`
	Min            map[string]string `json:"min,omitempty"`
}

type limitRangeSummary struct {
	Name   string                  `json:"name"`
	Limits []limitRangeItemSummary `json:"limits"`
}

func summarizeLimitRange(raw json.RawMessage) (limitRangeSummary, error) {
	var obj struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			Limits []struct {
				Type           string            `json:"type"`
				Default        map[string]string `json:"default"`
				DefaultRequest map[string]string `json:"defaultRequest"`
				Max            map[string]string `json:"max"`
				Min            map[string]string `json:"min"`
			} `json:"limits"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return limitRangeSummary{}, fmt.Errorf("decoding limitrange: %w", err)
	}
	limits := make([]limitRangeItemSummary, 0, len(obj.Spec.Limits))
	for _, l := range obj.Spec.Limits {
		limits = append(limits, limitRangeItemSummary{
			Type: l.Type, Default: l.Default, DefaultRequest: l.DefaultRequest, Max: l.Max, Min: l.Min,
		})
	}
	return limitRangeSummary{Name: obj.Metadata.Name, Limits: limits}, nil
}

type overviewResponse struct {
	ResourceQuotas []resourceQuotaSummary `json:"resourceQuotas"`
	LimitRanges    []limitRangeSummary    `json:"limitRanges"`
}

// overviewHandler implements GET /api/namespaces/{ns}/overview
// (docs/route-allowlist.md): the namespace's ResourceQuota and LimitRange
// objects combined into one response, since the frontend always wants both
// together on one overview panel.
func overviewHandler(client *capsule.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec, ok := sessionFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		namespace := r.PathValue("ns")
		if !validK8sName(namespace) {
			http.Error(w, "invalid namespace", http.StatusBadRequest)
			return
		}

		quotaItems, err := client.ListNamespacedResource(r.Context(), rec.IDToken, namespace, resourceQuotasGVR, 0)
		if err != nil {
			writeUpstreamError(w, r, err)
			return
		}
		limitItems, err := client.ListNamespacedResource(r.Context(), rec.IDToken, namespace, limitRangesGVR, 0)
		if err != nil {
			writeUpstreamError(w, r, err)
			return
		}

		resp := overviewResponse{
			ResourceQuotas: make([]resourceQuotaSummary, 0, len(quotaItems)),
			LimitRanges:    make([]limitRangeSummary, 0, len(limitItems)),
		}
		for _, raw := range quotaItems {
			s, err := summarizeResourceQuota(raw)
			if err != nil {
				slog.ErrorContext(r.Context(), "decoding resourcequota", "error", err)
				http.Error(w, "upstream returned an unexpected shape", http.StatusBadGateway)
				return
			}
			resp.ResourceQuotas = append(resp.ResourceQuotas, s)
		}
		for _, raw := range limitItems {
			s, err := summarizeLimitRange(raw)
			if err != nil {
				slog.ErrorContext(r.Context(), "decoding limitrange", "error", err)
				http.Error(w, "upstream returned an unexpected shape", http.StatusBadGateway)
				return
			}
			resp.LimitRanges = append(resp.LimitRanges, s)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}

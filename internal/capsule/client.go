// Package capsule is the upstream HTTP client for the existing Capsule
// Proxy. Every request it builds comes from an explicit allowlist - see
// docs/route-allowlist.md - never from forwarding an incoming browser
// request's headers. This is the boundary docs/spec/05-upstream-boundary-and-resilience.md
// describes.
package capsule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTimeout = 10 * time.Second

	// maxResponseBytes bounds an ordinary upstream response, per
	// docs/spec/05-upstream-boundary-and-resilience.md ("bound ... ordinary
	// upstream response sizes"). Namespace lists are small; this is not the
	// bound for bulk/streaming endpoints like pod logs, which will need
	// their own, separate limit when that route is implemented.
	maxResponseBytes = 10 << 20 // 10 MiB
)

// ErrUnauthorized and ErrForbidden let callers distinguish "the caller's
// credentials are no longer valid upstream" from "the caller is
// authenticated but Capsule denies this access" - docs/spec/05 requires
// preserving, not flattening, Kubernetes authorization outcomes.
var (
	ErrUnauthorized = errors.New("capsule: upstream rejected the credentials")
	ErrForbidden    = errors.New("capsule: upstream denied access")
)

// Namespace is the minimal shape TenantDeck exposes from a Capsule Proxy
// namespace list - see docs/route-allowlist.md.
type Namespace struct {
	Name string
}

// Client calls the fixed, trusted Capsule Proxy origin.
type Client struct {
	baseURL    string
	httpClient *http.Client
	// streamClient is used only for StreamPodLogs: it deliberately has no
	// overall http.Client.Timeout (which would otherwise cut off a "follow"
	// stream at a fixed wall-clock duration regardless of activity).
	// Callers bound stream lifetime via context instead - see logs.go.
	streamClient *http.Client
}

// NewClient builds a Client for baseURL. baseURL must come from trusted
// configuration (internal/config), never from a caller-supplied value. A
// nil httpClient gets a default with a bounded timeout.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	streamClient := &http.Client{Transport: httpClient.Transport}
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		httpClient:   httpClient,
		streamClient: streamClient,
	}
}

// ListNamespaces lists the namespaces visible to idToken's identity, via
// Capsule Proxy's own filtering - see docs/spec/03-architecture.md ("Capsule
// Proxy itself filters to the caller's tenant; the BFF does not compute
// this list").
func (c *Client) ListNamespaces(ctx context.Context, idToken string) ([]Namespace, error) {
	data, err := c.do(ctx, idToken, "/api/v1/namespaces")
	if err != nil {
		return nil, err
	}

	var list rawList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("capsule: decoding namespace list: %w", err)
	}

	namespaces := make([]Namespace, 0, len(list.Items))
	for _, raw := range list.Items {
		var item struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, fmt.Errorf("capsule: decoding namespace item: %w", err)
		}
		namespaces = append(namespaces, Namespace{Name: item.Metadata.Name})
	}
	return namespaces, nil
}

// GroupVersionResource identifies one namespaced Kubernetes resource type to
// list or get - see docs/route-allowlist.md for the exact set TenantDeck
// allows. APIPrefix is "api/v1" for the core group, or "apis/<group>/<version>"
// for a named group (e.g. "apis/apps/v1").
type GroupVersionResource struct {
	APIPrefix string
	Resource  string
}

// ListNamespacedResource lists gvr in namespace, returning each item's raw
// JSON for the caller to decode into its own small summary type - this
// client stays agnostic of any one resource kind's fields. limit bounds
// the page size if positive (docs/spec/05-upstream-boundary-and-resilience.md
// "bound pagination"); 0 or negative leaves it unset.
func (c *Client) ListNamespacedResource(ctx context.Context, idToken, namespace string, gvr GroupVersionResource, limit int64) ([]json.RawMessage, error) {
	path := fmt.Sprintf("/%s/namespaces/%s/%s", gvr.APIPrefix, namespace, gvr.Resource)
	if limit > 0 {
		path += "?limit=" + strconv.FormatInt(limit, 10)
	}
	data, err := c.do(ctx, idToken, path)
	if err != nil {
		return nil, err
	}
	var list rawList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("capsule: decoding %s list: %w", gvr.Resource, err)
	}
	return list.Items, nil
}

// GetNamespacedResource fetches one gvr object by name in namespace.
func (c *Client) GetNamespacedResource(ctx context.Context, idToken, namespace, name string, gvr GroupVersionResource) (json.RawMessage, error) {
	path := fmt.Sprintf("/%s/namespaces/%s/%s/%s", gvr.APIPrefix, namespace, gvr.Resource, name)
	return c.do(ctx, idToken, path)
}

// rawList is the minimal shape every Kubernetes List object shares -
// everything else is left for the caller's own summary type to decode.
type rawList struct {
	Items []json.RawMessage `json:"items"`
}

// do builds and sends one GET request to path with the standard, explicit
// header allowlist (see docs/route-allowlist.md), and returns the bounded
// response body on a 2xx OK status.
func (c *Client) do(ctx context.Context, idToken, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}

	// Built from nothing, not copied from an incoming request: this is the
	// entire outbound header allowlist for every call this client makes.
	req.Header = http.Header{}
	req.Header.Set("Authorization", "Bearer "+idToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("capsule: requesting %s: %w", path, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case http.StatusForbidden:
		return nil, ErrForbidden
	default:
		return nil, fmt.Errorf("capsule: unexpected upstream status %d for %s", resp.StatusCode, path)
	}

	return readBounded(resp.Body, maxResponseBytes)
}

func readBounded(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, fmt.Errorf("capsule: reading response: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("capsule: response exceeded %d bytes", limit)
	}
	return data, nil
}

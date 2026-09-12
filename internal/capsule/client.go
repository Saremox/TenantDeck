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
}

// NewClient builds a Client for baseURL. baseURL must come from trusted
// configuration (internal/config), never from a caller-supplied value. A
// nil httpClient gets a default with a bounded timeout.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), httpClient: httpClient}
}

// ListNamespaces lists the namespaces visible to idToken's identity, via
// Capsule Proxy's own filtering - see docs/spec/03-architecture.md ("Capsule
// Proxy itself filters to the caller's tenant; the BFF does not compute
// this list").
func (c *Client) ListNamespaces(ctx context.Context, idToken string) ([]Namespace, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/namespaces", nil)
	if err != nil {
		return nil, err
	}

	// Built from nothing, not copied from an incoming request: this is the
	// entire outbound header allowlist for this call.
	req.Header = http.Header{}
	req.Header.Set("Authorization", "Bearer "+idToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("capsule: listing namespaces: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case http.StatusForbidden:
		return nil, ErrForbidden
	default:
		return nil, fmt.Errorf("capsule: unexpected upstream status %d", resp.StatusCode)
	}

	data, err := readBounded(resp.Body, maxResponseBytes)
	if err != nil {
		return nil, err
	}

	var list namespaceList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("capsule: decoding namespace list: %w", err)
	}

	namespaces := make([]Namespace, 0, len(list.Items))
	for _, item := range list.Items {
		namespaces = append(namespaces, Namespace{Name: item.Metadata.Name})
	}
	return namespaces, nil
}

// namespaceList is the minimal subset of a Kubernetes core/v1 NamespaceList
// TenantDeck reads - everything else in the real object is ignored.
type namespaceList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	} `json:"items"`
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

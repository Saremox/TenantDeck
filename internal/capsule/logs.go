package capsule

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// LogOptions bounds one pod-logs request. A zero value of any numeric
// field means "don't send that query parameter" (upstream's own default
// applies) - callers (internal/httpapi) are responsible for clamping these
// to sane maxima before calling StreamPodLogs, per
// docs/spec/05-upstream-boundary-and-resilience.md ("bound ... log tail/
// bytes").
type LogOptions struct {
	Container    string
	TailLines    int64
	LimitBytes   int64
	SinceSeconds int64
	Follow       bool
}

// StreamPodLogs opens a (possibly long-lived, if Follow) stream of a pod's
// logs. The caller owns the returned ReadCloser and must Close it - and
// should bound ctx's lifetime, since that's the only thing that stops a
// follow stream (see the streamClient note on NewClient: it intentionally
// has no wall-clock http.Client.Timeout, unlike ordinary bounded calls).
func (c *Client) StreamPodLogs(ctx context.Context, idToken, namespace, podName string, opts LogOptions) (io.ReadCloser, error) {
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/log", namespace, podName)

	q := url.Values{}
	if opts.Container != "" {
		q.Set("container", opts.Container)
	}
	if opts.TailLines > 0 {
		q.Set("tailLines", strconv.FormatInt(opts.TailLines, 10))
	}
	if opts.LimitBytes > 0 {
		q.Set("limitBytes", strconv.FormatInt(opts.LimitBytes, 10))
	}
	if opts.SinceSeconds > 0 {
		q.Set("sinceSeconds", strconv.FormatInt(opts.SinceSeconds, 10))
	}
	if opts.Follow {
		q.Set("follow", "true")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header = http.Header{}
	req.Header.Set("Authorization", "Bearer "+idToken)
	req.Header.Set("Accept", "text/plain, */*")

	resp, err := c.streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("capsule: requesting pod logs: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return resp.Body, nil
	case http.StatusUnauthorized:
		resp.Body.Close()
		return nil, ErrUnauthorized
	case http.StatusForbidden:
		resp.Body.Close()
		return nil, ErrForbidden
	default:
		resp.Body.Close()
		return nil, fmt.Errorf("capsule: unexpected upstream status %d for pod logs", resp.StatusCode)
	}
}

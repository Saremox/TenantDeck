package httpapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Saremox/TenantDeck/internal/capsule"
)

const (
	defaultTailLines = 200
	maxTailLines     = 2000

	defaultLimitBytes = 256 << 10 // 256 KiB
	maxLimitBytes     = 1 << 20   // 1 MiB - what a caller may request

	// maxStreamBytes is an absolute backstop independent of limitBytes,
	// in case an upstream doesn't honor that parameter strictly - applies
	// even in follow mode. docs/spec/05-upstream-boundary-and-resilience.md
	// ("bound ... log tail/bytes").
	maxStreamBytes = 50 << 20 // 50 MiB

	// maxLogStreamDuration bounds every pod-logs request, follow or not -
	// "prevent long streams from outliving the dashboard session
	// indefinitely." A client that wants more just reconnects.
	maxLogStreamDuration = 10 * time.Minute
)

// podLogsHandler implements GET /api/namespaces/{ns}/pods/{name}/logs
// (docs/route-allowlist.md): bounded, optionally-following log streaming.
// Cancellation (client disconnect, or the duration bound above) terminates
// the upstream read - see internal/capsule's StreamPodLogs and its
// context-cancellation test.
func podLogsHandler(client *capsule.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec, ok := sessionFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		namespace, name := r.PathValue("ns"), r.PathValue("name")
		if !validK8sName(namespace) || !validK8sName(name) {
			http.Error(w, "invalid namespace or name", http.StatusBadRequest)
			return
		}

		opts, err := parseLogOptions(r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), maxLogStreamDuration)
		defer cancel()

		stream, err := client.StreamPodLogs(ctx, rec.IDToken, namespace, name, opts)
		if err != nil {
			writeUpstreamError(w, r, err)
			return
		}
		defer stream.Close()

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		copyLogStream(w, flusher, io.LimitReader(stream, maxStreamBytes))
	})
}

// copyLogStream copies src to w, flushing after every chunk so "follow"
// output reaches the browser as it arrives rather than sitting in a
// buffer. Returns (stops) on the first write or read error - a write error
// means the client disconnected; a read error means the upstream stream
// ended, failed, or its context was canceled.
func copyLogStream(w io.Writer, flusher http.Flusher, src io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr != nil {
			return
		}
	}
}

func parseLogOptions(q url.Values) (capsule.LogOptions, error) {
	opts := capsule.LogOptions{
		Follow:     q.Get("follow") == "true",
		TailLines:  defaultTailLines,
		LimitBytes: defaultLimitBytes,
	}

	if v := q.Get("tailLines"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return opts, fmt.Errorf("invalid tailLines")
		}
		opts.TailLines = min(n, maxTailLines)
	}
	if v := q.Get("limitBytes"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return opts, fmt.Errorf("invalid limitBytes")
		}
		opts.LimitBytes = min(n, maxLimitBytes)
	}
	if v := q.Get("sinceSeconds"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return opts, fmt.Errorf("invalid sinceSeconds")
		}
		opts.SinceSeconds = n
	}
	if v := q.Get("container"); v != "" {
		if !validK8sName(v) {
			return opts, fmt.Errorf("invalid container")
		}
		opts.Container = v
	}
	return opts, nil
}

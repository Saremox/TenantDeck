package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Saremox/TenantDeck/internal/capsule"
	"github.com/Saremox/TenantDeck/internal/session"
)

func TestParseLogOptions_UsesDefaultsWhenNoParamsGiven(t *testing.T) {
	opts, err := parseLogOptions(url.Values{})
	if err != nil {
		t.Fatalf("parseLogOptions: %v", err)
	}
	if opts.TailLines != defaultTailLines || opts.LimitBytes != defaultLimitBytes || opts.Follow {
		t.Errorf("opts = %+v, want the documented defaults", opts)
	}
}

func TestParseLogOptions_ClampsTailLinesToMax(t *testing.T) {
	opts, err := parseLogOptions(url.Values{"tailLines": {"999999"}})
	if err != nil {
		t.Fatalf("parseLogOptions: %v", err)
	}
	if opts.TailLines != maxTailLines {
		t.Errorf("TailLines = %d, want clamped to %d", opts.TailLines, maxTailLines)
	}
}

func TestParseLogOptions_ClampsLimitBytesToMax(t *testing.T) {
	opts, err := parseLogOptions(url.Values{"limitBytes": {"999999999"}})
	if err != nil {
		t.Fatalf("parseLogOptions: %v", err)
	}
	if opts.LimitBytes != maxLimitBytes {
		t.Errorf("LimitBytes = %d, want clamped to %d", opts.LimitBytes, maxLimitBytes)
	}
}

func TestParseLogOptions_RejectsNonNumericTailLines(t *testing.T) {
	if _, err := parseLogOptions(url.Values{"tailLines": {"abc"}}); err == nil {
		t.Fatal("parseLogOptions() succeeded with non-numeric tailLines, want error")
	}
}

func TestParseLogOptions_RejectsZeroOrNegativeTailLines(t *testing.T) {
	if _, err := parseLogOptions(url.Values{"tailLines": {"0"}}); err == nil {
		t.Fatal("parseLogOptions() succeeded with tailLines=0, want error")
	}
	if _, err := parseLogOptions(url.Values{"tailLines": {"-5"}}); err == nil {
		t.Fatal("parseLogOptions() succeeded with negative tailLines, want error")
	}
}

func TestParseLogOptions_RejectsNonNumericLimitBytes(t *testing.T) {
	if _, err := parseLogOptions(url.Values{"limitBytes": {"abc"}}); err == nil {
		t.Fatal("parseLogOptions() succeeded with non-numeric limitBytes, want error")
	}
}

func TestParseLogOptions_RejectsZeroOrNegativeLimitBytes(t *testing.T) {
	if _, err := parseLogOptions(url.Values{"limitBytes": {"0"}}); err == nil {
		t.Fatal("parseLogOptions() succeeded with limitBytes=0, want error")
	}
}

func TestParseLogOptions_AcceptsValidSinceSeconds(t *testing.T) {
	opts, err := parseLogOptions(url.Values{"sinceSeconds": {"60"}})
	if err != nil {
		t.Fatalf("parseLogOptions: %v", err)
	}
	if opts.SinceSeconds != 60 {
		t.Errorf("SinceSeconds = %d, want 60", opts.SinceSeconds)
	}
}

func TestParseLogOptions_RejectsNonNumericSinceSeconds(t *testing.T) {
	if _, err := parseLogOptions(url.Values{"sinceSeconds": {"abc"}}); err == nil {
		t.Fatal("parseLogOptions() succeeded with non-numeric sinceSeconds, want error")
	}
}

func TestParseLogOptions_RejectsZeroOrNegativeSinceSeconds(t *testing.T) {
	if _, err := parseLogOptions(url.Values{"sinceSeconds": {"-1"}}); err == nil {
		t.Fatal("parseLogOptions() succeeded with negative sinceSeconds, want error")
	}
}

func TestParseLogOptions_AcceptsValidContainerName(t *testing.T) {
	opts, err := parseLogOptions(url.Values{"container": {"app"}})
	if err != nil {
		t.Fatalf("parseLogOptions: %v", err)
	}
	if opts.Container != "app" {
		t.Errorf("Container = %q, want %q", opts.Container, "app")
	}
}

func TestParseLogOptions_RejectsInvalidContainerName(t *testing.T) {
	if _, err := parseLogOptions(url.Values{"container": {"../etc"}}); err == nil {
		t.Fatal("parseLogOptions() succeeded with an invalid container name, want error")
	}
}

func TestParseLogOptions_SetsFollowWhenRequested(t *testing.T) {
	opts, err := parseLogOptions(url.Values{"follow": {"true"}})
	if err != nil {
		t.Fatalf("parseLogOptions: %v", err)
	}
	if !opts.Follow {
		t.Error("Follow = false, want true")
	}
}

func logsRequest(t *testing.T, query string) *http.Request {
	t.Helper()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet,
		"/api/namespaces/ns/pods/web-1/logs?"+query, map[string]string{"ns": "ns", "name": "web-1"})
	return req
}

func TestPodLogsHandler_StreamsUpstreamContent(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("line one\nline two\n"))
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	podLogsHandler(client).ServeHTTP(rec, logsRequest(t, ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "line one\nline two\n" {
		t.Errorf("body = %q, want the exact log content", rec.Body.String())
	}
}

func TestPodLogsHandler_ForwardsTailLinesAndFollowToUpstream(t *testing.T) {
	var gotQuery url.Values
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	podLogsHandler(client).ServeHTTP(rec, logsRequest(t, "tailLines=50&follow=true"))

	if gotQuery.Get("tailLines") != "50" {
		t.Errorf("tailLines = %q, want %q", gotQuery.Get("tailLines"), "50")
	}
	if gotQuery.Get("follow") != "true" {
		t.Errorf("follow = %q, want %q", gotQuery.Get("follow"), "true")
	}
}

func TestPodLogsHandler_RejectsInvalidQueryParams(t *testing.T) {
	client := capsule.NewClient("http://unused.invalid", nil)
	rec := httptest.NewRecorder()
	podLogsHandler(client).ServeHTTP(rec, logsRequest(t, "tailLines=not-a-number"))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPodLogsHandler_RejectsInvalidPodName(t *testing.T) {
	client := capsule.NewClient("http://unused.invalid", nil)
	rec := httptest.NewRecorder()
	req := requestWithSessionPath(t, session.Record{IDToken: "itok"}, http.MethodGet,
		"/api/namespaces/ns/pods/../logs", map[string]string{"ns": "ns", "name": ".."})
	podLogsHandler(client).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPodLogsHandler_MapsUpstreamForbiddenTo403(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder()
	podLogsHandler(client).ServeHTTP(rec, logsRequest(t, ""))

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestPodLogsHandler_RejectsRequestWithNoSessionInContext(t *testing.T) {
	client := capsule.NewClient("http://unused.invalid", nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/namespaces/ns/pods/web-1/logs", nil)
	req.SetPathValue("ns", "ns")
	req.SetPathValue("name", "web-1")
	podLogsHandler(client).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// httptest.NewRecorder doesn't implement http.Flusher, so this also proves
// copyLogStream tolerates that (a real net/http ResponseWriter always
// does, but this keeps the handler from assuming so).
func TestPodLogsHandler_WorksWithoutAFlusher(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("log line\n"))
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	rec := httptest.NewRecorder() // *httptest.ResponseRecorder has no Flush method
	podLogsHandler(client).ServeHTTP(rec, logsRequest(t, ""))

	if rec.Body.String() != "log line\n" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "log line\n")
	}
}

func TestPodLogsHandler_StopsStreamingOnRequestContextCancellation(t *testing.T) {
	started := make(chan struct{})
	blockForever := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("first\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		close(started)
		<-blockForever
	}))
	defer upstream.Close()

	client := capsule.NewClient(upstream.URL, nil)
	req := logsRequest(t, "follow=true")
	cancelCtx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(cancelCtx)

	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		podLogsHandler(client).ServeHTTP(rec, req)
		close(done)
	}()

	<-started
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("podLogsHandler did not return after request context cancellation")
	}
	close(blockForever)
}

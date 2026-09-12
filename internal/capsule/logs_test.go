package capsule

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestStreamPodLogs_RequestsExactlyTheExpectedPathAndQuery(t *testing.T) {
	var gotPath string
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.Query()
		w.Write([]byte("log line 1\n"))
	}))
	defer srv.Close()

	rc, err := NewClient(srv.URL, nil).StreamPodLogs(context.Background(), "token", "ns", "pod-1", LogOptions{
		TailLines:  50,
		LimitBytes: 1024,
		Container:  "app",
	})
	if err != nil {
		t.Fatalf("StreamPodLogs: %v", err)
	}
	defer rc.Close()
	io.ReadAll(rc)

	if want := "/api/v1/namespaces/ns/pods/pod-1/log"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if gotQuery.Get("tailLines") != "50" {
		t.Errorf("tailLines = %q, want %q", gotQuery.Get("tailLines"), "50")
	}
	if gotQuery.Get("limitBytes") != "1024" {
		t.Errorf("limitBytes = %q, want %q", gotQuery.Get("limitBytes"), "1024")
	}
	if gotQuery.Get("container") != "app" {
		t.Errorf("container = %q, want %q", gotQuery.Get("container"), "app")
	}
	if gotQuery.Has("follow") {
		t.Error("follow param present, want absent when Follow is false")
	}
}

func TestStreamPodLogs_SetsFollowParamWhenRequested(t *testing.T) {
	var gotFollow string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFollow = r.URL.Query().Get("follow")
	}))
	defer srv.Close()

	rc, err := NewClient(srv.URL, nil).StreamPodLogs(context.Background(), "token", "ns", "pod-1", LogOptions{Follow: true})
	if err != nil {
		t.Fatalf("StreamPodLogs: %v", err)
	}
	rc.Close()

	if gotFollow != "true" {
		t.Errorf("follow = %q, want %q", gotFollow, "true")
	}
}

func TestStreamPodLogs_ReturnsStreamedContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("line one\nline two\n"))
	}))
	defer srv.Close()

	rc, err := NewClient(srv.URL, nil).StreamPodLogs(context.Background(), "token", "ns", "pod-1", LogOptions{})
	if err != nil {
		t.Fatalf("StreamPodLogs: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("reading stream: %v", err)
	}
	if string(data) != "line one\nline two\n" {
		t.Errorf("data = %q, want the exact log content", data)
	}
}

func TestStreamPodLogs_ReturnsErrUnauthorizedOn401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, nil).StreamPodLogs(context.Background(), "token", "ns", "pod-1", LogOptions{})
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("err = %v, want ErrUnauthorized", err)
	}
}

func TestStreamPodLogs_ReturnsErrForbiddenOn403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, nil).StreamPodLogs(context.Background(), "token", "ns", "pod-1", LogOptions{})
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
}

func TestStreamPodLogs_ReturnsErrorForInvalidBaseURL(t *testing.T) {
	client := NewClient("http://\x7f", nil)
	if _, err := client.StreamPodLogs(context.Background(), "token", "ns", "pod-1", LogOptions{}); err == nil {
		t.Fatal("StreamPodLogs() succeeded with an invalid base URL, want error")
	}
}

func TestStreamPodLogs_ReturnsErrorOnUnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL, nil).StreamPodLogs(context.Background(), "token", "ns", "pod-1", LogOptions{}); err == nil {
		t.Fatal("StreamPodLogs() succeeded on a 500 response, want error")
	}
}

// This is the property follow mode depends on: the stream must not be cut
// off by a wall-clock http.Client.Timeout while data is still arriving
// slowly but steadily - only context cancellation (tested next) should
// stop it.
func TestStreamPodLogs_DoesNotTimeOutDuringASlowButActiveFollowStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		for i := 0; i < 3; i++ {
			w.Write([]byte("line\n"))
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(60 * time.Millisecond)
		}
	}))
	defer srv.Close()

	// A client with a short ordinary Timeout, to prove StreamPodLogs's own
	// streamClient (built from this client's Transport but without that
	// Timeout) is what's actually used - not the bounded one.
	shortTimeoutClient := &http.Client{Timeout: 50 * time.Millisecond}
	rc, err := NewClient(srv.URL, shortTimeoutClient).StreamPodLogs(context.Background(), "token", "ns", "pod-1", LogOptions{Follow: true})
	if err != nil {
		t.Fatalf("StreamPodLogs: %v", err)
	}
	defer rc.Close()

	scanner := bufio.NewScanner(rc)
	lines := 0
	for scanner.Scan() {
		lines++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading stream: %v (timed out despite active data, want full stream)", err)
	}
	if lines != 3 {
		t.Errorf("read %d lines, want 3", lines)
	}
}

func TestStreamPodLogs_StopsWhenContextIsCanceled(t *testing.T) {
	// Closed explicitly (not via defer) before srv.Close() runs: srv.Close()
	// waits for in-flight handlers to return, and the handler below blocks
	// on this channel until we release it - deferring both in the wrong
	// order would deadlock the test.
	blockForever := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("line\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-blockForever // hold the connection open until the test releases it
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	rc, err := NewClient(srv.URL, nil).StreamPodLogs(ctx, "token", "ns", "pod-1", LogOptions{Follow: true})
	if err != nil {
		t.Fatalf("StreamPodLogs: %v", err)
	}
	defer rc.Close()

	buf := make([]byte, 5)
	if _, err := io.ReadFull(rc, buf); err != nil {
		t.Fatalf("reading first line: %v", err)
	}

	cancel()
	readErrCh := make(chan error, 1)
	go func() {
		_, err := rc.Read(make([]byte, 16))
		readErrCh <- err
	}()

	select {
	case err := <-readErrCh:
		if err == nil {
			t.Error("Read() after context cancellation succeeded, want an error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Read() did not return after context cancellation")
	}

	close(blockForever)
}

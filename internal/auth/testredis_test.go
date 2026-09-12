package auth

import (
	"net"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/Saremox/TenantDeck/internal/session"
)

// newTestStore starts a real redis-server for the test's duration and
// returns a ready session.Store backed by it - the same real-dependency
// approach used in internal/session's own tests (ADR-004), duplicated here
// rather than exported test-only surface from the session package.
func newTestStore(t *testing.T) *session.Store {
	t.Helper()
	if _, err := exec.LookPath("redis-server"); err != nil {
		t.Skip("redis-server not found on PATH, skipping test that needs a session store")
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("finding a free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	addr := "127.0.0.1:" + strconv.Itoa(port)

	cmd := exec.Command("redis-server",
		"--port", strconv.Itoa(port),
		"--bind", "127.0.0.1",
		"--save", "",
		"--appendonly", "no",
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting redis-server: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond); err == nil {
			conn.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	backend := session.NewRedisBackend(session.RedisOptions{Addr: addr})
	t.Cleanup(func() { _ = backend.Close() })

	key := make([]byte, session.SessionKeySize)
	store, err := session.NewStore(backend, key)
	if err != nil {
		t.Fatalf("session.NewStore: %v", err)
	}
	return store
}

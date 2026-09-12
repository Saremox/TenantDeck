package session

import (
	"context"
	"errors"
	"net"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// startTestRedis launches a real redis-server for the duration of the test
// and returns its address. This is a real integration test against the
// actual dependency (ADR-004), not a fake — docs/spec/07-mandatory-automated-testing.md
// asks for real infrastructure wherever practical. Skips if redis-server
// isn't on PATH rather than failing environments that don't have it.
func startTestRedis(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("redis-server"); err != nil {
		t.Skip("redis-server not found on PATH, skipping real-Redis integration test")
	}

	port := findFreePort(t)
	addr := "127.0.0.1:" + strconv.Itoa(port)

	cmd := exec.Command("redis-server",
		"--port", strconv.Itoa(port),
		"--bind", "127.0.0.1",
		"--save", "",
		"--appendonly", "no",
		"--daemonize", "no",
	)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting redis-server: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	waitForRedis(t, addr)
	return addr
}

func findFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("finding a free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func waitForRedis(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("redis-server at %s did not become reachable in time", addr)
}

func TestRedisBackend_GetRecoversWhatSetStored(t *testing.T) {
	addr := startTestRedis(t)
	b := NewRedisBackend(RedisOptions{Addr: addr})
	defer b.Close()
	ctx := context.Background()

	if err := b.Set(ctx, "k1", []byte("hello"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := b.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("Get() = %q, want %q", got, "hello")
	}
}

func TestRedisBackend_GetReturnsErrNotFoundForMissingKey(t *testing.T) {
	addr := startTestRedis(t)
	b := NewRedisBackend(RedisOptions{Addr: addr})
	defer b.Close()

	_, err := b.Get(context.Background(), "never-set")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() = %v, want ErrNotFound", err)
	}
}

func TestRedisBackend_KeyExpiresAfterTTL(t *testing.T) {
	addr := startTestRedis(t)
	b := NewRedisBackend(RedisOptions{Addr: addr})
	defer b.Close()
	ctx := context.Background()

	if err := b.Set(ctx, "k1", []byte("hello"), 150*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}
	time.Sleep(400 * time.Millisecond)

	if _, err := b.Get(ctx, "k1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() after TTL expiry = %v, want ErrNotFound", err)
	}
}

func TestRedisBackend_GetDelReturnsErrNotFoundForMissingKey(t *testing.T) {
	addr := startTestRedis(t)
	b := NewRedisBackend(RedisOptions{Addr: addr})
	defer b.Close()

	if _, err := b.GetDel(context.Background(), "never-set"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetDel() = %v, want ErrNotFound", err)
	}
}

func TestRedisBackend_GetDelRemovesKeyAfterOneRead(t *testing.T) {
	addr := startTestRedis(t)
	b := NewRedisBackend(RedisOptions{Addr: addr})
	defer b.Close()
	ctx := context.Background()

	if err := b.Set(ctx, "k1", []byte("hello"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := b.GetDel(ctx, "k1")
	if err != nil {
		t.Fatalf("GetDel: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("GetDel() = %q, want %q", got, "hello")
	}
	if _, err := b.Get(ctx, "k1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() after GetDel = %v, want ErrNotFound", err)
	}
}

// This is the fail-closed requirement exercised against the real client:
// an unreachable Redis must surface as an error, not a silent empty result.
func TestRedisBackend_OperationsFailWhenServerUnreachable(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1", // nothing listens here
		DialTimeout: 300 * time.Millisecond,
	})
	t.Cleanup(func() { _ = client.Close() })
	b := &RedisBackend{client: client}

	if err := b.Set(context.Background(), "k1", []byte("x"), time.Minute); err == nil {
		t.Error("Set() succeeded against an unreachable server, want error")
	}
	if _, err := b.Get(context.Background(), "k1"); err == nil {
		t.Error("Get() succeeded against an unreachable server, want error")
	}
	if _, err := b.GetDel(context.Background(), "k1"); err == nil {
		t.Error("GetDel() succeeded against an unreachable server, want error")
	}
	if err := b.Del(context.Background(), "k1"); err == nil {
		t.Error("Del() succeeded against an unreachable server, want error")
	}
}

func TestStore_EndToEndAgainstRealRedis(t *testing.T) {
	addr := startTestRedis(t)
	b := NewRedisBackend(RedisOptions{Addr: addr})
	defer b.Close()
	s := newTestStore(t, b)
	ctx := context.Background()

	want := Record{Subject: "alice", IDToken: "header.payload.sig", ExpiresAt: time.Now().Add(time.Hour)}
	id, err := s.CreateSession(ctx, want, time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := s.GetSession(ctx, id)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Subject != want.Subject {
		t.Errorf("GetSession().Subject = %q, want %q", got.Subject, want.Subject)
	}

	// Prove the record is actually encrypted at rest, not just JSON, by
	// reading the raw bytes straight out of Redis.
	raw, err := b.Get(ctx, "session:"+id)
	if err != nil {
		t.Fatalf("reading raw stored value: %v", err)
	}
	if containsPlaintextSubject(raw, want.Subject) {
		t.Error("raw stored session value contains the plaintext subject; it must be encrypted")
	}

	if err := s.DeleteSession(ctx, id); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := s.GetSession(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetSession() after delete = %v, want ErrNotFound", err)
	}
}

func containsPlaintextSubject(raw []byte, subject string) bool {
	return len(subject) > 0 && bytesContains(raw, []byte(subject))
}

func bytesContains(haystack, needle []byte) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if string(haystack[i:i+len(needle)]) == string(needle) {
			return true
		}
	}
	return false
}

package session

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when a key doesn't exist or has expired.
var ErrNotFound = errors.New("session: not found")

// backend is the minimal key/value operations the session store needs from
// Valkey/Redis. Kept as an interface so tests can exercise store.go against
// a fake without a running Redis, and separately against a real one.
type backend interface {
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Get(ctx context.Context, key string) ([]byte, error)    // returns ErrNotFound if absent
	GetDel(ctx context.Context, key string) ([]byte, error) // returns ErrNotFound if absent
	Del(ctx context.Context, key string) error
}

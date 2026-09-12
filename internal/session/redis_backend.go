package session

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisBackend implements backend against Valkey/Redis over go-redis.
// Works unmodified against either, per ADR-004 in docs/implementation-plan.md.
type RedisBackend struct {
	client *redis.Client
}

// RedisOptions configures the connection. TLSConfig is nil for a plain
// connection; callers wanting a custom CA build their own *tls.Config and
// set it here rather than TenantDeck reimplementing CA handling.
type RedisOptions struct {
	Addr      string
	Username  string
	Password  string
	TLSConfig *tls.Config
}

func NewRedisBackend(opts RedisOptions) *RedisBackend {
	return &RedisBackend{client: redis.NewClient(&redis.Options{
		Addr:      opts.Addr,
		Username:  opts.Username,
		Password:  opts.Password,
		TLSConfig: opts.TLSConfig,
	})}
}

func (b *RedisBackend) Close() error {
	return b.client.Close()
}

func (b *RedisBackend) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return b.client.Set(ctx, key, value, ttl).Err()
}

func (b *RedisBackend) Get(ctx context.Context, key string) ([]byte, error) {
	v, err := b.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, ErrNotFound
	}
	return v, err
}

func (b *RedisBackend) GetDel(ctx context.Context, key string) ([]byte, error) {
	v, err := b.client.GetDel(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, ErrNotFound
	}
	return v, err
}

func (b *RedisBackend) Del(ctx context.Context, key string) error {
	return b.client.Del(ctx, key).Err()
}

func (b *RedisBackend) Ping(ctx context.Context) error {
	return b.client.Ping(ctx).Err()
}

package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// BuildRedisTLSConfig returns the *tls.Config to pass to
// session.NewRedisBackend, or nil when TLS isn't enabled. A configured CA
// file is read and parsed here - fail closed: an unreadable file or one
// with no valid certificates is an error, never a silent fallback to an
// unverified connection or the system pool alone - since
// docs/spec/06-container-and-kubernetes-deployment.md requires custom CA
// support for a private Valkey/Redis deployment, not just "TLS on."
func (s SessionConfig) BuildRedisTLSConfig() (*tls.Config, error) {
	if !s.RedisTLS {
		return nil, nil
	}
	if s.RedisCAFile == "" {
		return &tls.Config{}, nil
	}

	pemBytes, err := os.ReadFile(s.RedisCAFile)
	if err != nil {
		return nil, fmt.Errorf("reading redis CA file %q: %w", s.RedisCAFile, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil, fmt.Errorf("redis CA file %q contains no valid PEM certificates", s.RedisCAFile)
	}
	return &tls.Config{RootCAs: pool}, nil
}

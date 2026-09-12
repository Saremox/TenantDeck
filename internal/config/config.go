// Package config loads TenantDeck's runtime configuration from the
// environment. There are no defaults for anything security-relevant —
// missing required values fail startup rather than falling back to an
// insecure default.
package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Saremox/TenantDeck/internal/session"
)

// SessionKeySize is the required length, in bytes, of the session
// encryption key (AES-256-GCM). Reuses session.SessionKeySize as the one
// source of truth rather than duplicating the number.
const SessionKeySize = session.SessionKeySize

type Config struct {
	// ListenAddr is the address the BFF's HTTP server binds to.
	ListenAddr string

	// ExternalURL is the fixed, operator-configured base URL this
	// deployment is reachable at. Redirect URIs and cookie decisions are
	// derived from this, never from a request's Host header.
	ExternalURL string

	// Insecure relaxes cookie Secure/TLS requirements for loopback
	// development only. Never true outside explicit local dev.
	Insecure bool

	OIDC     OIDCConfig
	Session  SessionConfig
	Upstream UpstreamConfig
}

type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

type SessionConfig struct {
	RedisAddr     string
	RedisUsername string
	RedisPassword string
	RedisTLS      bool

	// RedisCAFile, if set, is a path to a PEM file of additional CA
	// certificates to trust for the Redis/Valkey TLS connection - for a
	// private CA, rather than requiring a publicly trusted one. Only
	// meaningful when RedisTLS is true. See config.SessionConfig.BuildRedisTLSConfig.
	RedisCAFile string

	// EncryptionKey is the AES-256-GCM key used to encrypt session
	// records. Supplied separately from the store itself (CLAUDE.md "Code
	// style" / docs/spec/04-auth-session-browser-security.md): compromising
	// Redis alone must not be enough to decrypt a session.
	EncryptionKey []byte
}

type UpstreamConfig struct {
	// CapsuleProxyURL is the fixed, trusted Capsule Proxy origin. Never
	// derived from a caller-supplied value.
	CapsuleProxyURL string
}

// Load reads configuration from the environment. It fails closed: any
// missing required value is an error, not a silently-applied default.
func Load() (*Config, error) {
	var errs []string
	req := func(name string) string {
		v := os.Getenv(name)
		if v == "" {
			errs = append(errs, name+" is required")
		}
		return v
	}

	cfg := &Config{
		ListenAddr:  getEnvDefault("TENANTDECK_LISTEN_ADDR", ":8080"),
		ExternalURL: req("TENANTDECK_EXTERNAL_URL"),
		OIDC: OIDCConfig{
			IssuerURL:    req("TENANTDECK_OIDC_ISSUER_URL"),
			ClientID:     req("TENANTDECK_OIDC_CLIENT_ID"),
			ClientSecret: req("TENANTDECK_OIDC_CLIENT_SECRET"),
			Scopes:       splitScopes(getEnvDefault("TENANTDECK_OIDC_SCOPES", "openid profile")),
		},
		Session: SessionConfig{
			RedisAddr:     req("TENANTDECK_REDIS_ADDR"),
			RedisUsername: os.Getenv("TENANTDECK_REDIS_USERNAME"),
			RedisPassword: os.Getenv("TENANTDECK_REDIS_PASSWORD"),
		},
		Upstream: UpstreamConfig{
			CapsuleProxyURL: req("TENANTDECK_CAPSULE_PROXY_URL"),
		},
	}

	if v := os.Getenv("TENANTDECK_INSECURE"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			errs = append(errs, "TENANTDECK_INSECURE must be a bool: "+err.Error())
		}
		cfg.Insecure = b
	}
	if v := os.Getenv("TENANTDECK_REDIS_TLS"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			errs = append(errs, "TENANTDECK_REDIS_TLS must be a bool: "+err.Error())
		}
		cfg.Session.RedisTLS = b
	}
	cfg.Session.RedisCAFile = os.Getenv("TENANTDECK_REDIS_CA_FILE")

	keyB64 := req("TENANTDECK_SESSION_ENCRYPTION_KEY")
	if keyB64 != "" {
		key, err := base64.StdEncoding.DecodeString(keyB64)
		if err != nil {
			errs = append(errs, "TENANTDECK_SESSION_ENCRYPTION_KEY must be base64: "+err.Error())
		} else if len(key) != SessionKeySize {
			errs = append(errs, fmt.Sprintf("TENANTDECK_SESSION_ENCRYPTION_KEY must decode to %d bytes, got %d", SessionKeySize, len(key)))
		} else {
			cfg.Session.EncryptionKey = key
		}
	}

	if !cfg.Insecure && strings.HasPrefix(cfg.ExternalURL, "http://") {
		errs = append(errs, "TENANTDECK_EXTERNAL_URL must be https:// unless TENANTDECK_INSECURE=true (loopback dev only)")
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("invalid configuration:\n  %s", strings.Join(errs, "\n  "))
	}
	return cfg, nil
}

func getEnvDefault(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func splitScopes(v string) []string {
	return strings.Fields(v)
}

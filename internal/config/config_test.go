package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

// validEnv returns a minimal set of env vars that Load should accept.
func validEnv() map[string]string {
	key := make([]byte, SessionKeySize)
	return map[string]string{
		"TENANTDECK_EXTERNAL_URL":           "https://tenantdeck.example.com",
		"TENANTDECK_OIDC_ISSUER_URL":        "https://idp.example.com",
		"TENANTDECK_OIDC_CLIENT_ID":         "tenantdeck",
		"TENANTDECK_OIDC_CLIENT_SECRET":     "secret",
		"TENANTDECK_REDIS_ADDR":             "localhost:6379",
		"TENANTDECK_CAPSULE_PROXY_URL":      "https://capsule-proxy.example.com",
		"TENANTDECK_SESSION_ENCRYPTION_KEY": base64.StdEncoding.EncodeToString(key),
	}
}

func withEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func TestLoad_SucceedsWithAllRequiredValues(t *testing.T) {
	withEnv(t, validEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.OIDC.ClientID != "tenantdeck" {
		t.Errorf("ClientID = %q, want %q", cfg.OIDC.ClientID, "tenantdeck")
	}
	if len(cfg.Session.EncryptionKey) != SessionKeySize {
		t.Errorf("EncryptionKey len = %d, want %d", len(cfg.Session.EncryptionKey), SessionKeySize)
	}
}

func TestLoad_FailsWhenRequiredValueMissing(t *testing.T) {
	env := validEnv()
	delete(env, "TENANTDECK_OIDC_CLIENT_ID")
	withEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded, want error for missing TENANTDECK_OIDC_CLIENT_ID")
	}
	if !strings.Contains(err.Error(), "TENANTDECK_OIDC_CLIENT_ID") {
		t.Errorf("error %q does not name the missing variable", err)
	}
}

func TestLoad_RejectsSessionKeyOfWrongLength(t *testing.T) {
	env := validEnv()
	env["TENANTDECK_SESSION_ENCRYPTION_KEY"] = base64.StdEncoding.EncodeToString([]byte("too-short"))
	withEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded, want error for wrong-length session key")
	}
}

func TestLoad_RejectsNonHTTPSExternalURLWithoutInsecureFlag(t *testing.T) {
	env := validEnv()
	env["TENANTDECK_EXTERNAL_URL"] = "http://tenantdeck.example.com"
	withEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded, want error: http:// external URL requires TENANTDECK_INSECURE=true")
	}
}

func TestLoad_RejectsInvalidInsecureFlag(t *testing.T) {
	env := validEnv()
	env["TENANTDECK_INSECURE"] = "not-a-bool"
	withEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded with an invalid TENANTDECK_INSECURE value, want error")
	}
}

func TestLoad_RejectsInvalidRedisTLSFlag(t *testing.T) {
	env := validEnv()
	env["TENANTDECK_REDIS_TLS"] = "not-a-bool"
	withEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded with an invalid TENANTDECK_REDIS_TLS value, want error")
	}
}

func TestLoad_AcceptsValidRedisTLSFlag(t *testing.T) {
	env := validEnv()
	env["TENANTDECK_REDIS_TLS"] = "true"
	withEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if !cfg.Session.RedisTLS {
		t.Error("cfg.Session.RedisTLS = false, want true")
	}
}

func TestLoad_UsesExplicitListenAddrAndScopesWhenSet(t *testing.T) {
	env := validEnv()
	env["TENANTDECK_LISTEN_ADDR"] = ":9999"
	env["TENANTDECK_OIDC_SCOPES"] = "openid email"
	withEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.ListenAddr != ":9999" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9999")
	}
	if len(cfg.OIDC.Scopes) != 2 || cfg.OIDC.Scopes[0] != "openid" || cfg.OIDC.Scopes[1] != "email" {
		t.Errorf("Scopes = %v, want [openid email]", cfg.OIDC.Scopes)
	}
}

func TestLoad_AllowsHTTPExternalURLWhenInsecureFlagSet(t *testing.T) {
	env := validEnv()
	env["TENANTDECK_EXTERNAL_URL"] = "http://127.0.0.1:8080"
	env["TENANTDECK_INSECURE"] = "true"
	withEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if !cfg.Insecure {
		t.Error("cfg.Insecure = false, want true")
	}
}

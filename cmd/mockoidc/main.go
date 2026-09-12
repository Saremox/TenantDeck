// Command mockoidc is a deterministic, test-only OIDC provider for the
// mandatory E2E stack (docs/spec/07-mandatory-automated-testing.md item 3):
// real discovery, JWKS, authorization-code flow with enforced PKCE, and
// refresh, all with real RS256-signed JWTs. It is never imported by
// cmd/tenantdeck and never ships as part of the production image - see
// docs/implementation-plan.md Phase 4 and Dockerfile, neither of which
// reference this package.
package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("mockoidc: %v", err)
	}

	srv, err := newServer(cfg)
	if err != nil {
		log.Fatalf("mockoidc: %v", err)
	}

	log.Printf("mockoidc listening addr=%s issuer=%s", cfg.ListenAddr, cfg.IssuerURL)
	if err := http.ListenAndServe(cfg.ListenAddr, srv); err != nil {
		log.Fatalf("mockoidc: %v", err)
	}
}

type config struct {
	ListenAddr   string
	IssuerURL    string
	ClientID     string
	ClientSecret string
	IDTokenTTL   time.Duration
}

func loadConfig() (config, error) {
	cfg := config{
		ListenAddr:   getEnvDefault("MOCKOIDC_LISTEN_ADDR", ":9999"),
		IssuerURL:    os.Getenv("MOCKOIDC_ISSUER_URL"),
		ClientID:     os.Getenv("MOCKOIDC_CLIENT_ID"),
		ClientSecret: os.Getenv("MOCKOIDC_CLIENT_SECRET"),
		IDTokenTTL:   5 * time.Minute,
	}
	if cfg.IssuerURL == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return config{}, errors.New("MOCKOIDC_ISSUER_URL, MOCKOIDC_CLIENT_ID and MOCKOIDC_CLIENT_SECRET are all required")
	}
	return cfg, nil
}

func getEnvDefault(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

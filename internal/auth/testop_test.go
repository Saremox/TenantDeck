package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// testOPClaims controls exactly what ID token the fake token endpoint
// issues for every request it receives. This is a minimal, test-only OIDC
// provider fixture used to exercise internal/auth's own callback logic
// (nonce binding, PKCE verifier threading, expiry handling) against a real
// signature/discovery/token-exchange pipeline - it is NOT the mandatory
// Phase 4 E2E mock OIDC provider (docs/spec/07-mandatory-automated-testing.md),
// which needs a real authorization-code round trip and is a separate,
// not-yet-built, decision. This fixture never ships outside _test.go files.
type testOPClaims struct {
	Subject   string
	Audience  string
	Nonce     string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// testOP is a minimal httptest-backed OIDC provider: discovery, JWKS, and a
// token endpoint that always returns an ID token built from whatever
// testOPClaims it was constructed with.
type testOP struct {
	srv    *httptest.Server
	claims testOPClaims
}

const testOPKeyID = "test-key"

func newTestOP(t *testing.T, claims testOPClaims) *testOP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating test RSA key: %v", err)
	}

	op := &testOP{claims: claims}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", op.handleDiscovery)
	mux.HandleFunc("/jwks", op.handleJWKS(key.Public()))
	mux.HandleFunc("/token", op.handleToken(key))
	op.srv = httptest.NewServer(mux)
	t.Cleanup(op.srv.Close)
	return op
}

func (op *testOP) issuer() string { return op.srv.URL }

func (op *testOP) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	doc := map[string]any{
		"issuer":                                op.issuer(),
		"authorization_endpoint":                op.issuer() + "/authorize",
		"token_endpoint":                        op.issuer() + "/token",
		"jwks_uri":                              op.issuer() + "/jwks",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":                      []string{"openid", "profile"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
		"code_challenge_methods_supported":      []string{"S256"},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(doc)
}

func (op *testOP) handleJWKS(pub any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		set := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
			Key:       pub,
			KeyID:     testOPKeyID,
			Algorithm: "RS256",
			Use:       "sig",
		}}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(set)
	}
}

func (op *testOP) handleToken(key *rsa.PrivateKey) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idToken, err := op.signIDToken(key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp := map[string]any{
			"access_token": "test-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     idToken,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func (op *testOP) signIDToken(key *rsa.PrivateKey) (string, error) {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, &jose.SignerOptions{
		ExtraHeaders: map[jose.HeaderKey]any{"kid": testOPKeyID},
	})
	if err != nil {
		return "", err
	}
	c := op.claims
	claims := map[string]any{
		"iss":   op.issuer(),
		"sub":   c.Subject,
		"aud":   c.Audience,
		"exp":   c.ExpiresAt.Unix(),
		"iat":   c.IssuedAt.Unix(),
		"nonce": c.Nonce,
	}
	return jwt.Signed(signer).Claims(claims).Serialize()
}

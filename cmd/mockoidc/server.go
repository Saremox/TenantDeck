package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

const signingKeyID = "mockoidc-key"

// authCode is what /authorize stores and /token consumes exactly once -
// deleted on first exchange, so a replayed code always fails (the E2E
// suite asserts this per docs/spec/07-mandatory-automated-testing.md
// "Invalid ... replayed authorization codes fail").
type authCode struct {
	subject       string
	redirectURI   string
	nonce         string
	codeChallenge string
	expiresAt     time.Time
}

type refreshToken struct {
	subject   string
	expiresAt time.Time
}

// server implements a deterministic authorization-code + PKCE + refresh
// OIDC provider. State is in-memory and unauthenticated beyond the
// client_id/client_secret check - acceptable only because this exists
// solely for the disposable E2E stack, never for production.
type server struct {
	cfg     config
	signKey *rsa.PrivateKey

	mu     sync.Mutex
	codes  map[string]authCode
	tokens map[string]refreshToken

	mux *http.ServeMux
}

func newServer(cfg config) (*server, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	s := &server{
		cfg:     cfg,
		signKey: key,
		codes:   map[string]authCode{},
		tokens:  map[string]refreshToken{},
	}
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("GET /.well-known/openid-configuration", s.handleDiscovery)
	s.mux.HandleFunc("GET /jwks", s.handleJWKS)
	s.mux.HandleFunc("GET /authorize", s.handleAuthorize)
	s.mux.HandleFunc("POST /token", s.handleToken)
	return s, nil
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *server) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	doc := map[string]any{
		"issuer":                                s.cfg.IssuerURL,
		"authorization_endpoint":                s.cfg.IssuerURL + "/authorize",
		"token_endpoint":                        s.cfg.IssuerURL + "/token",
		"jwks_uri":                              s.cfg.IssuerURL + "/jwks",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":                      []string{"openid", "profile"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
		"code_challenge_methods_supported":      []string{"S256"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
	}
	writeJSON(w, http.StatusOK, doc)
}

func (s *server) handleJWKS(w http.ResponseWriter, r *http.Request) {
	set := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       s.signKey.Public(),
		KeyID:     signingKeyID,
		Algorithm: "RS256",
		Use:       "sig",
	}}}
	writeJSON(w, http.StatusOK, set)
}

// handleAuthorize is the deterministic stand-in for a login page: it
// never prompts for credentials, it picks the subject from login_hint
// (falling back to a fixed default) so an E2E test can choose which
// fixture user/tenant it's acting as just by varying the authorize URL -
// see docs/spec/07-mandatory-automated-testing.md item 6 ("a user in
// each tenant"). PKCE is mandatory, not optional: a request with no
// code_challenge/code_challenge_method=S256 is rejected outright, which
// is what lets the E2E suite prove TenantDeck's own RP-side PKCE
// generation is what makes login work at all, not an IdP that accepts it
// either way.
func (s *server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("response_type") != "code" {
		http.Error(w, "unsupported_response_type", http.StatusBadRequest)
		return
	}
	if q.Get("client_id") != s.cfg.ClientID {
		http.Error(w, "unauthorized_client", http.StatusBadRequest)
		return
	}
	redirectURI := q.Get("redirect_uri")
	if redirectURI == "" {
		http.Error(w, "invalid_request: redirect_uri required", http.StatusBadRequest)
		return
	}
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		http.Error(w, "invalid_request: PKCE (S256) is required", http.StatusBadRequest)
		return
	}

	subject := q.Get("login_hint")
	if subject == "" {
		subject = "test-user@example.com"
	}

	code := randomToken()
	s.mu.Lock()
	s.codes[code] = authCode{
		subject:       subject,
		redirectURI:   redirectURI,
		nonce:         q.Get("nonce"),
		codeChallenge: q.Get("code_challenge"),
		expiresAt:     time.Now().Add(time.Minute),
	}
	s.mu.Unlock()

	dest, err := url.Parse(redirectURI)
	if err != nil {
		http.Error(w, "invalid_request: malformed redirect_uri", http.StatusBadRequest)
		return
	}
	destQuery := dest.Query()
	destQuery.Set("code", code)
	if state := q.Get("state"); state != "" {
		destQuery.Set("state", state)
	}
	dest.RawQuery = destQuery.Encode()
	http.Redirect(w, r, dest.String(), http.StatusFound)
}

func (s *server) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeTokenError(w, "invalid_request")
		return
	}
	if !s.authenticateClient(r) {
		writeTokenError(w, "invalid_client")
		return
	}

	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		s.handleAuthorizationCodeGrant(w, r)
	case "refresh_token":
		s.handleRefreshTokenGrant(w, r)
	default:
		writeTokenError(w, "unsupported_grant_type")
	}
}

// authenticateClient accepts client_secret_basic or client_secret_post -
// both are real OIDC Core-defined methods, and the RP library TenantDeck
// uses picks one; the fixture shouldn't assume which.
func (s *server) authenticateClient(r *http.Request) bool {
	if id, secret, ok := r.BasicAuth(); ok {
		return id == s.cfg.ClientID && secret == s.cfg.ClientSecret
	}
	return r.PostForm.Get("client_id") == s.cfg.ClientID && r.PostForm.Get("client_secret") == s.cfg.ClientSecret
}

func (s *server) handleAuthorizationCodeGrant(w http.ResponseWriter, r *http.Request) {
	code := r.PostForm.Get("code")

	s.mu.Lock()
	ac, ok := s.codes[code]
	if ok {
		delete(s.codes, code) // single-use: a replay of this code is always rejected from here on.
	}
	s.mu.Unlock()

	if !ok || time.Now().After(ac.expiresAt) {
		writeTokenError(w, "invalid_grant")
		return
	}
	if r.PostForm.Get("redirect_uri") != ac.redirectURI {
		writeTokenError(w, "invalid_grant")
		return
	}
	if !verifyPKCE(ac.codeChallenge, r.PostForm.Get("code_verifier")) {
		writeTokenError(w, "invalid_grant")
		return
	}

	s.issueTokens(w, ac.subject, ac.nonce)
}

func (s *server) handleRefreshTokenGrant(w http.ResponseWriter, r *http.Request) {
	token := r.PostForm.Get("refresh_token")

	s.mu.Lock()
	rt, ok := s.tokens[token]
	s.mu.Unlock()

	if !ok || time.Now().After(rt.expiresAt) {
		writeTokenError(w, "invalid_grant")
		return
	}
	// A refreshed ID token carries no nonce - OIDC Core ties nonce to the
	// original authentication request, not to a refresh.
	s.issueTokens(w, rt.subject, "")
}

func (s *server) issueTokens(w http.ResponseWriter, subject, nonce string) {
	idToken, err := s.signIDToken(subject, nonce)
	if err != nil {
		http.Error(w, "server_error", http.StatusInternalServerError)
		return
	}

	refresh := randomToken()
	s.mu.Lock()
	s.tokens[refresh] = refreshToken{subject: subject, expiresAt: time.Now().Add(time.Hour)}
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  randomToken(),
		"token_type":    "Bearer",
		"expires_in":    int(s.cfg.IDTokenTTL.Seconds()),
		"id_token":      idToken,
		"refresh_token": refresh,
	})
}

func (s *server) signIDToken(subject, nonce string) (string, error) {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: s.signKey}, &jose.SignerOptions{
		ExtraHeaders: map[jose.HeaderKey]any{"kid": signingKeyID},
	})
	if err != nil {
		return "", err
	}
	now := time.Now()
	claims := map[string]any{
		"iss": s.cfg.IssuerURL,
		"sub": subject,
		"aud": s.cfg.ClientID,
		"iat": now.Unix(),
		"exp": now.Add(s.cfg.IDTokenTTL).Unix(),
	}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	return jwt.Signed(signer).Claims(claims).Serialize()
}

// verifyPKCE checks the RFC 7636 S256 transform: the verifier's SHA-256,
// base64url-encoded (no padding), must equal the challenge /authorize
// stored. This is the actual PKCE enforcement
// docs/spec/07-mandatory-automated-testing.md item 3 asks for - not just
// accepting whatever TenantDeck sends.
func verifyPKCE(challenge, verifier string) bool {
	if verifier == "" {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return computed == challenge
}

func randomToken() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return base64.RawURLEncoding.EncodeToString(buf)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeTokenError follows RFC 6749 section 5.2's error body shape, which
// the RP library TenantDeck uses (zitadel/oidc) parses for its own error
// handling/tests.
func writeTokenError(w http.ResponseWriter, code string) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": code})
}

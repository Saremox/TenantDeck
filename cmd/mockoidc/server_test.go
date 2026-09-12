package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

func testConfig() config {
	return config{
		IssuerURL:    "https://mockoidc.test",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		IDTokenTTL:   5 * time.Minute,
	}
}

func newTestServer(t *testing.T) *server {
	t.Helper()
	s, err := newServer(testConfig())
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	return s
}

// pkcePair returns a verifier and its S256 challenge, exactly as a real
// RP would generate them per RFC 7636.
func pkcePair() (verifier, challenge string) {
	verifier = "a-fixed-test-verifier-that-is-at-least-43-characters-long"
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge
}

func authorize(t *testing.T, s *server, extraQuery url.Values) *http.Response {
	t.Helper()
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {"test-client"},
		"redirect_uri":          {"https://rp.test/callback"},
		"code_challenge_method": {"S256"},
	}
	for k, v := range extraQuery {
		q[k] = v
	}
	req := httptest.NewRequest(http.MethodGet, "/authorize?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec.Result()
}

func codeFromRedirect(t *testing.T, resp *http.Response) string {
	t.Helper()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("/authorize status = %d, want %d", resp.StatusCode, http.StatusFound)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parsing Location header: %v", err)
	}
	code := loc.Query().Get("code")
	if code == "" {
		t.Fatal("Location has no code query param")
	}
	return code
}

func exchangeCode(s *server, code, verifier string) *http.Response {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {"https://rp.test/callback"},
		"code_verifier": {verifier},
	}
	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "test-secret")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec.Result()
}

func TestDiscovery_AdvertisesRequiredFields(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`"issuer":"https://mockoidc.test"`, `"code_challenge_methods_supported":["S256"]`} {
		if !strings.Contains(body, want) {
			t.Errorf("discovery document missing %q: %s", want, body)
		}
	}
}

func TestAuthorize_RejectsMissingPKCEChallenge(t *testing.T) {
	s := newTestServer(t)
	resp := authorize(t, s, url.Values{"code_challenge_method": {""}})

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (PKCE must be mandatory)", resp.StatusCode)
	}
}

func TestAuthorize_RejectsUnknownClient(t *testing.T) {
	s := newTestServer(t)
	resp := authorize(t, s, url.Values{"client_id": {"someone-else"}, "code_challenge": {"x"}})

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestFullAuthorizationCodeFlow_IssuesAValidSignedIDToken(t *testing.T) {
	s := newTestServer(t)
	verifier, challenge := pkcePair()

	resp := authorize(t, s, url.Values{"code_challenge": {challenge}, "nonce": {"test-nonce"}, "login_hint": {"alice@tenant-a.example.com"}})
	code := codeFromRedirect(t, resp)

	tokenResp := exchangeCode(s, code, verifier)
	if tokenResp.StatusCode != http.StatusOK {
		t.Fatalf("/token status = %d, want 200", tokenResp.StatusCode)
	}

	var body struct {
		IDToken      string `json:"id_token"`
		RefreshToken string `json:"refresh_token"`
	}
	decodeJSON(t, tokenResp, &body)
	if body.IDToken == "" || body.RefreshToken == "" {
		t.Fatal("expected a non-empty id_token and refresh_token")
	}

	claims := parseIDTokenClaims(t, s, body.IDToken)
	if claims["sub"] != "alice@tenant-a.example.com" {
		t.Errorf("sub = %v, want alice@tenant-a.example.com", claims["sub"])
	}
	if claims["nonce"] != "test-nonce" {
		t.Errorf("nonce = %v, want test-nonce", claims["nonce"])
	}
	if claims["aud"] != "test-client" {
		t.Errorf("aud = %v, want test-client", claims["aud"])
	}
}

func TestTokenExchange_RejectsWrongPKCEVerifier(t *testing.T) {
	s := newTestServer(t)
	_, challenge := pkcePair()
	resp := authorize(t, s, url.Values{"code_challenge": {challenge}})
	code := codeFromRedirect(t, resp)

	tokenResp := exchangeCode(s, code, "the-wrong-verifier-entirely-not-matching-challenge")
	if tokenResp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (PKCE mismatch must be rejected)", tokenResp.StatusCode)
	}
}

func TestTokenExchange_RejectsAReplayedCode(t *testing.T) {
	s := newTestServer(t)
	verifier, challenge := pkcePair()
	resp := authorize(t, s, url.Values{"code_challenge": {challenge}})
	code := codeFromRedirect(t, resp)

	first := exchangeCode(s, code, verifier)
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first exchange status = %d, want 200", first.StatusCode)
	}

	second := exchangeCode(s, code, verifier)
	if second.StatusCode != http.StatusBadRequest {
		t.Errorf("replayed exchange status = %d, want 400", second.StatusCode)
	}
}

func TestTokenExchange_RejectsWrongClientSecret(t *testing.T) {
	s := newTestServer(t)
	verifier, challenge := pkcePair()
	resp := authorize(t, s, url.Values{"code_challenge": {challenge}})
	code := codeFromRedirect(t, resp)

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {"https://rp.test/callback"},
		"code_verifier": {verifier},
	}
	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "the-wrong-secret")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestRefreshTokenGrant_IssuesANewIDTokenForTheSameSubject(t *testing.T) {
	s := newTestServer(t)
	verifier, challenge := pkcePair()
	resp := authorize(t, s, url.Values{"code_challenge": {challenge}, "login_hint": {"bob@tenant-b.example.com"}})
	code := codeFromRedirect(t, resp)

	var first struct {
		RefreshToken string `json:"refresh_token"`
	}
	decodeJSON(t, exchangeCode(s, code, verifier), &first)

	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {first.RefreshToken}}
	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "test-secret")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, want 200", rec.Code)
	}
	var second struct {
		IDToken string `json:"id_token"`
	}
	decodeJSON(t, rec.Result(), &second)
	claims := parseIDTokenClaims(t, s, second.IDToken)
	if claims["sub"] != "bob@tenant-b.example.com" {
		t.Errorf("sub = %v, want bob@tenant-b.example.com", claims["sub"])
	}
}

func TestRefreshTokenGrant_RejectsUnknownRefreshToken(t *testing.T) {
	s := newTestServer(t)
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"not-a-real-token"}}
	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "test-secret")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decoding JSON response: %v", err)
	}
}

// parseIDTokenClaims verifies the token's signature against the server's
// own JWKS (not just base64-decoding it unchecked) before returning its
// claims, so these tests would fail if signIDToken ever produced a token
// that doesn't verify against its own published key.
func parseIDTokenClaims(t *testing.T, s *server, rawIDToken string) map[string]any {
	t.Helper()
	tok, err := jwt.ParseSigned(rawIDToken, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		t.Fatalf("parsing ID token: %v", err)
	}
	var claims map[string]any
	if err := tok.Claims(s.signKey.Public(), &claims); err != nil {
		t.Fatalf("verifying ID token signature: %v", err)
	}
	return claims
}

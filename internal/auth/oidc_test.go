package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Saremox/TenantDeck/internal/config"
	"github.com/Saremox/TenantDeck/internal/session"
)

const testClientID = "tenantdeck-test-client"

func newTestHandler(t *testing.T, op *testOP, insecure bool, store *session.Store) *Handler {
	t.Helper()
	cfg := &config.Config{
		ExternalURL: "https://tenantdeck.example.com",
		Insecure:    insecure,
		OIDC: config.OIDCConfig{
			IssuerURL:    op.issuer(),
			ClientID:     testClientID,
			ClientSecret: "test-client-secret",
			Scopes:       []string{"openid"},
		},
	}
	h, err := NewHandler(context.Background(), cfg, store)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	return h
}

func defaultClaims(nonce string) testOPClaims {
	return testOPClaims{
		Subject:   "alice",
		Audience:  testClientID,
		Nonce:     nonce,
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
}

func TestLoginHandler_RedirectsToAuthorizationEndpointWithPKCEAndNonce(t *testing.T) {
	op := newTestOP(t, defaultClaims(""))
	h := newTestHandler(t, op, true, newTestStore(t))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	h.LoginHandler(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parsing Location header: %v", err)
	}
	q := loc.Query()
	if q.Get("client_id") != testClientID {
		t.Errorf("client_id = %q, want %q", q.Get("client_id"), testClientID)
	}
	if q.Get("response_type") != "code" {
		t.Errorf("response_type = %q, want %q", q.Get("response_type"), "code")
	}
	if q.Get("code_challenge") == "" {
		t.Error("code_challenge is empty, want a PKCE challenge")
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Errorf("code_challenge_method = %q, want %q", q.Get("code_challenge_method"), "S256")
	}
	if q.Get("nonce") == "" {
		t.Error("nonce is empty, want a generated nonce")
	}
	if q.Get("state") == "" {
		t.Error("state is empty, want a generated state")
	}
}

func TestLoginHandler_SetsHostPrefixedSecureCookieWhenNotInsecure(t *testing.T) {
	op := newTestOP(t, defaultClaims(""))
	h := newTestHandler(t, op, false, newTestStore(t))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	h.LoginHandler(rec, req)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	c := cookies[0]
	if c.Name != "__Host-td-login-state" {
		t.Errorf("cookie name = %q, want %q", c.Name, "__Host-td-login-state")
	}
	if !c.Secure || !c.HttpOnly {
		t.Errorf("cookie Secure=%v HttpOnly=%v, want both true", c.Secure, c.HttpOnly)
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie SameSite = %v, want Lax (must survive the IdP's top-level GET redirect back to /auth/callback)", c.SameSite)
	}
	if c.Path != "/" {
		t.Errorf("cookie Path = %q, want %q", c.Path, "/")
	}
}

// login performs LoginHandler and returns the state value, the generated
// nonce (parsed back out of the redirect URL - a real browser never reads
// this, but the test needs it to configure the fake IdP's response), and
// the login-state cookie to attach to the subsequent callback request.
func login(t *testing.T, h *Handler) (state, nonce string, cookie *http.Cookie) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	h.LoginHandler(rec, req)

	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parsing Location header: %v", err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies from login, want 1", len(cookies))
	}
	return loc.Query().Get("state"), loc.Query().Get("nonce"), cookies[0]
}

func findCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no %q cookie among %+v", name, cookies)
	return nil
}

func callbackRequest(state, code string, cookie *http.Cookie) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state+"&code="+code, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req
}

func TestCallbackHandler_CreatesSessionOnValidCallback(t *testing.T) {
	op := newTestOP(t, defaultClaims(""))
	store := newTestStore(t)
	h := newTestHandler(t, op, true, store)

	state, nonce, cookie := login(t, h)
	op.claims.Nonce = nonce // the fake IdP "remembers" the nonce the real authorize request carried

	rec := httptest.NewRecorder()
	h.CallbackHandler(rec, callbackRequest(state, "test-code", cookie))

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, body = %q, want %d", rec.Code, rec.Body.String(), http.StatusFound)
	}
	if got := rec.Header().Get("Location"); got != "/" {
		t.Errorf("Location = %q, want %q", got, "/")
	}
	sessionCookie := findCookie(t, rec.Result().Cookies(), "td-session")

	rec2, err := store.GetSession(context.Background(), sessionCookie.Value)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if rec2.Subject != "alice" {
		t.Errorf("session Subject = %q, want %q", rec2.Subject, "alice")
	}
}

func TestCallbackHandler_RejectsNonceMismatch(t *testing.T) {
	op := newTestOP(t, defaultClaims(""))
	store := newTestStore(t)
	h := newTestHandler(t, op, true, store)

	state, _, cookie := login(t, h)
	op.claims.Nonce = "a-completely-different-nonce"

	rec := httptest.NewRecorder()
	h.CallbackHandler(rec, callbackRequest(state, "test-code", cookie))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCallbackHandler_RejectsMissingStateCookie(t *testing.T) {
	op := newTestOP(t, defaultClaims(""))
	h := newTestHandler(t, op, true, newTestStore(t))

	rec := httptest.NewRecorder()
	h.CallbackHandler(rec, callbackRequest("some-state", "test-code", nil))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCallbackHandler_RejectsStateQueryMismatchingCookie(t *testing.T) {
	op := newTestOP(t, defaultClaims(""))
	h := newTestHandler(t, op, true, newTestStore(t))

	cookie := &http.Cookie{Name: LoginStateCookieName(true), Value: "cookie-state-value"}
	rec := httptest.NewRecorder()
	h.CallbackHandler(rec, callbackRequest("different-state-value", "test-code", cookie))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCallbackHandler_RejectsReplayedState(t *testing.T) {
	op := newTestOP(t, defaultClaims(""))
	store := newTestStore(t)
	h := newTestHandler(t, op, true, store)

	state, nonce, cookie := login(t, h)
	op.claims.Nonce = nonce

	first := httptest.NewRecorder()
	h.CallbackHandler(first, callbackRequest(state, "test-code", cookie))
	if first.Code != http.StatusFound {
		t.Fatalf("first callback status = %d, want %d (setup failed)", first.Code, http.StatusFound)
	}

	second := httptest.NewRecorder()
	h.CallbackHandler(second, callbackRequest(state, "test-code", cookie))
	if second.Code != http.StatusBadRequest {
		t.Errorf("replayed callback status = %d, want %d", second.Code, http.StatusBadRequest)
	}
}

func TestCallbackHandler_RejectsIdPErrorParameter(t *testing.T) {
	op := newTestOP(t, defaultClaims(""))
	h := newTestHandler(t, op, true, newTestStore(t))

	state, _, cookie := login(t, h)
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state+"&error=access_denied", nil)
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	h.CallbackHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCallbackHandler_RejectsMissingCode(t *testing.T) {
	op := newTestOP(t, defaultClaims(""))
	h := newTestHandler(t, op, true, newTestStore(t))

	state, _, cookie := login(t, h)
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state, nil)
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	h.CallbackHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestNewHandler_ReturnsErrorWhenIssuerDiscoveryFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r) // no /.well-known/openid-configuration here
	}))
	defer srv.Close()

	cfg := &config.Config{
		ExternalURL: "https://tenantdeck.example.com",
		Insecure:    true,
		OIDC: config.OIDCConfig{
			IssuerURL: srv.URL,
			ClientID:  testClientID,
		},
	}
	if _, err := NewHandler(context.Background(), cfg, newTestStore(t)); err == nil {
		t.Fatal("NewHandler() succeeded against an issuer with no discovery document, want error")
	}
}

func TestNewHandler_ReturnsErrorWhenExternalURLIsInvalid(t *testing.T) {
	op := newTestOP(t, defaultClaims(""))
	cfg := &config.Config{
		ExternalURL: "http://\x7f", // control character: net/url rejects this
		Insecure:    true,
		OIDC: config.OIDCConfig{
			IssuerURL: op.issuer(),
			ClientID:  testClientID,
		},
	}
	if _, err := NewHandler(context.Background(), cfg, newTestStore(t)); err == nil {
		t.Fatal("NewHandler() succeeded with an invalid ExternalURL, want error")
	}
}

func TestCallbackHandler_FailsClosedWhenCSRFTokenGenerationFails(t *testing.T) {
	op := newTestOP(t, defaultClaims(""))
	h := newTestHandler(t, op, true, newTestStore(t))

	state, nonce, cookie := login(t, h)
	op.claims.Nonce = nonce

	orig := randReader
	randReader = failingReader{}
	defer func() { randReader = orig }()

	rec := httptest.NewRecorder()
	h.CallbackHandler(rec, callbackRequest(state, "test-code", cookie))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, body = %q, want %d", rec.Code, rec.Body.String(), http.StatusInternalServerError)
	}
}

func TestLoginHandler_FailsClosedWhenRandReaderFails(t *testing.T) {
	op := newTestOP(t, defaultClaims(""))
	h := newTestHandler(t, op, true, newTestStore(t))

	orig := randReader
	randReader = failingReader{}
	defer func() { randReader = orig }()

	rec := httptest.NewRecorder()
	h.LoginHandler(rec, httptest.NewRequest(http.MethodGet, "/auth/login", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestLoginHandler_FailsClosedWhenStoreUnavailable(t *testing.T) {
	op := newTestOP(t, defaultClaims(""))
	// A store pointed at a port nothing listens on, simulating Redis being
	// down - login must fail, not proceed without a stored transaction.
	backend := session.NewRedisBackend(session.RedisOptions{Addr: "127.0.0.1:1"})
	store, err := session.NewStore(backend, make([]byte, session.SessionKeySize))
	if err != nil {
		t.Fatalf("session.NewStore: %v", err)
	}
	h := newTestHandler(t, op, true, store)

	rec := httptest.NewRecorder()
	h.LoginHandler(rec, httptest.NewRequest(http.MethodGet, "/auth/login", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestClearSessionCookie_ExpiresTheSessionCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	ClearSessionCookie(rec, true)

	cookies := rec.Result().Cookies()
	c := findCookie(t, cookies, SessionCookieName(true))
	if c.MaxAge >= 0 {
		t.Errorf("MaxAge = %d, want a negative value (delete immediately)", c.MaxAge)
	}
}

func TestCallbackHandler_RejectsAlreadyExpiredToken(t *testing.T) {
	claims := defaultClaims("")
	claims.ExpiresAt = time.Now().Add(-time.Hour)
	op := newTestOP(t, claims)
	h := newTestHandler(t, op, true, newTestStore(t))

	state, nonce, cookie := login(t, h)
	op.claims.Nonce = nonce

	rec := httptest.NewRecorder()
	h.CallbackHandler(rec, callbackRequest(state, "test-code", cookie))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

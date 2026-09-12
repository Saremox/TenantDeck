// Package auth implements the BFF's own OIDC Authorization Code + PKCE
// flow: /auth/login starts a login transaction, /auth/callback redeems it
// and creates a server-side session. The browser never sees an ID, access,
// or refresh token — see docs/spec/03-architecture.md.
package auth

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"golang.org/x/oauth2"

	"github.com/Saremox/TenantDeck/internal/config"
	"github.com/Saremox/TenantDeck/internal/session"
)

type contextKey string

// expectedNonceContextKey carries the current callback's expected nonce
// into the RelyingParty's verifier (wired via rp.WithNonce below), so the
// library's own OIDC Core nonce check - not a parallel hand-rolled one -
// is what actually enforces it. See docs/spec/04-auth-session-browser-security.md.
const expectedNonceContextKey contextKey = "tenantdeck-expected-nonce"

func contextWithExpectedNonce(ctx context.Context, nonce string) context.Context {
	return context.WithValue(ctx, expectedNonceContextKey, nonce)
}

func expectedNonceFromContext(ctx context.Context) string {
	v, _ := ctx.Value(expectedNonceContextKey).(string)
	return v
}

// loginTransactionTTL bounds how long a started-but-not-completed login may
// be redeemed for, per docs/spec/04-auth-session-browser-security.md
// ("bounded login-transaction lifetime").
const loginTransactionTTL = 5 * time.Minute

// postLoginRedirect is fixed and relative, never derived from a request
// parameter — trivially satisfies the "only relative, allowlisted post-login
// destinations" requirement because there is only ever this one destination.
const postLoginRedirect = "/"

// Handler implements the login/callback/session/logout HTTP endpoints.
type Handler struct {
	party    rp.RelyingParty
	store    *session.Store
	insecure bool

	// externalOrigin is scheme://host derived from cfg.ExternalURL, used to
	// validate the Origin header on state-changing requests (logout). Fixed
	// at startup, never derived from an incoming request.
	externalOrigin string
}

// NewHandler performs OIDC discovery against cfg.OIDC.IssuerURL (failing if
// it can't) and returns a ready-to-use Handler.
func NewHandler(ctx context.Context, cfg *config.Config, store *session.Store) (*Handler, error) {
	redirectURL := strings.TrimRight(cfg.ExternalURL, "/") + "/auth/callback"

	externalOrigin, err := originOf(cfg.ExternalURL)
	if err != nil {
		return nil, err
	}

	party, err := rp.NewRelyingPartyOIDC(
		ctx,
		cfg.OIDC.IssuerURL,
		cfg.OIDC.ClientID,
		cfg.OIDC.ClientSecret,
		redirectURL,
		cfg.OIDC.Scopes,
		rp.WithVerifierOpts(
			rp.WithIssuedAtOffset(5*time.Second),
			rp.WithNonce(expectedNonceFromContext),
		),
		// Accept only the signing algorithms the IdP itself advertises via
		// discovery, rather than a hardcoded list - docs/spec/04-auth-session-browser-security.md
		// ("allowed algorithms").
		rp.WithSigningAlgsFromDiscovery(),
	)
	if err != nil {
		return nil, err
	}
	return &Handler{party: party, store: store, insecure: cfg.Insecure, externalOrigin: externalOrigin}, nil
}

func originOf(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return u.Scheme + "://" + u.Host, nil
}

// LoginHandler starts a new login transaction and redirects the browser to
// the IdP. GET only; it has no side effect a CSRF'd cross-site GET could
// meaningfully abuse (it only creates a transaction the attacker already
// knows the state of), so no CSRF token is needed here — unlike logout.
func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	nonce, err := randomToken()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	verifier, err := randomToken()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	challenge := oidc.NewSHACodeChallenge(verifier)

	state, err := h.store.CreateLoginTransaction(r.Context(), session.LoginTransaction{
		Nonce:        nonce,
		PKCEVerifier: verifier,
		CreatedAt:    time.Now(),
	}, loginTransactionTTL)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// This cookie is what binds the callback to this browser and this
	// specific login transaction, not just "some valid state exists" -
	// see docs/spec/04-auth-session-browser-security.md on binding the
	// callback to its login transaction rather than a blanket same-origin
	// rule.
	setLoginStateCookie(w, h.insecure, state, loginTransactionTTL)

	authURL := rp.AuthURL(state, h.party, rp.WithCodeChallenge(challenge), withNonceParam(nonce))
	http.Redirect(w, r, authURL, http.StatusFound)
}

// CallbackHandler redeems a login transaction: validates the state cookie,
// exchanges the code for tokens, verifies the nonce, and creates a session.
func (h *Handler) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(LoginStateCookieName(h.insecure))
	if err != nil {
		http.Error(w, "missing or expired login", http.StatusBadRequest)
		return
	}
	// Clear it now, regardless of outcome below: this cookie is single-use
	// by design, same as the transaction it names.
	clearLoginStateCookie(w, h.insecure)

	providedState := r.URL.Query().Get("state")
	if providedState == "" || !constantTimeEqual(providedState, cookie.Value) {
		http.Error(w, "login state mismatch", http.StatusBadRequest)
		return
	}

	// TakeLoginTransaction deletes on read, so a replayed callback with the
	// same state (e.g. the browser back button, or an attacker who
	// observed it) fails here on its second attempt.
	txn, err := h.store.TakeLoginTransaction(r.Context(), providedState)
	if err != nil {
		http.Error(w, "login expired or already used", http.StatusBadRequest)
		return
	}

	if errVal := r.URL.Query().Get("error"); errVal != "" {
		http.Error(w, "login failed: "+errVal, http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	// CodeExchange validates the ID token's issuer, signature, audience,
	// expiry, and nonce (against expectedNonceContextKey, set just below)
	// via the RelyingParty's verifier before returning it - see NewHandler's
	// rp.WithNonce wiring. The nonce check is what stops a token legitimately
	// issued for a *different* login transaction from being accepted here.
	ctx := contextWithExpectedNonce(r.Context(), txn.Nonce)
	tokens, err := rp.CodeExchange[*oidc.IDTokenClaims](ctx, code, h.party, rp.WithCodeVerifier(txn.PKCEVerifier))
	if err != nil {
		slog.ErrorContext(r.Context(), "oidc token exchange failed", "error", err)
		http.Error(w, "token exchange failed", http.StatusUnauthorized)
		return
	}

	expiresAt := tokens.IDTokenClaims.GetExpiration()
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		http.Error(w, "token already expired", http.StatusUnauthorized)
		return
	}

	csrfToken, err := randomToken()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sessionID, err := h.store.CreateSession(r.Context(), session.Record{
		Subject:   tokens.IDTokenClaims.GetSubject(),
		IDToken:   tokens.IDToken,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
		CSRFToken: csrfToken,
	}, ttl)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, h.insecure, sessionID, ttl)
	http.Redirect(w, r, postLoginRedirect, http.StatusFound)
}

func withNonceParam(nonce string) rp.AuthURLOpt {
	return func() []oauth2.AuthCodeOption {
		return []oauth2.AuthCodeOption{oauth2.SetAuthURLParam("nonce", nonce)}
	}
}

func constantTimeEqual(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

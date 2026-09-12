package auth

import (
	"net/http"
	"time"
)

// Cookie base names. The __Host- prefix (Secure + Path=/ + no Domain) is
// applied whenever the deployment isn't running in loopback dev mode; see
// docs/spec/04-auth-session-browser-security.md on isolating that exception.
const (
	loginStateCookieBase = "td-login-state"
	sessionCookieBase    = "td-session"
)

func cookieName(base string, insecure bool) string {
	if insecure {
		return base
	}
	return "__Host-" + base
}

// LoginStateCookieName and SessionCookieName are exported so the httpapi
// package (logout, the session-required middleware) can read/clear the
// session cookie without duplicating this naming decision.
func LoginStateCookieName(insecure bool) string { return cookieName(loginStateCookieBase, insecure) }
func SessionCookieName(insecure bool) string    { return cookieName(sessionCookieBase, insecure) }

// setLoginStateCookie carries the login transaction's state across the
// redirect to the IdP and back. SameSite=Lax (not Strict) is required here:
// the browser must still send it on the top-level GET navigation back from
// the IdP to /auth/callback, which Strict would block. See
// docs/spec/04-auth-session-browser-security.md.
func setLoginStateCookie(w http.ResponseWriter, insecure bool, state string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     LoginStateCookieName(insecure),
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   !insecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
}

func clearLoginStateCookie(w http.ResponseWriter, insecure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     LoginStateCookieName(insecure),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   !insecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// setSessionCookie issues the opaque session ID. Strict is safe (and
// preferred over Lax) here: unlike the login-state cookie, this one is never
// needed on a cross-site-initiated request, only on same-origin app use.
func setSessionCookie(w http.ResponseWriter, insecure bool, sessionID string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName(insecure),
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   !insecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(ttl.Seconds()),
	})
}

// ClearSessionCookie is exported for logout (in httpapi) to reuse.
func ClearSessionCookie(w http.ResponseWriter, insecure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName(insecure),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   !insecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// loggedInHandler returns a Handler plus a valid session cookie and the
// session's own CSRF token, as if a login had already completed.
func loggedInHandler(t *testing.T) (h *Handler, sessionCookie *http.Cookie, csrfToken string) {
	t.Helper()
	op := newTestOP(t, defaultClaims(""))
	store := newTestStore(t)
	h = newTestHandler(t, op, true, store)

	state, nonce, loginCookie := login(t, h)
	op.claims.Nonce = nonce

	rec := httptest.NewRecorder()
	h.CallbackHandler(rec, callbackRequest(state, "test-code", loginCookie))
	if rec.Code != http.StatusFound {
		t.Fatalf("callback setup failed: status = %d, body = %q", rec.Code, rec.Body.String())
	}
	sessionCookie = findCookie(t, rec.Result().Cookies(), SessionCookieName(true))

	rec2, err := store.GetSession(context.Background(), sessionCookie.Value)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	return h, sessionCookie, rec2.CSRFToken
}

func decodeSessionStatus(t *testing.T, body []byte) sessionStatusResponse {
	t.Helper()
	var resp sessionStatusResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decoding session status response: %v", err)
	}
	return resp
}

func TestSessionHandler_ReportsUnauthenticatedWithNoCookie(t *testing.T) {
	op := newTestOP(t, defaultClaims(""))
	h := newTestHandler(t, op, true, newTestStore(t))

	rec := httptest.NewRecorder()
	h.SessionHandler(rec, httptest.NewRequest(http.MethodGet, "/auth/session", nil))

	resp := decodeSessionStatus(t, rec.Body.Bytes())
	if resp.Authenticated {
		t.Error("Authenticated = true, want false with no session cookie")
	}
}

func TestSessionHandler_ReportsAuthenticatedWithValidSession(t *testing.T) {
	h, cookie, csrfToken := loggedInHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.SessionHandler(rec, req)

	resp := decodeSessionStatus(t, rec.Body.Bytes())
	if !resp.Authenticated {
		t.Fatal("Authenticated = false, want true with a valid session cookie")
	}
	if resp.Subject != "alice" {
		t.Errorf("Subject = %q, want %q", resp.Subject, "alice")
	}
	if resp.CSRFToken != csrfToken {
		t.Errorf("CSRFToken = %q, want %q", resp.CSRFToken, csrfToken)
	}
}

func TestSessionHandler_ReportsUnauthenticatedForExpiredOrUnknownSessionID(t *testing.T) {
	op := newTestOP(t, defaultClaims(""))
	h := newTestHandler(t, op, true, newTestStore(t))

	req := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName(true), Value: "never-issued"})
	rec := httptest.NewRecorder()
	h.SessionHandler(rec, req)

	resp := decodeSessionStatus(t, rec.Body.Bytes())
	if resp.Authenticated {
		t.Error("Authenticated = true, want false for an unknown session ID")
	}
}

func TestSessionHandler_AllowsMissingOrigin(t *testing.T) {
	h, cookie, _ := loggedInHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.SessionHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (missing Origin must be allowed on a same-origin read)", rec.Code, http.StatusOK)
	}
}

func TestSessionHandler_RejectsMismatchedOrigin(t *testing.T) {
	h, cookie, _ := loggedInHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	req.AddCookie(cookie)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	h.SessionHandler(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func logoutRequest(cookie *http.Cookie, origin, csrfToken string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if csrfToken != "" {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}
	return req
}

func TestLogoutHandler_DeletesSessionOnValidRequest(t *testing.T) {
	h, cookie, csrfToken := loggedInHandler(t)

	rec := httptest.NewRecorder()
	h.LogoutHandler(rec, logoutRequest(cookie, "https://tenantdeck.example.com", csrfToken))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %q, want %d", rec.Code, rec.Body.String(), http.StatusNoContent)
	}
}

// This is the property that matters most: once logged out, the old cookie
// must not work anymore, even though the browser could still present it.
func TestLogoutHandler_OldCookieIsRejectedAfterLogout(t *testing.T) {
	h, cookie, csrfToken := loggedInHandler(t)

	first := httptest.NewRecorder()
	h.LogoutHandler(first, logoutRequest(cookie, "https://tenantdeck.example.com", csrfToken))
	if first.Code != http.StatusNoContent {
		t.Fatalf("logout setup failed: status = %d", first.Code)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	statusReq.AddCookie(cookie)
	statusRec := httptest.NewRecorder()
	h.SessionHandler(statusRec, statusReq)

	resp := decodeSessionStatus(t, statusRec.Body.Bytes())
	if resp.Authenticated {
		t.Error("session is still authenticated after logout, want it invalidated")
	}
}

func TestLogoutHandler_RejectsMissingOrigin(t *testing.T) {
	h, cookie, csrfToken := loggedInHandler(t)

	rec := httptest.NewRecorder()
	h.LogoutHandler(rec, logoutRequest(cookie, "", csrfToken))

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d (Origin must be required, not just checked-if-present, on logout)", rec.Code, http.StatusForbidden)
	}
}

func TestLogoutHandler_RejectsMismatchedOrigin(t *testing.T) {
	h, cookie, csrfToken := loggedInHandler(t)

	rec := httptest.NewRecorder()
	h.LogoutHandler(rec, logoutRequest(cookie, "https://evil.example.com", csrfToken))

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestLogoutHandler_RejectsMissingCSRFToken(t *testing.T) {
	h, cookie, _ := loggedInHandler(t)

	rec := httptest.NewRecorder()
	h.LogoutHandler(rec, logoutRequest(cookie, "https://tenantdeck.example.com", ""))

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestLogoutHandler_RejectsWrongCSRFToken(t *testing.T) {
	h, cookie, _ := loggedInHandler(t)

	rec := httptest.NewRecorder()
	h.LogoutHandler(rec, logoutRequest(cookie, "https://tenantdeck.example.com", "wrong-token"))

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestLogoutHandler_RejectsMissingSessionCookie(t *testing.T) {
	op := newTestOP(t, defaultClaims(""))
	h := newTestHandler(t, op, true, newTestStore(t))

	rec := httptest.NewRecorder()
	h.LogoutHandler(rec, logoutRequest(nil, "https://tenantdeck.example.com", "anything"))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLogoutHandler_RejectsUnknownSessionID(t *testing.T) {
	op := newTestOP(t, defaultClaims(""))
	h := newTestHandler(t, op, true, newTestStore(t))

	rec := httptest.NewRecorder()
	cookie := &http.Cookie{Name: SessionCookieName(true), Value: "never-issued"}
	h.LogoutHandler(rec, logoutRequest(cookie, "https://tenantdeck.example.com", "anything"))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLogoutHandler_ClearsSessionCookieOnSuccess(t *testing.T) {
	h, cookie, csrfToken := loggedInHandler(t)

	rec := httptest.NewRecorder()
	h.LogoutHandler(rec, logoutRequest(cookie, "https://tenantdeck.example.com", csrfToken))

	cleared := findCookie(t, rec.Result().Cookies(), SessionCookieName(true))
	if cleared.MaxAge >= 0 {
		t.Errorf("MaxAge = %d, want negative (cookie deleted)", cleared.MaxAge)
	}
}

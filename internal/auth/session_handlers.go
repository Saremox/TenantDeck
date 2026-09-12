package auth

import (
	"encoding/json"
	"net/http"
)

// sessionStatusResponse is the /auth/session body. csrfToken is only ever
// delivered here - in a same-origin JSON response read by the frontend's
// own JS - never in a cookie, per the synchronizer-token pattern used by
// LogoutHandler.
type sessionStatusResponse struct {
	Authenticated bool   `json:"authenticated"`
	Subject       string `json:"subject,omitempty"`
	CSRFToken     string `json:"csrfToken,omitempty"`
}

// SessionHandler reports whether the caller has a valid session, without
// ever requiring one - logged-out is not an error. GET is a read; per
// docs/spec/04-auth-session-browser-security.md, a same-origin request may
// have no Origin header at all, so a missing Origin is allowed here (unlike
// LogoutHandler). A *mismatched* Origin is still rejected.
func (h *Handler) SessionHandler(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" && origin != h.externalOrigin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	resp := sessionStatusResponse{Authenticated: false}
	if cookie, err := r.Cookie(SessionCookieName(h.insecure)); err == nil {
		if rec, err := h.store.GetSession(r.Context(), cookie.Value); err == nil {
			resp = sessionStatusResponse{Authenticated: true, Subject: rec.Subject, CSRFToken: rec.CSRFToken}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// LogoutHandler deletes the server-side session. State-changing, so unlike
// SessionHandler it requires an exact Origin match (present and equal, not
// just "absent is fine") plus the session's own CSRF token echoed back -
// see docs/spec/04-auth-session-browser-security.md.
func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != h.externalOrigin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	cookie, err := r.Cookie(SessionCookieName(h.insecure))
	if err != nil {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}

	rec, err := h.store.GetSession(r.Context(), cookie.Value)
	if err != nil {
		// Already gone (expired, already logged out, or the store failed -
		// either way there is nothing left to delete). Clear the client's
		// cookie regardless so it stops being sent.
		ClearSessionCookie(w, h.insecure)
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}

	if !constantTimeEqual(r.Header.Get("X-CSRF-Token"), rec.CSRFToken) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := h.store.DeleteSession(r.Context(), cookie.Value); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	ClearSessionCookie(w, h.insecure)
	w.WriteHeader(http.StatusNoContent)
}

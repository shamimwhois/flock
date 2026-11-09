package databaseadmin

import (
	"net/http"
)

// LogoutHandler revokes the current server-side session and expires its browser cookie.
// It is intentionally POST-only and applies the same loopback, Host, and Origin checks as
// session exchange so a third-party page cannot log the local Adminer session out.
func LogoutHandler(sessions *SessionStore, expectedHost string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /session/logout", func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w)
		if !isLoopbackRequest(r) || r.Host != expectedHost || !validOrigin(r, expectedHost) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		RevokeSession(sessions, w, r)
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

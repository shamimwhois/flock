package databaseadmin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookieName = "flock_dbadmin_session"
	sessionIdleTTL    = 15 * time.Minute
	sessionAbsoluteTTL = 2 * time.Hour
	maxExchangeBody   = 4 << 10
)

var ErrInvalidSession = errors.New("invalid or expired database admin session")

type sessionRecord struct {
	scope     LaunchScope
	lastSeenAt time.Time
	expiresAt time.Time
}

// SessionStore stores only digests of opaque browser session tokens.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[[sha256.Size]byte]sessionRecord
	now      func() time.Time
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[[sha256.Size]byte]sessionRecord), now: time.Now}
}

func (s *SessionStore) create(scope LaunchScope) (string, error) {
	if !validScope(scope) {
		return "", ErrInvalidScope
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	digest := sha256.Sum256([]byte(token))
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()
	for key, record := range s.sessions {
		if !now.Before(record.expiresAt) {
			delete(s.sessions, key)
		}
	}
	s.sessions[digest] = sessionRecord{scope: scope, lastSeenAt: now, expiresAt: now.Add(sessionAbsoluteTTL)}
	return token, nil
}

func (s *SessionStore) lookup(token string) (LaunchScope, error) {
	if _, ok := decodeOpaqueToken(token); !ok {
		return LaunchScope{}, ErrInvalidSession
	}
	digest := sha256.Sum256([]byte(token))
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.sessions[digest]
	if !ok {
		return LaunchScope{}, ErrInvalidSession
	}
	now := s.now()
	if !now.Before(record.expiresAt) || now.Sub(record.lastSeenAt) >= sessionIdleTTL {
		delete(s.sessions, digest)
		return LaunchScope{}, ErrInvalidSession
	}
	record.lastSeenAt = now
	s.sessions[digest] = record
	return record.scope, nil
}

func (s *SessionStore) revoke(token string) {
	if _, ok := decodeOpaqueToken(token); !ok {
		return
	}
	digest := sha256.Sum256([]byte(token))
	s.mu.Lock()
	delete(s.sessions, digest)
	s.mu.Unlock()
}

// ExchangeHandler exchanges a one-time launch ticket for an opaque HttpOnly session cookie.
// expectedHost must be the exact host:port bound by the dedicated loopback listener.
func ExchangeHandler(tickets *TicketStore, sessions *SessionStore, expectedHost string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /session/exchange", func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w)
		if !isLoopbackRequest(r) || r.Host != expectedHost || !validOrigin(r, expectedHost) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
			return
		}

		var body struct {
			Ticket string `json:"ticket"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxExchangeBody))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil || body.Ticket == "" {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		scope, err := tickets.Consume(body.Ticket)
		if err != nil {
			http.Error(w, "invalid or expired launch ticket", http.StatusUnauthorized)
			return
		}
		session, err := sessions.create(scope)
		if err != nil {
			http.Error(w, "could not create session", http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name: sessionCookieName, Value: session, Path: "/", HttpOnly: true,
			Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode,
			MaxAge: int(sessionAbsoluteTTL.Seconds()), Expires: time.Now().Add(sessionAbsoluteTTL),
		})
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

// SessionScope returns the authorization scope attached to a valid browser session.
func SessionScope(sessions *SessionStore, r *http.Request) (LaunchScope, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return LaunchScope{}, ErrInvalidSession
	}
	return sessions.lookup(cookie.Value)
}

// RevokeSession invalidates the current browser session and expires its cookie.
func RevokeSession(sessions *SessionStore, w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		sessions.revoke(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", HttpOnly: true,
		Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode,
		MaxAge: -1, Expires: time.Unix(1, 0),
	})
	setSecurityHeaders(w)
}

 
// ScopedHandler serves a request only when it comes from the loopback peer, uses the exact
// listener Host, and carries a live server-side session. The scope is supplied out-of-band.
func ScopedHandler(sessions *SessionStore, expectedHost string, next func(http.ResponseWriter, *http.Request, LaunchScope)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setAdminerSecurityHeaders(w)
		if !isLoopbackRequest(r) || r.Host != expectedHost {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		// Fail closed on connection identity in the browser URL. This is defense in
		// depth; the pinned Adminer compatibility patch must also prevent emitting
		// these fields in links, forms and redirects.
		if hasConnectionIdentityQuery(r.URL.RawQuery) {
			http.Error(w, "connection identity is not accepted in the URL", http.StatusBadRequest)
			return
		}
		if requiresOriginCheck(r.Method) && !validOrigin(r, expectedHost) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		scope, err := SessionScope(sessions, r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if scope.Kind == SiteScope && hasAdminerAuthPost(r) {
			http.Error(w, "site connection identity cannot be changed by the browser", http.StatusForbidden)
			return
		}
		next(w, r, scope)
	})
}


// hasAdminerAuthPost detects Adminer's generic login payload. Site-scoped sessions
// receive their connection from server-side scope and must not accept browser-selected
// driver/server/username/database fields. Global sessions retain the generic login flow.
func hasAdminerAuthPost(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
	if mediaType != "application/x-www-form-urlencoded" {
		return false
	}
	const maxInspectBody = 64 << 20
	body, err := io.ReadAll(io.LimitReader(r.Body, maxInspectBody+1))
	if err != nil || len(body) > maxInspectBody {
		r.Body = io.NopCloser(bytes.NewReader(body))
		return true
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return true
	}
	_, present := values["auth"]
	return present
}

// hasConnectionIdentityQuery rejects Adminer routing and credential fields from
// query strings while preserving ordinary page navigation such as ?table=users.
func hasConnectionIdentityQuery(rawQuery string) bool {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return true
	}
	for key := range values {
		normalized := strings.ToLower(strings.NewReplacer("[", "", "]", "", "_", "", "-", "").Replace(key))
		for _, identity := range []string{
			"username", "password", "server", "driver", "dsn", "ticket",
			"connectionid", "profileid", "mysql", "pgsql", "sqlite", "mssql",
			"oracle", "db", "ns",
		} {
			if normalized == identity || strings.HasSuffix(normalized, identity) {
				return true
			}
		}
	}
	return false
}

func isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}


// requiresOriginCheck identifies methods that can change server-side state.
func requiresOriginCheck(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}

func validOrigin(r *http.Request, expectedHost string) bool {
	origin := r.Header.Get("Origin")
	if origin == "" || strings.Contains(origin, ",") {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return origin == scheme+"://"+expectedHost
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
}

// setAdminerSecurityHeaders applies a same-origin-only policy to the authenticated Adminer
// surface. Adminer currently emits inline scripts and styles, so those are allowed only for
// this isolated loopback origin; remote scripts, frames, connections and form targets remain
// blocked. The Flock compatibility patch must keep all generated URLs same-origin.
func setAdminerSecurityHeaders(w http.ResponseWriter) {
	setSecurityHeaders(w)
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'; object-src 'none'")
}

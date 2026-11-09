package databaseadmin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func exchangeRequest(handler http.Handler, host, origin, remote, ticket string) *httptest.ResponseRecorder {
	body := `{"ticket":"` + ticket + `"}`
	req := httptest.NewRequest(http.MethodPost, "http://"+host+"/session/exchange", strings.NewReader(body))
	req.Host = host
	req.RemoteAddr = remote
	req.Header.Set("Origin", origin)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func TestExchangeCreatesScopedHttpOnlySession(t *testing.T) {
	tickets, sessions := NewTicketStore(), NewSessionStore()
	scope := LaunchScope{Kind: SiteScope, SiteID: "site-a", ConnectionID: "mysql-a"}
	ticket, err := tickets.Issue(scope)
	if err != nil {
		t.Fatal(err)
	}
	const host = "127.0.0.1:43127"
	handler := ExchangeHandler(tickets, sessions, host)
	response := exchangeRequest(handler, host, "http://"+host, "127.0.0.1:54321", ticket)

	if response.Code != http.StatusNoContent {
		t.Fatalf("exchange status = %d, want 204: %s", response.Code, response.Body)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("session cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Secure {
		t.Fatalf("unexpected loopback cookie attributes: %+v", cookie)
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("missing security headers: %v", response.Header())
	}

	req := httptest.NewRequest(http.MethodGet, "http://"+host+"/", nil)
	req.AddCookie(cookie)
	got, err := SessionScope(sessions, req)
	if err != nil || got != scope {
		t.Fatalf("SessionScope() = %#v, %v; want %#v", got, err, scope)
	}
}

func TestExchangeRejectsUntrustedOriginWithoutConsumingTicket(t *testing.T) {
	tickets, sessions := NewTicketStore(), NewSessionStore()
	ticket, err := tickets.Issue(LaunchScope{Kind: GlobalScope})
	if err != nil {
		t.Fatal(err)
	}
	const host = "127.0.0.1:43128"
	handler := ExchangeHandler(tickets, sessions, host)

	bad := exchangeRequest(handler, host, "http://evil.example", "127.0.0.1:54321", ticket)
	if bad.Code != http.StatusForbidden {
		t.Fatalf("untrusted origin status = %d, want 403", bad.Code)
	}
	good := exchangeRequest(handler, host, "http://"+host, "127.0.0.1:54321", ticket)
	if good.Code != http.StatusNoContent {
		t.Fatalf("valid retry status = %d, want 204", good.Code)
	}
}

func TestExchangeRejectsNonLoopbackPeerAndHostMismatch(t *testing.T) {
	tickets := NewTicketStore()
	const host = "127.0.0.1:43129"
	handler := ExchangeHandler(tickets, NewSessionStore(), host)

	if got := exchangeRequest(handler, host, "http://"+host, "192.0.2.10:1234", ""); got.Code != http.StatusForbidden {
		t.Fatalf("remote peer status = %d, want 403", got.Code)
	}
	if got := exchangeRequest(handler, "127.0.0.1:43130", "http://127.0.0.1:43130", "127.0.0.1:1234", ""); got.Code != http.StatusForbidden {
		t.Fatalf("wrong Host status = %d, want 403", got.Code)
	}
}

func TestSessionExpiryAndRevocation(t *testing.T) {
	store := NewSessionStore()
	now := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	scope := LaunchScope{Kind: GlobalScope}

	token, err := store.create(scope)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := store.lookup(token); err != nil || got != scope {
		t.Fatalf("lookup() = %#v, %v", got, err)
	}
	store.revoke(token)
	if _, err := store.lookup(token); err != ErrInvalidSession {
		t.Fatalf("lookup after revoke error = %v, want ErrInvalidSession", err)
	}

	token, err = store.create(scope)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(sessionAbsoluteTTL)
	if _, err := store.lookup(token); err != ErrInvalidSession {
		t.Fatalf("lookup after expiry error = %v, want ErrInvalidSession", err)
	}
}

func TestSessionIdleExpiryIsNotExtendedBeyondAbsoluteLifetime(t *testing.T) {
	store := NewSessionStore()
	now := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	token, err := store.create(LaunchScope{Kind: GlobalScope})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(sessionIdleTTL)
	if _, err := store.lookup(token); err != ErrInvalidSession {
		t.Fatalf("lookup after idle timeout error = %v, want ErrInvalidSession", err)
	}
}

func TestScopedHandlerRequiresLoopbackHostAndLiveSession(t *testing.T) {
	sessions := NewSessionStore()
	scope := LaunchScope{Kind: SiteScope, SiteID: "site-x", ConnectionID: "pg-x"}
	token, err := sessions.create(scope)
	if err != nil {
		t.Fatal(err)
	}
	const host = "127.0.0.1:43131"
	var servedScope LaunchScope
	handler := ScopedHandler(sessions, host, func(w http.ResponseWriter, _ *http.Request, got LaunchScope) {
		servedScope = got
		w.WriteHeader(http.StatusNoContent)
	})

	request := func(requestHost, remote, cookie string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "http://"+requestHost+"/", nil)
		req.Host = requestHost
		req.RemoteAddr = remote
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookie})
		}
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		return recorder
	}

	if got := request(host, "127.0.0.1:54321", ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("missing session status = %d, want 401", got.Code)
	}
	if got := request(host, "192.0.2.1:54321", token); got.Code != http.StatusForbidden {
		t.Fatalf("non-loopback status = %d, want 403", got.Code)
	}
	if got := request("127.0.0.1:43132", "127.0.0.1:54321", token); got.Code != http.StatusForbidden {
		t.Fatalf("wrong Host status = %d, want 403", got.Code)
	}
	if got := request(host, "127.0.0.1:54321", token); got.Code != http.StatusNoContent || servedScope != scope {
		t.Fatalf("authorized request = %d, scope %#v", got.Code, servedScope)
	}
	authorized := request(host, "127.0.0.1:54321", token)
	csp := authorized.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'self'") || !strings.Contains(csp, "form-action 'self'") || !strings.Contains(csp, "object-src 'none'") {
		t.Fatalf("Adminer CSP is missing same-origin restrictions: %q", csp)
	}
}

func TestScopedHandlerRejectsConnectionIdentityInQuery(t *testing.T) {
	sessions := NewSessionStore()
	token, err := sessions.create(LaunchScope{Kind: GlobalScope})
	if err != nil {
		t.Fatal(err)
	}
	const host = "127.0.0.1:43134"
	handler := ScopedHandler(sessions, host, func(w http.ResponseWriter, _ *http.Request, _ LaunchScope) {
		w.WriteHeader(http.StatusNoContent)
	})

	for _, query := range []string{
		"username=admin",
		"db=production",
		"ns=private",
		"mysql=db.internal",
		"pgsql=db.internal",
		"auth%5Busername%5D=admin",
		"auth%5Bdb%5D=production",
		"connection_id=secret",
		"profile-id=secret",
		"password=secret",
		"ticket=secret",
		"malformed=%zz",
	} {
		t.Run(query, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://"+host+"/?"+query, nil)
			req.RemoteAddr = "127.0.0.1:54321"
			req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("query %q status = %d, want 400", query, response.Code)
			}
		})
	}
}

func TestScopedHandlerAllowsOrdinaryAdminerNavigationQuery(t *testing.T) {
	sessions := NewSessionStore()
	token, err := sessions.create(LaunchScope{Kind: GlobalScope})
	if err != nil {
		t.Fatal(err)
	}
	const host = "127.0.0.1:43135"
	handler := ScopedHandler(sessions, host, func(w http.ResponseWriter, _ *http.Request, _ LaunchScope) {
		w.WriteHeader(http.StatusNoContent)
	})
	for _, query := range []string{"table=users", "select=users", "sql=", "dump=users", "script=connect"} {
		req := httptest.NewRequest(http.MethodGet, "http://"+host+"/?"+query, nil)
		req.RemoteAddr = "127.0.0.1:54321"
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != http.StatusNoContent {
			t.Errorf("ordinary query %q status = %d, want 204", query, response.Code)
		}
	}
}

func TestScopedHandlerRequiresSameOriginForStateChanges(t *testing.T) {
	sessions := NewSessionStore()
	token, err := sessions.create(LaunchScope{Kind: GlobalScope})
	if err != nil {
		t.Fatal(err)
	}
	const host = "127.0.0.1:43133"
	handler := ScopedHandler(sessions, host, func(w http.ResponseWriter, _ *http.Request, _ LaunchScope) {
		w.WriteHeader(http.StatusNoContent)
	})

	request := func(method, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://"+host+"/", nil)
		req.Host = host
		req.RemoteAddr = "127.0.0.1:54321"
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		return recorder
	}

	if got := request(http.MethodPost, ""); got.Code != http.StatusForbidden {
		t.Fatalf("missing Origin status = %d, want 403", got.Code)
	}
	if got := request(http.MethodPost, "http://attacker.example"); got.Code != http.StatusForbidden {
		t.Fatalf("cross-origin POST status = %d, want 403", got.Code)
	}
	if got := request(http.MethodPost, "http://"+host); got.Code != http.StatusNoContent {
		t.Fatalf("same-origin POST status = %d, want 204", got.Code)
	}
	if got := request(http.MethodGet, ""); got.Code != http.StatusNoContent {
		t.Fatalf("GET without Origin status = %d, want 204", got.Code)
	}
}

func TestExchangeRejectsInvalidBodiesWithoutConsumingTicket(t *testing.T) {
	const host = "127.0.0.1:43127"
	for _, tc := range []struct {
		name, body, contentType string
		status                  int
	}{
		{"empty", "", "application/json", 400},
		{"missing ticket", `{}`, "application/json", 400},
		{"null", `null`, "application/json", 400},
		{"wrong type", `{"ticket":42}`, "application/json", 400},
		{"truncated", `{"ticket":`, "application/json", 400},
		{"unknown field", `{"ticket":"%s","password":"secret"}`, "application/json", 400},
		{"second value", `{"ticket":"%s"} {}`, "application/json", 400},
		{"trailing junk", `{"ticket":"%s"}!`, "application/json", 400},
		{"oversized", `{"ticket":"%s","extra":"` + strings.Repeat("x", maxExchangeBody) + `"}`, "application/json", 400},
		{"missing content type", `{"ticket":"%s"}`, "", 415},
		{"wrong content type", `{"ticket":"%s"}`, "text/plain", 415},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tickets, sessions := NewTicketStore(), NewSessionStore()
			ticket, err := tickets.Issue(LaunchScope{Kind: GlobalScope})
			if err != nil {
				t.Fatal(err)
			}
			handler := ExchangeHandler(tickets, sessions, host)
			req := httptest.NewRequest(http.MethodPost, "http://"+host+"/session/exchange", strings.NewReader(strings.ReplaceAll(tc.body, "%s", ticket)))
			req.RemoteAddr = "127.0.0.1:1234"
			req.Header.Set("Origin", "http://"+host)
			req.Header.Set("Content-Type", tc.contentType)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d", response.Code, tc.status)
			}
			if len(response.Result().Cookies()) != 0 || len(sessions.sessions) != 0 {
				t.Fatal("rejected request created a session")
			}
			if got := exchangeRequest(handler, host, "http://"+host, "127.0.0.1:1234", ticket); got.Code != http.StatusNoContent {
				t.Fatalf("valid retry status = %d", got.Code)
			}
		})
	}
}

func TestExchangeReplayDoesNotCreateAnotherSession(t *testing.T) {
	tickets, sessions := NewTicketStore(), NewSessionStore()
	ticket, err := tickets.Issue(LaunchScope{Kind: GlobalScope})
	if err != nil {
		t.Fatal(err)
	}
	const host = "127.0.0.1:43127"
	handler := ExchangeHandler(tickets, sessions, host)
	for i, want := range []int{http.StatusNoContent, http.StatusUnauthorized} {
		response := exchangeRequest(handler, host, "http://"+host, "127.0.0.1:1234", ticket)
		if response.Code != want {
			t.Fatalf("exchange %d status = %d, want %d", i, response.Code, want)
		}
		if i == 1 && len(response.Result().Cookies()) != 0 {
			t.Fatal("replay set a cookie")
		}
	}
	if len(sessions.sessions) != 1 {
		t.Fatalf("sessions = %d, want 1", len(sessions.sessions))
	}
}

func TestActiveSessionStillExpiresAtAbsoluteDeadline(t *testing.T) {
	store := NewSessionStore()
	start := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	now := start
	store.now = func() time.Time { return now }
	want := LaunchScope{Kind: SiteScope, SiteID: "site", ConnectionID: "db"}
	token, err := store.create(want)
	if err != nil {
		t.Fatal(err)
	}
	for now = start.Add(10 * time.Minute); now.Before(start.Add(sessionAbsoluteTTL)); now = now.Add(10 * time.Minute) {
		if got, err := store.lookup(token); err != nil || got != want {
			t.Fatalf("active lookup at %v = %#v, %v", now, got, err)
		}
	}
	now = start.Add(sessionAbsoluteTTL - time.Nanosecond)
	if _, err := store.lookup(token); err != nil {
		t.Fatalf("before absolute deadline: %v", err)
	}
	now = start.Add(sessionAbsoluteTTL)
	if _, err := store.lookup(token); err != ErrInvalidSession {
		t.Fatalf("at absolute deadline: %v", err)
	}
	if len(store.sessions) != 0 {
		t.Fatal("expired session retained")
	}
}

func TestSessionIdleDeadlineRefreshesOnActivity(t *testing.T) {
	store := NewSessionStore()
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	token, err := store.create(LaunchScope{Kind: GlobalScope})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		now = now.Add(sessionIdleTTL - time.Nanosecond)
		if _, err := store.lookup(token); err != nil {
			t.Fatalf("active session expired: %v", err)
		}
	}
	now = now.Add(sessionIdleTTL)
	if _, err := store.lookup(token); err != ErrInvalidSession {
		t.Fatalf("idle deadline error = %v", err)
	}
}

func TestScopedHandlerOriginPolicy(t *testing.T) {
	const host = "127.0.0.1:43127"
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, "CUSTOM"} {
		for _, origin := range []string{"", "null", "http://attacker.example", "http://" + host + ", http://attacker.example", "http://" + host + "/", "https://" + host, "http://" + host} {
			t.Run(method+"/"+origin, func(t *testing.T) {
				sessions := NewSessionStore()
				token, err := sessions.create(LaunchScope{Kind: GlobalScope})
				if err != nil {
					t.Fatal(err)
				}
				called := false
				handler := ScopedHandler(sessions, host, func(w http.ResponseWriter, _ *http.Request, scope LaunchScope) {
					called = true
					if scope != (LaunchScope{Kind: GlobalScope}) {
						t.Errorf("scope = %#v", scope)
					}
					w.WriteHeader(http.StatusNoContent)
				})
				req := httptest.NewRequest(method, "http://"+host+"/", nil)
				req.RemoteAddr = "127.0.0.1:1234"
				req.Header.Set("Origin", origin)
				req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, req)
				safe := method == "GET" || method == "HEAD" || method == "OPTIONS" || method == "TRACE"
				allowed := safe || origin == "http://"+host
				want := http.StatusForbidden
				if allowed {
					want = http.StatusNoContent
				}
				if response.Code != want || called != allowed {
					t.Fatalf("status = %d, called = %v; want %d, %v", response.Code, called, want, allowed)
				}
			})
		}
	}
}

func TestSessionStoreRejectsNonCanonicalBase64URL(t *testing.T) {
	store := NewSessionStore()
	canonical := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	nonCanonical := canonical[:len(canonical)-1] + "B" // non-zero unused trailing bits
	if _, err := store.lookup(nonCanonical); err != ErrInvalidSession {
		t.Fatalf("lookup(non-canonical token) error = %v, want ErrInvalidSession", err)
	}
	store.revoke(nonCanonical)
	if len(store.sessions) != 0 {
		t.Fatal("non-canonical token changed session state")
	}
}

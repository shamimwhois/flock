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
	now = now.Add(sessionTTL)
	if _, err := store.lookup(token); err != ErrInvalidSession {
		t.Fatalf("lookup after expiry error = %v, want ErrInvalidSession", err)
	}
}

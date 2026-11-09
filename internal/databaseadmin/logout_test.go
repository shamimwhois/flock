package databaseadmin

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLogoutRevokesSessionAndExpiresCookie(t *testing.T) {
	sessions := NewSessionStore()
	scope := LaunchScope{Kind: GlobalScope}
	token, err := sessions.create(scope)
	if err != nil {
		t.Fatal(err)
	}

	const host = "127.0.0.1:43140"
	handler := LogoutHandler(sessions, host)
	req := httptest.NewRequest(http.MethodPost, "http://"+host+"/session/logout", nil)
	req.Host = host
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("Origin", "http://"+host)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204: %s", recorder.Code, recorder.Body)
	}
	if _, err := sessions.lookup(token); err != ErrInvalidSession {
		t.Fatalf("session lookup after logout error = %v, want ErrInvalidSession", err)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName || cookies[0].MaxAge >= 0 {
		t.Fatalf("logout cookie was not expired: %+v", cookies)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("logout response missing no-store: %v", recorder.Header())
	}
}

func TestLogoutRejectsCrossOriginAndWrongMethod(t *testing.T) {
	sessions := NewSessionStore()
	token, err := sessions.create(LaunchScope{Kind: GlobalScope})
	if err != nil {
		t.Fatal(err)
	}
	const host = "127.0.0.1:43141"
	handler := LogoutHandler(sessions, host)

	request := func(method, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://"+host+"/session/logout", nil)
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

	if got := request(http.MethodPost, "http://attacker.example"); got.Code != http.StatusForbidden {
		t.Fatalf("cross-origin logout status = %d, want 403", got.Code)
	}
	if got := request(http.MethodGet, "http://"+host); got.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET logout status = %d, want 405", got.Code)
	}
	if _, err := sessions.lookup(token); err != nil {
		t.Fatalf("rejected logout changed session state: %v", err)
	}
}

func TestRejectedLogoutPreservesSession(t *testing.T) {
	const host = "127.0.0.1:43141"
	for _, tc := range []struct{ name, host, remote, origin string }{
		{"wrong host", "localhost:43141", "127.0.0.1:1234", "http://" + host},
		{"remote peer", host, "192.0.2.1:1234", "http://" + host},
		{"malformed peer", host, "127.0.0.1", "http://" + host},
		{"missing origin", host, "127.0.0.1:1234", ""},
		{"null origin", host, "127.0.0.1:1234", "null"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sessions := NewSessionStore()
			scope := LaunchScope{Kind: GlobalScope}
			token, err := sessions.create(scope)
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "http://"+tc.host+"/session/logout", nil)
			req.RemoteAddr = tc.remote
			req.Header.Set("Origin", tc.origin)
			req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
			response := httptest.NewRecorder()
			LogoutHandler(sessions, host).ServeHTTP(response, req)
			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d", response.Code)
			}
			if len(response.Result().Cookies()) != 0 {
				t.Fatal("rejected logout changed browser cookie")
			}
			if got, err := sessions.lookup(token); err != nil || got != scope {
				t.Fatalf("session changed: %#v, %v", got, err)
			}
		})
	}
}

func TestLogoutWithoutLiveSessionIsIdempotentAndPreservesOtherSessions(t *testing.T) {
	const host = "127.0.0.1:43141"
	sessions := NewSessionStore()
	scope := LaunchScope{Kind: SiteScope, SiteID: "site", ConnectionID: "db"}
	other, err := sessions.create(scope)
	if err != nil {
		t.Fatal(err)
	}
	handler := LogoutHandler(sessions, host)
	for _, token := range []string{"", "malformed", "malformed"} {
		req := httptest.NewRequest(http.MethodPost, "https://"+host+"/session/logout", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		req.Header.Set("Origin", "https://"+host)
		if token != "" {
			req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != http.StatusNoContent {
			t.Fatalf("status = %d", response.Code)
		}
		cookies := response.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("cookies = %v", cookies)
		}
		cookie := cookies[0]
		if cookie.Name != sessionCookieName || cookie.Value != "" || cookie.Path != "/" || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.MaxAge != -1 || !cookie.Expires.Before(time.Now()) {
			t.Fatalf("invalid expired cookie: %+v", cookie)
		}
		if got, err := sessions.lookup(other); err != nil || got != scope {
			t.Fatalf("unrelated session changed: %#v, %v", got, err)
		}
	}
}

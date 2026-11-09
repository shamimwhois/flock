package databaseadmin

import (
	"encoding/base64"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTicketStoreConsumesOnceAndPreservesScope(t *testing.T) {
	store := NewTicketStore()
	want := LaunchScope{Kind: SiteScope, SiteID: "site-1", ConnectionID: "db-1"}

	token, err := store.Issue(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.Consume(token)
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if got != want {
		t.Fatalf("Consume() scope = %#v, want %#v", got, want)
	}
	if _, err := store.Consume(token); err != ErrInvalidTicket {
		t.Fatalf("second Consume() error = %v, want ErrInvalidTicket", err)
	}
}

func TestTicketStoreRejectsExpiredTicket(t *testing.T) {
	store := NewTicketStore()
	now := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	token, err := store.Issue(LaunchScope{Kind: GlobalScope})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(launchTicketTTL)
	if _, err := store.Consume(token); err != ErrInvalidTicket {
		t.Fatalf("Consume() error = %v, want ErrInvalidTicket", err)
	}
}

func TestTicketStoreRejectsMalformedAndInvalidScope(t *testing.T) {
	store := NewTicketStore()
	if _, err := store.Issue(LaunchScope{Kind: SiteScope, SiteID: "site-1"}); err != ErrInvalidScope {
		t.Fatalf("Issue() error = %v, want ErrInvalidScope", err)
	}
	if _, err := store.Consume("not-a-ticket"); err != ErrInvalidTicket {
		t.Fatalf("Consume() error = %v, want ErrInvalidTicket", err)
	}
}

func TestTicketStoreAllowsOnlyOneConcurrentConsumer(t *testing.T) {
	store := NewTicketStore()
	token, err := store.Issue(LaunchScope{Kind: GlobalScope})
	if err != nil {
		t.Fatal(err)
	}

	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.Consume(token); err == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := successes.Load(); got != 1 {
		t.Fatalf("successful consumers = %d, want 1", got)
	}
}

func TestStoresRejectInvalidScopes(t *testing.T) {
	for _, scope := range []LaunchScope{
		{}, {Kind: "unknown"}, {Kind: GlobalScope, SiteID: "site"},
		{Kind: GlobalScope, ConnectionID: "db"}, {Kind: SiteScope},
		{Kind: SiteScope, SiteID: "site"}, {Kind: SiteScope, ConnectionID: "db"},
	} {
		t.Run(string(scope.Kind)+"/"+scope.SiteID+"/"+scope.ConnectionID, func(t *testing.T) {
			tickets, sessions := NewTicketStore(), NewSessionStore()
			if token, err := tickets.Issue(scope); token != "" || err != ErrInvalidScope {
				t.Fatalf("Issue() = %q, %v", token, err)
			}
			if token, err := sessions.create(scope); token != "" || err != ErrInvalidScope {
				t.Fatalf("create() = %q, %v", token, err)
			}
			if len(tickets.tickets) != 0 || len(sessions.sessions) != 0 {
				t.Fatal("invalid scope created state")
			}
		})
	}
}

func TestStoresRejectMalformedAndUnknownTokens(t *testing.T) {
	tickets, sessions := NewTicketStore(), NewSessionStore()
	for _, token := range []string{"", "!", strings.Repeat("a", 42), base64.RawURLEncoding.EncodeToString(make([]byte, 33)), base64.RawURLEncoding.EncodeToString(make([]byte, 32))} {
		t.Run(token, func(t *testing.T) {
			if scope, err := tickets.Consume(token); err != ErrInvalidTicket || scope != (LaunchScope{}) {
				t.Fatalf("Consume() = %#v, %v", scope, err)
			}
			if scope, err := sessions.lookup(token); err != ErrInvalidSession || scope != (LaunchScope{}) {
				t.Fatalf("lookup() = %#v, %v", scope, err)
			}
			sessions.revoke(token)
		})
	}
}

func TestTicketValidImmediatelyBeforeExpiry(t *testing.T) {
	store := NewTicketStore()
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	want := LaunchScope{Kind: GlobalScope}
	token, err := store.Issue(want)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(launchTicketTTL - time.Nanosecond)
	if got, err := store.Consume(token); err != nil || got != want {
		t.Fatalf("Consume() = %#v, %v", got, err)
	}
}

func TestTicketStoreRejectsNonCanonicalBase64URL(t *testing.T) {
	store := NewTicketStore()
	canonical := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	nonCanonical := canonical[:len(canonical)-1] + "B" // non-zero unused trailing bits
	if _, err := store.Consume(nonCanonical); err != ErrInvalidTicket {
		t.Fatalf("Consume(non-canonical token) error = %v, want ErrInvalidTicket", err)
	}
	if _, ok := decodeOpaqueToken(canonical); !ok {
		t.Fatal("canonical token was rejected")
	}
	if _, ok := decodeOpaqueToken(nonCanonical); ok {
		t.Fatal("non-canonical token was accepted")
	}
}

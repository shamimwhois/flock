package databaseadmin

import (
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

// Package databaseadmin contains the security primitives for Flock's local database
// administration feature. It deliberately has no dependency on the public site proxy.
package databaseadmin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

const launchTicketTTL = 60 * time.Second

var (
	ErrInvalidTicket = errors.New("invalid or expired database admin launch ticket")
	ErrInvalidScope  = errors.New("invalid database admin launch scope")
)

// ScopeKind identifies whether a launch belongs to the global entry point or one site.
type ScopeKind string

const (
	GlobalScope ScopeKind = "global"
	SiteScope   ScopeKind = "site"
)

// LaunchScope is server-side authorization context. It must never be serialized into a URL.
type LaunchScope struct {
	Kind         ScopeKind
	SiteID       string
	ConnectionID string
}

// TicketStore holds only SHA-256 digests of one-time launch tickets.
// Consume deletes a ticket before returning its scope, making concurrent reuse impossible.
type TicketStore struct {
	mu      sync.Mutex
	tickets map[[sha256.Size]byte]ticketRecord
	now     func() time.Time
}

type ticketRecord struct {
	scope     LaunchScope
	expiresAt time.Time
}

// NewTicketStore creates an in-memory, process-local ticket store.
func NewTicketStore() *TicketStore {
	return &TicketStore{
		tickets: make(map[[sha256.Size]byte]ticketRecord),
		now:     time.Now,
	}
}

// Issue creates a cryptographically random 256-bit ticket with a fixed 60-second lifetime.
// The returned bearer value is shown only to the caller; the store retains only its digest.
func (s *TicketStore) Issue(scope LaunchScope) (string, error) {
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
	for key, record := range s.tickets {
		if !now.Before(record.expiresAt) {
			delete(s.tickets, key)
		}
	}
	s.tickets[digest] = ticketRecord{
		scope:     scope,
		expiresAt: now.Add(launchTicketTTL),
	}

	return token, nil
}

// Consume atomically validates and consumes a ticket. A ticket can succeed exactly once.
func (s *TicketStore) Consume(token string) (LaunchScope, error) {
	if _, ok := decodeOpaqueToken(token); !ok {
		return LaunchScope{}, ErrInvalidTicket
	}
	digest := sha256.Sum256([]byte(token))

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.tickets[digest]
	if !ok {
		return LaunchScope{}, ErrInvalidTicket
	}
	delete(s.tickets, digest)
	if !s.now().Before(record.expiresAt) {
		return LaunchScope{}, ErrInvalidTicket
	}

	return record.scope, nil
}

func validScope(scope LaunchScope) bool {
	switch scope.Kind {
	case GlobalScope:
		return scope.SiteID == "" && scope.ConnectionID == ""
	case SiteScope:
		return scope.SiteID != "" && scope.ConnectionID != ""
	default:
		return false
	}
}

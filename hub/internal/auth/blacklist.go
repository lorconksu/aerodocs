package auth

import (
	"sync"
	"time"
)

// TokenBlacklist tracks revoked JWTs by their JTI until they expire naturally.
type TokenBlacklist struct {
	mu      sync.RWMutex
	entries map[string]time.Time // jti -> expiry
}

// NewTokenBlacklist creates an empty token blacklist.
func NewTokenBlacklist() *TokenBlacklist {
	return &TokenBlacklist{
		entries: make(map[string]time.Time),
	}
}

// Add marks a token JTI as revoked. Expired entries are cleaned up on each add.
func (b *TokenBlacklist) Add(jti string, expiry time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries[jti] = expiry

	// Cleanup expired entries
	now := time.Now()
	for k, exp := range b.entries {
		if now.After(exp) {
			delete(b.entries, k)
		}
	}
}

// IsBlacklisted returns true if the given JTI has been revoked.
func (b *TokenBlacklist) IsBlacklisted(jti string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, ok := b.entries[jti]
	return ok
}

// Len returns the number of entries in the blacklist (for testing).
func (b *TokenBlacklist) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.entries)
}

package auth

import (
	"database/sql"
	"log"
	"sync"
	"time"
)

const sqliteTimeLayout = "2006-01-02 15:04:05"

// TokenBlacklist tracks revoked JWTs by their JTI until they expire naturally.
// Entries are persisted to a SQLite table so revocations survive restarts.
type TokenBlacklist struct {
	mu      sync.RWMutex
	entries map[string]time.Time // jti -> expiry
	db      *sql.DB              // nil = in-memory only (tests)
}

// NewTokenBlacklist creates a token blacklist backed by the given database.
// If db is nil, the blacklist operates in memory only (useful for tests).
// On startup, all non-expired entries are loaded from the token_blacklist table.
func NewTokenBlacklist(db ...*sql.DB) *TokenBlacklist {
	b := &TokenBlacklist{
		entries: make(map[string]time.Time),
	}
	if len(db) > 0 && db[0] != nil {
		b.db = db[0]
		b.loadFromDB()
		go b.periodicCleanup()
	}
	return b
}

// loadFromDB loads all non-expired entries from the database into memory.
func (b *TokenBlacklist) loadFromDB() {
	if b.db == nil {
		return
	}
	rows, err := b.db.Query("SELECT jti, expires_at FROM token_blacklist WHERE expires_at > datetime('now')")
	if err != nil {
		log.Printf("auth: load token blacklist from DB: %v", err)
		return
	}
	defer rows.Close()

	b.mu.Lock()
	defer b.mu.Unlock()
	for rows.Next() {
		var jti, expiresAt string
		if err := rows.Scan(&jti, &expiresAt); err != nil {
			log.Printf("auth: scan token blacklist row: %v", err)
			continue
		}
		t, err := time.Parse(sqliteTimeLayout, expiresAt)
		if err != nil {
			// Try RFC3339 format
			t, err = time.Parse(time.RFC3339, expiresAt)
			if err != nil {
				log.Printf("auth: parse token blacklist expiry %q: %v", expiresAt, err)
				continue
			}
		}
		b.entries[jti] = t
	}
}

// periodicCleanup removes expired entries from both memory and DB every 5 minutes.
func (b *TokenBlacklist) periodicCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		b.cleanup()
	}
}

// cleanup removes expired entries from memory and DB.
func (b *TokenBlacklist) cleanup() {
	now := time.Now()
	b.mu.Lock()
	for k, exp := range b.entries {
		if now.After(exp) {
			delete(b.entries, k)
		}
	}
	b.mu.Unlock()

	if b.db != nil {
		if _, err := b.db.Exec("DELETE FROM token_blacklist WHERE expires_at <= datetime('now')"); err != nil {
			log.Printf("auth: cleanup expired tokens from DB: %v", err)
		}
	}
}

// Add marks a token JTI as revoked. The entry is stored in both memory and DB.
// Expired entries in memory are cleaned up on each add.
func (b *TokenBlacklist) Add(jti string, expiry time.Time) {
	b.mu.Lock()
	b.entries[jti] = expiry

	// Cleanup expired entries from memory
	now := time.Now()
	for k, exp := range b.entries {
		if now.After(exp) {
			delete(b.entries, k)
		}
	}
	b.mu.Unlock()

	// Persist to DB
	if b.db != nil {
		_, err := b.db.Exec(
			"INSERT OR REPLACE INTO token_blacklist (jti, expires_at) VALUES (?, ?)",
			jti, expiry.UTC().Format(sqliteTimeLayout),
		)
		if err != nil {
			log.Printf("auth: persist token blacklist entry: %v", err)
		}
	}
}

// IsBlacklisted returns true if the given JTI has been revoked.
// Checks the in-memory map first (fast path), then falls back to the DB.
func (b *TokenBlacklist) IsBlacklisted(jti string) bool {
	b.mu.RLock()
	_, ok := b.entries[jti]
	b.mu.RUnlock()
	if ok {
		return true
	}

	// DB fallback — entry might have been added by another process
	if b.db != nil {
		var count int
		err := b.db.QueryRow(
			"SELECT COUNT(*) FROM token_blacklist WHERE jti = ? AND expires_at > datetime('now')", jti,
		).Scan(&count)
		if err == nil && count > 0 {
			// Cache it in memory for future lookups
			b.mu.Lock()
			// Re-read expiry from DB to cache properly
			var expiresAt string
			if err := b.db.QueryRow("SELECT expires_at FROM token_blacklist WHERE jti = ?", jti).Scan(&expiresAt); err == nil {
				if t, err := time.Parse(sqliteTimeLayout, expiresAt); err == nil {
					b.entries[jti] = t
				}
			}
			b.mu.Unlock()
			return true
		}
	}

	return false
}

// Len returns the number of entries in the blacklist (for testing).
func (b *TokenBlacklist) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.entries)
}

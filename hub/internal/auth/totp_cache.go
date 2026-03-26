package auth

import (
	"sync"
	"time"
)

// TOTPUsedCodes tracks recently used TOTP codes to prevent replay attacks.
type TOTPUsedCodes struct {
	mu    sync.Mutex
	codes map[string]time.Time // key: "userID:code"
}

func NewTOTPUsedCodes() *TOTPUsedCodes {
	return &TOTPUsedCodes{codes: make(map[string]time.Time)}
}

// MarkUsed records a code as used. Cleans up entries older than 90s.
func (c *TOTPUsedCodes) MarkUsed(userID, code string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.codes[userID+":"+code] = time.Now()
	// Cleanup stale entries
	for k, t := range c.codes {
		if time.Since(t) > 90*time.Second {
			delete(c.codes, k)
		}
	}
}

// WasUsed returns true if the code was recently used for this user.
func (c *TOTPUsedCodes) WasUsed(userID, code string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.codes[userID+":"+code]
	return ok
}

// CheckAndMark atomically checks if a code was used and marks it if not.
// Returns true if the code is fresh (not previously used), false if it was already used.
func (c *TOTPUsedCodes) CheckAndMark(userID, code string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := userID + ":" + code
	if _, ok := c.codes[key]; ok {
		return false // already used
	}
	c.codes[key] = time.Now()
	// Cleanup stale entries
	for k, t := range c.codes {
		if time.Since(t) > 90*time.Second {
			delete(c.codes, k)
		}
	}
	return true // fresh code
}

// Clear removes all tracked codes. Intended for use in tests.
func (c *TOTPUsedCodes) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.codes = make(map[string]time.Time)
}

package auth

import (
	"testing"
	"time"
)

func TestTOTPUsedCodes_MarkAndCheck(t *testing.T) {
	cache := NewTOTPUsedCodes()

	if cache.WasUsed("user1", "123456") {
		t.Fatal("expected unused code to return false")
	}

	cache.MarkUsed("user1", "123456")

	if !cache.WasUsed("user1", "123456") {
		t.Fatal("expected used code to return true")
	}
}

func TestTOTPUsedCodes_DifferentUsers(t *testing.T) {
	cache := NewTOTPUsedCodes()

	cache.MarkUsed("user1", "123456")

	if cache.WasUsed("user2", "123456") {
		t.Fatal("code used by user1 should not affect user2")
	}
}

func TestTOTPUsedCodes_DifferentCodes(t *testing.T) {
	cache := NewTOTPUsedCodes()

	cache.MarkUsed("user1", "123456")

	if cache.WasUsed("user1", "654321") {
		t.Fatal("different code should not be marked as used")
	}
}

func TestTOTPUsedCodes_ReplayPrevented(t *testing.T) {
	cache := NewTOTPUsedCodes()

	// Simulate: generate a valid code, use it, then try again
	secret := "JBSWY3DPEHPK3PXP" // test secret
	code, err := GenerateValidCode(secret)
	if err != nil {
		t.Fatalf("failed to generate code: %v", err)
	}

	// First use should succeed
	if !ValidateTOTPWithReplay(cache, "user1", secret, code) {
		t.Fatal("first use of valid code should succeed")
	}

	// Second use of same code should be rejected (replay)
	if ValidateTOTPWithReplay(cache, "user1", secret, code) {
		t.Fatal("replay of same code should be rejected")
	}
}

func TestTOTPUsedCodes_NilCacheFallback(t *testing.T) {
	// With nil cache, should just validate without replay check
	secret := "JBSWY3DPEHPK3PXP"
	code, err := GenerateValidCode(secret)
	if err != nil {
		t.Fatalf("failed to generate code: %v", err)
	}

	if !ValidateTOTPWithReplay(nil, "user1", secret, code) {
		t.Fatal("nil cache should still validate valid codes")
	}

	// With nil cache, replay is allowed (backward compat)
	if !ValidateTOTPWithReplay(nil, "user1", secret, code) {
		t.Fatal("nil cache should allow replay (no tracking)")
	}
}

func TestTOTPUsedCodes_Expiry(t *testing.T) {
	cache := NewTOTPUsedCodes()

	// Manually insert an old entry
	cache.mu.Lock()
	cache.codes["user1:111111"] = time.Now().Add(-91 * time.Second)
	cache.mu.Unlock()

	// Trigger cleanup by marking a new code
	cache.MarkUsed("user2", "222222")

	// The old entry should have been cleaned up
	if cache.WasUsed("user1", "111111") {
		t.Fatal("expired entry should have been cleaned up")
	}

	// The new entry should still exist
	if !cache.WasUsed("user2", "222222") {
		t.Fatal("new entry should still exist")
	}
}

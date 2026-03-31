package auth_test

import (
	"testing"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/auth"
)

func TestTokenBlacklist_AddAndCheck(t *testing.T) {
	bl := auth.NewTokenBlacklist()

	bl.Add("jti-1", time.Now().Add(5*time.Minute))

	if !bl.IsBlacklisted("jti-1") {
		t.Fatal("expected jti-1 to be blacklisted")
	}
	if bl.IsBlacklisted("jti-2") {
		t.Fatal("expected jti-2 to not be blacklisted")
	}
}

func TestTokenBlacklist_ExpiredEntryCleaned(t *testing.T) {
	bl := auth.NewTokenBlacklist()

	// Add an already-expired entry
	bl.Add("old-jti", time.Now().Add(-1*time.Minute))

	// Adding a new entry triggers cleanup
	bl.Add("new-jti", time.Now().Add(5*time.Minute))

	if bl.IsBlacklisted("old-jti") {
		t.Fatal("expected expired entry to be cleaned up")
	}
	if !bl.IsBlacklisted("new-jti") {
		t.Fatal("expected new-jti to be blacklisted")
	}
}

func TestTokenBlacklist_Len(t *testing.T) {
	bl := auth.NewTokenBlacklist()

	if bl.Len() != 0 {
		t.Fatal("expected empty blacklist")
	}

	bl.Add("jti-1", time.Now().Add(5*time.Minute))
	bl.Add("jti-2", time.Now().Add(5*time.Minute))

	if bl.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", bl.Len())
	}
}

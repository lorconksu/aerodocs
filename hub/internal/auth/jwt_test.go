package auth_test

import (
	"testing"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/auth"
)

func TestGenerateAndValidateAccessToken(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"

	access, _, err := auth.GenerateTokenPair(secret, "user-1", "admin", 0)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	claims, err := auth.ValidateToken(secret, access)
	if err != nil {
		t.Fatalf("validate access: %v", err)
	}

	if claims.Subject != "user-1" {
		t.Fatalf("expected sub 'user-1', got '%s'", claims.Subject)
	}
	if claims.Role != "admin" {
		t.Fatalf("expected role 'admin', got '%s'", claims.Role)
	}
	if claims.TokenType != auth.TokenTypeAccess {
		t.Fatalf("expected type 'access', got '%s'", claims.TokenType)
	}
}

func TestValidateRefreshToken(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"

	_, refresh, _ := auth.GenerateTokenPair(secret, "user-1", "admin", 0)

	claims, err := auth.ValidateToken(secret, refresh)
	if err != nil {
		t.Fatalf("validate refresh: %v", err)
	}
	if claims.TokenType != auth.TokenTypeRefresh {
		t.Fatalf("expected type 'refresh', got '%s'", claims.TokenType)
	}
}

func TestGenerateSetupToken(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"

	token, err := auth.GenerateSetupToken(secret, "user-1", "admin")
	if err != nil {
		t.Fatalf("generate setup: %v", err)
	}

	claims, err := auth.ValidateToken(secret, token)
	if err != nil {
		t.Fatalf("validate setup: %v", err)
	}
	if claims.TokenType != auth.TokenTypeSetup {
		t.Fatalf("expected type 'setup', got '%s'", claims.TokenType)
	}
}

func TestGenerateTOTPToken(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"

	token, err := auth.GenerateTOTPToken(secret, "user-1", "admin")
	if err != nil {
		t.Fatalf("generate totp: %v", err)
	}

	claims, err := auth.ValidateToken(secret, token)
	if err != nil {
		t.Fatalf("validate totp: %v", err)
	}
	if claims.TokenType != auth.TokenTypeTOTP {
		t.Fatalf("expected type 'totp', got '%s'", claims.TokenType)
	}
}

func TestExpiredToken(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"

	// Create a token that's already expired
	token, _ := auth.GenerateTokenWithExpiry(secret, "user-1", "admin", auth.TokenTypeAccess, -1*time.Minute)

	_, err := auth.ValidateToken(secret, token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestWrongSecret(t *testing.T) {
	access, _, _ := auth.GenerateTokenPair("correct-secret-key-256-bits!!!!", "user-1", "admin", 0)

	_, err := auth.ValidateToken("wrong-secret-key-256-bits-long!", access)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

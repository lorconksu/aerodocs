package auth_test

import (
	"testing"

	"github.com/pquerna/otp/totp"
	"github.com/wyiu/aerodocs/hub/internal/auth"
)

func TestGenerateTOTPSecret(t *testing.T) {
	key, err := auth.GenerateTOTPSecret("admin", "AeroDocs")
	if err != nil {
		t.Fatalf("generate secret: %v", err)
	}

	if key.Secret() == "" {
		t.Fatal("secret should not be empty")
	}
	if key.URL() == "" {
		t.Fatal("URL should not be empty")
	}
}

func TestValidateTOTPCode(t *testing.T) {
	key, _ := auth.GenerateTOTPSecret("admin", "AeroDocs")

	// Generate a valid code
	code, err := totp.GenerateCode(key.Secret(), auth.TOTPTime())
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}

	if !auth.ValidateTOTPCode(key.Secret(), code) {
		t.Fatal("valid code should pass validation")
	}

	if auth.ValidateTOTPCode(key.Secret(), "000000") {
		t.Fatal("invalid code should fail validation")
	}
}

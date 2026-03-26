package auth

import (
	"fmt"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func GenerateTOTPSecret(username, issuer string) (*otp.Key, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: username,
	})
	if err != nil {
		return nil, fmt.Errorf("generate TOTP secret: %w", err)
	}
	return key, nil
}

func ValidateTOTPCode(secret, code string) bool {
	return totp.Validate(code, secret)
}

// ValidateTOTPWithReplay checks the code is valid AND has not been used before.
func ValidateTOTPWithReplay(cache *TOTPUsedCodes, userID, secret, code string) bool {
	if cache != nil && cache.WasUsed(userID, code) {
		return false
	}
	if !totp.Validate(code, secret) {
		return false
	}
	if cache != nil {
		cache.MarkUsed(userID, code)
	}
	return true
}

// GenerateValidCode produces a valid TOTP code for the given secret.
// Used in tests to simulate authenticator app input.
func GenerateValidCode(secret string) (string, error) {
	return totp.GenerateCode(secret, time.Now())
}

func TOTPTime() time.Time {
	return time.Now()
}

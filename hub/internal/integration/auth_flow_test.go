package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/auth"
	"github.com/wyiu/aerodocs/hub/internal/model"
)

func TestFullAuthFlow(t *testing.T) {
	h := StartHarness(t)

	// 1. GET /api/auth/status → {"initialized": false}
	statusResp := h.HTTPGet(t, "/api/auth/status", "")
	defer statusResp.Body.Close()
	if statusResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(statusResp.Body)
		t.Fatalf("auth status: status=%d body=%s", statusResp.StatusCode, body)
	}
	var statusResult model.AuthStatusResponse
	if err := json.NewDecoder(statusResp.Body).Decode(&statusResult); err != nil {
		t.Fatalf("decode auth status: %v", err)
	}
	if statusResult.Initialized {
		t.Fatal("expected initialized=false before any users exist")
	}
	t.Log("auth status: initialized=false ✓")

	// 2. POST /api/auth/register → get setup_token
	const (
		username = "testadmin"
		email    = "testadmin@example.com"
		password = "SecurePass123!"
	)
	regResp := h.HTTPPost(t, "/api/auth/register", model.RegisterRequest{
		Username: username,
		Email:    email,
		Password: password,
	}, "")
	defer regResp.Body.Close()
	if regResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(regResp.Body)
		t.Fatalf("register: status=%d body=%s", regResp.StatusCode, body)
	}
	var regResult struct {
		SetupToken string `json:"setup_token"`
	}
	if err := json.NewDecoder(regResp.Body).Decode(&regResult); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	if regResult.SetupToken == "" {
		t.Fatal("expected non-empty setup_token after register")
	}
	t.Log("register: got setup_token ✓")

	// 3. POST /api/auth/totp/setup (Bearer setup_token) → get secret, qr_url
	totpSetupResp := h.HTTPPost(t, "/api/auth/totp/setup", nil, regResult.SetupToken)
	defer totpSetupResp.Body.Close()
	if totpSetupResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(totpSetupResp.Body)
		t.Fatalf("totp setup: status=%d body=%s", totpSetupResp.StatusCode, body)
	}
	var totpSetupResult model.TOTPSetupResponse
	if err := json.NewDecoder(totpSetupResp.Body).Decode(&totpSetupResult); err != nil {
		t.Fatalf("decode totp setup response: %v", err)
	}
	if totpSetupResult.Secret == "" {
		t.Fatal("expected non-empty TOTP secret")
	}
	if totpSetupResult.QRURL == "" {
		t.Fatal("expected non-empty TOTP QR URL")
	}
	t.Logf("totp setup: got secret and qr_url ✓")

	// 4. Generate TOTP code and POST /api/auth/totp/enable → get access/refresh tokens
	enableCode, err := auth.GenerateValidCode(totpSetupResult.Secret)
	if err != nil {
		t.Fatalf("generate TOTP enable code: %v", err)
	}
	enableResp := h.HTTPPost(t, "/api/auth/totp/enable", model.TOTPEnableRequest{Code: enableCode}, regResult.SetupToken)
	defer enableResp.Body.Close()
	if enableResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(enableResp.Body)
		t.Fatalf("totp enable: status=%d body=%s", enableResp.StatusCode, body)
	}
	var authResult model.AuthResponse
	if err := json.NewDecoder(enableResp.Body).Decode(&authResult); err != nil {
		t.Fatalf("decode totp enable response: %v", err)
	}
	if authResult.AccessToken == "" || authResult.RefreshToken == "" {
		t.Fatal("expected access_token and refresh_token after totp enable")
	}
	accessToken := authResult.AccessToken
	refreshToken := authResult.RefreshToken
	t.Log("totp enable: got access_token and refresh_token ✓")

	// 5. GET /api/auth/status → {"initialized": true}
	statusResp2 := h.HTTPGet(t, "/api/auth/status", "")
	defer statusResp2.Body.Close()
	var statusResult2 model.AuthStatusResponse
	json.NewDecoder(statusResp2.Body).Decode(&statusResult2)
	if !statusResult2.Initialized {
		t.Fatal("expected initialized=true after user created")
	}
	t.Log("auth status: initialized=true ✓")

	// 6. POST /api/auth/login with credentials → get totp_token
	loginResp := h.HTTPPost(t, "/api/auth/login", model.LoginRequest{
		Username: username,
		Password: password,
	}, "")
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(loginResp.Body)
		t.Fatalf("login: status=%d body=%s (expected 202)", loginResp.StatusCode, body)
	}
	var loginResult model.LoginResponse
	if err := json.NewDecoder(loginResp.Body).Decode(&loginResult); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if loginResult.TOTPToken == "" {
		t.Fatal("expected totp_token after login (TOTP enabled)")
	}
	t.Log("login: got totp_token ✓")

	// 7. Generate TOTP code and POST /api/auth/login/totp → get access/refresh tokens
	// Clear the replay cache so the code (already used during enable) can be reused
	h.HTTPServer.ClearTOTPCache()
	loginTOTPCode, err := auth.GenerateValidCode(totpSetupResult.Secret)
	if err != nil {
		t.Fatalf("generate TOTP login code: %v", err)
	}
	loginTOTPResp := h.HTTPPost(t, "/api/auth/login/totp", model.LoginTOTPRequest{
		TOTPToken: loginResult.TOTPToken,
		Code:      loginTOTPCode,
	}, "")
	defer loginTOTPResp.Body.Close()
	if loginTOTPResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(loginTOTPResp.Body)
		t.Fatalf("login/totp: status=%d body=%s", loginTOTPResp.StatusCode, body)
	}
	var loginTOTPResult model.AuthResponse
	if err := json.NewDecoder(loginTOTPResp.Body).Decode(&loginTOTPResult); err != nil {
		t.Fatalf("decode login/totp response: %v", err)
	}
	if loginTOTPResult.AccessToken == "" {
		t.Fatal("expected access_token after login/totp")
	}
	accessToken = loginTOTPResult.AccessToken
	refreshToken = loginTOTPResult.RefreshToken
	t.Log("login/totp: got access_token and refresh_token ✓")

	// 8. GET /api/auth/me → verify username
	meResp := h.HTTPGet(t, "/api/auth/me", accessToken)
	defer meResp.Body.Close()
	if meResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(meResp.Body)
		t.Fatalf("me: status=%d body=%s", meResp.StatusCode, body)
	}
	var meResult model.User
	if err := json.NewDecoder(meResp.Body).Decode(&meResult); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if meResult.Username != username {
		t.Fatalf("username mismatch: got %q, want %q", meResult.Username, username)
	}
	if meResult.Role != model.RoleAdmin {
		t.Fatalf("role mismatch: got %q, want %q", meResult.Role, model.RoleAdmin)
	}
	t.Logf("me: username=%s role=%s ✓", meResult.Username, meResult.Role)

	// 9. PUT /api/auth/password → change password
	const newPassword = "NewSecurePass456!"
	pwResp := h.HTTPPut(t, "/api/auth/password", model.ChangePasswordRequest{
		CurrentPassword: password,
		NewPassword:     newPassword,
	}, accessToken)
	defer pwResp.Body.Close()
	if pwResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(pwResp.Body)
		t.Fatalf("change password: status=%d body=%s", pwResp.StatusCode, body)
	}
	t.Log("change password: 200 OK ✓")

	// 10. POST /api/auth/login with NEW password → should succeed (get totp_token)
	loginNewResp := h.HTTPPost(t, "/api/auth/login", model.LoginRequest{
		Username: username,
		Password: newPassword,
	}, "")
	defer loginNewResp.Body.Close()
	if loginNewResp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(loginNewResp.Body)
		t.Fatalf("login with new password: status=%d body=%s (expected 202)", loginNewResp.StatusCode, body)
	}
	t.Log("login with new password: 202 Accepted ✓")

	// 11. POST /api/auth/refresh with refresh_token → get new token pair
	refreshResp := h.HTTPPost(t, "/api/auth/refresh", model.RefreshRequest{
		RefreshToken: refreshToken,
	}, "")
	defer refreshResp.Body.Close()
	if refreshResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(refreshResp.Body)
		t.Fatalf("refresh: status=%d body=%s", refreshResp.StatusCode, body)
	}
	var refreshResult model.TokenPair
	if err := json.NewDecoder(refreshResp.Body).Decode(&refreshResult); err != nil {
		t.Fatalf("decode refresh response: %v", err)
	}
	if refreshResult.AccessToken == "" || refreshResult.RefreshToken == "" {
		t.Fatal("expected new access_token and refresh_token after refresh")
	}
	t.Log("refresh: got new token pair ✓")

	t.Log("full auth flow completed successfully")
}

package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/auth"
	"github.com/wyiu/aerodocs/hub/internal/model"
	"github.com/wyiu/aerodocs/hub/internal/store"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	jwtSecret, err := InitJWTSecret(st)
	if err != nil {
		t.Fatalf("init jwt secret: %v", err)
	}

	return New(Config{
		Addr:      ":0",
		Store:     st,
		JWTSecret: jwtSecret,
		IsDev:     true,
	})
}

func TestAuthStatus_NotInitialized(t *testing.T) {
	s := testServer(t)

	req := httptest.NewRequest("GET", "/api/auth/status", nil)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp model.AuthStatusResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Initialized {
		t.Fatal("expected initialized=false")
	}
}

func TestRegisterFirstUser(t *testing.T) {
	s := testServer(t)

	body, _ := json.Marshal(model.RegisterRequest{
		Username: "admin",
		Email:    "admin@test.com",
		Password: "MyP@ssw0rd!234",
	})

	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["setup_token"] == nil {
		t.Fatal("expected setup_token in response")
	}
}

func TestRegisterBlocked_AfterFirstUser(t *testing.T) {
	s := testServer(t)

	// Register first user
	body, _ := json.Marshal(model.RegisterRequest{
		Username: "admin", Email: "admin@test.com", Password: "MyP@ssw0rd!234",
	})
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	// Try to register again
	body2, _ := json.Marshal(model.RegisterRequest{
		Username: "hacker", Email: "hacker@test.com", Password: "MyP@ssw0rd!234",
	})
	req2 := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body2))
	rec2 := httptest.NewRecorder()
	s.routes().ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec2.Code)
	}
}

func TestChangePassword_Success(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body, _ := json.Marshal(model.ChangePasswordRequest{
		CurrentPassword: "MyP@ssw0rd!234",
		NewPassword:     "NewP@ssw0rd!567",
	})
	req := httptest.NewRequest("PUT", "/api/auth/password", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["status"] != "password updated" {
		t.Fatalf("expected status 'password updated', got '%s'", resp["status"])
	}
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body, _ := json.Marshal(model.ChangePasswordRequest{
		CurrentPassword: "WrongP@ssword!1",
		NewPassword:     "NewP@ssw0rd!567",
	})
	req := httptest.NewRequest("PUT", "/api/auth/password", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangePassword_PolicyViolation(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body, _ := json.Marshal(model.ChangePasswordRequest{
		CurrentPassword: "MyP@ssw0rd!234",
		NewPassword:     "short",
	})
	req := httptest.NewRequest("PUT", "/api/auth/password", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	s := testServer(t)

	body, _ := json.Marshal(model.LoginRequest{
		Username: "nobody", Password: "wrong",
	})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestLogin_Success_NeedsTOTPSetup(t *testing.T) {
	s := testServer(t)

	// Register first user — get setup token
	regBody, _ := json.Marshal(model.RegisterRequest{
		Username: "admin", Email: "admin@test.com", Password: "MyP@ssw0rd!234",
	})
	regReq := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(regBody))
	regRec := httptest.NewRecorder()
	s.routes().ServeHTTP(regRec, regReq)

	// Login with correct credentials (TOTP not yet set up)
	body, _ := json.Marshal(model.LoginRequest{
		Username: "admin", Password: "MyP@ssw0rd!234",
	})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.LoginResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if !resp.RequiresTOTPSetup {
		t.Fatal("expected requires_totp_setup=true")
	}
	if resp.SetupToken == "" {
		t.Fatal("expected setup_token in response")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	s := testServer(t)

	// Register user first
	regBody, _ := json.Marshal(model.RegisterRequest{
		Username: "admin", Email: "admin@test.com", Password: "MyP@ssw0rd!234",
	})
	regReq := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(regBody))
	regRec := httptest.NewRecorder()
	s.routes().ServeHTTP(regRec, regReq)

	body, _ := json.Marshal(model.LoginRequest{
		Username: "admin", Password: "WrongPassword!999",
	})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestLogin_WithTOTPEnabled(t *testing.T) {
	s := testServer(t)
	_ = registerAndGetAdminToken(t, s)

	// Now try logging in again — should return TOTP token
	body, _ := json.Marshal(model.LoginRequest{
		Username: "admin", Password: "MyP@ssw0rd!234",
	})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.LoginResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.TOTPToken == "" {
		t.Fatal("expected totp_token in response")
	}
}

func TestLogin_InvalidJSON(t *testing.T) {
	s := testServer(t)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader([]byte("not-json")))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestLoginTOTP_Success(t *testing.T) {
	s := testServer(t)
	_ = registerAndGetAdminToken(t, s)

	// Login to get TOTP token
	loginBody, _ := json.Marshal(model.LoginRequest{
		Username: "admin", Password: "MyP@ssw0rd!234",
	})
	loginReq := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(loginBody))
	loginRec := httptest.NewRecorder()
	s.routes().ServeHTTP(loginRec, loginReq)

	var loginResp model.LoginResponse
	json.NewDecoder(loginRec.Body).Decode(&loginResp)

	// Get user's TOTP secret from store
	user, err := s.store.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	code, _ := auth.GenerateValidCode(*user.TOTPSecret)

	// Login with TOTP
	totpBody, _ := json.Marshal(model.LoginTOTPRequest{
		TOTPToken: loginResp.TOTPToken,
		Code:      code,
	})
	totpReq := httptest.NewRequest("POST", "/api/auth/login/totp", bytes.NewReader(totpBody))
	totpRec := httptest.NewRecorder()
	s.routes().ServeHTTP(totpRec, totpReq)

	if totpRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", totpRec.Code, totpRec.Body.String())
	}

	var authResp model.AuthResponse
	json.NewDecoder(totpRec.Body).Decode(&authResp)
	if authResp.AccessToken == "" {
		t.Fatal("expected access_token in response")
	}
}

func TestLoginTOTP_InvalidCode(t *testing.T) {
	s := testServer(t)
	_ = registerAndGetAdminToken(t, s)

	// Login to get TOTP token
	loginBody, _ := json.Marshal(model.LoginRequest{
		Username: "admin", Password: "MyP@ssw0rd!234",
	})
	loginReq := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(loginBody))
	loginRec := httptest.NewRecorder()
	s.routes().ServeHTTP(loginRec, loginReq)

	var loginResp model.LoginResponse
	json.NewDecoder(loginRec.Body).Decode(&loginResp)

	// Use wrong code
	totpBody, _ := json.Marshal(model.LoginTOTPRequest{
		TOTPToken: loginResp.TOTPToken,
		Code:      "000000",
	})
	totpReq := httptest.NewRequest("POST", "/api/auth/login/totp", bytes.NewReader(totpBody))
	totpRec := httptest.NewRecorder()
	s.routes().ServeHTTP(totpRec, totpReq)

	if totpRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", totpRec.Code, totpRec.Body.String())
	}
}

func TestLoginTOTP_InvalidTOTPToken(t *testing.T) {
	s := testServer(t)

	body, _ := json.Marshal(model.LoginTOTPRequest{
		TOTPToken: "invalid-token",
		Code:      "123456",
	})
	req := httptest.NewRequest("POST", "/api/auth/login/totp", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestLoginTOTP_InvalidJSON(t *testing.T) {
	s := testServer(t)

	req := httptest.NewRequest("POST", "/api/auth/login/totp", bytes.NewReader([]byte("not-json")))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestRefresh_ValidToken(t *testing.T) {
	s := testServer(t)

	// We need a refresh token — use registerAndGetAdminToken and call login
	_ = registerAndGetAdminToken(t, s)

	// Get refresh token via TOTP login flow
	loginBody, _ := json.Marshal(model.LoginRequest{
		Username: "admin", Password: "MyP@ssw0rd!234",
	})
	loginReq := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(loginBody))
	loginRec := httptest.NewRecorder()
	s.routes().ServeHTTP(loginRec, loginReq)

	var loginResp model.LoginResponse
	json.NewDecoder(loginRec.Body).Decode(&loginResp)

	user, _ := s.store.GetUserByUsername("admin")
	code, _ := auth.GenerateValidCode(*user.TOTPSecret)

	totpBody, _ := json.Marshal(model.LoginTOTPRequest{
		TOTPToken: loginResp.TOTPToken,
		Code:      code,
	})
	totpReq := httptest.NewRequest("POST", "/api/auth/login/totp", bytes.NewReader(totpBody))
	totpRec := httptest.NewRecorder()
	s.routes().ServeHTTP(totpRec, totpReq)

	var authResp model.AuthResponse
	json.NewDecoder(totpRec.Body).Decode(&authResp)

	// Now refresh
	refreshBody, _ := json.Marshal(model.RefreshRequest{
		RefreshToken: authResp.RefreshToken,
	})
	refreshReq := httptest.NewRequest("POST", "/api/auth/refresh", bytes.NewReader(refreshBody))
	refreshRec := httptest.NewRecorder()
	s.routes().ServeHTTP(refreshRec, refreshReq)

	if refreshRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", refreshRec.Code, refreshRec.Body.String())
	}

	var tokenPair model.TokenPair
	json.NewDecoder(refreshRec.Body).Decode(&tokenPair)
	if tokenPair.AccessToken == "" {
		t.Fatal("expected access_token in response")
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	s := testServer(t)

	body, _ := json.Marshal(model.RefreshRequest{
		RefreshToken: "invalid-token",
	})
	req := httptest.NewRequest("POST", "/api/auth/refresh", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRefresh_AccessTokenAsRefresh(t *testing.T) {
	s := testServer(t)
	accessToken := registerAndGetAdminToken(t, s)

	// Use access token where refresh token is expected
	body, _ := json.Marshal(model.RefreshRequest{
		RefreshToken: accessToken,
	})
	req := httptest.NewRequest("POST", "/api/auth/refresh", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRefresh_InvalidJSON(t *testing.T) {
	s := testServer(t)

	req := httptest.NewRequest("POST", "/api/auth/refresh", bytes.NewReader([]byte("not-json")))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTOTPSetup_Success(t *testing.T) {
	s := testServer(t)

	// Register to get setup token
	regBody, _ := json.Marshal(model.RegisterRequest{
		Username: "admin", Email: "admin@test.com", Password: "MyP@ssw0rd!234",
	})
	regReq := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(regBody))
	regRec := httptest.NewRecorder()
	s.routes().ServeHTTP(regRec, regReq)

	var regResp map[string]interface{}
	json.NewDecoder(regRec.Body).Decode(&regResp)
	setupToken := regResp["setup_token"].(string)

	req := httptest.NewRequest("POST", "/api/auth/totp/setup", nil)
	req.Header.Set("Authorization", "Bearer "+setupToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.TOTPSetupResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Secret == "" {
		t.Fatal("expected secret in response")
	}
	if resp.QRURL == "" {
		t.Fatal("expected qr_url in response")
	}
}

func TestTOTPEnable_Success(t *testing.T) {
	s := testServer(t)

	// Register and setup TOTP, then enable
	regBody, _ := json.Marshal(model.RegisterRequest{
		Username: "admin", Email: "admin@test.com", Password: "MyP@ssw0rd!234",
	})
	regReq := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(regBody))
	regRec := httptest.NewRecorder()
	s.routes().ServeHTTP(regRec, regReq)

	var regResp map[string]interface{}
	json.NewDecoder(regRec.Body).Decode(&regResp)
	setupToken := regResp["setup_token"].(string)

	// Setup TOTP
	setupReq := httptest.NewRequest("POST", "/api/auth/totp/setup", nil)
	setupReq.Header.Set("Authorization", "Bearer "+setupToken)
	setupRec := httptest.NewRecorder()
	s.routes().ServeHTTP(setupRec, setupReq)

	var totpResp model.TOTPSetupResponse
	json.NewDecoder(setupRec.Body).Decode(&totpResp)

	// Enable TOTP with valid code
	code, _ := auth.GenerateValidCode(totpResp.Secret)
	enableBody, _ := json.Marshal(model.TOTPEnableRequest{Code: code})
	enableReq := httptest.NewRequest("POST", "/api/auth/totp/enable", bytes.NewReader(enableBody))
	enableReq.Header.Set("Authorization", "Bearer "+setupToken)
	enableRec := httptest.NewRecorder()
	s.routes().ServeHTTP(enableRec, enableReq)

	if enableRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", enableRec.Code, enableRec.Body.String())
	}

	var authResp model.AuthResponse
	json.NewDecoder(enableRec.Body).Decode(&authResp)
	if authResp.AccessToken == "" {
		t.Fatal("expected access_token in response")
	}
	if !authResp.User.TOTPEnabled {
		t.Fatal("expected totp_enabled=true")
	}
}

func TestTOTPEnable_InvalidCode(t *testing.T) {
	s := testServer(t)

	regBody, _ := json.Marshal(model.RegisterRequest{
		Username: "admin", Email: "admin@test.com", Password: "MyP@ssw0rd!234",
	})
	regReq := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(regBody))
	regRec := httptest.NewRecorder()
	s.routes().ServeHTTP(regRec, regReq)

	var regResp map[string]interface{}
	json.NewDecoder(regRec.Body).Decode(&regResp)
	setupToken := regResp["setup_token"].(string)

	setupReq := httptest.NewRequest("POST", "/api/auth/totp/setup", nil)
	setupReq.Header.Set("Authorization", "Bearer "+setupToken)
	setupRec := httptest.NewRecorder()
	s.routes().ServeHTTP(setupRec, setupReq)

	enableBody, _ := json.Marshal(model.TOTPEnableRequest{Code: "000000"})
	enableReq := httptest.NewRequest("POST", "/api/auth/totp/enable", bytes.NewReader(enableBody))
	enableReq.Header.Set("Authorization", "Bearer "+setupToken)
	enableRec := httptest.NewRecorder()
	s.routes().ServeHTTP(enableRec, enableReq)

	if enableRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", enableRec.Code, enableRec.Body.String())
	}
}

func TestTOTPEnable_NotSetUp(t *testing.T) {
	s := testServer(t)

	regBody, _ := json.Marshal(model.RegisterRequest{
		Username: "admin", Email: "admin@test.com", Password: "MyP@ssw0rd!234",
	})
	regReq := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(regBody))
	regRec := httptest.NewRecorder()
	s.routes().ServeHTTP(regRec, regReq)

	var regResp map[string]interface{}
	json.NewDecoder(regRec.Body).Decode(&regResp)
	setupToken := regResp["setup_token"].(string)

	// Try to enable without calling setup first
	enableBody, _ := json.Marshal(model.TOTPEnableRequest{Code: "123456"})
	enableReq := httptest.NewRequest("POST", "/api/auth/totp/enable", bytes.NewReader(enableBody))
	enableReq.Header.Set("Authorization", "Bearer "+setupToken)
	enableRec := httptest.NewRecorder()
	s.routes().ServeHTTP(enableRec, enableReq)

	if enableRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", enableRec.Code, enableRec.Body.String())
	}
}

func TestTOTPEnable_InvalidJSON(t *testing.T) {
	s := testServer(t)

	// Get a setup token (TOTP enable requires setup token)
	regBody, _ := json.Marshal(model.RegisterRequest{
		Username: "admin", Email: "admin@test.com", Password: "MyP@ssw0rd!234",
	})
	regReq := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(regBody))
	regRec := httptest.NewRecorder()
	s.routes().ServeHTTP(regRec, regReq)

	var regResp map[string]interface{}
	json.NewDecoder(regRec.Body).Decode(&regResp)
	setupToken := regResp["setup_token"].(string)

	req := httptest.NewRequest("POST", "/api/auth/totp/enable", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Authorization", "Bearer "+setupToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTOTPDisable_Success(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)

	// Create viewer user
	viewerToken := createViewerAndGetToken(t, s, adminToken)

	// Get viewer's user id
	meReq := httptest.NewRequest("GET", "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+viewerToken)
	meRec := httptest.NewRecorder()
	s.routes().ServeHTTP(meRec, meReq)

	var viewerUser model.User
	json.NewDecoder(meRec.Body).Decode(&viewerUser)

	// Get admin's TOTP secret
	adminUser, _ := s.store.GetUserByUsername("admin")
	adminCode, _ := auth.GenerateValidCode(*adminUser.TOTPSecret)

	// Disable viewer's TOTP
	disableBody, _ := json.Marshal(model.TOTPDisableRequest{
		UserID:        viewerUser.ID,
		AdminTOTPCode: adminCode,
	})
	disableReq := httptest.NewRequest("POST", "/api/auth/totp/disable", bytes.NewReader(disableBody))
	disableReq.Header.Set("Authorization", "Bearer "+adminToken)
	disableRec := httptest.NewRecorder()
	s.routes().ServeHTTP(disableRec, disableReq)

	if disableRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", disableRec.Code, disableRec.Body.String())
	}
}

func TestTOTPDisable_WrongAdminCode(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)

	viewerToken := createViewerAndGetToken(t, s, adminToken)

	meReq := httptest.NewRequest("GET", "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+viewerToken)
	meRec := httptest.NewRecorder()
	s.routes().ServeHTTP(meRec, meReq)

	var viewerUser model.User
	json.NewDecoder(meRec.Body).Decode(&viewerUser)

	disableBody, _ := json.Marshal(model.TOTPDisableRequest{
		UserID:        viewerUser.ID,
		AdminTOTPCode: "000000",
	})
	disableReq := httptest.NewRequest("POST", "/api/auth/totp/disable", bytes.NewReader(disableBody))
	disableReq.Header.Set("Authorization", "Bearer "+adminToken)
	disableRec := httptest.NewRecorder()
	s.routes().ServeHTTP(disableRec, disableReq)

	if disableRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", disableRec.Code, disableRec.Body.String())
	}
}

func TestTOTPDisable_InvalidJSON(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("POST", "/api/auth/totp/disable", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleMe(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var user model.User
	json.NewDecoder(rec.Body).Decode(&user)
	if user.Username != "admin" {
		t.Fatalf("expected username 'admin', got '%s'", user.Username)
	}
}

func TestUpdateAvatar_Success(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body, _ := json.Marshal(model.UpdateAvatarRequest{
		Avatar: "data:image/png;base64,abc123",
	})
	req := httptest.NewRequest("PUT", "/api/auth/avatar", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateAvatar_TooLarge(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// Generate a large avatar string (>700KB)
	largeAvatar := make([]byte, 700001)
	for i := range largeAvatar {
		largeAvatar[i] = 'a'
	}

	body, _ := json.Marshal(model.UpdateAvatarRequest{
		Avatar: string(largeAvatar),
	})
	req := httptest.NewRequest("PUT", "/api/auth/avatar", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateAvatar_InvalidJSON(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("PUT", "/api/auth/avatar", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestRegister_InvalidJSON(t *testing.T) {
	s := testServer(t)

	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader([]byte("not-json")))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestRegister_InvalidUsername(t *testing.T) {
	s := testServer(t)

	body, _ := json.Marshal(model.RegisterRequest{
		Username: "ab", // too short
		Email:    "test@test.com",
		Password: "MyP@ssw0rd!234",
	})
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestRegister_WeakPassword(t *testing.T) {
	s := testServer(t)

	body, _ := json.Marshal(model.RegisterRequest{
		Username: "admin",
		Email:    "test@test.com",
		Password: "weak",
	})
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAuthStatus_Initialized(t *testing.T) {
	s := testServer(t)

	// Register a user first
	body, _ := json.Marshal(model.RegisterRequest{
		Username: "admin", Email: "admin@test.com", Password: "MyP@ssw0rd!234",
	})
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	// Check status
	statusReq := httptest.NewRequest("GET", "/api/auth/status", nil)
	statusRec := httptest.NewRecorder()
	s.routes().ServeHTTP(statusRec, statusReq)

	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", statusRec.Code)
	}

	var resp model.AuthStatusResponse
	json.NewDecoder(statusRec.Body).Decode(&resp)
	if !resp.Initialized {
		t.Fatal("expected initialized=true after registration")
	}
}

func TestLoginTOTP_WrongTokenType(t *testing.T) {
	s := testServer(t)
	accessToken := registerAndGetAdminToken(t, s)

	// Use access token as totp_token (wrong type)
	body, _ := json.Marshal(model.LoginTOTPRequest{
		TOTPToken: accessToken,
		Code:      "123456",
	})
	req := httptest.NewRequest("POST", "/api/auth/login/totp", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

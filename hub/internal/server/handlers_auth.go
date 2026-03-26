package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/wyiu/aerodocs/hub/internal/auth"
	"github.com/wyiu/aerodocs/hub/internal/model"
)

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	count, err := s.store.UserCount()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check user count")
		return
	}
	respondJSON(w, http.StatusOK, model.AuthStatusResponse{Initialized: count > 0})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	// Only allowed when no users exist
	count, err := s.store.UserCount()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check user count")
		return
	}
	if count > 0 {
		respondError(w, http.StatusForbidden, "registration disabled — use admin to create users")
		return
	}

	var req model.RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, errInvalidRequestBody)
		return
	}

	if err := validateUsername(req.Username); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := auth.ValidatePasswordPolicy(req.Password); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user := &model.User{
		ID:           uuid.NewString(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		Role:         model.RoleAdmin,
	}

	if err := s.store.CreateUser(user); err != nil {
		// Unique constraint violation = another request won the race
		respondError(w, http.StatusConflict, "setup already completed")
		return
	}

	// Recheck: if another registration won the race with a different username, roll back
	count, _ = s.store.UserCount()
	if count > 1 {
		s.store.DeleteUser(user.ID)
		respondError(w, http.StatusConflict, "setup already completed")
		return
	}

	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &user.ID,
		Action: model.AuditUserRegistered, IPAddress: &ip,
	})

	setupToken, err := auth.GenerateSetupToken(s.jwtSecret, user.ID, string(user.Role))
	if err != nil {
		respondError(w, http.StatusInternalServerError, errFailedToGenerateToken)
		return
	}

	respondJSON(w, http.StatusOK, model.SetupResponse{
		SetupToken: setupToken,
		User:       user,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, errInvalidRequestBody)
		return
	}

	user, err := s.store.GetUserByUsername(req.Username)
	if err != nil {
		ip := clientIP(r)
		s.store.LogAudit(model.AuditEntry{
			ID: uuid.NewString(), Action: model.AuditUserLoginFailed, IPAddress: &ip,
		})
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !auth.ComparePassword(user.PasswordHash, req.Password) {
		ip := clientIP(r)
		s.store.LogAudit(model.AuditEntry{
			ID: uuid.NewString(), UserID: &user.ID,
			Action: model.AuditUserLoginFailed, IPAddress: &ip,
		})
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// If TOTP not set up yet, return setup token
	if !user.TOTPEnabled {
		setupToken, err := auth.GenerateSetupToken(s.jwtSecret, user.ID, string(user.Role))
		if err != nil {
			respondError(w, http.StatusInternalServerError, errFailedToGenerateToken)
			return
		}
		respondJSON(w, http.StatusOK, model.LoginResponse{
			SetupToken:        setupToken,
			RequiresTOTPSetup: true,
		})
		return
	}

	// TOTP is enabled — require TOTP code
	totpToken, err := auth.GenerateTOTPToken(s.jwtSecret, user.ID, string(user.Role))
	if err != nil {
		respondError(w, http.StatusInternalServerError, errFailedToGenerateToken)
		return
	}

	respondJSON(w, http.StatusAccepted, model.LoginResponse{
		TOTPToken: totpToken,
	})
}

func (s *Server) handleLoginTOTP(w http.ResponseWriter, r *http.Request) {
	var req model.LoginTOTPRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, errInvalidRequestBody)
		return
	}

	// Validate the TOTP token (proves password was already verified)
	claims, err := auth.ValidateToken(s.jwtSecret, req.TOTPToken)
	if err != nil || claims.TokenType != auth.TokenTypeTOTP {
		respondError(w, http.StatusUnauthorized, "invalid or expired TOTP token")
		return
	}

	user, err := s.store.GetUserByID(claims.Subject)
	if err != nil {
		respondError(w, http.StatusUnauthorized, errUserNotFound)
		return
	}

	if user.TOTPSecret == nil || !auth.ValidateTOTPWithReplay(s.totpCache, user.ID, *user.TOTPSecret, req.Code) {
		ip := clientIP(r)
		s.store.LogAudit(model.AuditEntry{
			ID: uuid.NewString(), UserID: &user.ID,
			Action: model.AuditUserLoginTOTPFailed, IPAddress: &ip,
		})
		respondError(w, http.StatusUnauthorized, "invalid TOTP code")
		return
	}

	accessToken, refreshToken, err := auth.GenerateTokenPair(s.jwtSecret, user.ID, string(user.Role))
	if err != nil {
		respondError(w, http.StatusInternalServerError, errFailedToGenerateTokens)
		return
	}

	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &user.ID,
		Action: model.AuditUserLogin, IPAddress: &ip,
	})

	respondJSON(w, http.StatusOK, model.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         *user,
	})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req model.RefreshRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, errInvalidRequestBody)
		return
	}

	claims, err := auth.ValidateToken(s.jwtSecret, req.RefreshToken)
	if err != nil || claims.TokenType != auth.TokenTypeRefresh {
		respondError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	// Verify user still exists and use current role from DB
	user, err := s.store.GetUserByID(claims.Subject)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "user no longer exists")
		return
	}

	// Use user.Role from DB, not claims.Role from the old token
	accessToken, refreshToken, err := auth.GenerateTokenPair(s.jwtSecret, user.ID, string(user.Role))
	if err != nil {
		respondError(w, http.StatusInternalServerError, errFailedToGenerateTokens)
		return
	}

	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID:        uuid.NewString(),
		UserID:    &user.ID,
		Action:    "user.token_refreshed",
		IPAddress: &ip,
	})

	respondJSON(w, http.StatusOK, model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	user, err := s.store.GetUserByID(userID)
	if err != nil {
		respondError(w, http.StatusNotFound, errUserNotFound)
		return
	}
	respondJSON(w, http.StatusOK, user)
}

func (s *Server) handleUpdateAvatar(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateAvatarRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, errInvalidRequestBody)
		return
	}

	// Validate the avatar is a reasonable data URL (max ~500KB base64)
	if len(req.Avatar) > 700000 {
		respondError(w, http.StatusBadRequest, "avatar image too large (max 500KB)")
		return
	}

	if req.Avatar != "" && !strings.HasPrefix(req.Avatar, "data:image/") {
		respondError(w, http.StatusBadRequest, "avatar must be a data:image/ URL")
		return
	}

	userID := UserIDFromContext(r.Context())
	var avatar *string
	if req.Avatar != "" {
		avatar = &req.Avatar
	}

	if err := s.store.UpdateUserAvatar(userID, avatar); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update avatar")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "avatar updated"})
}

func (s *Server) handleTOTPSetup(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	user, err := s.store.GetUserByID(userID)
	if err != nil {
		respondError(w, http.StatusNotFound, errUserNotFound)
		return
	}
	if user.TOTPEnabled {
		respondError(w, http.StatusConflict, "TOTP is already enabled")
		return
	}

	key, err := auth.GenerateTOTPSecret(user.Username, "AeroDocs")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate TOTP secret")
		return
	}

	// Store secret (not yet enabled)
	secret := key.Secret()
	if err := s.store.UpdateUserTOTP(userID, &secret, false); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to store TOTP secret")
		return
	}

	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &userID,
		Action: model.AuditUserTOTPSetup, IPAddress: &ip,
	})

	respondJSON(w, http.StatusOK, model.TOTPSetupResponse{
		Secret: key.Secret(),
		QRURL:  key.URL(),
	})
}

func (s *Server) handleTOTPEnable(w http.ResponseWriter, r *http.Request) {
	var req model.TOTPEnableRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, errInvalidRequestBody)
		return
	}

	userID := UserIDFromContext(r.Context())
	user, err := s.store.GetUserByID(userID)
	if err != nil {
		respondError(w, http.StatusNotFound, errUserNotFound)
		return
	}

	if user.TOTPSecret == nil {
		respondError(w, http.StatusBadRequest, "TOTP not set up — call /api/auth/totp/setup first")
		return
	}

	if !auth.ValidateTOTPWithReplay(s.totpCache, userID, *user.TOTPSecret, req.Code) {
		respondError(w, http.StatusUnauthorized, "invalid TOTP code")
		return
	}

	if err := s.store.UpdateUserTOTP(userID, user.TOTPSecret, true); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to enable TOTP")
		return
	}

	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &userID,
		Action: model.AuditUserTOTPEnabled, IPAddress: &ip,
	})

	// Generate full access tokens now that 2FA is enabled
	accessToken, refreshToken, err := auth.GenerateTokenPair(s.jwtSecret, user.ID, string(user.Role))
	if err != nil {
		respondError(w, http.StatusInternalServerError, errFailedToGenerateTokens)
		return
	}

	// Refresh user to get updated totp_enabled
	user, _ = s.store.GetUserByID(userID)

	respondJSON(w, http.StatusOK, model.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         *user,
	})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req model.ChangePasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, errInvalidRequestBody)
		return
	}

	userID := UserIDFromContext(r.Context())
	user, err := s.store.GetUserByID(userID)
	if err != nil {
		respondError(w, http.StatusNotFound, errUserNotFound)
		return
	}

	if !auth.ComparePassword(user.PasswordHash, req.CurrentPassword) {
		respondError(w, http.StatusUnauthorized, "invalid current password")
		return
	}

	if err := auth.ValidatePasswordPolicy(req.NewPassword); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	if err := s.store.UpdateUserPassword(userID, hash); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &userID,
		Action: model.AuditUserPasswordChanged, IPAddress: &ip,
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": "password updated"})
}

func (s *Server) handleTOTPDisable(w http.ResponseWriter, r *http.Request) {
	var req model.TOTPDisableRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, errInvalidRequestBody)
		return
	}

	adminID := UserIDFromContext(r.Context())
	if req.UserID == adminID {
		respondError(w, http.StatusBadRequest, "cannot disable your own 2FA")
		return
	}

	// Verify admin's own TOTP code
	admin, err := s.store.GetUserByID(adminID)
	if err != nil {
		respondError(w, http.StatusNotFound, "admin user not found")
		return
	}

	if admin.TOTPSecret == nil || !auth.ValidateTOTPWithReplay(s.totpCache, adminID, *admin.TOTPSecret, req.AdminTOTPCode) {
		respondError(w, http.StatusUnauthorized, "invalid admin TOTP code")
		return
	}

	// Verify target user exists
	targetUser, err := s.store.GetUserByID(req.UserID)
	if err != nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}
	_ = targetUser // used for existence check

	// Disable target user's TOTP
	if err := s.store.UpdateUserTOTP(req.UserID, nil, false); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to disable TOTP")
		return
	}

	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &adminID,
		Action: model.AuditUserTOTPDisabled, Target: &req.UserID, IPAddress: &ip,
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": "totp disabled"})
}

func validateUsername(username string) error {
	if len(username) < 3 || len(username) > 32 {
		return fmt.Errorf("username must be 3-32 characters")
	}
	for _, r := range username {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return fmt.Errorf("username may only contain alphanumeric characters and underscores")
		}
	}
	return nil
}

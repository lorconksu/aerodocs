package model

import "time"

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleViewer Role = "viewer"
)

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         Role      `json:"role"`
	TOTPSecret   *string   `json:"-"`
	TOTPEnabled  bool      `json:"totp_enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CreateUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     Role   `json:"role"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginTOTPRequest struct {
	TOTPToken string `json:"totp_token"`
	Code      string `json:"code"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type TOTPEnableRequest struct {
	Code string `json:"code"`
}

type TOTPDisableRequest struct {
	UserID        string `json:"user_id"`
	AdminTOTPCode string `json:"admin_totp_code"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type UpdateRoleRequest struct {
	Role Role `json:"role"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type AuthStatusResponse struct {
	Initialized bool `json:"initialized"`
}

type LoginResponse struct {
	TOTPToken         string `json:"totp_token,omitempty"`
	SetupToken        string `json:"setup_token,omitempty"`
	RequiresTOTPSetup bool   `json:"requires_totp_setup,omitempty"`
}

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

type TOTPSetupResponse struct {
	Secret string `json:"secret"`
	QRURL  string `json:"qr_url"`
}

type CreateUserResponse struct {
	User              User   `json:"user"`
	TemporaryPassword string `json:"temporary_password"`
}

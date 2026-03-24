package model

import "time"

type AuditEntry struct {
	ID        string    `json:"id"`
	UserID    *string   `json:"user_id"`
	Action    string    `json:"action"`
	Target    *string   `json:"target"`
	Detail    *string   `json:"detail"`
	IPAddress *string   `json:"ip_address"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditFilter struct {
	UserID *string
	Action *string
	From   *string // ISO 8601 datetime
	To     *string // ISO 8601 datetime
	Limit  int
	Offset int
}

// Audit action constants
const (
	AuditUserLogin           = "user.login"
	AuditUserLoginFailed     = "user.login_failed"
	AuditUserLoginTOTPFailed = "user.login_totp_failed"
	AuditUserRegistered      = "user.registered"
	AuditUserTOTPSetup       = "user.totp_setup"
	AuditUserTOTPEnabled     = "user.totp_enabled"
	AuditUserTOTPDisabled    = "user.totp_disabled"
	AuditUserCreated         = "user.created"
	AuditUserTOTPReset       = "user.totp_reset"
	AuditUserPasswordChanged = "user.password_changed"
	AuditUserRoleUpdated     = "user.role_updated"
	AuditUserDeleted         = "user.deleted"
)

const (
	AuditServerCreated      = "server.created"
	AuditServerUpdated      = "server.updated"
	AuditServerDeleted      = "server.deleted"
	AuditServerBatchDeleted = "server.batch_deleted"
	AuditServerRegistered   = "server.registered"
	AuditServerConnected    = "server.connected"
	AuditServerDisconnected = "server.disconnected"
)

// File access events
const (
	AuditFileRead    = "file.read"
	AuditPathGranted = "path.granted"
	AuditPathRevoked = "path.revoked"
)

const (
	AuditLogTailStarted = "log.tail_started"
)

const (
	AuditFileUploaded = "file.uploaded"
)

package server

import (
	"net/http"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/auth"
)

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	loginLimiter := newRateLimiter(10, 60*time.Second)

	// Public auth endpoints (rate-limited)
	mux.Handle("GET /api/auth/status", loggingMiddleware(http.HandlerFunc(s.handleAuthStatus)))
	mux.Handle("POST /api/auth/register", loggingMiddleware(loginLimiter.middleware(http.HandlerFunc(s.handleRegister))))
	mux.Handle("POST /api/auth/login", loggingMiddleware(loginLimiter.middleware(http.HandlerFunc(s.handleLogin))))
	mux.Handle("POST /api/auth/login/totp", loggingMiddleware(loginLimiter.middleware(http.HandlerFunc(s.handleLoginTOTP))))

	// Refresh endpoint (token in body)
	mux.Handle("POST /api/auth/refresh", loggingMiddleware(http.HandlerFunc(s.handleRefresh)))

	// Setup-token-protected endpoints
	mux.Handle("POST /api/auth/totp/setup", loggingMiddleware(s.authMiddleware(auth.TokenTypeSetup, http.HandlerFunc(s.handleTOTPSetup))))
	mux.Handle("POST /api/auth/totp/enable", loggingMiddleware(s.authMiddleware(auth.TokenTypeSetup, http.HandlerFunc(s.handleTOTPEnable))))

	// Access-token-protected endpoints
	mux.Handle("GET /api/auth/me", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(s.handleMe))))
	mux.Handle("PUT /api/auth/password", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(s.handleChangePassword))))
	mux.Handle("PUT /api/auth/avatar", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(s.handleUpdateAvatar))))
	mux.Handle("POST /api/auth/totp/disable", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleTOTPDisable)))))

	// Admin endpoints
	mux.Handle("GET /api/users", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleListUsers)))))
	mux.Handle("POST /api/users", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleCreateUser)))))
	mux.Handle("PUT /api/users/{id}/role", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleUpdateUserRole)))))
	mux.Handle("DELETE /api/users/{id}", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleDeleteUser)))))
	mux.Handle("GET /api/audit-logs", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleListAuditLogs)))))

	// Server endpoints (any authenticated user, role-filtered in handler)
	mux.Handle("GET /api/servers", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(s.handleListServers))))
	mux.Handle("POST /api/servers", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleCreateServer)))))
	// batch-delete must be registered before /{id} routes so Go's ServeMux matches the literal path first
	mux.Handle("POST /api/servers/batch-delete", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleBatchDeleteServers)))))
	mux.Handle("GET /api/servers/{id}", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(s.handleGetServer))))
	mux.Handle("PUT /api/servers/{id}", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleUpdateServer)))))
	mux.Handle("DELETE /api/servers/{id}", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleDeleteServer)))))

	// File access path management (admin)
	mux.Handle("GET /api/servers/{id}/paths", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleListPaths)))))
	mux.Handle("POST /api/servers/{id}/paths", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleCreatePath)))))
	mux.Handle("DELETE /api/servers/{id}/paths/{pathId}", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleDeletePath)))))
	// User's own paths (any authenticated user)
	mux.Handle("GET /api/servers/{id}/my-paths", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(s.handleGetUserPaths))))

	// File browsing endpoints (any authenticated user, permission-checked in handler)
	mux.Handle("GET /api/servers/{id}/files", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(s.handleListFiles))))
	mux.Handle("GET /api/servers/{id}/files/read", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(s.handleReadFile))))

	// Log tailing SSE endpoint (any authenticated user, permission-checked in handler)
	mux.Handle("GET /api/servers/{id}/logs/tail", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(s.handleTailLog))))

	// Dropzone endpoints (admin only)
	mux.Handle("POST /api/servers/{id}/upload", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleUploadFile)))))
	mux.Handle("GET /api/servers/{id}/dropzone", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleListDropzone)))))
	mux.Handle("DELETE /api/servers/{id}/dropzone", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleDeleteDropzoneFile)))))

	// Server unregister (admin only)
	mux.Handle("DELETE /api/servers/{id}/unregister", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleUnregisterServer)))))

	// Public endpoints (no auth required)
	mux.Handle("GET /install.sh", loggingMiddleware(http.HandlerFunc(s.handleInstallScript)))
	mux.Handle("GET /install/{os}/{arch}", loggingMiddleware(http.HandlerFunc(s.handleAgentBinary)))
	mux.Handle("DELETE /api/servers/{id}/self-unregister", loggingMiddleware(http.HandlerFunc(s.handleSelfUnregister)))

	// SPA catch-all — serves embedded frontend, falls back to index.html
	mux.Handle("/", s.spaHandler())

	// Apply CORS globally, then security headers as outermost wrapper
	return securityHeaders(s.corsMiddleware(mux))
}

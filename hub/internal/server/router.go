package server

import (
	"net/http"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/auth"
)

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	loginLimiter := newRateLimiter(5, 60*time.Second)

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
	// Public agent registration endpoint (no auth required)
	mux.Handle("POST /api/servers/register", loggingMiddleware(http.HandlerFunc(s.handleRegisterAgent)))
	mux.Handle("GET /api/servers/{id}", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(s.handleGetServer))))
	mux.Handle("PUT /api/servers/{id}", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleUpdateServer)))))
	mux.Handle("DELETE /api/servers/{id}", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleDeleteServer)))))

	// SPA catch-all — serves embedded frontend, falls back to index.html
	mux.Handle("/", s.spaHandler())

	// Apply CORS globally
	return s.corsMiddleware(mux)
}

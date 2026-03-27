# Security Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 24 security vulnerabilities identified in the full codebase security review, organized into 4 parallel task groups by severity.

**Architecture:** Four independent work streams that can run in parallel - each group touches different files with no overlap. Group A fixes critical auth/endpoint issues, Group B fixes input validation/path traversal, Group C fixes transport/headers/gRPC, Group D fixes frontend/config hardening.

**Tech Stack:** Go (hub + agent), React/TypeScript (frontend), gRPC, SQLite, Docker

---

## Parallel Execution Strategy

These 4 groups are independent and can be assigned to separate worktrees/agents:

| Group | Severity | Issues | Files Touched |
|-------|----------|--------|---------------|
| A | Critical + High | #1, #3, #4, #5, #14, #15 | `handlers_unregister.go`, `middleware.go`, `handlers_auth.go`, `totp.go`, `handlers_servers.go` |
| B | Medium (paths) | #10, #11, #12, #13, #19, #22 | `handlers_upload.go`, `handlers_paths.go`, `install.sh`, `logtailer/tailer.go`, `filebrowser/filebrowser.go` |
| C | High (transport) | #7, #8, #9, #18, #23 | `server.go` (security headers), `grpcserver/pending.go`, `grpcserver/handler.go`, `client/client.go`, `docker-compose.yml` |
| D | Medium + Low (frontend/misc) | #6, #16, #17, #21, #24 | `auth.ts`, `security-model.md`, `server-detail.tsx`, `handlers_auth.go` (TOTP guards), `model/` (typed responses) |

---

## Group A: Critical Auth & Endpoint Fixes

### Task A1: Secure the self-unregister endpoint (#1 - Critical)

**Files:**
- Modify: `hub/internal/server/handlers_unregister.go:65-97`
- Test: `hub/internal/server/handlers_unregister_test.go`

The `DELETE /api/servers/{id}/self-unregister` endpoint has no authentication. Anyone who knows a server UUID can delete it. Fix by verifying the request comes from the registered agent IP.

- [ ] **Step 1: Write failing test for self-unregister requiring IP match**

```go
func TestHandleSelfUnregister_RejectsWrongIP(t *testing.T) {
    // Request from IP that doesn't match the server's registered ip_address should be rejected
    req := httptest.NewRequest("DELETE", "/api/servers/"+serverID+"/self-unregister", nil)
    req.RemoteAddr = "10.99.99.99:12345" // wrong IP
    w := httptest.NewRecorder()
    s.handleSelfUnregister(w, req)
    if w.Code != http.StatusForbidden {
        t.Fatalf("expected 403, got %d", w.Code)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd hub && go test ./internal/server/ -run TestHandleSelfUnregister_RejectsWrongIP -v`

- [ ] **Step 3: Implement IP verification in self-unregister**

```go
func (s *Server) handleSelfUnregister(w http.ResponseWriter, r *http.Request) {
    serverID := r.PathValue("id")

    server, err := s.store.GetServerByID(serverID)
    if err != nil {
        w.WriteHeader(http.StatusNoContent)
        return
    }

    // Verify request comes from the registered agent IP
    reqIP := clientIP(r)
    if server.IPAddress == nil || *server.IPAddress != reqIP {
        respondError(w, http.StatusForbidden, "unauthorized")
        return
    }

    // ... rest of handler unchanged
}
```

- [ ] **Step 4: Run tests**

Run: `cd hub && go test ./internal/server/ -v`

- [ ] **Step 5: Commit**

```bash
git add hub/internal/server/handlers_unregister.go hub/internal/server/handlers_unregister_test.go
git commit -m "security: require agent IP verification for self-unregister endpoint"
```

---

### Task A2: Fix rate limiter X-Forwarded-For bypass (#4 - High)

**Files:**
- Modify: `hub/internal/server/middleware.go:148-171`
- Test: `hub/internal/server/middleware_test.go`

The rate limiter trusts the X-Forwarded-For header from any client. Fix by always using RemoteAddr (with port stripped) for rate limiting.

- [ ] **Step 1: Write failing test**

```go
func TestRateLimiter_IgnoresSpoofedXFF(t *testing.T) {
    rl := newRateLimiter(2, 60*time.Second)
    handler := rl.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))

    for i := 0; i < 3; i++ {
        req := httptest.NewRequest("POST", "/api/auth/login", nil)
        req.RemoteAddr = "192.168.1.1:12345"
        req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.0.0.%d", i))
        w := httptest.NewRecorder()
        handler.ServeHTTP(w, req)
        if i == 2 && w.Code != 429 {
            t.Fatalf("request %d should be rate limited, got %d", i, w.Code)
        }
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Fix rate limiter to use RemoteAddr, strip port**

```go
func (rl *rateLimiter) middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ip, _, err := net.SplitHostPort(r.RemoteAddr)
        if err != nil {
            ip = r.RemoteAddr
        }

        if !rl.allow(ip) {
            w.Header().Set("Retry-After", "60")
            respondError(w, http.StatusTooManyRequests, "too many login attempts")
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

- [ ] **Step 4: Run tests**
- [ ] **Step 5: Commit**

```bash
git add hub/internal/server/middleware.go hub/internal/server/middleware_test.go
git commit -m "security: fix rate limiter to use RemoteAddr, ignore spoofable X-Forwarded-For"
```

---

### Task A3: Fix refresh token not checking DB state (#5 - High)

**Files:**
- Modify: `hub/internal/server/handlers_auth.go:187-210`
- Test: `hub/internal/server/handlers_auth_test.go`

The refresh handler reissues tokens using claims from the old token without checking if the user still exists or if their role changed.

- [ ] **Step 1: Write failing test**

```go
func TestHandleRefresh_DeletedUserRejected(t *testing.T) {
    // Create user, get refresh token, delete user, try refresh - should get 401
}
```

- [ ] **Step 2: Implement DB lookup in refresh handler**

```go
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

    accessToken, refreshToken, err := auth.GenerateTokenPair(s.jwtSecret, user.ID, user.Role)
    if err != nil {
        respondError(w, http.StatusInternalServerError, errFailedToGenerateTokens)
        return
    }

    respondJSON(w, http.StatusOK, model.TokenPair{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
    })
}
```

- [ ] **Step 3: Run tests**
- [ ] **Step 4: Commit**

```bash
git add hub/internal/server/handlers_auth.go hub/internal/server/handlers_auth_test.go
git commit -m "security: refresh token now verifies user exists and uses current DB role"
```

---

### Task A4: Add TOTP replay protection (#3 - Critical)

**Files:**
- Create: `hub/internal/auth/totp_cache.go`
- Modify: `hub/internal/auth/totp.go`
- Modify: `hub/internal/server/server.go` (wire cache)
- Modify: `hub/internal/server/handlers_auth.go` (use cache at login, enable, disable)
- Test: `hub/internal/auth/totp_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestTOTPCode_CannotBeReusedInSameWindow(t *testing.T) {
    cache := NewTOTPUsedCodeCache()
    secret := "JBSWY3DPEHPK3PXP"
    code, _ := GenerateValidCode(secret)
    if !ValidateTOTPWithReplay(cache, "user1", secret, code) {
        t.Fatal("first use should succeed")
    }
    if ValidateTOTPWithReplay(cache, "user1", secret, code) {
        t.Fatal("replay should be rejected")
    }
}
```

- [ ] **Step 2: Implement used-code cache in totp_cache.go**

```go
type TOTPUsedCodes struct {
    mu    sync.Mutex
    codes map[string]time.Time
}

func NewTOTPUsedCodeCache() *TOTPUsedCodes {
    return &TOTPUsedCodes{codes: make(map[string]time.Time)}
}

func (c *TOTPUsedCodes) MarkUsed(userID, code string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.codes[userID+":"+code] = time.Now()
    for k, t := range c.codes {
        if time.Since(t) > 90*time.Second {
            delete(c.codes, k)
        }
    }
}

func (c *TOTPUsedCodes) WasUsed(userID, code string) bool {
    c.mu.Lock()
    defer c.mu.Unlock()
    _, ok := c.codes[userID+":"+code]
    return ok
}
```

- [ ] **Step 3: Wire into Server struct and auth handlers**
- [ ] **Step 4: Run tests**
- [ ] **Step 5: Commit**

```bash
git commit -m "security: add TOTP replay protection with used-code cache"
```

---

### Task A5: Fix viewer permission check (#14 - Medium)

**Files:**
- Modify: `hub/internal/server/handlers_servers.go:143-162`
- Test: `hub/internal/server/handlers_servers_test.go`

- [ ] **Step 1: Replace Limit:1 query with direct permission check for the target server**
- [ ] **Step 2: Run tests**
- [ ] **Step 3: Commit**

```bash
git commit -m "security: fix viewer access check to query specific server instead of Limit:1"
```

---

### Task A6: Fix registration race condition (#15 - Medium)

**Files:**
- Modify: `hub/internal/server/handlers_auth.go:21-83`

- [ ] **Step 1: Let DB unique constraint handle the race - treat CreateUser error as "setup already completed"**
- [ ] **Step 2: Run tests**
- [ ] **Step 3: Commit**

```bash
git commit -m "security: handle registration race condition via DB constraint"
```

---

## Group B: Input Validation & Path Traversal

### Task B1: Sanitize dropzone filenames (#10, #22 - Medium)

**Files:**
- Modify: `hub/internal/server/handlers_upload.go:39,169`
- Test: `hub/internal/server/handlers_upload_test.go`

- [ ] **Step 1: Write failing test for traversal in delete filename**
- [ ] **Step 2: Apply `filepath.Base()` to both upload filename (line 39) and delete filename (line 169)**

```go
// Delete handler
filename = filepath.Base(filename)
if filename == "." || filename == "/" {
    respondError(w, http.StatusBadRequest, "invalid filename")
    return
}
path := "/tmp/aerodocs-dropzone/" + filename
```

- [ ] **Step 3: Run tests**
- [ ] **Step 4: Commit**

```bash
git commit -m "security: sanitize dropzone filenames with filepath.Base to prevent traversal"
```

---

### Task B2: Validate path in handleCreatePath (#11 - Medium)

**Files:**
- Modify: `hub/internal/server/handlers_paths.go:23-57`
- Test: `hub/internal/server/handlers_paths_test.go`

- [ ] **Step 1: Add `validateRequestPath(req.Path)` call before `CreatePermission`**
- [ ] **Step 2: Run tests**
- [ ] **Step 3: Commit**

```bash
git commit -m "security: validate paths before storing in permissions table"
```

---

### Task B3: Fix shell injection in install.sh (#12 - Medium)

**Files:**
- Modify: `hub/static/install.sh:~130-140`

- [ ] **Step 1: Quote heredoc delimiter (`<<'EOF'`), use sed for substitution**
- [ ] **Step 2: Add input validation regex for HUB and TOKEN**
- [ ] **Step 3: Commit**

```bash
git commit -m "security: fix shell injection in install.sh via quoted heredoc and input validation"
```

---

### Task B4: Add path validation to logtailer (#13 - Medium)

**Files:**
- Modify: `agent/internal/logtailer/tailer.go:19`

- [ ] **Step 1: Add `..` and absolute path checks at start of `StartTail`**
- [ ] **Step 2: Run tests**
- [ ] **Step 3: Commit**

```bash
git commit -m "security: add defense-in-depth path validation to agent logtailer"
```

---

### Task B5: Add symlink boundary check (#19 - Medium)

**Files:**
- Modify: `agent/internal/filebrowser/filebrowser.go`

- [ ] **Step 1: After EvalSymlinks, verify resolved path stays within the requested root**
- [ ] **Step 2: Run tests**
- [ ] **Step 3: Commit**

```bash
git commit -m "security: verify symlink targets stay within allowed directory boundaries"
```

---

## Group C: Transport, Headers & gRPC Hardening

### Task C1: Add security headers middleware (#7 - High)

**Files:**
- Modify: `hub/internal/server/server.go`
- Test: `hub/internal/server/middleware_test.go`

- [ ] **Step 1: Write failing test checking for X-Frame-Options, CSP, nosniff**
- [ ] **Step 2: Create `securityHeaders` middleware**

```go
func securityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
        w.Header().Set("Content-Security-Policy",
            "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
        next.ServeHTTP(w, r)
    })
}
```

- [ ] **Step 3: Wire into main mux**
- [ ] **Step 4: Run tests**
- [ ] **Step 5: Commit**

```bash
git commit -m "security: add security headers (CSP, X-Frame-Options, nosniff, Referrer-Policy)"
```

---

### Task C2: Fix agent tailSessions race condition (#8 - High)

**Files:**
- Modify: `agent/internal/client/client.go:155-168`

- [ ] **Step 1: Add `sync.Mutex` for `tailSessions` map**
- [ ] **Step 2: Wrap all map access with lock/unlock**
- [ ] **Step 3: Run with race detector**: `cd agent && go test -race ./...`
- [ ] **Step 4: Commit**

```bash
git commit -m "security: fix race condition on agent tailSessions map with mutex"
```

---

### Task C3: Scope pending requests per server (#9 - High)

**Files:**
- Modify: `hub/internal/grpcserver/pending.go`
- Modify: `hub/internal/grpcserver/handler.go`
- Modify: `hub/internal/server/agent_request.go` (if exists, update callers)

- [ ] **Step 1: Change pending map key to `serverID:requestID`**
- [ ] **Step 2: Update Register, Deliver, Remove to accept serverID**
- [ ] **Step 3: Update all callers**
- [ ] **Step 4: Run tests**
- [ ] **Step 5: Commit**

```bash
git commit -m "security: scope pending requests per server to prevent cross-agent spoofing"
```

---

### Task C4: Invalidate registration token after use (#18 - Medium)

**Files:**
- Modify: `hub/internal/grpcserver/handler.go:178-195`
- Create: `hub/internal/store/servers.go` (add ClearRegistrationToken method)

- [ ] **Step 1: Add store method to null out token**

```go
func (s *Store) ClearRegistrationToken(serverID string) error {
    _, err := s.db.Exec(
        "UPDATE servers SET registration_token = NULL, token_expires_at = NULL WHERE id = ?",
        serverID,
    )
    return err
}
```

- [ ] **Step 2: Call after successful ActivateServer in handler.go**
- [ ] **Step 3: Run tests**
- [ ] **Step 4: Commit**

```bash
git commit -m "security: invalidate registration token after successful agent registration"
```

---

### Task C5: Bind gRPC port to localhost in docker-compose (#23 - Low)

**Files:**
- Modify: `docker-compose.yml`

- [ ] **Step 1: Change `"9090:9090"` to `"127.0.0.1:9090:9090"`**
- [ ] **Step 2: Add comment explaining agents connect via Traefik**
- [ ] **Step 3: Commit**

```bash
git commit -m "security: bind gRPC port to localhost in docker-compose, route agents via Traefik"
```

---

## Group D: Frontend & Misc Hardening

### Task D1: Document localStorage token trade-off (#6 - High, informational)

**Files:**
- Modify: `docs/engineering/security-model.md`

Moving to httpOnly cookies requires significant refactoring. For v1.0, document as a known limitation with mitigations (CSP from C1, short access token lifetime).

- [ ] **Step 1: Add "Known Limitations" section**
- [ ] **Step 2: Commit**

```bash
git commit -m "docs: document localStorage token storage trade-off in security model"
```

---

### Task D2: Fix inconsistent XSS sanitization in markdown (#21 - Low)

**Files:**
- Modify: `web/src/pages/server-detail.tsx:164`

- [ ] **Step 1: Wrap hljs output with existing `sanitizeHljsHtml()` function**

The existing `HighlightedCodeBlock` component already uses `sanitizeHljsHtml()`. Apply the same to the `markdownComponents` code path for consistency.

- [ ] **Step 2: Commit**

```bash
git commit -m "security: apply sanitizeHljsHtml to markdown code blocks for consistent XSS defense"
```

---

### Task D3: Fix TOTP setup overwrite (#16) and admin self-disable (#17)

**Files:**
- Modify: `hub/internal/server/handlers_auth.go:249-280,378-411`
- Test: `hub/internal/server/handlers_auth_test.go`

- [ ] **Step 1: In `handleTOTPSetup`, reject if user already has TOTP enabled**

```go
user, err := s.store.GetUserByID(userID)
if err != nil {
    respondError(w, http.StatusInternalServerError, "failed to get user")
    return
}
if user.TOTPEnabled {
    respondError(w, http.StatusConflict, "TOTP is already enabled")
    return
}
```

- [ ] **Step 2: In `handleTOTPDisable`, prevent self-disable**

```go
adminID := UserIDFromContext(r.Context())
if req.UserID == adminID {
    respondError(w, http.StatusBadRequest, "cannot disable your own 2FA")
    return
}
```

- [ ] **Step 3: Run tests**
- [ ] **Step 4: Commit**

```bash
git commit -m "security: prevent TOTP overwrite when active, block admin self-disable"
```

---

### Task D4: Use typed response structs (#24 - Low)

**Files:**
- Create: `hub/internal/model/responses.go`
- Modify: Multiple `hub/internal/server/handlers_*.go` files

- [ ] **Step 1: Add typed response structs to `model/responses.go`**
- [ ] **Step 2: Replace `map[string]interface{}` in handlers one file at a time**
- [ ] **Step 3: Run tests after each file**
- [ ] **Step 4: Commit per file**

```bash
git commit -m "refactor: replace map[string]interface{} with typed response structs"
```

---

## Deferred to v1.1 (documented, not implemented now)

| Issue | Reason for Deferral |
|-------|-------------------|
| #2 - gRPC TLS/mTLS | Requires certificate management infrastructure. Mitigated by Traefik TLS termination for production. Document as requirement for direct-IP deployments. |
| #6 - httpOnly cookies | Requires full auth flow refactor (CSRF tokens, SameSite policy). Mitigated by CSP (Task C1) and short token lifetime. |
| #20 - RemoteAddr port in rate limiter | Fixed as part of Task A2 (strip port with `net.SplitHostPort`). |

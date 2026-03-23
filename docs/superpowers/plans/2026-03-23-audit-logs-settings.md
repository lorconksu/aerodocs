# Audit Logs & Settings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add audit log viewer with full filtering, user management UI, and profile settings (change password) to AeroDocs.

**Architecture:** Two new API endpoints (change password, update role), enhanced audit log filtering (date range), React pages for audit logs and tabbed settings.

**Tech Stack:** Same as sub-project 1 — Go, SQLite, React, TypeScript, Tailwind CSS, TanStack Query

**Spec:** `docs/superpowers/specs/2026-03-23-audit-logs-settings-design.md`

---

## File Map

### Go Backend Changes (`hub/`)

| File | Change |
|------|--------|
| `hub/internal/model/audit.go` | Add `From`/`To` fields to `AuditFilter`, add two new audit action constants |
| `hub/internal/model/user.go` | Add `ChangePasswordRequest`, `UpdateRoleRequest` types |
| `hub/internal/store/users.go` | Add `UpdateUserRole` method |
| `hub/internal/store/users_test.go` | Tests for `UpdateUserRole` |
| `hub/internal/store/audit.go` | Enhance `ListAuditLogs` with `From`/`To` WHERE clauses |
| `hub/internal/store/audit_test.go` | Tests for date-range filtering |
| `hub/internal/server/handlers_auth.go` | Add `handleChangePassword` handler |
| `hub/internal/server/handlers_auth_test.go` | Tests for change password |
| `hub/internal/server/handlers_users.go` | Add `handleUpdateUserRole` handler |
| `hub/internal/server/handlers_users_test.go` | Tests for update role |
| `hub/internal/server/handlers_audit.go` | Parse `from`/`to` query params |
| `hub/internal/server/router.go` | Register `PUT /api/auth/password` and `PUT /api/users/{id}/role` routes |

### React Frontend Changes (`web/`)

| File | Change |
|------|--------|
| `web/src/types/api.ts` | Add `ChangePasswordRequest`, `UpdateRoleRequest`, `TOTPDisableRequest` types |
| `web/src/pages/audit-logs.tsx` | Full audit log viewer with filters and pagination |
| `web/src/pages/settings.tsx` | Tabbed settings: Profile + Users |
| `web/src/pages/create-user-modal.tsx` | Create user modal component |
| `web/src/App.tsx` | Replace placeholder routes with real page imports |

---

## Task 1: Domain Model Additions

**Files:**
- Edit: `hub/internal/model/audit.go`
- Edit: `hub/internal/model/user.go`

- [ ] **Step 1: Add `From` and `To` fields to `AuditFilter` and new audit action constants**

Edit `hub/internal/model/audit.go` — add `From *string` and `To *string` fields to the `AuditFilter` struct, and add two new audit action constants `AuditUserPasswordChanged` and `AuditUserRoleUpdated`:

```go
type AuditFilter struct {
	UserID *string
	Action *string
	From   *string // ISO 8601 datetime
	To     *string // ISO 8601 datetime
	Limit  int
	Offset int
}

// Add to the const block:
AuditUserPasswordChanged = "user.password_changed"
AuditUserRoleUpdated     = "user.role_updated"
```

- [ ] **Step 2: Add `ChangePasswordRequest` and `UpdateRoleRequest` to user model**

Edit `hub/internal/model/user.go` — add these two structs after the existing `TOTPDisableRequest` struct:

```go
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type UpdateRoleRequest struct {
	Role Role `json:"role"`
}
```

- [ ] **Step 3: Verify compilation**

```bash
cd hub && go build ./...
```

Expected: compiles cleanly, no errors.

---

## Task 2: Store Method — `UpdateUserRole`

**Files:**
- Edit: `hub/internal/store/users_test.go`
- Edit: `hub/internal/store/users.go`

- [ ] **Step 1: Write test for `UpdateUserRole`**

Add to `hub/internal/store/users_test.go`:

```go
func TestUpdateUserRole(t *testing.T) {
	s := testStore(t)

	s.CreateUser(&model.User{
		ID: "u1", Username: "alice", Email: "alice@test.com",
		PasswordHash: "h", Role: model.RoleViewer,
	})

	if err := s.UpdateUserRole("u1", model.RoleAdmin); err != nil {
		t.Fatalf("update role: %v", err)
	}

	user, err := s.GetUserByID("u1")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if user.Role != model.RoleAdmin {
		t.Fatalf("expected role 'admin', got '%s'", user.Role)
	}
}

func TestUpdateUserRole_NonexistentUser(t *testing.T) {
	s := testStore(t)

	err := s.UpdateUserRole("nonexistent", model.RoleAdmin)
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}
```

- [ ] **Step 2: Verify test fails**

```bash
cd hub && go test ./internal/store/ -run TestUpdateUserRole -v
```

Expected: compilation error — `UpdateUserRole` does not exist.

- [ ] **Step 3: Implement `UpdateUserRole`**

Add to `hub/internal/store/users.go`:

```go
func (s *Store) UpdateUserRole(userID string, role model.Role) error {
	result, err := s.db.Exec(
		"UPDATE users SET role = ?, updated_at = datetime('now') WHERE id = ?",
		role, userID,
	)
	if err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update role rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}
```

- [ ] **Step 4: Verify tests pass**

```bash
cd hub && go test ./internal/store/ -run TestUpdateUserRole -v
```

Expected: both `TestUpdateUserRole` and `TestUpdateUserRole_NonexistentUser` pass.

---

## Task 3: Store Enhancement — Audit Log Date Filtering

**Files:**
- Edit: `hub/internal/store/audit_test.go`
- Edit: `hub/internal/store/audit.go`

- [ ] **Step 1: Write test for date-range filtering**

Add to `hub/internal/store/audit_test.go`:

```go
func TestListAuditWithDateRange(t *testing.T) {
	s := testStore(t)

	// Insert entries directly with controlled timestamps
	db := s.DB()
	db.Exec(`INSERT INTO audit_logs (id, action, created_at) VALUES ('a1', 'user.login', '2026-03-01 10:00:00')`)
	db.Exec(`INSERT INTO audit_logs (id, action, created_at) VALUES ('a2', 'user.login', '2026-03-15 10:00:00')`)
	db.Exec(`INSERT INTO audit_logs (id, action, created_at) VALUES ('a3', 'user.login', '2026-03-25 10:00:00')`)

	// Filter from March 10 to March 20
	from := "2026-03-10T00:00:00Z"
	to := "2026-03-20T23:59:59Z"
	entries, total, err := s.ListAuditLogs(model.AuditFilter{
		From:  &from,
		To:    &to,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].ID != "a2" {
		t.Fatalf("expected entry a2, got %s", entries[0].ID)
	}
}

func TestListAuditWithFromOnly(t *testing.T) {
	s := testStore(t)

	db := s.DB()
	db.Exec(`INSERT INTO audit_logs (id, action, created_at) VALUES ('a1', 'user.login', '2026-03-01 10:00:00')`)
	db.Exec(`INSERT INTO audit_logs (id, action, created_at) VALUES ('a2', 'user.login', '2026-03-15 10:00:00')`)

	from := "2026-03-10T00:00:00Z"
	entries, total, err := s.ListAuditLogs(model.AuditFilter{
		From:  &from,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(entries) != 1 || entries[0].ID != "a2" {
		t.Fatalf("expected entry a2")
	}
}

func TestListAuditWithToOnly(t *testing.T) {
	s := testStore(t)

	db := s.DB()
	db.Exec(`INSERT INTO audit_logs (id, action, created_at) VALUES ('a1', 'user.login', '2026-03-01 10:00:00')`)
	db.Exec(`INSERT INTO audit_logs (id, action, created_at) VALUES ('a2', 'user.login', '2026-03-15 10:00:00')`)

	to := "2026-03-10T23:59:59Z"
	entries, total, err := s.ListAuditLogs(model.AuditFilter{
		To:    &to,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(entries) != 1 || entries[0].ID != "a1" {
		t.Fatalf("expected entry a1")
	}
}
```

- [ ] **Step 2: Verify tests fail**

```bash
cd hub && go test ./internal/store/ -run TestListAuditWith -v
```

Expected: tests fail because `From`/`To` fields exist but are not used in the query.

- [ ] **Step 3: Enhance `ListAuditLogs` to support `From` and `To`**

Edit `hub/internal/store/audit.go` — in the `ListAuditLogs` function, add two new WHERE clause blocks after the existing `filter.Action` check:

```go
	if filter.From != nil {
		where = append(where, "created_at >= ?")
		args = append(args, *filter.From)
	}
	if filter.To != nil {
		where = append(where, "created_at <= ?")
		args = append(args, *filter.To)
	}
```

These go right after the existing `filter.Action` block and before the `whereClause` construction.

- [ ] **Step 4: Verify tests pass**

```bash
cd hub && go test ./internal/store/ -run TestListAudit -v
```

Expected: all audit log tests pass including the new date-range tests.

---

## Task 4: Handler — Change Password

**Files:**
- Edit: `hub/internal/server/handlers_auth_test.go`
- Edit: `hub/internal/server/handlers_auth.go`
- Edit: `hub/internal/server/router.go`

- [ ] **Step 1: Write tests for change password**

Add to `hub/internal/server/handlers_auth_test.go`. This requires a helper that creates a fully authenticated user (register + TOTP setup + enable). The existing `registerAndGetAdminToken` helper in `handlers_users_test.go` does this; reuse it from the same package.

```go
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
```

- [ ] **Step 2: Verify tests fail**

```bash
cd hub && go test ./internal/server/ -run TestChangePassword -v
```

Expected: 404 — route not registered yet.

- [ ] **Step 3: Implement `handleChangePassword`**

Add to `hub/internal/server/handlers_auth.go`:

```go
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req model.ChangePasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := UserIDFromContext(r.Context())
	user, err := s.store.GetUserByID(userID)
	if err != nil {
		respondError(w, http.StatusNotFound, "user not found")
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
```

- [ ] **Step 4: Register the route**

Edit `hub/internal/server/router.go` — add this line in the "Access-token-protected endpoints" section, after the `handleMe` route:

```go
	mux.Handle("PUT /api/auth/password", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(s.handleChangePassword))))
```

- [ ] **Step 5: Verify tests pass**

```bash
cd hub && go test ./internal/server/ -run TestChangePassword -v
```

Expected: all three change-password tests pass.

---

## Task 5: Handler — Update User Role

**Files:**
- Edit: `hub/internal/server/handlers_users_test.go`
- Edit: `hub/internal/server/handlers_users.go`
- Edit: `hub/internal/server/router.go`

- [ ] **Step 1: Write tests for update user role**

Add to `hub/internal/server/handlers_users_test.go`:

```go
func TestUpdateUserRole_Success(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// Create a viewer user first
	createBody, _ := json.Marshal(model.CreateUserRequest{
		Username: "viewer1", Email: "viewer@test.com", Role: model.RoleViewer,
	})
	createReq := httptest.NewRequest("POST", "/api/users", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	s.routes().ServeHTTP(createRec, createReq)

	var createResp model.CreateUserResponse
	json.NewDecoder(createRec.Body).Decode(&createResp)

	// Update role to admin
	body, _ := json.Marshal(model.UpdateRoleRequest{Role: model.RoleAdmin})
	req := httptest.NewRequest("PUT", "/api/users/"+createResp.User.ID+"/role", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	user := resp["user"].(map[string]interface{})
	if user["role"] != "admin" {
		t.Fatalf("expected role 'admin', got '%v'", user["role"])
	}
}

func TestUpdateUserRole_CannotChangeOwnRole(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// Get own user ID from /api/auth/me
	meReq := httptest.NewRequest("GET", "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+token)
	meRec := httptest.NewRecorder()
	s.routes().ServeHTTP(meRec, meReq)

	var me model.User
	json.NewDecoder(meRec.Body).Decode(&me)

	// Try to change own role
	body, _ := json.Marshal(model.UpdateRoleRequest{Role: model.RoleViewer})
	req := httptest.NewRequest("PUT", "/api/users/"+me.ID+"/role", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUserRole_InvalidRole(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// Create a user first
	createBody, _ := json.Marshal(model.CreateUserRequest{
		Username: "viewer1", Email: "viewer@test.com", Role: model.RoleViewer,
	})
	createReq := httptest.NewRequest("POST", "/api/users", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	s.routes().ServeHTTP(createRec, createReq)

	var createResp model.CreateUserResponse
	json.NewDecoder(createRec.Body).Decode(&createResp)

	// Try invalid role
	body := []byte(`{"role": "superadmin"}`)
	req := httptest.NewRequest("PUT", "/api/users/"+createResp.User.ID+"/role", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: Verify tests fail**

```bash
cd hub && go test ./internal/server/ -run TestUpdateUserRole -v
```

Expected: 404 — route not registered yet.

- [ ] **Step 3: Implement `handleUpdateUserRole`**

Add to `hub/internal/server/handlers_users.go`:

```go
func (s *Server) handleUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	targetID := r.PathValue("id")
	if targetID == "" {
		respondError(w, http.StatusBadRequest, "missing user id")
		return
	}

	var req model.UpdateRoleRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Role != model.RoleAdmin && req.Role != model.RoleViewer {
		respondError(w, http.StatusBadRequest, "role must be 'admin' or 'viewer'")
		return
	}

	adminID := UserIDFromContext(r.Context())
	if adminID == targetID {
		respondError(w, http.StatusBadRequest, "cannot change your own role")
		return
	}

	if err := s.store.UpdateUserRole(targetID, req.Role); err != nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	user, err := s.store.GetUserByID(targetID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch updated user")
		return
	}

	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &adminID,
		Action: model.AuditUserRoleUpdated, Target: &targetID, IPAddress: &ip,
	})

	respondJSON(w, http.StatusOK, map[string]interface{}{"user": user})
}
```

- [ ] **Step 4: Register the route**

Edit `hub/internal/server/router.go` — add this line in the "Admin endpoints" section, after the `POST /api/users` route:

```go
	mux.Handle("PUT /api/users/{id}/role", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleUpdateUserRole)))))
```

- [ ] **Step 5: Verify tests pass**

```bash
cd hub && go test ./internal/server/ -run TestUpdateUserRole -v
```

Expected: all three update-role tests pass.

---

## Task 6: Audit Log Handler — Date Filtering

**Files:**
- Edit: `hub/internal/server/handlers_audit.go`

- [ ] **Step 1: Add `from` and `to` query parameter parsing**

Edit `hub/internal/server/handlers_audit.go` — add these two blocks after the existing `user_id` query param parsing in `handleListAuditLogs`:

```go
	if v := q.Get("from"); v != "" {
		filter.From = &v
	}
	if v := q.Get("to"); v != "" {
		filter.To = &v
	}
```

- [ ] **Step 2: Run all backend tests**

```bash
cd hub && go test ./... -v
```

Expected: all tests pass. The date filtering is already tested at the store level; the handler just passes through the query params.

---

## Task 7: Frontend Types

**Files:**
- Edit: `web/src/types/api.ts`

- [ ] **Step 1: Add new TypeScript types**

Add to `web/src/types/api.ts` before the `ApiError` interface:

```typescript
export interface ChangePasswordRequest {
  current_password: string
  new_password: string
}

export interface UpdateRoleRequest {
  role: Role
}

export interface TOTPDisableRequest {
  user_id: string
  admin_totp_code: string
}
```

Note: `TOTPDisableRequest` already exists in the Go model but was not in the frontend types — add it now since the Settings/Users tab needs it.

- [ ] **Step 2: Verify frontend compiles**

```bash
cd web && npx tsc --noEmit
```

Expected: no type errors.

---

## Task 8: Audit Logs Page

**Files:**
- Create: `web/src/pages/audit-logs.tsx`
- Edit: `web/src/App.tsx`

- [ ] **Step 1: Create the Audit Logs page**

Create `web/src/pages/audit-logs.tsx` with the following structure:

```typescript
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '@/lib/api'
import type { AuditLogResponse, User } from '@/types/api'

const AUDIT_ACTIONS = [
  'user.login',
  'user.login_failed',
  'user.login_totp_failed',
  'user.registered',
  'user.created',
  'user.password_changed',
  'user.role_updated',
  'user.totp_setup',
  'user.totp_enabled',
  'user.totp_disabled',
  'user.totp_reset',
  'server.created',
  'server.updated',
  'server.deleted',
  'server.batch_deleted',
  'server.registered',
] as const

const PAGE_SIZE = 50

export function AuditLogsPage() {
  const [filters, setFilters] = useState<{
    action: string
    userId: string
    from: string
    to: string
  }>({ action: '', userId: '', from: '', to: '' })
  const [offset, setOffset] = useState(0)

  // Build query string from filters
  const buildParams = () => {
    const params = new URLSearchParams()
    params.set('limit', String(PAGE_SIZE))
    params.set('offset', String(offset))
    if (filters.action) params.set('action', filters.action)
    if (filters.userId) params.set('user_id', filters.userId)
    if (filters.from) params.set('from', new Date(filters.from).toISOString())
    if (filters.to) params.set('to', new Date(filters.to + 'T23:59:59').toISOString())
    return params.toString()
  }

  const { data, isLoading } = useQuery({
    queryKey: ['audit-logs', filters, offset],
    queryFn: () => apiFetch<AuditLogResponse>(`/audit-logs?${buildParams()}`),
  })

  const { data: users } = useQuery({
    queryKey: ['users'],
    queryFn: () => apiFetch<User[]>('/users'),
  })

  const hasActiveFilters = filters.action || filters.userId || filters.from || filters.to

  const clearFilters = () => {
    setFilters({ action: '', userId: '', from: '', to: '' })
    setOffset(0)
  }

  const updateFilter = (key: keyof typeof filters, value: string) => {
    setFilters(prev => ({ ...prev, [key]: value }))
    setOffset(0)
  }

  const formatDate = (iso: string) => {
    return new Date(iso).toLocaleDateString('en-US', {
      month: 'short', day: 'numeric', year: 'numeric',
      hour: 'numeric', minute: '2-digit',
    })
  }

  const getUsernameById = (userId: string | null) => {
    if (!userId) return 'System'
    const user = users?.find(u => u.id === userId)
    return user?.username ?? userId
  }

  const total = data?.total ?? 0
  const entries = data?.entries ?? []
  const showingFrom = total > 0 ? offset + 1 : 0
  const showingTo = Math.min(offset + PAGE_SIZE, total)

  return (
    <div>
      <h2 className="text-lg font-semibold text-text-primary mb-4">Audit Logs</h2>

      {/* Filter Bar */}
      <div className="flex items-center gap-3 mb-4 flex-wrap">
        <input
          type="date"
          value={filters.from}
          onChange={e => updateFilter('from', e.target.value)}
          className="bg-elevated border border-border rounded px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:border-accent"
          placeholder="From date"
        />
        <input
          type="date"
          value={filters.to}
          onChange={e => updateFilter('to', e.target.value)}
          className="bg-elevated border border-border rounded px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:border-accent"
          placeholder="To date"
        />
        <select
          value={filters.userId}
          onChange={e => updateFilter('userId', e.target.value)}
          className="bg-elevated border border-border rounded px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:border-accent"
        >
          <option value="">All Users</option>
          {users?.map(u => (
            <option key={u.id} value={u.id}>{u.username}</option>
          ))}
        </select>
        <select
          value={filters.action}
          onChange={e => updateFilter('action', e.target.value)}
          className="bg-elevated border border-border rounded px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:border-accent"
        >
          <option value="">All Actions</option>
          {AUDIT_ACTIONS.map(a => (
            <option key={a} value={a}>{a}</option>
          ))}
        </select>
        {hasActiveFilters && (
          <button
            onClick={clearFilters}
            className="text-accent hover:text-accent-hover text-sm transition-colors"
          >
            Clear Filters
          </button>
        )}
      </div>

      {/* Table */}
      <div className="border border-border rounded overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-surface border-b border-border">
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">Timestamp</th>
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">User</th>
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">Action</th>
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">Target</th>
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">IP Address</th>
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr><td colSpan={5} className="px-4 py-8 text-center text-text-muted">Loading...</td></tr>
            ) : entries.length === 0 ? (
              <tr><td colSpan={5} className="px-4 py-8 text-center text-text-muted">No audit log entries found.</td></tr>
            ) : (
              entries.map(entry => (
                <tr key={entry.id} className="border-b border-border last:border-b-0 hover:bg-surface/50">
                  <td className="px-4 py-2 text-text-secondary">{formatDate(entry.created_at)}</td>
                  <td className="px-4 py-2 text-text-primary">{getUsernameById(entry.user_id)}</td>
                  <td className="px-4 py-2">
                    <span className="font-mono text-xs bg-elevated px-2 py-0.5 rounded text-text-secondary">{entry.action}</span>
                  </td>
                  <td className="px-4 py-2 text-text-muted font-mono text-xs">{entry.target ?? '—'}</td>
                  <td className="px-4 py-2 text-text-muted font-mono text-xs">{entry.ip_address ?? '—'}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination Footer */}
      {total > 0 && (
        <div className="flex items-center justify-between mt-3 text-sm text-text-muted">
          <span>Showing {showingFrom}-{showingTo} of {total}</span>
          <div className="flex gap-2">
            <button
              onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
              disabled={offset === 0}
              className="px-3 py-1 border border-border rounded text-text-secondary hover:bg-surface disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              Previous
            </button>
            <button
              onClick={() => setOffset(offset + PAGE_SIZE)}
              disabled={offset + PAGE_SIZE >= total}
              className="px-3 py-1 border border-border rounded text-text-secondary hover:bg-surface disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              Next
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Update App.tsx to import AuditLogsPage**

Edit `web/src/App.tsx`:
- Add import: `import { AuditLogsPage } from '@/pages/audit-logs'`
- Replace the placeholder route `<Route path="/audit-logs" element={<div className="text-text-muted text-sm">Audit logs — sub-project 7</div>} />` with: `<Route path="/audit-logs" element={<AuditLogsPage />} />`

- [ ] **Step 3: Verify frontend compiles**

```bash
cd web && npx tsc --noEmit
```

Expected: no type errors.

---

## Task 9: Settings Page — Profile Tab

**Files:**
- Create: `web/src/pages/settings.tsx`
- Edit: `web/src/App.tsx`

- [ ] **Step 1: Create the Settings page with Profile tab**

Create `web/src/pages/settings.tsx` with the full tabbed layout. This step includes the Profile tab; the Users tab is added in Task 10.

```typescript
import { useState } from 'react'
import { useAuth } from '@/hooks/use-auth'
import { useMutation } from '@tanstack/react-query'
import { apiFetch } from '@/lib/api'
import type { ChangePasswordRequest } from '@/types/api'

function ProfileTab() {
  const { user } = useAuth()
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [passwordErrors, setPasswordErrors] = useState<string[]>([])
  const [success, setSuccess] = useState('')

  const validatePassword = (pw: string) => {
    const errors: string[] = []
    if (pw.length < 12) errors.push('At least 12 characters')
    if (!/[A-Z]/.test(pw)) errors.push('One uppercase letter')
    if (!/[a-z]/.test(pw)) errors.push('One lowercase letter')
    if (!/\d/.test(pw)) errors.push('One digit')
    if (!/[^a-zA-Z0-9]/.test(pw)) errors.push('One special character')
    setPasswordErrors(errors)
    return errors.length === 0
  }

  const mutation = useMutation({
    mutationFn: (data: ChangePasswordRequest) =>
      apiFetch<{ status: string }>('/auth/password', {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      setSuccess('Password updated successfully.')
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
      setPasswordErrors([])
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setSuccess('')

    if (!validatePassword(newPassword)) return
    if (newPassword !== confirmPassword) {
      mutation.reset()
      return
    }

    mutation.mutate({
      current_password: currentPassword,
      new_password: newPassword,
    })
  }

  return (
    <div className="max-w-lg space-y-6">
      {/* Account Info */}
      <div>
        <h3 className="text-sm font-semibold text-text-primary mb-3">Account Info</h3>
        <div className="bg-surface border border-border rounded p-4 space-y-2 text-sm">
          <div className="flex justify-between">
            <span className="text-text-muted">Username</span>
            <span className="text-text-primary">{user?.username}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-text-muted">Email</span>
            <span className="text-text-primary">{user?.email}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-text-muted">Role</span>
            <span className="text-text-primary capitalize">{user?.role}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-text-muted">2FA</span>
            <span className="text-status-online">Enabled</span>
          </div>
        </div>
      </div>

      {/* Change Password */}
      <div>
        <h3 className="text-sm font-semibold text-text-primary mb-3">Change Password</h3>
        <form onSubmit={handleSubmit} className="space-y-3">
          {mutation.isError && (
            <div className="bg-status-error/10 border border-status-error/20 text-status-error text-xs rounded px-3 py-2">
              {mutation.error instanceof Error ? mutation.error.message : 'Failed to update password'}
            </div>
          )}
          {success && (
            <div className="bg-status-online/10 border border-status-online/20 text-status-online text-xs rounded px-3 py-2">
              {success}
            </div>
          )}

          <input
            type="password"
            placeholder="Current password"
            value={currentPassword}
            onChange={e => setCurrentPassword(e.target.value)}
            className="w-full bg-elevated border border-border rounded px-3 py-2 text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent"
            required
          />
          <div>
            <input
              type="password"
              placeholder="New password (min 12 chars)"
              value={newPassword}
              onChange={e => { setNewPassword(e.target.value); validatePassword(e.target.value) }}
              className="w-full bg-elevated border border-border rounded px-3 py-2 text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent"
              required
            />
            {newPassword && passwordErrors.length > 0 && (
              <div className="mt-2 space-y-1">
                {passwordErrors.map(err => (
                  <div key={err} className="text-status-warning text-[10px]">• {err}</div>
                ))}
              </div>
            )}
          </div>
          <input
            type="password"
            placeholder="Confirm new password"
            value={confirmPassword}
            onChange={e => setConfirmPassword(e.target.value)}
            className="w-full bg-elevated border border-border rounded px-3 py-2 text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent"
            required
          />
          {confirmPassword && newPassword !== confirmPassword && (
            <div className="text-status-warning text-[10px]">• Passwords do not match</div>
          )}
          <button
            type="submit"
            disabled={mutation.isPending || passwordErrors.length > 0 || newPassword !== confirmPassword}
            className="bg-accent hover:bg-accent-hover text-white text-sm font-semibold rounded px-4 py-2 transition-colors disabled:opacity-50"
          >
            {mutation.isPending ? 'Updating...' : 'Update Password'}
          </button>
        </form>
      </div>
    </div>
  )
}

export function SettingsPage() {
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'
  const [activeTab, setActiveTab] = useState<'profile' | 'users'>('profile')

  return (
    <div>
      <h2 className="text-lg font-semibold text-text-primary mb-4">Settings</h2>

      {/* Tab bar */}
      <div className="flex border-b border-border mb-6">
        <button
          onClick={() => setActiveTab('profile')}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            activeTab === 'profile'
              ? 'border-accent text-text-primary'
              : 'border-transparent text-text-muted hover:text-text-secondary'
          }`}
        >
          Profile
        </button>
        {isAdmin && (
          <button
            onClick={() => setActiveTab('users')}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              activeTab === 'users'
                ? 'border-accent text-text-primary'
                : 'border-transparent text-text-muted hover:text-text-secondary'
            }`}
          >
            Users
          </button>
        )}
      </div>

      {/* Tab content */}
      {activeTab === 'profile' && <ProfileTab />}
      {activeTab === 'users' && isAdmin && <UsersTab />}
    </div>
  )
}

// Placeholder — implemented in Task 10
function UsersTab() {
  return <div className="text-text-muted text-sm">Loading users tab...</div>
}
```

- [ ] **Step 2: Update App.tsx to import SettingsPage**

Edit `web/src/App.tsx`:
- Add import: `import { SettingsPage } from '@/pages/settings'`
- Replace the placeholder route `<Route path="/settings" element={<div className="text-text-muted text-sm">Settings — sub-project 7</div>} />` with: `<Route path="/settings" element={<SettingsPage />} />`

- [ ] **Step 3: Verify frontend compiles**

```bash
cd web && npx tsc --noEmit
```

Expected: no type errors.

---

## Task 10: Settings Page — Users Tab + Create User Modal

**Files:**
- Create: `web/src/pages/create-user-modal.tsx`
- Edit: `web/src/pages/settings.tsx`

- [ ] **Step 1: Create the Create User modal**

Create `web/src/pages/create-user-modal.tsx`:

```typescript
import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '@/lib/api'
import type { CreateUserRequest, CreateUserResponse, Role } from '@/types/api'

interface CreateUserModalProps {
  onClose: () => void
}

export function CreateUserModal({ onClose }: CreateUserModalProps) {
  const queryClient = useQueryClient()
  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [role, setRole] = useState<Role>('viewer')
  const [tempPassword, setTempPassword] = useState('')
  const [copied, setCopied] = useState(false)

  const mutation = useMutation({
    mutationFn: (data: CreateUserRequest) =>
      apiFetch<CreateUserResponse>('/users', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: (data) => {
      setTempPassword(data.temporary_password)
      queryClient.invalidateQueries({ queryKey: ['users'] })
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    mutation.mutate({ username, email, role })
  }

  const copyPassword = async () => {
    await navigator.clipboard.writeText(tempPassword)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-surface border border-border rounded-lg p-6 w-full max-w-md">
        {tempPassword ? (
          // Success state — show temporary password
          <div>
            <h3 className="text-sm font-semibold text-text-primary mb-4">User Created</h3>
            <p className="text-text-muted text-xs mb-3">
              Share this temporary password with the user. It will not be shown again.
            </p>
            <div className="flex items-center gap-2 mb-4">
              <code className="flex-1 bg-elevated border border-border rounded px-3 py-2 text-sm text-text-primary font-mono select-all">
                {tempPassword}
              </code>
              <button
                onClick={copyPassword}
                className="px-3 py-2 bg-accent hover:bg-accent-hover text-white text-sm rounded transition-colors"
              >
                {copied ? 'Copied!' : 'Copy'}
              </button>
            </div>
            <button
              onClick={onClose}
              className="w-full border border-border rounded py-2 text-sm text-text-secondary hover:bg-elevated transition-colors"
            >
              Done
            </button>
          </div>
        ) : (
          // Form state
          <form onSubmit={handleSubmit}>
            <h3 className="text-sm font-semibold text-text-primary mb-4">Create User</h3>

            {mutation.isError && (
              <div className="bg-status-error/10 border border-status-error/20 text-status-error text-xs rounded px-3 py-2 mb-3">
                {mutation.error instanceof Error ? mutation.error.message : 'Failed to create user'}
              </div>
            )}

            <div className="space-y-3">
              <input
                type="text"
                placeholder="Username"
                value={username}
                onChange={e => setUsername(e.target.value)}
                className="w-full bg-elevated border border-border rounded px-3 py-2 text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent"
                autoFocus
                required
              />
              <input
                type="email"
                placeholder="Email"
                value={email}
                onChange={e => setEmail(e.target.value)}
                className="w-full bg-elevated border border-border rounded px-3 py-2 text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent"
                required
              />
              <select
                value={role}
                onChange={e => setRole(e.target.value as Role)}
                className="w-full bg-elevated border border-border rounded px-3 py-2 text-sm text-text-primary focus:outline-none focus:border-accent"
              >
                <option value="viewer">Viewer</option>
                <option value="admin">Admin</option>
              </select>
            </div>

            <div className="flex gap-2 mt-4">
              <button
                type="button"
                onClick={onClose}
                className="flex-1 border border-border rounded py-2 text-sm text-text-secondary hover:bg-elevated transition-colors"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={mutation.isPending}
                className="flex-1 bg-accent hover:bg-accent-hover text-white text-sm font-semibold rounded py-2 transition-colors disabled:opacity-50"
              >
                {mutation.isPending ? 'Creating...' : 'Create User'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Implement the Users tab in settings.tsx**

Edit `web/src/pages/settings.tsx` — replace the placeholder `UsersTab` function with the full implementation. The function needs to be moved above `SettingsPage` so it's available when referenced, or kept below since JavaScript hoists function declarations.

Replace the placeholder `UsersTab` function at the bottom of the file with:

```typescript
function UsersTab() {
  const { user: currentUser } = useAuth()
  const queryClient = useQueryClient()
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [disableTotpUserId, setDisableTotpUserId] = useState<string | null>(null)
  const [adminTotpCode, setAdminTotpCode] = useState('')

  const { data: users, isLoading } = useQuery({
    queryKey: ['users'],
    queryFn: () => apiFetch<User[]>('/users'),
  })

  const updateRoleMutation = useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: Role }) =>
      apiFetch<{ user: User }>(`/users/${userId}/role`, {
        method: 'PUT',
        body: JSON.stringify({ role }),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
    },
  })

  const disableTotpMutation = useMutation({
    mutationFn: (data: TOTPDisableRequest) =>
      apiFetch<{ status: string }>('/auth/totp/disable', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      setDisableTotpUserId(null)
      setAdminTotpCode('')
    },
  })

  const handleDisableTotp = (e: React.FormEvent) => {
    e.preventDefault()
    if (!disableTotpUserId) return
    disableTotpMutation.mutate({
      user_id: disableTotpUserId,
      admin_totp_code: adminTotpCode,
    })
  }

  const formatDate = (iso: string) => {
    const date = new Date(iso)
    const now = new Date()
    const diffMs = now.getTime() - date.getTime()
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))
    if (diffDays === 0) return 'Today'
    if (diffDays === 1) return 'Yesterday'
    if (diffDays < 30) return `${diffDays} days ago`
    return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-sm font-semibold text-text-primary">User Management</h3>
        <button
          onClick={() => setShowCreateModal(true)}
          className="bg-accent hover:bg-accent-hover text-white text-sm font-semibold rounded px-4 py-1.5 transition-colors"
        >
          Create User
        </button>
      </div>

      {updateRoleMutation.isError && (
        <div className="bg-status-error/10 border border-status-error/20 text-status-error text-xs rounded px-3 py-2 mb-3">
          {updateRoleMutation.error instanceof Error ? updateRoleMutation.error.message : 'Failed to update role'}
        </div>
      )}

      <div className="border border-border rounded overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-surface border-b border-border">
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">Username</th>
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">Email</th>
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">Role</th>
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">2FA</th>
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">Created</th>
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">Actions</th>
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr><td colSpan={6} className="px-4 py-8 text-center text-text-muted">Loading...</td></tr>
            ) : !users || users.length === 0 ? (
              <tr><td colSpan={6} className="px-4 py-8 text-center text-text-muted">No users found.</td></tr>
            ) : (
              users.map(u => (
                <tr key={u.id} className="border-b border-border last:border-b-0 hover:bg-surface/50">
                  <td className="px-4 py-2 text-text-primary">{u.username}</td>
                  <td className="px-4 py-2 text-text-secondary">{u.email}</td>
                  <td className="px-4 py-2">
                    {u.id === currentUser?.id ? (
                      <span className={`text-xs px-2 py-0.5 rounded ${
                        u.role === 'admin' ? 'bg-accent/20 text-accent' : 'bg-elevated text-text-muted'
                      }`}>
                        {u.role === 'admin' ? 'Admin' : 'Viewer'}
                      </span>
                    ) : (
                      <select
                        value={u.role}
                        onChange={e => updateRoleMutation.mutate({ userId: u.id, role: e.target.value as Role })}
                        disabled={updateRoleMutation.isPending}
                        className="bg-elevated border border-border rounded px-2 py-0.5 text-xs text-text-primary focus:outline-none focus:border-accent"
                      >
                        <option value="admin">Admin</option>
                        <option value="viewer">Viewer</option>
                      </select>
                    )}
                  </td>
                  <td className="px-4 py-2">
                    {u.totp_enabled ? (
                      <span className="text-xs text-status-online">Enabled</span>
                    ) : (
                      <span className="text-xs text-status-warning">Not set up</span>
                    )}
                  </td>
                  <td className="px-4 py-2 text-text-muted text-xs">{formatDate(u.created_at)}</td>
                  <td className="px-4 py-2">
                    {u.id !== currentUser?.id && u.totp_enabled && (
                      <button
                        onClick={() => setDisableTotpUserId(u.id)}
                        className="text-xs text-status-warning hover:text-status-error transition-colors"
                      >
                        Disable 2FA
                      </button>
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Create User Modal */}
      {showCreateModal && (
        <CreateUserModal onClose={() => setShowCreateModal(false)} />
      )}

      {/* Disable TOTP Confirmation Modal */}
      {disableTotpUserId && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-surface border border-border rounded-lg p-6 w-full max-w-sm">
            <h3 className="text-sm font-semibold text-text-primary mb-2">Disable 2FA</h3>
            <p className="text-text-muted text-xs mb-4">
              Enter your own TOTP code to confirm disabling 2FA for this user.
            </p>

            {disableTotpMutation.isError && (
              <div className="bg-status-error/10 border border-status-error/20 text-status-error text-xs rounded px-3 py-2 mb-3">
                {disableTotpMutation.error instanceof Error ? disableTotpMutation.error.message : 'Failed to disable 2FA'}
              </div>
            )}

            <form onSubmit={handleDisableTotp}>
              <input
                type="text"
                placeholder="Your 6-digit TOTP code"
                value={adminTotpCode}
                onChange={e => setAdminTotpCode(e.target.value)}
                className="w-full bg-elevated border border-border rounded px-3 py-2 text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent mb-4 font-mono text-center tracking-widest"
                maxLength={6}
                autoFocus
                required
              />
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => { setDisableTotpUserId(null); setAdminTotpCode('') }}
                  className="flex-1 border border-border rounded py-2 text-sm text-text-secondary hover:bg-elevated transition-colors"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={disableTotpMutation.isPending || adminTotpCode.length !== 6}
                  className="flex-1 bg-status-error hover:bg-status-error/80 text-white text-sm font-semibold rounded py-2 transition-colors disabled:opacity-50"
                >
                  {disableTotpMutation.isPending ? 'Disabling...' : 'Disable 2FA'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
```

This requires adding new imports to the top of `web/src/pages/settings.tsx`:

```typescript
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useAuth } from '@/hooks/use-auth'
import { apiFetch } from '@/lib/api'
import type { ChangePasswordRequest, User, Role, TOTPDisableRequest } from '@/types/api'
import { CreateUserModal } from '@/pages/create-user-modal'
```

- [ ] **Step 3: Verify frontend compiles**

```bash
cd web && npx tsc --noEmit
```

Expected: no type errors.

---

## Task 11: Final Integration Verification

- [ ] **Step 1: Run all backend tests**

```bash
cd hub && go test ./... -v
```

Expected: all tests pass, including the new ones for `UpdateUserRole`, date-range audit filtering, `handleChangePassword`, and `handleUpdateUserRole`.

- [ ] **Step 2: Run frontend type check**

```bash
cd web && npx tsc --noEmit
```

Expected: no type errors.

- [ ] **Step 3: Verify the frontend builds**

```bash
cd web && npm run build
```

Expected: clean build with no warnings or errors.

- [ ] **Step 4: Verify the full Go build (with embedded frontend)**

```bash
cd hub && go build ./cmd/aerodocs/
```

Expected: compiles successfully.

---

## Summary of Changes

### New files (3)
- `web/src/pages/audit-logs.tsx` — Audit log viewer with filters and pagination
- `web/src/pages/settings.tsx` — Tabbed settings: Profile tab (change password) + Users tab (user table, role editing, 2FA disable)
- `web/src/pages/create-user-modal.tsx` — Create user modal with temp password display

### Modified files (10)
- `hub/internal/model/audit.go` — Add `From`/`To` to `AuditFilter`, add 2 audit action constants
- `hub/internal/model/user.go` — Add `ChangePasswordRequest`, `UpdateRoleRequest`
- `hub/internal/store/users.go` — Add `UpdateUserRole`
- `hub/internal/store/users_test.go` — Tests for `UpdateUserRole`
- `hub/internal/store/audit.go` — Date-range WHERE clauses in `ListAuditLogs`
- `hub/internal/store/audit_test.go` — Tests for date-range filtering
- `hub/internal/server/handlers_auth.go` — Add `handleChangePassword`
- `hub/internal/server/handlers_auth_test.go` — Tests for change password
- `hub/internal/server/handlers_users.go` — Add `handleUpdateUserRole`
- `hub/internal/server/handlers_users_test.go` — Tests for update role
- `hub/internal/server/handlers_audit.go` — Parse `from`/`to` query params
- `hub/internal/server/router.go` — Register 2 new routes
- `web/src/types/api.ts` — Add 3 new TypeScript interfaces
- `web/src/App.tsx` — Replace placeholder routes with real page imports

### New API endpoints (2)
- `PUT /api/auth/password` — Change own password (access token, any role)
- `PUT /api/users/{id}/role` — Update user role (access token, admin only)

### Enhanced endpoints (1)
- `GET /api/audit-logs` — Now accepts `from` and `to` query parameters for date filtering

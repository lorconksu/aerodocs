# AeroDocs Sub-project 7: Audit Logs & Settings — Design Spec

**Date:** 2026-03-23
**Status:** Approved
**Scope:** Audit log viewer UI with full filtering, user management (list/create/edit/disable 2FA), profile settings (change password), two new API endpoints

## 1. Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Audit log filtering | Full filtering | Date range, user, action type, target. Paginated sortable table. |
| Settings scope | User management only | List users, create users, edit roles, disable 2FA, change own password. Permissions UI deferred until agents exist. |
| Page structure | Tabbed settings page | Profile tab (all users) + Users tab (admin only) on `/settings`. Audit logs on `/audit-logs`. |

## 2. API Endpoints

### 2.1 Existing Endpoints (from sub-project 1)

| Method | Path | Auth | Already Built |
|--------|------|------|--------------|
| GET | `/api/users` | Access (Admin) | Yes — list all users |
| POST | `/api/users` | Access (Admin) | Yes — create user with temp password |
| POST | `/api/auth/totp/disable` | Access (Admin) | Yes — disable another user's 2FA |
| GET | `/api/audit-logs` | Access (Admin) | Yes — paginated, filterable by user_id and action |

### 2.2 New Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| PUT | `/api/auth/password` | Access | Change own password. Body: `{ "current_password": "...", "new_password": "..." }`. Validates current password, applies password policy to new password, updates hash. |
| PUT | `/api/users/:id/role` | Access (Admin) | Update user role. Body: `{ "role": "admin" \| "viewer" }`. Cannot change own role. |

### 2.3 Audit Log Endpoint Enhancement

The existing `GET /api/audit-logs` needs two new query parameters for date filtering:

| Param | Type | Description |
|-------|------|-------------|
| `from` | string | ISO 8601 datetime — filter entries after this time |
| `to` | string | ISO 8601 datetime — filter entries before this time |

These are added to the existing `AuditFilter` struct and the `ListAuditLogs` store method.

### 2.4 Change Password Response

Success: `200 { "status": "password updated" }`

Failures:
- Wrong current password: `401 { "error": "invalid current password" }`
- New password fails policy: `400 { "error": "password must be at least 12 characters" }`

### 2.5 Update Role Response

Success: `200 { "user": { ...updated user... } }`

Failures:
- Cannot change own role: `400 { "error": "cannot change your own role" }`
- Invalid role: `400 { "error": "role must be 'admin' or 'viewer'" }`

## 3. Audit Events

New audit actions logged:
- `user.password_changed` — user changed their own password
- `user.role_updated` — admin changed a user's role

## 4. Store Method Additions

| Method | Description |
|--------|-------------|
| `ListAuditLogs(filter)` | Enhanced: add `From` and `To` fields to `AuditFilter`, add WHERE clauses for `created_at >= ?` and `created_at <= ?` |
| `UpdateUserRole(userID string, role model.Role) error` | Update user's role |

## 5. Domain Model Additions

```go
type ChangePasswordRequest struct {
    CurrentPassword string `json:"current_password"`
    NewPassword     string `json:"new_password"`
}

type UpdateRoleRequest struct {
    Role Role `json:"role"`
}
```

Add `From` and `To` fields to the existing `AuditFilter`:

```go
type AuditFilter struct {
    UserID *string
    Action *string
    From   *string  // ISO 8601 datetime
    To     *string  // ISO 8601 datetime
    Limit  int
    Offset int
}
```

## 6. Audit Logs Page UI

### 6.1 Route

| Path | Layout | Component | Auth |
|------|--------|-----------|------|
| `/audit-logs` | AppShell | AuditLogsPage | Yes (Admin) |

### 6.2 Layout

**Filter bar** at the top of the page, horizontal row:
- Date range: two date inputs (from / to)
- User: dropdown select populated from `GET /api/users`
- Action: dropdown select with all known action types
- "Clear Filters" button (appears when any filter is active)

**Table** below the filter bar:

| Column | Content |
|--------|---------|
| Timestamp | ISO datetime formatted to local time, e.g., "Mar 23, 2026 2:15 PM" |
| User | Username. "System" if user_id is null. |
| Action | Action type in a monospaced badge, e.g., `user.login` |
| Target | Target ID/name if present, "—" if null |
| IP Address | Monospaced |

**Pagination footer**: "Showing 1-50 of 234" with Previous/Next buttons.

### 6.3 Data Fetching

TanStack Query:
- `useQuery(['audit-logs', filters])` — refetches when filters change
- `useQuery(['users'])` — populates the user dropdown

### 6.4 Action Type List

The dropdown includes all known audit actions:
- `user.login`, `user.login_failed`, `user.login_totp_failed`
- `user.registered`, `user.created`, `user.password_changed`, `user.role_updated`
- `user.totp_setup`, `user.totp_enabled`, `user.totp_disabled`, `user.totp_reset`
- `server.created`, `server.updated`, `server.deleted`, `server.batch_deleted`, `server.registered` (from sub-project 2)

## 7. Settings Page UI

### 7.1 Route

| Path | Layout | Component | Auth |
|------|--------|-----------|------|
| `/settings` | AppShell | SettingsPage | Yes |

### 7.2 Tab Structure

Two tabs at the top: **Profile** | **Users** (admin only)

### 7.3 Profile Tab (all users)

**Account Info section:**
- Display: username, email, role (read-only)
- 2FA status: "Enabled" with green badge

**Change Password section:**
- Current password input
- New password input with real-time policy validation (same component as setup page)
- Confirm new password input
- "Update Password" button

### 7.4 Users Tab (admin only)

**Header:** "User Management" title + "Create User" button

**User table:**

| Column | Content |
|--------|---------|
| Username | Text |
| Email | Text |
| Role | Badge: "Admin" (blue) or "Viewer" (gray) |
| 2FA | "Enabled" (green) or "Not set up" (amber) |
| Created | Relative date |
| Actions | Edit Role / Disable 2FA |

**"Edit Role" action:** Inline dropdown that toggles between admin/viewer. Saves immediately on change.

**"Disable 2FA" action:** Confirmation modal requiring admin's own TOTP code. Uses existing `POST /api/auth/totp/disable` endpoint.

**"Create User" button:** Modal with username, email, role fields. On success, shows the temporary password with copy button (same pattern as Add Server). Uses existing `POST /api/users` endpoint.

### 7.5 Frontend Files

| File | Responsibility |
|------|---------------|
| `web/src/pages/audit-logs.tsx` | Full audit log viewer with filters and pagination |
| `web/src/pages/settings.tsx` | Tabbed settings: Profile + Users |
| `web/src/pages/create-user-modal.tsx` | Create user modal component |
| `web/src/types/api.ts` | Add ChangePasswordRequest, UpdateRoleRequest types |

## 8. Out of Scope

- Permissions management UI (deferred until agents exist)
- User deletion (not in spec — users can be deactivated by changing role or disabling 2FA)
- Email notifications
- Audit log export/download

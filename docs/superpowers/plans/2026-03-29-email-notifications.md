# Email Notifications Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add configurable email notifications so users are alerted when important events occur (agents going offline, security events, system changes).

**Architecture:** A `notify` package provides a `Notifier` service with a buffered channel queue and background worker goroutine. Events are triggered alongside existing `LogAudit` calls. SMTP config is stored in the `_config` table, per-user preferences in a new `notification_preferences` table, and delivery history in a `notification_log` table. The frontend adds a "Notifications" tab to Settings with SMTP config (admin) and per-user preference toggles.

**Tech Stack:** Go `net/smtp`, SQLite migrations, React + TanStack Query, Tailwind CSS

---

## File Structure

### New Files (Backend)
| File | Responsibility |
|------|---------------|
| `hub/internal/migrate/migrations/009_create_notifications.sql` | DB schema for preferences + log tables |
| `hub/internal/notify/notifier.go` | Notifier service: queue, worker, Notify() method |
| `hub/internal/notify/notifier_test.go` | Unit tests for notifier |
| `hub/internal/notify/smtp.go` | SMTP dial, send, TLS handling |
| `hub/internal/notify/smtp_test.go` | SMTP tests with mock server |
| `hub/internal/notify/templates.go` | Email subject/body templates per event type |
| `hub/internal/notify/templates_test.go` | Template rendering tests |
| `hub/internal/store/notifications.go` | Store methods for preferences + log |
| `hub/internal/store/notifications_test.go` | Store tests |
| `hub/internal/server/handlers_notifications.go` | HTTP handlers for SMTP config, preferences, log |
| `hub/internal/server/handlers_notifications_test.go` | Handler tests |
| `hub/internal/model/notification.go` | Types: SMTPConfig, NotificationPreference, NotificationLogEntry, event type constants |

### New Files (Frontend)
| File | Responsibility |
|------|---------------|
| `web/src/pages/settings-notifications-tab.tsx` | Admin SMTP config + notification log |
| `web/src/pages/settings-preferences-tab.tsx` | Per-user notification preference toggles |
| `web/src/pages/__tests__/settings-notifications-tab.test.tsx` | Admin notifications tab tests |
| `web/src/pages/__tests__/settings-preferences-tab.test.tsx` | User preferences tab tests |

### Modified Files
| File | Change |
|------|--------|
| `hub/internal/server/server.go` | Add `notifier` field to Server struct, init in `New()` |
| `hub/internal/server/router.go` | Register notification API routes |
| `hub/internal/server/handlers_auth.go` | Add `Notify()` calls for login_failed, password_changed, totp_changed |
| `hub/internal/server/handlers_users.go` | Add `Notify()` call for user_created |
| `hub/internal/server/handlers_upload.go` | Add `Notify()` call for file_uploaded |
| `hub/internal/grpcserver/handler.go` | Add `Notifier` to Handler, call for agent connect/disconnect |
| `hub/internal/grpcserver/server.go` | Accept Notifier in Config |
| `hub/cmd/aerodocs/main.go` | Create and pass Notifier to server + grpc configs |
| `web/src/pages/settings.tsx` | Add "Notifications" + "Alerts" tabs |
| `web/src/types/api.ts` | Add notification-related types |

---

## Task 1: Database Schema

**Files:**
- Create: `hub/internal/migrate/migrations/009_create_notifications.sql`

- [ ] **Step 1: Write migration**

```sql
-- Per-user notification preferences
CREATE TABLE IF NOT EXISTS notification_preferences (
    user_id    TEXT NOT NULL,
    event_type TEXT NOT NULL,
    enabled    INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (user_id, event_type),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Notification delivery log
CREATE TABLE IF NOT EXISTS notification_log (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    event_type TEXT NOT NULL,
    subject    TEXT NOT NULL,
    status     TEXT NOT NULL CHECK(status IN ('sent', 'failed')),
    error      TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_notification_log_created ON notification_log(created_at);
CREATE INDEX IF NOT EXISTS idx_notification_log_user ON notification_log(user_id);
```

- [ ] **Step 2: Verify migration runs**

Run: `cd hub && go test ./internal/migrate/ -v`
Expected: PASS (migration auto-discovered by embed)

- [ ] **Step 3: Commit**

```bash
git add hub/internal/migrate/migrations/009_create_notifications.sql
git commit -m "feat(notify): add notification_preferences and notification_log tables"
```

---

## Task 2: Notification Model Types

**Files:**
- Create: `hub/internal/model/notification.go`

- [ ] **Step 1: Define types and event constants**

```go
package model

// Notification event types
const (
	NotifyAgentOffline      = "agent.offline"
	NotifyAgentOnline       = "agent.online"
	NotifyAgentRegistered   = "agent.registered"
	NotifyLoginFailed       = "security.login_failed"
	NotifyUserCreated       = "security.user_created"
	NotifyTOTPChanged       = "security.totp_changed"
	NotifyPasswordChanged   = "security.password_changed"
	NotifyFileUploaded      = "system.file_uploaded"
)

// AllNotifyEvents lists every event type with its default enabled state and display label.
var AllNotifyEvents = []NotifyEventDef{
	{Type: NotifyAgentOffline, Label: "Agent went offline", Category: "Agent", DefaultOn: true},
	{Type: NotifyAgentOnline, Label: "Agent came online", Category: "Agent", DefaultOn: false},
	{Type: NotifyAgentRegistered, Label: "New agent enrolled", Category: "Agent", DefaultOn: true},
	{Type: NotifyLoginFailed, Label: "Failed login attempt", Category: "Security", DefaultOn: true},
	{Type: NotifyUserCreated, Label: "New user created", Category: "Security", DefaultOn: true},
	{Type: NotifyTOTPChanged, Label: "2FA configuration changed", Category: "Security", DefaultOn: true},
	{Type: NotifyPasswordChanged, Label: "Password changed", Category: "Security", DefaultOn: false},
	{Type: NotifyFileUploaded, Label: "File uploaded", Category: "System", DefaultOn: false},
}

type NotifyEventDef struct {
	Type      string `json:"type"`
	Label     string `json:"label"`
	Category  string `json:"category"`
	DefaultOn bool   `json:"default_on"`
}

type NotificationPreference struct {
	EventType string `json:"event_type"`
	Label     string `json:"label"`
	Category  string `json:"category"`
	Enabled   bool   `json:"enabled"`
}

type NotificationLogEntry struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	EventType string `json:"event_type"`
	Subject   string `json:"subject"`
	Status    string `json:"status"`
	Error     *string `json:"error"`
	CreatedAt string `json:"created_at"`
}

type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"` // write-only: masked on read
	From     string `json:"from"`
	TLS      bool   `json:"tls"`
	Enabled  bool   `json:"enabled"`
}

type SMTPTestRequest struct {
	Recipient string `json:"recipient"`
}

type NotificationPreferencesRequest struct {
	Preferences []NotificationPrefUpdate `json:"preferences"`
}

type NotificationPrefUpdate struct {
	EventType string `json:"event_type"`
	Enabled   bool   `json:"enabled"`
}
```

- [ ] **Step 2: Commit**

```bash
git add hub/internal/model/notification.go
git commit -m "feat(notify): add notification model types and event constants"
```

---

## Task 3: Store Methods for Notifications

**Files:**
- Create: `hub/internal/store/notifications.go`
- Create: `hub/internal/store/notifications_test.go`

- [ ] **Step 1: Write failing tests**

```go
package store

import (
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func TestGetNotificationPreferences_Defaults(t *testing.T) {
	st := testStore(t)
	createTestUser(t, st, "user1")

	prefs, err := st.GetNotificationPreferences("user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prefs) != len(model.AllNotifyEvents) {
		t.Fatalf("expected %d prefs, got %d", len(model.AllNotifyEvents), len(prefs))
	}
	// agent.offline should default to enabled
	for _, p := range prefs {
		if p.EventType == model.NotifyAgentOffline && !p.Enabled {
			t.Fatal("expected agent.offline to default to enabled")
		}
		if p.EventType == model.NotifyAgentOnline && p.Enabled {
			t.Fatal("expected agent.online to default to disabled")
		}
	}
}

func TestSetNotificationPreference(t *testing.T) {
	st := testStore(t)
	createTestUser(t, st, "user1")

	// Disable an event that defaults to on
	err := st.SetNotificationPreference("user1", model.NotifyAgentOffline, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prefs, _ := st.GetNotificationPreferences("user1")
	for _, p := range prefs {
		if p.EventType == model.NotifyAgentOffline && p.Enabled {
			t.Fatal("expected agent.offline to be disabled after update")
		}
	}
}

func TestGetEnabledRecipients(t *testing.T) {
	st := testStore(t)
	createTestUser(t, st, "user1")

	// Default: agent.offline is on
	users, err := st.GetEnabledRecipients(model.NotifyAgentOffline)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 recipient, got %d", len(users))
	}

	// Disable it
	st.SetNotificationPreference("user1", model.NotifyAgentOffline, false)
	users, _ = st.GetEnabledRecipients(model.NotifyAgentOffline)
	if len(users) != 0 {
		t.Fatalf("expected 0 recipients after disable, got %d", len(users))
	}
}

func TestLogNotification(t *testing.T) {
	st := testStore(t)
	createTestUser(t, st, "user1")

	err := st.LogNotification("notif-1", "user1", model.NotifyAgentOffline, "Agent went offline: web-01", "sent", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, total, err := st.ListNotificationLog(50, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 || len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d (total %d)", len(entries), total)
	}
	if entries[0].Subject != "Agent went offline: web-01" {
		t.Fatalf("unexpected subject: %s", entries[0].Subject)
	}
}
```

Note: `testStore` and `createTestUser` are helpers — check if they exist in existing store test files. If not, they follow this pattern:
```go
func testStore(t *testing.T) *Store {
	t.Helper()
	st, err := New(":memory:")
	if err != nil { t.Fatalf("create store: %v", err) }
	t.Cleanup(func() { st.Close() })
	return st
}

func createTestUser(t *testing.T, st *Store, id string) {
	t.Helper()
	st.CreateUser(&model.User{
		ID: id, Username: id, Email: id + "@test.com",
		PasswordHash: "$2a$12$dummy", Role: model.RoleAdmin,
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd hub && go test ./internal/store/ -run "TestGetNotificationPreferences|TestSetNotificationPreference|TestGetEnabledRecipients|TestLogNotification" -v`
Expected: FAIL — functions don't exist

- [ ] **Step 3: Implement store methods**

```go
package store

import (
	"fmt"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

// GetNotificationPreferences returns all event types with the user's
// enabled/disabled preference. Events without an explicit preference
// use the default from AllNotifyEvents.
func (s *Store) GetNotificationPreferences(userID string) ([]model.NotificationPreference, error) {
	// Load explicit overrides
	overrides := make(map[string]bool)
	rows, err := s.db.Query("SELECT event_type, enabled FROM notification_preferences WHERE user_id = ?", userID)
	if err != nil {
		return nil, fmt.Errorf("get notification prefs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var eventType string
		var enabled bool
		if err := rows.Scan(&eventType, &enabled); err != nil {
			return nil, fmt.Errorf("scan notification pref: %w", err)
		}
		overrides[eventType] = enabled
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notification prefs: %w", err)
	}

	// Merge with defaults
	result := make([]model.NotificationPreference, 0, len(model.AllNotifyEvents))
	for _, evt := range model.AllNotifyEvents {
		enabled := evt.DefaultOn
		if override, ok := overrides[evt.Type]; ok {
			enabled = override
		}
		result = append(result, model.NotificationPreference{
			EventType: evt.Type,
			Label:     evt.Label,
			Category:  evt.Category,
			Enabled:   enabled,
		})
	}
	return result, nil
}

// SetNotificationPreference upserts a user's preference for a specific event type.
func (s *Store) SetNotificationPreference(userID, eventType string, enabled bool) error {
	_, err := s.db.Exec(
		`INSERT INTO notification_preferences (user_id, event_type, enabled)
		 VALUES (?, ?, ?)
		 ON CONFLICT(user_id, event_type) DO UPDATE SET enabled = excluded.enabled`,
		userID, eventType, enabled,
	)
	if err != nil {
		return fmt.Errorf("set notification pref: %w", err)
	}
	return nil
}

// GetEnabledRecipients returns users (id, email) who have the given event type enabled.
// Users without an explicit preference are included if the event defaults to ON.
func (s *Store) GetEnabledRecipients(eventType string) ([]model.User, error) {
	// Find default for this event type
	defaultOn := false
	for _, evt := range model.AllNotifyEvents {
		if evt.Type == eventType {
			defaultOn = evt.DefaultOn
			break
		}
	}

	var query string
	if defaultOn {
		// Include users who haven't explicitly disabled
		query = `SELECT id, username, email, password_hash, role, totp_secret, totp_enabled, avatar, created_at, updated_at
			FROM users WHERE id NOT IN (
				SELECT user_id FROM notification_preferences WHERE event_type = ? AND enabled = 0
			)`
	} else {
		// Only include users who explicitly enabled
		query = `SELECT u.id, u.username, u.email, u.password_hash, u.role, u.totp_secret, u.totp_enabled, u.avatar, u.created_at, u.updated_at
			FROM users u INNER JOIN notification_preferences np
			ON u.id = np.user_id WHERE np.event_type = ? AND np.enabled = 1`
	}

	rows, err := s.db.Query(query, eventType)
	if err != nil {
		return nil, fmt.Errorf("get enabled recipients: %w", err)
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		u, err := s.scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}

// LogNotification records a sent or failed notification in the log.
func (s *Store) LogNotification(id, userID, eventType, subject, status string, errMsg *string) error {
	_, err := s.db.Exec(
		`INSERT INTO notification_log (id, user_id, event_type, subject, status, error)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, userID, eventType, subject, status, errMsg,
	)
	if err != nil {
		return fmt.Errorf("log notification: %w", err)
	}
	return nil
}

// ListNotificationLog returns recent notification log entries with pagination.
func (s *Store) ListNotificationLog(limit, offset int) ([]model.NotificationLogEntry, int, error) {
	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM notification_log").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count notification log: %w", err)
	}

	rows, err := s.db.Query(
		`SELECT nl.id, nl.user_id, u.username, nl.event_type, nl.subject, nl.status, nl.error, nl.created_at
		 FROM notification_log nl
		 LEFT JOIN users u ON nl.user_id = u.id
		 ORDER BY nl.created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list notification log: %w", err)
	}
	defer rows.Close()

	var entries []model.NotificationLogEntry
	for rows.Next() {
		var e model.NotificationLogEntry
		var username *string
		if err := rows.Scan(&e.ID, &e.UserID, &username, &e.EventType, &e.Subject, &e.Status, &e.Error, &e.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan notification log: %w", err)
		}
		if username != nil {
			e.Username = *username
		}
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd hub && go test ./internal/store/ -run "TestGetNotificationPreferences|TestSetNotificationPreference|TestGetEnabledRecipients|TestLogNotification" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add hub/internal/store/notifications.go hub/internal/store/notifications_test.go
git commit -m "feat(notify): add store methods for preferences and notification log"
```

---

## Task 4: Email Templates

**Files:**
- Create: `hub/internal/notify/templates.go`
- Create: `hub/internal/notify/templates_test.go`

- [ ] **Step 1: Write failing test**

```go
package notify

import (
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func TestRenderEmail_AgentOffline(t *testing.T) {
	subject, body := RenderEmail(model.NotifyAgentOffline, map[string]string{
		"server_name": "web-01",
		"server_id":   "srv-123",
		"timestamp":   "2026-03-29 12:00:00 UTC",
	})
	if subject == "" {
		t.Fatal("expected non-empty subject")
	}
	if body == "" {
		t.Fatal("expected non-empty body")
	}
	if !contains(body, "web-01") {
		t.Fatal("expected body to contain server name")
	}
}

func TestRenderEmail_UnknownEvent(t *testing.T) {
	subject, body := RenderEmail("unknown.event", map[string]string{})
	if subject == "" || body == "" {
		t.Fatal("expected fallback template for unknown events")
	}
}

func TestRenderEmail_AllEventTypes(t *testing.T) {
	for _, evt := range model.AllNotifyEvents {
		subject, body := RenderEmail(evt.Type, map[string]string{
			"server_name": "test-server",
			"server_id":   "srv-1",
			"username":    "admin",
			"ip":          "1.2.3.4",
			"filename":    "test.txt",
			"timestamp":   "2026-03-29 12:00:00 UTC",
		})
		if subject == "" {
			t.Errorf("empty subject for event %s", evt.Type)
		}
		if body == "" {
			t.Errorf("empty body for event %s", evt.Type)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd hub && go test ./internal/notify/ -run "TestRenderEmail" -v`
Expected: FAIL — package doesn't exist

- [ ] **Step 3: Implement templates**

```go
package notify

import (
	"fmt"
	"strings"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

type emailTemplate struct {
	Subject string // format string with %s placeholders
	Body    string // format string with named {{key}} placeholders
}

var templates = map[string]emailTemplate{
	model.NotifyAgentOffline: {
		Subject: "Agent Offline: {{server_name}}",
		Body: `An agent has gone offline.

Server: {{server_name}} ({{server_id}})
Time: {{timestamp}}

The agent has disconnected from the Hub. This may indicate a network issue, server reboot, or agent crash.

— AeroDocs`,
	},
	model.NotifyAgentOnline: {
		Subject: "Agent Online: {{server_name}}",
		Body: `An agent has connected.

Server: {{server_name}} ({{server_id}})
Time: {{timestamp}}

— AeroDocs`,
	},
	model.NotifyAgentRegistered: {
		Subject: "New Agent Enrolled: {{server_name}}",
		Body: `A new agent has been registered.

Server: {{server_name}} ({{server_id}})
Time: {{timestamp}}

— AeroDocs`,
	},
	model.NotifyLoginFailed: {
		Subject: "Failed Login Attempt",
		Body: `A failed login attempt was detected.

Username: {{username}}
IP Address: {{ip}}
Time: {{timestamp}}

If this was not you, consider reviewing your security settings.

— AeroDocs`,
	},
	model.NotifyUserCreated: {
		Subject: "New User Created: {{username}}",
		Body: `A new user account has been created.

Username: {{username}}
Time: {{timestamp}}

— AeroDocs`,
	},
	model.NotifyTOTPChanged: {
		Subject: "2FA Configuration Changed",
		Body: `A user's two-factor authentication settings have been modified.

Username: {{username}}
Action: {{detail}}
Time: {{timestamp}}

— AeroDocs`,
	},
	model.NotifyPasswordChanged: {
		Subject: "Password Changed: {{username}}",
		Body: `A user's password has been changed.

Username: {{username}}
Time: {{timestamp}}

— AeroDocs`,
	},
	model.NotifyFileUploaded: {
		Subject: "File Uploaded: {{filename}}",
		Body: `A file has been uploaded to the dropzone.

Server: {{server_name}} ({{server_id}})
Filename: {{filename}}
Uploaded by: {{username}}
Time: {{timestamp}}

— AeroDocs`,
	},
}

// RenderEmail returns a subject and body for the given event type,
// substituting {{key}} placeholders from the context map.
func RenderEmail(eventType string, context map[string]string) (string, string) {
	tmpl, ok := templates[eventType]
	if !ok {
		tmpl = emailTemplate{
			Subject: fmt.Sprintf("[AeroDocs] %s", eventType),
			Body:    fmt.Sprintf("Event: %s\nTime: %s\n\n— AeroDocs", eventType, context["timestamp"]),
		}
	}

	subject := substituteTemplate(tmpl.Subject, context)
	body := substituteTemplate(tmpl.Body, context)

	return "[AeroDocs] " + subject, body
}

func substituteTemplate(tmpl string, context map[string]string) string {
	result := tmpl
	for key, value := range context {
		result = strings.ReplaceAll(result, "{{"+key+"}}", value)
	}
	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd hub && go test ./internal/notify/ -run "TestRenderEmail" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add hub/internal/notify/templates.go hub/internal/notify/templates_test.go
git commit -m "feat(notify): add email templates for all event types"
```

---

## Task 5: SMTP Sender

**Files:**
- Create: `hub/internal/notify/smtp.go`
- Create: `hub/internal/notify/smtp_test.go`

- [ ] **Step 1: Write failing test**

```go
package notify

import (
	"net"
	"strings"
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func TestSendEmail_Success(t *testing.T) {
	// Start a mock SMTP server
	received := make(chan string, 1)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go mockSMTPServer(t, ln, received)

	addr := ln.Addr().String()
	host, port := splitHostPort(addr)

	cfg := model.SMTPConfig{
		Host:    host,
		Port:    port,
		From:    "noreply@aerodocs.local",
		TLS:     false,
		Enabled: true,
	}

	err = SendEmail(cfg, "user@test.com", "Test Subject", "Test Body")
	if err != nil {
		t.Fatalf("send email: %v", err)
	}

	data := <-received
	if !strings.Contains(data, "Test Subject") {
		t.Fatal("expected email to contain subject")
	}
}

func TestSendEmail_Disabled(t *testing.T) {
	cfg := model.SMTPConfig{Enabled: false}
	err := SendEmail(cfg, "user@test.com", "Subject", "Body")
	if err != nil {
		t.Fatal("expected no error when SMTP is disabled")
	}
}
```

Note: `mockSMTPServer` and `splitHostPort` are test helpers — implement them inline in the test file.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd hub && go test ./internal/notify/ -run "TestSendEmail" -v`
Expected: FAIL — `SendEmail` not defined

- [ ] **Step 3: Implement SMTP sender**

```go
package notify

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

// SendEmail sends a plain-text email via SMTP. Returns nil immediately
// if SMTP is not enabled.
func SendEmail(cfg model.SMTPConfig, to, subject, body string) error {
	if !cfg.Enabled {
		return nil
	}

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		cfg.From, to, subject, body)

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}

	if cfg.TLS {
		return sendWithTLS(cfg, addr, auth, to, []byte(msg))
	}
	return smtp.SendMail(addr, auth, cfg.From, []string{to}, []byte(msg))
}

func sendWithTLS(cfg model.SMTPConfig, addr string, auth smtp.Auth, to string, msg []byte) error {
	tlsConfig := &tls.Config{ServerName: cfg.Host}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("tls dial: %w", err)
	}

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := client.Mail(cfg.From); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	return client.Quit()
}
```

- [ ] **Step 4: Add mock SMTP test helpers**

Add to `smtp_test.go`:
```go
func mockSMTPServer(t *testing.T, ln net.Listener, received chan<- string) {
	t.Helper()
	conn, err := ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	// Minimal SMTP conversation
	fmt.Fprintf(conn, "220 localhost ESMTP\r\n")
	buf := make([]byte, 4096)
	var allData string
	for {
		n, err := conn.Read(buf)
		if err != nil {
			break
		}
		data := string(buf[:n])
		allData += data
		if strings.HasPrefix(data, "EHLO") || strings.HasPrefix(data, "HELO") {
			fmt.Fprintf(conn, "250-localhost\r\n250 OK\r\n")
		} else if strings.HasPrefix(data, "MAIL FROM") {
			fmt.Fprintf(conn, "250 OK\r\n")
		} else if strings.HasPrefix(data, "RCPT TO") {
			fmt.Fprintf(conn, "250 OK\r\n")
		} else if strings.HasPrefix(data, "DATA") {
			fmt.Fprintf(conn, "354 Send data\r\n")
		} else if strings.Contains(data, "\r\n.\r\n") {
			fmt.Fprintf(conn, "250 OK\r\n")
		} else if strings.HasPrefix(data, "QUIT") {
			fmt.Fprintf(conn, "221 Bye\r\n")
			break
		}
	}
	received <- allData
}

func splitHostPort(addr string) (string, int) {
	host, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)
	return host, port
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd hub && go test ./internal/notify/ -run "TestSendEmail" -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add hub/internal/notify/smtp.go hub/internal/notify/smtp_test.go
git commit -m "feat(notify): add SMTP email sender with TLS support"
```

---

## Task 6: Notifier Service

**Files:**
- Create: `hub/internal/notify/notifier.go`
- Create: `hub/internal/notify/notifier_test.go`

- [ ] **Step 1: Write failing test**

```go
package notify

import (
	"testing"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/model"
	"github.com/wyiu/aerodocs/hub/internal/store"
)

func testStoreAndUser(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	// Create a test user
	st.CreateUser(&model.User{
		ID: "user1", Username: "admin", Email: "admin@test.com",
		PasswordHash: "$2a$12$dummy", Role: model.RoleAdmin,
		TOTPEnabled: true,
	})
	return st
}

func TestNotifier_NoopWhenDisabled(t *testing.T) {
	st := testStoreAndUser(t)
	n := New(st)
	defer n.Close()

	// SMTP not configured — Notify should not panic or error
	n.Notify(model.NotifyAgentOffline, map[string]string{
		"server_name": "web-01",
		"server_id":   "srv-1",
		"timestamp":   "2026-03-29 12:00:00 UTC",
	})

	// Give worker time to process
	time.Sleep(100 * time.Millisecond)

	// No notifications should be logged (SMTP disabled)
	entries, total, _ := st.ListNotificationLog(50, 0)
	if total != 0 || len(entries) != 0 {
		t.Fatalf("expected 0 notifications when SMTP disabled, got %d", total)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd hub && go test ./internal/notify/ -run "TestNotifier" -v`
Expected: FAIL — `New` not defined

- [ ] **Step 3: Implement Notifier**

```go
package notify

import (
	"encoding/json"
	"log"

	"github.com/google/uuid"
	"github.com/wyiu/aerodocs/hub/internal/model"
	"github.com/wyiu/aerodocs/hub/internal/store"
)

type emailJob struct {
	To        string
	UserID    string
	EventType string
	Subject   string
	Body      string
}

// Notifier manages email notification delivery.
type Notifier struct {
	store *store.Store
	queue chan emailJob
	done  chan struct{}
}

// New creates a Notifier and starts its background worker.
func New(st *store.Store) *Notifier {
	n := &Notifier{
		store: st,
		queue: make(chan emailJob, 100),
		done:  make(chan struct{}),
	}
	go n.worker()
	return n
}

// Close stops the background worker and drains the queue.
func (n *Notifier) Close() {
	close(n.queue)
	<-n.done
}

// Notify enqueues email notifications for all users who have the given
// event type enabled. It reads SMTP config from the store; if SMTP is
// not configured or disabled, it returns immediately.
func (n *Notifier) Notify(eventType string, context map[string]string) {
	cfg := n.loadSMTPConfig()
	if !cfg.Enabled || cfg.Host == "" {
		return
	}

	recipients, err := n.store.GetEnabledRecipients(eventType)
	if err != nil {
		log.Printf("notify: failed to get recipients for %s: %v", eventType, err)
		return
	}
	if len(recipients) == 0 {
		return
	}

	subject, body := RenderEmail(eventType, context)

	for _, user := range recipients {
		select {
		case n.queue <- emailJob{
			To:        user.Email,
			UserID:    user.ID,
			EventType: eventType,
			Subject:   subject,
			Body:      body,
		}:
		default:
			log.Printf("notify: queue full, dropping notification for %s to %s", eventType, user.Email)
		}
	}
}

func (n *Notifier) worker() {
	defer close(n.done)
	for job := range n.queue {
		cfg := n.loadSMTPConfig()
		err := SendEmail(cfg, job.To, job.Subject, job.Body)

		status := "sent"
		var errMsg *string
		if err != nil {
			status = "failed"
			msg := err.Error()
			errMsg = &msg
			log.Printf("notify: failed to send %s to %s: %v", job.EventType, job.To, err)
		}

		if logErr := n.store.LogNotification(uuid.NewString(), job.UserID, job.EventType, job.Subject, status, errMsg); logErr != nil {
			log.Printf("notify: failed to log notification: %v", logErr)
		}
	}
}

func (n *Notifier) loadSMTPConfig() model.SMTPConfig {
	cfg := model.SMTPConfig{}
	cfg.Host, _ = n.store.GetConfig("smtp_host")
	portStr, _ := n.store.GetConfig("smtp_port")
	if portStr != "" {
		json.Unmarshal([]byte(portStr), &cfg.Port)
	}
	cfg.Username, _ = n.store.GetConfig("smtp_username")
	cfg.Password, _ = n.store.GetConfig("smtp_password")
	cfg.From, _ = n.store.GetConfig("smtp_from")
	tlsStr, _ := n.store.GetConfig("smtp_tls")
	cfg.TLS = tlsStr == "true"
	enabledStr, _ := n.store.GetConfig("smtp_enabled")
	cfg.Enabled = enabledStr == "true"
	return cfg
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd hub && go test ./internal/notify/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add hub/internal/notify/notifier.go hub/internal/notify/notifier_test.go
git commit -m "feat(notify): add Notifier service with background worker queue"
```

---

## Task 7: HTTP Handlers for Notifications

**Files:**
- Create: `hub/internal/server/handlers_notifications.go`
- Create: `hub/internal/server/handlers_notifications_test.go`
- Modify: `hub/internal/server/router.go` — add routes
- Modify: `hub/internal/server/server.go` — add notifier field

- [ ] **Step 1: Add notifier to Server**

In `hub/internal/server/server.go`, add to `Server` struct and `Config`:
```go
// In Server struct, after totpCache:
notifier    *notify.Notifier

// In Config struct, after LogSessions:
Notifier    *notify.Notifier
```

In `New()`, add:
```go
notifier:    cfg.Notifier,
```

Add import: `"github.com/wyiu/aerodocs/hub/internal/notify"`

- [ ] **Step 2: Write handlers**

```go
package server

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func (s *Server) handleGetSMTPConfig(w http.ResponseWriter, r *http.Request) {
	cfg := model.SMTPConfig{}
	cfg.Host, _ = s.store.GetConfig("smtp_host")
	portStr, _ := s.store.GetConfig("smtp_port")
	if portStr != "" {
		cfg.Port, _ = strconv.Atoi(portStr)
	}
	cfg.Username, _ = s.store.GetConfig("smtp_username")
	// Password is write-only — show masked version
	pw, _ := s.store.GetConfig("smtp_password")
	if pw != "" {
		cfg.Password = "********"
	}
	cfg.From, _ = s.store.GetConfig("smtp_from")
	tlsStr, _ := s.store.GetConfig("smtp_tls")
	cfg.TLS = tlsStr == "true"
	enabledStr, _ := s.store.GetConfig("smtp_enabled")
	cfg.Enabled = enabledStr == "true"

	respondJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleUpdateSMTPConfig(w http.ResponseWriter, r *http.Request) {
	var req model.SMTPConfig
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, errInvalidRequestBody)
		return
	}

	s.store.SetConfig("smtp_host", req.Host)
	s.store.SetConfig("smtp_port", strconv.Itoa(req.Port))
	s.store.SetConfig("smtp_username", req.Username)
	if req.Password != "" && req.Password != "********" {
		s.store.SetConfig("smtp_password", req.Password)
	}
	s.store.SetConfig("smtp_from", req.From)
	s.store.SetConfig("smtp_tls", fmt.Sprintf("%t", req.TLS))
	s.store.SetConfig("smtp_enabled", fmt.Sprintf("%t", req.Enabled))

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleTestSMTP(w http.ResponseWriter, r *http.Request) {
	var req model.SMTPTestRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, errInvalidRequestBody)
		return
	}

	if req.Recipient == "" {
		respondError(w, http.StatusBadRequest, "recipient email required")
		return
	}

	cfg := s.loadSMTPConfig()
	if !cfg.Enabled || cfg.Host == "" {
		respondError(w, http.StatusBadRequest, "SMTP is not configured or disabled")
		return
	}

	err := notify.SendEmail(cfg, req.Recipient, "[AeroDocs] Test Email", "This is a test email from AeroDocs to verify your SMTP configuration is working correctly.\n\n— AeroDocs")
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("SMTP send failed: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (s *Server) handleGetNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	prefs, err := s.store.GetNotificationPreferences(userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load preferences")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"preferences": prefs})
}

func (s *Server) handleUpdateNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	var req model.NotificationPreferencesRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, errInvalidRequestBody)
		return
	}
	for _, p := range req.Preferences {
		if err := s.store.SetNotificationPreference(userID, p.EventType, p.Enabled); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to update preferences")
			return
		}
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListNotificationLog(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		limit, _ = strconv.Atoi(v)
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		offset, _ = strconv.Atoi(v)
	}

	entries, total, err := s.store.ListNotificationLog(limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load notification log")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"total":   total,
	})
}

func (s *Server) loadSMTPConfig() model.SMTPConfig {
	cfg := model.SMTPConfig{}
	cfg.Host, _ = s.store.GetConfig("smtp_host")
	portStr, _ := s.store.GetConfig("smtp_port")
	if portStr != "" {
		cfg.Port, _ = strconv.Atoi(portStr)
	}
	cfg.Username, _ = s.store.GetConfig("smtp_username")
	cfg.Password, _ = s.store.GetConfig("smtp_password")
	cfg.From, _ = s.store.GetConfig("smtp_from")
	tlsStr, _ := s.store.GetConfig("smtp_tls")
	cfg.TLS = tlsStr == "true"
	enabledStr, _ := s.store.GetConfig("smtp_enabled")
	cfg.Enabled = enabledStr == "true"
	return cfg
}
```

Add import for `notify` package at the top (use the package's `SendEmail` function).

- [ ] **Step 3: Register routes in router.go**

After the audit-logs route in `hub/internal/server/router.go`, add:
```go
	// Notification endpoints
	mux.Handle("GET /api/settings/smtp", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleGetSMTPConfig)))))
	mux.Handle("PUT /api/settings/smtp", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleUpdateSMTPConfig)))))
	mux.Handle("POST /api/settings/smtp/test", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleTestSMTP)))))
	mux.Handle("GET /api/notifications/preferences", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(s.handleGetNotificationPreferences))))
	mux.Handle("PUT /api/notifications/preferences", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(s.handleUpdateNotificationPreferences))))
	mux.Handle("GET /api/notifications/log", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleListNotificationLog)))))
```

- [ ] **Step 4: Write handler tests**

Test SMTP config CRUD, preferences CRUD, and notification log listing. Use the existing `testServer` and `registerAndGetAdminToken` patterns from `handlers_auth_test.go`.

- [ ] **Step 5: Run all tests**

Run: `cd hub && go test ./... -count=1`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add hub/internal/server/handlers_notifications.go hub/internal/server/handlers_notifications_test.go hub/internal/server/router.go hub/internal/server/server.go
git commit -m "feat(notify): add HTTP handlers for SMTP config, preferences, and log"
```

---

## Task 8: Wire Notification Triggers

**Files:**
- Modify: `hub/internal/server/handlers_auth.go` — login_failed, password_changed, totp_changed
- Modify: `hub/internal/server/handlers_users.go` — user_created
- Modify: `hub/internal/server/handlers_upload.go` — file_uploaded
- Modify: `hub/internal/grpcserver/handler.go` — agent connect/disconnect
- Modify: `hub/internal/grpcserver/server.go` — accept Notifier
- Modify: `hub/cmd/aerodocs/main.go` — create and pass Notifier

- [ ] **Step 1: Add Notifier to gRPC Handler**

In `hub/internal/grpcserver/server.go`, add `Notifier *notify.Notifier` to Config and pass to Handler.

In `hub/internal/grpcserver/handler.go`:
- Add `notifier *notify.Notifier` field to Handler struct
- In the `Connect()` defer block (agent disconnect), after the audit log, add:
```go
if h.notifier != nil {
	serverName := serverID
	if srv, err := h.store.GetServerByID(serverID); err == nil {
		serverName = srv.Name
	}
	h.notifier.Notify(model.NotifyAgentOffline, map[string]string{
		"server_name": serverName,
		"server_id":   serverID,
		"timestamp":   time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
	})
}
```
- After successful `Register()` (agent connect), add similar call for `NotifyAgentOnline`.

- [ ] **Step 2: Add Notify calls to HTTP handlers**

In `handlers_auth.go`:
- After `AuditUserLoginFailed` log, add:
```go
if s.notifier != nil {
	s.notifier.Notify(model.NotifyLoginFailed, map[string]string{
		"username": req.Username, "ip": ip,
		"timestamp": time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
	})
}
```
- In `handleChangePassword` after success, add `NotifyPasswordChanged` call
- In `handleTOTPEnable`/`handleTOTPDisable` after success, add `NotifyTOTPChanged` call

In `handlers_users.go`:
- In `handleCreateUser` after success, add `NotifyUserCreated` call

In `handlers_upload.go`:
- In `handleUploadFile` after success, add `NotifyFileUploaded` call

- [ ] **Step 3: Wire Notifier in main.go**

In `hub/cmd/aerodocs/main.go`:
```go
import "github.com/wyiu/aerodocs/hub/internal/notify"

// After store initialization:
notifier := notify.New(st)
defer notifier.Close()

// Pass to server Config:
Notifier: notifier,

// Pass to grpcserver Config:
Notifier: notifier,
```

- [ ] **Step 4: Run all tests**

Run: `cd hub && go test ./... -count=1`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add hub/internal/server/handlers_auth.go hub/internal/server/handlers_users.go hub/internal/server/handlers_upload.go hub/internal/grpcserver/handler.go hub/internal/grpcserver/server.go hub/cmd/aerodocs/main.go
git commit -m "feat(notify): wire notification triggers into all event handlers"
```

---

## Task 9: Frontend — API Types

**Files:**
- Modify: `web/src/types/api.ts`

- [ ] **Step 1: Add notification types**

Add to `web/src/types/api.ts`:
```typescript
export interface SMTPConfig {
  host: string
  port: number
  username: string
  password: string
  from: string
  tls: boolean
  enabled: boolean
}

export interface NotificationPreference {
  event_type: string
  label: string
  category: string
  enabled: boolean
}

export interface NotificationPrefUpdate {
  event_type: string
  enabled: boolean
}

export interface NotificationLogEntry {
  id: string
  user_id: string
  username: string
  event_type: string
  subject: string
  status: 'sent' | 'failed'
  error: string | null
  created_at: string
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/types/api.ts
git commit -m "feat(notify): add notification TypeScript types"
```

---

## Task 10: Frontend — Admin Notifications Tab

**Files:**
- Create: `web/src/pages/settings-notifications-tab.tsx`
- Create: `web/src/pages/__tests__/settings-notifications-tab.test.tsx`

- [ ] **Step 1: Implement the admin Notifications tab**

This tab has two sections:
1. **SMTP Configuration** — form with host, port, username, password, from, TLS toggle, enabled toggle, test email button, save button
2. **Notification Log** — table showing recent notification deliveries with pagination

Follow the existing form patterns from `settings.tsx` (ProfileTab's change password form) for state management and mutation patterns.

Use `useQuery({ queryKey: ['smtp-config'], queryFn: () => apiFetch<SMTPConfig>('/settings/smtp') })` for loading.

Use `useMutation` for save and test email actions.

- [ ] **Step 2: Write tests**

Test that:
- SMTP form renders with fields
- Save button triggers PUT to `/settings/smtp`
- Test email button triggers POST to `/settings/smtp/test`
- Notification log table renders entries

- [ ] **Step 3: Run frontend tests**

Run: `cd web && npx vitest run src/pages/__tests__/settings-notifications-tab.test.tsx`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/settings-notifications-tab.tsx web/src/pages/__tests__/settings-notifications-tab.test.tsx
git commit -m "feat(notify): add admin SMTP config and notification log UI"
```

---

## Task 11: Frontend — User Preferences Tab

**Files:**
- Create: `web/src/pages/settings-preferences-tab.tsx`
- Create: `web/src/pages/__tests__/settings-preferences-tab.test.tsx`

- [ ] **Step 1: Implement user notification preferences tab**

Renders grouped toggles (Agent, Security, System categories) for each event type.
Uses `useQuery({ queryKey: ['notification-prefs'], queryFn: () => apiFetch<{preferences: NotificationPreference[]}>('/notifications/preferences') })`.
Save button sends PUT to `/notifications/preferences`.

- [ ] **Step 2: Write tests**

Test that toggles render, categories group correctly, and save triggers PUT.

- [ ] **Step 3: Run tests**

Run: `cd web && npx vitest run src/pages/__tests__/settings-preferences-tab.test.tsx`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/settings-preferences-tab.tsx web/src/pages/__tests__/settings-preferences-tab.test.tsx
git commit -m "feat(notify): add per-user notification preferences UI"
```

---

## Task 12: Frontend — Wire Tabs into Settings Page

**Files:**
- Modify: `web/src/pages/settings.tsx`

- [ ] **Step 1: Add Notifications and Alerts tabs**

Update the settings page to include:
- "Notifications" tab (admin only) — shows `NotificationsTab` component
- "Alerts" tab (all users) — shows `PreferencesTab` component

Update `activeTab` type to include `'notifications' | 'alerts'`.
Add tab buttons after the existing "Users" tab.
Import the new tab components.

- [ ] **Step 2: Run all frontend tests**

Run: `cd web && npx vitest run`
Expected: all PASS

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/settings.tsx
git commit -m "feat(notify): wire notification tabs into settings page"
```

---

## Task 13: Integration Test & Final Verification

- [ ] **Step 1: Run all backend tests**

Run: `cd hub && go test ./... -count=1`
Expected: all PASS

- [ ] **Step 2: Run all frontend tests**

Run: `cd web && npx vitest run`
Expected: all PASS

- [ ] **Step 3: Build frontend**

Run: `cd web && npx vite build`
Expected: BUILD SUCCESS

- [ ] **Step 4: Type check**

Run: `cd web && npx tsc --noEmit`
Expected: no errors

- [ ] **Step 5: Commit any remaining changes and push**

```bash
git push origin main
```

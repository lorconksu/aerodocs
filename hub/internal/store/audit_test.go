package store_test

import (
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func TestLogAndListAudit(t *testing.T) {
	s := testStore(t)

	// Create user first to satisfy FK constraint on audit_logs.user_id
	if err := s.CreateUser(&model.User{
		ID: "user-1", Username: "testuser", Email: "test@test.com",
		PasswordHash: "h", Role: model.RoleViewer,
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	userID := "user-1"
	ip := "127.0.0.1"
	if err := s.LogAudit(model.AuditEntry{
		ID:        "a1",
		UserID:    &userID,
		Action:    model.AuditUserLogin,
		IPAddress: &ip,
	}); err != nil {
		t.Fatalf("log audit: %v", err)
	}

	entries, total, err := s.ListAuditLogs(model.AuditFilter{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Action != model.AuditUserLogin {
		t.Fatalf("expected action %q, got %q", model.AuditUserLogin, entries[0].Action)
	}
}

func TestListAuditWithFilter(t *testing.T) {
	s := testStore(t)

	s.LogAudit(model.AuditEntry{ID: "a1", Action: model.AuditUserLogin})
	s.LogAudit(model.AuditEntry{ID: "a2", Action: model.AuditUserLoginFailed})
	s.LogAudit(model.AuditEntry{ID: "a3", Action: model.AuditUserLogin})

	action := model.AuditUserLogin
	entries, total, err := s.ListAuditLogs(model.AuditFilter{
		Action: &action,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

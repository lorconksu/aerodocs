package store_test

import (
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func TestCreateAndGetUser(t *testing.T) {
	s := testStore(t)

	user := &model.User{
		ID:           "test-uuid-1",
		Username:     "admin",
		Email:        "admin@test.com",
		PasswordHash: "$2a$12$fakehash",
		Role:         model.RoleAdmin,
	}

	if err := s.CreateUser(user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	got, err := s.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if got.ID != "test-uuid-1" {
		t.Fatalf("expected id 'test-uuid-1', got '%s'", got.ID)
	}
	if got.Role != model.RoleAdmin {
		t.Fatalf("expected role 'admin', got '%s'", got.Role)
	}
}

func TestCreateUser_DuplicateUsername(t *testing.T) {
	s := testStore(t)

	user := &model.User{
		ID: "u1", Username: "dup", Email: "a@a.com",
		PasswordHash: "hash", Role: model.RoleViewer,
	}
	if err := s.CreateUser(user); err != nil {
		t.Fatalf("first create: %v", err)
	}

	user2 := &model.User{
		ID: "u2", Username: "dup", Email: "b@b.com",
		PasswordHash: "hash", Role: model.RoleViewer,
	}
	if err := s.CreateUser(user2); err == nil {
		t.Fatal("expected error for duplicate username")
	}
}

func TestUserCount(t *testing.T) {
	s := testStore(t)

	count, err := s.UserCount()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 users, got %d", count)
	}

	s.CreateUser(&model.User{
		ID: "u1", Username: "a", Email: "a@a.com",
		PasswordHash: "h", Role: model.RoleAdmin,
	})

	count, _ = s.UserCount()
	if count != 1 {
		t.Fatalf("expected 1 user, got %d", count)
	}
}

func TestUpdateUserTOTP(t *testing.T) {
	s := testStore(t)

	s.CreateUser(&model.User{
		ID: "u1", Username: "a", Email: "a@a.com",
		PasswordHash: "h", Role: model.RoleAdmin,
	})

	secret := "JBSWY3DPEHPK3PXP"
	if err := s.UpdateUserTOTP("u1", &secret, true); err != nil {
		t.Fatalf("update totp: %v", err)
	}

	user, _ := s.GetUserByID("u1")
	if !user.TOTPEnabled {
		t.Fatal("expected totp_enabled = true")
	}
	if user.TOTPSecret == nil || *user.TOTPSecret != secret {
		t.Fatal("totp_secret not set correctly")
	}
}

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

func TestListUsers(t *testing.T) {
	s := testStore(t)

	s.CreateUser(&model.User{
		ID: "u1", Username: "alice", Email: "alice@test.com",
		PasswordHash: "h", Role: model.RoleAdmin,
	})
	s.CreateUser(&model.User{
		ID: "u2", Username: "bob", Email: "bob@test.com",
		PasswordHash: "h", Role: model.RoleViewer,
	})

	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

func TestDeleteUser(t *testing.T) {
	s := testStore(t)

	s.CreateUser(&model.User{
		ID: "u1", Username: "alice", Email: "alice@test.com",
		PasswordHash: "h", Role: model.RoleAdmin,
	})

	if err := s.DeleteUser("u1"); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	_, err := s.GetUserByID("u1")
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestDeleteUser_NotFound(t *testing.T) {
	s := testStore(t)

	err := s.DeleteUser("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}

func TestUpdateUserAvatar(t *testing.T) {
	s := testStore(t)

	s.CreateUser(&model.User{
		ID: "u1", Username: "alice", Email: "alice@test.com",
		PasswordHash: "h", Role: model.RoleAdmin,
	})

	avatar := "data:image/png;base64,abc123"
	if err := s.UpdateUserAvatar("u1", &avatar); err != nil {
		t.Fatalf("update avatar: %v", err)
	}

	user, err := s.GetUserByID("u1")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if user.Avatar == nil || *user.Avatar != avatar {
		t.Fatalf("expected avatar '%s', got '%v'", avatar, user.Avatar)
	}
}

func TestUpdateUserAvatar_ClearAvatar(t *testing.T) {
	s := testStore(t)

	s.CreateUser(&model.User{
		ID: "u1", Username: "alice", Email: "alice@test.com",
		PasswordHash: "h", Role: model.RoleAdmin,
	})

	// Set avatar
	avatar := "data:image/png;base64,abc123"
	s.UpdateUserAvatar("u1", &avatar)

	// Clear avatar
	if err := s.UpdateUserAvatar("u1", nil); err != nil {
		t.Fatalf("clear avatar: %v", err)
	}

	user, _ := s.GetUserByID("u1")
	if user.Avatar != nil {
		t.Fatalf("expected nil avatar after clear, got '%v'", user.Avatar)
	}
}

func TestUpdateUserPassword(t *testing.T) {
	s := testStore(t)

	s.CreateUser(&model.User{
		ID: "u1", Username: "alice", Email: "alice@test.com",
		PasswordHash: "oldhash", Role: model.RoleAdmin,
	})

	if err := s.UpdateUserPassword("u1", "newhash"); err != nil {
		t.Fatalf("update password: %v", err)
	}

	user, _ := s.GetUserByID("u1")
	if user.PasswordHash != "newhash" {
		t.Fatalf("expected 'newhash', got '%s'", user.PasswordHash)
	}
}

func TestGetUserByID_NotFound(t *testing.T) {
	s := testStore(t)

	_, err := s.GetUserByID("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent user ID")
	}
}

func TestGetUserByUsername_NotFound(t *testing.T) {
	s := testStore(t)

	_, err := s.GetUserByUsername("nobody")
	if err == nil {
		t.Fatal("expected error for nonexistent username")
	}
}

func TestUpdateUserTOTP_Clear(t *testing.T) {
	s := testStore(t)

	s.CreateUser(&model.User{
		ID: "u1", Username: "alice", Email: "alice@test.com",
		PasswordHash: "h", Role: model.RoleAdmin,
	})

	secret := "JBSWY3DPEHPK3PXP"
	s.UpdateUserTOTP("u1", &secret, true)

	// Clear TOTP
	if err := s.UpdateUserTOTP("u1", nil, false); err != nil {
		t.Fatalf("clear totp: %v", err)
	}

	user, _ := s.GetUserByID("u1")
	if user.TOTPEnabled {
		t.Fatal("expected totp_enabled=false after clear")
	}
	if user.TOTPSecret != nil {
		t.Fatal("expected nil totp_secret after clear")
	}
}

func TestListUsers_Empty(t *testing.T) {
	s := testStore(t)

	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("list empty users: %v", err)
	}
	if users != nil && len(users) != 0 {
		t.Fatalf("expected 0 users, got %d", len(users))
	}
}

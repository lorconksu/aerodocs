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

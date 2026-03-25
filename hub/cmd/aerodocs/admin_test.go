package main

import (
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/wyiu/aerodocs/hub/internal/auth"
	"github.com/wyiu/aerodocs/hub/internal/model"
	"github.com/wyiu/aerodocs/hub/internal/store"
)

// TestRunAdmin_NoArgs verifies that runAdmin with no arguments returns a usage error.
func TestRunAdmin_NoArgs(t *testing.T) {
	err := runAdmin([]string{})
	if err == nil {
		t.Fatal("expected error for no args")
	}
}

// TestRunAdmin_UnknownCommand verifies that runAdmin with an unknown command returns an error.
func TestRunAdmin_UnknownCommand(t *testing.T) {
	err := runAdmin([]string{"unknown-cmd"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

// TestRunAdmin_ResetTOTP_NoUsername verifies that reset-totp without --username returns an error.
func TestRunResetTOTP_NoUsername(t *testing.T) {
	err := runResetTOTP([]string{})
	if err == nil {
		t.Fatal("expected error when --username is missing")
	}
}

// TestRunResetTOTP_BadDB verifies that reset-totp with an invalid db path returns an error.
func TestRunResetTOTP_BadDB(t *testing.T) {
	err := runResetTOTP([]string{"--username", "testuser", "--db", "/dev/null/nonexistent/path.db"})
	if err == nil {
		t.Fatal("expected error for bad db path")
	}
}

// TestRunResetTOTP_UserNotFound verifies that reset-totp with a nonexistent user errors.
func TestRunResetTOTP_UserNotFound(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Create and immediately close a valid DB (so it has migrations applied).
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	st.Close()

	err = runResetTOTP([]string{"--username", "nonexistentuser", "--db", dbPath})
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}

// TestRunResetTOTP_Success verifies reset-totp completes successfully with a real user.
func TestRunResetTOTP_Success(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Create store and seed a user.
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	hash, err := auth.HashPassword("MyP@ssw0rd!234")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := &model.User{
		ID:           uuid.NewString(),
		Username:     "testadmin",
		Email:        "testadmin@test.com",
		PasswordHash: hash,
		Role:         model.RoleAdmin,
	}
	if err := st.CreateUser(user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	st.Close()

	// Now call reset-totp — should succeed.
	err = runResetTOTP([]string{"--username", "testadmin", "--db", dbPath})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

// TestRunAdmin_ResetTOTP_Dispatches verifies that runAdmin("reset-totp") dispatches correctly.
func TestRunAdmin_ResetTOTP_Dispatches(t *testing.T) {
	// Passing "reset-totp" with no username should give --username required error
	err := runAdmin([]string{"reset-totp"})
	if err == nil {
		t.Fatal("expected error when dispatching reset-totp with no args")
	}
}

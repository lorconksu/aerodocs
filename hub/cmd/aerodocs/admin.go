package main

import (
	"flag"
	"fmt"

	"github.com/wyiu/aerodocs/hub/internal/auth"
	"github.com/wyiu/aerodocs/hub/internal/store"
)

func runAdmin(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: aerodocs admin <command>\ncommands: reset-totp")
	}

	switch args[0] {
	case "reset-totp":
		return runResetTOTP(args[1:])
	default:
		return fmt.Errorf("unknown admin command: %s", args[0])
	}
}

func runResetTOTP(args []string) error {
	fs := flag.NewFlagSet("reset-totp", flag.ExitOnError)
	username := fs.String("username", "", "username to reset TOTP for")
	dbPath := fs.String("db", "aerodocs.db", "SQLite database path")
	fs.Parse(args)

	if *username == "" {
		return fmt.Errorf("--username is required")
	}

	st, err := store.New(*dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer st.Close()

	user, err := st.GetUserByUsername(*username)
	if err != nil {
		return fmt.Errorf("user %q not found", *username)
	}

	// Generate new temporary password
	tempPassword := auth.GenerateTemporaryPassword()
	hash, err := auth.HashPassword(tempPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	// Reset TOTP and password
	if err := st.UpdateUserTOTP(user.ID, nil, false); err != nil {
		return fmt.Errorf("reset TOTP: %w", err)
	}
	if err := st.UpdateUserPassword(user.ID, hash); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	fmt.Printf("TOTP reset for user %q\n", *username)
	fmt.Printf("Temporary password: %s\n", tempPassword)
	fmt.Println("User must set up TOTP again on next login.")

	return nil
}

package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func (s *Store) CreateUser(u *model.User) error {
	_, err := s.db.Exec(
		`INSERT INTO users (id, username, email, password_hash, role, totp_secret, totp_enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Username, u.Email, u.PasswordHash, u.Role, u.TOTPSecret, u.TOTPEnabled,
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (s *Store) GetUserByID(id string) (*model.User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, username, email, password_hash, role, totp_secret, totp_enabled, created_at, updated_at
		 FROM users WHERE id = ?`, id,
	))
}

func (s *Store) GetUserByUsername(username string) (*model.User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, username, email, password_hash, role, totp_secret, totp_enabled, created_at, updated_at
		 FROM users WHERE username = ?`, username,
	))
}

func (s *Store) UserCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

func (s *Store) UpdateUserTOTP(userID string, secret *string, enabled bool) error {
	_, err := s.db.Exec(
		"UPDATE users SET totp_secret = ?, totp_enabled = ?, updated_at = datetime('now') WHERE id = ?",
		secret, enabled, userID,
	)
	if err != nil {
		return fmt.Errorf("update totp: %w", err)
	}
	return nil
}

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

func (s *Store) UpdateUserPassword(userID, passwordHash string) error {
	_, err := s.db.Exec(
		"UPDATE users SET password_hash = ?, updated_at = datetime('now') WHERE id = ?",
		passwordHash, userID,
	)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

func (s *Store) ListUsers() ([]model.User, error) {
	rows, err := s.db.Query(
		`SELECT id, username, email, password_hash, role, totp_secret, totp_enabled, created_at, updated_at
		 FROM users ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
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

func (s *Store) scanUser(row *sql.Row) (*model.User, error) {
	var u model.User
	var createdAt, updatedAt string
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role,
		&u.TOTPSecret, &u.TOTPEnabled, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	u.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &u, nil
}

func (s *Store) scanUserRow(rows *sql.Rows) (*model.User, error) {
	var u model.User
	var createdAt, updatedAt string
	err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role,
		&u.TOTPSecret, &u.TOTPEnabled, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan user row: %w", err)
	}
	u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	u.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &u, nil
}

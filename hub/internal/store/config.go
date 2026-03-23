package store

import (
	"database/sql"
	"fmt"
)

func (s *Store) GetConfig(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM _config WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("config key %q not found", key)
	}
	if err != nil {
		return "", fmt.Errorf("get config %q: %w", key, err)
	}
	return value, nil
}

func (s *Store) SetConfig(key, value string) error {
	_, err := s.db.Exec(
		"INSERT INTO _config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set config %q: %w", key, err)
	}
	return nil
}

package migrate

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func Run(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS _migrations (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		filename   TEXT NOT NULL UNIQUE,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return fmt.Errorf("create _migrations table: %w", err)
	}

	applied := make(map[string]bool)
	rows, err := db.Query("SELECT filename FROM _migrations")
	if err != nil {
		return fmt.Errorf("query _migrations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return fmt.Errorf("scan migration: %w", err)
		}
		applied[f] = true
	}

	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") || applied[entry.Name()] {
			continue
		}
		if err := applyMigration(db, entry.Name()); err != nil {
			return err
		}
	}

	return nil
}

func applyMigration(db *sql.DB, filename string) error {
	content, err := fs.ReadFile(migrationFS, "migrations/"+filename)
	if err != nil {
		return fmt.Errorf("read %s: %w", filename, err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx for %s: %w", filename, err)
	}

	if _, err := tx.Exec(string(content)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("execute %s: %w", filename, err)
	}

	if _, err := tx.Exec("INSERT INTO _migrations (filename) VALUES (?)", filename); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record %s: %w", filename, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit %s: %w", filename, err)
	}
	return nil
}

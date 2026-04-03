package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/migrate"
	_ "modernc.org/sqlite" // registers the SQLite driver
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	if dbPath == ":memory:" {
		dbPath = fmt.Sprintf("file:aerodocs-%d?mode=memory&cache=shared", time.Now().UnixNano())
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// SQLite performance and safety PRAGMAs
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",  // safe with WAL; reduces fsync from every write to checkpoint
		"PRAGMA busy_timeout=5000",   // wait up to 5s on lock contention instead of immediate BUSY error
		"PRAGMA cache_size=-20000",   // 20MB page cache (negative = KB)
		"PRAGMA mmap_size=268435456", // 256MB memory-mapped I/O for read performance
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("exec %s: %w", p, err)
		}
	}

	// Run migrations
	if err := migrate.Run(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

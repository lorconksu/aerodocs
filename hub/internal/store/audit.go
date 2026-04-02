package store

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func (s *Store) LogAudit(entry model.AuditEntry) error {
	// Use BEGIN IMMEDIATE to acquire a write lock upfront, preventing two
	// concurrent goroutines from reading the same prev_hash before inserting.
	tx, err := s.db.Exec("BEGIN IMMEDIATE")
	_ = tx // driver returns a result but we only care about errors
	if err != nil {
		return fmt.Errorf("begin immediate: %w", err)
	}

	// Compute integrity hash chain: SHA-256(prev_hash + current entry fields)
	var prevHash string
	_ = s.db.QueryRow("SELECT prev_hash FROM audit_logs ORDER BY rowid DESC LIMIT 1").Scan(&prevHash)

	hashInput := fmt.Sprintf("%s|%s|%v|%s|%v|%v|%v",
		prevHash, entry.ID, entry.UserID, entry.Action, entry.Target, entry.Detail, entry.IPAddress)
	hash := sha256.Sum256([]byte(hashInput))
	entry.PrevHash = fmt.Sprintf("%x", hash[:])

	_, err = s.db.Exec(
		`INSERT INTO audit_logs (id, user_id, action, target, detail, ip_address, prev_hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.UserID, entry.Action, entry.Target, entry.Detail, entry.IPAddress, entry.PrevHash,
	)
	if err != nil {
		s.db.Exec("ROLLBACK")
		return fmt.Errorf("log audit: %w", err)
	}

	if _, err := s.db.Exec("COMMIT"); err != nil {
		s.db.Exec("ROLLBACK")
		return fmt.Errorf("commit audit: %w", err)
	}
	return nil
}

func (s *Store) ListAuditLogs(filter model.AuditFilter) ([]model.AuditEntry, int, error) {
	qb := newQueryBuilder("SELECT id, user_id, action, target, detail, ip_address, prev_hash, created_at FROM audit_logs")

	if filter.UserID != nil {
		qb.Where("user_id = ?", *filter.UserID)
	}
	if filter.Action != nil {
		qb.Where("action = ?", *filter.Action)
	}
	if filter.From != nil {
		qb.Where("created_at >= ?", *filter.From)
	}
	if filter.To != nil {
		qb.Where("created_at <= ?", *filter.To)
	}

	// Get total count
	var total int
	countQuery, countArgs := qb.CountQuery("audit_logs")
	if err := s.db.QueryRow(countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	// Get paginated results
	qb.OrderBy("created_at DESC")
	if filter.Limit > 0 {
		qb.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		qb.Offset(filter.Offset)
	}
	query, args := qb.Build()

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	entries, err := scanAuditRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return entries, total, rows.Err()
}

// scanAuditRows scans all audit log rows into a slice.
func scanAuditRows(rows interface{ Next() bool; Scan(...interface{}) error; Err() error }) ([]model.AuditEntry, error) {
	var entries []model.AuditEntry
	for rows.Next() {
		var e model.AuditEntry
		var createdAt string
		if err := rows.Scan(&e.ID, &e.UserID, &e.Action, &e.Target, &e.Detail, &e.IPAddress, &e.PrevHash, &createdAt); err != nil {
			return nil, fmt.Errorf("scan audit entry: %w", err)
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		entries = append(entries, e)
	}
	return entries, nil
}

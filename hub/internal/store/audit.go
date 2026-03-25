package store

import (
	"fmt"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func (s *Store) LogAudit(entry model.AuditEntry) error {
	_, err := s.db.Exec(
		`INSERT INTO audit_logs (id, user_id, action, target, detail, ip_address)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.UserID, entry.Action, entry.Target, entry.Detail, entry.IPAddress,
	)
	if err != nil {
		return fmt.Errorf("log audit: %w", err)
	}
	return nil
}

func (s *Store) ListAuditLogs(filter model.AuditFilter) ([]model.AuditEntry, int, error) {
	qb := newQueryBuilder("SELECT id, user_id, action, target, detail, ip_address, created_at FROM audit_logs")

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

	var entries []model.AuditEntry
	for rows.Next() {
		var e model.AuditEntry
		var createdAt string
		if err := rows.Scan(&e.ID, &e.UserID, &e.Action, &e.Target, &e.Detail, &e.IPAddress, &createdAt); err != nil {
			return nil, 0, fmt.Errorf("scan audit entry: %w", err)
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		entries = append(entries, e)
	}

	return entries, total, rows.Err()
}

package store

import (
	"fmt"
	"strings"
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
	var where []string
	var args []interface{}

	if filter.UserID != nil {
		where = append(where, "user_id = ?")
		args = append(args, *filter.UserID)
	}
	if filter.Action != nil {
		where = append(where, "action = ?")
		args = append(args, *filter.Action)
	}
	if filter.From != nil {
		where = append(where, "created_at >= ?")
		args = append(args, *filter.From)
	}
	if filter.To != nil {
		where = append(where, "created_at <= ?")
		args = append(args, *filter.To)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	// Get total count
	var total int
	countQuery := "SELECT COUNT(*) FROM audit_logs" + whereClause
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	// Get paginated results
	query := "SELECT id, user_id, action, target, detail, ip_address, created_at FROM audit_logs" +
		whereClause + " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

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

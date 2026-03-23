package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func (s *Store) CreateServer(srv *model.Server) error {
	_, err := s.db.Exec(
		`INSERT INTO servers (id, name, hostname, ip_address, os, status, registration_token, token_expires_at, agent_version, labels)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		srv.ID, srv.Name, srv.Hostname, srv.IPAddress, srv.OS, srv.Status,
		srv.RegistrationToken, srv.TokenExpiresAt, srv.AgentVersion, srv.Labels,
	)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}
	return nil
}

func (s *Store) GetServerByID(id string) (*model.Server, error) {
	return s.scanServer(s.db.QueryRow(
		`SELECT id, name, hostname, ip_address, os, status, registration_token, token_expires_at,
		        agent_version, labels, last_seen_at, created_at, updated_at
		 FROM servers WHERE id = ?`, id,
	))
}

func (s *Store) ListServers(filter model.ServerFilter) ([]model.Server, int, error) {
	var where []string
	var args []interface{}

	if filter.Status != nil {
		where = append(where, "status = ?")
		args = append(args, *filter.Status)
	}
	if filter.Search != nil {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+*filter.Search+"%")
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	// Get total count
	var total int
	countQuery := "SELECT COUNT(*) FROM servers" + whereClause
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count servers: %w", err)
	}

	// Get paginated results
	query := `SELECT id, name, hostname, ip_address, os, status, registration_token, token_expires_at,
	                 agent_version, labels, last_seen_at, created_at, updated_at
	          FROM servers` + whereClause + " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query servers: %w", err)
	}
	defer rows.Close()

	var servers []model.Server
	for rows.Next() {
		srv, err := s.scanServerRow(rows)
		if err != nil {
			return nil, 0, err
		}
		servers = append(servers, *srv)
	}

	return servers, total, rows.Err()
}

func (s *Store) ListServersForUser(userID string, filter model.ServerFilter) ([]model.Server, int, error) {
	var where []string
	var args []interface{}

	// JOIN with permissions table to restrict to user's servers
	joinClause := " INNER JOIN permissions p ON servers.id = p.server_id AND p.user_id = ?"
	args = append(args, userID)

	if filter.Status != nil {
		where = append(where, "servers.status = ?")
		args = append(args, *filter.Status)
	}
	if filter.Search != nil {
		where = append(where, "servers.name LIKE ?")
		args = append(args, "%"+*filter.Search+"%")
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	// Get total count
	var total int
	countQuery := "SELECT COUNT(DISTINCT servers.id) FROM servers" + joinClause + whereClause
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count servers for user: %w", err)
	}

	// Get paginated results
	query := `SELECT DISTINCT servers.id, servers.name, servers.hostname, servers.ip_address,
	                 servers.os, servers.status, servers.registration_token, servers.token_expires_at,
	                 servers.agent_version, servers.labels, servers.last_seen_at,
	                 servers.created_at, servers.updated_at
	          FROM servers` + joinClause + whereClause + " ORDER BY servers.created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query servers for user: %w", err)
	}
	defer rows.Close()

	var servers []model.Server
	for rows.Next() {
		srv, err := s.scanServerRow(rows)
		if err != nil {
			return nil, 0, err
		}
		servers = append(servers, *srv)
	}

	return servers, total, rows.Err()
}

func (s *Store) UpdateServer(id, name, labels string) error {
	result, err := s.db.Exec(
		"UPDATE servers SET name = ?, labels = ?, updated_at = datetime('now') WHERE id = ?",
		name, labels, id,
	)
	if err != nil {
		return fmt.Errorf("update server: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("server not found")
	}
	return nil
}

func (s *Store) DeleteServer(id string) error {
	result, err := s.db.Exec("DELETE FROM servers WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete server: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("server not found")
	}
	return nil
}

func (s *Store) DeleteServers(ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := "DELETE FROM servers WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	_, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("batch delete servers: %w", err)
	}
	return nil
}

func (s *Store) GetServerByToken(tokenHash string) (*model.Server, error) {
	return s.scanServer(s.db.QueryRow(
		`SELECT id, name, hostname, ip_address, os, status, registration_token, token_expires_at,
		        agent_version, labels, last_seen_at, created_at, updated_at
		 FROM servers WHERE registration_token = ?`, tokenHash,
	))
}

func (s *Store) ActivateServer(id, hostname, ip, os, agentVersion string) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	result, err := s.db.Exec(
		`UPDATE servers
		 SET hostname = ?, ip_address = ?, os = ?, agent_version = ?,
		     status = 'online', registration_token = NULL, token_expires_at = NULL,
		     last_seen_at = ?, updated_at = datetime('now')
		 WHERE id = ?`,
		hostname, ip, os, agentVersion, now, id,
	)
	if err != nil {
		return fmt.Errorf("activate server: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("server not found")
	}
	return nil
}

func (s *Store) scanServer(row *sql.Row) (*model.Server, error) {
	var srv model.Server
	var createdAt, updatedAt string
	err := row.Scan(
		&srv.ID, &srv.Name, &srv.Hostname, &srv.IPAddress, &srv.OS, &srv.Status,
		&srv.RegistrationToken, &srv.TokenExpiresAt,
		&srv.AgentVersion, &srv.Labels, &srv.LastSeenAt, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("server not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan server: %w", err)
	}
	srv.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	srv.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &srv, nil
}

func (s *Store) scanServerRow(rows *sql.Rows) (*model.Server, error) {
	var srv model.Server
	var createdAt, updatedAt string
	err := rows.Scan(
		&srv.ID, &srv.Name, &srv.Hostname, &srv.IPAddress, &srv.OS, &srv.Status,
		&srv.RegistrationToken, &srv.TokenExpiresAt,
		&srv.AgentVersion, &srv.Labels, &srv.LastSeenAt, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan server row: %w", err)
	}
	srv.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	srv.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &srv, nil
}

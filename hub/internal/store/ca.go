package store

import (
	"database/sql"
	"fmt"
)

func (s *Store) SaveCA(certDER, keyEncrypted []byte) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO ca_config (id, ca_cert, ca_key_encrypted) VALUES ('default', ?, ?)`,
		certDER, keyEncrypted,
	)
	return err
}

func (s *Store) LoadCA() (certDER, keyEncrypted []byte, err error) {
	err = s.db.QueryRow(`SELECT ca_cert, ca_key_encrypted FROM ca_config WHERE id = 'default'`).
		Scan(&certDER, &keyEncrypted)
	if err == sql.ErrNoRows {
		return nil, nil, fmt.Errorf("CA not initialized")
	}
	return certDER, keyEncrypted, err
}

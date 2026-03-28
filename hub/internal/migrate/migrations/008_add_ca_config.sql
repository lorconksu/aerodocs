CREATE TABLE IF NOT EXISTS ca_config (
    id TEXT PRIMARY KEY DEFAULT 'default',
    ca_cert BLOB NOT NULL,
    ca_key_encrypted BLOB NOT NULL,
    created_at DATETIME DEFAULT (datetime('now'))
);

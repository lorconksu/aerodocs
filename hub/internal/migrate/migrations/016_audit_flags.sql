CREATE TABLE IF NOT EXISTS audit_flags (
    id           TEXT PRIMARY KEY,
    entry_id     TEXT,
    created_by   TEXT NOT NULL,
    filters_json TEXT NOT NULL DEFAULT '{}',
    note         TEXT NOT NULL,
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (entry_id) REFERENCES audit_logs(id) ON DELETE SET NULL,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_audit_flags_created_at ON audit_flags(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_flags_created_by ON audit_flags(created_by);

-- codex: no-transaction
PRAGMA foreign_keys=OFF;

CREATE TABLE users_new (
    id                      TEXT PRIMARY KEY,
    username                TEXT NOT NULL UNIQUE,
    email                   TEXT NOT NULL UNIQUE,
    password_hash           TEXT NOT NULL,
    role                    TEXT NOT NULL DEFAULT 'viewer' CHECK(role IN ('admin', 'auditor', 'viewer')),
    totp_secret             TEXT,
    totp_enabled            INTEGER NOT NULL DEFAULT 0,
    avatar                  TEXT,
    token_generation        INTEGER NOT NULL DEFAULT 0,
    must_change_password    INTEGER NOT NULL DEFAULT 0,
    temp_password_expires_at TEXT,
    created_at              TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at              TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO users_new (
    id, username, email, password_hash, role, totp_secret, totp_enabled, avatar,
    token_generation, must_change_password, temp_password_expires_at, created_at, updated_at
)
SELECT
    id, username, email, password_hash, role, totp_secret, totp_enabled, avatar,
    token_generation, 0, NULL, created_at, updated_at
FROM users;

DROP TABLE users;
ALTER TABLE users_new RENAME TO users;

CREATE TABLE audit_logs_new (
    id             TEXT PRIMARY KEY,
    user_id        TEXT,
    action         TEXT NOT NULL,
    target         TEXT,
    detail         TEXT,
    ip_address     TEXT,
    outcome        TEXT NOT NULL DEFAULT '',
    actor_type     TEXT NOT NULL DEFAULT '',
    correlation_id TEXT,
    resource_type  TEXT,
    prev_hash      TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

INSERT INTO audit_logs_new (
    id, user_id, action, target, detail, ip_address, outcome, actor_type,
    correlation_id, resource_type, prev_hash, created_at
)
SELECT
    id,
    user_id,
    action,
    target,
    detail,
    ip_address,
    CASE
        WHEN action LIKE '%_failed' THEN 'failure'
        ELSE 'success'
    END,
    CASE
        WHEN user_id IS NOT NULL THEN 'user'
        ELSE 'system'
    END,
    NULL,
    CASE
        WHEN action LIKE 'user.%' THEN 'user'
        WHEN action LIKE 'server.%' THEN 'server'
        WHEN action LIKE 'file.%' THEN 'file'
        WHEN action LIKE 'path.%' THEN 'path'
        WHEN action LIKE 'log.%' THEN 'log'
        ELSE 'system'
    END,
    prev_hash,
    created_at
FROM audit_logs;

DROP TABLE audit_logs;
ALTER TABLE audit_logs_new RENAME TO audit_logs;

CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX idx_audit_logs_outcome ON audit_logs(outcome);
CREATE INDEX idx_audit_logs_correlation_id ON audit_logs(correlation_id);
CREATE INDEX idx_audit_logs_resource_type ON audit_logs(resource_type);

CREATE TABLE IF NOT EXISTS password_history (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_password_history_user_id ON password_history(user_id);
CREATE INDEX IF NOT EXISTS idx_password_history_created_at ON password_history(created_at);

INSERT INTO password_history (id, user_id, password_hash, created_at)
SELECT 'history-' || id, id, password_hash, created_at
FROM users
WHERE password_hash <> '';

CREATE TABLE IF NOT EXISTS audit_saved_filters (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    created_by   TEXT NOT NULL,
    filters_json TEXT NOT NULL,
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at   TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_audit_saved_filters_created_by ON audit_saved_filters(created_by);

CREATE TABLE IF NOT EXISTS audit_reviews (
    id           TEXT PRIMARY KEY,
    reviewer_id  TEXT NOT NULL,
    filters_json TEXT NOT NULL,
    notes        TEXT NOT NULL DEFAULT '',
    from_time    TEXT,
    to_time      TEXT,
    completed_at TEXT NOT NULL DEFAULT (datetime('now')),
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (reviewer_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_audit_reviews_reviewer_id ON audit_reviews(reviewer_id);
CREATE INDEX IF NOT EXISTS idx_audit_reviews_completed_at ON audit_reviews(completed_at);

PRAGMA foreign_keys=ON;

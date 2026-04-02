CREATE TABLE IF NOT EXISTS token_blacklist (
    jti        TEXT PRIMARY KEY,
    expires_at TEXT NOT NULL
);

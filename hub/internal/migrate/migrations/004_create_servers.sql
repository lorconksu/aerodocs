CREATE TABLE servers (
    id                 TEXT PRIMARY KEY,
    name               TEXT NOT NULL,
    hostname           TEXT,
    ip_address         TEXT,
    os                 TEXT,
    status             TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','online','offline')),
    registration_token TEXT UNIQUE,
    token_expires_at   TEXT,
    agent_version      TEXT,
    labels             TEXT DEFAULT '{}',
    last_seen_at       TEXT,
    created_at         TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at         TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE permissions (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    server_id  TEXT NOT NULL,
    path       TEXT NOT NULL DEFAULT '/',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE,
    UNIQUE(user_id, server_id, path)
);

CREATE INDEX idx_permissions_user_id ON permissions(user_id);
CREATE INDEX idx_permissions_server_id ON permissions(server_id);

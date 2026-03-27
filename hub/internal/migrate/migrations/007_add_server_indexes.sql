CREATE INDEX idx_servers_status ON servers (status);
CREATE INDEX idx_servers_created_at ON servers (created_at DESC);
CREATE INDEX idx_servers_status_created_at ON servers (status, created_at DESC);

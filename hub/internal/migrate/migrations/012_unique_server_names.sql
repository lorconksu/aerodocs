-- Add UNIQUE constraint to server names to prevent duplicates.
CREATE UNIQUE INDEX IF NOT EXISTS idx_servers_name_unique ON servers(name);

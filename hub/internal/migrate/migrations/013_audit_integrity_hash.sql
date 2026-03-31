-- Add integrity hash chain to audit logs for tamper detection.
ALTER TABLE audit_logs ADD COLUMN prev_hash TEXT NOT NULL DEFAULT '';

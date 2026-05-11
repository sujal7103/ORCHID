-- ClaraVerse MySQL Schema Migration
-- Date: 2026-01-25
-- Purpose: Add missing columns to mcp_connections table for client tracking

-- Add client_version column
ALTER TABLE mcp_connections
ADD COLUMN client_version VARCHAR(50) COMMENT 'MCP client version string' AFTER client_id;

-- Add platform column
ALTER TABLE mcp_connections
ADD COLUMN platform VARCHAR(50) COMMENT 'Client operating system (darwin, linux, windows)' AFTER client_version;

-- Add last_heartbeat column
ALTER TABLE mcp_connections
ADD COLUMN last_heartbeat TIMESTAMP NULL COMMENT 'Last heartbeat received from client' AFTER connected_at;

-- Add index for heartbeat monitoring (find stale connections)
ALTER TABLE mcp_connections
ADD INDEX idx_heartbeat (last_heartbeat);

-- Update schema version
INSERT INTO schema_version (version, description) VALUES
    (3, 'Add client_version, platform, last_heartbeat to mcp_connections');

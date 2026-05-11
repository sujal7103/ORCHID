-- ClaraVerse MySQL Schema Migration
-- Date: 2026-01-26
-- Purpose: Add device_tokens table for device authorization (OAuth 2.0 Device Authorization Grant)

-- Device tokens table for fast revocation checks
-- Main device data is stored in MongoDB, this is a cache for fast token validation
CREATE TABLE IF NOT EXISTS device_tokens (
    device_id VARCHAR(36) PRIMARY KEY COMMENT 'Device UUID',
    user_id VARCHAR(255) NOT NULL COMMENT 'Supabase user ID',
    token_hash VARCHAR(64) NOT NULL COMMENT 'SHA-256 hash of access token prefix',
    is_revoked BOOLEAN DEFAULT FALSE COMMENT 'Whether device has been revoked',
    revoked_at TIMESTAMP NULL COMMENT 'When device was revoked',
    expires_at TIMESTAMP NOT NULL COMMENT 'When current access token expires',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    INDEX idx_user_active (user_id, is_revoked),
    INDEX idx_token_lookup (token_hash, is_revoked),
    INDEX idx_expiry (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
COMMENT='Device token validation cache for fast revocation checks';

-- Update schema version
INSERT INTO schema_version (version, description) VALUES
    (4, 'Add device_tokens table for device authorization');

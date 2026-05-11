-- Migration: Convert from per-provider tiers to 5 global tiers
-- This migration updates the recommended_models table to support a global tier system
-- where only 5 models (one per tier) can be recommended across all providers.

-- Step 1: Drop foreign key constraint (required before dropping index)
ALTER TABLE recommended_models
  DROP FOREIGN KEY recommended_models_ibfk_1;

-- Step 2: Drop old per-provider unique constraint
ALTER TABLE recommended_models
  DROP INDEX unique_provider_tier;

-- Step 3: Clear existing tier data (will need to be reassigned via admin UI)
TRUNCATE TABLE recommended_models;

-- Step 4: Update tier enum to use tier1-tier5 naming
ALTER TABLE recommended_models
  MODIFY COLUMN tier ENUM('tier1', 'tier2', 'tier3', 'tier4', 'tier5') NOT NULL
  COMMENT 'Global tier assignment';

-- Step 5: Add new global tier uniqueness constraint
-- This ensures only ONE model can occupy each of the 5 global tier slots
ALTER TABLE recommended_models
  ADD UNIQUE KEY unique_global_tier (tier);

-- Step 6: Re-add index on provider_id for foreign key
ALTER TABLE recommended_models
  ADD KEY idx_provider (provider_id);

-- Step 7: Recreate foreign key constraint
ALTER TABLE recommended_models
  ADD CONSTRAINT recommended_models_ibfk_1
  FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE;

-- Step 8: Create tier_labels table for UI display customization
CREATE TABLE IF NOT EXISTS tier_labels (
    tier ENUM('tier1', 'tier2', 'tier3', 'tier4', 'tier5') PRIMARY KEY
      COMMENT 'Tier identifier',
    label VARCHAR(50) NOT NULL
      COMMENT 'Display label (e.g., "Elite", "Premium")',
    description TEXT
      COMMENT 'Tier description for admin UI',
    icon VARCHAR(20)
      COMMENT 'Icon/emoji for UI display',
    display_order INT NOT NULL
      COMMENT 'Sort order for UI display',

    INDEX idx_display_order (display_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
COMMENT='Customizable tier labels for UI display';

-- Step 9: Insert default tier labels
INSERT INTO tier_labels (tier, label, description, icon, display_order) VALUES
('tier1', 'Elite', 'Most powerful and capable models', '⭐', 1),
('tier2', 'Premium', 'High-quality professional models', '💎', 2),
('tier3', 'Standard', 'Balanced performance and cost', '🎯', 3),
('tier4', 'Fast', 'Speed-optimized models', '⚡', 4),
('tier5', 'New', 'Latest model additions', '✨', 5);

-- Migration Notes:
-- ================
-- 1. All existing tier assignments have been cleared
-- 2. Admins must reassign models to the 5 global tiers via the admin UI
-- 3. The tier system is now global across all providers (only 5 models total can be in tiers)
-- 4. Tier labels can be customized by updating the tier_labels table

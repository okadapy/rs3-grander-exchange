-- Add the recipe fields that drive tick-derived craft rates.
--
-- ticks is the per-action cost from the wiki's recipe infobox, and
-- facility separates mechanics that share a skill: smelting at a furnace
-- and forging at an anvil are both Smithing but are not the same
-- operation and do not have the same rate.
--
-- Both are populated by the next scrape. AutoMigrate creates them on a
-- fresh database from the model tags; this script is for databases that
-- already exist.
--
-- Safe to run more than once.

USE recipe_db;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA='recipe_db' AND TABLE_NAME='recipes' AND COLUMN_NAME='ticks'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE recipes ADD COLUMN ticks INT NOT NULL DEFAULT 0, ADD INDEX idx_recipes_ticks (ticks)',
  'SELECT "ticks column already present"');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA='recipe_db' AND TABLE_NAME='recipes' AND COLUMN_NAME='facility'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE recipes ADD COLUMN facility VARCHAR(48) NOT NULL DEFAULT ""',
  'SELECT "facility column already present"');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

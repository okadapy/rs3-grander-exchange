-- Index the recipe output name that ID resolution joins on.
--
-- BackfillInputItemIDs matches every input name against every recipe
-- output name:
--
--   UPDATE recipe_inputs ri
--   JOIN recipes r ON ri.item_name = r.output_item_name
--
-- Neither side was indexed, so this was a full scan against a full
-- scan: ~13s with nothing to write, ~44s on a run that actually
-- resolved rows. It used to run once at startup and once a day, where
-- nobody noticed it; it now runs after every scrape, because a scrape
-- clears the IDs it produces.
--
-- geid_maps.name and item_limits.name already carry indexes, which is
-- why the other resolution passes were never slow.
--
-- Only recipes.output_item_name is indexed. The mirror index on
-- recipe_inputs.item_name is not: all three resolution passes drive off
-- recipe_inputs.item_id and look the name up on the other side, and the
-- item search filters with a leading-wildcard LIKE over a derived table,
-- which no index can serve. It would only have cost write time on the
-- ~15k input rows every scrape inserts.
--
-- AutoMigrate creates the index on a fresh database from the model tag.
-- This script is for databases that already exist.
--
-- Safe to run more than once.

USE recipe_db;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA='recipe_db' AND TABLE_NAME='recipes'
    AND INDEX_NAME='idx_recipes_output_item_name'
);
SET @sql := IF(@exists = 0,
  'CREATE INDEX idx_recipes_output_item_name ON recipes (output_item_name)',
  'SELECT "idx_recipes_output_item_name already present"');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

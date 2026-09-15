-- Drop recipe_db.ge_id_maps, left behind by a model rename.
--
-- GORM derives a table name from the struct name, and for GEIDMap it
-- derived `ge_id_maps`. The name was later pinned to `geid_maps` with a
-- TableName method, but AutoMigrate only creates tables — it never
-- removes the one it stopped using. The old table stayed behind with a
-- full copy of the item ID map, going stale from the day of the rename
-- and confusing anyone who opened the schema looking for the live one.
--
-- Nothing reads it: the only reference to either name in the code is
-- models.GEIDMap.TableName, which returns `geid_maps`.
--
-- Safe to run more than once.

USE recipe_db;

DROP TABLE IF EXISTS ge_id_maps;

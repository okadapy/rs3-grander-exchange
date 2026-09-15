-- Collapse PriceSnapshot's buy_price/sell_price pair into a single
-- `price` column.
--
-- Both columns only ever held the same value: the Grand Exchange guide
-- price, written twice by the poller. Presenting them as a buy and a
-- sell implied a bid/ask spread that RS3 does not publish, and anything
-- reading them as two sides of a trade understated the real cost of a
-- round trip. Callers that need both sides now apply an explicit,
-- configurable spread (see calc-service `market.spread_pct`).
--
-- Safe to run more than once. Apply against ge_db before starting the
-- new ge-price-service build; AutoMigrate adds `price` but will not
-- backfill it or drop the old columns.

USE ge_db;

-- 1. Add the new column if this is a fresh database.
SET @exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = 'ge_db' AND TABLE_NAME = 'price_snapshots'
    AND COLUMN_NAME = 'price'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE price_snapshots ADD COLUMN price BIGINT NOT NULL DEFAULT 0',
  'SELECT "price column already present"');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 2. Backfill from buy_price, which held the guide price.
SET @has_buy := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = 'ge_db' AND TABLE_NAME = 'price_snapshots'
    AND COLUMN_NAME = 'buy_price'
);
SET @sql := IF(@has_buy = 1,
  'UPDATE price_snapshots SET price = buy_price WHERE price = 0 AND buy_price > 0',
  'SELECT "no buy_price column to backfill from"');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 3. Drop the retired columns.
SET @sql := IF(@has_buy = 1,
  'ALTER TABLE price_snapshots DROP COLUMN buy_price',
  'SELECT "buy_price already dropped"');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_sell := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = 'ge_db' AND TABLE_NAME = 'price_snapshots'
    AND COLUMN_NAME = 'sell_price'
);
SET @sql := IF(@has_sell = 1,
  'ALTER TABLE price_snapshots DROP COLUMN sell_price',
  'SELECT "sell_price already dropped"');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_norm := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = 'ge_db' AND TABLE_NAME = 'price_snapshots'
    AND COLUMN_NAME = 'normalized'
);
SET @sql := IF(@has_norm = 1,
  'ALTER TABLE price_snapshots DROP COLUMN normalized',
  'SELECT "normalized already dropped"');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 4. daily_changes was created by AutoMigrate but never written to.
DROP TABLE IF EXISTS daily_changes;

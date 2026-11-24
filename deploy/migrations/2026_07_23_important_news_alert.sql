-- ============================================================
-- Important news spotlight settings
--   News remains the source of truth for title/summary/cover/content.
--   Alert settings only control when and where the spotlight appears.
-- ============================================================
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_add_news_alert_col;
DELIMITER $$
CREATE PROCEDURE jc_add_news_alert_col(
  IN p_column VARCHAR(64),
  IN p_definition TEXT
)
BEGIN
  IF NOT EXISTS (
    SELECT 1
      FROM information_schema.columns
     WHERE table_schema = DATABASE()
       AND table_name = 'news'
       AND column_name = p_column
  ) THEN
    SET @ddl = CONCAT('ALTER TABLE `news` ADD COLUMN `', p_column, '` ', p_definition);
    PREPARE stmt FROM @ddl;
    EXECUTE stmt;
    DEALLOCATE PREPARE stmt;
  END IF;
END$$
DELIMITER ;

CALL jc_add_news_alert_col('is_important', 'TINYINT(1) NOT NULL DEFAULT 0 COMMENT ''show as important-news spotlight''');
CALL jc_add_news_alert_col('alert_priority', 'INT NOT NULL DEFAULT 0 COMMENT ''higher value wins when alerts overlap''');
CALL jc_add_news_alert_col('alert_start_at', 'DATETIME NULL DEFAULT NULL COMMENT ''spotlight schedule start''');
CALL jc_add_news_alert_col('alert_end_at', 'DATETIME NULL DEFAULT NULL COMMENT ''spotlight schedule end''');
CALL jc_add_news_alert_col('alert_frequency', 'VARCHAR(16) NOT NULL DEFAULT ''once'' COMMENT ''once / daily / session''');
CALL jc_add_news_alert_col('alert_pages', 'VARCHAR(255) NOT NULL DEFAULT ''home,ecosystem,launchpad'' COMMENT ''comma-separated route groups''');
CALL jc_add_news_alert_col('alert_delay_seconds', 'INT NOT NULL DEFAULT 3 COMMENT ''delay after page is ready, 0-15''');
CALL jc_add_news_alert_col('alert_revision', 'INT NOT NULL DEFAULT 1 COMMENT ''increment to show the alert again''');

DROP PROCEDURE jc_add_news_alert_col;

DROP PROCEDURE IF EXISTS jc_add_news_alert_idx;
DELIMITER $$
CREATE PROCEDURE jc_add_news_alert_idx()
BEGIN
  IF NOT EXISTS (
    SELECT 1
      FROM information_schema.statistics
     WHERE table_schema = DATABASE()
       AND table_name = 'news'
       AND index_name = 'idx_news_alert_active'
  ) THEN
    ALTER TABLE `news`
      ADD INDEX `idx_news_alert_active`
      (`is_important`, `status`, `alert_priority`, `publish_at`);
  END IF;
END$$
DELIMITER ;

CALL jc_add_news_alert_idx();
DROP PROCEDURE jc_add_news_alert_idx;

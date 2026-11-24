-- JoinCare IDO Sprint 1A-P0 · event recovery and observability
-- Adds durable retry/dead-letter/replay records, sync health checkpoints and block hashes.

SET NAMES utf8mb4;

SET @has_block_hash = (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_record' AND COLUMN_NAME = 'block_hash'
);
SET @add_block_hash_sql = IF(
  @has_block_hash = 0,
  'ALTER TABLE chain_event_record ADD COLUMN block_hash VARCHAR(66) NOT NULL DEFAULT '''' AFTER block_number',
  'SELECT 1'
);
PREPARE stmt FROM @add_block_hash_sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `chain_event_failure` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `event` VARCHAR(50) NOT NULL DEFAULT '',
  `block_number` BIGINT UNSIGNED NOT NULL,
  `block_hash` VARCHAR(66) NOT NULL DEFAULT '',
  `tx_hash` VARCHAR(100) NOT NULL,
  `log_index` INT UNSIGNED NOT NULL,
  `raw_log` LONGTEXT NOT NULL,
  `error_message` TEXT NOT NULL,
  `status` VARCHAR(20) NOT NULL,
  `retry_count` INT UNSIGNED NOT NULL DEFAULT 0,
  `consecutive_count` INT UNSIGNED NOT NULL DEFAULT 0,
  `replay_count` INT UNSIGNED NOT NULL DEFAULT 0,
  `first_failed_at` DATETIME(6) NOT NULL,
  `last_failed_at` DATETIME(6) NOT NULL,
  `replay_requested_at` DATETIME(6) NULL,
  `resolved_at` DATETIME(6) NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_chain_event_failure_identity` (`block_number`, `tx_hash`, `log_index`),
  KEY `idx_chain_event_failure_status` (`status`),
  KEY `idx_chain_event_failure_last_failed` (`last_failed_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `chain_event_sync_state` (
  `id` BIGINT UNSIGNED NOT NULL,
  `initialized` TINYINT(1) NOT NULL DEFAULT 0,
  `cursor_block` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `cursor_block_hash` VARCHAR(66) NOT NULL DEFAULT '',
  `lease_token` VARCHAR(64) NOT NULL DEFAULT '',
  `latest_block` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `safe_block` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `lag_blocks` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `last_started_at` DATETIME(6) NULL,
  `last_success_at` DATETIME(6) NULL,
  `last_failure_at` DATETIME(6) NULL,
  `last_error` TEXT NOT NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO chain_event_sync_state (
  id, cursor_block, cursor_block_hash, latest_block, safe_block, lag_blocks,
  last_error, created_at, updated_at
) VALUES (1, 0, '', 0, 0, 0, '', CURRENT_TIMESTAMP(6), CURRENT_TIMESTAMP(6))
ON DUPLICATE KEY UPDATE id = VALUES(id);

DROP PROCEDURE IF EXISTS jc_assert_chain_event_recovery_schema;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_recovery_schema()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_failure'
      AND COLUMN_NAME IN (
        'id','event','block_number','block_hash','tx_hash','log_index','raw_log','error_message',
        'status','retry_count','consecutive_count','replay_count','first_failed_at','last_failed_at',
        'replay_requested_at','resolved_at','created_at','updated_at'
      )
  ) <> 18 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'chain_event_failure schema is incomplete';
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM (
      SELECT INDEX_NAME,
             GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') AS columns_in_order,
             MIN(NON_UNIQUE) AS non_unique
      FROM information_schema.STATISTICS
      WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_failure'
      GROUP BY INDEX_NAME
    ) indexes_for_failure
    WHERE columns_in_order = 'block_number,tx_hash,log_index' AND non_unique = 0
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'chain_event_failure unique identity is missing';
  END IF;

  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_sync_state'
      AND COLUMN_NAME IN (
        'id','initialized','cursor_block','cursor_block_hash','lease_token','latest_block','safe_block',
        'lag_blocks','last_started_at','last_success_at','last_failure_at','last_error','created_at','updated_at'
      )
  ) <> 14 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'chain_event_sync_state schema is incomplete';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_recovery_schema();
DROP PROCEDURE jc_assert_chain_event_recovery_schema;

-- Verification:
--   SHOW CREATE TABLE chain_event_failure;
--   SHOW CREATE TABLE chain_event_sync_state;
--   SHOW COLUMNS FROM chain_event_record LIKE 'block_hash';

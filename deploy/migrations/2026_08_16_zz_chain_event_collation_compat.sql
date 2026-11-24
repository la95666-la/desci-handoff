-- JoinCare IDO migration-path compatibility.
-- The original chain_event_record table used utf8mb4_general_ci while
-- transaction_record uses utf8mb4_unicode_ci. MySQL 8 rejects the equality
-- predicates in the 2026_08_17 outbox preflight when these collations differ.
-- Normalize only the two compared legacy columns before that migration.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_event_collation_compat;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_event_collation_compat()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'
      AND TABLE_NAME IN ('chain_event_record','transaction_record')
  )<>2 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='chain event collation prerequisite tables are missing';
  END IF;

  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA=DATABASE() AND (
      (TABLE_NAME='chain_event_record' AND COLUMN_NAME='event'
        AND COLUMN_TYPE='varchar(50)' AND IS_NULLABLE='NO'
        AND CHARACTER_SET_NAME='utf8mb4'
        AND COLLATION_NAME IN ('utf8mb4_general_ci','utf8mb4_unicode_ci'))
      OR (TABLE_NAME='chain_event_record' AND COLUMN_NAME='tx_hash'
        AND COLUMN_TYPE='varchar(100)' AND IS_NULLABLE='NO'
        AND CHARACTER_SET_NAME='utf8mb4'
        AND COLLATION_NAME IN ('utf8mb4_general_ci','utf8mb4_unicode_ci'))
      OR (TABLE_NAME='transaction_record' AND COLUMN_NAME='type'
        AND COLUMN_TYPE='varchar(20)' AND IS_NULLABLE='NO'
        AND CHARACTER_SET_NAME='utf8mb4' AND COLLATION_NAME='utf8mb4_unicode_ci')
      OR (TABLE_NAME='transaction_record' AND COLUMN_NAME='tx_hash'
        AND COLUMN_TYPE='varchar(66)' AND IS_NULLABLE='NO'
        AND CHARACTER_SET_NAME='utf8mb4' AND COLLATION_NAME='utf8mb4_unicode_ci')
    )
  )<>4 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='chain event comparison column definitions are incompatible';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM (
      SELECT INDEX_NAME,
        GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') columns_in_order,
        MIN(NON_UNIQUE) non_unique
      FROM information_schema.STATISTICS
      WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_event_record'
      GROUP BY INDEX_NAME
    ) indexes_for_event
    WHERE columns_in_order='block_number,tx_hash,log_index' AND non_unique=0
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='chain event identity index is missing before collation normalization';
  END IF;

  IF EXISTS (
    SELECT 1 FROM chain_event_record
    GROUP BY block_number,tx_hash COLLATE utf8mb4_unicode_ci,log_index
    HAVING COUNT(*)>1
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='chain event identities collide under target collation';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_event_collation_compat();
DROP PROCEDURE jc_preflight_chain_event_collation_compat;

ALTER TABLE chain_event_record
  MODIFY COLUMN event VARCHAR(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
  MODIFY COLUMN tx_hash VARCHAR(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL;

DROP PROCEDURE IF EXISTS jc_assert_chain_event_collation_compat;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_collation_compat()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA=DATABASE() AND (
      (TABLE_NAME='chain_event_record' AND COLUMN_NAME='event'
        AND COLUMN_TYPE='varchar(50)' AND IS_NULLABLE='NO'
        AND CHARACTER_SET_NAME='utf8mb4' AND COLLATION_NAME='utf8mb4_unicode_ci')
      OR (TABLE_NAME='chain_event_record' AND COLUMN_NAME='tx_hash'
        AND COLUMN_TYPE='varchar(100)' AND IS_NULLABLE='NO'
        AND CHARACTER_SET_NAME='utf8mb4' AND COLLATION_NAME='utf8mb4_unicode_ci')
      OR (TABLE_NAME='transaction_record' AND COLUMN_NAME='type'
        AND COLUMN_TYPE='varchar(20)' AND IS_NULLABLE='NO'
        AND CHARACTER_SET_NAME='utf8mb4' AND COLLATION_NAME='utf8mb4_unicode_ci')
      OR (TABLE_NAME='transaction_record' AND COLUMN_NAME='tx_hash'
        AND COLUMN_TYPE='varchar(66)' AND IS_NULLABLE='NO'
        AND CHARACTER_SET_NAME='utf8mb4' AND COLLATION_NAME='utf8mb4_unicode_ci')
    )
  )<>4 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='chain event comparison collations were not normalized';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM (
      SELECT INDEX_NAME,
        GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') columns_in_order,
        MIN(NON_UNIQUE) non_unique
      FROM information_schema.STATISTICS
      WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_event_record'
      GROUP BY INDEX_NAME
    ) indexes_for_event
    WHERE columns_in_order='block_number,tx_hash,log_index' AND non_unique=0
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='chain event identity index changed during collation normalization';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_collation_compat();
DROP PROCEDURE jc_assert_chain_event_collation_compat;

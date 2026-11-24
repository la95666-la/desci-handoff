-- JoinCare IDO Sprint 1A-P2 · append-only administrator replay evidence.
-- This migration extends 2026_08_14 and 2026_08_16; do not run it independently.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_event_replay_audit;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_event_replay_audit()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_failure' AND TABLE_TYPE = 'BASE TABLE'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain_event_failure prerequisite table is missing';
  END IF;
  IF EXISTS (
    SELECT 1 FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_replay_audit'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'pre-existing chain event replay audit table requires manual verification';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_failure'
      AND COLUMN_NAME IN ('id','event','status','replay_count','replay_requested_at')
  ) <> 5 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain event recovery prerequisite columns are missing';
  END IF;
  IF EXISTS (
    SELECT 1 FROM chain_event_failure
    WHERE replay_count <> 0 OR replay_requested_at IS NOT NULL
    LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'historical replay records require manual evidence reconciliation before migration';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_event_replay_audit();
DROP PROCEDURE jc_preflight_chain_event_replay_audit;

CREATE TABLE `chain_event_replay_audit` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `failure_id` BIGINT UNSIGNED NOT NULL,
  `replay_no` INT UNSIGNED NOT NULL,
  `admin_id` BIGINT UNSIGNED NOT NULL,
  `reason` VARCHAR(500) NOT NULL,
  `requested_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_chain_event_replay_no` (`failure_id`,`replay_no`),
  KEY `idx_chain_event_replay_admin` (`admin_id`,`requested_at`),
  CONSTRAINT `fk_chain_event_replay_failure` FOREIGN KEY (`failure_id`) REFERENCES `chain_event_failure` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `chk_chain_event_replay_number` CHECK (`replay_no` > 0) ENFORCED,
  CONSTRAINT `chk_chain_event_replay_admin` CHECK (`admin_id` > 0) ENFORCED,
  CONSTRAINT `chk_chain_event_replay_reason` CHECK (CHAR_LENGTH(TRIM(`reason`)) BETWEEN 3 AND 500) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DROP TRIGGER IF EXISTS `trg_chain_event_replay_audit_no_update`;
DROP TRIGGER IF EXISTS `trg_chain_event_replay_audit_no_delete`;
DELIMITER $$
CREATE TRIGGER `trg_chain_event_replay_audit_no_update`
BEFORE UPDATE ON `chain_event_replay_audit`
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain event replay audit rows are append-only';
END$$
CREATE TRIGGER `trg_chain_event_replay_audit_no_delete`
BEFORE DELETE ON `chain_event_replay_audit`
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain event replay audit rows are append-only';
END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS jc_assert_chain_event_replay_audit;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_replay_audit()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_replay_audit' AND (
      (COLUMN_NAME = 'id' AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME = 'failure_id' AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME = 'replay_no' AND COLUMN_TYPE = 'int unsigned' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME = 'admin_id' AND COLUMN_TYPE = 'bigint unsigned' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME = 'reason' AND COLUMN_TYPE = 'varchar(500)' AND IS_NULLABLE = 'NO')
      OR (COLUMN_NAME = 'requested_at' AND COLUMN_TYPE = 'datetime(6)' AND IS_NULLABLE = 'NO')
    )
  ) <> 6 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain event replay audit columns are incompatible';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM (
      SELECT INDEX_NAME, GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ',') AS columns_in_order, MIN(NON_UNIQUE) AS non_unique
      FROM information_schema.STATISTICS
      WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_replay_audit'
      GROUP BY INDEX_NAME
    ) replay_indexes
    WHERE columns_in_order = 'failure_id,replay_no' AND non_unique = 0
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain event replay audit unique sequence is missing';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_replay_audit'
      AND CONSTRAINT_NAME IN ('chk_chain_event_replay_number','chk_chain_event_replay_admin','chk_chain_event_replay_reason')
      AND CONSTRAINT_TYPE = 'CHECK' AND ENFORCED = 'YES'
  ) <> 3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain event replay audit checks are incomplete';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.REFERENTIAL_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'chain_event_replay_audit'
      AND CONSTRAINT_NAME = 'fk_chain_event_replay_failure' AND REFERENCED_TABLE_NAME = 'chain_event_failure'
      AND DELETE_RULE = 'RESTRICT'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain event replay audit foreign key is missing';
  END IF;
  IF (
    SELECT COUNT(*) FROM information_schema.TRIGGERS
    WHERE TRIGGER_SCHEMA = DATABASE() AND EVENT_OBJECT_TABLE = 'chain_event_replay_audit'
      AND TRIGGER_NAME IN ('trg_chain_event_replay_audit_no_update','trg_chain_event_replay_audit_no_delete')
  ) <> 2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'chain event replay audit append-only triggers are incomplete';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_replay_audit();
DROP PROCEDURE jc_assert_chain_event_replay_audit;

-- Match the shared Go ChainTxSubmission model used by Core and the IDO Job.
-- Explicit operator-controlled migration: do not auto-apply to production.
-- Existing transaction states and nonce fences are not changed here.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_chain_tx_signing_metadata_1002;
DELIMITER $$
CREATE PROCEDURE jc_chain_tx_signing_metadata_1002()
BEGIN
  IF COALESCE(@JC_CHAIN_TX_SIGNING_METADATA_APPROVED, 0) <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain tx signing metadata migration requires approved execution gate';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_tx_submission' AND TABLE_TYPE='BASE TABLE') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain_tx_submission table missing';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_tx_submission' AND COLUMN_NAME='unsigned_tx_digest') THEN
    ALTER TABLE chain_tx_submission ADD COLUMN unsigned_tx_digest VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' AFTER state;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_tx_submission' AND COLUMN_NAME='signing_digest') THEN
    ALTER TABLE chain_tx_submission ADD COLUMN signing_digest VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' AFTER unsigned_tx_digest;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_tx_submission' AND COLUMN_NAME='replacement_generation') THEN
    ALTER TABLE chain_tx_submission ADD COLUMN replacement_generation INT UNSIGNED NOT NULL DEFAULT 0 AFTER signing_digest;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_tx_submission' AND COLUMN_NAME='signing_reserved_at') THEN
    ALTER TABLE chain_tx_submission ADD COLUMN signing_reserved_at DATETIME(6) NULL AFTER replacement_generation;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_tx_submission' AND COLUMN_NAME='signing_recovery_reason') THEN
    ALTER TABLE chain_tx_submission ADD COLUMN signing_recovery_reason TEXT NOT NULL DEFAULT ('') AFTER signing_reserved_at;
  END IF;

  IF (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_tx_submission' AND (
    (COLUMN_NAME IN ('unsigned_tx_digest','signing_digest') AND NOT (COLUMN_TYPE='varchar(66)' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='' AND CHARACTER_SET_NAME='ascii' AND COLLATION_NAME='ascii_bin')) OR
    (COLUMN_NAME='replacement_generation' AND NOT (COLUMN_TYPE='int unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='0')) OR
    (COLUMN_NAME='signing_reserved_at' AND NOT (COLUMN_TYPE='datetime(6)' AND IS_NULLABLE='YES')) OR
    (COLUMN_NAME='signing_recovery_reason' AND NOT (DATA_TYPE='text' AND IS_NULLABLE='NO' AND (COLUMN_DEFAULT='' OR REPLACE(COLUMN_DEFAULT,CHAR(92),'')=CONCAT('_utf8mb4',CHAR(39),CHAR(39)))))
  )) <> 0 OR (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_tx_submission' AND COLUMN_NAME IN ('unsigned_tx_digest','signing_digest','replacement_generation','signing_reserved_at','signing_recovery_reason')) <> 5 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='chain tx signing metadata columns do not match expected shape';
  END IF;
END$$
DELIMITER ;
CALL jc_chain_tx_signing_metadata_1002();
DROP PROCEDURE jc_chain_tx_signing_metadata_1002;

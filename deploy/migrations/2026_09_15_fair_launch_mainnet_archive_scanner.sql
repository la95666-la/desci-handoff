-- JoinCare Fair Launch BSC mainnet finalized archive scanner state.
-- ADDITIVE ONLY. Creates no activation/cursor/lease/watermark and starts no job.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_require_fair_launch_mainnet_scanner_0915;
DELIMITER $$
CREATE PROCEDURE jc_require_fair_launch_mainnet_scanner_0915()
BEGIN
  IF COALESCE(@JC_FAIR_LAUNCH_MAINNET_SCANNER_APPROVED,0)<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet scanner migration requires approved execution gate';
  END IF;
  IF NOT EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_mainnet_project_bootstrap_receipt_v1') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet scanner migration requires 0914';
  END IF;
END$$
DELIMITER ;
CALL jc_require_fair_launch_mainnet_scanner_0915();
DROP PROCEDURE jc_require_fair_launch_mainnet_scanner_0915;

CREATE TABLE IF NOT EXISTS fair_launch_mainnet_archive_cursor_v1 (
  activation_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  archived_through_block BIGINT UNSIGNED NOT NULL,
  archived_through_block_hash CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  lease_generation BIGINT UNSIGNED NOT NULL,
  fence_epoch BIGINT UNSIGNED NOT NULL,
  lease_token_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  last_error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  last_error_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  last_batch_receipt_id BIGINT UNSIGNED NULL,
  cursor_generation BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY(activation_row_id),
  UNIQUE KEY uk_fl_mainnet_archive_cursor_source(activation_row_id,chain_id,source_id,source_revision_id),
  CONSTRAINT fk_fl_mainnet_archive_cursor_activation FOREIGN KEY(activation_row_id,chain_id,source_id,source_revision_id) REFERENCES fair_launch_mainnet_activation_v1(authority_row_id,chain_id,source_id,source_revision_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_mainnet_archive_cursor CHECK(
    chain_id=56 AND lease_generation>0 AND fence_epoch>0 AND cursor_generation>0 AND
    archived_through_block_hash REGEXP '^0x[0-9a-f]{64}$' AND lease_token_hash REGEXP '^[0-9a-f]{64}$' AND
    status IN('active','blocked','reorg_detected','revoked') AND
    ((status='active' AND last_error_code IS NULL AND last_error_digest IS NULL) OR
     (status IN('blocked','reorg_detected') AND CHAR_LENGTH(last_error_code)>0 AND last_error_digest REGEXP '^[0-9a-f]{64}$') OR
     (status='revoked' AND last_error_code IS NULL AND last_error_digest IS NULL))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_mainnet_archive_batch_receipt_v1 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  activation_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  lease_generation BIGINT UNSIGNED NOT NULL,
  fence_epoch BIGINT UNSIGNED NOT NULL,
  from_block BIGINT UNSIGNED NOT NULL,
  through_block BIGINT UNSIGNED NOT NULL,
  from_parent_block_hash CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  through_block_hash CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  provider_identity_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  block_count INT UNSIGNED NOT NULL,
  event_count INT UNSIGNED NOT NULL,
  coverage_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_receipt_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  receipt_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_mainnet_archive_batch_start(activation_row_id,from_block),
  UNIQUE KEY uk_fl_mainnet_archive_batch_receipt(activation_row_id,receipt_digest),
  UNIQUE KEY uk_fl_mainnet_archive_batch_identity(id,activation_row_id),
  CONSTRAINT fk_fl_mainnet_archive_batch_cursor FOREIGN KEY(activation_row_id,chain_id,source_id,source_revision_id) REFERENCES fair_launch_mainnet_archive_cursor_v1(activation_row_id,chain_id,source_id,source_revision_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_mainnet_archive_batch CHECK(
    chain_id=56 AND lease_generation>0 AND fence_epoch>0 AND from_block<=through_block AND
    block_count=through_block-from_block+1 AND block_count BETWEEN 1 AND 64 AND
    from_parent_block_hash REGEXP '^0x[0-9a-f]{64}$' AND through_block_hash REGEXP '^0x[0-9a-f]{64}$' AND
    provider_identity_digest REGEXP '^[0-9a-f]{64}$' AND coverage_digest REGEXP '^[0-9a-f]{64}$' AND
    (previous_receipt_digest IS NULL OR previous_receipt_digest REGEXP '^[0-9a-f]{64}$') AND receipt_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE fair_launch_mainnet_archive_cursor_v1
  ADD CONSTRAINT fk_fl_mainnet_archive_cursor_last_batch FOREIGN KEY(last_batch_receipt_id,activation_row_id) REFERENCES fair_launch_mainnet_archive_batch_receipt_v1(id,activation_row_id) ON UPDATE RESTRICT ON DELETE RESTRICT;

ALTER TABLE chain_event_placement_observation_v2
  ADD UNIQUE KEY uk_ce_observation_identity_placement(id,event_identity_id,placement_id);

CREATE TABLE IF NOT EXISTS fair_launch_mainnet_archive_batch_event_v1 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  batch_receipt_id BIGINT UNSIGNED NOT NULL,
  activation_row_id BIGINT UNSIGNED NOT NULL,
  event_ordinal INT UNSIGNED NOT NULL,
  event_identity_id BIGINT UNSIGNED NOT NULL,
  placement_id BIGINT UNSIGNED NOT NULL,
  observation_id BIGINT UNSIGNED NOT NULL,
  event_name VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  payload_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  row_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_mainnet_archive_batch_event_ordinal(batch_receipt_id,event_ordinal),
  UNIQUE KEY uk_fl_mainnet_archive_batch_event_identity(batch_receipt_id,event_identity_id),
  UNIQUE KEY uk_fl_mainnet_archive_batch_event_row(batch_receipt_id,row_digest),
  CONSTRAINT fk_fl_mainnet_archive_batch_event_batch FOREIGN KEY(batch_receipt_id,activation_row_id) REFERENCES fair_launch_mainnet_archive_batch_receipt_v1(id,activation_row_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_archive_batch_event_identity FOREIGN KEY(event_identity_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_archive_batch_event_placement FOREIGN KEY(placement_id,event_identity_id) REFERENCES chain_event_placement_v2(id,event_identity_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_archive_batch_event_observation FOREIGN KEY(observation_id,event_identity_id,placement_id) REFERENCES chain_event_placement_observation_v2(id,event_identity_id,placement_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_mainnet_archive_batch_event CHECK(
    event_name IN(
      'FairLaunchProjectCreated','FairLaunchTokensDeposited','FairLaunchActivated','FairLaunchContribution','FairLaunchEnded','FairLaunchSuccessfulReleased',
      'FairLaunchPendingReleaseInvalidated','FairLaunchPaymentTokenInvalidated','FairLaunchCancelled','FairLaunchClaimed','FairLaunchRefunded',
      'FairLaunchProjectProceedsWithdrawn','FairLaunchPlatformFeeWithdrawn','FairLaunchFailedProjectTokensWithdrawn','FairLaunchContributionsPaused',
      'FairLaunchContributionsUnpaused','FairLaunchPayoutsPaused','FairLaunchPayoutsUnpaused','FairLaunchClaimsPaused','FairLaunchClaimsUnpaused',
      'FairLaunchOperatorUpdated','FairLaunchPauseGuardianUpdated','FairLaunchUnpauseTimelockUpdated','FairLaunchTreasuryUpdated','FairLaunchPlatformFeeUpdated',
      'FairLaunchPaymentTokenPolicyRevisionRegistered','FairLaunchPaymentTokenAdmissionUpdated','FairLaunchPaymentTokenEmergencyBlockUpdated',
      'FairLaunchProjectTokenConfigured','FairLaunchBeneficiarySafeConfigured','FairLaunchReleasePolicyConfigured','FairLaunchExcessTokensRecovered'
    ) AND event_ordinal<1000000 AND payload_digest REGEXP '^[0-9a-f]{64}$' AND row_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_mainnet_archive_dead_letter_v1 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  activation_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  block_number BIGINT UNSIGNED NULL,
  block_hash CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL,
  transaction_index INT UNSIGNED NULL,
  tx_log_ordinal INT UNSIGNED NULL,
  rpc_log_index INT UNSIGNED NULL,
  tx_hash CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL,
  reason_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  evidence_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_mainnet_archive_dead_letter_evidence(activation_row_id,evidence_digest),
  CONSTRAINT fk_fl_mainnet_archive_dead_letter_activation FOREIGN KEY(activation_row_id,chain_id,source_id,source_revision_id) REFERENCES fair_launch_mainnet_activation_v1(authority_row_id,chain_id,source_id,source_revision_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_mainnet_archive_dead_letter CHECK(
    chain_id=56 AND reason_code IN('unknown_event','malformed_log','receipt_mismatch','canonical_conflict','rpc_inconsistent','archive_conflict') AND
    evidence_digest REGEXP '^[0-9a-f]{64}$' AND
    ((block_number IS NULL AND block_hash IS NULL AND transaction_index IS NULL AND tx_log_ordinal IS NULL AND rpc_log_index IS NULL AND tx_hash IS NULL) OR
     (block_number IS NOT NULL AND block_hash REGEXP '^0x[0-9a-f]{64}$' AND transaction_index IS NOT NULL AND rpc_log_index IS NOT NULL AND tx_hash REGEXP '^0x[0-9a-f]{64}$'))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

DROP TRIGGER IF EXISTS trg_fl_mainnet_archive_cursor_validate_insert;
DROP TRIGGER IF EXISTS trg_fl_mainnet_archive_cursor_validate_update;
DROP TRIGGER IF EXISTS trg_fl_mainnet_archive_cursor_no_delete;
DROP TRIGGER IF EXISTS trg_fl_mainnet_archive_batch_validate_insert;
DROP TRIGGER IF EXISTS trg_fl_mainnet_archive_batch_no_update;
DROP TRIGGER IF EXISTS trg_fl_mainnet_archive_batch_no_delete;
DROP TRIGGER IF EXISTS trg_fl_mainnet_archive_batch_event_no_update;
DROP TRIGGER IF EXISTS trg_fl_mainnet_archive_batch_event_no_delete;
DROP TRIGGER IF EXISTS trg_fl_mainnet_archive_dead_letter_no_update;
DROP TRIGGER IF EXISTS trg_fl_mainnet_archive_dead_letter_no_delete;
DELIMITER $$
CREATE TRIGGER trg_fl_mainnet_archive_cursor_validate_insert BEFORE INSERT ON fair_launch_mainnet_archive_cursor_v1 FOR EACH ROW BEGIN
  IF NEW.status<>'active' OR NEW.cursor_generation<>1 OR NEW.last_batch_receipt_id IS NOT NULL OR NEW.last_error_code IS NOT NULL OR NEW.last_error_digest IS NOT NULL OR
     NOT EXISTS(
       SELECT 1 FROM fair_launch_mainnet_activation_v1 a
       JOIN fair_launch_mainnet_activation_lease_v1 l ON l.activation_row_id=a.authority_row_id AND l.chain_id=a.chain_id
       WHERE a.authority_row_id=NEW.activation_row_id AND a.chain_id=NEW.chain_id AND a.source_id=NEW.source_id AND a.source_revision_id=NEW.source_revision_id AND
         NEW.archived_through_block+1=a.source_start_block AND l.status='active' AND l.active_slot=1 AND l.lease_generation=NEW.lease_generation AND
         l.fence_epoch=NEW.fence_epoch AND l.lease_token_hash=NEW.lease_token_hash
     ) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet archive cursor admission mismatch'; END IF;
END$$
CREATE TRIGGER trg_fl_mainnet_archive_batch_validate_insert BEFORE INSERT ON fair_launch_mainnet_archive_batch_receipt_v1 FOR EACH ROW BEGIN
  IF NOT EXISTS(
    SELECT 1 FROM fair_launch_mainnet_archive_cursor_v1 c
    JOIN fair_launch_mainnet_activation_lease_v1 l ON l.activation_row_id=c.activation_row_id AND l.chain_id=c.chain_id
    LEFT JOIN fair_launch_mainnet_archive_batch_receipt_v1 p ON p.id=c.last_batch_receipt_id AND p.activation_row_id=c.activation_row_id
    WHERE c.activation_row_id=NEW.activation_row_id AND c.chain_id=NEW.chain_id AND c.source_id=NEW.source_id AND c.source_revision_id=NEW.source_revision_id AND
      c.status='active' AND c.last_error_code IS NULL AND c.archived_through_block+1=NEW.from_block AND c.archived_through_block_hash=NEW.from_parent_block_hash AND
      c.lease_generation=NEW.lease_generation AND c.fence_epoch=NEW.fence_epoch AND
      ((c.last_batch_receipt_id IS NULL AND NEW.previous_receipt_digest IS NULL) OR (c.last_batch_receipt_id IS NOT NULL AND p.receipt_digest=NEW.previous_receipt_digest)) AND
      l.status='active' AND l.active_slot=1 AND l.lease_generation=NEW.lease_generation AND l.fence_epoch=NEW.fence_epoch AND l.lease_token_hash=c.lease_token_hash
  ) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet archive batch predecessor mismatch'; END IF;
END$$
CREATE TRIGGER trg_fl_mainnet_archive_cursor_validate_update BEFORE UPDATE ON fair_launch_mainnet_archive_cursor_v1 FOR EACH ROW BEGIN
  IF NOT(OLD.activation_row_id<=>NEW.activation_row_id AND OLD.chain_id<=>NEW.chain_id AND OLD.source_id<=>NEW.source_id AND OLD.source_revision_id<=>NEW.source_revision_id AND
         OLD.lease_generation<=>NEW.lease_generation AND OLD.fence_epoch<=>NEW.fence_epoch AND OLD.lease_token_hash<=>NEW.lease_token_hash AND OLD.created_at<=>NEW.created_at) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet archive cursor identity is immutable';
  END IF;
  IF OLD.status<>'active' OR NEW.cursor_generation<>OLD.cursor_generation+1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet archive cursor transition invalid';
  END IF;
  IF NEW.status='active' THEN
    IF NEW.archived_through_block<=OLD.archived_through_block OR NEW.last_batch_receipt_id IS NULL OR NEW.last_batch_receipt_id<=>OLD.last_batch_receipt_id OR NEW.last_error_code IS NOT NULL OR NEW.last_error_digest IS NOT NULL OR
       NOT EXISTS(SELECT 1 FROM fair_launch_mainnet_archive_batch_receipt_v1 b WHERE b.id=NEW.last_batch_receipt_id AND b.activation_row_id=NEW.activation_row_id AND
         b.from_block=OLD.archived_through_block+1 AND b.through_block=NEW.archived_through_block AND b.from_parent_block_hash=OLD.archived_through_block_hash AND
         b.through_block_hash=NEW.archived_through_block_hash AND b.lease_generation=NEW.lease_generation AND b.fence_epoch=NEW.fence_epoch AND
         (SELECT COUNT(*) FROM fair_launch_mainnet_archive_batch_event_v1 e WHERE e.batch_receipt_id=b.id)=b.event_count AND
         NOT EXISTS(SELECT 1 FROM fair_launch_mainnet_archive_batch_event_v1 e JOIN chain_event_placement_v2 p ON p.id=e.placement_id AND p.event_identity_id=e.event_identity_id WHERE e.batch_receipt_id=b.id AND (p.source_revision_id<>NEW.source_revision_id OR p.canonical_slot<>1 OR p.finalized_at IS NULL OR p.noncanonical_at IS NOT NULL))) THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet archive cursor batch receipt mismatch';
    END IF;
  ELSEIF NEW.status IN('blocked','reorg_detected') THEN
    IF NOT(NEW.archived_through_block<=>OLD.archived_through_block AND NEW.archived_through_block_hash<=>OLD.archived_through_block_hash AND NEW.last_batch_receipt_id<=>OLD.last_batch_receipt_id) OR NEW.last_error_code IS NULL OR NEW.last_error_digest IS NULL THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet archive cursor blocker mismatch';
    END IF;
  ELSEIF NEW.status='revoked' THEN
    IF NOT(NEW.archived_through_block<=>OLD.archived_through_block AND NEW.archived_through_block_hash<=>OLD.archived_through_block_hash AND NEW.last_batch_receipt_id<=>OLD.last_batch_receipt_id) OR NEW.last_error_code IS NOT NULL OR NEW.last_error_digest IS NOT NULL THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet archive cursor revocation mismatch';
    END IF;
  ELSE
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet archive cursor state invalid';
  END IF;
END$$
CREATE TRIGGER trg_fl_mainnet_archive_cursor_no_delete BEFORE DELETE ON fair_launch_mainnet_archive_cursor_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_archive_cursor_v1 cannot be deleted'; END$$
CREATE TRIGGER trg_fl_mainnet_archive_batch_no_update BEFORE UPDATE ON fair_launch_mainnet_archive_batch_receipt_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_archive_batch_receipt_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_archive_batch_no_delete BEFORE DELETE ON fair_launch_mainnet_archive_batch_receipt_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_archive_batch_receipt_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_archive_batch_event_no_update BEFORE UPDATE ON fair_launch_mainnet_archive_batch_event_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_archive_batch_event_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_archive_batch_event_no_delete BEFORE DELETE ON fair_launch_mainnet_archive_batch_event_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_archive_batch_event_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_archive_dead_letter_no_update BEFORE UPDATE ON fair_launch_mainnet_archive_dead_letter_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_archive_dead_letter_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_archive_dead_letter_no_delete BEFORE DELETE ON fair_launch_mainnet_archive_dead_letter_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_archive_dead_letter_v1 is append-only'; END$$
DELIMITER ;

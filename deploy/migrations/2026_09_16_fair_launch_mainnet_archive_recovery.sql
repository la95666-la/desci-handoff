-- JoinCare Fair Launch BSC mainnet archive cursor recovery authority.
-- ADDITIVE ONLY. Creates no recovery receipt, lease, cursor, activation or job.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_require_fair_launch_mainnet_recovery_0916;
DELIMITER $$
CREATE PROCEDURE jc_require_fair_launch_mainnet_recovery_0916()
BEGIN
  IF COALESCE(@JC_FAIR_LAUNCH_MAINNET_RECOVERY_APPROVED,0)<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet recovery migration requires approved execution gate';
  END IF;
  IF NOT EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_mainnet_archive_dead_letter_v1') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet recovery migration requires 0915';
  END IF;
END$$
DELIMITER ;
CALL jc_require_fair_launch_mainnet_recovery_0916();
DROP PROCEDURE jc_require_fair_launch_mainnet_recovery_0916;

CREATE TABLE IF NOT EXISTS fair_launch_mainnet_archive_recovery_receipt_v1 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  activation_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  dead_letter_id BIGINT UNSIGNED NOT NULL,
  previous_status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  reason_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_error_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  archived_through_block BIGINT UNSIGNED NOT NULL,
  archived_through_block_hash CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  last_batch_receipt_id BIGINT UNSIGNED NULL,
  previous_cursor_generation BIGINT UNSIGNED NOT NULL,
  previous_lease_id BIGINT UNSIGNED NOT NULL,
  replacement_lease_id BIGINT UNSIGNED NOT NULL,
  recovery_action VARCHAR(40) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  confirmation_policy VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  verification_provider_set_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  verification_count INT UNSIGNED NOT NULL,
  canonical_chain_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  approval_evidence_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  recovery_manifest_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  receipt_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_mainnet_archive_recovery_dead_letter(activation_row_id,dead_letter_id),
  UNIQUE KEY uk_fl_mainnet_archive_recovery_generation(activation_row_id,previous_cursor_generation),
  UNIQUE KEY uk_fl_mainnet_archive_recovery_receipt(activation_row_id,receipt_digest),
  UNIQUE KEY uk_fl_mainnet_archive_recovery_identity(id,activation_row_id),
  CONSTRAINT fk_fl_mainnet_archive_recovery_activation FOREIGN KEY(activation_row_id,chain_id,source_id,source_revision_id) REFERENCES fair_launch_mainnet_activation_v1(authority_row_id,chain_id,source_id,source_revision_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_archive_recovery_dead_letter FOREIGN KEY(dead_letter_id) REFERENCES fair_launch_mainnet_archive_dead_letter_v1(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_archive_recovery_previous_lease FOREIGN KEY(previous_lease_id) REFERENCES fair_launch_mainnet_activation_lease_v1(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mainnet_archive_recovery_replacement_lease FOREIGN KEY(replacement_lease_id) REFERENCES fair_launch_mainnet_activation_lease_v1(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_mainnet_archive_recovery CHECK(
    chain_id=56 AND previous_status IN('blocked','reorg_detected') AND
    reason_code IN('unknown_event','malformed_log','receipt_mismatch','canonical_conflict','rpc_inconsistent','archive_conflict') AND
    previous_error_digest REGEXP '^[0-9a-f]{64}$' AND archived_through_block_hash REGEXP '^0x[0-9a-f]{64}$' AND previous_cursor_generation>0 AND
    recovery_action IN('canonical_reverified','provider_consensus_reverified','decoder_remediated','receipt_reverified','repository_remediated') AND
    confirmation_policy='20_blocks' AND verification_count BETWEEN 1 AND 8 AND
    verification_provider_set_digest REGEXP '^[0-9a-f]{64}$' AND canonical_chain_digest REGEXP '^[0-9a-f]{64}$' AND
    approval_evidence_digest REGEXP '^[0-9a-f]{64}$' AND recovery_manifest_digest REGEXP '^[0-9a-f]{64}$' AND receipt_digest REGEXP '^[0-9a-f]{64}$'
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE fair_launch_mainnet_archive_cursor_v1
  ADD COLUMN last_recovery_receipt_id BIGINT UNSIGNED NULL AFTER last_batch_receipt_id,
  ADD CONSTRAINT fk_fl_mainnet_archive_cursor_last_recovery FOREIGN KEY(last_recovery_receipt_id,activation_row_id) REFERENCES fair_launch_mainnet_archive_recovery_receipt_v1(id,activation_row_id) ON UPDATE RESTRICT ON DELETE RESTRICT;

DROP TRIGGER IF EXISTS trg_fl_mainnet_archive_recovery_validate_insert;
DROP TRIGGER IF EXISTS trg_fl_mainnet_archive_recovery_no_update;
DROP TRIGGER IF EXISTS trg_fl_mainnet_archive_recovery_no_delete;
DROP TRIGGER IF EXISTS trg_fl_mainnet_archive_cursor_validate_update;
DELIMITER $$
CREATE TRIGGER trg_fl_mainnet_archive_recovery_validate_insert BEFORE INSERT ON fair_launch_mainnet_archive_recovery_receipt_v1 FOR EACH ROW BEGIN
  IF NOT(
    (NEW.reason_code='canonical_conflict' AND NEW.previous_status='reorg_detected' AND NEW.recovery_action='canonical_reverified' AND NEW.verification_count>=2) OR
    (NEW.reason_code='rpc_inconsistent' AND NEW.previous_status='reorg_detected' AND NEW.recovery_action='provider_consensus_reverified' AND NEW.verification_count>=2) OR
    (NEW.reason_code IN('unknown_event','malformed_log') AND NEW.previous_status='blocked' AND NEW.recovery_action='decoder_remediated') OR
    (NEW.reason_code='receipt_mismatch' AND NEW.previous_status='blocked' AND NEW.recovery_action='receipt_reverified') OR
    (NEW.reason_code='archive_conflict' AND NEW.previous_status='blocked' AND NEW.recovery_action='repository_remediated')
  ) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet archive recovery action mismatch'; END IF;
  IF NOT EXISTS(
    SELECT 1 FROM fair_launch_mainnet_archive_cursor_v1 c
    JOIN fair_launch_mainnet_archive_dead_letter_v1 d ON d.id=NEW.dead_letter_id AND d.activation_row_id=c.activation_row_id
    JOIN fair_launch_mainnet_activation_lease_v1 old_l ON old_l.id=NEW.previous_lease_id AND old_l.activation_row_id=c.activation_row_id
    JOIN fair_launch_mainnet_activation_lease_v1 new_l ON new_l.id=NEW.replacement_lease_id AND new_l.activation_row_id=c.activation_row_id
    WHERE c.activation_row_id=NEW.activation_row_id AND c.chain_id=NEW.chain_id AND c.source_id=NEW.source_id AND c.source_revision_id=NEW.source_revision_id AND
      c.status=NEW.previous_status AND c.last_error_code=NEW.reason_code AND c.last_error_digest=NEW.previous_error_digest AND
      c.archived_through_block=NEW.archived_through_block AND c.archived_through_block_hash=NEW.archived_through_block_hash AND
      c.last_batch_receipt_id<=>NEW.last_batch_receipt_id AND c.cursor_generation=NEW.previous_cursor_generation AND
      d.chain_id=NEW.chain_id AND d.source_id=NEW.source_id AND d.source_revision_id=NEW.source_revision_id AND d.reason_code=NEW.reason_code AND d.evidence_digest=NEW.previous_error_digest AND
      old_l.chain_id=NEW.chain_id AND old_l.lease_generation=c.lease_generation AND old_l.fence_epoch=c.fence_epoch AND old_l.lease_token_hash=c.lease_token_hash AND old_l.status IN('revoked','expired') AND old_l.active_slot IS NULL AND
      new_l.chain_id=NEW.chain_id AND new_l.status='active' AND new_l.active_slot=1 AND new_l.valid_from<=NEW.created_at AND new_l.valid_until>NEW.created_at AND
      new_l.lease_generation>old_l.lease_generation AND new_l.fence_epoch>old_l.fence_epoch
  ) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet archive recovery authority mismatch'; END IF;
END$$
CREATE TRIGGER trg_fl_mainnet_archive_cursor_validate_update BEFORE UPDATE ON fair_launch_mainnet_archive_cursor_v1 FOR EACH ROW BEGIN
  IF NOT(OLD.activation_row_id<=>NEW.activation_row_id AND OLD.chain_id<=>NEW.chain_id AND OLD.source_id<=>NEW.source_id AND OLD.source_revision_id<=>NEW.source_revision_id AND OLD.created_at<=>NEW.created_at) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet archive cursor identity is immutable';
  END IF;
  IF NEW.cursor_generation<>OLD.cursor_generation+1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet archive cursor transition invalid';
  END IF;
  IF OLD.status='active' THEN
    IF NOT(OLD.lease_generation<=>NEW.lease_generation AND OLD.fence_epoch<=>NEW.fence_epoch AND OLD.lease_token_hash<=>NEW.lease_token_hash AND OLD.last_recovery_receipt_id<=>NEW.last_recovery_receipt_id) THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet archive active cursor identity is immutable';
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
  ELSEIF OLD.status IN('blocked','reorg_detected') AND NEW.status='active' THEN
    IF NOT(NEW.archived_through_block<=>OLD.archived_through_block AND NEW.archived_through_block_hash<=>OLD.archived_through_block_hash AND NEW.last_batch_receipt_id<=>OLD.last_batch_receipt_id) OR
       NEW.last_error_code IS NOT NULL OR NEW.last_error_digest IS NOT NULL OR NEW.last_recovery_receipt_id IS NULL OR NEW.last_recovery_receipt_id<=>OLD.last_recovery_receipt_id OR
       NOT EXISTS(SELECT 1 FROM fair_launch_mainnet_archive_recovery_receipt_v1 r JOIN fair_launch_mainnet_activation_lease_v1 l ON l.id=r.replacement_lease_id
         WHERE r.id=NEW.last_recovery_receipt_id AND r.activation_row_id=NEW.activation_row_id AND r.chain_id=NEW.chain_id AND r.source_id=NEW.source_id AND r.source_revision_id=NEW.source_revision_id AND
           r.previous_status=OLD.status AND r.reason_code=OLD.last_error_code AND r.previous_error_digest=OLD.last_error_digest AND r.previous_cursor_generation=OLD.cursor_generation AND
           r.archived_through_block=OLD.archived_through_block AND r.archived_through_block_hash=OLD.archived_through_block_hash AND r.last_batch_receipt_id<=>OLD.last_batch_receipt_id AND
           l.activation_row_id=NEW.activation_row_id AND l.chain_id=NEW.chain_id AND l.lease_generation=NEW.lease_generation AND l.fence_epoch=NEW.fence_epoch AND l.lease_token_hash=NEW.lease_token_hash AND
           l.status='active' AND l.active_slot=1) THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet archive cursor recovery mismatch';
    END IF;
  ELSE
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch mainnet archive cursor transition invalid';
  END IF;
END$$
CREATE TRIGGER trg_fl_mainnet_archive_recovery_no_update BEFORE UPDATE ON fair_launch_mainnet_archive_recovery_receipt_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_archive_recovery_receipt_v1 is append-only'; END$$
CREATE TRIGGER trg_fl_mainnet_archive_recovery_no_delete BEFORE DELETE ON fair_launch_mainnet_archive_recovery_receipt_v1 FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_mainnet_archive_recovery_receipt_v1 cannot be deleted'; END$$
DELIMITER ;

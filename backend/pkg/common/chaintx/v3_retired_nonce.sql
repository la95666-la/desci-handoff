-- Controlled candidate. Does not alter any outbox/project/nonce state.
SET NAMES utf8mb4;
DROP PROCEDURE IF EXISTS jc_v3_retired_nonce_install;
DELIMITER $$
CREATE PROCEDURE jc_v3_retired_nonce_install()
BEGIN
 IF COALESCE(@JC_V3_RETIRED_NONCE_APPROVED,0)<>1 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='retired nonce migration approval required';
 END IF;
 IF EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='ido_v3_retired_nonce_evidence') THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='retired nonce evidence already installed';
 END IF;
 IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME IN ('jc_ido_v3_nonce_insert_1020','jc_ido_v3_nonce_update_1020'))<>2
 OR (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN ('ido_v3_proxy_coverage','ido_v3_event_journal','chain_relayer_nonce','chain_tx_submission','projects'))<>5 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='retired nonce prerequisites unavailable';
 END IF;
 CREATE TABLE ido_v3_retired_nonce_evidence (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  submission_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  project_id BIGINT UNSIGNED NOT NULL,
  sender_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  action VARCHAR(40) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  nonce BIGINT UNSIGNED NOT NULL,
  tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  raw_sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  receipt_block BIGINT UNSIGNED NOT NULL,
  receipt_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  journal_id BIGINT UNSIGNED NOT NULL,
  confirmed_block BIGINT UNSIGNED NOT NULL,
  confirmed_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  consumed_nonce BIGINT UNSIGNED NOT NULL,
  evidence_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  state VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'valid',
  UNIQUE KEY uk_retired_evidence_digest(evidence_digest),
  KEY idx_retired_submission(submission_id,state,expires_at),
  CONSTRAINT fk_retired_nonce_submission FOREIGN KEY(submission_id) REFERENCES chain_tx_submission(id),
  CONSTRAINT chk_retired_nonce CHECK(chain_id>0 AND consumed_nonce>nonce AND confirmed_block>=receipt_block AND state IN ('valid','revoked') AND expires_at>created_at
   AND sender_address REGEXP '^0x[0-9a-f]{40}$' AND contract_address REGEXP '^0x[0-9a-f]{40}$'
   AND tx_hash REGEXP '^0x[0-9a-f]{64}$' AND receipt_hash REGEXP '^0x[0-9a-f]{64}$'
   AND confirmed_hash REGEXP '^0x[0-9a-f]{64}$' AND raw_sha256 REGEXP '^[0-9a-f]{64}$'
   AND evidence_digest REGEXP '^[0-9a-f]{64}$')
 ) ENGINE=InnoDB;
END$$
DELIMITER ;
CALL jc_v3_retired_nonce_install();
DROP PROCEDURE jc_v3_retired_nonce_install;
DELIMITER $$
CREATE TRIGGER jc_v3_retired_nonce_no_delete BEFORE DELETE ON ido_v3_retired_nonce_evidence FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='retired nonce evidence retained'; END$$
CREATE TRIGGER jc_v3_retired_nonce_insert BEFORE INSERT ON ido_v3_retired_nonce_evidence FOR EACH ROW
BEGIN
 IF NEW.state<>'valid' OR NEW.created_at>UTC_TIMESTAMP(6) OR NEW.expires_at>UTC_TIMESTAMP(6)+INTERVAL 5 MINUTE
 OR NOT EXISTS(SELECT 1 FROM chain_tx_submission s JOIN projects p ON p.project_id=s.project_id
  WHERE s.id=NEW.submission_id AND s.chain_id=NEW.chain_id AND s.project_id=NEW.project_id
  AND s.state='v3_reorg_suspended' AND s.active_slot IS NULL AND s.receipt_status=1
  AND BINARY p.status=BINARY 'archived' AND NEW.tx_hash=LOWER(s.tx_hash)
  AND NEW.raw_sha256=LOWER(SHA2(s.raw_tx,256)) AND NEW.nonce=s.nonce
  AND NEW.sender_address=LOWER(s.sender_address) AND NEW.contract_address=LOWER(s.contract_address)
  AND NEW.action=s.action AND NEW.receipt_block=s.receipt_block_number
  AND NEW.receipt_hash=LOWER(s.receipt_block_hash) AND NEW.journal_id=s.final_event_record_id) THEN
 SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='retired nonce evidence source or TTL invalid'; END IF;
END$$
CREATE TRIGGER jc_v3_retired_nonce_update BEFORE UPDATE ON ido_v3_retired_nonce_evidence FOR EACH ROW
BEGIN
 IF NOT(OLD.state='valid' AND NEW.state='revoked' AND NEW.id=OLD.id AND NEW.submission_id=OLD.submission_id
 AND NEW.chain_id=OLD.chain_id AND NEW.project_id=OLD.project_id AND NEW.sender_address=OLD.sender_address
 AND NEW.contract_address=OLD.contract_address AND NEW.action=OLD.action AND NEW.nonce=OLD.nonce
 AND NEW.tx_hash=OLD.tx_hash AND NEW.raw_sha256=OLD.raw_sha256 AND NEW.receipt_block=OLD.receipt_block
 AND NEW.receipt_hash=OLD.receipt_hash AND NEW.journal_id=OLD.journal_id AND NEW.confirmed_block=OLD.confirmed_block
 AND NEW.confirmed_hash=OLD.confirmed_hash AND NEW.consumed_nonce=OLD.consumed_nonce
 AND NEW.evidence_digest=OLD.evidence_digest AND NEW.expires_at=OLD.expires_at AND NEW.created_at=OLD.created_at) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='retired nonce evidence immutable except revocation';
 END IF;
END$$
DELIMITER ;
-- ONE predicate shared by SQL triggers and Go signing guard (tested below).
CREATE SQL SECURITY INVOKER VIEW ido_v3_retired_nonce_exemption AS
 SELECT DISTINCT s.id FROM chain_tx_submission s
 JOIN ido_v3_retired_nonce_evidence e ON e.submission_id=s.id
 JOIN projects p ON p.project_id=s.project_id AND p.chain_id=s.chain_id
 JOIN ido_v3_event_journal j ON j.id=s.final_event_record_id
 WHERE s.state='v3_reorg_suspended' AND s.active_slot IS NULL AND s.receipt_status=1
 AND BINARY p.status=BINARY 'archived' AND p.onchain_locked=1
 AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin' AND BINARY p.contract_address=BINARY LOWER(s.contract_address)
 AND e.state='valid' AND e.expires_at>UTC_TIMESTAMP(6) AND e.created_at<=UTC_TIMESTAMP(6)
 AND e.chain_id=s.chain_id AND e.project_id=s.project_id AND e.sender_address=LOWER(s.sender_address)
 AND e.contract_address=LOWER(s.contract_address) AND e.action=s.action AND e.nonce=s.nonce
 AND e.tx_hash=LOWER(s.tx_hash) AND e.raw_sha256=LOWER(SHA2(s.raw_tx,256))
 AND e.receipt_block=s.receipt_block_number AND e.receipt_hash=LOWER(s.receipt_block_hash)
 AND e.journal_id=j.id AND j.chain_id=s.chain_id AND j.project_id=s.project_id
 AND j.proxy_address=e.contract_address AND j.contract_version='ido_v3_stablecoin' AND j.status='canonical'
 AND j.tx_hash=e.tx_hash AND j.block_number=e.receipt_block AND j.block_hash=e.receipt_hash
 AND j.event_kind=CASE s.action WHEN 'ido_v3_create' THEN 'created' WHEN 'ido_v3_activate' THEN 'ProjectActivated' WHEN 'ido_v3_end' THEN 'ProjectEnded' WHEN 'ido_v3_withdraw_platform_fee' THEN 'PlatformFeeWithdrawn' ELSE '' END
 AND NOT EXISTS(SELECT 1 FROM ido_v3_retired_nonce_evidence newer WHERE newer.submission_id=s.id AND newer.id>e.id)
 AND e.consumed_nonce>e.nonce AND e.confirmed_block>=e.receipt_block+14
 AND EXISTS(SELECT 1 FROM ido_v3_proxy_coverage c WHERE c.chain_id=s.chain_id AND c.proxy_address=e.contract_address
  AND JSON_CONTAINS(c.payload,CAST(s.project_id AS JSON),'$.RetiredTests'));
DROP TRIGGER jc_ido_v3_nonce_insert_1020;
DROP TRIGGER jc_ido_v3_nonce_update_1020;
DELIMITER $$
CREATE TRIGGER jc_ido_v3_nonce_insert_1020 BEFORE INSERT ON chain_relayer_nonce FOR EACH ROW
BEGIN
 IF NEW.active_submission_id IS NOT NULL AND EXISTS(SELECT 1 FROM chain_tx_submission s
 WHERE s.chain_id=NEW.chain_id AND BINARY LOWER(s.sender_address)=BINARY LOWER(NEW.sender_address)
 AND s.state='v3_reorg_suspended' AND NOT EXISTS(SELECT 1 FROM ido_v3_retired_nonce_exemption e WHERE e.id=s.id)) THEN
 SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V3 suspended signature fences shared sender'; END IF;
END$$
CREATE TRIGGER jc_ido_v3_nonce_update_1020 BEFORE UPDATE ON chain_relayer_nonce FOR EACH ROW
BEGIN
 IF NEW.active_submission_id IS NOT NULL AND NOT(NEW.active_submission_id<=>OLD.active_submission_id)
 AND EXISTS(SELECT 1 FROM chain_tx_submission s WHERE s.chain_id=NEW.chain_id
 AND BINARY LOWER(s.sender_address)=BINARY LOWER(NEW.sender_address) AND s.state='v3_reorg_suspended'
 AND NOT EXISTS(SELECT 1 FROM ido_v3_retired_nonce_exemption e WHERE e.id=s.id)) THEN
 SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V3 suspended signature fences shared sender'; END IF;
END$$
DELIMITER ;

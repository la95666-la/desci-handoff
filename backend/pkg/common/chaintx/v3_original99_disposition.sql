-- Explicitly authorized #99-only exception. No signer/broadcast/config enable.
-- Original raw bytes/hash/nonce remain immutable. Never label it final_revert.
SET NAMES utf8mb4;
DROP PROCEDURE IF EXISTS jc_original99_install;
DELIMITER $$
CREATE PROCEDURE jc_original99_install()
BEGIN
 IF COALESCE(@JC_ORIGINAL99_DISPOSITION_APPROVED,0)<>1 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='original99 disposition approval required';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='ido_v3_original99_disposition') THEN
  CREATE TABLE ido_v3_original99_disposition (
   original_id BIGINT UNSIGNED PRIMARY KEY,
   original_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
   original_raw_sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
   chain_id BIGINT UNSIGNED NOT NULL,
   sender_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
   nonce BIGINT UNSIGNED NOT NULL,
   target_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
   gas_limit BIGINT UNSIGNED NOT NULL,
   gas_price_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
   value_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
   raw_tx MEDIUMBLOB NOT NULL,
   tx_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
   state VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
   broadcast_count INT UNSIGNED NOT NULL DEFAULT 0,
   broadcast_marked_at DATETIME(6) NULL,
   receipt_status BIGINT UNSIGNED NULL,
   receipt_block BIGINT UNSIGNED NULL,
   receipt_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL,
   confirmed_block BIGINT UNSIGNED NULL,
   confirmed_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL,
   stream_epoch BIGINT UNSIGNED NULL,
   authorization_ref VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
   created_at DATETIME(6) NOT NULL DEFAULT (UTC_TIMESTAMP(6)),
   updated_at DATETIME(6) NOT NULL DEFAULT (UTC_TIMESTAMP(6)),
   UNIQUE KEY uk_original99_hash(tx_hash),
   CONSTRAINT fk_original99_submission FOREIGN KEY(original_id) REFERENCES chain_tx_submission(id),
   CONSTRAINT chk_original99_identity CHECK(original_id=99 AND chain_id=56 AND nonce=81
    AND original_hash='0x5bb02ae9cc1e61cf0f352c83ad536b776622b0ca8cdd9002974f18ef92d3158b'
    AND sender_address='0xfd712061ee08e070a930ecd45e935ae5d7f5445c' AND target_address=sender_address
    AND gas_limit=21000 AND gas_price_raw='100000000' AND value_raw='0'
    AND original_raw_sha256 REGEXP '^[0-9a-f]{64}$' AND tx_hash REGEXP '^0x[0-9a-f]{64}$'
    AND OCTET_LENGTH(raw_tx)>0 AND authorization_ref='operator-authorized-original99-disposition-20261031'),
   CONSTRAINT chk_original99_state CHECK(state IN ('signed','broadcast_unknown','confirmed','reorg_suspended') AND broadcast_count<=1
    AND ((state='signed' AND broadcast_count=0 AND broadcast_marked_at IS NULL)
      OR (state<>'signed' AND broadcast_count=1 AND broadcast_marked_at IS NOT NULL))
    AND (state<>'confirmed' OR (receipt_status IS NOT NULL AND receipt_block IS NOT NULL AND receipt_hash IS NOT NULL
      AND confirmed_block IS NOT NULL AND confirmed_hash IS NOT NULL AND stream_epoch IS NOT NULL AND receipt_status=1 AND receipt_block>0
      AND receipt_hash REGEXP '^0x[0-9a-f]{64}$' AND confirmed_block>=receipt_block+14
      AND confirmed_hash REGEXP '^0x[0-9a-f]{64}$' AND stream_epoch>0)))
  ) ENGINE=InnoDB;
 END IF;
 IF NOT EXISTS(SELECT 1 FROM information_schema.CHECK_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE()
  AND CONSTRAINT_NAME='chk_chain_tx_state' AND CHECK_CLAUSE LIKE '%v3_nonce_disposed%') THEN
  ALTER TABLE chain_tx_submission
   DROP CHECK chk_chain_tx_state,DROP CHECK chk_chain_tx_active_state,
   ADD CONSTRAINT chk_chain_tx_state CHECK(state IN ('intent','ready','signed','submitted','broadcast_unknown',
    'mined_success','mined_revert','final_success','final_revert','applied','cancelled','admin_cancelled',
    'ambiguous','v3_reorg_suspended','v3_nonce_disposed')) ENFORCED,
   ADD CONSTRAINT chk_chain_tx_active_state CHECK (
    (state IN ('applied','cancelled','admin_cancelled','final_revert','v3_reorg_suspended','v3_nonce_disposed') AND active_slot IS NULL)
    OR (state NOT IN ('applied','cancelled','admin_cancelled','final_revert','v3_reorg_suspended','v3_nonce_disposed') AND active_slot=1)) ENFORCED;
 END IF;
END$$
DELIMITER ;
CALL jc_original99_install();
DROP PROCEDURE jc_original99_install;
DELIMITER $$
CREATE TRIGGER IF NOT EXISTS jc_original99_insert BEFORE INSERT ON ido_v3_original99_disposition FOR EACH ROW
BEGIN
 IF NEW.state<>'signed' OR NOT EXISTS(SELECT 1 FROM chain_tx_submission s JOIN chain_relayer_nonce n
  ON n.chain_id=s.chain_id AND BINARY LOWER(n.sender_address)=BINARY LOWER(s.sender_address)
  WHERE s.id=99 AND s.project_id=43880023173627905 AND s.chain_id=56 AND s.action='ido_v3_create'
  AND s.state='signed' AND s.active_slot=1 AND s.broadcast_count=0 AND s.nonce=81
  AND s.tx_hash=NEW.original_hash AND LOWER(SHA2(s.raw_tx,256))=NEW.original_raw_sha256
  AND n.active_submission_id=99 AND n.next_nonce=82) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='original99 disposition original fence mismatch';
 END IF;
END$$
CREATE TRIGGER IF NOT EXISTS jc_original99_update BEFORE UPDATE ON ido_v3_original99_disposition FOR EACH ROW
BEGIN
 IF NOT(NEW.original_id=OLD.original_id AND NEW.original_hash=OLD.original_hash AND NEW.original_raw_sha256=OLD.original_raw_sha256
  AND NEW.chain_id=OLD.chain_id AND NEW.sender_address=OLD.sender_address AND NEW.nonce=OLD.nonce AND NEW.target_address=OLD.target_address
  AND NEW.gas_limit=OLD.gas_limit AND NEW.gas_price_raw=OLD.gas_price_raw AND NEW.value_raw=OLD.value_raw
  AND BINARY NEW.raw_tx=BINARY OLD.raw_tx AND NEW.tx_hash=OLD.tx_hash AND NEW.authorization_ref=OLD.authorization_ref
  AND NEW.created_at=OLD.created_at) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='original99 signed identity immutable'; END IF;
 IF NOT((OLD.state='signed' AND NEW.state='broadcast_unknown' AND NEW.broadcast_count=1)
  OR (OLD.state='broadcast_unknown' AND NEW.state='confirmed' AND NEW.broadcast_count=OLD.broadcast_count AND NEW.broadcast_marked_at<=>OLD.broadcast_marked_at)
  OR (OLD.state='confirmed' AND NEW.state='reorg_suspended' AND NEW.broadcast_count=OLD.broadcast_count AND NEW.broadcast_marked_at<=>OLD.broadcast_marked_at)) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='original99 disposition transition refused'; END IF;
 IF NEW.state='confirmed' AND NOT EXISTS(SELECT 1 FROM ido_v3_proxy_stream s JOIN ido_v3_proxy_coverage c
  ON c.chain_id=s.chain_id AND c.proxy_address=s.proxy_address AND c.contract_version=s.contract_version AND c.epoch=s.epoch
  WHERE s.chain_id=56 AND s.proxy_address='0x0d4040875742975269c55e16c8884d3ee9510b8e'
  AND s.state='running' AND c.block_number=NEW.receipt_block AND c.block_hash=NEW.receipt_hash
  AND s.cursor_block>=NEW.receipt_block AND NEW.stream_epoch=s.epoch) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='original99 receipt lacks canonical stream anchor'; END IF;
END$$
CREATE TRIGGER IF NOT EXISTS jc_original99_no_delete BEFORE DELETE ON ido_v3_original99_disposition FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='original99 disposition retained'; END$$
CREATE TRIGGER IF NOT EXISTS jc_original99_submission BEFORE UPDATE ON chain_tx_submission FOR EACH ROW
BEGIN
 IF NEW.state='v3_nonce_disposed' AND (OLD.state<>'signed' OR NEW.id<>99 OR NEW.active_slot IS NOT NULL
  OR NOT EXISTS(SELECT 1 FROM ido_v3_original99_disposition d WHERE d.original_id=OLD.id AND d.state='confirmed'
   AND d.original_hash=OLD.tx_hash AND d.original_raw_sha256=LOWER(SHA2(OLD.raw_tx,256)))) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='nonce disposition lacks confirmed original-linked receipt'; END IF;
 IF OLD.state='v3_nonce_disposed' AND NEW.state<>'v3_nonce_disposed' THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='disposed original retained'; END IF;
END$$
CREATE TRIGGER IF NOT EXISTS jc_original99_no_fabricated_insert BEFORE INSERT ON chain_tx_submission FOR EACH ROW
BEGIN
 IF NEW.state='v3_nonce_disposed' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='disposed original requires original transition'; END IF;
END$$
-- Fail closed after a producer epoch change/reorg, including ordinary V2
-- allocations on this sender. No permanent unanchored exemption is created.
CREATE TRIGGER IF NOT EXISTS jc_original99_nonce BEFORE UPDATE ON chain_relayer_nonce FOR EACH ROW
BEGIN
 IF OLD.active_submission_id=99 AND NEW.active_submission_id IS NULL
 AND EXISTS(SELECT 1 FROM ido_v3_original99_disposition) AND
 (NEW.next_nonce<>82 OR NOT EXISTS(SELECT 1 FROM ido_v3_original99_disposition d JOIN chain_tx_submission t ON t.id=d.original_id
  WHERE d.state='confirmed' AND t.state='v3_nonce_disposed' AND t.active_slot IS NULL)) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='original99 slot cannot clear before disposition confirmation'; END IF;
 IF NEW.active_submission_id IS NOT NULL AND NOT(NEW.active_submission_id<=>OLD.active_submission_id)
 AND NEW.chain_id=56 AND LOWER(NEW.sender_address)='0xfd712061ee08e070a930ecd45e935ae5d7f5445c'
 AND EXISTS(SELECT 1 FROM ido_v3_original99_disposition d WHERE d.state IN ('confirmed','reorg_suspended')
  AND NOT EXISTS(SELECT 1 FROM ido_v3_proxy_stream s JOIN ido_v3_proxy_coverage c
   ON c.chain_id=s.chain_id AND c.proxy_address=s.proxy_address AND c.contract_version=s.contract_version AND c.epoch=s.epoch
   WHERE s.chain_id=56 AND s.proxy_address='0x0d4040875742975269c55e16c8884d3ee9510b8e'
   AND s.state='running' AND d.state='confirmed' AND s.epoch=d.stream_epoch
   AND c.block_number=d.receipt_block AND c.block_hash=d.receipt_hash)) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='original99 disposed nonce anchor requires recovery'; END IF;
END$$
DELIMITER ;

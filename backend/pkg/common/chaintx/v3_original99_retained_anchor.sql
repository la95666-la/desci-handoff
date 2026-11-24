-- Separate operator-reviewed compatibility migration. No business row updates.
DELIMITER $$
DROP PROCEDURE IF EXISTS jc_original99_retained_install$$
CREATE PROCEDURE jc_original99_retained_install()
BEGIN
 IF COALESCE(@JC_ORIGINAL99_RETAINED_ANCHOR_APPROVED,0)<>1 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='original99 retained anchor approval required';
 END IF;
 IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME='jc_original99_nonce')<>1 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='original99 nonce protection missing';
 END IF;
END$$
CALL jc_original99_retained_install()$$
DROP PROCEDURE jc_original99_retained_install$$
DROP TRIGGER jc_original99_nonce$$
CREATE TRIGGER jc_original99_nonce BEFORE UPDATE ON chain_relayer_nonce FOR EACH ROW
BEGIN
 -- original99 retained anchor v1: preserve original nonce and immutable epoch.
 IF OLD.active_submission_id=99 AND NEW.active_submission_id IS NULL
 AND EXISTS(SELECT 1 FROM ido_v3_original99_disposition) AND
 (NEW.next_nonce<>82 OR NOT EXISTS(SELECT 1 FROM ido_v3_original99_disposition d JOIN chain_tx_submission t ON t.id=d.original_id
  WHERE d.state='confirmed' AND t.state='v3_nonce_disposed' AND t.active_slot IS NULL)) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='original99 slot cannot clear before disposition confirmation';
 END IF;
 IF NEW.active_submission_id IS NOT NULL AND NOT(NEW.active_submission_id<=>OLD.active_submission_id)
 AND NEW.chain_id=56 AND LOWER(NEW.sender_address)='0xfd712061ee08e070a930ecd45e935ae5d7f5445c'
 AND EXISTS(SELECT 1 FROM ido_v3_original99_disposition d WHERE d.state IN ('confirmed','reorg_suspended')
  AND NOT EXISTS(SELECT 1 FROM ido_v3_proxy_stream s JOIN ido_v3_proxy_coverage c
   ON c.chain_id=s.chain_id AND c.proxy_address=s.proxy_address AND c.contract_version=s.contract_version
   JOIN chain_tx_submission t ON t.id=d.original_id
   WHERE s.chain_id=56 AND s.proxy_address='0x0d4040875742975269c55e16c8884d3ee9510b8e'
   AND s.state='running' AND s.cursor_block>=d.receipt_block AND d.state='confirmed'
   AND c.epoch=d.stream_epoch AND c.block_number=d.receipt_block AND c.block_hash=d.receipt_hash
   AND d.original_id=99 AND d.chain_id=56 AND d.nonce=81 AND d.receipt_status=1
   AND d.confirmed_block>=d.receipt_block+14 AND NEW.next_nonce>d.nonce+1
   AND t.chain_id=d.chain_id AND LOWER(t.sender_address)=LOWER(NEW.sender_address)
   AND t.state='v3_nonce_disposed' AND t.active_slot IS NULL AND t.nonce=d.nonce
   AND t.tx_hash=d.original_hash AND LOWER(SHA2(t.raw_tx,256))=d.original_raw_sha256)) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='original99 disposed nonce anchor requires recovery';
 END IF;
END$$
DELIMITER ;

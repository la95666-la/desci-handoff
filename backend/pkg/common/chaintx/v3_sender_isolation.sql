-- Separate, reviewed migration. Never run as part of an application build.
-- Does not change any outbox, financial, project or nonce row.
DELIMITER $$
DROP PROCEDURE IF EXISTS jc_v3_sender_isolation_install$$
CREATE PROCEDURE jc_v3_sender_isolation_install()
BEGIN
 IF COALESCE(@JC_V3_SENDER_ISOLATION_APPROVED,0)<>1 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='sender isolation migration approval required';
 END IF;
 IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME IN ('jc_ido_v3_nonce_insert_1020','jc_ido_v3_nonce_update_1020'))<>2 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='sender isolation prerequisites unavailable';
 END IF;
END$$
CALL jc_v3_sender_isolation_install()$$
DROP PROCEDURE jc_v3_sender_isolation_install$$
DROP TRIGGER jc_ido_v3_nonce_insert_1020$$
DROP TRIGGER jc_ido_v3_nonce_update_1020$$
CREATE TRIGGER jc_ido_v3_nonce_insert_1020 BEFORE INSERT ON chain_relayer_nonce FOR EACH ROW
BEGIN
 -- V3 nonce isolation v1: old project state is not a sender-wide financial gate.
 IF NEW.active_submission_id IS NOT NULL THEN
  IF NOT EXISTS(SELECT 1 FROM chain_tx_submission a WHERE a.id=NEW.active_submission_id
   AND a.chain_id=NEW.chain_id AND BINARY LOWER(a.sender_address)=BINARY LOWER(NEW.sender_address)
   AND a.active_slot=1 AND a.nonce IS NOT NULL AND a.nonce<18446744073709551615 AND NEW.next_nonce=a.nonce+1)
  OR EXISTS(SELECT 1 FROM chain_tx_submission s JOIN chain_tx_submission a ON a.id=NEW.active_submission_id
   WHERE s.chain_id=NEW.chain_id AND BINARY LOWER(s.sender_address)=BINARY LOWER(NEW.sender_address)
   AND s.id<>a.id AND ((s.state='v3_reorg_suspended' AND (s.nonce IS NULL OR s.nonce>=a.nonce))
    OR (s.active_slot=1 AND s.nonce IS NOT NULL))) THEN
   SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V3 unresolved signature fences shared sender';
  END IF;
 END IF;
END$$
CREATE TRIGGER jc_ido_v3_nonce_update_1020 BEFORE UPDATE ON chain_relayer_nonce FOR EACH ROW
BEGIN
 -- V3 nonce isolation v1: application also verifies consumed nonce on two RPCs.
 IF NEW.active_submission_id IS NOT NULL AND NOT(NEW.active_submission_id<=>OLD.active_submission_id) THEN
  IF OLD.active_submission_id IS NOT NULL OR NEW.next_nonce<>OLD.next_nonce+1
  OR NOT EXISTS(SELECT 1 FROM chain_tx_submission a WHERE a.id=NEW.active_submission_id
   AND a.chain_id=NEW.chain_id AND BINARY LOWER(a.sender_address)=BINARY LOWER(NEW.sender_address)
   AND a.active_slot=1 AND a.nonce=OLD.next_nonce AND a.nonce<18446744073709551615)
  OR EXISTS(SELECT 1 FROM chain_tx_submission s JOIN chain_tx_submission a ON a.id=NEW.active_submission_id
   WHERE s.chain_id=NEW.chain_id AND BINARY LOWER(s.sender_address)=BINARY LOWER(NEW.sender_address)
   AND s.id<>a.id AND ((s.state='v3_reorg_suspended' AND (s.nonce IS NULL OR s.nonce>=a.nonce))
    OR (s.active_slot=1 AND s.nonce IS NOT NULL))) THEN
   SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V3 unresolved signature fences shared sender';
  END IF;
 END IF;
END$$
DELIMITER ;

-- Fail-closed rollback: disable Core feature first, quiesce nonce allocators.
-- Retain evidence history; restore conservative suspension fence, not nonce.
DROP PROCEDURE IF EXISTS jc_v3_retired_nonce_rollback;
DELIMITER $$
CREATE PROCEDURE jc_v3_retired_nonce_rollback()
BEGIN
 IF COALESCE(@JC_V3_RETIRED_NONCE_ROLLBACK_APPROVED,0)<>1 THEN
 SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='retired nonce rollback approval required'; END IF;
 UPDATE ido_v3_retired_nonce_evidence SET state='revoked' WHERE state='valid';
END$$
DELIMITER ;
CALL jc_v3_retired_nonce_rollback();
DROP PROCEDURE jc_v3_retired_nonce_rollback;
DROP TRIGGER jc_ido_v3_nonce_insert_1020;
DROP TRIGGER jc_ido_v3_nonce_update_1020;
DELIMITER $$
CREATE TRIGGER jc_ido_v3_nonce_insert_1020 BEFORE INSERT ON chain_relayer_nonce FOR EACH ROW
BEGIN
 IF NEW.active_submission_id IS NOT NULL AND EXISTS(SELECT 1 FROM chain_tx_submission s
 WHERE s.chain_id=NEW.chain_id AND BINARY LOWER(s.sender_address)=BINARY LOWER(NEW.sender_address)
 AND s.state='v3_reorg_suspended') THEN
 SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V3 suspended signature fences shared sender'; END IF;
END$$
CREATE TRIGGER jc_ido_v3_nonce_update_1020 BEFORE UPDATE ON chain_relayer_nonce FOR EACH ROW
BEGIN
 IF NEW.active_submission_id IS NOT NULL AND NOT(NEW.active_submission_id<=>OLD.active_submission_id)
 AND EXISTS(SELECT 1 FROM chain_tx_submission s WHERE s.chain_id=NEW.chain_id
 AND BINARY LOWER(s.sender_address)=BINARY LOWER(NEW.sender_address) AND s.state='v3_reorg_suspended') THEN
 SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V3 suspended signature fences shared sender'; END IF;
END$$
DELIMITER ;

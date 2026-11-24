-- JoinCare Fair Launch event-derived wallet position projection.
-- ADDITIVE ONLY. This file is executable but MUST NOT be auto-applied.
-- It does not authorize production migration, runtime activation, or Job writes.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_fair_launch_position_projection_0907;
DELIMITER $$
CREATE PROCEDURE jc_fair_launch_position_projection_0907()
BEGIN
  DECLARE v_position_check_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin;
  DECLARE v_effect_check_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin;
  IF COALESCE(@JC_FAIR_LAUNCH_MIGRATION_APPROVED,0)<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch position migration requires approved execution gate';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_action_request') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='09_06 Fair Launch action migration missing';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_event_identity_v2') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='09_04 CE-V2 identity migration missing';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='projects table missing';
  END IF;

  CREATE TABLE IF NOT EXISTS fair_launch_position (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    chain_id BIGINT UNSIGNED NOT NULL,
    contract_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    project_id BIGINT UNSIGNED NOT NULL,
    wallet_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    payment_token_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    payment_token_decimals TINYINT UNSIGNED NOT NULL,
    contribution_amount_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '0',
    contribution_amount_display VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '0',
    project_token_address CHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    project_token_decimals TINYINT UNSIGNED NOT NULL,
    claimed_tokens_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '0',
    claimed_tokens_display VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '0',
    refunded TINYINT UNSIGNED NOT NULL DEFAULT 0,
    first_contribution_event_id BIGINT UNSIGNED NULL,
    last_contribution_event_id BIGINT UNSIGNED NULL,
    last_claim_event_id BIGINT UNSIGNED NULL,
    last_refund_event_id BIGINT UNSIGNED NULL,
    status VARCHAR(20) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'canonical',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY(id),
    UNIQUE KEY uk_fl_position_identity(chain_id,contract_address,contract_version,project_id,wallet_address),
    KEY idx_fl_position_wallet(wallet_address,project_id),
    KEY idx_fl_position_first_contribution(first_contribution_event_id),
    KEY idx_fl_position_last_contribution(last_contribution_event_id),
    KEY idx_fl_position_last_claim(last_claim_event_id),
    KEY idx_fl_position_last_refund(last_refund_event_id),
    CONSTRAINT fk_fl_position_project FOREIGN KEY(project_id) REFERENCES projects(project_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT fk_fl_position_first_contribution FOREIGN KEY(first_contribution_event_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT fk_fl_position_last_contribution FOREIGN KEY(last_contribution_event_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT fk_fl_position_last_claim FOREIGN KEY(last_claim_event_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT fk_fl_position_last_refund FOREIGN KEY(last_refund_event_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT chk_fl_position CHECK(
      chain_id>0 AND project_id>0 AND contract_version='fair_launch_v1' AND
      contract_address REGEXP '^0x[0-9a-f]{40}$' AND wallet_address REGEXP '^0x[0-9a-f]{40}$' AND
      payment_token_address REGEXP '^0x[0-9a-f]{40}$' AND project_token_address REGEXP '^0x[0-9a-f]{40}$' AND
      contract_address<>'0x0000000000000000000000000000000000000000' AND wallet_address<>'0x0000000000000000000000000000000000000000' AND
      payment_token_address<>'0x0000000000000000000000000000000000000000' AND project_token_address<>'0x0000000000000000000000000000000000000000' AND
      payment_token_address<>project_token_address AND
      payment_token_decimals BETWEEN 1 AND 18 AND project_token_decimals BETWEEN 1 AND 18 AND
      contribution_amount_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND
      (CHAR_LENGTH(contribution_amount_raw)<78 OR contribution_amount_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
      claimed_tokens_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND
      (CHAR_LENGTH(claimed_tokens_raw)<78 OR claimed_tokens_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
      BINARY contribution_amount_display=BINARY CASE
        WHEN contribution_amount_raw='0' THEN '0'
        WHEN CHAR_LENGTH(contribution_amount_raw)<=payment_token_decimals THEN CONCAT('0.',TRIM(TRAILING '0' FROM LPAD(contribution_amount_raw,payment_token_decimals,'0')))
        WHEN TRIM(TRAILING '0' FROM RIGHT(contribution_amount_raw,payment_token_decimals))='' THEN LEFT(contribution_amount_raw,CHAR_LENGTH(contribution_amount_raw)-payment_token_decimals)
        ELSE CONCAT(LEFT(contribution_amount_raw,CHAR_LENGTH(contribution_amount_raw)-payment_token_decimals),'.',TRIM(TRAILING '0' FROM RIGHT(contribution_amount_raw,payment_token_decimals)))
      END AND
      BINARY claimed_tokens_display=BINARY CASE
        WHEN claimed_tokens_raw='0' THEN '0'
        WHEN CHAR_LENGTH(claimed_tokens_raw)<=project_token_decimals THEN CONCAT('0.',TRIM(TRAILING '0' FROM LPAD(claimed_tokens_raw,project_token_decimals,'0')))
        WHEN TRIM(TRAILING '0' FROM RIGHT(claimed_tokens_raw,project_token_decimals))='' THEN LEFT(claimed_tokens_raw,CHAR_LENGTH(claimed_tokens_raw)-project_token_decimals)
        ELSE CONCAT(LEFT(claimed_tokens_raw,CHAR_LENGTH(claimed_tokens_raw)-project_token_decimals),'.',TRIM(TRAILING '0' FROM RIGHT(claimed_tokens_raw,project_token_decimals)))
      END AND
      ((first_contribution_event_id IS NULL AND last_contribution_event_id IS NULL AND contribution_amount_raw='0') OR (first_contribution_event_id IS NOT NULL AND last_contribution_event_id IS NOT NULL AND contribution_amount_raw<>'0')) AND
      ((last_claim_event_id IS NULL AND claimed_tokens_raw='0') OR (last_claim_event_id IS NOT NULL AND claimed_tokens_raw<>'0')) AND
      ((refunded=0 AND last_refund_event_id IS NULL) OR (refunded=1 AND last_refund_event_id IS NOT NULL)) AND
      refunded IN (0,1) AND status IN ('canonical','quarantined')
    ) ENFORCED
  ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

  CREATE TABLE IF NOT EXISTS fair_launch_effect_application (
    event_identity_id BIGINT UNSIGNED NOT NULL,
    effect_version INT UNSIGNED NOT NULL,
    application_attempt_id BIGINT UNSIGNED NOT NULL,
    observation_id BIGINT UNSIGNED NOT NULL,
    event_name VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    payload_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    effect_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    applied_at DATETIME(6) NOT NULL,
    PRIMARY KEY(event_identity_id,effect_version),
    UNIQUE KEY uk_fl_effect_application_attempt(application_attempt_id,event_identity_id),
    UNIQUE KEY uk_fl_effect_application_observation(observation_id,event_identity_id),
    CONSTRAINT fk_fl_effect_application_identity FOREIGN KEY(event_identity_id) REFERENCES chain_event_identity_v2(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT fk_fl_effect_application_attempt FOREIGN KEY(application_attempt_id,event_identity_id) REFERENCES chain_event_application_attempt_v2(id,event_identity_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT fk_fl_effect_application_observation FOREIGN KEY(observation_id,event_identity_id) REFERENCES chain_event_placement_observation_v2(id,event_identity_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT chk_fl_effect_application CHECK(
      event_identity_id>0 AND effect_version>0 AND application_attempt_id>0 AND observation_id>0 AND
      event_name REGEXP '^FairLaunch[A-Za-z]+$' AND
      payload_digest REGEXP '^[0-9a-f]{64}$' AND effect_digest REGEXP '^[0-9a-f]{64}$'
    ) ENFORCED
  ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN ('fair_launch_position','fair_launch_effect_application') AND TABLE_TYPE='BASE TABLE' AND ENGINE='InnoDB' AND TABLE_COLLATION='utf8mb4_0900_ai_ci')<>2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair launch position table engine/collation mismatch';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_position')<>22 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_position exact column count mismatch';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_effect_application')<>8 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_effect_application exact column count mismatch';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_position' AND CONCAT_WS('|',ORDINAL_POSITION,COLUMN_NAME,LOWER(COLUMN_TYPE),IS_NULLABLE,COALESCE(COLUMN_DEFAULT,'<NULL>'),COALESCE(CHARACTER_SET_NAME,'<NULL>'),COALESCE(COLLATION_NAME,'<NULL>'),EXTRA) IN (
    '1|id|bigint unsigned|NO|<NULL>|<NULL>|<NULL>|auto_increment','2|chain_id|bigint unsigned|NO|<NULL>|<NULL>|<NULL>|','3|contract_address|char(42)|NO|<NULL>|ascii|ascii_bin|','4|contract_version|varchar(32)|NO|<NULL>|ascii|ascii_bin|','5|project_id|bigint unsigned|NO|<NULL>|<NULL>|<NULL>|','6|wallet_address|char(42)|NO|<NULL>|ascii|ascii_bin|','7|payment_token_address|char(42)|NO|<NULL>|ascii|ascii_bin|','8|payment_token_decimals|tinyint unsigned|NO|<NULL>|<NULL>|<NULL>|','9|contribution_amount_raw|varchar(78)|NO|0|ascii|ascii_bin|','10|contribution_amount_display|varchar(160)|NO|0|ascii|ascii_bin|','11|project_token_address|char(42)|NO|<NULL>|ascii|ascii_bin|','12|project_token_decimals|tinyint unsigned|NO|<NULL>|<NULL>|<NULL>|','13|claimed_tokens_raw|varchar(78)|NO|0|ascii|ascii_bin|','14|claimed_tokens_display|varchar(160)|NO|0|ascii|ascii_bin|','15|refunded|tinyint unsigned|NO|0|<NULL>|<NULL>|','16|first_contribution_event_id|bigint unsigned|YES|<NULL>|<NULL>|<NULL>|','17|last_contribution_event_id|bigint unsigned|YES|<NULL>|<NULL>|<NULL>|','18|last_claim_event_id|bigint unsigned|YES|<NULL>|<NULL>|<NULL>|','19|last_refund_event_id|bigint unsigned|YES|<NULL>|<NULL>|<NULL>|','20|status|varchar(20)|NO|canonical|ascii|ascii_bin|','21|created_at|datetime(6)|NO|CURRENT_TIMESTAMP(6)|<NULL>|<NULL>|DEFAULT_GENERATED','22|updated_at|datetime(6)|NO|CURRENT_TIMESTAMP(6)|<NULL>|<NULL>|DEFAULT_GENERATED on update CURRENT_TIMESTAMP(6)'
  ))<>22 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_position exact column definition mismatch';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_effect_application' AND CONCAT_WS('|',ORDINAL_POSITION,COLUMN_NAME,LOWER(COLUMN_TYPE),IS_NULLABLE,COALESCE(COLUMN_DEFAULT,'<NULL>'),COALESCE(CHARACTER_SET_NAME,'<NULL>'),COALESCE(COLLATION_NAME,'<NULL>'),EXTRA) IN (
    '1|event_identity_id|bigint unsigned|NO|<NULL>|<NULL>|<NULL>|','2|effect_version|int unsigned|NO|<NULL>|<NULL>|<NULL>|','3|application_attempt_id|bigint unsigned|NO|<NULL>|<NULL>|<NULL>|','4|observation_id|bigint unsigned|NO|<NULL>|<NULL>|<NULL>|','5|event_name|varchar(32)|NO|<NULL>|ascii|ascii_bin|','6|payload_digest|char(64)|NO|<NULL>|ascii|ascii_bin|','7|effect_digest|char(64)|NO|<NULL>|ascii|ascii_bin|','8|applied_at|datetime(6)|NO|<NULL>|<NULL>|<NULL>|'
  ))<>8 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_effect_application exact column definition mismatch';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_position')<>13 OR
     (SELECT GROUP_CONCAT(CONCAT(INDEX_NAME,':',NON_UNIQUE,':',SEQ_IN_INDEX,':',COLUMN_NAME) ORDER BY BINARY INDEX_NAME,SEQ_IN_INDEX SEPARATOR '|') FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_position')<>'PRIMARY:0:1:id|fk_fl_position_project:1:1:project_id|idx_fl_position_first_contribution:1:1:first_contribution_event_id|idx_fl_position_last_claim:1:1:last_claim_event_id|idx_fl_position_last_contribution:1:1:last_contribution_event_id|idx_fl_position_last_refund:1:1:last_refund_event_id|idx_fl_position_wallet:1:1:wallet_address|idx_fl_position_wallet:1:2:project_id|uk_fl_position_identity:0:1:chain_id|uk_fl_position_identity:0:2:contract_address|uk_fl_position_identity:0:3:contract_version|uk_fl_position_identity:0:4:project_id|uk_fl_position_identity:0:5:wallet_address' THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_position exact index set mismatch';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.KEY_COLUMN_USAGE WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_position' AND REFERENCED_TABLE_NAME IS NOT NULL)<>5 OR
     NOT ((SELECT GROUP_CONCAT(CONCAT(k.CONSTRAINT_NAME,':',k.COLUMN_NAME,':',k.REFERENCED_TABLE_NAME,':',k.REFERENCED_COLUMN_NAME,':',r.UPDATE_RULE,':',r.DELETE_RULE) ORDER BY BINARY k.CONSTRAINT_NAME,k.ORDINAL_POSITION SEPARATOR '|') FROM information_schema.KEY_COLUMN_USAGE k JOIN information_schema.REFERENTIAL_CONSTRAINTS r ON r.CONSTRAINT_SCHEMA=k.CONSTRAINT_SCHEMA AND r.CONSTRAINT_NAME=k.CONSTRAINT_NAME AND r.TABLE_NAME=k.TABLE_NAME WHERE k.CONSTRAINT_SCHEMA=DATABASE() AND k.TABLE_NAME='fair_launch_position' AND k.REFERENCED_TABLE_NAME IS NOT NULL) <=> 'fk_fl_position_first_contribution:first_contribution_event_id:chain_event_identity_v2:id:RESTRICT:RESTRICT|fk_fl_position_last_claim:last_claim_event_id:chain_event_identity_v2:id:RESTRICT:RESTRICT|fk_fl_position_last_contribution:last_contribution_event_id:chain_event_identity_v2:id:RESTRICT:RESTRICT|fk_fl_position_last_refund:last_refund_event_id:chain_event_identity_v2:id:RESTRICT:RESTRICT|fk_fl_position_project:project_id:projects:project_id:RESTRICT:RESTRICT') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_position exact foreign key set mismatch';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_effect_application')<>6 OR
     NOT ((SELECT GROUP_CONCAT(CONCAT(INDEX_NAME,':',NON_UNIQUE,':',SEQ_IN_INDEX,':',COLUMN_NAME) ORDER BY BINARY INDEX_NAME,SEQ_IN_INDEX SEPARATOR '|') FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_effect_application') <=> 'PRIMARY:0:1:event_identity_id|PRIMARY:0:2:effect_version|uk_fl_effect_application_attempt:0:1:application_attempt_id|uk_fl_effect_application_attempt:0:2:event_identity_id|uk_fl_effect_application_observation:0:1:observation_id|uk_fl_effect_application_observation:0:2:event_identity_id') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_effect_application exact index set mismatch';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.KEY_COLUMN_USAGE WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_effect_application' AND REFERENCED_TABLE_NAME IS NOT NULL)<>5 OR
     NOT ((SELECT GROUP_CONCAT(CONCAT(k.CONSTRAINT_NAME,':',k.COLUMN_NAME,':',k.REFERENCED_TABLE_NAME,':',k.REFERENCED_COLUMN_NAME,':',r.UPDATE_RULE,':',r.DELETE_RULE) ORDER BY BINARY k.CONSTRAINT_NAME,k.ORDINAL_POSITION SEPARATOR '|') FROM information_schema.KEY_COLUMN_USAGE k JOIN information_schema.REFERENTIAL_CONSTRAINTS r ON r.CONSTRAINT_SCHEMA=k.CONSTRAINT_SCHEMA AND r.CONSTRAINT_NAME=k.CONSTRAINT_NAME AND r.TABLE_NAME=k.TABLE_NAME WHERE k.CONSTRAINT_SCHEMA=DATABASE() AND k.TABLE_NAME='fair_launch_effect_application' AND k.REFERENCED_TABLE_NAME IS NOT NULL) <=> 'fk_fl_effect_application_attempt:application_attempt_id:chain_event_application_attempt_v2:id:RESTRICT:RESTRICT|fk_fl_effect_application_attempt:event_identity_id:chain_event_application_attempt_v2:event_identity_id:RESTRICT:RESTRICT|fk_fl_effect_application_identity:event_identity_id:chain_event_identity_v2:id:RESTRICT:RESTRICT|fk_fl_effect_application_observation:observation_id:chain_event_placement_observation_v2:id:RESTRICT:RESTRICT|fk_fl_effect_application_observation:event_identity_id:chain_event_placement_observation_v2:event_identity_id:RESTRICT:RESTRICT') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_effect_application exact foreign key set mismatch';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS tc JOIN information_schema.CHECK_CONSTRAINTS cc ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.TABLE_NAME IN ('fair_launch_position','fair_launch_effect_application') AND tc.CONSTRAINT_TYPE='CHECK' AND tc.ENFORCED='YES' AND tc.CONSTRAINT_NAME IN ('chk_fl_position','chk_fl_effect_application'))<>2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair launch position enforced check set mismatch';
  END IF;
  SET v_position_check_digest=(SELECT LOWER(SHA2(CHECK_CLAUSE,256)) FROM information_schema.CHECK_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_NAME='chk_fl_position');
  IF NOT (HEX(v_position_check_digest) <=> HEX('31935c17b91980ba9dfe92397f5c2173cc98f87114203af0e7ce0ba9670a252a')) OR
     (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_position' AND CONSTRAINT_TYPE='CHECK')<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_position exact check mismatch';
  END IF;
  SET v_effect_check_digest=(SELECT LOWER(SHA2(CHECK_CLAUSE,256)) FROM information_schema.CHECK_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_NAME='chk_fl_effect_application');
  IF NOT (HEX(v_effect_check_digest) <=> HEX('e2e243770353daacd76d70b86ec5a87740f53102f33f55c8f29e98d0aa5e5f89')) OR
     (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_effect_application' AND CONSTRAINT_TYPE='CHECK')<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_effect_application exact check mismatch';
  END IF;
END$$
DELIMITER ;
CALL jc_fair_launch_position_projection_0907();
DROP PROCEDURE jc_fair_launch_position_projection_0907;

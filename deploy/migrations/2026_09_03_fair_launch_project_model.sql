-- JoinCare Fair Launch V1 project discriminator and economic facts.
-- ADDITIVE ONLY. This file is executable but MUST NOT be auto-applied.
-- scripts/cmd/migrate.sh requires an approved manifest and explicit Fair Launch gate.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_fair_launch_project_model_0903;
DELIMITER $$
CREATE PROCEDURE jc_fair_launch_project_model_0903()
BEGIN
  IF COALESCE(@JC_FAIR_LAUNCH_MIGRATION_APPROVED,0) <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch migration requires approved execution gate';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='projects table missing';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='funding_mode') THEN
    ALTER TABLE projects ADD COLUMN funding_mode VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER contract_version;
  END IF;

  CREATE TABLE IF NOT EXISTS fair_launch_project_inventory (
    project_row_id BIGINT UNSIGNED NOT NULL,
    observed_contract_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
    inferred_funding_mode VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    inventory_state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    reason VARCHAR(255) NOT NULL,
    inventoried_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY(project_row_id),
    CONSTRAINT chk_fl_project_inventory CHECK (inferred_funding_mode IN ('capped_presale','fair_launch') AND inventory_state IN ('backfilled','quarantined')) ENFORCED
  ) ENGINE=InnoDB;
  INSERT INTO fair_launch_project_inventory(project_row_id,observed_contract_version,inferred_funding_mode,inventory_state,reason)
  SELECT id,contract_version,
    IF(contract_version='fair_launch_v1','fair_launch','capped_presale'),
    IF(contract_version='fair_launch_v1' OR contract_version IS NULL OR contract_version NOT IN ('v1_legacy','v1_legacy_unverified','v2_stablecoin','unassigned'),'quarantined','backfilled'),
    IF(contract_version='fair_launch_v1','pre-existing Fair Launch identity requires approved row evidence',IF(contract_version IS NULL OR contract_version NOT IN ('v1_legacy','v1_legacy_unverified','v2_stablecoin','unassigned'),'unknown contract version requires quarantine','known legacy inventory backfilled as capped presale'))
  FROM projects WHERE funding_mode IS NULL
  ON DUPLICATE KEY UPDATE observed_contract_version=VALUES(observed_contract_version), inferred_funding_mode=VALUES(inferred_funding_mode), inventory_state=VALUES(inventory_state), reason=VALUES(reason);
  UPDATE projects p JOIN fair_launch_project_inventory i ON i.project_row_id=p.id
    SET p.funding_mode=i.inferred_funding_mode WHERE p.funding_mode IS NULL AND i.inventory_state='backfilled';
  IF EXISTS (SELECT 1 FROM projects WHERE funding_mode IS NULL) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='funding_mode inventory quarantine unresolved; NOT NULL transition blocked';
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='funding_mode' AND IS_NULLABLE='YES') THEN
    ALTER TABLE projects MODIFY COLUMN funding_mode VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='sale_token_amount_raw') THEN
    ALTER TABLE projects ADD COLUMN sale_token_amount_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER total_deposited_raw;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='min_contribution_raw') THEN
    ALTER TABLE projects ADD COLUMN min_contribution_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER sale_token_amount_raw;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='max_contribution_raw') THEN
    ALTER TABLE projects ADD COLUMN max_contribution_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER min_contribution_raw;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='final_total_raised_raw') THEN
    ALTER TABLE projects ADD COLUMN final_total_raised_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER total_raised_raw;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='final_sale_token_amount_raw') THEN
    ALTER TABLE projects ADD COLUMN final_sale_token_amount_raw VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER final_total_raised_raw;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='payment_token_policy_revision') THEN
    ALTER TABLE projects ADD COLUMN payment_token_policy_revision INT UNSIGNED NULL AFTER payment_token_decimals;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='project_token_policy_digest') THEN
    ALTER TABLE projects ADD COLUMN project_token_policy_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER token_decimals;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='project_token_runtime_code_hash') THEN
    ALTER TABLE projects ADD COLUMN project_token_runtime_code_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER project_token_policy_digest;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='settlement_block_number') THEN
    ALTER TABLE projects ADD COLUMN settlement_block_number BIGINT UNSIGNED NULL AFTER final_sale_token_amount_raw;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='settlement_block_hash') THEN
    ALTER TABLE projects ADD COLUMN settlement_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER settlement_block_number;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='settlement_parent_block_number') THEN
    ALTER TABLE projects ADD COLUMN settlement_parent_block_number BIGINT UNSIGNED NULL AFTER settlement_block_hash;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='settlement_parent_block_hash') THEN
    ALTER TABLE projects ADD COLUMN settlement_parent_block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER settlement_parent_block_number;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND COLUMN_NAME='release_attestation_digest') THEN
    ALTER TABLE projects ADD COLUMN release_attestation_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER settlement_parent_block_hash;
  END IF;

  IF NOT EXISTS (SELECT 1 FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND CONSTRAINT_NAME='chk_projects_funding_mode_pair') THEN
    ALTER TABLE projects ADD CONSTRAINT chk_projects_funding_mode_pair CHECK (
      (funding_mode='capped_presale' AND contract_version IN ('unassigned','v1_legacy','v1_legacy_unverified','v2_stablecoin')) OR
      (funding_mode='fair_launch' AND contract_version IN ('unassigned','fair_launch_v1'))
    ) ENFORCED;
  END IF;
  CREATE TABLE IF NOT EXISTS fair_launch_uint256_quarantine (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    source_table VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    source_primary_key VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    source_column VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    raw_value VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NULL,
    failure_reason VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    inventoried_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY(id),
    UNIQUE KEY uk_fl_uint256_quarantine(source_table,source_primary_key,source_column),
    CONSTRAINT chk_fl_uint256_quarantine CHECK (failure_reason IN ('null_not_allowed','non_canonical','uint256_overflow','funding_mode_rule')) ENFORCED
  ) ENGINE=InnoDB;

  INSERT IGNORE INTO fair_launch_uint256_quarantine(source_table,source_primary_key,source_column,raw_value,failure_reason)
  SELECT 'projects',CAST(id AS CHAR),'token_price_raw',token_price_raw,
    IF(token_price_raw IS NULL,'null_not_allowed',IF(token_price_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$','non_canonical','uint256_overflow')) FROM projects
    WHERE token_price_raw IS NULL OR token_price_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$' OR (CHAR_LENGTH(token_price_raw)=78 AND token_price_raw>'115792089237316195423570985008687907853269984665640564039457584007913129639935')
  UNION ALL SELECT 'projects',CAST(id AS CHAR),'hard_cap_raw',hard_cap_raw,IF(hard_cap_raw IS NULL,'null_not_allowed',IF(hard_cap_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$','non_canonical','uint256_overflow')) FROM projects WHERE hard_cap_raw IS NULL OR hard_cap_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$' OR (CHAR_LENGTH(hard_cap_raw)=78 AND hard_cap_raw>'115792089237316195423570985008687907853269984665640564039457584007913129639935')
  UNION ALL SELECT 'projects',CAST(id AS CHAR),'soft_cap_raw',soft_cap_raw,IF(soft_cap_raw IS NULL,'null_not_allowed',IF(soft_cap_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$','non_canonical','uint256_overflow')) FROM projects WHERE soft_cap_raw IS NULL OR soft_cap_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$' OR (CHAR_LENGTH(soft_cap_raw)=78 AND soft_cap_raw>'115792089237316195423570985008687907853269984665640564039457584007913129639935')
  UNION ALL SELECT 'projects',CAST(id AS CHAR),'max_per_user_raw',max_per_user_raw,IF(max_per_user_raw IS NULL,'null_not_allowed',IF(max_per_user_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$','non_canonical','uint256_overflow')) FROM projects WHERE max_per_user_raw IS NULL OR max_per_user_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$' OR (CHAR_LENGTH(max_per_user_raw)=78 AND max_per_user_raw>'115792089237316195423570985008687907853269984665640564039457584007913129639935')
  UNION ALL SELECT 'projects',CAST(id AS CHAR),'total_raised_raw',total_raised_raw,IF(total_raised_raw IS NULL,'null_not_allowed',IF(total_raised_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$','non_canonical','uint256_overflow')) FROM projects WHERE total_raised_raw IS NULL OR total_raised_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$' OR (CHAR_LENGTH(total_raised_raw)=78 AND total_raised_raw>'115792089237316195423570985008687907853269984665640564039457584007913129639935')
  UNION ALL SELECT 'projects',CAST(id AS CHAR),'total_deposited_raw',total_deposited_raw,IF(total_deposited_raw IS NULL,'null_not_allowed',IF(total_deposited_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$','non_canonical','uint256_overflow')) FROM projects WHERE total_deposited_raw IS NULL OR total_deposited_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$' OR (CHAR_LENGTH(total_deposited_raw)=78 AND total_deposited_raw>'115792089237316195423570985008687907853269984665640564039457584007913129639935')
  UNION ALL SELECT 'projects',CAST(id AS CHAR),'sale_token_amount_raw',sale_token_amount_raw,'funding_mode_rule' FROM projects WHERE (funding_mode='fair_launch' AND (sale_token_amount_raw IS NULL OR sale_token_amount_raw='0')) OR (funding_mode='capped_presale' AND sale_token_amount_raw IS NOT NULL)
  UNION ALL SELECT 'projects',CAST(id AS CHAR),'min_contribution_raw',min_contribution_raw,'funding_mode_rule' FROM projects WHERE (funding_mode='fair_launch' AND (min_contribution_raw IS NULL OR min_contribution_raw='0')) OR (funding_mode='capped_presale' AND min_contribution_raw IS NOT NULL)
  UNION ALL SELECT 'projects',CAST(id AS CHAR),'max_contribution_raw',max_contribution_raw,'funding_mode_rule' FROM projects WHERE funding_mode='capped_presale' AND max_contribution_raw IS NOT NULL
  UNION ALL SELECT 'projects',CAST(id AS CHAR),'final_total_raised_raw',final_total_raised_raw,'funding_mode_rule' FROM projects WHERE funding_mode='capped_presale' AND final_total_raised_raw IS NOT NULL
  UNION ALL SELECT 'projects',CAST(id AS CHAR),'final_sale_token_amount_raw',final_sale_token_amount_raw,'funding_mode_rule' FROM projects WHERE funding_mode='capped_presale' AND final_sale_token_amount_raw IS NOT NULL;

  INSERT IGNORE INTO fair_launch_uint256_quarantine(source_table,source_primary_key,source_column,raw_value,failure_reason)
  SELECT 'chain_tx_submission',CAST(id AS CHAR),'value_raw',value_raw,IF(value_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$','non_canonical','uint256_overflow') FROM chain_tx_submission
    WHERE value_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$' OR (CHAR_LENGTH(value_raw)=78 AND value_raw>'115792089237316195423570985008687907853269984665640564039457584007913129639935')
  UNION ALL SELECT 'chain_tx_submission',CAST(id AS CHAR),'gas_price_raw',gas_price_raw,IF(gas_price_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$','non_canonical','uint256_overflow') FROM chain_tx_submission
    WHERE gas_price_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$' OR (CHAR_LENGTH(gas_price_raw)=78 AND gas_price_raw>'115792089237316195423570985008687907853269984665640564039457584007913129639935');

  INSERT IGNORE INTO fair_launch_uint256_quarantine(source_table,source_primary_key,source_column,raw_value,failure_reason)
  SELECT 'chain_event_reorg_generation_submission',CAST(id AS CHAR),'value_raw',value_raw,IF(value_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$','non_canonical','uint256_overflow') FROM chain_event_reorg_generation_submission
    WHERE value_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$' OR (CHAR_LENGTH(value_raw)=78 AND value_raw>'115792089237316195423570985008687907853269984665640564039457584007913129639935')
  UNION ALL SELECT 'chain_event_reorg_generation_submission',CAST(id AS CHAR),'gas_price_raw',gas_price_raw,IF(gas_price_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$','non_canonical','uint256_overflow') FROM chain_event_reorg_generation_submission
    WHERE gas_price_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$' OR (CHAR_LENGTH(gas_price_raw)=78 AND gas_price_raw>'115792089237316195423570985008687907853269984665640564039457584007913129639935')
  UNION ALL SELECT 'chain_event_reorg_candidate_project',CAST(id AS CHAR),'project_id_raw',project_id_raw,IF(project_id_raw NOT REGEXP '^[1-9][0-9]{0,77}$','non_canonical','uint256_overflow') FROM chain_event_reorg_candidate_project
    WHERE project_id_raw NOT REGEXP '^[1-9][0-9]{0,77}$' OR (CHAR_LENGTH(project_id_raw)=78 AND project_id_raw>'115792089237316195423570985008687907853269984665640564039457584007913129639935')
  UNION ALL SELECT 'chain_event_reorg_candidate_wallet_aggregate',CAST(id AS CHAR),'project_id_raw',project_id_raw,IF(project_id_raw NOT REGEXP '^[1-9][0-9]{0,77}$','non_canonical','uint256_overflow') FROM chain_event_reorg_candidate_wallet_aggregate
    WHERE project_id_raw NOT REGEXP '^[1-9][0-9]{0,77}$' OR (CHAR_LENGTH(project_id_raw)=78 AND project_id_raw>'115792089237316195423570985008687907853269984665640564039457584007913129639935')
  UNION ALL SELECT 'chain_event_reorg_candidate_wallet_aggregate',CAST(id AS CHAR),'amount_raw',amount_raw,IF(amount_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$','non_canonical','uint256_overflow') FROM chain_event_reorg_candidate_wallet_aggregate
    WHERE amount_raw NOT REGEXP '^(0|[1-9][0-9]{0,77})$' OR (CHAR_LENGTH(amount_raw)=78 AND amount_raw>'115792089237316195423570985008687907853269984665640564039457584007913129639935');

  IF EXISTS (SELECT 1 FROM fair_launch_uint256_quarantine) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='financial raw inventory quarantine unresolved; uint256 constraints blocked';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND CONSTRAINT_NAME='chk_projects_uint256_raw') THEN
    ALTER TABLE projects ADD CONSTRAINT chk_projects_uint256_raw CHECK (
      token_price_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(token_price_raw)<78 OR token_price_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
      hard_cap_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(hard_cap_raw)<78 OR hard_cap_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
      soft_cap_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(soft_cap_raw)<78 OR soft_cap_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
      max_per_user_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(max_per_user_raw)<78 OR max_per_user_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
      total_raised_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(total_raised_raw)<78 OR total_raised_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
      total_deposited_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(total_deposited_raw)<78 OR total_deposited_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
      (sale_token_amount_raw IS NULL OR (sale_token_amount_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(sale_token_amount_raw)<78 OR sale_token_amount_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935'))) AND
      (min_contribution_raw IS NULL OR (min_contribution_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(min_contribution_raw)<78 OR min_contribution_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935'))) AND
      (max_contribution_raw IS NULL OR (max_contribution_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(max_contribution_raw)<78 OR max_contribution_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935'))) AND
      (final_total_raised_raw IS NULL OR (final_total_raised_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(final_total_raised_raw)<78 OR final_total_raised_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935'))) AND
      (final_sale_token_amount_raw IS NULL OR (final_sale_token_amount_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(final_sale_token_amount_raw)<78 OR final_sale_token_amount_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935')))
    ) ENFORCED;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='chain_tx_submission' AND CONSTRAINT_NAME='chk_chain_tx_uint256_raw') THEN
    ALTER TABLE chain_tx_submission ADD CONSTRAINT chk_chain_tx_uint256_raw CHECK (
      value_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(value_raw)<78 OR value_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
      gas_price_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(gas_price_raw)<78 OR gas_price_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935')
    ) ENFORCED;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='chain_event_reorg_generation_submission' AND CONSTRAINT_NAME='chk_reorg_submission_uint256_raw') THEN
    ALTER TABLE chain_event_reorg_generation_submission ADD CONSTRAINT chk_reorg_submission_uint256_raw CHECK (
      value_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(value_raw)<78 OR value_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
      gas_price_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(gas_price_raw)<78 OR gas_price_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935')
    ) ENFORCED;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='chain_event_reorg_candidate_project' AND CONSTRAINT_NAME='chk_reorg_candidate_project_uint256_raw') THEN
    ALTER TABLE chain_event_reorg_candidate_project ADD CONSTRAINT chk_reorg_candidate_project_uint256_raw CHECK (project_id_raw REGEXP '^[1-9][0-9]{0,77}$' AND (CHAR_LENGTH(project_id_raw)<78 OR project_id_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935')) ENFORCED;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='chain_event_reorg_candidate_wallet_aggregate' AND CONSTRAINT_NAME='chk_reorg_candidate_wallet_uint256_raw') THEN
    ALTER TABLE chain_event_reorg_candidate_wallet_aggregate ADD CONSTRAINT chk_reorg_candidate_wallet_uint256_raw CHECK (
      project_id_raw REGEXP '^[1-9][0-9]{0,77}$' AND (CHAR_LENGTH(project_id_raw)<78 OR project_id_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935') AND
      amount_raw REGEXP '^(0|[1-9][0-9]{0,77})$' AND (CHAR_LENGTH(amount_raw)<78 OR amount_raw<='115792089237316195423570985008687907853269984665640564039457584007913129639935')
    ) ENFORCED;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='projects' AND CONSTRAINT_NAME='chk_projects_funding_raw_rules') THEN
    ALTER TABLE projects ADD CONSTRAINT chk_projects_funding_raw_rules CHECK (
      (funding_mode='capped_presale' AND sale_token_amount_raw IS NULL AND min_contribution_raw IS NULL AND max_contribution_raw IS NULL AND final_total_raised_raw IS NULL AND final_sale_token_amount_raw IS NULL) OR
      (funding_mode='fair_launch' AND token_price_raw='0' AND hard_cap_raw='0' AND max_per_user_raw='0' AND sale_token_amount_raw IS NOT NULL AND sale_token_amount_raw<>'0' AND soft_cap_raw<>'0' AND min_contribution_raw IS NOT NULL AND min_contribution_raw<>'0')
    ) ENFORCED;
  END IF;
END$$
DELIMITER ;
CALL jc_fair_launch_project_model_0903();
DROP PROCEDURE jc_fair_launch_project_model_0903;

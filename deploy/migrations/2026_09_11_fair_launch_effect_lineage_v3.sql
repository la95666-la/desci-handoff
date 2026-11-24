-- JoinCare Fair Launch v3 application lineage, guard, journal, attestation and cursor checkpoint.
-- ADDITIVE LOCAL-ONLY candidate. Production Apply remains disabled.
SET NAMES utf8mb4;
DROP PROCEDURE IF EXISTS jc_fair_launch_local_e2e_0911;
DELIMITER $$
CREATE PROCEDURE jc_fair_launch_local_e2e_0911()
BEGIN
  IF COALESCE(@JC_FAIR_LAUNCH_LOCAL_E2E_APPROVED,0)<>1 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch local E2E migration requires local-only execution gate'; END IF;
  IF NOT EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_governance_current_v3') THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='09_10 Fair Launch governance projection migration missing'; END IF;
  IF NOT EXISTS(SELECT 1 FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='chain_event_placement_v2' AND INDEX_NAME='uk_ce_placement_lineage_generation_v3') THEN
    ALTER TABLE chain_event_placement_v2 ADD UNIQUE KEY uk_ce_placement_lineage_generation_v3(id,event_identity_id,source_revision_id,generation);
  END IF;
END$$
DELIMITER ;
CALL jc_fair_launch_local_e2e_0911();
DROP PROCEDURE jc_fair_launch_local_e2e_0911;

CREATE TABLE IF NOT EXISTS fair_launch_application_lineage_v3 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  ce_application_attempt_id BIGINT UNSIGNED NOT NULL,
  run_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_binding_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  event_identity_id BIGINT UNSIGNED NOT NULL,
  placement_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  effect_manifest_version INT UNSIGNED NOT NULL,
  placement_generation INT UNSIGNED NOT NULL,
  predecessor_lineage_id BIGINT UNSIGNED NULL,
  reverses_lineage_id BIGINT UNSIGNED NULL,
  disposition VARCHAR(20) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  expected_effect TINYINT UNSIGNED NOT NULL,
  payload_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  effect_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  ledger_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  mutation_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  applied_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_lineage_ce_application(ce_application_attempt_id),
  UNIQUE KEY uk_fl_lineage_provenance(id,event_identity_id,placement_id,source_revision_id,effect_manifest_version),
  UNIQUE KEY uk_fl_lineage_chain_provenance(id,run_row_id,chain_id,source_id,event_identity_id,placement_id,source_revision_id,effect_manifest_version),
  UNIQUE KEY uk_fl_lineage_full_provenance(id,run_row_id,chain_id,source_binding_id,source_id,event_identity_id,placement_id,source_revision_id,effect_manifest_version),
  UNIQUE KEY uk_fl_lineage_chain_event_effect(id,run_row_id,chain_id,source_id,event_identity_id,effect_manifest_version),
  UNIQUE KEY uk_fl_lineage_chain_source(id,run_row_id,chain_id,source_id),
  UNIQUE KEY uk_fl_lineage_branch(id,run_row_id,chain_id,source_binding_id,source_id,source_revision_id),
  UNIQUE KEY uk_fl_lineage_branch_effect(id,run_row_id,chain_id,source_binding_id,source_id,source_revision_id,effect_manifest_version),
  UNIQUE KEY uk_fl_lineage_event_effect(id,event_identity_id,effect_manifest_version),
  UNIQUE KEY uk_fl_lineage_generation(event_identity_id,effect_manifest_version,placement_generation,disposition),
  CONSTRAINT fk_fl_lineage_source_binding FOREIGN KEY(source_binding_id,run_row_id,chain_id,source_id,source_revision_id) REFERENCES fair_launch_source_revision_binding_v3(id,run_row_id,chain_id,source_id,source_revision_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_lineage_event_chain FOREIGN KEY(event_identity_id,chain_id) REFERENCES chain_event_identity_v2(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_lineage_ce_application FOREIGN KEY(ce_application_attempt_id,event_identity_id,placement_id,effect_manifest_version) REFERENCES chain_event_application_attempt_v2(id,event_identity_id,placement_id,effect_version) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_lineage_placement_revision FOREIGN KEY(placement_id,event_identity_id,source_revision_id,placement_generation) REFERENCES chain_event_placement_v2(id,event_identity_id,source_revision_id,generation) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_lineage_predecessor FOREIGN KEY(predecessor_lineage_id,run_row_id,chain_id,source_binding_id,source_id,source_revision_id) REFERENCES fair_launch_application_lineage_v3(id,run_row_id,chain_id,source_binding_id,source_id,source_revision_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_lineage_reverses FOREIGN KEY(reverses_lineage_id,run_row_id,chain_id,source_binding_id,source_id,source_revision_id) REFERENCES fair_launch_application_lineage_v3(id,run_row_id,chain_id,source_binding_id,source_id,source_revision_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_lineage CHECK(
    chain_id=31337 AND effect_manifest_version>0 AND placement_generation>0 AND expected_effect IN(0,1) AND
    disposition IN('forward','replay_no_op','reversal','blocked','rejected') AND state IN('reserved','applied','blocked','failed') AND
    payload_digest REGEXP '^[0-9a-f]{64}$' AND effect_digest REGEXP '^[0-9a-f]{64}$' AND ledger_digest REGEXP '^[0-9a-f]{64}$' AND mutation_digest REGEXP '^[0-9a-f]{64}$' AND
    ((disposition='reversal' AND reverses_lineage_id IS NOT NULL) OR (disposition<>'reversal' AND reverses_lineage_id IS NULL))
  ) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- One durable lock row exists for every forward lineage before any reversal is
-- prepared. All reversal-set writers lock this same row. The pair is bound once
-- by the append-only lineage reversal link; closure changes only this guard to
-- closed in the same transaction that inserts immutable closure evidence.
CREATE TABLE IF NOT EXISTS fair_launch_reversal_serialization_guard_v3 (
  reversed_lineage_id BIGINT UNSIGNED NOT NULL,
  reversal_lineage_id BIGINT UNSIGNED NULL,
  run_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_binding_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  bound_at DATETIME(6) NULL,
  closed_at DATETIME(6) NULL,
  PRIMARY KEY(reversed_lineage_id),
  UNIQUE KEY uk_fl_reversal_guard_pair(reversed_lineage_id,reversal_lineage_id),
  UNIQUE KEY uk_fl_reversal_guard_application(reversal_lineage_id),
  CONSTRAINT fk_fl_reversal_guard_forward FOREIGN KEY(reversed_lineage_id,run_row_id,chain_id,source_binding_id,source_id,source_revision_id) REFERENCES fair_launch_application_lineage_v3(id,run_row_id,chain_id,source_binding_id,source_id,source_revision_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_reversal_guard_reverse FOREIGN KEY(reversal_lineage_id,run_row_id,chain_id,source_binding_id,source_id,source_revision_id) REFERENCES fair_launch_application_lineage_v3(id,run_row_id,chain_id,source_binding_id,source_id,source_revision_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_reversal_guard CHECK(chain_id=31337 AND state IN('open','bound','closed') AND ((state='open' AND reversal_lineage_id IS NULL AND bound_at IS NULL AND closed_at IS NULL) OR (state='bound' AND reversal_lineage_id IS NOT NULL AND bound_at IS NOT NULL AND closed_at IS NULL) OR (state='closed' AND reversal_lineage_id IS NOT NULL AND bound_at IS NOT NULL AND closed_at IS NOT NULL))) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_effect_guard_v3 (
  run_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  event_identity_id BIGINT UNSIGNED NOT NULL,
  effect_manifest_version INT UNSIGNED NOT NULL,
  placement_generation INT UNSIGNED NOT NULL,
  source_application_lineage_id BIGINT UNSIGNED NOT NULL,
  state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  active_slot TINYINT UNSIGNED NULL,
  payload_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  effect_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(event_identity_id,effect_manifest_version,placement_generation),
  UNIQUE KEY uk_fl_effect_guard_active(event_identity_id,effect_manifest_version,active_slot),
  CONSTRAINT fk_fl_effect_guard_lineage FOREIGN KEY(source_application_lineage_id,run_row_id,chain_id,source_id,event_identity_id,effect_manifest_version) REFERENCES fair_launch_application_lineage_v3(id,run_row_id,chain_id,source_id,event_identity_id,effect_manifest_version) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_effect_guard CHECK(chain_id=31337 AND effect_manifest_version>0 AND placement_generation>0 AND state IN('reserved','applied','reversed','replay_no_op','blocked') AND (active_slot IS NULL OR active_slot=1) AND payload_digest REGEXP '^[0-9a-f]{64}$' AND effect_digest REGEXP '^[0-9a-f]{64}$' AND ((state IN('reserved','applied') AND active_slot=1) OR (state NOT IN('reserved','applied') AND active_slot IS NULL))) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_effect_journal_v3 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  event_identity_id BIGINT UNSIGNED NOT NULL,
  placement_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  source_application_lineage_id BIGINT UNSIGNED NOT NULL,
  effect_manifest_version INT UNSIGNED NOT NULL,
  effect_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  aggregate_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  aggregate_key VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  mutation_ordinal INT UNSIGNED NOT NULL,
  before_payload_jcs LONGTEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  before_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  after_payload_jcs LONGTEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  after_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_journal_mutation(source_application_lineage_id,mutation_ordinal),
  UNIQUE KEY uk_fl_journal_lineage_identity(id,source_application_lineage_id),
  KEY idx_fl_journal_aggregate(aggregate_kind,aggregate_key,source_application_lineage_id),
  CONSTRAINT fk_fl_journal_lineage FOREIGN KEY(source_application_lineage_id,run_row_id,chain_id,source_id,event_identity_id,placement_id,source_revision_id,effect_manifest_version) REFERENCES fair_launch_application_lineage_v3(id,run_row_id,chain_id,source_id,event_identity_id,placement_id,source_revision_id,effect_manifest_version) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_journal CHECK(chain_id=31337 AND effect_manifest_version>0 AND mutation_ordinal>0 AND effect_kind REGEXP '^[a-z][a-z0-9_]{2,31}$' AND aggregate_kind IN('project','position','governance','policy') AND CHAR_LENGTH(aggregate_key)>0 AND JSON_VALID(before_payload_jcs) AND JSON_VALID(after_payload_jcs) AND before_digest REGEXP '^[0-9a-f]{64}$' AND after_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Forward lineage and journals remain immutable. Reversal is represented by
-- one-time append-only links inserted in the same transaction as compensation.
CREATE TABLE IF NOT EXISTS fair_launch_lineage_reversal_v3 (
  reversed_lineage_id BIGINT UNSIGNED NOT NULL,
  reversal_lineage_id BIGINT UNSIGNED NOT NULL,
  reversed_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(reversed_lineage_id),
  UNIQUE KEY uk_fl_lineage_reversal_pair(reversed_lineage_id,reversal_lineage_id),
  UNIQUE KEY uk_fl_lineage_reversal_application(reversal_lineage_id),
  CONSTRAINT fk_fl_lineage_reversal_source FOREIGN KEY(reversed_lineage_id) REFERENCES fair_launch_application_lineage_v3(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_lineage_reversal_application FOREIGN KEY(reversal_lineage_id) REFERENCES fair_launch_application_lineage_v3(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_lineage_reversal CHECK(reversed_lineage_id<>reversal_lineage_id) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_journal_reversal_v3 (
  source_journal_id BIGINT UNSIGNED NOT NULL,
  reversed_lineage_id BIGINT UNSIGNED NOT NULL,
  reversal_lineage_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(source_journal_id),
  KEY idx_fl_journal_reversal_application(reversal_lineage_id,source_journal_id),
  CONSTRAINT fk_fl_journal_reversal_source FOREIGN KEY(source_journal_id,reversed_lineage_id) REFERENCES fair_launch_effect_journal_v3(id,source_application_lineage_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_journal_reversal_pair FOREIGN KEY(reversed_lineage_id,reversal_lineage_id) REFERENCES fair_launch_lineage_reversal_v3(reversed_lineage_id,reversal_lineage_id) ON UPDATE RESTRICT ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_reversal_evidence_item_v3 (
  reversed_lineage_id BIGINT UNSIGNED NOT NULL,
  reversal_lineage_id BIGINT UNSIGNED NOT NULL,
  evidence_ordinal BIGINT UNSIGNED NOT NULL,
  evidence_kind VARCHAR(40) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  evidence_id BIGINT UNSIGNED NOT NULL,
  evidence_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  previous_chain_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  item_chain_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(reversed_lineage_id,evidence_ordinal),
  UNIQUE KEY uk_fl_reversal_evidence_natural(reversed_lineage_id,evidence_kind,evidence_id),
  UNIQUE KEY uk_fl_reversal_evidence_item_pair(reversed_lineage_id,reversal_lineage_id,evidence_ordinal,item_chain_digest),
  CONSTRAINT fk_fl_reversal_evidence_item_pair FOREIGN KEY(reversed_lineage_id,reversal_lineage_id) REFERENCES fair_launch_lineage_reversal_v3(reversed_lineage_id,reversal_lineage_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_reversal_evidence_item CHECK(evidence_ordinal>0 AND evidence_kind IN('source_journal','dependency','attestation','cursor_checkpoint','governance_history','policy_history','source_ledger','source_governance_ledger') AND evidence_digest REGEXP '^[0-9a-f]{64}$' AND previous_chain_digest REGEXP '^[0-9a-f]{64}$' AND item_chain_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_reversal_evidence_manifest_v3 (
  reversed_lineage_id BIGINT UNSIGNED NOT NULL,
  reversal_lineage_id BIGINT UNSIGNED NOT NULL,
  source_journal_count BIGINT UNSIGNED NOT NULL,
  dependency_count BIGINT UNSIGNED NOT NULL,
  attestation_count BIGINT UNSIGNED NOT NULL,
  cursor_checkpoint_count BIGINT UNSIGNED NOT NULL,
  governance_history_count BIGINT UNSIGNED NOT NULL,
  policy_history_count BIGINT UNSIGNED NOT NULL,
  source_ledger_count BIGINT UNSIGNED NOT NULL,
  source_governance_ledger_count BIGINT UNSIGNED NOT NULL,
  evidence_item_count BIGINT UNSIGNED NOT NULL,
  final_item_chain_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  manifest_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(reversed_lineage_id),
  UNIQUE KEY uk_fl_reversal_evidence_application(reversal_lineage_id),
  UNIQUE KEY uk_fl_reversal_evidence_identity(reversed_lineage_id,reversal_lineage_id,manifest_digest),
  CONSTRAINT fk_fl_reversal_evidence_pair FOREIGN KEY(reversed_lineage_id,reversal_lineage_id) REFERENCES fair_launch_lineage_reversal_v3(reversed_lineage_id,reversal_lineage_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_reversal_evidence CHECK(evidence_item_count=source_journal_count+dependency_count+attestation_count+cursor_checkpoint_count+governance_history_count+policy_history_count+source_ledger_count+source_governance_ledger_count AND final_item_chain_digest REGEXP '^[0-9a-f]{64}$' AND manifest_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_reversal_closure_v3 (
  reversed_lineage_id BIGINT UNSIGNED NOT NULL,
  reversal_lineage_id BIGINT UNSIGNED NOT NULL,
  evidence_manifest_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_journal_count BIGINT UNSIGNED NOT NULL,
  linked_journal_count BIGINT UNSIGNED NOT NULL,
  dependency_count BIGINT UNSIGNED NOT NULL,
  attestation_count BIGINT UNSIGNED NOT NULL,
  cursor_checkpoint_count BIGINT UNSIGNED NOT NULL,
  governance_history_count BIGINT UNSIGNED NOT NULL,
  policy_history_count BIGINT UNSIGNED NOT NULL,
  source_ledger_count BIGINT UNSIGNED NOT NULL,
  compensation_ledger_count BIGINT UNSIGNED NOT NULL,
  source_governance_ledger_count BIGINT UNSIGNED NOT NULL,
  compensation_governance_ledger_count BIGINT UNSIGNED NOT NULL,
  closure_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(reversed_lineage_id),
  UNIQUE KEY uk_fl_reversal_closure_application(reversal_lineage_id),
  CONSTRAINT fk_fl_reversal_closure_pair FOREIGN KEY(reversed_lineage_id,reversal_lineage_id) REFERENCES fair_launch_lineage_reversal_v3(reversed_lineage_id,reversal_lineage_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_reversal_closure_manifest FOREIGN KEY(reversed_lineage_id,reversal_lineage_id,evidence_manifest_digest) REFERENCES fair_launch_reversal_evidence_manifest_v3(reversed_lineage_id,reversal_lineage_id,manifest_digest) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_reversal_closure CHECK(source_journal_count=linked_journal_count AND source_ledger_count=compensation_ledger_count AND source_governance_ledger_count=compensation_governance_ledger_count AND evidence_manifest_digest REGEXP '^[0-9a-f]{64}$' AND closure_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_effect_dependency_v3 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  predecessor_lineage_id BIGINT UNSIGNED NOT NULL,
  dependent_lineage_id BIGINT UNSIGNED NOT NULL,
  run_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_binding_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  effect_manifest_version INT UNSIGNED NOT NULL,
  aggregate_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  aggregate_key VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  dependency_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_dependency_natural(predecessor_lineage_id,dependent_lineage_id,aggregate_kind,aggregate_key),
  UNIQUE KEY uk_fl_dependency_identity(id,dependent_lineage_id,dependency_digest),
  CONSTRAINT fk_fl_dependency_predecessor FOREIGN KEY(predecessor_lineage_id,run_row_id,chain_id,source_binding_id,source_id,source_revision_id,effect_manifest_version) REFERENCES fair_launch_application_lineage_v3(id,run_row_id,chain_id,source_binding_id,source_id,source_revision_id,effect_manifest_version) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_dependency_dependent FOREIGN KEY(dependent_lineage_id,run_row_id,chain_id,source_binding_id,source_id,source_revision_id,effect_manifest_version) REFERENCES fair_launch_application_lineage_v3(id,run_row_id,chain_id,source_binding_id,source_id,source_revision_id,effect_manifest_version) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_dependency CHECK(chain_id=31337 AND effect_manifest_version>0 AND predecessor_lineage_id<>dependent_lineage_id AND aggregate_kind IN('project','position','governance','policy') AND CHAR_LENGTH(aggregate_key)>0 AND dependency_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_effect_attestation_v3 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_binding_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  event_identity_id BIGINT UNSIGNED NOT NULL,
  placement_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  source_application_lineage_id BIGINT UNSIGNED NOT NULL,
  effect_manifest_version INT UNSIGNED NOT NULL,
  payload_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  effect_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  ledger_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  mutation_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  ledger_row_count BIGINT UNSIGNED NOT NULL,
  mutation_row_count BIGINT UNSIGNED NOT NULL,
  project_mutation_count BIGINT UNSIGNED NOT NULL,
  position_mutation_count BIGINT UNSIGNED NOT NULL,
  governance_mutation_count BIGINT UNSIGNED NOT NULL,
  policy_mutation_count BIGINT UNSIGNED NOT NULL,
  attestation_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_attestation_lineage(source_application_lineage_id),
  CONSTRAINT fk_fl_attestation_lineage FOREIGN KEY(source_application_lineage_id,run_row_id,chain_id,source_binding_id,source_id,event_identity_id,placement_id,source_revision_id,effect_manifest_version) REFERENCES fair_launch_application_lineage_v3(id,run_row_id,chain_id,source_binding_id,source_id,event_identity_id,placement_id,source_revision_id,effect_manifest_version) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_attestation CHECK(chain_id=31337 AND effect_manifest_version>0 AND payload_digest REGEXP '^[0-9a-f]{64}$' AND effect_digest REGEXP '^[0-9a-f]{64}$' AND ledger_digest REGEXP '^[0-9a-f]{64}$' AND mutation_digest REGEXP '^[0-9a-f]{64}$' AND attestation_digest REGEXP '^[0-9a-f]{64}$' AND mutation_row_count=project_mutation_count+position_mutation_count+governance_mutation_count+policy_mutation_count) ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fair_launch_cursor_checkpoint_v3 (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  run_row_id BIGINT UNSIGNED NOT NULL,
  chain_id BIGINT UNSIGNED NOT NULL,
  source_binding_id BIGINT UNSIGNED NOT NULL,
  source_id BIGINT UNSIGNED NOT NULL,
  source_revision_id BIGINT UNSIGNED NOT NULL,
  predecessor_block_number BIGINT UNSIGNED NOT NULL,
  predecessor_block_hash CHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  predecessor_tx_index INT UNSIGNED NOT NULL,
  predecessor_tx_log_ordinal INT UNSIGNED NOT NULL,
  predecessor_event_identity_id BIGINT UNSIGNED NULL,
  reason VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  reversal_application_lineage_id BIGINT UNSIGNED NOT NULL,
  checkpoint_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_cursor_checkpoint_reversal(reversal_application_lineage_id),
  CONSTRAINT fk_fl_cursor_checkpoint_run_chain FOREIGN KEY(run_row_id,chain_id) REFERENCES fair_launch_local_e2e_run(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_cursor_checkpoint_source_chain FOREIGN KEY(source_id,chain_id) REFERENCES chain_contract_source(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_cursor_checkpoint_revision FOREIGN KEY(source_revision_id,source_id) REFERENCES chain_contract_source_revision(id,source_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_cursor_checkpoint_event FOREIGN KEY(predecessor_event_identity_id,chain_id) REFERENCES chain_event_identity_v2(id,chain_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_cursor_checkpoint_reversal FOREIGN KEY(reversal_application_lineage_id,run_row_id,chain_id,source_binding_id,source_id,source_revision_id) REFERENCES fair_launch_application_lineage_v3(id,run_row_id,chain_id,source_binding_id,source_id,source_revision_id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_cursor_checkpoint CHECK(chain_id=31337 AND predecessor_block_hash REGEXP '^0x[0-9a-f]{64}$' AND reason IN('leaf_reorg','replacement','manual_test_reset') AND checkpoint_digest REGEXP '^[0-9a-f]{64}$') ENFORCED
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

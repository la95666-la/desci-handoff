-- JoinCare IDO V2-only registered-source policy.
-- This additive migration allows a chain environment to attest that no legacy
-- V1/JCC event source was ever registered. It does not satisfy historical V1
-- origin requirements, build a candidate, switch projections, or resolve an
-- incident.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_chain_event_v2_only_source_policy;
DELIMITER $$
CREATE PROCEDURE jc_preflight_chain_event_v2_only_source_policy()
BEGIN
  IF (SELECT COUNT(*) FROM schema_migrations WHERE status='applied'
      AND filename='2026_08_28_chain_event_reorg_generation_foundation.sql'
      AND checksum='467eebfacca807f72271b0414624c787e48fd4f0e0cebb834fcfaba2a4b04b15')<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V2-only source policy requires the frozen 08_28 foundation';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'
      AND TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_materialization_run','chain_event_reorg_generation'))<>3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V2-only source policy run tables are incomplete';
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_materialization_run','chain_event_reorg_generation')
      AND COLUMN_NAME='legacy_source_registered') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='pre-existing V2-only source policy columns require manual verification';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_chain_event_v2_only_source_policy();
DROP PROCEDURE jc_preflight_chain_event_v2_only_source_policy;

ALTER TABLE chain_event_reorg_recovery_run
  ADD COLUMN legacy_source_registered TINYINT UNSIGNED NOT NULL DEFAULT 1 AFTER legacy_contract_address,
  DROP CHECK chk_chain_event_reorg_recovery_run_identity,
  ADD CONSTRAINT chk_chain_event_reorg_recovery_run_identity CHECK (
    incident_id>0 AND attempt_no>0 AND chain_id>0
    AND run_uuid REGEXP '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
    AND contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND contract_address<>'0x0000000000000000000000000000000000000000'
    AND legacy_source_registered IN (0,1)
    AND ((legacy_source_registered=1
      AND legacy_contract_address REGEXP '^0x[0-9a-f]{40}$'
      AND legacy_contract_address<>'0x0000000000000000000000000000000000000000'
      AND legacy_contract_address<>contract_address)
      OR (legacy_source_registered=0 AND legacy_contract_address=''))
    AND contract_version='v2_stablecoin'
    AND algorithm_version='joincare-reorg-recovery-scan-v1'
  ) ENFORCED;

ALTER TABLE chain_event_reorg_materialization_run
  ADD COLUMN legacy_source_registered TINYINT UNSIGNED NOT NULL DEFAULT 1 AFTER legacy_contract_address,
  DROP CHECK chk_chain_event_reorg_materialization_run_identity,
  ADD CONSTRAINT chk_chain_event_reorg_materialization_run_identity CHECK (
    recovery_run_id>0 AND incident_id>0 AND chain_id>0
    AND run_uuid REGEXP '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
    AND algorithm_version='joincare-reorg-materialization-prepare-v1'
    AND contract_address REGEXP '^0x[0-9a-f]{40}$'
    AND contract_address<>'0x0000000000000000000000000000000000000000'
    AND legacy_source_registered IN (0,1)
    AND ((legacy_source_registered=1
      AND legacy_contract_address REGEXP '^0x[0-9a-f]{40}$'
      AND legacy_contract_address<>'0x0000000000000000000000000000000000000000'
      AND contract_address<>legacy_contract_address)
      OR (legacy_source_registered=0 AND legacy_contract_address=''))
  ) ENFORCED;

ALTER TABLE chain_event_reorg_generation
  ADD COLUMN legacy_source_registered TINYINT UNSIGNED NOT NULL DEFAULT 1 AFTER legacy_contract_address,
  DROP CHECK chk_chain_event_reorg_generation_sources,
  ADD CONSTRAINT chk_chain_event_reorg_generation_sources CHECK (
    prerequisite_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND assessment_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND materialization_manifest_digest REGEXP '^[0-9a-f]{64}$'
    AND old_effect_root REGEXP '^[0-9a-f]{64}$' AND replacement_effect_root REGEXP '^[0-9a-f]{64}$'
    AND descriptor_digest REGEXP '^[0-9a-f]{64}$'
    AND boundary_block_hash REGEXP '^0x[0-9a-f]{64}$' AND target_safe_block_hash REGEXP '^0x[0-9a-f]{64}$'
    AND legacy_source_registered IN (0,1)
    AND ((legacy_source_registered=1
      AND legacy_contract_address REGEXP '^0x[0-9a-f]{40}$'
      AND legacy_contract_address<>CONCAT('0x',REPEAT('0',40))
      AND legacy_contract_address<>v2_contract_address)
      OR (legacy_source_registered=0 AND legacy_contract_address=''))
    AND v2_contract_address REGEXP '^0x[0-9a-f]{40}$' AND v2_contract_address<>CONCAT('0x',REPEAT('0',40))
  ) ENFORCED;

DROP TRIGGER IF EXISTS trg_chain_event_reorg_materialization_run_validate_insert;
DROP TRIGGER IF EXISTS trg_chain_event_reorg_generation_validate_insert;
DELIMITER $$
CREATE TRIGGER trg_chain_event_reorg_materialization_run_validate_insert BEFORE INSERT ON chain_event_reorg_materialization_run FOR EACH ROW
BEGIN
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization run id must be database assigned'; END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_incident i WHERE i.id=NEW.incident_id AND i.status='detected' AND i.active_slot=1
      AND i.chain_id=NEW.chain_id AND i.cursor_block=NEW.old_cursor_block AND i.expected_block_hash=NEW.old_cursor_block_hash) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization run requires matching active incident';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_recovery_run r
      JOIN chain_event_reorg_recovery_job j ON j.run_id=r.id AND j.state='completed'
      JOIN chain_event_reorg_recovery_manifest m ON m.run_id=r.id
      WHERE r.id=NEW.recovery_run_id AND r.incident_id=NEW.incident_id AND r.chain_id=NEW.chain_id
      AND r.contract_address=NEW.contract_address AND r.legacy_contract_address=NEW.legacy_contract_address
      AND r.legacy_source_registered=NEW.legacy_source_registered
      AND r.boundary_block=NEW.boundary_block AND r.boundary_block_hash=NEW.boundary_block_hash
      AND r.target_safe_block=NEW.target_safe_block AND r.target_safe_block_hash=NEW.target_safe_block_hash
      AND m.manifest_digest=NEW.recovery_manifest_digest AND m.log_count=NEW.expected_replacement_log_count
      AND m.log_evidence_digest=NEW.replacement_log_source_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization run recovery manifest identity mismatch';
  END IF;
  IF NEW.expected_old_record_count<>(SELECT COUNT(*) FROM chain_event_record e WHERE e.block_number>NEW.boundary_block
      AND e.block_number<=NEW.old_cursor_block AND ((NEW.legacy_source_registered=1 AND e.contract_version='v1_legacy'
        AND LOWER(e.contract_address)=NEW.legacy_contract_address) OR (e.contract_version='v2_stablecoin'
        AND LOWER(e.contract_address)=NEW.contract_address))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization old source count mismatch';
  END IF;
  IF (SELECT COALESCE(SUM(OCTET_LENGTH(e.data)),0) FROM chain_event_record e WHERE e.block_number>NEW.boundary_block
      AND e.block_number<=NEW.old_cursor_block AND ((NEW.legacy_source_registered=1 AND e.contract_version='v1_legacy'
        AND LOWER(e.contract_address)=NEW.legacy_contract_address) OR (e.contract_version='v2_stablecoin'
        AND LOWER(e.contract_address)=NEW.contract_address)))>12582912
     OR (SELECT COALESCE(SUM(OCTET_LENGTH(l.raw_log)),0) FROM chain_event_reorg_recovery_log l
        WHERE l.run_id=NEW.recovery_run_id)>12582912 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='materialization source payload limit exceeded';
  END IF;
END$$

CREATE TRIGGER trg_chain_event_reorg_generation_validate_insert BEFORE INSERT ON chain_event_reorg_generation FOR EACH ROW
BEGIN
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation id must be database assigned'; END IF;
  IF NEW.attempt_no<>COALESCE((SELECT MAX(g.attempt_no)+1 FROM chain_event_reorg_generation g WHERE g.incident_id=NEW.incident_id),1) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation attempt sequence is not contiguous';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_incident i
      JOIN chain_event_reorg_prerequisite_run p ON p.incident_id=i.id
      JOIN chain_event_reorg_prerequisite_job pj ON pj.run_id=p.id AND pj.state='completed'
      JOIN chain_event_reorg_prerequisite_manifest pm ON pm.run_id=p.id AND pm.incident_id=i.id
      JOIN chain_event_reorg_assessment_run a ON a.id=p.assessment_run_id AND a.incident_id=i.id
      JOIN chain_event_reorg_assessment_manifest am ON am.run_id=a.id AND am.incident_id=i.id
      JOIN chain_event_reorg_materialization_run m ON m.id=a.materialization_run_id AND m.incident_id=i.id
      JOIN chain_event_reorg_materialization_manifest mm ON mm.run_id=m.id AND mm.incident_id=i.id
      WHERE i.id=NEW.incident_id AND i.active_slot=1 AND i.status='detected'
      AND p.id=NEW.prerequisite_run_id AND pm.manifest_digest=NEW.prerequisite_manifest_digest
      AND a.id=NEW.assessment_run_id AND am.manifest_digest=NEW.assessment_manifest_digest
      AND m.id=NEW.materialization_run_id AND mm.manifest_digest=NEW.materialization_manifest_digest
      AND pm.outcome='BLOCKED' AND pm.candidate_ready=0 AND NEW.chain_id=a.chain_id
      AND NEW.boundary_block=a.boundary_block AND NEW.boundary_block_hash=a.boundary_block_hash
      AND NEW.target_safe_block=a.target_safe_block AND NEW.target_safe_block_hash=a.target_safe_block_hash
      AND NEW.legacy_source_registered=m.legacy_source_registered
      AND NEW.legacy_contract_address=m.legacy_contract_address AND NEW.v2_contract_address=m.contract_address
      AND NEW.old_effect_root=mm.old_effect_digest AND NEW.replacement_effect_root=mm.replacement_effect_digest) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation descriptor source identity mismatch';
  END IF;
  IF NEW.descriptor_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-generation-descriptor.v1',CHAR(10),
      NEW.generation_uuid,CHAR(10),NEW.incident_id,CHAR(10),NEW.attempt_no,CHAR(10),NEW.generation_kind,CHAR(10),
      NEW.prerequisite_run_id,CHAR(10),NEW.prerequisite_manifest_digest,CHAR(10),NEW.assessment_run_id,CHAR(10),
      NEW.assessment_manifest_digest,CHAR(10),NEW.materialization_run_id,CHAR(10),NEW.materialization_manifest_digest,CHAR(10),
      NEW.chain_id,CHAR(10),NEW.boundary_block,CHAR(10),NEW.boundary_block_hash,CHAR(10),NEW.target_safe_block,CHAR(10),
      NEW.target_safe_block_hash,CHAR(10),IF(NEW.legacy_source_registered=1,'true','false'),CHAR(10),NEW.legacy_contract_address,CHAR(10),NEW.v2_contract_address,CHAR(10),
      NEW.old_effect_root,CHAR(10),NEW.replacement_effect_root,CHAR(10),NEW.algorithm_version,CHAR(10),NEW.schema_version,CHAR(10),
      DATE_FORMAT(NEW.created_at,'%Y-%m-%dT%H:%i:%s.%fZ')),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='generation descriptor digest mismatch';
  END IF;
END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS jc_assert_chain_event_v2_only_source_policy;
DELIMITER $$
CREATE PROCEDURE jc_assert_chain_event_v2_only_source_policy()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()
      AND TABLE_NAME IN ('chain_event_reorg_recovery_run','chain_event_reorg_materialization_run','chain_event_reorg_generation')
      AND COLUMN_NAME='legacy_source_registered' AND COLUMN_TYPE='tinyint unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='1')<>3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V2-only source policy columns are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS tc
      JOIN information_schema.CHECK_CONSTRAINTS cc ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
      WHERE tc.CONSTRAINT_SCHEMA=DATABASE() AND tc.CONSTRAINT_TYPE='CHECK' AND tc.ENFORCED='YES' AND (
      (tc.CONSTRAINT_NAME='chk_chain_event_reorg_recovery_run_identity' AND REPLACE(cc.CHECK_CLAUSE,' ','') LIKE '%legacy_source_registered%in(0,1)%' AND cc.CHECK_CLAUSE LIKE '%legacy_contract_address%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_materialization_run_identity' AND REPLACE(cc.CHECK_CLAUSE,' ','') LIKE '%legacy_source_registered%in(0,1)%' AND cc.CHECK_CLAUSE LIKE '%legacy_contract_address%')
      OR (tc.CONSTRAINT_NAME='chk_chain_event_reorg_generation_sources' AND REPLACE(cc.CHECK_CLAUSE,' ','') LIKE '%legacy_source_registered%in(0,1)%' AND cc.CHECK_CLAUSE LIKE '%legacy_contract_address%')))<>3 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V2-only source policy checks are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND (
      (TRIGGER_NAME='trg_chain_event_reorg_materialization_run_validate_insert' AND ACTION_STATEMENT LIKE '%legacy_source_registered%')
      OR (TRIGGER_NAME='trg_chain_event_reorg_generation_validate_insert' AND ACTION_STATEMENT LIKE '%legacy_source_registered%' AND ACTION_STATEMENT LIKE '%descriptor digest mismatch%')))<>2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V2-only source policy triggers are incomplete';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_chain_event_v2_only_source_policy();
DROP PROCEDURE jc_assert_chain_event_v2_only_source_policy;

-- Deployment runner records checksum/status only after every postflight succeeds.

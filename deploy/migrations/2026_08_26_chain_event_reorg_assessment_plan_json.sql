-- MySQL JSON functions reject MEDIUMBLOB input as binary text. Keep the
-- immutable raw plan bytes and SHA-256 binding, but validate their explicit
-- utf8mb4 representation in the assessment disposition trigger.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_reorg_assessment_plan_json;
DELIMITER $$
CREATE PROCEDURE jc_preflight_reorg_assessment_plan_json()
BEGIN
  IF (SELECT COUNT(*) FROM schema_migrations WHERE status='applied' AND (
      (filename='2026_08_25_chain_event_reorg_reversibility_assessment.sql'
        AND checksum='31764df81b1f74ef8a00a8bd785b81f9835dd9132a3e02361c217c8ca98ab68e')
      OR (filename='2026_08_25_zz_chain_event_reorg_assessment_fk_restrict.sql'
        AND checksum='ae2387d475ca19edf3292592e051b7d00bc4ed2e167b3be7d788bb431c799575')
    ))<>2 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment plan JSON prerequisite migrations are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS
      WHERE TRIGGER_SCHEMA=DATABASE()
        AND TRIGGER_NAME='trg_chain_event_reorg_assessment_disposition_validate_insert'
        AND EVENT_OBJECT_TABLE='chain_event_reorg_assessment_disposition'
        AND ACTION_TIMING='BEFORE' AND EVENT_MANIPULATION='INSERT'
        AND ACTION_ORIENTATION='ROW'
        AND LOWER(SHA2(ACTION_STATEMENT,256))='d2c83257179e123d87e8a98bf4a583b2811d9ceb9bafd5de9e46110e908f2e8d'
        AND ACTION_STATEMENT LIKE '%JSON_VALID(NEW.plan_payload)%'
        AND ACTION_STATEMENT NOT LIKE '%CONVERT(NEW.plan_payload USING utf8mb4)%'
        AND ACTION_STATEMENT LIKE '%SHA2(NEW.plan_payload,256)%')<>1 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment plan JSON trigger does not match approved binary-input baseline';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_reorg_assessment_plan_json();
DROP PROCEDURE jc_preflight_reorg_assessment_plan_json;

DROP TRIGGER IF EXISTS trg_chain_event_reorg_assessment_disposition_migration_guard;
DELIMITER $$
CREATE TRIGGER trg_chain_event_reorg_assessment_disposition_migration_guard
BEFORE INSERT ON chain_event_reorg_assessment_disposition FOR EACH ROW
PRECEDES trg_chain_event_reorg_assessment_disposition_validate_insert
BEGIN
  SIGNAL SQLSTATE '45000'
    SET MESSAGE_TEXT='assessment disposition writes are blocked during schema migration';
END$$
DELIMITER ;

DROP TRIGGER trg_chain_event_reorg_assessment_disposition_validate_insert;
DELIMITER $$
CREATE TRIGGER trg_chain_event_reorg_assessment_disposition_validate_insert
BEFORE INSERT ON chain_event_reorg_assessment_disposition FOR EACH ROW
BEGIN
  DECLARE expected_previous VARCHAR(64);
  DECLARE expected_ordinal BIGINT UNSIGNED;
  DECLARE plan_json LONGTEXT CHARACTER SET utf8mb4;
  IF NEW.id<>0 THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition id must be database assigned'; END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_job j WHERE j.run_id=NEW.run_id
      AND j.state='preparing' AND j.active_slot=1
      AND j.lease_token REGEXP '^[0-9a-f]{64}$' AND j.lease_expires_at>CURRENT_TIMESTAMP(6)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition requires active leased checkpoint';
  END IF;
  IF EXISTS (SELECT 1 FROM chain_event_reorg_assessment_event e WHERE e.run_id=NEW.run_id
      AND e.event_type IN ('COMPLETED','INVALIDATED')) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment terminal event forbids dispositions';
  END IF;
  SET expected_ordinal=COALESCE((SELECT MAX(ordinal)+1 FROM chain_event_reorg_assessment_disposition WHERE run_id=NEW.run_id),1);
  SET expected_previous=COALESCE((SELECT row_digest FROM chain_event_reorg_assessment_disposition
      WHERE run_id=NEW.run_id ORDER BY ordinal DESC LIMIT 1),(SELECT disposition_digest FROM chain_event_reorg_assessment_job WHERE run_id=NEW.run_id));
  IF NEW.ordinal<>expected_ordinal OR NEW.previous_row_digest<>expected_previous THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition chain is not contiguous';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM chain_event_reorg_assessment_run r
      JOIN chain_event_reorg_materialization_effect e ON e.run_id=r.materialization_run_id
      WHERE r.id=NEW.run_id AND e.id=NEW.materialization_effect_id AND e.ordinal=NEW.ordinal
      AND e.effect_digest=NEW.source_effect_digest AND e.branch=NEW.branch
      AND e.contract_version=NEW.contract_version AND e.event_type=NEW.event_type AND e.project_id=NEW.project_id
      AND ((e.classification='ignored_abi_log' AND NEW.replayability='noop' AND NEW.action_kind='apply_approved_abi_noop'
          AND NEW.branch='replacement' AND NEW.event_type='ignored' AND NEW.project_id=0)
        OR (e.classification='semantic_event' AND NEW.replayability<>'noop' AND NEW.event_type<>'ignored'))
      AND ((NEW.branch='old_committed' AND LEFT(NEW.action_kind,8)='reverse_')
        OR (NEW.branch='replacement' AND LEFT(NEW.action_kind,6)='apply_'))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition source effect mismatch';
  END IF;
  SET plan_json=CONVERT(NEW.plan_payload USING utf8mb4);
  IF plan_json IS NULL
     OR CONVERT(plan_json USING binary) IS NULL
     OR CONVERT(plan_json USING binary)<>NEW.plan_payload
     OR JSON_VALID(plan_json) IS NULL OR JSON_VALID(plan_json)<>1
     OR JSON_CONTAINS_PATH(plan_json,'all','$.schema_version','$.source_effect_digest','$.branch',
       '$.contract_version','$.event_type','$.project_id','$.replayability','$.action_kind')<>1
     OR JSON_TYPE(JSON_EXTRACT(plan_json,'$.schema_version'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(plan_json,'$.source_effect_digest'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(plan_json,'$.branch'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(plan_json,'$.contract_version'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(plan_json,'$.event_type'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(plan_json,'$.project_id'))<>'INTEGER'
     OR JSON_TYPE(JSON_EXTRACT(plan_json,'$.replayability'))<>'STRING'
     OR JSON_TYPE(JSON_EXTRACT(plan_json,'$.action_kind'))<>'STRING'
     OR JSON_UNQUOTE(JSON_EXTRACT(plan_json,'$.schema_version'))<>'reorg-assessment-plan-v1'
     OR JSON_UNQUOTE(JSON_EXTRACT(plan_json,'$.source_effect_digest'))<>NEW.source_effect_digest
     OR JSON_UNQUOTE(JSON_EXTRACT(plan_json,'$.branch'))<>NEW.branch
     OR JSON_UNQUOTE(JSON_EXTRACT(plan_json,'$.contract_version'))<>NEW.contract_version
     OR JSON_UNQUOTE(JSON_EXTRACT(plan_json,'$.event_type'))<>NEW.event_type
     OR CAST(JSON_EXTRACT(plan_json,'$.project_id') AS UNSIGNED)<>NEW.project_id
     OR JSON_UNQUOTE(JSON_EXTRACT(plan_json,'$.replayability'))<>NEW.replayability
     OR JSON_UNQUOTE(JSON_EXTRACT(plan_json,'$.action_kind'))<>NEW.action_kind
     OR ((NEW.blocker_code='' AND (JSON_LENGTH(plan_json)<>8 OR JSON_CONTAINS_PATH(plan_json,'one','$.blocker_code')=1))
       OR (NEW.blocker_code<>'' AND (JSON_LENGTH(plan_json)<>9
         OR JSON_CONTAINS_PATH(plan_json,'all','$.blocker_code')<>1
         OR JSON_TYPE(JSON_EXTRACT(plan_json,'$.blocker_code'))<>'STRING'
         OR JSON_UNQUOTE(JSON_EXTRACT(plan_json,'$.blocker_code'))<>NEW.blocker_code))) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition plan payload identity mismatch';
  END IF;
  IF NEW.plan_payload_digest<>LOWER(SHA2(NEW.plan_payload,256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition payload digest mismatch';
  END IF;
  IF NEW.row_digest<>LOWER(SHA2(CONCAT('joincare.chain-event.reorg-assessment-disposition.v1',CHAR(10),
      NEW.run_id,CHAR(10),NEW.ordinal,CHAR(10),NEW.materialization_effect_id,CHAR(10),NEW.source_effect_digest,CHAR(10),
      NEW.branch,CHAR(10),NEW.contract_version,CHAR(10),NEW.event_type,CHAR(10),NEW.project_id,CHAR(10),
      NEW.replayability,CHAR(10),NEW.action_kind,CHAR(10),NEW.blocker_code,CHAR(10),NEW.plan_payload_digest,CHAR(10),
      NEW.previous_row_digest),256)) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='assessment disposition row digest mismatch';
  END IF;
END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS jc_assert_reorg_assessment_plan_json;
DELIMITER $$
CREATE PROCEDURE jc_assert_reorg_assessment_plan_json()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS
      WHERE TRIGGER_SCHEMA=DATABASE()
        AND TRIGGER_NAME='trg_chain_event_reorg_assessment_disposition_validate_insert'
        AND EVENT_OBJECT_TABLE='chain_event_reorg_assessment_disposition'
        AND ACTION_TIMING='BEFORE' AND EVENT_MANIPULATION='INSERT'
        AND ACTION_ORIENTATION='ROW'
        AND LOWER(SHA2(ACTION_STATEMENT,256))='18468a554548fda14e35185e1b49a9739b4a8957b62b96c1c952f906da413564'
        AND ACTION_STATEMENT LIKE '%DECLARE plan_json LONGTEXT CHARACTER SET utf8mb4%'
        AND ACTION_STATEMENT LIKE '%CONVERT(NEW.plan_payload USING utf8mb4)%'
        AND ACTION_STATEMENT LIKE '%CONVERT(plan_json USING binary)<>NEW.plan_payload%'
        AND ACTION_STATEMENT LIKE '%JSON_VALID(plan_json) IS NULL%'
        AND ACTION_STATEMENT LIKE '%JSON_VALID(plan_json)%'
        AND ACTION_STATEMENT LIKE '%JSON_CONTAINS_PATH(plan_json,''all''%'
        AND ACTION_STATEMENT LIKE '%JSON_LENGTH(plan_json)%'
        AND ACTION_STATEMENT LIKE '%SHA2(NEW.plan_payload,256)%'
        AND ACTION_STATEMENT LIKE '%assessment disposition row digest mismatch%')<>1 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment plan JSON trigger normalization is incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS
      WHERE TRIGGER_SCHEMA=DATABASE()
        AND TRIGGER_NAME='trg_chain_event_reorg_assessment_disposition_migration_guard'
        AND EVENT_OBJECT_TABLE='chain_event_reorg_assessment_disposition'
        AND ACTION_TIMING='BEFORE' AND EVENT_MANIPULATION='INSERT'
        AND ACTION_ORIENTATION='ROW'
        AND ACTION_STATEMENT LIKE '%assessment disposition writes are blocked during schema migration%')<>1 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment plan JSON migration guard is unavailable';
  END IF;
  IF NOT EXISTS (SELECT 1
      FROM information_schema.TRIGGERS guard_trigger
      JOIN information_schema.TRIGGERS validator_trigger
        ON validator_trigger.TRIGGER_SCHEMA=guard_trigger.TRIGGER_SCHEMA
       AND validator_trigger.EVENT_OBJECT_TABLE=guard_trigger.EVENT_OBJECT_TABLE
      WHERE guard_trigger.TRIGGER_SCHEMA=DATABASE()
        AND guard_trigger.TRIGGER_NAME='trg_chain_event_reorg_assessment_disposition_migration_guard'
        AND validator_trigger.TRIGGER_NAME='trg_chain_event_reorg_assessment_disposition_validate_insert'
        AND guard_trigger.ACTION_ORDER<validator_trigger.ACTION_ORDER) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment plan JSON migration guard order is unsafe';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS
      WHERE TRIGGER_SCHEMA=DATABASE()
        AND EVENT_OBJECT_TABLE IN ('chain_event_reorg_assessment_run','chain_event_reorg_assessment_job',
          'chain_event_reorg_assessment_disposition','chain_event_reorg_assessment_blocker',
          'chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest'))<>19 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment trigger set changed during plan JSON normalization';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_reorg_assessment_plan_json();
DROP PROCEDURE jc_assert_reorg_assessment_plan_json;

DROP TRIGGER trg_chain_event_reorg_assessment_disposition_migration_guard;

DROP PROCEDURE IF EXISTS jc_assert_reorg_assessment_plan_json_final;
DELIMITER $$
CREATE PROCEDURE jc_assert_reorg_assessment_plan_json_final()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS
      WHERE TRIGGER_SCHEMA=DATABASE()
        AND EVENT_OBJECT_TABLE IN ('chain_event_reorg_assessment_run','chain_event_reorg_assessment_job',
          'chain_event_reorg_assessment_disposition','chain_event_reorg_assessment_blocker',
          'chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest'))<>18
     OR EXISTS (SELECT 1 FROM information_schema.TRIGGERS
      WHERE TRIGGER_SCHEMA=DATABASE()
        AND TRIGGER_NAME='trg_chain_event_reorg_assessment_disposition_migration_guard') THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment plan JSON migration guard cleanup is incomplete';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_reorg_assessment_plan_json_final();
DROP PROCEDURE jc_assert_reorg_assessment_plan_json_final;

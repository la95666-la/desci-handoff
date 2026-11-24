-- Preserve the frozen 2026_08_25 assessment migration while making its
-- intended update/delete restrictions explicit in MySQL information_schema.
SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS jc_preflight_reorg_assessment_fk_restrict;
DELIMITER $$
CREATE PROCEDURE jc_preflight_reorg_assessment_fk_restrict()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_TYPE='FOREIGN KEY'
      AND TABLE_NAME IN ('chain_event_reorg_assessment_run','chain_event_reorg_assessment_job',
        'chain_event_reorg_assessment_disposition','chain_event_reorg_assessment_blocker',
        'chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest')
  )<>10 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment foreign key count does not match frozen migration baseline';
  END IF;

  IF (
    SELECT COUNT(*) FROM (
      SELECT rc.TABLE_NAME,rc.CONSTRAINT_NAME,rc.REFERENCED_TABLE_NAME,
        rc.UPDATE_RULE,rc.DELETE_RULE,
        GROUP_CONCAT(k.COLUMN_NAME ORDER BY k.ORDINAL_POSITION SEPARATOR ',') child_columns,
        GROUP_CONCAT(k.REFERENCED_COLUMN_NAME ORDER BY k.ORDINAL_POSITION SEPARATOR ',') parent_columns
      FROM information_schema.REFERENTIAL_CONSTRAINTS rc
      JOIN information_schema.KEY_COLUMN_USAGE k
        ON k.CONSTRAINT_SCHEMA=rc.CONSTRAINT_SCHEMA
        AND k.CONSTRAINT_NAME=rc.CONSTRAINT_NAME
        AND k.TABLE_NAME=rc.TABLE_NAME
      WHERE rc.CONSTRAINT_SCHEMA=DATABASE()
        AND rc.UNIQUE_CONSTRAINT_SCHEMA=DATABASE()
        AND k.REFERENCED_TABLE_SCHEMA=DATABASE()
      GROUP BY rc.TABLE_NAME,rc.CONSTRAINT_NAME,rc.REFERENCED_TABLE_NAME,
        rc.UPDATE_RULE,rc.DELETE_RULE
    ) f
    WHERE CONCAT(TABLE_NAME,'|',CONSTRAINT_NAME,'|',child_columns,'|',
      REFERENCED_TABLE_NAME,'|',parent_columns,'|',UPDATE_RULE,'|',DELETE_RULE) IN (
      'chain_event_reorg_assessment_run|fk_chain_event_reorg_assessment_run_incident|incident_id|chain_event_reorg_incident|id|NO ACTION|RESTRICT',
      'chain_event_reorg_assessment_run|fk_chain_event_reorg_assessment_run_materialization|materialization_run_id|chain_event_reorg_materialization_run|id|NO ACTION|RESTRICT',
      'chain_event_reorg_assessment_job|fk_chain_event_reorg_assessment_job_run|run_id|chain_event_reorg_assessment_run|id|NO ACTION|RESTRICT',
      'chain_event_reorg_assessment_job|fk_chain_event_reorg_assessment_job_incident|incident_id|chain_event_reorg_incident|id|NO ACTION|RESTRICT',
      'chain_event_reorg_assessment_disposition|fk_chain_event_reorg_assessment_disposition_run|run_id|chain_event_reorg_assessment_run|id|NO ACTION|RESTRICT',
      'chain_event_reorg_assessment_disposition|fk_chain_event_reorg_assessment_disposition_effect|materialization_effect_id|chain_event_reorg_materialization_effect|id|NO ACTION|RESTRICT',
      'chain_event_reorg_assessment_blocker|fk_chain_event_reorg_assessment_blocker_run|run_id|chain_event_reorg_assessment_run|id|NO ACTION|RESTRICT',
      'chain_event_reorg_assessment_blocker|fk_chain_event_reorg_assessment_blocker_disposition|run_id,disposition_id|chain_event_reorg_assessment_disposition|run_id,id|NO ACTION|RESTRICT',
      'chain_event_reorg_assessment_event|fk_chain_event_reorg_assessment_event_run|run_id|chain_event_reorg_assessment_run|id|NO ACTION|RESTRICT',
      'chain_event_reorg_assessment_manifest|fk_chain_event_reorg_assessment_manifest_run|run_id|chain_event_reorg_assessment_run|id|NO ACTION|RESTRICT'
    )
  )<>10 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment foreign keys do not match frozen migration baseline';
  END IF;
END$$
DELIMITER ;
CALL jc_preflight_reorg_assessment_fk_restrict();
DROP PROCEDURE jc_preflight_reorg_assessment_fk_restrict;

ALTER TABLE chain_event_reorg_assessment_run
  ADD CONSTRAINT fk_chain_event_reorg_assessment_run_incident_shadow FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  ADD CONSTRAINT fk_chain_event_reorg_assessment_run_materialization_shadow FOREIGN KEY (materialization_run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_run
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_run_incident,
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_run_materialization;
ALTER TABLE chain_event_reorg_assessment_run
  ADD CONSTRAINT fk_chain_event_reorg_assessment_run_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  ADD CONSTRAINT fk_chain_event_reorg_assessment_run_materialization FOREIGN KEY (materialization_run_id)
    REFERENCES chain_event_reorg_materialization_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_run
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_run_incident_shadow,
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_run_materialization_shadow;

ALTER TABLE chain_event_reorg_assessment_job
  ADD CONSTRAINT fk_chain_event_reorg_assessment_job_run_shadow FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  ADD CONSTRAINT fk_chain_event_reorg_assessment_job_incident_shadow FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_job
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_job_run,
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_job_incident;
ALTER TABLE chain_event_reorg_assessment_job
  ADD CONSTRAINT fk_chain_event_reorg_assessment_job_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  ADD CONSTRAINT fk_chain_event_reorg_assessment_job_incident FOREIGN KEY (incident_id)
    REFERENCES chain_event_reorg_incident(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_job
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_job_run_shadow,
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_job_incident_shadow;

ALTER TABLE chain_event_reorg_assessment_disposition
  ADD CONSTRAINT fk_chain_event_reorg_assessment_disposition_run_shadow FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  ADD CONSTRAINT fk_chain_event_reorg_assessment_disposition_effect_shadow FOREIGN KEY (materialization_effect_id)
    REFERENCES chain_event_reorg_materialization_effect(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_disposition
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_disposition_run,
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_disposition_effect;
ALTER TABLE chain_event_reorg_assessment_disposition
  ADD CONSTRAINT fk_chain_event_reorg_assessment_disposition_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  ADD CONSTRAINT fk_chain_event_reorg_assessment_disposition_effect FOREIGN KEY (materialization_effect_id)
    REFERENCES chain_event_reorg_materialization_effect(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_disposition
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_disposition_run_shadow,
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_disposition_effect_shadow;

ALTER TABLE chain_event_reorg_assessment_blocker
  ADD CONSTRAINT fk_chain_event_reorg_assessment_blocker_run_shadow FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  ADD CONSTRAINT fk_chain_event_reorg_assessment_blocker_disposition_shadow FOREIGN KEY (run_id,disposition_id)
    REFERENCES chain_event_reorg_assessment_disposition(run_id,id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_blocker
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_blocker_run,
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_blocker_disposition;
ALTER TABLE chain_event_reorg_assessment_blocker
  ADD CONSTRAINT fk_chain_event_reorg_assessment_blocker_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  ADD CONSTRAINT fk_chain_event_reorg_assessment_blocker_disposition FOREIGN KEY (run_id,disposition_id)
    REFERENCES chain_event_reorg_assessment_disposition(run_id,id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_blocker
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_blocker_run_shadow,
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_blocker_disposition_shadow;

ALTER TABLE chain_event_reorg_assessment_event
  ADD CONSTRAINT fk_chain_event_reorg_assessment_event_run_shadow FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_event
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_event_run;
ALTER TABLE chain_event_reorg_assessment_event
  ADD CONSTRAINT fk_chain_event_reorg_assessment_event_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_event
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_event_run_shadow;

ALTER TABLE chain_event_reorg_assessment_manifest
  ADD CONSTRAINT fk_chain_event_reorg_assessment_manifest_run_shadow FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_manifest
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_manifest_run;
ALTER TABLE chain_event_reorg_assessment_manifest
  ADD CONSTRAINT fk_chain_event_reorg_assessment_manifest_run FOREIGN KEY (run_id)
    REFERENCES chain_event_reorg_assessment_run(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
ALTER TABLE chain_event_reorg_assessment_manifest
  DROP FOREIGN KEY fk_chain_event_reorg_assessment_manifest_run_shadow;

DROP PROCEDURE IF EXISTS jc_assert_reorg_assessment_fk_restrict;
DELIMITER $$
CREATE PROCEDURE jc_assert_reorg_assessment_fk_restrict()
BEGIN
  IF (
    SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
    WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_TYPE='FOREIGN KEY'
      AND TABLE_NAME IN ('chain_event_reorg_assessment_run','chain_event_reorg_assessment_job',
        'chain_event_reorg_assessment_disposition','chain_event_reorg_assessment_blocker',
        'chain_event_reorg_assessment_event','chain_event_reorg_assessment_manifest')
  )<>10 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment foreign key count changed during RESTRICT normalization';
  END IF;

  IF (
    SELECT COUNT(*) FROM (
      SELECT rc.TABLE_NAME,rc.CONSTRAINT_NAME,rc.REFERENCED_TABLE_NAME,
        rc.UPDATE_RULE,rc.DELETE_RULE,
        GROUP_CONCAT(k.COLUMN_NAME ORDER BY k.ORDINAL_POSITION SEPARATOR ',') child_columns,
        GROUP_CONCAT(k.REFERENCED_COLUMN_NAME ORDER BY k.ORDINAL_POSITION SEPARATOR ',') parent_columns
      FROM information_schema.REFERENTIAL_CONSTRAINTS rc
      JOIN information_schema.KEY_COLUMN_USAGE k
        ON k.CONSTRAINT_SCHEMA=rc.CONSTRAINT_SCHEMA
        AND k.CONSTRAINT_NAME=rc.CONSTRAINT_NAME
        AND k.TABLE_NAME=rc.TABLE_NAME
      WHERE rc.CONSTRAINT_SCHEMA=DATABASE()
        AND rc.UNIQUE_CONSTRAINT_SCHEMA=DATABASE()
        AND k.REFERENCED_TABLE_SCHEMA=DATABASE()
      GROUP BY rc.TABLE_NAME,rc.CONSTRAINT_NAME,rc.REFERENCED_TABLE_NAME,
        rc.UPDATE_RULE,rc.DELETE_RULE
    ) f
    WHERE CONCAT(TABLE_NAME,'|',CONSTRAINT_NAME,'|',child_columns,'|',
      REFERENCED_TABLE_NAME,'|',parent_columns,'|',UPDATE_RULE,'|',DELETE_RULE) IN (
      'chain_event_reorg_assessment_run|fk_chain_event_reorg_assessment_run_incident|incident_id|chain_event_reorg_incident|id|RESTRICT|RESTRICT',
      'chain_event_reorg_assessment_run|fk_chain_event_reorg_assessment_run_materialization|materialization_run_id|chain_event_reorg_materialization_run|id|RESTRICT|RESTRICT',
      'chain_event_reorg_assessment_job|fk_chain_event_reorg_assessment_job_run|run_id|chain_event_reorg_assessment_run|id|RESTRICT|RESTRICT',
      'chain_event_reorg_assessment_job|fk_chain_event_reorg_assessment_job_incident|incident_id|chain_event_reorg_incident|id|RESTRICT|RESTRICT',
      'chain_event_reorg_assessment_disposition|fk_chain_event_reorg_assessment_disposition_run|run_id|chain_event_reorg_assessment_run|id|RESTRICT|RESTRICT',
      'chain_event_reorg_assessment_disposition|fk_chain_event_reorg_assessment_disposition_effect|materialization_effect_id|chain_event_reorg_materialization_effect|id|RESTRICT|RESTRICT',
      'chain_event_reorg_assessment_blocker|fk_chain_event_reorg_assessment_blocker_run|run_id|chain_event_reorg_assessment_run|id|RESTRICT|RESTRICT',
      'chain_event_reorg_assessment_blocker|fk_chain_event_reorg_assessment_blocker_disposition|run_id,disposition_id|chain_event_reorg_assessment_disposition|run_id,id|RESTRICT|RESTRICT',
      'chain_event_reorg_assessment_event|fk_chain_event_reorg_assessment_event_run|run_id|chain_event_reorg_assessment_run|id|RESTRICT|RESTRICT',
      'chain_event_reorg_assessment_manifest|fk_chain_event_reorg_assessment_manifest_run|run_id|chain_event_reorg_assessment_run|id|RESTRICT|RESTRICT'
    )
  )<>10 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT='assessment foreign keys were not normalized to RESTRICT';
  END IF;
END$$
DELIMITER ;
CALL jc_assert_reorg_assessment_fk_restrict();
DROP PROCEDURE jc_assert_reorg_assessment_fk_restrict;

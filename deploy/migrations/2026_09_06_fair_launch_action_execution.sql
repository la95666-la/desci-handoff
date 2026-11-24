-- JoinCare Fair Launch explicit three-party Admin action execution stage.
-- ADDITIVE ONLY. Runtime remains disabled unless independently admitted.
SET NAMES utf8mb4;
DROP PROCEDURE IF EXISTS jc_fair_launch_action_execution_0906;
DELIMITER $$
CREATE PROCEDURE jc_fair_launch_action_execution_0906()
BEGIN
  IF COALESCE(@JC_FAIR_LAUNCH_MIGRATION_APPROVED,0)<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Fair Launch action execution migration requires approved execution gate';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_feature_watermark') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='09_05 Fair Launch control migration missing';
  END IF;
END$$
DELIMITER ;
CALL jc_fair_launch_action_execution_0906();
DROP PROCEDURE jc_fair_launch_action_execution_0906;

CREATE TABLE IF NOT EXISTS fair_launch_preview (
  id BIGINT UNSIGNED NOT NULL,
  token_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  preview_kind VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id BIGINT UNSIGNED NOT NULL,
  action VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  operation VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  admin_id BIGINT UNSIGNED NOT NULL,
  canonical_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  base_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  canonical_payload LONGTEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  issued_at DATETIME(6) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  consumed_at DATETIME(6) NULL,
  consumed_by_admin_id BIGINT UNSIGNED NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_preview_token(token_digest),
  KEY idx_fl_preview_digest(canonical_digest),
  CONSTRAINT fk_fl_preview_admin FOREIGN KEY(admin_id) REFERENCES admin_users(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_preview_consumer FOREIGN KEY(consumed_by_admin_id) REFERENCES admin_users(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_preview CHECK(preview_kind IN ('set','action') AND project_id>0 AND admin_id>0 AND token_digest REGEXP '^[0-9a-f]{64}$' AND canonical_digest REGEXP '^[0-9a-f]{64}$' AND base_digest REGEXP '^[0-9a-f]{64}$' AND JSON_VALID(canonical_payload) AND issued_at<expires_at AND (consumed_at IS NULL OR (consumed_by_admin_id IS NOT NULL AND consumed_at>=issued_at))) ENFORCED
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS fair_launch_mfa_provider (
  provider_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  admission_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  manifest_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  adapter_identity VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  generation BIGINT UNSIGNED NOT NULL,
  enabled TINYINT(1) NOT NULL,
  revoked_at DATETIME(6) NULL,
  valid_from DATETIME(6) NOT NULL,
  valid_until DATETIME(6) NULL,
  PRIMARY KEY(provider_id),
  UNIQUE KEY uk_fl_mfa_provider_admission(provider_id,generation,admission_digest),
  CONSTRAINT chk_fl_mfa_provider CHECK(provider_id<>'' AND provider_id<>'test' AND generation>0 AND adapter_identity<>'' AND admission_digest REGEXP '^[0-9a-f]{64}$' AND manifest_digest REGEXP '^[0-9a-f]{64}$' AND (valid_until IS NULL OR valid_until>valid_from)) ENFORCED
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS fair_launch_mfa_assertion (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  token_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  provider_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  provider_evidence_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  provider_admission_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  provider_generation BIGINT UNSIGNED NOT NULL,
  externally_verified TINYINT(1) NOT NULL,
  admin_id BIGINT UNSIGNED NOT NULL,
  operation VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  stage VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id BIGINT UNSIGNED NOT NULL,
  preview_id BIGINT UNSIGNED NULL,
  action_request_id BIGINT UNSIGNED NULL,
  admin_approval_id BIGINT UNSIGNED NULL,
  issued_at DATETIME(6) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  consumed_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_mfa_token(token_digest),
  CONSTRAINT fk_fl_mfa_admin FOREIGN KEY(admin_id) REFERENCES admin_users(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mfa_provider FOREIGN KEY(provider_id,provider_generation,provider_admission_digest) REFERENCES fair_launch_mfa_provider(provider_id,generation,admission_digest) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mfa_preview FOREIGN KEY(preview_id) REFERENCES fair_launch_preview(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_mfa_admin_approval FOREIGN KEY(admin_approval_id) REFERENCES admin_approval(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_mfa_assertion CHECK(externally_verified=1 AND admin_id>0 AND project_id>0 AND provider_id<>'' AND provider_id<>'test' AND provider_generation>0 AND provider_evidence_digest REGEXP '^[0-9a-f]{64}$' AND provider_admission_digest REGEXP '^[0-9a-f]{64}$' AND token_digest REGEXP '^[0-9a-f]{64}$' AND stage IN ('set_initiation','set_approval','action_initiation','action_approval','action_execution') AND issued_at<expires_at AND expires_at<=DATE_ADD(issued_at,INTERVAL 15 MINUTE) AND (consumed_at IS NULL OR consumed_at>=issued_at)) ENFORCED
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS fair_launch_action_request (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  project_id BIGINT UNSIGNED NOT NULL,
  canonical_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  canonical_payload LONGTEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  action VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  execution_domain VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  initiator_admin_id BIGINT UNSIGNED NOT NULL,
  approver_admin_id BIGINT UNSIGNED NULL,
  executor_admin_id BIGINT UNSIGNED NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  preview_expires_at DATETIME(6) NOT NULL,
  approval_expires_at DATETIME(6) NOT NULL,
  execution_expires_at DATETIME(6) NOT NULL,
  initiator_mfa_evidence_ref VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  initiator_mfa_verified_at DATETIME(6) NOT NULL,
  approver_mfa_evidence_ref VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL,
  approver_mfa_verified_at DATETIME(6) NULL,
  executor_mfa_evidence_ref VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL,
  executor_mfa_verified_at DATETIME(6) NULL,
  runtime_generation BIGINT UNSIGNED NOT NULL,
  policy_generation BIGINT UNSIGNED NOT NULL,
  source_registry_generation BIGINT UNSIGNED NOT NULL,
  config_generation BIGINT UNSIGNED NOT NULL,
  submission_id BIGINT UNSIGNED NULL,
  timelock_operation_row_id BIGINT UNSIGNED NULL,
  safe_tx_id BIGINT UNSIGNED NULL,
  external_observation_ref VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL,
  admin_approval_id BIGINT UNSIGNED NULL,
  preview_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_action_digest(canonical_digest),
  CONSTRAINT fk_fl_action_submission FOREIGN KEY(submission_id) REFERENCES chain_tx_submission(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_action_admin_approval FOREIGN KEY(admin_approval_id) REFERENCES admin_approval(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_action_timelock FOREIGN KEY(timelock_operation_row_id) REFERENCES governance_timelock_operation(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_action_safe FOREIGN KEY(safe_tx_id) REFERENCES governance_safe_tx(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_action_initiator_admin FOREIGN KEY(initiator_admin_id) REFERENCES admin_users(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_action_approver_admin FOREIGN KEY(approver_admin_id) REFERENCES admin_users(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_action_executor_admin FOREIGN KEY(executor_admin_id) REFERENCES admin_users(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_action_preview FOREIGN KEY(preview_id) REFERENCES fair_launch_preview(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_action_request CHECK (
    canonical_digest REGEXP '^[0-9a-f]{64}$' AND
    status IN ('pending_approval','approval_bound','approved','execution_bound','executing','dispatched','observing','applied','failed','expired','cancelled') AND
    execution_domain IN ('operator_durable_outbox','external_safe_observation','pause_guardian_safe','unpause_timelock','governance_timelock') AND
    initiator_admin_id>0 AND (approver_admin_id IS NULL OR approver_admin_id>0) AND (executor_admin_id IS NULL OR executor_admin_id>0) AND
    runtime_generation>0 AND policy_generation>0 AND source_registry_generation>0 AND config_generation>0 AND
    preview_expires_at<=approval_expires_at AND approval_expires_at<=execution_expires_at AND
    (approver_admin_id IS NULL OR initiator_admin_id<>approver_admin_id) AND
    (executor_admin_id IS NULL OR (initiator_admin_id<>executor_admin_id AND (approver_admin_id IS NULL OR approver_admin_id<>executor_admin_id))) AND
    ((status='pending_approval' AND approver_admin_id IS NULL AND executor_admin_id IS NULL) OR
     (status IN ('approval_bound','approved') AND admin_approval_id IS NOT NULL AND approver_admin_id IS NOT NULL AND approver_mfa_evidence_ref IS NOT NULL AND approver_mfa_verified_at IS NOT NULL AND executor_admin_id IS NULL) OR
     (status IN ('execution_bound','executing','dispatched','observing','applied','failed') AND executor_admin_id IS NOT NULL AND approver_admin_id IS NOT NULL AND approver_mfa_evidence_ref IS NOT NULL AND approver_mfa_verified_at IS NOT NULL AND executor_mfa_evidence_ref IS NOT NULL AND executor_mfa_verified_at IS NOT NULL) OR
     (status IN ('expired','cancelled') AND executor_admin_id IS NULL))
  ) ENFORCED
) ENGINE=InnoDB;

DROP PROCEDURE IF EXISTS jc_fl_add_mfa_action_fk;
DELIMITER $$
CREATE PROCEDURE jc_fl_add_mfa_action_fk()
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='fair_launch_mfa_assertion' AND CONSTRAINT_NAME='fk_fl_mfa_action_request') THEN
    ALTER TABLE fair_launch_mfa_assertion ADD CONSTRAINT fk_fl_mfa_action_request FOREIGN KEY(action_request_id) REFERENCES fair_launch_action_request(id) ON UPDATE RESTRICT ON DELETE RESTRICT;
  END IF;
END$$
DELIMITER ;
CALL jc_fl_add_mfa_action_fk();
DROP PROCEDURE jc_fl_add_mfa_action_fk;

CREATE TABLE IF NOT EXISTS fair_launch_action_transition (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  action_request_id BIGINT UNSIGNED NOT NULL,
  sequence_no INT UNSIGNED NOT NULL,
  from_status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  to_status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  actor_admin_id BIGINT UNSIGNED NOT NULL,
  mfa_evidence_ref VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  evidence_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY(id),
  UNIQUE KEY uk_fl_action_transition(action_request_id,sequence_no),
  CONSTRAINT fk_fl_action_transition FOREIGN KEY(action_request_id) REFERENCES fair_launch_action_request(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT fk_fl_action_transition_actor FOREIGN KEY(actor_admin_id) REFERENCES admin_users(id) ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT chk_fl_action_transition CHECK(
    sequence_no>0 AND actor_admin_id>0 AND evidence_digest REGEXP '^[0-9a-f]{64}$' AND
    ((from_status='pending_approval' AND to_status IN ('approval_bound','expired','cancelled')) OR
     (from_status='approval_bound' AND to_status IN ('approved','expired','cancelled')) OR
     (from_status='approved' AND to_status IN ('execution_bound','expired','cancelled')) OR
     (from_status='execution_bound' AND to_status IN ('executing','expired','cancelled')) OR
     (from_status='executing' AND to_status IN ('dispatched','failed')) OR
     (from_status='dispatched' AND to_status IN ('observing','failed')) OR
     (from_status='observing' AND to_status IN ('applied','failed')))
  ) ENFORCED
) ENGINE=InnoDB;

DROP TRIGGER IF EXISTS trg_fl_preview_consume_guard;
DROP TRIGGER IF EXISTS trg_fl_preview_no_delete;
DROP TRIGGER IF EXISTS trg_fl_mfa_consume_guard;
DROP TRIGGER IF EXISTS trg_fl_mfa_no_delete;
DROP TRIGGER IF EXISTS trg_fl_action_request_status_guard;
DROP TRIGGER IF EXISTS trg_fl_action_request_no_delete;
DROP TRIGGER IF EXISTS trg_fl_action_transition_before_insert;
DROP TRIGGER IF EXISTS trg_fl_action_transition_no_update;
DROP TRIGGER IF EXISTS trg_fl_action_transition_no_delete;
DELIMITER $$
CREATE TRIGGER trg_fl_preview_consume_guard BEFORE UPDATE ON fair_launch_preview FOR EACH ROW
BEGIN
  IF NOT (NEW.id<=>OLD.id AND NEW.token_digest<=>OLD.token_digest AND NEW.preview_kind<=>OLD.preview_kind AND NEW.project_id<=>OLD.project_id AND NEW.action<=>OLD.action AND NEW.operation<=>OLD.operation AND NEW.admin_id<=>OLD.admin_id AND NEW.canonical_digest<=>OLD.canonical_digest AND NEW.base_digest<=>OLD.base_digest AND NEW.canonical_payload<=>OLD.canonical_payload AND NEW.issued_at<=>OLD.issued_at AND NEW.expires_at<=>OLD.expires_at AND NEW.created_at<=>OLD.created_at AND OLD.consumed_at IS NULL AND NEW.consumed_at IS NOT NULL AND NEW.consumed_by_admin_id<=>OLD.admin_id) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='preview is immutable and single-use'; END IF;
END$$
CREATE TRIGGER trg_fl_preview_no_delete BEFORE DELETE ON fair_launch_preview FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='preview audit is append-only'; END$$
CREATE TRIGGER trg_fl_mfa_consume_guard BEFORE UPDATE ON fair_launch_mfa_assertion FOR EACH ROW
BEGIN
  IF NOT (NEW.id<=>OLD.id AND NEW.token_digest<=>OLD.token_digest AND NEW.provider_id<=>OLD.provider_id AND NEW.provider_evidence_digest<=>OLD.provider_evidence_digest AND NEW.externally_verified<=>OLD.externally_verified AND NEW.admin_id<=>OLD.admin_id AND NEW.operation<=>OLD.operation AND NEW.stage<=>OLD.stage AND NEW.project_id<=>OLD.project_id AND NEW.preview_id<=>OLD.preview_id AND NEW.action_request_id<=>OLD.action_request_id AND NEW.admin_approval_id<=>OLD.admin_approval_id AND NEW.issued_at<=>OLD.issued_at AND NEW.expires_at<=>OLD.expires_at AND NEW.created_at<=>OLD.created_at AND OLD.consumed_at IS NULL AND NEW.consumed_at IS NOT NULL) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='MFA assertion is immutable and single-use'; END IF;
END$$
CREATE TRIGGER trg_fl_mfa_no_delete BEFORE DELETE ON fair_launch_mfa_assertion FOR EACH ROW BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='MFA assertion audit is append-only'; END$$
CREATE TRIGGER trg_fl_action_request_status_guard BEFORE UPDATE ON fair_launch_action_request FOR EACH ROW
BEGIN
  IF NOT (NEW.project_id<=>OLD.project_id AND NEW.canonical_digest<=>OLD.canonical_digest AND NEW.canonical_payload<=>OLD.canonical_payload AND NEW.action<=>OLD.action AND NEW.execution_domain<=>OLD.execution_domain AND NEW.initiator_admin_id<=>OLD.initiator_admin_id AND NEW.initiator_mfa_evidence_ref<=>OLD.initiator_mfa_evidence_ref AND NEW.initiator_mfa_verified_at<=>OLD.initiator_mfa_verified_at AND NEW.runtime_generation<=>OLD.runtime_generation AND NEW.policy_generation<=>OLD.policy_generation AND NEW.source_registry_generation<=>OLD.source_registry_generation AND NEW.config_generation<=>OLD.config_generation AND NEW.preview_expires_at<=>OLD.preview_expires_at AND NEW.approval_expires_at<=>OLD.approval_expires_at AND NEW.execution_expires_at<=>OLD.execution_expires_at AND NEW.created_at<=>OLD.created_at) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='action request canonical identity is immutable'; END IF;
  IF OLD.approver_admin_id IS NOT NULL AND NOT (NEW.approver_admin_id<=>OLD.approver_admin_id AND NEW.approver_mfa_evidence_ref<=>OLD.approver_mfa_evidence_ref AND NEW.approver_mfa_verified_at<=>OLD.approver_mfa_verified_at AND NEW.admin_approval_id<=>OLD.admin_approval_id) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='completed approval identity is immutable'; END IF;
  IF OLD.executor_admin_id IS NOT NULL AND NOT (NEW.executor_admin_id<=>OLD.executor_admin_id AND NEW.executor_mfa_evidence_ref<=>OLD.executor_mfa_evidence_ref AND NEW.executor_mfa_verified_at<=>OLD.executor_mfa_verified_at) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='completed executor identity is immutable'; END IF;
  IF (OLD.submission_id IS NOT NULL AND NOT (NEW.submission_id<=>OLD.submission_id)) OR (OLD.timelock_operation_row_id IS NOT NULL AND NOT (NEW.timelock_operation_row_id<=>OLD.timelock_operation_row_id)) OR (OLD.safe_tx_id IS NOT NULL AND NOT (NEW.safe_tx_id<=>OLD.safe_tx_id)) OR (OLD.external_observation_ref IS NOT NULL AND NOT (NEW.external_observation_ref<=>OLD.external_observation_ref)) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='execution linkage is append-only'; END IF;
  IF NEW.status=OLD.status AND NOT (
    (OLD.status='pending_approval' AND OLD.admin_approval_id IS NULL AND NEW.admin_approval_id IS NOT NULL AND NEW.approver_admin_id<=>OLD.approver_admin_id AND NEW.approver_mfa_evidence_ref<=>OLD.approver_mfa_evidence_ref AND NEW.approver_mfa_verified_at<=>OLD.approver_mfa_verified_at AND NEW.executor_admin_id<=>OLD.executor_admin_id AND NEW.executor_mfa_evidence_ref<=>OLD.executor_mfa_evidence_ref AND NEW.executor_mfa_verified_at<=>OLD.executor_mfa_verified_at AND NEW.submission_id<=>OLD.submission_id AND NEW.timelock_operation_row_id<=>OLD.timelock_operation_row_id AND NEW.safe_tx_id<=>OLD.safe_tx_id AND NEW.external_observation_ref<=>OLD.external_observation_ref) OR
    (OLD.status IN ('executing','dispatched','observing') AND NEW.approver_admin_id<=>OLD.approver_admin_id AND NEW.approver_mfa_evidence_ref<=>OLD.approver_mfa_evidence_ref AND NEW.approver_mfa_verified_at<=>OLD.approver_mfa_verified_at AND NEW.admin_approval_id<=>OLD.admin_approval_id AND NEW.executor_admin_id<=>OLD.executor_admin_id AND NEW.executor_mfa_evidence_ref<=>OLD.executor_mfa_evidence_ref AND NEW.executor_mfa_verified_at<=>OLD.executor_mfa_verified_at AND (
      (OLD.submission_id IS NULL AND NEW.submission_id IS NOT NULL AND NEW.timelock_operation_row_id<=>OLD.timelock_operation_row_id AND NEW.safe_tx_id<=>OLD.safe_tx_id AND NEW.external_observation_ref<=>OLD.external_observation_ref) OR
      (OLD.timelock_operation_row_id IS NULL AND NEW.timelock_operation_row_id IS NOT NULL AND NEW.submission_id<=>OLD.submission_id AND NEW.safe_tx_id<=>OLD.safe_tx_id AND NEW.external_observation_ref<=>OLD.external_observation_ref) OR
      (OLD.safe_tx_id IS NULL AND NEW.safe_tx_id IS NOT NULL AND NEW.submission_id<=>OLD.submission_id AND NEW.timelock_operation_row_id<=>OLD.timelock_operation_row_id AND NEW.external_observation_ref<=>OLD.external_observation_ref) OR
      (OLD.external_observation_ref IS NULL AND NEW.external_observation_ref IS NOT NULL AND NEW.submission_id<=>OLD.submission_id AND NEW.timelock_operation_row_id<=>OLD.timelock_operation_row_id AND NEW.safe_tx_id<=>OLD.safe_tx_id)))
  ) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='same-status action request mutation rejected'; END IF;
  IF OLD.status='pending_approval' AND NEW.status='approval_bound' AND NOT (OLD.approver_admin_id IS NULL AND NEW.approver_admin_id IS NOT NULL AND NEW.approver_mfa_evidence_ref IS NOT NULL AND NEW.approver_mfa_verified_at IS NOT NULL AND NEW.admin_approval_id IS NOT NULL AND NEW.executor_admin_id<=>OLD.executor_admin_id AND NEW.executor_mfa_evidence_ref<=>OLD.executor_mfa_evidence_ref AND NEW.executor_mfa_verified_at<=>OLD.executor_mfa_verified_at AND NEW.submission_id<=>OLD.submission_id AND NEW.timelock_operation_row_id<=>OLD.timelock_operation_row_id AND NEW.safe_tx_id<=>OLD.safe_tx_id AND NEW.external_observation_ref<=>OLD.external_observation_ref) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='approval binding mutation invalid'; END IF;
  IF OLD.status='approval_bound' AND NEW.status='approved' AND NOT (NEW.approver_admin_id<=>OLD.approver_admin_id AND NEW.approver_mfa_evidence_ref<=>OLD.approver_mfa_evidence_ref AND NEW.approver_mfa_verified_at<=>OLD.approver_mfa_verified_at AND NEW.admin_approval_id<=>OLD.admin_approval_id AND NEW.executor_admin_id<=>OLD.executor_admin_id AND NEW.submission_id<=>OLD.submission_id AND NEW.timelock_operation_row_id<=>OLD.timelock_operation_row_id AND NEW.safe_tx_id<=>OLD.safe_tx_id AND NEW.external_observation_ref<=>OLD.external_observation_ref) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='approval completion mutation invalid'; END IF;
  IF OLD.status='approved' AND NEW.status='execution_bound' AND NOT (OLD.executor_admin_id IS NULL AND NEW.executor_admin_id IS NOT NULL AND NEW.executor_mfa_evidence_ref IS NOT NULL AND NEW.executor_mfa_verified_at IS NOT NULL AND NEW.approver_admin_id<=>OLD.approver_admin_id AND NEW.approver_mfa_evidence_ref<=>OLD.approver_mfa_evidence_ref AND NEW.approver_mfa_verified_at<=>OLD.approver_mfa_verified_at AND NEW.admin_approval_id<=>OLD.admin_approval_id AND NEW.submission_id<=>OLD.submission_id AND NEW.timelock_operation_row_id<=>OLD.timelock_operation_row_id AND NEW.safe_tx_id<=>OLD.safe_tx_id AND NEW.external_observation_ref<=>OLD.external_observation_ref) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='execution binding mutation invalid'; END IF;
  IF OLD.status='execution_bound' AND NEW.status='executing' AND NOT (NEW.executor_admin_id<=>OLD.executor_admin_id AND NEW.executor_mfa_evidence_ref<=>OLD.executor_mfa_evidence_ref AND NEW.executor_mfa_verified_at<=>OLD.executor_mfa_verified_at AND NEW.approver_admin_id<=>OLD.approver_admin_id AND NEW.admin_approval_id<=>OLD.admin_approval_id AND NEW.submission_id<=>OLD.submission_id AND NEW.timelock_operation_row_id<=>OLD.timelock_operation_row_id AND NEW.safe_tx_id<=>OLD.safe_tx_id AND NEW.external_observation_ref<=>OLD.external_observation_ref) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='execution start mutation invalid'; END IF;
  IF NEW.status<>OLD.status AND NOT EXISTS (SELECT 1 FROM fair_launch_action_transition t WHERE t.action_request_id=OLD.id AND t.from_status=OLD.status AND t.to_status=NEW.status AND t.sequence_no=(SELECT COALESCE(MAX(t2.sequence_no),0) FROM fair_launch_action_transition t2 WHERE t2.action_request_id=OLD.id)) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='action request status change requires latest valid transition'; END IF;
END$$
CREATE TRIGGER trg_fl_action_request_no_delete BEFORE DELETE ON fair_launch_action_request FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_action_request is append-only'; END$$
CREATE TRIGGER trg_fl_action_transition_before_insert BEFORE INSERT ON fair_launch_action_transition FOR EACH ROW
BEGIN
  DECLARE v_status VARCHAR(24); DECLARE v_next INT UNSIGNED; DECLARE v_initiator BIGINT UNSIGNED; DECLARE v_approver BIGINT UNSIGNED; DECLARE v_executor BIGINT UNSIGNED; DECLARE v_approver_mfa VARCHAR(128); DECLARE v_executor_mfa VARCHAR(128); DECLARE v_approver_mfa_at DATETIME(6); DECLARE v_executor_mfa_at DATETIME(6);
  SELECT status,initiator_admin_id,approver_admin_id,executor_admin_id,approver_mfa_evidence_ref,executor_mfa_evidence_ref,approver_mfa_verified_at,executor_mfa_verified_at INTO v_status,v_initiator,v_approver,v_executor,v_approver_mfa,v_executor_mfa,v_approver_mfa_at,v_executor_mfa_at FROM fair_launch_action_request WHERE id=NEW.action_request_id FOR UPDATE;
  SELECT COALESCE(MAX(sequence_no),0)+1 INTO v_next FROM fair_launch_action_transition WHERE action_request_id=NEW.action_request_id;
  IF NOT (NEW.from_status<=>v_status) OR NOT (NEW.sequence_no<=>v_next) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='action transition lineage/sequence mismatch'; END IF;
  IF NEW.from_status='pending_approval' AND NEW.to_status='approval_bound' AND (NEW.actor_admin_id=v_initiator OR NEW.mfa_evidence_ref='') THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='approval binding requires distinct actor/MFA'; END IF;
  IF NEW.from_status='approval_bound' AND NEW.to_status='approved' AND (v_approver IS NULL OR v_approver_mfa IS NULL OR v_approver_mfa_at IS NULL OR NOT (NEW.actor_admin_id<=>v_approver) OR NOT (NEW.mfa_evidence_ref<=>v_approver_mfa)) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='approval transition requires prebound actor/MFA'; END IF;
  IF NEW.from_status='approved' AND NEW.to_status='execution_bound' AND (NEW.actor_admin_id=v_initiator OR NEW.actor_admin_id=v_approver OR NEW.mfa_evidence_ref='') THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='execution binding requires distinct actor/MFA'; END IF;
  IF NEW.from_status='execution_bound' AND NEW.to_status='executing' AND (v_executor IS NULL OR v_executor_mfa IS NULL OR v_executor_mfa_at IS NULL OR NOT (NEW.actor_admin_id<=>v_executor) OR NOT (NEW.mfa_evidence_ref<=>v_executor_mfa)) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='execution transition requires prebound actor/MFA'; END IF;
  IF NEW.to_status IN ('expired','cancelled') AND NOT (NEW.actor_admin_id<=>v_initiator) AND NOT (NEW.actor_admin_id<=>v_approver) AND NOT (NEW.actor_admin_id<=>v_executor) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='terminal transition actor is not a recorded stage identity'; END IF;
  IF NEW.from_status IN ('executing','dispatched','observing') AND (v_executor IS NULL OR v_executor_mfa IS NULL OR v_executor_mfa_at IS NULL OR NOT (NEW.actor_admin_id<=>v_executor) OR NOT (NEW.mfa_evidence_ref<=>v_executor_mfa)) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='execution lifecycle requires prebound executor/MFA'; END IF;
END$$
CREATE TRIGGER trg_fl_action_transition_no_update BEFORE UPDATE ON fair_launch_action_transition FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_action_transition is append-only'; END$$
CREATE TRIGGER trg_fl_action_transition_no_delete BEFORE DELETE ON fair_launch_action_transition FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fair_launch_action_transition is append-only'; END$$
DELIMITER ;

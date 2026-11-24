-- JoinCare first-batch security blockers: live session revocation, verified
-- secondary wallets, least-privilege bot RBAC, and canonical event failure fence.
-- Additive only. Existing JWTs intentionally become invalid after application rollout.

DELIMITER $$
DROP PROCEDURE IF EXISTS jc_security_0830$$
CREATE PROCEDURE jc_security_0830()
BEGIN
  IF (SELECT COUNT(*) FROM schema_migrations WHERE status='applied' AND filename='2026_08_29_chain_event_v2_only_source_policy.sql')<>1
     OR EXISTS (SELECT 1 FROM schema_migrations WHERE status='applying' AND filename<>'2026_08_30_security_control_plane_and_event_fence.sql') THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='08_30 security migration prerequisites are incomplete';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='admin_users' AND COLUMN_NAME='session_version') THEN
    ALTER TABLE admin_users ADD COLUMN session_version BIGINT UNSIGNED NOT NULL DEFAULT 1 AFTER is_enable;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='admin_users' AND COLUMN_NAME='password_changed_at') THEN
    ALTER TABLE admin_users ADD COLUMN password_changed_at DATETIME NULL AFTER password_hash;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='admin_users' AND COLUMN_NAME='locked_until') THEN
    ALTER TABLE admin_users ADD COLUMN locked_until DATETIME NULL AFTER password_changed_at;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='admin_users' AND COLUMN_NAME='failed_login_count') THEN
    ALTER TABLE admin_users ADD COLUMN failed_login_count INT NOT NULL DEFAULT 0 AFTER locked_until;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='admin_users' AND COLUMN_NAME='deleted_at') THEN
    ALTER TABLE admin_users ADD COLUMN deleted_at DATETIME NULL AFTER updated_at, ADD KEY idx_deleted_at(deleted_at);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='user' AND COLUMN_NAME='is_enable') THEN
    ALTER TABLE `user` ADD COLUMN is_enable TINYINT NOT NULL DEFAULT 1 AFTER last_login_ip;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='user' AND COLUMN_NAME='session_version') THEN
    ALTER TABLE `user` ADD COLUMN session_version BIGINT UNSIGNED NOT NULL DEFAULT 1 AFTER is_enable;
  END IF;
  CREATE TABLE IF NOT EXISTS user_wallet (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    uid INT NOT NULL,
    wallet_address VARCHAR(64) NOT NULL,
    chain VARCHAR(16) NOT NULL DEFAULT 'evm',
    is_primary TINYINT NOT NULL DEFAULT 0,
    bound_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY(id),
    UNIQUE KEY uk_uid_wallet_chain(uid,wallet_address,chain),
    KEY idx_wallet(wallet_address),
    KEY idx_uid_primary(uid,is_primary)
  ) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='verified user wallet bindings';
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet' AND COLUMN_NAME='verified_at') THEN
    ALTER TABLE user_wallet ADD COLUMN verified_at DATETIME(6) NULL AFTER is_primary;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet' AND COLUMN_NAME='verification_method') THEN
    ALTER TABLE user_wallet ADD COLUMN verification_method VARCHAR(32) NOT NULL DEFAULT '' AFTER verified_at;
  END IF;

  CREATE TABLE IF NOT EXISTS admin_user_roles (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    admin_id BIGINT UNSIGNED NOT NULL,
    role_code VARCHAR(32) NOT NULL,
    granted_by BIGINT UNSIGNED NULL,
    granted_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY(id),
    UNIQUE KEY uk_admin_role(admin_id,role_code),
    KEY idx_role(role_code)
  ) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='admin user role membership';
  INSERT IGNORE INTO admin_user_roles(admin_id,role_code,granted_at)
    SELECT id,role_code,created_at FROM admin_users WHERE role_code<>'';

  CREATE TABLE IF NOT EXISTS admin_approval (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    operation VARCHAR(64) NOT NULL,
    target_kind VARCHAR(32) NOT NULL DEFAULT '',
    target_id VARCHAR(64) NOT NULL DEFAULT '',
    payload MEDIUMTEXT NOT NULL,
    reason VARCHAR(500) NOT NULL DEFAULT '',
    initiator_id BIGINT UNSIGNED NOT NULL,
    initiator_name VARCHAR(64) NOT NULL DEFAULT '',
    approver_id BIGINT UNSIGNED NULL,
    approver_name VARCHAR(64) NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    reject_reason VARCHAR(500) NOT NULL DEFAULT '',
    result TEXT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    reviewed_at DATETIME NULL,
    executed_at DATETIME NULL,
    expires_at DATETIME NULL,
    PRIMARY KEY(id),
    KEY idx_status_created(status,created_at),
    KEY idx_initiator(initiator_id),
    KEY idx_target(target_kind,target_id)
  ) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='four-eye approval work orders';

  CREATE TABLE IF NOT EXISTS admin_audit_log (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    admin_id BIGINT UNSIGNED NOT NULL,
    username VARCHAR(64) NOT NULL DEFAULT '',
    role_codes VARCHAR(255) NOT NULL DEFAULT '',
    ip VARCHAR(45) NOT NULL DEFAULT '',
    user_agent VARCHAR(500) NOT NULL DEFAULT '',
    method VARCHAR(10) NOT NULL,
    path VARCHAR(255) NOT NULL,
    route VARCHAR(255) NOT NULL DEFAULT '',
    target_id VARCHAR(64) NOT NULL DEFAULT '',
    req_body MEDIUMTEXT NULL,
    resp_status INT NOT NULL DEFAULT 0,
    resp_hash CHAR(64) NOT NULL DEFAULT '',
    prev_hash CHAR(64) NOT NULL DEFAULT '',
    hash CHAR(64) NOT NULL,
    duration_ms INT NOT NULL DEFAULT 0,
    note VARCHAR(500) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY(id),
    KEY idx_admin_created(admin_id,created_at),
    KEY idx_target(target_id,created_at),
    KEY idx_created(created_at)
  ) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='append-only admin operation audit';

  UPDATE admin_approval
  SET status='expired', reviewed_at=COALESCE(reviewed_at,CURRENT_TIMESTAMP)
  WHERE status='pending';
  UPDATE admin_approval
  SET payload='{"redacted":"legacy admin credential payload removed by 08_30"}'
  WHERE operation='admin.create';

  -- Only an existing row equal to the registration wallet is trusted without a
  -- new signature. All other historical Phase-1 rows remain visibly unverified.
  UPDATE user_wallet w
  JOIN `user` u ON u.id=w.uid AND LOWER(u.wallet_address)=LOWER(w.wallet_address)
  SET w.verified_at=COALESCE(w.verified_at,w.bound_at),
      w.verification_method=IF(w.verification_method='', 'primary_registration', w.verification_method),
      w.is_primary=1
  WHERE w.chain='evm' AND w.wallet_address REGEXP '^0x[0-9A-Fa-f]{40}$';
  UPDATE user_wallet w
  LEFT JOIN `user` u ON u.id=w.uid AND LOWER(u.wallet_address)=LOWER(w.wallet_address)
  SET w.is_primary=0
  WHERE w.is_primary<>0 AND (w.verified_at IS NULL OR u.id IS NULL);

  IF EXISTS (
    SELECT 1 FROM user_wallet
    WHERE verified_at IS NOT NULL AND chain='evm'
    GROUP BY LOWER(wallet_address) HAVING COUNT(DISTINCT uid)>1 LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='verified EVM wallet belongs to multiple users; reconcile before migration';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet' AND COLUMN_NAME='verified_evm_identity') THEN
    ALTER TABLE user_wallet ADD COLUMN verified_evm_identity VARCHAR(42)
      CHARACTER SET ascii COLLATE ascii_bin
      GENERATED ALWAYS AS (CASE WHEN verified_at IS NOT NULL AND chain='evm' THEN LOWER(wallet_address) ELSE NULL END) STORED;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet' AND INDEX_NAME='uk_user_wallet_verified_evm') THEN
    ALTER TABLE user_wallet ADD UNIQUE KEY uk_user_wallet_verified_evm (verified_evm_identity);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet' AND CONSTRAINT_NAME='chk_user_wallet_verification') THEN
    ALTER TABLE user_wallet ADD CONSTRAINT chk_user_wallet_verification CHECK ((
      (verified_at IS NULL AND verification_method='') OR
      (verified_at IS NOT NULL AND verification_method IN ('primary_registration','personal_sign') AND chain='evm'
       AND wallet_address REGEXP '^0x[0-9A-Fa-f]{40}$'
       AND LOWER(wallet_address)<>'0x0000000000000000000000000000000000000000')
    ) AND (is_primary=0 OR verified_at IS NOT NULL)) ENFORCED;
  END IF;

  CREATE TABLE IF NOT EXISTS user_wallet_identity (
    wallet_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    uid INT NOT NULL,
    chain VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    claim_method VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    claimed_at DATETIME(6) NOT NULL,
    PRIMARY KEY(wallet_address),
    KEY idx_user_wallet_identity_uid(uid),
    CONSTRAINT chk_user_wallet_identity CHECK (
      wallet_address REGEXP '^0x[0-9a-f]{40}$' AND
      wallet_address<>'0x0000000000000000000000000000000000000000' AND
      uid>0 AND chain='evm' AND claim_method IN ('primary_registration','personal_sign')
    ) ENFORCED
  ) ENGINE=InnoDB COMMENT='non-recyclable verified wallet ownership registry';

  IF EXISTS (
    SELECT 1 FROM `user` u JOIN user_wallet_identity i ON i.wallet_address=LOWER(u.wallet_address)
    WHERE i.uid<>u.id LIMIT 1
  ) OR EXISTS (
    SELECT 1 FROM user_wallet w JOIN user_wallet_identity i ON i.wallet_address=LOWER(w.wallet_address)
    WHERE w.verified_at IS NOT NULL AND w.chain='evm' AND i.uid<>w.uid LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='existing wallet identity ownership conflicts with authoritative users';
  END IF;
  INSERT IGNORE INTO user_wallet_identity(wallet_address,uid,chain,claim_method,claimed_at)
    SELECT LOWER(wallet_address),id,'evm','primary_registration',created_at FROM `user`
    WHERE wallet_address REGEXP '^0x[0-9A-Fa-f]{40}$' AND LOWER(wallet_address)<>'0x0000000000000000000000000000000000000000';
  INSERT IGNORE INTO user_wallet_identity(wallet_address,uid,chain,claim_method,claimed_at)
    SELECT LOWER(wallet_address),uid,'evm',verification_method,verified_at FROM user_wallet
    WHERE verified_at IS NOT NULL AND chain='evm';
  IF EXISTS (
    SELECT 1 FROM `user` u LEFT JOIN user_wallet_identity i ON i.wallet_address=LOWER(u.wallet_address) AND i.uid=u.id
    WHERE u.wallet_address REGEXP '^0x[0-9A-Fa-f]{40}$' AND i.wallet_address IS NULL LIMIT 1
  ) OR EXISTS (
    SELECT 1 FROM user_wallet w LEFT JOIN user_wallet_identity i ON i.wallet_address=LOWER(w.wallet_address) AND i.uid=w.uid
    WHERE w.verified_at IS NOT NULL AND w.chain='evm' AND i.wallet_address IS NULL LIMIT 1
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='wallet identity backfill is incomplete';
  END IF;

  INSERT IGNORE INTO admin_roles(code,name,description,is_built_in,sort)
  VALUES('tgbot_service','TG Bot Service','Telegram bot least-privilege service identity',1,70);
  DELETE FROM admin_role_permissions WHERE role_code='tgbot_service';
  INSERT IGNORE INTO admin_role_permissions(role_code,module,action,scope) VALUES
    ('tgbot_service','service','read','all'),
    ('tgbot_service','service','execute','all'),
    ('super_admin','approval','read','all'),
    ('operator','approval','read','all'),
    ('approver','approval','read','all'),
    ('compliance','approval','read','all'),
    ('support','approval','read','all'),
    ('auditor','approval','read','all'),
    ('approver','m7_user','approve','all'),
    ('approver','rbac','approve','all'),
    ('approver','news','approve','all'),
    ('super_admin','rbac','approve','all'),
    ('super_admin','news','approve','all');

  -- Existing bot login is narrowed automatically. Incrementing session_version
  -- ensures any former super-admin token cannot survive the role transition.
  UPDATE admin_users a
  SET a.role_code='tgbot_service', a.session_version=a.session_version+1
  WHERE a.username='tgbot_service' AND (
    a.role_code<>'tgbot_service' OR EXISTS (
      SELECT 1 FROM admin_user_roles r WHERE r.admin_id=a.id AND r.role_code<>'tgbot_service'
    )
  );
  DELETE r FROM admin_user_roles r JOIN admin_users a ON a.id=r.admin_id WHERE a.username='tgbot_service';
  INSERT IGNORE INTO admin_user_roles(admin_id,role_code,granted_at)
  SELECT id,'tgbot_service',CURRENT_TIMESTAMP FROM admin_users WHERE username='tgbot_service';
END$$
CALL jc_security_0830()$$
DROP PROCEDURE jc_security_0830$$

DROP TRIGGER IF EXISTS trg_user_wallet_verified_owner_insert$$
CREATE TRIGGER trg_user_wallet_verified_owner_insert
BEFORE INSERT ON user_wallet FOR EACH ROW
BEGIN
  IF NEW.verified_at IS NOT NULL AND NEW.chain='evm' AND NOT EXISTS (
    SELECT 1 FROM user_wallet_identity i WHERE i.wallet_address=LOWER(NEW.wallet_address) AND i.uid=NEW.uid AND i.chain='evm'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='verified EVM wallet lacks an immutable owner claim';
  END IF;
END$$

DROP TRIGGER IF EXISTS trg_user_wallet_verified_owner_update$$
CREATE TRIGGER trg_user_wallet_verified_owner_update
BEFORE UPDATE ON user_wallet FOR EACH ROW
BEGIN
  IF NEW.verified_at IS NOT NULL AND NEW.chain='evm' AND NOT EXISTS (
    SELECT 1 FROM user_wallet_identity i WHERE i.wallet_address=LOWER(NEW.wallet_address) AND i.uid=NEW.uid AND i.chain='evm'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='verified EVM wallet lacks an immutable owner claim';
  END IF;
END$$

DROP TRIGGER IF EXISTS trg_user_primary_identity_claim_insert$$
CREATE TRIGGER trg_user_primary_identity_claim_insert
AFTER INSERT ON `user` FOR EACH ROW
BEGIN
  IF NEW.wallet_address REGEXP '^0x[0-9A-Fa-f]{40}$' THEN
    INSERT INTO user_wallet_identity(wallet_address,uid,chain,claim_method,claimed_at)
    VALUES(LOWER(NEW.wallet_address),NEW.id,'evm','primary_registration',CURRENT_TIMESTAMP(6));
  END IF;
END$$

DROP TRIGGER IF EXISTS trg_user_primary_identity_require_update$$
CREATE TRIGGER trg_user_primary_identity_require_update
BEFORE UPDATE ON `user` FOR EACH ROW
BEGIN
  IF LOWER(NEW.wallet_address)<>LOWER(OLD.wallet_address) AND NOT EXISTS (
    SELECT 1 FROM user_wallet_identity i WHERE i.wallet_address=LOWER(NEW.wallet_address) AND i.uid=NEW.id AND i.chain='evm'
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='primary wallet lacks an immutable owner claim';
  END IF;
END$$

DROP TRIGGER IF EXISTS trg_user_wallet_identity_no_update$$
CREATE TRIGGER trg_user_wallet_identity_no_update
BEFORE UPDATE ON user_wallet_identity FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='verified wallet identities are non-recyclable';
END$$

DROP TRIGGER IF EXISTS trg_user_wallet_identity_no_delete$$
CREATE TRIGGER trg_user_wallet_identity_no_delete
BEFORE DELETE ON user_wallet_identity FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='verified wallet identities are non-recyclable';
END$$

DROP TRIGGER IF EXISTS trg_chain_event_failure_canonical_insert$$
CREATE TRIGGER trg_chain_event_failure_canonical_insert
BEFORE INSERT ON chain_event_failure FOR EACH ROW
BEGIN
  IF NEW.status<>'resolved' AND EXISTS (
    SELECT 1 FROM chain_event_record r
    WHERE r.block_number=NEW.block_number AND r.tx_hash=NEW.tx_hash AND r.log_index=NEW.log_index
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='canonical chain event cannot have an unresolved failure';
  END IF;
END$$

DROP TRIGGER IF EXISTS trg_chain_event_failure_canonical_update$$
CREATE TRIGGER trg_chain_event_failure_canonical_update
BEFORE UPDATE ON chain_event_failure FOR EACH ROW
BEGIN
  IF NEW.status<>'resolved' AND EXISTS (
    SELECT 1 FROM chain_event_record r
    WHERE r.block_number=NEW.block_number AND r.tx_hash=NEW.tx_hash AND r.log_index=NEW.log_index
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='canonical chain event cannot reopen an unresolved failure';
  END IF;
END$$

DROP PROCEDURE IF EXISTS jc_security_0830_postflight$$
CREATE PROCEDURE jc_security_0830_postflight()
BEGIN
  IF (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND (
      (TABLE_NAME='admin_users' AND COLUMN_NAME='session_version' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='1') OR
      (TABLE_NAME='admin_users' AND COLUMN_NAME='password_changed_at' AND COLUMN_TYPE='datetime' AND IS_NULLABLE='YES') OR
      (TABLE_NAME='admin_users' AND COLUMN_NAME='locked_until' AND COLUMN_TYPE='datetime' AND IS_NULLABLE='YES') OR
      (TABLE_NAME='admin_users' AND COLUMN_NAME='failed_login_count' AND COLUMN_TYPE='int' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='0') OR
      (TABLE_NAME='admin_users' AND COLUMN_NAME='deleted_at' AND COLUMN_TYPE='datetime' AND IS_NULLABLE='YES') OR
      (TABLE_NAME='user' AND COLUMN_NAME='is_enable' AND COLUMN_TYPE IN ('tinyint','tinyint(1)') AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='1') OR
      (TABLE_NAME='user' AND COLUMN_NAME='session_version' AND COLUMN_TYPE='bigint unsigned' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='1') OR
      (TABLE_NAME='user_wallet' AND COLUMN_NAME='verified_at' AND COLUMN_TYPE='datetime(6)' AND IS_NULLABLE='YES') OR
      (TABLE_NAME='user_wallet' AND COLUMN_NAME='verification_method' AND COLUMN_TYPE='varchar(32)' AND IS_NULLABLE='NO' AND COLUMN_DEFAULT='') OR
      (TABLE_NAME='user_wallet' AND COLUMN_NAME='verified_evm_identity' AND COLUMN_TYPE='varchar(42)' AND EXTRA LIKE '%STORED GENERATED%')
    ))<>10 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='08_30 security columns are incomplete';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet' AND INDEX_NAME='uk_user_wallet_verified_evm' AND NON_UNIQUE=0)<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='verified EVM wallet uniqueness is unavailable';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet_identity')<>5
     OR (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet_identity' AND CONSTRAINT_NAME='chk_user_wallet_identity' AND ENFORCED='YES')<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='non-recyclable wallet identity registry is unavailable';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='user_wallet' AND CONSTRAINT_NAME='chk_user_wallet_verification' AND ENFORCED='YES')<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='verified wallet CHECK is unavailable';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
      AND TRIGGER_NAME IN ('trg_chain_event_failure_canonical_insert','trg_chain_event_failure_canonical_update')
      AND ACTION_STATEMENT LIKE '%canonical chain event cannot%')<>2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='canonical event failure triggers are unavailable';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
      AND TRIGGER_NAME IN ('trg_user_wallet_identity_no_update','trg_user_wallet_identity_no_delete')
      AND ACTION_STATEMENT LIKE '%non-recyclable%')<>2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='wallet identity append-only guards are unavailable';
  END IF;
  IF (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()
      AND TRIGGER_NAME IN ('trg_user_wallet_verified_owner_insert','trg_user_wallet_verified_owner_update','trg_user_primary_identity_claim_insert','trg_user_primary_identity_require_update')
      AND ACTION_STATEMENT LIKE '%owner claim%')<>3
     OR (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME='trg_user_primary_identity_claim_insert' AND ACTION_STATEMENT LIKE '%INSERT INTO user_wallet_identity%')<>1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='global verified wallet ownership triggers are unavailable';
  END IF;
  IF (SELECT COUNT(*) FROM admin_role_permissions WHERE role_code='tgbot_service')<>2
     OR (SELECT COUNT(*) FROM admin_role_permissions WHERE role_code='tgbot_service' AND module='service' AND action IN ('read','execute') AND scope='all')<>2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='TG bot least-privilege permissions are unavailable';
  END IF;
END$$
CALL jc_security_0830_postflight()$$
DROP PROCEDURE jc_security_0830_postflight$$
DELIMITER ;

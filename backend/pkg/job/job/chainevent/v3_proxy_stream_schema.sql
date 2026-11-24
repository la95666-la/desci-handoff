-- Draft only. No production execution or writer switch is authorized.
SET NAMES utf8mb4;
DROP PROCEDURE IF EXISTS jc_v3_stream_install;
DELIMITER $$
CREATE PROCEDURE jc_v3_stream_install()
BEGIN
 IF COALESCE(@JC_V3_PROXY_STREAM_APPROVED,0)<>1 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V3 proxy stream migration approval required';
 END IF;
 IF EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='ido_v3_proxy_stream') THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V3 proxy stream already installed; review existing schema instead';
 END IF;
 CREATE TABLE IF NOT EXISTS ido_v3_proxy_stream (
  chain_id BIGINT UNSIGNED NOT NULL,
  proxy_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  mode VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  state VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  anchor_block BIGINT UNSIGNED NOT NULL,
  cursor_block BIGINT UNSIGNED NOT NULL,
  cursor_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  revision BIGINT UNSIGNED NOT NULL,
  epoch BIGINT UNSIGNED NOT NULL DEFAULT 1,
  observed_head BIGINT UNSIGNED NOT NULL DEFAULT 0,
  last_success_at DATETIME(6) NULL,
  PRIMARY KEY(chain_id,proxy_address,contract_version),
  CONSTRAINT chk_v3_stream CHECK(chain_id>0 AND contract_version='ido_v3_stablecoin' AND proxy_address REGEXP '^0x[0-9a-f]{40}$' AND mode IN ('shadow','stream') AND state IN ('running','halted') AND anchor_block>0 AND cursor_block>=anchor_block AND revision>0 AND epoch>0 AND cursor_hash REGEXP '^0x[0-9a-f]{64}$')
 ) ENGINE=InnoDB;
 CREATE TABLE IF NOT EXISTS ido_v3_proxy_coverage (
  chain_id BIGINT UNSIGNED NOT NULL,
  proxy_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  block_number BIGINT UNSIGNED NOT NULL,
  epoch BIGINT UNSIGNED NOT NULL,
  block_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  parent_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  -- Exact bytes: native MySQL JSON may round >64-bit numeric header/receipt
  -- fields. Keep validated JSON text rather than normalize those integers.
  payload LONGTEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  PRIMARY KEY(chain_id,proxy_address,contract_version,epoch,block_number),
  CONSTRAINT fk_v3_coverage_stream FOREIGN KEY(chain_id,proxy_address,contract_version) REFERENCES ido_v3_proxy_stream(chain_id,proxy_address,contract_version) ON DELETE RESTRICT,
  CONSTRAINT chk_v3_coverage CHECK(block_number>0 AND epoch>0 AND JSON_VALID(payload) AND block_hash REGEXP '^0x[0-9a-f]{64}$' AND parent_hash REGEXP '^0x[0-9a-f]{64}$' AND digest REGEXP '^[0-9a-f]{64}$')
 ) ENGINE=InnoDB;
 CREATE TABLE IF NOT EXISTS ido_v3_proxy_consumer (
  chain_id BIGINT UNSIGNED NOT NULL,
  proxy_address VARCHAR(42) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  contract_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id BIGINT UNSIGNED NOT NULL,
  cursor_block BIGINT UNSIGNED NOT NULL,
  cursor_hash VARCHAR(66) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  revision BIGINT UNSIGNED NOT NULL,
  last_success_at DATETIME(6) NOT NULL,
  PRIMARY KEY(chain_id,proxy_address,contract_version,project_id),
  CONSTRAINT fk_v3_consumer_binding FOREIGN KEY(chain_id,proxy_address,project_id,contract_version) REFERENCES ido_v3_project_binding(chain_id,proxy_address,project_id,contract_version) ON DELETE RESTRICT
 ) ENGINE=InnoDB;
 DROP TRIGGER IF EXISTS jc_v3_coverage_update;
 DROP TRIGGER IF EXISTS jc_v3_coverage_delete;
 DROP TRIGGER IF EXISTS jc_v3_stream_cursor_insert;
 DROP TRIGGER IF EXISTS jc_v3_stream_cursor_update;
END$$
DELIMITER ;
CALL jc_v3_stream_install();
DROP PROCEDURE jc_v3_stream_install;
-- Evidence is immutable, even on reorg. Recovery keeps the halted old stream;
-- replacing its prefix requires a separately reviewed migration/reconciliation.
DELIMITER $$
CREATE TRIGGER jc_v3_coverage_update BEFORE UPDATE ON ido_v3_proxy_coverage FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V3 stream evidence immutable'; END$$
CREATE TRIGGER jc_v3_coverage_delete BEFORE DELETE ON ido_v3_proxy_coverage FOR EACH ROW
BEGIN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V3 stream evidence retained'; END$$
CREATE TRIGGER jc_v3_stream_cursor_insert BEFORE INSERT ON ido_v3_scan_cursor FOR EACH ROW
BEGIN
 IF EXISTS(SELECT 1 FROM ido_v3_proxy_stream s WHERE s.chain_id=NEW.chain_id AND s.proxy_address=NEW.proxy_address AND s.contract_version=NEW.contract_version AND s.mode='stream') AND COALESCE(@jc_v3_stream_writer,0)<>1 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V3 legacy writer fenced by stream';
 END IF;
END$$
CREATE TRIGGER jc_v3_stream_cursor_update BEFORE UPDATE ON ido_v3_scan_cursor FOR EACH ROW
BEGIN
 IF EXISTS(SELECT 1 FROM ido_v3_proxy_stream s WHERE s.chain_id=NEW.chain_id AND s.proxy_address=NEW.proxy_address AND s.contract_version=NEW.contract_version AND s.mode='stream') AND COALESCE(@jc_v3_stream_writer,0)<>1 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='V3 legacy writer fenced by stream';
 END IF;
END$$
DELIMITER ;

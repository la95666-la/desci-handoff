-- Sprint 07-04 · project 双语字段 (English 主 + 繁體中文 副)
-- tagline / story_html / research_html / risks_html 4 个字段做双语并列
-- JSON 字段 (team/tokenomics/roadmap/documents/faq/onchain) 内嵌 _zh 后缀属性，无需加 DB 列
--
-- 幂等：用 information_schema 检查列是否存在（MySQL 8 不支持 ADD COLUMN IF NOT EXISTS）
SET NAMES utf8mb4;

-- helper macro replaced with inline prepared statements
DELIMITER $$
DROP PROCEDURE IF EXISTS jc_add_col_if_missing $$
CREATE PROCEDURE jc_add_col_if_missing(
    IN col_name VARCHAR(64),
    IN col_def  VARCHAR(255)
)
BEGIN
  IF NOT EXISTS (
      SELECT 1 FROM information_schema.columns
      WHERE table_schema = DATABASE() AND table_name = 'projects' AND column_name = col_name
  ) THEN
      SET @sql := CONCAT('ALTER TABLE projects ADD COLUMN ', col_name, ' ', col_def);
      PREPARE stmt FROM @sql;
      EXECUTE stmt;
      DEALLOCATE PREPARE stmt;
  END IF;
END $$
DELIMITER ;

CALL jc_add_col_if_missing('tagline_zh',       "VARCHAR(500) NOT NULL DEFAULT '' AFTER tagline");
CALL jc_add_col_if_missing('story_html_zh',    "MEDIUMTEXT NULL AFTER story_html");
CALL jc_add_col_if_missing('research_html_zh', "MEDIUMTEXT NULL AFTER research_html");
CALL jc_add_col_if_missing('risks_html_zh',    "MEDIUMTEXT NULL AFTER risks_html");

DROP PROCEDURE jc_add_col_if_missing;

-- Match the signed 64-bit ID accepted by the Launchpad engagement handlers.
-- projects.project_id is BIGINT UNSIGNED; current Sonyflake IDs fit BIGINT SIGNED.
-- Widen all four Sprint A project references together, preserving existing rows.
ALTER TABLE `project_updates`
  MODIFY COLUMN `project_id` BIGINT NOT NULL COMMENT '关联 project.project_id';
ALTER TABLE `project_subscribers`
  MODIFY COLUMN `project_id` BIGINT NOT NULL;
ALTER TABLE `project_watches`
  MODIFY COLUMN `project_id` BIGINT NOT NULL;
ALTER TABLE `project_whitelists`
  MODIFY COLUMN `project_id` BIGINT NOT NULL;

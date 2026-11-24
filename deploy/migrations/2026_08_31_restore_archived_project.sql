-- 2026_08_31_restore_archived_project.sql
-- Admin project restore: POST /admin/v1/projects/:id/restore
--
-- POST /admin/v1/projects/:id/disable archives a project by setting
-- status='archived'. Before this migration the pre-archive lifecycle status
-- was lost, so a later restore could only guess a target (e.g. pre_launch),
-- which the restore endpoint must never do unconditionally.
--
-- This column records the pre-archive status at disable time so restore can
-- revive the project to its exact prior lifecycle state. It is intentionally
-- NOT a boolean "was_pre_launch" flag because drafts can also be
-- Setup / Set / warmup and the recorded value is the restore target.
ALTER TABLE `projects`
  ADD COLUMN `pre_archive_status` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci
    NOT NULL DEFAULT '' COMMENT 'status before the project was archived (restore target)' AFTER `status`;

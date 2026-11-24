-- Sprint · 修根因：删除 user 时联动清关联表
-- 事件：手动 DELETE FROM user 时残留了 invite_user_code / user_point 等表的孤儿数据，
-- 导致新用户误用旧邀请码时 invite_record.inviter_uid 指向已删 uid，邀请奖励发到孤儿账号。
--
-- 修复策略：因 user.id (bigint unsigned) 与 uid (int) 列类型不兼容无法加 FK CASCADE，
-- 改在 backend admin service.DeleteUser 里用 SQL Transaction 联动清理（见 pkg/core/admin/service/user）。
-- 本迁移只做 **孤儿数据清理**，把历史遗留干净。

SET NAMES utf8mb4;

DELETE FROM user_point           WHERE uid NOT IN (SELECT id FROM user);
DELETE FROM user_point_flow      WHERE uid NOT IN (SELECT id FROM user);
DELETE FROM user_check_in_record WHERE uid NOT IN (SELECT id FROM user);
DELETE FROM task_record          WHERE uid NOT IN (SELECT id FROM user);
DELETE FROM invite_user_code     WHERE uid NOT IN (SELECT id FROM user);
DELETE FROM invite_record        WHERE uid NOT IN (SELECT id FROM user);
UPDATE invite_record SET inviter_uid = 0 WHERE inviter_uid > 0 AND inviter_uid NOT IN (SELECT id FROM user);

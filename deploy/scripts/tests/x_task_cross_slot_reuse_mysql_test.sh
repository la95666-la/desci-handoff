#!/usr/bin/env bash
# Isolated MySQL 8.4 integration check. Requires a local Docker daemon and
# the mysql:8.4.10 image; never connects to a production/test database.
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
name="jc-x-reuse-test-$$"
docker run -d --name "$name" --network none -e MYSQL_ALLOW_EMPTY_PASSWORD=yes mysql:8.4.10 --skip-log-bin >/dev/null
trap 'status=$?; if (( status != 0 )); then docker logs --tail 35 "$name" >&2 || true; fi; docker rm -f "$name" >/dev/null 2>&1 || true' EXIT
for i in $(seq 1 90); do
  # TCP waits for the final server, not the temporary initialization server.
  if docker exec "$name" mysql -uroot -h127.0.0.1 -e 'SELECT 1' >/dev/null 2>&1; then break; fi
  sleep 1
done
docker exec "$name" mysql -uroot -h127.0.0.1 -e 'SELECT 1' >/dev/null
docker exec -i "$name" mysql -uroot -h127.0.0.1 <<'SQL'
CREATE DATABASE x_reuse;
USE x_reuse;
CREATE TABLE task (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 platform VARCHAR(20) NOT NULL,
 task_key VARCHAR(50) NOT NULL,
 target_slot TINYINT UNSIGNED NOT NULL,
 target_ref VARCHAR(200) NOT NULL,
 UNIQUE INDEX uk_platform_key_slot (platform,task_key,target_slot),
 UNIQUE INDEX uk_platform_key_target (platform,task_key,target_ref)
);
CREATE TABLE task_target_history (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 platform VARCHAR(20) NOT NULL,
 task_key VARCHAR(50) NOT NULL,
 target_ref VARCHAR(200) NOT NULL,
 task_id BIGINT UNSIGNED NOT NULL,
 task_version INT UNSIGNED NOT NULL,
 UNIQUE INDEX uk_task_target_history (platform,task_key,target_ref),
 UNIQUE INDEX uk_task_target_history_round (task_id,task_version)
);
CREATE TABLE task_round (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 task_id BIGINT UNSIGNED NOT NULL,
 version INT UNSIGNED NOT NULL,
 target_ref VARCHAR(200) NOT NULL,
 UNIQUE INDEX uk_task_round (task_id,version),
 UNIQUE INDEX uk_task_target (task_id,target_ref)
);
INSERT INTO task VALUES (1,'twitter','like',1,'new'),(2,'twitter','like',2,'other');
INSERT INTO task_target_history VALUES (1,'twitter','like','old',1,1);
INSERT INTO task_round VALUES (1,1,1,'old');
SQL
for pass in 1 2; do
  docker exec -i "$name" mysql -uroot -h127.0.0.1 x_reuse < "$root/migrations/2026_10_05_allow_x_post_cross_slot_reuse.sql"
done
docker exec -i "$name" mysql -uroot -h127.0.0.1 x_reuse <<'SQL'
INSERT INTO task_target_history VALUES (2,'twitter','like','old',2,1);
UPDATE task SET target_ref='old' WHERE id=2;
SQL
if docker exec "$name" mysql -uroot -h127.0.0.1 x_reuse -e "INSERT INTO task_target_history VALUES (3,'twitter','like','else',2,1);" >/dev/null 2>&1; then
  echo 'FAIL: repeated task/version accepted' >&2; exit 1
fi
if docker exec "$name" mysql -uroot -h127.0.0.1 x_reuse -e "INSERT INTO task_round VALUES (2,1,2,'old');" >/dev/null 2>&1; then
  echo 'FAIL: repeated same-task target accepted' >&2; exit 1
fi
if docker exec "$name" mysql -uroot -h127.0.0.1 x_reuse -e "INSERT INTO task VALUES (3,'twitter','like',2,'another');" >/dev/null 2>&1; then
  echo 'FAIL: repeated slot accepted' >&2; exit 1
fi
docker exec "$name" mysql -uroot -h127.0.0.1 x_reuse -e 'SELECT task_id,task_version,target_ref FROM task_target_history ORDER BY task_id;'
echo 'PASS: cross-slot history reuse, migration replay, and retained per-task/slot uniqueness'

#!/usr/bin/env bash
# Controlled MySQL migration entrypoint. This command intentionally does not
# discover credentials, create a migration ledger, make backups, deploy
# services, or activate any Fair Launch runtime authority.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MIGRATION_DIR="$ROOT/migrations"

usage() {
  cat <<'EOF'
Usage:
  deploy.sh migrate status
  deploy.sh migrate preflight --defaults-extra-file FILE --database DB \
    --manifest FILE --backup-receipt FILE --evidence-dir DIR
  deploy.sh migrate apply --defaults-extra-file FILE --database DB \
    --manifest FILE --backup-receipt FILE --evidence-dir DIR \
    --confirm APPLY-CHECKSUM-BOUND-MIGRATIONS

The entrypoint is available, but apply remains fail-closed unless all of these
are supplied and verified:
  * a protected MySQL defaults-extra-file (password never enters argv/logs);
  * an external, protected, checksum-bound migration manifest;
  * a protected verified backup receipt and its dump artifact;
  * an existing exact schema_migrations ledger with no applying row;
  * exact database/server/version identity and repository SQL checksums;
  * a new evidence directory; and
  * the explicit apply confirmation token.

Manifest format (key=value, one migration per tab-separated migration line):
  format=JOINCARE_MYSQL_MIGRATION_MANIFEST_V1
  database=joincare
  server_uuid=<lowercase UUID>
  mysql_version=8.4.10
  backup_receipt_sha256=<64 lowercase hex>
  migration=<64 lowercase hex><TAB><migration filename.sql>

Backup receipt format:
  format=JOINCARE_MYSQL_BACKUP_RECEIPT_V1
  database=joincare
  server_uuid=<lowercase UUID>
  dump_path=<absolute path to immutable logical backup>
  dump_sha256=<64 lowercase hex>
  restore_test=passed
  status=verified

The manifest and receipt must be regular non-symlink files outside this
repository and must not be group/world accessible. The dump must be a regular
non-symlink file. Migrations execute strictly in manifest order, one at a time,
with an applying -> applied ledger transition. A failure leaves the applying
row as incident evidence and stops immediately; this command never auto-resumes.

This command does not start Backend, Job, scanner or materializer and performs
no chain, Timelock, runtime-authority or real-funds operation.
EOF
}

fail() { printf 'database_migration_rejected: %s\n' "$*" >&2; exit 78; }
die() { printf 'database_migration_failed: %s\n' "$*" >&2; exit 1; }

ACTION="${1:-}"
case "$ACTION" in
  status)
    [ "$#" -eq 1 ] || { usage >&2; exit 2; }
    printf '%s\n' 'database_migration_entrypoint=available_controlled'
    printf '%s\n' 'database_migration_apply=requires_external_manifest_backup_receipt_and_confirmation'
    exit 0
    ;;
  help|--help|-h|'') usage; exit 0 ;;
  preflight|apply) shift ;;
  *) usage >&2; exit 2 ;;
esac

DEFAULTS_FILE=''
DATABASE=''
MANIFEST=''
BACKUP_RECEIPT=''
EVIDENCE_DIR=''
CONFIRM=''
while [ "$#" -gt 0 ]; do
  case "$1" in
    --defaults-extra-file) [ "$#" -ge 2 ] || exit 2; DEFAULTS_FILE="$2"; shift 2 ;;
    --database) [ "$#" -ge 2 ] || exit 2; DATABASE="$2"; shift 2 ;;
    --manifest) [ "$#" -ge 2 ] || exit 2; MANIFEST="$2"; shift 2 ;;
    --backup-receipt) [ "$#" -ge 2 ] || exit 2; BACKUP_RECEIPT="$2"; shift 2 ;;
    --evidence-dir) [ "$#" -ge 2 ] || exit 2; EVIDENCE_DIR="$2"; shift 2 ;;
    --confirm) [ "$#" -ge 2 ] || exit 2; CONFIRM="$2"; shift 2 ;;
    --help|-h) usage; exit 0 ;;
    *) printf 'unknown migrate option: %s\n' "$1" >&2; usage >&2; exit 2 ;;
  esac
done

[ -n "$DEFAULTS_FILE" ] || fail 'defaults_extra_file_required'
[ -n "$DATABASE" ] || fail 'database_required'
[ -n "$MANIFEST" ] || fail 'manifest_required'
[ -n "$BACKUP_RECEIPT" ] || fail 'backup_receipt_required'
[ -n "$EVIDENCE_DIR" ] || fail 'evidence_dir_required'
[[ "$DATABASE" =~ ^[A-Za-z0-9_]{1,64}$ ]] || fail 'invalid_database_name'
[ "$ACTION" != apply ] || [ "$CONFIRM" = 'APPLY-CHECKSUM-BOUND-MIGRATIONS' ] || fail 'explicit_apply_confirmation_required'
[ "$(uname -s)" = Linux ] || fail 'linux_permission_semantics_required'

for command in mysql sha256sum awk stat date; do
  command -v "$command" >/dev/null 2>&1 || fail "required_command_missing:$command"
done

canonical_path() {
  local target="$1" dir base
  dir="$(cd "$(dirname "$target")" 2>/dev/null && pwd -P)" || return 1
  base="$(basename "$target")"
  printf '%s/%s\n' "$dir" "$base"
}

require_protected_file() {
  local label="$1" file="$2" mode
  [ -f "$file" ] || fail "${label}_not_regular_file"
  [ ! -L "$file" ] || fail "${label}_symlink_rejected"
  mode="$(stat -c '%a' "$file" 2>/dev/null)" || fail "${label}_mode_unavailable"
  [[ "$mode" =~ ^[0-7]{3,4}$ ]] || fail "${label}_invalid_mode"
  (( (8#$mode & 077) == 0 )) || fail "${label}_must_not_be_group_or_world_accessible"
}

require_external_file() {
  local label="$1" file="$2" canonical
  require_protected_file "$label" "$file"
  canonical="$(canonical_path "$file")" || fail "${label}_canonical_path_unavailable"
  [[ "$canonical" != "$ROOT/"* ]] || fail "${label}_must_be_outside_repository"
}

kv_value() {
  local file="$1" key="$2"
  awk -F= -v key="$key" '
    $1==key { count++; sub(/^[^=]*=/,""); value=$0 }
    END { if(count==1) print value; else exit 42 }
  ' "$file"
}

require_external_file defaults_extra_file "$DEFAULTS_FILE"
require_external_file manifest "$MANIFEST"
require_external_file backup_receipt "$BACKUP_RECEIPT"
[ ! -e "$EVIDENCE_DIR" ] || fail 'evidence_dir_already_exists'
mkdir -m 700 "$EVIDENCE_DIR" || fail 'cannot_create_evidence_dir'
EVIDENCE_DIR="$(canonical_path "$EVIDENCE_DIR")" || fail 'evidence_dir_canonical_path_unavailable'
case "$EVIDENCE_DIR" in "$ROOT"/*) fail 'evidence_dir_must_be_outside_repository' ;; esac

MANIFEST_FORMAT="$(kv_value "$MANIFEST" format)" || fail 'manifest_format_missing_or_duplicated'
MANIFEST_DATABASE="$(kv_value "$MANIFEST" database)" || fail 'manifest_database_missing_or_duplicated'
MANIFEST_SERVER_UUID="$(kv_value "$MANIFEST" server_uuid)" || fail 'manifest_server_uuid_missing_or_duplicated'
MANIFEST_MYSQL_VERSION="$(kv_value "$MANIFEST" mysql_version)" || fail 'manifest_mysql_version_missing_or_duplicated'
MANIFEST_BACKUP_RECEIPT_SHA="$(kv_value "$MANIFEST" backup_receipt_sha256)" || fail 'manifest_backup_receipt_sha256_missing_or_duplicated'
[ "$MANIFEST_FORMAT" = 'JOINCARE_MYSQL_MIGRATION_MANIFEST_V1' ] || fail 'manifest_format_mismatch'
[ "$MANIFEST_DATABASE" = "$DATABASE" ] || fail 'manifest_database_mismatch'
[[ "$MANIFEST_SERVER_UUID" =~ ^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$ ]] || fail 'manifest_server_uuid_invalid'
[[ "$MANIFEST_MYSQL_VERSION" =~ ^8[.][0-9]+[.][0-9]+([-.+][0-9A-Za-z._-]+)?$ ]] || fail 'manifest_mysql_version_not_mysql8'
[[ "$MANIFEST_BACKUP_RECEIPT_SHA" =~ ^[0-9a-f]{64}$ ]] || fail 'manifest_backup_receipt_sha256_invalid'

RECEIPT_SHA="$(sha256sum "$BACKUP_RECEIPT" | awk '{print $1}')"
[ "$RECEIPT_SHA" = "$MANIFEST_BACKUP_RECEIPT_SHA" ] || fail 'backup_receipt_checksum_mismatch'
RECEIPT_FORMAT="$(kv_value "$BACKUP_RECEIPT" format)" || fail 'backup_receipt_format_missing_or_duplicated'
RECEIPT_DATABASE="$(kv_value "$BACKUP_RECEIPT" database)" || fail 'backup_receipt_database_missing_or_duplicated'
RECEIPT_SERVER_UUID="$(kv_value "$BACKUP_RECEIPT" server_uuid)" || fail 'backup_receipt_server_uuid_missing_or_duplicated'
DUMP_PATH="$(kv_value "$BACKUP_RECEIPT" dump_path)" || fail 'backup_receipt_dump_path_missing_or_duplicated'
DUMP_SHA="$(kv_value "$BACKUP_RECEIPT" dump_sha256)" || fail 'backup_receipt_dump_sha256_missing_or_duplicated'
RESTORE_TEST="$(kv_value "$BACKUP_RECEIPT" restore_test)" || fail 'backup_receipt_restore_test_missing_or_duplicated'
BACKUP_STATUS="$(kv_value "$BACKUP_RECEIPT" status)" || fail 'backup_receipt_status_missing_or_duplicated'
[ "$RECEIPT_FORMAT" = 'JOINCARE_MYSQL_BACKUP_RECEIPT_V1' ] || fail 'backup_receipt_format_mismatch'
[ "$RECEIPT_DATABASE" = "$DATABASE" ] || fail 'backup_receipt_database_mismatch'
[ "$RECEIPT_SERVER_UUID" = "$MANIFEST_SERVER_UUID" ] || fail 'backup_receipt_server_uuid_mismatch'
[[ "$DUMP_PATH" = /* ]] || fail 'backup_dump_path_must_be_absolute'
[[ "$DUMP_SHA" =~ ^[0-9a-f]{64}$ ]] || fail 'backup_dump_sha256_invalid'
[ "$RESTORE_TEST" = passed ] || fail 'backup_restore_test_not_passed'
[ "$BACKUP_STATUS" = verified ] || fail 'backup_status_not_verified'
require_external_file backup_dump "$DUMP_PATH"
[ "$(sha256sum "$DUMP_PATH" | awk '{print $1}')" = "$DUMP_SHA" ] || fail 'backup_dump_checksum_mismatch'

mapfile -t MIGRATION_ROWS < <(awk -F= '$1=="migration" { sub(/^[^=]*=/,""); print }' "$MANIFEST")
[ "${#MIGRATION_ROWS[@]}" -gt 0 ] || fail 'manifest_has_no_migrations'
MIGRATION_FILES=()
MIGRATION_SHAS=()
previous=''
declare -A seen=()
for row in "${MIGRATION_ROWS[@]}"; do
  IFS=$'\t' read -r checksum filename extra <<<"$row"
  [ -z "${extra:-}" ] || fail 'migration_row_has_extra_fields'
  [[ "$checksum" =~ ^[0-9a-f]{64}$ ]] || fail 'migration_checksum_invalid'
  [[ "$filename" =~ ^20[0-9]{2}_[0-9]{2}_[0-9]{2}_[a-z0-9_]+[.]sql$ ]] || fail 'migration_filename_invalid'
  [ -z "${seen[$filename]:-}" ] || fail "duplicate_migration:$filename"
  [ -z "$previous" ] || [[ "$filename" > "$previous" ]] || fail 'migration_manifest_not_strictly_ordered'
  [ -f "$MIGRATION_DIR/$filename" ] || fail "migration_file_missing:$filename"
  actual="$(sha256sum "$MIGRATION_DIR/$filename" | awk '{print $1}')"
  [ "$actual" = "$checksum" ] || fail "migration_repository_checksum_mismatch:$filename"
  seen[$filename]=1
  previous="$filename"
  MIGRATION_FILES+=("$filename")
  MIGRATION_SHAS+=("$checksum")
done

mysql_base=(mysql "--defaults-extra-file=$DEFAULTS_FILE" --batch --raw --skip-column-names --connect-timeout=10 "--database=$DATABASE")
query() { printf '%s\n' "$1" | "${mysql_base[@]}" | tr -d '\r'; }

IDENTITY="$(query 'SELECT CONCAT(LOWER(@@server_uuid),CHAR(9),VERSION(),CHAR(9),DATABASE(),CHAR(9),@@session.time_zone,CHAR(9),@@global.time_zone);')" || fail 'database_identity_query_failed'
IFS=$'\t' read -r DB_SERVER_UUID DB_MYSQL_VERSION DB_DATABASE SESSION_TZ GLOBAL_TZ <<<"$IDENTITY"
[ "$DB_SERVER_UUID" = "$MANIFEST_SERVER_UUID" ] || fail 'database_server_uuid_mismatch'
[ "$DB_MYSQL_VERSION" = "$MANIFEST_MYSQL_VERSION" ] || fail 'database_mysql_version_mismatch'
[ "$DB_DATABASE" = "$DATABASE" ] || fail 'connected_database_mismatch'

LEDGER_EXISTS="$(query "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='schema_migrations' AND TABLE_TYPE='BASE TABLE';")" || fail 'migration_ledger_probe_failed'
[ "$LEDGER_EXISTS" = 1 ] || fail 'migration_ledger_missing_manual_baseline_reconciliation_required'
LEDGER_SHAPE="$(query "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='schema_migrations' AND ((COLUMN_NAME='filename' AND COLUMN_TYPE='varchar(255)' AND IS_NULLABLE='NO') OR (COLUMN_NAME='checksum' AND COLUMN_TYPE='char(64)' AND IS_NULLABLE='NO') OR (COLUMN_NAME='status' AND COLUMN_TYPE='varchar(16)' AND IS_NULLABLE='NO') OR (COLUMN_NAME='applied_at' AND COLUMN_TYPE='datetime(6)' AND IS_NULLABLE='NO'));")" || fail 'migration_ledger_shape_probe_failed'
[ "$LEDGER_SHAPE" = 4 ] || fail 'migration_ledger_shape_mismatch'
[ "$(query "SELECT COUNT(*) FROM schema_migrations WHERE status='applying';")" = 0 ] || fail 'migration_ledger_has_applying_incident'
[ "$(query "SELECT COUNT(*) FROM schema_migrations WHERE status NOT IN ('applying','applied');")" = 0 ] || fail 'migration_ledger_has_unknown_status'

PENDING=()
for index in "${!MIGRATION_FILES[@]}"; do
  filename="${MIGRATION_FILES[$index]}"
  checksum="${MIGRATION_SHAS[$index]}"
  row="$(query "SELECT CONCAT(checksum,CHAR(9),status) FROM schema_migrations WHERE filename='$filename';")" || fail "migration_ledger_read_failed:$filename"
  if [ -n "$row" ]; then
    [ "$(printf '%s\n' "$row" | wc -l | tr -d ' ')" = 1 ] || fail "migration_ledger_duplicate:$filename"
    IFS=$'\t' read -r ledger_checksum ledger_status <<<"$row"
    [ "$ledger_checksum" = "$checksum" ] || fail "migration_ledger_checksum_mismatch:$filename"
    [ "$ledger_status" = applied ] || fail "migration_not_applied_or_clean:$filename"
  else
    PENDING+=("$index")
  fi
done

MANIFEST_SHA="$(sha256sum "$MANIFEST" | awk '{print $1}')"
cat > "$EVIDENCE_DIR/preflight.txt" <<EOF
format=JOINCARE_MYSQL_MIGRATION_PREFLIGHT_V1
action=$ACTION
database=$DATABASE
server_uuid=$DB_SERVER_UUID
mysql_version=$DB_MYSQL_VERSION
session_time_zone=$SESSION_TZ
global_time_zone=$GLOBAL_TZ
manifest_sha256=$MANIFEST_SHA
backup_receipt_sha256=$RECEIPT_SHA
backup_dump_sha256=$DUMP_SHA
migration_count=${#MIGRATION_FILES[@]}
pending_count=${#PENDING[@]}
ledger_applying_count=0
preflight=passed
EOF
chmod 600 "$EVIDENCE_DIR/preflight.txt"
sha256sum "$EVIDENCE_DIR/preflight.txt" > "$EVIDENCE_DIR/preflight.txt.sha256"
chmod 600 "$EVIDENCE_DIR/preflight.txt.sha256"

if [ "$ACTION" = preflight ]; then
  printf 'database_migration_preflight=passed\n'
  printf 'migration_count=%s\n' "${#MIGRATION_FILES[@]}"
  printf 'pending_count=%s\n' "${#PENDING[@]}"
  printf 'evidence_dir=%s\n' "$EVIDENCE_DIR"
  exit 0
fi

gate_for() {
  case "$1" in
    2026_09_0[3-7]_*) printf '%s\n' 'SET @JC_FAIR_LAUNCH_MIGRATION_APPROVED=1;' ;;
    2026_09_0[8-9]_*|2026_09_1[0-2]_*)
      printf '%s\n' 'SET @JC_FAIR_LAUNCH_LOCAL_E2E_APPROVED=1;'
      printf "%s\n" "SET @JC_FAIR_LAUNCH_LOCAL_E2E_RUN_ID='$(cat /proc/sys/kernel/random/uuid)';"
      ;;
    2026_09_13_*) printf '%s\n' 'SET @JC_FAIR_LAUNCH_MAINNET_ACTIVATION_APPROVED=1;' ;;
    2026_09_14_*) printf '%s\n' 'SET @JC_FAIR_LAUNCH_MAINNET_BOOTSTRAP_APPROVED=1;' ;;
    2026_09_15_*) printf '%s\n' 'SET @JC_FAIR_LAUNCH_MAINNET_SCANNER_APPROVED=1;' ;;
    2026_09_16_*) printf '%s\n' 'SET @JC_FAIR_LAUNCH_MAINNET_RECOVERY_APPROVED=1;' ;;
    2026_09_17_*) printf '%s\n' 'SET @JC_FAIR_LAUNCH_MAINNET_RECOVERY_PROVIDER_APPROVED=1;' ;;
    2026_09_18_*) printf '%s\n' 'SET @JC_FAIR_LAUNCH_MAINNET_MATERIALIZATION_APPROVED=1;' ;;
  esac
}

APPLY_LOG="$EVIDENCE_DIR/apply.txt"
: > "$APPLY_LOG"
chmod 600 "$APPLY_LOG"
for index in "${PENDING[@]}"; do
  filename="${MIGRATION_FILES[$index]}"
  checksum="${MIGRATION_SHAS[$index]}"
  printf 'migration=%s status=applying checksum=%s\n' "$filename" "$checksum" | tee -a "$APPLY_LOG"
  query "INSERT INTO schema_migrations(filename,checksum,status) VALUES ('$filename','$checksum','applying');" >/dev/null || die "ledger_begin_failed:$filename"
  if ! { gate_for "$filename"; cat "$MIGRATION_DIR/$filename"; } | "${mysql_base[@]}" >>"$APPLY_LOG" 2>&1; then
    printf 'migration=%s status=failed ledger=applying\n' "$filename" | tee -a "$APPLY_LOG" >&2
    sha256sum "$APPLY_LOG" > "$EVIDENCE_DIR/apply.txt.sha256"
    chmod 600 "$EVIDENCE_DIR/apply.txt.sha256"
    die "migration_sql_failed:$filename"
  fi
  updated="$(query "UPDATE schema_migrations SET status='applied',applied_at=CURRENT_TIMESTAMP(6) WHERE filename='$filename' AND checksum='$checksum' AND status='applying'; SELECT ROW_COUNT();")" || die "ledger_finish_failed:$filename"
  [ "$updated" = 1 ] || die "ledger_finish_row_count_mismatch:$filename"
  printf 'migration=%s status=applied checksum=%s\n' "$filename" "$checksum" | tee -a "$APPLY_LOG"
done

[ "$(query "SELECT COUNT(*) FROM schema_migrations WHERE status='applying';")" = 0 ] || die 'post_apply_ledger_has_applying_rows'
for index in "${!MIGRATION_FILES[@]}"; do
  filename="${MIGRATION_FILES[$index]}"; checksum="${MIGRATION_SHAS[$index]}"
  [ "$(query "SELECT COUNT(*) FROM schema_migrations WHERE filename='$filename' AND checksum='$checksum' AND status='applied';")" = 1 ] || die "post_apply_ledger_verification_failed:$filename"
done
printf 'database_migration_apply=passed\n' | tee -a "$APPLY_LOG"
sha256sum "$APPLY_LOG" > "$EVIDENCE_DIR/apply.txt.sha256"
chmod 600 "$EVIDENCE_DIR/apply.txt.sha256"
printf 'evidence_dir=%s\n' "$EVIDENCE_DIR"

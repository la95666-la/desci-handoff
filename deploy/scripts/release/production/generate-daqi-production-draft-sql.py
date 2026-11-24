#!/usr/bin/env python3
"""Generate fail-closed SQL for the DaQi archived Launchpad draft.

The generated SQL contains no credentials. Production execution requires a
session-scoped confirmation token bound to the verified pre-create backup.
"""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path


DATABASE = "joincare"
SERVER_UUID = "78dbed8b-77a9-11f1-ac1c-06eab0296b63"
MYSQL_VERSION = "8.4.10"
BACKUP_SHA256 = "84f0ec230a2d3469cc1547b10eb4487698b7ac9d8a043af3f7be6fe38e4cfc26"
SOURCE_SHA256 = "47223dd90ad2537105dd26faf1b14787c1e142cd558909f1190381affbc06b61"
PROJECT_ID = 4
SLUG = "daqi-inference"
CREATE_TOKEN = f"CREATE_DAQI_ARCHIVED_DRAFT_{BACKUP_SHA256[:16]}"
ROLLBACK_TOKEN = f"ROLLBACK_DAQI_ARCHIVED_DRAFT_{BACKUP_SHA256[:16]}"

JSON_FIELDS = (
    "team_json",
    "tokenomics_json",
    "roadmap_json",
    "documents_json",
    "faq_json",
)
HTML_FIELDS = (
    "story_html",
    "story_html_zh",
    "research_html",
    "research_html_zh",
    "risks_html",
    "risks_html_zh",
)

AUXILIARY_TABLES = (
    "chain_event_reorg_assessment_disposition",
    "chain_event_reorg_generation_submission",
    "chain_event_reorg_materialization_effect",
    "chain_tx_submission",
    "fair_launch_action_request",
    "fair_launch_ledger_entry_v3",
    "fair_launch_mainnet_project_admission_v1",
    "fair_launch_mainnet_project_bootstrap_receipt_v1",
    "fair_launch_mainnet_watermark_v1",
    "fair_launch_mfa_assertion",
    "fair_launch_position",
    "fair_launch_position_v3",
    "fair_launch_preview",
    "fair_launch_project_binding_v3",
    "fair_launch_project_state_v3",
    "fund_ledger",
    "ido_purchase_entitlement",
    "ido_reconciliation_job",
    "ido_reconciliation_manifest",
    "ido_reconciliation_run",
    "project_files",
    "project_subscribers",
    "project_tge",
    "project_updates",
    "project_watches",
    "project_whitelists",
    "transaction_record",
    "user_claim_record",
    "user_project",
    "user_refund_record",
)


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def compact_json(value: object) -> str:
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"))


def utf8_hex(value: str) -> str:
    return "0x" + value.encode("utf-8").hex()


def text_expr(value: str) -> str:
    if value == "":
        return "_utf8mb4'' COLLATE utf8mb4_unicode_ci"
    return f"CONVERT({utf8_hex(value)} USING utf8mb4) COLLATE utf8mb4_unicode_ci"


def sql_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def assertion(code: str, condition: str) -> str:
    return (
        "INSERT INTO jc_daqi_assertion(assertion_code, ok) "
        f"SELECT {sql_quote(code)}, IF(({condition}), 1, 0);"
    )


def json_equal_condition(field: str, value: str, alias: str = "p") -> str:
    return f"{alias}.{field}=CAST({text_expr(value)} AS JSON)"


def text_hash_condition(field: str, value: str, alias: str = "p") -> str:
    digest = sha256(value.encode("utf-8"))
    return f"SHA2({alias}.{field},256)={sql_quote(digest)}"


def exact_row_conditions(content: dict[str, object], alias: str = "p") -> list[str]:
    conditions = [
        f"{alias}.project_id={PROJECT_ID}",
        f"{alias}.name_en={text_expr(str(content['name_en']))}",
        f"{alias}.name_cn={text_expr(str(content['name_cn']))}",
        f"{alias}.slug={sql_quote(SLUG)}",
        f"{alias}.status='archived'",
        f"{alias}.pre_archive_status='pre_launch'",
        f"{alias}.contract_version='unassigned'",
        f"{alias}.funding_mode='capped_presale'",
        f"{alias}.onchain_locked=0",
        f"{alias}.owner_address=''",
        f"{alias}.chain_id=0",
        f"{alias}.contract_address=''",
        f"{alias}.token_address=''",
        f"{alias}.token_symbol=''",
        f"{alias}.payment_token=''",
        f"{alias}.payment_token_address=''",
        f"{alias}.fee_receiver=''",
        f"{alias}.success_fee_rate=0",
        f"{alias}.token_price_raw='0'",
        f"{alias}.hard_cap_raw='0'",
        f"{alias}.soft_cap_raw='0'",
        f"{alias}.max_per_user_raw='0'",
        f"{alias}.total_raised_raw='0'",
        f"{alias}.total_deposited_raw='0'",
        f"{alias}.sale_token_amount_raw IS NULL",
        f"{alias}.min_contribution_raw IS NULL",
        f"{alias}.max_contribution_raw IS NULL",
        f"{alias}.final_total_raised_raw IS NULL",
        f"{alias}.final_sale_token_amount_raw IS NULL",
        f"{alias}.launch_at IS NULL",
        f"CAST({alias}.onchain_json AS CHAR)='{{}}'",
    ]
    for field in (
        "tagline",
        "tagline_zh",
        "description",
        *HTML_FIELDS,
    ):
        conditions.append(text_hash_condition(field, str(content[field]), alias))
    tags = compact_json(content["tags"])
    conditions.append(text_hash_condition("tags", tags, alias))
    for field in JSON_FIELDS:
        conditions.append(json_equal_condition(field, compact_json(content[field]), alias))
    return conditions


def auxiliary_count_expression() -> str:
    return " +\n    ".join(
        f"(SELECT COUNT(*) FROM `{table}` WHERE project_id={PROJECT_ID})"
        for table in AUXILIARY_TABLES
    )


def header(title: str) -> str:
    return f"""-- {title}
-- Database: {DATABASE}
-- server_uuid: {SERVER_UUID}
-- MySQL: {MYSQL_VERSION}
-- Pre-create backup SHA-256: {BACKUP_SHA256}
-- Content source SHA-256: {SOURCE_SHA256}
-- Safety identity: archived -> pre_launch, unassigned, capped_presale,
--                  onchain_locked=0, no wallet/token/contract/economic values.
-- Generated file; do not edit by hand.

SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci;
SET time_zone = '+00:00';
"""


def assertion_table() -> str:
    return """DROP TEMPORARY TABLE IF EXISTS jc_daqi_assertion;
CREATE TEMPORARY TABLE jc_daqi_assertion (
  assertion_code VARCHAR(128) NOT NULL,
  ok TINYINT NOT NULL,
  CONSTRAINT chk_jc_daqi_assertion_ok CHECK (ok = 1)
) ENGINE=InnoDB;
"""


def preflight_sql(content: dict[str, object]) -> str:
    exact = " AND\n      ".join(exact_row_conditions(content))
    statements = [
        header("DaQi Inference production draft preflight (read-only)"),
        assertion_table(),
        assertion("database", f"DATABASE()={sql_quote(DATABASE)}"),
        assertion("server_uuid", f"@@server_uuid={sql_quote(SERVER_UUID)}"),
        assertion("mysql_version", f"VERSION()={sql_quote(MYSQL_VERSION)}"),
        assertion("timezone", "@@session.time_zone='+00:00'"),
        assertion("project_id_conflict", f"(SELECT COUNT(*) FROM projects WHERE project_id={PROJECT_ID}) IN (0,1)"),
        assertion(
            "identity_conflict",
            "(SELECT COUNT(*) FROM projects WHERE "
            f"(slug={sql_quote(SLUG)} OR name_en={text_expr(str(content['name_en']))} "
            f"OR name_cn={text_expr(str(content['name_cn']))}) AND project_id<>{PROJECT_ID})=0",
        ),
        assertion(
            "allocation",
            f"((SELECT COUNT(*) FROM projects WHERE project_id={PROJECT_ID})=1 OR "
            f"(SELECT COALESCE(MAX(project_id),0) FROM projects)={PROJECT_ID - 1})",
        ),
        assertion(
            "existing_row_exact",
            f"((SELECT COUNT(*) FROM projects WHERE project_id={PROJECT_ID})=0 OR "
            f"(SELECT COUNT(*) FROM projects p WHERE {exact})=1)",
        ),
        assertion("no_auxiliary_rows", f"({auxiliary_count_expression()})=0"),
        "SELECT CONCAT('preflight|PASS|project_id=', "
        f"{PROJECT_ID}, '|existing=', (SELECT COUNT(*) FROM projects WHERE project_id={PROJECT_ID}), "
        "'|max_project_id=', (SELECT COALESCE(MAX(project_id),0) FROM projects), "
        "'|project_count=', (SELECT COUNT(*) FROM projects));",
    ]
    return "\n".join(statements) + "\n"


def create_sql(content: dict[str, object]) -> str:
    tags = compact_json(content["tags"])
    json_values = {field: compact_json(content[field]) for field in JSON_FIELDS}
    exact = " AND\n      ".join(exact_row_conditions(content))
    columns = [
        "project_id", "owner_address", "name_en", "name_cn", "slug",
        "tagline", "tagline_zh", "description", "tags", "team_info",
        "roadmap", "partners", "token_address", "token_symbol",
        "token_decimals", "project_token_policy_digest",
        "project_token_runtime_code_hash", "payment_token",
        "payment_token_address", "payment_token_decimals",
        "payment_token_policy_revision", "chain_id", "contract_address",
        "contract_version", "funding_mode", "onchain_locked", "fee_receiver",
        "success_fee_rate", "token_price", "token_price_raw", "hard_cap",
        "hard_cap_raw", "soft_cap", "soft_cap_raw", "max_per_user",
        "max_per_user_raw", "start_time", "end_time", "tge_time",
        "total_deposited", "total_deposited_raw", "sale_token_amount_raw",
        "min_contribution_raw", "max_contribution_raw", "total_raised",
        "total_raised_raw", "final_total_raised_raw",
        "final_sale_token_amount_raw", "participant_count", "token_withdrawn",
        "fund_withdrawn", "platform_fee_withdrawn", "status",
        "pre_archive_status", "category", "cover_url", "cover_video_url",
        "launch_at", "story_html", "story_html_zh", "research_html",
        "research_html_zh", "risks_html", "risks_html_zh", "team_json",
        "tokenomics_json", "roadmap_json", "documents_json", "faq_json",
        "onchain_json", "created_at", "updated_at",
    ]
    values = [
        str(PROJECT_ID), "''", text_expr(str(content["name_en"])),
        text_expr(str(content["name_cn"])), sql_quote(SLUG),
        text_expr(str(content["tagline"])), text_expr(str(content["tagline_zh"])),
        text_expr(str(content["description"])), text_expr(tags), "''", "''", "''",
        "''", "''", "18", "NULL", "NULL", "''", "''", "0", "NULL", "0",
        "''", "'unassigned'", "'capped_presale'", "0", "''", "0", "0", "'0'",
        "0", "'0'", "0", "'0'", "0", "'0'", "'1970-01-01 00:00:00'",
        "'1970-01-01 00:00:00'", "'1970-01-01 00:00:00'", "0", "'0'", "NULL",
        "NULL", "NULL", "0", "'0'", "NULL", "NULL", "0", "0", "0", "0",
        "'archived'", "'pre_launch'", sql_quote(str(content["category"])),
        text_expr(str(content["cover_url"])), text_expr(str(content["cover_video_url"])),
        "NULL", text_expr(str(content["story_html"])),
        text_expr(str(content["story_html_zh"])), text_expr(str(content["research_html"])),
        text_expr(str(content["research_html_zh"])), text_expr(str(content["risks_html"])),
        text_expr(str(content["risks_html_zh"])), text_expr(json_values["team_json"]),
        text_expr(json_values["tokenomics_json"]), text_expr(json_values["roadmap_json"]),
        text_expr(json_values["documents_json"]), text_expr(json_values["faq_json"]),
        "JSON_OBJECT()", "UTC_TIMESTAMP()", "UTC_TIMESTAMP()",
    ]
    if len(columns) != len(values):
        raise RuntimeError(f"column/value mismatch: {len(columns)} != {len(values)}")
    insert = (
        "INSERT INTO projects (\n  " + ",\n  ".join(columns) +
        "\n)\nSELECT\n  " + ",\n  ".join(values) +
        f"\nWHERE NOT EXISTS (SELECT 1 FROM projects WHERE project_id={PROJECT_ID});"
    )
    statements = [
        header("DaQi Inference production archived-draft creation"),
        "-- Required invocation pattern:\n"
        f"-- mysql ... --init-command=\"SET @jc_confirm_token='{CREATE_TOKEN}'\" < this-file.sql",
        assertion_table(),
        assertion("confirmation_token", f"@jc_confirm_token={sql_quote(CREATE_TOKEN)}"),
        assertion("database", f"DATABASE()={sql_quote(DATABASE)}"),
        assertion("server_uuid", f"@@server_uuid={sql_quote(SERVER_UUID)}"),
        assertion("mysql_version", f"VERSION()={sql_quote(MYSQL_VERSION)}"),
        "SELECT GET_LOCK('joincare:projects:create:daqi-inference', 10) INTO @jc_lock_acquired;",
        assertion("advisory_lock", "@jc_lock_acquired=1"),
        "START TRANSACTION;",
        "SELECT project_id FROM projects ORDER BY project_id DESC LIMIT 1 FOR UPDATE;",
        assertion(
            "identity_conflict",
            "(SELECT COUNT(*) FROM projects WHERE "
            f"(slug={sql_quote(SLUG)} OR name_en={text_expr(str(content['name_en']))} "
            f"OR name_cn={text_expr(str(content['name_cn']))}) AND project_id<>{PROJECT_ID})=0",
        ),
        assertion(
            "allocation",
            f"((SELECT COUNT(*) FROM projects WHERE project_id={PROJECT_ID})=1 OR "
            f"(SELECT COALESCE(MAX(project_id),0) FROM projects)={PROJECT_ID - 1})",
        ),
        assertion("no_auxiliary_rows_before", f"({auxiliary_count_expression()})=0"),
        insert,
        "SET @jc_inserted_rows = ROW_COUNT();",
        assertion("inserted_rows", "@jc_inserted_rows IN (0,1)"),
        assertion("exact_project_row", f"(SELECT COUNT(*) FROM projects p WHERE {exact})=1"),
        assertion("no_auxiliary_rows_after", f"({auxiliary_count_expression()})=0"),
        "COMMIT;",
        "SELECT RELEASE_LOCK('joincare:projects:create:daqi-inference') INTO @jc_lock_released;",
        assertion("advisory_unlock", "@jc_lock_released=1"),
        "SELECT CONCAT('create|PASS|project_id=', "
        f"{PROJECT_ID}, '|inserted=', @jc_inserted_rows, '|status=archived|restore_target=pre_launch');",
    ]
    return "\n".join(statements) + "\n"


def verify_sql(content: dict[str, object]) -> str:
    exact = " AND\n      ".join(exact_row_conditions(content))
    statements = [
        header("DaQi Inference production archived-draft verification (read-only)"),
        assertion_table(),
        assertion("database", f"DATABASE()={sql_quote(DATABASE)}"),
        assertion("server_uuid", f"@@server_uuid={sql_quote(SERVER_UUID)}"),
        assertion("mysql_version", f"VERSION()={sql_quote(MYSQL_VERSION)}"),
        assertion("exact_project_row", f"(SELECT COUNT(*) FROM projects p WHERE {exact})=1"),
        assertion("no_auxiliary_rows", f"({auxiliary_count_expression()})=0"),
        "SELECT id, project_id, name_en, name_cn, slug, status, pre_archive_status, "
        "contract_version, funding_mode, onchain_locked, chain_id, "
        "CHAR_LENGTH(owner_address) AS owner_len, CHAR_LENGTH(token_address) AS token_len, "
        "CHAR_LENGTH(contract_address) AS contract_len, "
        "SHA2(story_html,256) AS story_sha256, SHA2(story_html_zh,256) AS story_zh_sha256, "
        "SHA2(CAST(team_json AS CHAR),256) AS team_sha256, "
        "SHA2(CAST(tokenomics_json AS CHAR),256) AS tokenomics_sha256, "
        "created_at, updated_at FROM projects "
        f"WHERE project_id={PROJECT_ID};",
        "SELECT CONCAT('verify|PASS|project_id=', "
        f"{PROJECT_ID}, '|auxiliary_rows=0|public_state=archived');",
    ]
    return "\n".join(statements) + "\n"


def rollback_sql(content: dict[str, object]) -> str:
    exact = " AND\n      ".join(exact_row_conditions(content))
    statements = [
        header("DaQi Inference production archived-draft rollback"),
        "-- Required invocation pattern:\n"
        f"-- mysql ... --init-command=\"SET @jc_confirm_token='{ROLLBACK_TOKEN}'\" < this-file.sql",
        assertion_table(),
        assertion("confirmation_token", f"@jc_confirm_token={sql_quote(ROLLBACK_TOKEN)}"),
        assertion("database", f"DATABASE()={sql_quote(DATABASE)}"),
        assertion("server_uuid", f"@@server_uuid={sql_quote(SERVER_UUID)}"),
        assertion("mysql_version", f"VERSION()={sql_quote(MYSQL_VERSION)}"),
        "SELECT GET_LOCK('joincare:projects:create:daqi-inference', 10) INTO @jc_lock_acquired;",
        assertion("advisory_lock", "@jc_lock_acquired=1"),
        "START TRANSACTION;",
        f"SELECT project_id FROM projects WHERE project_id={PROJECT_ID} FOR UPDATE;",
        assertion(
            "row_absent_or_exact",
            f"((SELECT COUNT(*) FROM projects WHERE project_id={PROJECT_ID})=0 OR "
            f"(SELECT COUNT(*) FROM projects p WHERE {exact})=1)",
        ),
        assertion("no_auxiliary_rows", f"({auxiliary_count_expression()})=0"),
        f"DELETE FROM projects WHERE project_id={PROJECT_ID} AND status='archived' "
        "AND pre_archive_status='pre_launch' AND contract_version='unassigned' "
        "AND funding_mode='capped_presale' AND onchain_locked=0 "
        f"AND SHA2(story_html,256)={sql_quote(sha256(str(content['story_html']).encode('utf-8')))};",
        "SET @jc_deleted_rows = ROW_COUNT();",
        assertion("deleted_rows", "@jc_deleted_rows IN (0,1)"),
        assertion("project_removed", f"(SELECT COUNT(*) FROM projects WHERE project_id={PROJECT_ID})=0"),
        "COMMIT;",
        "SELECT RELEASE_LOCK('joincare:projects:create:daqi-inference') INTO @jc_lock_released;",
        assertion("advisory_unlock", "@jc_lock_released=1"),
        "SELECT CONCAT('rollback|PASS|project_id=', "
        f"{PROJECT_ID}, '|deleted=', @jc_deleted_rows);",
    ]
    return "\n".join(statements) + "\n"


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--output-dir", type=Path, required=True)
    args = parser.parse_args()

    raw = args.source.read_bytes()
    if sha256(raw) != SOURCE_SHA256:
        raise SystemExit("content source SHA-256 mismatch")
    content = json.loads(raw.decode("utf-8"))
    if "\ufffd" in raw.decode("utf-8"):
        raise SystemExit("content contains Unicode replacement characters")
    if content.get("_status") != "V2_DRAFT_REQUIRES_PROJECT_MEDICAL_LEGAL_DUE_DILIGENCE_REVIEW":
        raise SystemExit("content is not the reviewed V2 draft source")
    if content.get("launch_at") is not None:
        raise SystemExit("launch_at must remain null")
    if content.get("cover_url") or content.get("cover_video_url"):
        raise SystemExit("unreviewed media URLs must remain empty")
    if content.get("tokenomics_json", {}).get("allocations") != []:
        raise SystemExit("tokenomics allocations must remain empty")
    if "onchain_json" in content:
        raise SystemExit("source must not contain onchain_json")
    for field in JSON_FIELDS:
        compact_json(content[field])

    args.output_dir.mkdir(parents=True, exist_ok=True)
    files = {
        "preflight.sql": preflight_sql(content),
        "create-daqi-project.sql": create_sql(content),
        "verify-daqi-project.sql": verify_sql(content),
        "rollback-daqi-project.sql": rollback_sql(content),
    }
    for name, body in files.items():
        (args.output_dir / name).write_text(body, encoding="utf-8", newline="\n")
    sums = "".join(
        f"{sha256(body.encode('utf-8'))}  {name}\n" for name, body in sorted(files.items())
    )
    (args.output_dir / "SHA256SUMS.txt").write_text(sums, encoding="ascii", newline="\n")
    receipt = {
        "database": DATABASE,
        "server_uuid": SERVER_UUID,
        "mysql_version": MYSQL_VERSION,
        "project_id": PROJECT_ID,
        "slug": SLUG,
        "status": "archived",
        "pre_archive_status": "pre_launch",
        "backup_sha256": BACKUP_SHA256,
        "source_sha256": SOURCE_SHA256,
        "create_confirmation_token": CREATE_TOKEN,
        "rollback_confirmation_token": ROLLBACK_TOKEN,
        "production_execution": "BLOCKED_UNTIL_ARCHIVED_PUBLIC_DETAIL_GUARD_IS_DEPLOYED",
    }
    (args.output_dir / "receipt.json").write_text(
        json.dumps(receipt, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
        newline="\n",
    )


if __name__ == "__main__":
    main()

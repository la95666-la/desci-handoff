#!/usr/bin/env python3
"""Create the immutable Fair Launch LOCAL-ONLY Backend bootstrap.

This command is intentionally outside every production migration/release path.
It consumes only the accepted disposable Deploy run and the separately admitted
Contract manifest, verifies both again, and performs one create-once MySQL
transaction. Password material is read from the run-owned 0600 root secret and
is never placed in argv, environment, evidence, or output.
"""

from __future__ import annotations

import base64
import datetime as dt
import hashlib
import json
import os
import pathlib
import re
import stat
import sys
import urllib.request
from decimal import Decimal

import pymysql
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey


AUTHORITY = "TEST_ONLY_NOT_PRODUCTION_ADMITTED"
RUN_PATTERN = re.compile(r"^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$")
HEX64 = re.compile(r"^[0-9a-f]{64}$")
HASH = re.compile(r"^0x[0-9a-f]{64}$")
ADDRESS = re.compile(r"^0x[0-9a-f]{40}$")
COLLECTOR = "joincare-fair-launch-local-e2e-bootstrap-v1"
ZERO_HASH = "0x" + "0" * 64


class Refusal(RuntimeError):
    pass


def refuse(message: str) -> None:
    raise Refusal(message)


def object_no_duplicates(pairs):
    out = {}
    for key, value in pairs:
        if key in out:
            refuse("JSON contains duplicate object keys")
        out[key] = value
    return out


def strict_json_bytes(raw: bytes, label: str):
    try:
        text = raw.decode("utf-8")
        value = json.loads(text, object_pairs_hook=object_no_duplicates)
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        refuse(f"{label} is not strict UTF-8 JSON: {exc}")
    if not isinstance(value, dict):
        refuse(f"{label} root is not an object")
    return value


def canonical(value) -> bytes:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")


def sha_bytes(raw: bytes) -> str:
    return hashlib.sha256(raw).hexdigest()


def sha_file(path: pathlib.Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(65536), b""):
            h.update(chunk)
    return h.hexdigest()


def owned_file(path: pathlib.Path, mode: int = 0o600) -> bytes:
    if not path.is_absolute() or path != pathlib.Path(os.path.realpath(path)):
        refuse(f"non-canonical authority path: {path}")
    before = path.lstat()
    if not stat.S_ISREG(before.st_mode) or stat.S_ISLNK(before.st_mode) or before.st_nlink != 1:
        refuse(f"authority file identity invalid: {path.name}")
    if before.st_uid != os.getuid() or stat.S_IMODE(before.st_mode) != mode:
        refuse(f"authority file ownership/mode invalid: {path.name}")
    raw = path.read_bytes()
    after = path.lstat()
    identity = lambda row: (row.st_dev, row.st_ino, row.st_uid, row.st_mode, row.st_nlink, row.st_size, row.st_mtime_ns, row.st_ctime_ns)
    if identity(before) != identity(after):
        refuse(f"authority file changed while read: {path.name}")
    return raw


def ordinary_file(path: pathlib.Path) -> bytes:
    if not path.is_absolute() or path != pathlib.Path(os.path.realpath(path)):
        refuse(f"non-canonical Contract path: {path}")
    before = path.lstat()
    if not stat.S_ISREG(before.st_mode) or stat.S_ISLNK(before.st_mode) or before.st_nlink != 1:
        refuse(f"Contract file identity invalid: {path.name}")
    raw = path.read_bytes()
    after = path.lstat()
    if (before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns, before.st_ctime_ns) != (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns, after.st_ctime_ns):
        refuse(f"Contract file changed while read: {path.name}")
    return raw


def windows_to_wsl(value: str) -> pathlib.Path:
    if not isinstance(value, str) or not re.fullmatch(r"[A-Za-z]:\\[^\x00]+", value):
        refuse("Contract authority path is not an absolute Windows drive path")
    if ".." in value.replace("/", "\\").split("\\") or value.startswith(("\\\\", "\\?\\", "\\.\\")):
        refuse("Contract authority path syntax rejected")
    drive = value[0].lower()
    rest = value[3:].replace("\\", "/")
    return pathlib.Path(f"/mnt/{drive}/{rest}")


def exact_keys(value, keys, label: str) -> None:
    if not isinstance(value, dict) or set(value) != set(keys):
        refuse(f"{label} fields mismatch")


def parse_time(value: str, label: str) -> dt.datetime:
    try:
        parsed = dt.datetime.fromisoformat(value.replace("Z", "+00:00"))
    except (TypeError, ValueError):
        refuse(f"{label} timestamp invalid")
    if parsed.tzinfo is None:
        refuse(f"{label} timestamp lacks timezone")
    return parsed.astimezone(dt.timezone.utc)


def rpc(url: str, method: str, params):
    body = canonical({"id": 1, "jsonrpc": "2.0", "method": method, "params": params})
    request = urllib.request.Request(url, data=body, headers={"content-type": "application/json"}, method="POST")
    try:
        with urllib.request.urlopen(request, timeout=3) as response:
            value = strict_json_bytes(response.read(), f"RPC {method}")
    except Exception as exc:
        refuse(f"RPC {method} unavailable: {type(exc).__name__}")
    if value.get("id") != 1 or value.get("error") is not None or "result" not in value:
        refuse(f"RPC {method} failed")
    return value["result"]


def parse_run_state(path: pathlib.Path, run_id: str, run_dir: pathlib.Path) -> dict[str, str]:
    raw = owned_file(path).decode("utf-8")
    out = {}
    line_pattern = re.compile(r"^([A-Z][A-Z0-9_]*)='([^'\r\n]*)'$|^(__JC_FL_STATE_VERIFIED__)=yes$")
    for line in raw.splitlines():
        match = line_pattern.fullmatch(line)
        if not match:
            refuse("state.env is not inert canonical data")
        key = match.group(1) or match.group(3)
        if key in out:
            refuse("state.env contains duplicate keys")
        out[key] = match.group(2) if match.group(1) else "yes"
    required = {
        "RUN_ID", "RUN_DIR", "MYSQL_CONTAINER_ID", "MYSQL_ENDPOINT", "MYSQL_SERVER_UUID", "MYSQL_VERSION", "DB_NAME", "REDIS_PORT", "RPC_PORT",
        "MYSQL_IMAGE_ID", "MYSQL_TRANSPORT_DIGEST", "BASELINE_DIGEST", "MIGRATION_MANIFEST_DIGEST", "SCHEMA_DIGEST", "GRANT_POLICY_DIGEST", "OWNER_LABEL_DIGEST", "MYSQL_SCHEMA_READY",
    }
    if not required.issubset(out) or out["RUN_ID"] != run_id or out["RUN_DIR"] != str(run_dir) or out["MYSQL_SCHEMA_READY"] != "1":
        refuse("state.env run/schema binding mismatch")
    if not re.fullmatch(r"127[.]0[.]0[.]1:[1-9][0-9]{3,4}", out["MYSQL_ENDPOINT"]) or out["RPC_PORT"] == out["REDIS_PORT"]:
        refuse("state.env endpoint identity invalid")
    return out


def verify_contract_authority(run_id: str, state: dict[str, str], authority_path: pathlib.Path):
    authority_raw = owned_file(authority_path)
    authority = strict_json_bytes(authority_raw, "Contract runtime admission")
    exact_keys(authority, ["authority", "chainId", "contractStateRoot", "createdAt", "genesisHash", "manifestPath", "manifestSha256", "payloadSha256", "projectId", "rpcUrl", "runId", "schemaVersion", "trustPinPath", "trustPinSha256"], "Contract runtime admission")
    rpc_url = f"http://127.0.0.1:{state['RPC_PORT']}"
    if authority["schemaVersion"] != "fair-launch-local-e2e-contract-runtime-admission-v1" or authority["authority"] != AUTHORITY or authority["runId"] != run_id or authority["chainId"] != 31337 or authority["rpcUrl"] != rpc_url or authority["projectId"] != "102":
        refuse("Contract runtime admission identity mismatch")
    if not HASH.fullmatch(authority["genesisHash"] or "") or not HEX64.fullmatch(authority["manifestSha256"] or "") or not HEX64.fullmatch(authority["trustPinSha256"] or "") or not HEX64.fullmatch(authority["payloadSha256"] or ""):
        refuse("Contract runtime admission digest shape invalid")

    manifest_path = windows_to_wsl(authority["manifestPath"])
    pin_path = windows_to_wsl(authority["trustPinPath"])
    contract_root = windows_to_wsl(authority["contractStateRoot"])
    expected_run_dir = contract_root / run_id
    if manifest_path.parent != expected_run_dir or manifest_path.name != "deployment-manifest.json":
        refuse("manifest path is outside the admitted Contract run")
    trust_path = expected_run_dir / "public-trust-root.json"
    manifest_raw = ordinary_file(manifest_path)
    pin_raw = ordinary_file(pin_path)
    trust_raw = ordinary_file(trust_path)
    if sha_bytes(manifest_raw) != authority["manifestSha256"] or sha_bytes(pin_raw) != authority["trustPinSha256"]:
        refuse("Contract manifest/pin exact-byte digest mismatch")
    envelope = strict_json_bytes(manifest_raw, "Contract manifest")
    pin = strict_json_bytes(pin_raw, "Contract trust pin")
    trust = strict_json_bytes(trust_raw, "Contract public trust root")
    if manifest_raw != canonical(envelope) + b"\n" or pin_raw != canonical(pin) + b"\n":
        refuse("Contract manifest or pin is not canonical JSON")
    exact_keys(envelope, ["payload", "signature", "testTrustRoot"], "manifest envelope")
    exact_keys(envelope["signature"], ["algorithm", "keyId", "payloadSha256", "value"], "manifest signature")
    exact_keys(envelope["testTrustRoot"], ["authority", "keyId", "publicKeyJwk"], "manifest trust projection")
    exact_keys(trust, ["schemaVersion", "authority", "runId", "rpcUrl", "chainId", "genesisHash", "rootAuthoritySha256", "processAuthoritySha256", "keyId", "publicKeyJwk", "createdAt"], "public trust root")
    exact_keys(pin, ["schemaVersion", "authority", "runId", "rpcUrl", "chainId", "genesisHash", "keyId", "trustRecordSha256", "publicKeySha256", "trustCreatedAt", "createdAt"], "external pin")
    for value in (trust, pin):
        if value["authority"] != AUTHORITY or value["runId"] != run_id or value["rpcUrl"] != rpc_url or value["chainId"] != 31337 or value["genesisHash"] != authority["genesisHash"]:
            refuse("Contract trust authority cross-binding mismatch")
    jwk = trust["publicKeyJwk"]
    exact_keys(jwk, ["crv", "kty", "x"], "Ed25519 public JWK")
    if jwk["crv"] != "Ed25519" or jwk["kty"] != "OKP" or envelope["testTrustRoot"] != {"authority": AUTHORITY, "keyId": trust["keyId"], "publicKeyJwk": jwk}:
        refuse("manifest embedded trust mismatch")
    if pin["keyId"] != trust["keyId"] or pin["trustRecordSha256"] != sha_bytes(canonical(trust)) or pin["publicKeySha256"] != sha_bytes(canonical(jwk)) or pin["trustCreatedAt"] != trust["createdAt"]:
        refuse("external trust pin mismatch")
    payload_bytes = canonical(envelope["payload"])
    signature = envelope["signature"]
    if signature["algorithm"] != "ed25519" or signature["keyId"] != trust["keyId"] or signature["payloadSha256"] != sha_bytes(payload_bytes) or signature["payloadSha256"] != authority["payloadSha256"]:
        refuse("manifest signature metadata mismatch")
    try:
        public_raw = base64.urlsafe_b64decode(jwk["x"] + "=" * (-len(jwk["x"]) % 4))
        signature_raw = base64.b64decode(signature["value"], validate=True)
        Ed25519PublicKey.from_public_bytes(public_raw).verify(signature_raw, payload_bytes)
    except Exception:
        refuse("manifest Ed25519 verification failed")

    payload = envelope["payload"]
    exact_keys(payload, ["schemaVersion", "authority", "runId", "createdAt", "expiresAt", "network", "contracts", "assets", "safes", "projects", "roles", "deployment", "source"], "manifest payload")
    if payload["schemaVersion"] != "fair-launch-local-e2e-deployment-v1" or payload["authority"] != AUTHORITY or payload["runId"] != run_id:
        refuse("manifest payload identity mismatch")
    network = payload["network"]
    if network != {"name": "hardhat-local-persistent", "chainId": 31337, "rpcUrl": rpc_url, "genesisHash": authority["genesisHash"]}:
        refuse("manifest network mismatch")
    now = dt.datetime.now(dt.timezone.utc)
    created = parse_time(payload["createdAt"], "manifest createdAt")
    expires = parse_time(payload["expiresAt"], "manifest expiresAt")
    if not (created <= now < expires) or expires - created > dt.timedelta(hours=24):
        refuse("manifest validity interval is not currently admitted")

    latest = rpc(rpc_url, "eth_getBlockByNumber", ["latest", False])
    genesis = rpc(rpc_url, "eth_getBlockByNumber", ["0x0", False])
    observation = payload["deployment"]["observation"]
    latest_number = int(latest["number"], 16)
    if genesis["hash"].lower() != authority["genesisHash"] or latest_number != int(observation["blockNumber"]) or latest["hash"].lower() != observation["blockHash"] or int(latest["timestamp"], 16) != int(observation["blockTimestamp"]):
        refuse("live Hardhat chain is not at the signed pristine observation tip")
    if rpc(rpc_url, "eth_chainId", []) != "0x7a69":
        refuse("live Hardhat chain ID mismatch")

    required_contracts = {"proxy", "dispatcher", "coreFacet", "releaseFacet", "fundsFacet", "adminFacet", "viewsFacet", "logicLibrary", "runtimeLibrary"}
    if set(payload["contracts"]) != required_contracts:
        refuse("manifest Contract set mismatch")
    code_rows = list(payload["contracts"].values()) + [payload["assets"]["paymentToken"]] + payload["assets"]["projectTokens"] + [payload["safes"]["proceeds"], payload["safes"]["platformFee"]]
    for row in code_rows:
        if not ADDRESS.fullmatch(row.get("address", "")) or not HASH.fullmatch(row.get("runtimeCodeHash", "")):
            refuse("manifest address/code identity invalid")
        code = rpc(rpc_url, "eth_getCode", [row["address"], "latest"])
        # Ethereum runtime identities are Keccak-256. Python's sha3_256 is not
        # Keccak, so compare by asking the already-admitted manifest only after
        # independently proving non-empty code and the exact signed pristine tip.
        if code == "0x":
            refuse("manifest Contract/token code is absent")

    expected_projects = {"failed": "101", "successful": "102", "cancelled": "103"}
    projects = {row["key"]: row for row in payload["projects"]}
    tokens = {row["key"]: row for row in payload["assets"]["projectTokens"]}
    if set(projects) != set(expected_projects) or set(tokens) != set(expected_projects):
        refuse("manifest project/token collection mismatch")
    for key, project_id in expected_projects.items():
        row = projects[key]
        if row["projectId"] != project_id or row["projectToken"] != tokens[key]["address"] or row["paymentToken"] != payload["assets"]["paymentToken"]["address"] or row["creation"]["to"] != payload["contracts"]["proxy"]["address"]:
            refuse("manifest project binding mismatch")
        if row["pristineStorage"]["projectId"] != project_id or row["pristineStorage"]["totalTokensDeposited"] != row["totalTokensDepositedRaw"]:
            refuse("manifest pristine project storage mismatch")
    return authority, authority_raw, payload, manifest_raw, pin_raw, trust, expires


def parse_root_password(path: pathlib.Path) -> str:
    raw = owned_file(path)
    try:
        password = raw.decode("ascii").strip()
    except UnicodeDecodeError:
        refuse("root credential is not ASCII")
    if not password or "\x00" in password or "\n" in password:
        refuse("root credential shape invalid")
    return password


def decimal_display(raw: str, decimals: int) -> Decimal:
    if not re.fullmatch(r"0|[1-9][0-9]{0,77}", raw):
        refuse("manifest raw amount invalid")
    return Decimal(raw) / (Decimal(10) ** decimals)


def row_digest(domain: str, value) -> str:
    return sha_bytes(domain.encode("ascii") + b"\x00" + canonical(value))


def execute_bootstrap(run_id: str, run_dir: pathlib.Path, state: dict[str, str], authority_path: pathlib.Path):
    authority, authority_raw, payload, manifest_raw, _pin_raw, _trust, expires = verify_contract_authority(run_id, state, authority_path)
    database_evidence_path = run_dir / "evidence" / "database-schema.json"
    readiness_path = run_dir / "evidence" / "schema-readiness.json"
    infrastructure_path = run_dir / "evidence" / "infrastructure.json"
    container_path = run_dir / "evidence" / "docker-created.source.json"
    database_raw = owned_file(database_evidence_path)
    readiness_raw = owned_file(readiness_path)
    infrastructure_raw = owned_file(infrastructure_path)
    container_raw = owned_file(container_path)
    database_evidence = strict_json_bytes(database_raw, "database schema evidence")
    readiness = strict_json_bytes(readiness_raw, "schema readiness")
    infrastructure = strict_json_bytes(infrastructure_raw, "infrastructure evidence")
    container = strict_json_bytes(container_raw, "container authority")
    if database_evidence.get("runId") != run_id or database_evidence.get("schemaDigest") != state["SCHEMA_DIGEST"] or readiness.get("runId") != run_id or readiness.get("schemaDigest") != state["SCHEMA_DIGEST"] or infrastructure.get("runId") != run_id or container.get("runId") != run_id:
        refuse("Deploy evidence cross-binding mismatch")
    for key in ("BASELINE_DIGEST", "MIGRATION_MANIFEST_DIGEST", "SCHEMA_DIGEST", "GRANT_POLICY_DIGEST", "OWNER_LABEL_DIGEST", "MYSQL_TRANSPORT_DIGEST"):
        if not HEX64.fullmatch(state[key]):
            refuse(f"Deploy state digest invalid: {key}")
    actual_grants = {}
    for role in ("runtime", "admin", "job", "ingest"):
        actual_path = run_dir / "grants" / f"actual-{role}"
        expected_path = run_dir / "grants" / f"expected-{role}"
        actual_raw = ordinary_file(actual_path)
        expected_raw = ordinary_file(expected_path)
        if actual_raw != expected_raw:
            refuse(f"Deploy {role} grant authority mismatch")
        actual_grants[role] = sha_bytes(actual_raw)

    password = parse_root_password(run_dir / "secrets" / "mysql-root")
    port = int(state["MYSQL_ENDPOINT"].rsplit(":", 1)[1])
    connection = pymysql.connect(host="127.0.0.1", port=port, user="root", password=password, database=state["DB_NAME"], charset="utf8mb4", autocommit=False, connect_timeout=5, read_timeout=20, write_timeout=20)
    password = ""
    now = dt.datetime.now(dt.timezone.utc).replace(tzinfo=None)
    expires_db = expires.replace(tzinfo=None)
    if expires_db <= now:
        refuse("Contract admission expired before bootstrap")
    proxy = payload["contracts"]["proxy"]
    dispatcher = payload["contracts"]["dispatcher"]
    observation = payload["deployment"]["observation"]
    proxy_receipt = payload["deployment"]["proxy"]
    source_plan = {
        "chainId": 31337,
        "contractAddress": proxy["address"],
        "contractVersion": "fair_launch_v1",
        "fundingMode": "fair_launch",
        "implementationAddress": dispatcher["address"],
        "implementationCodeHash": dispatcher["runtimeCodeHash"][2:],
        "abiDigest": payload["source"]["aggregateAbiDigest"][2:],
        "dependencyManifestDigest": payload["source"]["fullSetDigest"][2:],
        "fromBlock": int(proxy_receipt["blockNumber"]),
        "fromTransactionIndex": proxy_receipt["transactionIndex"],
        "fromTxLogOrdinalInclusive": 0,
    }
    try:
        with connection.cursor() as cursor:
            cursor.execute("SET SESSION time_zone='+00:00'")
            cursor.execute("SELECT CURRENT_USER(),DATABASE(),LOWER(@@server_uuid),VERSION(),@@FOREIGN_KEY_CHECKS")
            identity = cursor.fetchone()
            if identity != ("root@%", state["DB_NAME"], state["MYSQL_SERVER_UUID"], state["MYSQL_VERSION"], 1):
                refuse("bootstrap MySQL root/server identity mismatch")
            cursor.execute("SELECT COUNT(*) FROM fair_launch_local_e2e_run WHERE run_id=%s", (run_id,))
            if cursor.fetchone()[0] != 0:
                refuse("bootstrap is create-once and this run already exists")
            cursor.execute("SELECT COUNT(*) FROM chain_contract_source WHERE chain_id=31337 OR contract_address=%s", (proxy["address"],))
            if cursor.fetchone()[0] != 0:
                refuse("bootstrap source namespace is not empty")
            cursor.execute("SELECT COUNT(*) FROM projects WHERE project_id IN (101,102,103)")
            if cursor.fetchone()[0] != 0:
                refuse("bootstrap project namespace is not empty")
            cursor.execute("""SELECT COUNT(*) FROM projects
                              WHERE project_id=2
                                AND CHAR_LENGTH(name_en)>0
                                AND CHAR_LENGTH(description)>0
                                AND CHAR_LENGTH(story_html)>0
                                AND CHAR_LENGTH(research_html)>0
                                AND CHAR_LENGTH(risks_html)>0
                                AND JSON_LENGTH(team_json)>0
                                AND JSON_LENGTH(JSON_EXTRACT(tokenomics_json, '$.allocations'))>0
                                AND JSON_LENGTH(roadmap_json)>0
                                AND JSON_LENGTH(documents_json)>0
                                AND JSON_LENGTH(faq_json)>0""")
            if cursor.fetchone()[0] != 1:
                refuse("complete reference project 2 content is unavailable")
            for table in ("fair_launch_application_lineage_v3", "fair_launch_effect_guard_v3", "fair_launch_effect_journal_v3", "fair_launch_effect_attestation_v3", "fair_launch_ledger_entry_v3", "fair_launch_project_state_v3", "fair_launch_position_v3"):
                cursor.execute(f"SELECT COUNT(*) FROM `{table}`")
                if cursor.fetchone()[0] != 0:
                    refuse(f"bootstrap requires empty disposable {table}")

            cursor.execute("INSERT INTO chain_contract_source(chain_id,contract_address,contract_version,funding_mode,registered_at,retired_at,start_block,enabled) VALUES(31337,%s,'fair_launch_v1','fair_launch',%s,NULL,%s,1)", (proxy["address"], now, source_plan["fromBlock"]))
            source_id = cursor.lastrowid
            cursor.execute("INSERT INTO chain_contract_source_revision(source_id,revision_no,activation_block_hash,proxy_kind,abi_digest,confirmation_policy,governance_tx_hash,governance_operation_id,implementation_address,implementation_code_hash,dependency_manifest_digest,abi_version,from_block_number,from_transaction_index,from_tx_log_ordinal_inclusive,to_block_number,to_transaction_index,to_tx_log_ordinal_exclusive,activation_event_identity_id,predecessor_revision_id,created_at) VALUES(%s,1,%s,'local_dispatcher',%s,'signed_pristine_tip',%s,%s,%s,%s,%s,'fair-launch-v1',%s,%s,0,NULL,NULL,NULL,NULL,NULL,%s)", (source_id, proxy_receipt["blockHash"], source_plan["abiDigest"], proxy_receipt["txHash"], observation["txHash"], source_plan["implementationAddress"], source_plan["implementationCodeHash"], source_plan["dependencyManifestDigest"], source_plan["fromBlock"], source_plan["fromTransactionIndex"], now))
            revision_id = cursor.lastrowid
            registry_projection = {**source_plan, "sourceId": source_id, "sourceRevisionId": revision_id, "revisionNo": 1}
            source_registry_digest = row_digest("JC_FL_SOURCE_REGISTRY_BOOTSTRAP_V1", registry_projection)
            runtime_admission_digest = sha_bytes(authority_raw)
            cursor.execute("INSERT INTO fair_launch_local_e2e_run(run_id,environment,exact_rpc_url,redis_endpoint,chain_id,database_name,mysql_server_uuid,mysql_version,container_id,owner_label_digest,schema_generation,baseline_digest,migration_manifest_digest,schema_digest,grant_digest,source_registry_digest,hardhat_genesis_hash,chain_manifest_digest,deployment_manifest_digest,runtime_admission_digest,created_at,expires_at,closed_at,state) VALUES(%s,'local-e2e',%s,%s,31337,%s,%s,%s,%s,%s,12,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,NULL,'verified')", (run_id, authority["rpcUrl"], f"127.0.0.1:{state['REDIS_PORT']}", state["DB_NAME"], state["MYSQL_SERVER_UUID"], state["MYSQL_VERSION"], state["MYSQL_CONTAINER_ID"], state["OWNER_LABEL_DIGEST"], state["BASELINE_DIGEST"], state["MIGRATION_MANIFEST_DIGEST"], state["SCHEMA_DIGEST"], state["GRANT_POLICY_DIGEST"], source_registry_digest, authority["genesisHash"], authority["payloadSha256"], authority["manifestSha256"], runtime_admission_digest, now, expires_db))
            run_row_id = cursor.lastrowid
            source_binding_projection = {"runRowId": run_row_id, "sourceId": source_id, "sourceRevisionId": revision_id, **source_plan}
            source_binding_digest = row_digest("JC_FL_SOURCE_REVISION_BINDING_V1", source_binding_projection)
            cursor.execute("INSERT INTO fair_launch_source_revision_binding_v3(run_row_id,chain_id,source_id,source_revision_id,from_block_number,from_transaction_index,from_tx_log_ordinal_inclusive,to_block_number,to_transaction_index,to_tx_log_ordinal_exclusive,source_binding_digest,created_at) VALUES(%s,31337,%s,%s,%s,%s,0,NULL,NULL,NULL,%s,%s)", (run_row_id, source_id, revision_id, source_plan["fromBlock"], source_plan["fromTransactionIndex"], source_binding_digest, now))
            source_binding_id = cursor.lastrowid

            payment = payload["assets"]["paymentToken"]
            project_tokens = {row["key"]: row for row in payload["assets"]["projectTokens"]}
            project_rows = {row["key"]: row for row in payload["projects"]}
            for token in [payment, *payload["assets"]["projectTokens"]]:
                metadata = {"runRowId": run_row_id, "sourceId": source_id, "contractAddress": proxy["address"], "tokenAddress": token["address"], "tokenDecimals": token["decimals"], "runtimeCodeHash": token["runtimeCodeHash"][2:], "symbol": token["symbol"], "metadataVersion": 1, "admissionGeneration": 1}
                metadata_digest = row_digest("JC_FL_SOURCE_TOKEN_BINDING_V1", metadata)
                cursor.execute("INSERT INTO fair_launch_source_token_binding_v3(run_row_id,chain_id,source_id,contract_address,token_address,token_decimals,metadata_version,admission_generation,runtime_code_hash,metadata_digest,admitted_at) VALUES(%s,31337,%s,%s,%s,%s,1,1,%s,%s,%s)", (run_row_id, source_id, proxy["address"], token["address"], token["decimals"], token["runtimeCodeHash"][2:], metadata_digest, now))

            for key in ("failed", "successful", "cancelled"):
                project = project_rows[key]
                storage = project["pristineStorage"]
                token = project_tokens[key]
                status = project["initialStatus"]
                start = dt.datetime.fromtimestamp(int(project["startTime"]), dt.timezone.utc).replace(tzinfo=None)
                end = dt.datetime.fromtimestamp(int(project["endTime"]), dt.timezone.utc).replace(tzinfo=None)
                soft_display = decimal_display(project["softCapRaw"], payment["decimals"])
                max_display = decimal_display(project["maxContributionRaw"], payment["decimals"])
                deposited_display = decimal_display(project["totalTokensDepositedRaw"], token["decimals"])
                name = {"failed": "LOCAL Fair Launch Refund", "successful": "LOCAL Fair Launch Success", "cancelled": "LOCAL Fair Launch Cancelled"}[key]
                cursor.execute("""INSERT INTO projects(project_id,owner_address,name_en,name_cn,slug,tagline,tagline_zh,description,tags,team_info,roadmap,partners,token_address,token_symbol,token_decimals,project_token_policy_digest,project_token_runtime_code_hash,payment_token,payment_token_address,payment_token_decimals,payment_token_policy_revision,chain_id,contract_address,contract_version,funding_mode,onchain_locked,fee_receiver,success_fee_rate,token_price,token_price_raw,hard_cap,hard_cap_raw,soft_cap,soft_cap_raw,max_per_user,max_per_user_raw,start_time,end_time,tge_time,total_deposited,total_deposited_raw,sale_token_amount_raw,min_contribution_raw,max_contribution_raw,total_raised,total_raised_raw,final_total_raised_raw,final_sale_token_amount_raw,participant_count,token_withdrawn,fund_withdrawn,platform_fee_withdrawn,status,pre_archive_status,category,cover_url,cover_video_url,story_html,story_html_zh,research_html,research_html_zh,risks_html,risks_html_zh,team_json,tokenomics_json,roadmap_json,documents_json,faq_json,onchain_json,created_at,updated_at) VALUES(%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,31337,%s,'fair_launch_v1','fair_launch',1,%s,%s,0,'0',0,'0',%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,0,'0',NULL,NULL,0,0,0,0,%s,'','local-e2e','','','','','','','','','[]','{}','[]','[]','[]',%s,%s,%s)""", (int(project["projectId"]), project["projectOwner"], name, name, f"local-fair-launch-{key}", AUTHORITY, AUTHORITY, AUTHORITY, "[]", AUTHORITY, AUTHORITY, "[]", token["address"], token["symbol"], token["decimals"], storage["projectTokenPolicyDigest"][2:], token["runtimeCodeHash"][2:], payment["symbol"], payment["address"], payment["decimals"], int(storage["paymentTokenPolicyVersion"]), proxy["address"], project["platformFeeReceiver"], project["platformFeeBps"], soft_display, project["softCapRaw"], max_display, project["maxContributionRaw"], start, end, end, deposited_display, project["totalTokensDepositedRaw"], project["saleTokenAmountRaw"], project["minContributionRaw"], project["maxContributionRaw"], status, canonical({"authority": AUTHORITY, "key": key, "runId": run_id}).decode(), now, now))
                project_row_id = cursor.lastrowid
                # Reuse the complete, migration-owned project-2 presentation fixture
                # so browser E2E exercises the real rich-content layout. Only
                # marketing/content columns are copied: run-bound ownership, token,
                # contract, lifecycle and financial values remain those admitted
                # from this disposable Hardhat fixture.
                cursor.execute("""UPDATE projects AS target
                                  JOIN projects AS reference ON reference.project_id=2
                                  SET target.name_en=reference.name_en,
                                      target.name_cn=reference.name_cn,
                                      target.tagline=reference.tagline,
                                      target.tagline_zh=reference.tagline_zh,
                                      target.description=reference.description,
                                      target.tags=reference.tags,
                                      target.team_info=reference.team_info,
                                      target.roadmap=reference.roadmap,
                                      target.partners=reference.partners,
                                      target.category=reference.category,
                                      target.cover_url=COALESCE(NULLIF(reference.cover_url, ''), '/figma/v2/project-card-image-3.png'),
                                      target.cover_video_url=reference.cover_video_url,
                                      target.story_html=reference.story_html,
                                      target.story_html_zh=reference.story_html_zh,
                                      target.research_html=reference.research_html,
                                      target.research_html_zh=reference.research_html_zh,
                                      target.risks_html=reference.risks_html,
                                      target.risks_html_zh=reference.risks_html_zh,
                                      target.team_json=reference.team_json,
                                      target.tokenomics_json=reference.tokenomics_json,
                                      target.roadmap_json=reference.roadmap_json,
                                      target.documents_json=reference.documents_json,
                                      target.faq_json=reference.faq_json,
                                      target.onchain_json=reference.onchain_json
                                  WHERE target.id=%s""", (project_row_id,))
                if cursor.rowcount != 1:
                    refuse("reference project content copy failed")
                binding = {"runRowId": run_row_id, "projectRowId": project_row_id, "chainId": 31337, "contractAddress": proxy["address"], "contractVersion": "fair_launch_v1", "projectId": int(project["projectId"]), "sourceBindingId": source_binding_id, "sourceId": source_id, "sourceRevisionId": revision_id, "projectTokenAddress": token["address"], "projectTokenDecimals": token["decimals"], "paymentTokenAddress": payment["address"], "paymentTokenDecimals": payment["decimals"]}
                binding_digest = row_digest("JC_FL_PROJECT_BINDING_V1", binding)
                cursor.execute("INSERT INTO fair_launch_project_binding_v3(run_row_id,project_row_id,chain_id,contract_address,contract_version,project_id,project_id_raw,source_binding_id,source_id,source_revision_id,project_token_address,project_token_decimals,payment_token_address,payment_token_decimals,binding_digest,created_at) VALUES(%s,%s,31337,%s,'fair_launch_v1',%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s)", (run_row_id, project_row_id, proxy["address"], int(project["projectId"]), project["projectId"], source_binding_id, source_id, revision_id, token["address"], token["decimals"], payment["address"], payment["decimals"], binding_digest, now))
                project_binding_id = cursor.lastrowid
                # This payload and domain must remain byte-for-byte aligned with
                # fairLaunchProjectStatePayload/fairLaunchV3Payload in the
                # dedicated LOCAL-ONLY Backend. The writer verifies this digest
                # before every forward mutation and replay; bootstrap must not
                # create a weaker or differently-scoped baseline digest.
                state_projection = {"projectId": int(project["projectId"]), "status": {"0": "Pending", "1": "Active", "2": "SuccessfulPendingRelease", "3": "Successful", "4": "Failed", "5": "Cancelled"}[storage["status"]], "terminalReason": "cancelled" if storage["terminalReason"] == "2" else "none", "saleTokenAmountRaw": storage["saleTokenAmount"], "totalRaisedRaw": storage["totalRaised"], "totalTokensDepositedRaw": storage["totalTokensDeposited"], "totalTokensClaimedRaw": storage["totalTokensClaimed"], "participantCountRaw": storage["participantCount"], "finalTotalRaisedRaw": None, "finalPriceNumeratorRaw": None, "finalPriceDenominatorRaw": None, "platformFeeReservedRaw": "0", "projectProceedsPaidRaw": "0", "platformFeePaidRaw": "0", "projectProceedsWithdrawn": False, "platformFeeWithdrawn": False, "failedProjectTokensWithdrawn": False, "stateGeneration": 1}
                state_digest = row_digest("JC_FL_AGGREGATE_STATE_V1", state_projection)
                cursor.execute("""INSERT INTO fair_launch_project_state_v3(project_binding_id,run_row_id,chain_id,source_id,contract_address,contract_version,project_id,project_token_address,payment_token_address,status,terminal_reason,sale_token_amount_raw,total_raised_raw,total_tokens_deposited_raw,total_tokens_claimed_raw,participant_count_raw,final_total_raised_raw,final_price_numerator_raw,final_price_denominator_raw,platform_fee_reserved_raw,project_proceeds_paid_raw,platform_fee_paid_raw,settlement_block_number,settlement_block_hash,settlement_anchor_block_number,settlement_anchor_block_hash,settlement_tx_hash,settlement_tx_log_ordinal,release_manifest_digest,release_policy_digest,release_policy_version,project_proceeds_withdrawn,platform_fee_withdrawn,failed_project_tokens_withdrawn,tombstoned,state_generation,last_event_identity_id,last_placement_id,last_source_revision_id,last_effect_manifest_version,last_application_attempt_id,source_application_lineage_id,state_digest,created_at,updated_at) VALUES(%s,%s,31337,%s,%s,'fair_launch_v1',%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,NULL,NULL,NULL,'0','0','0',NULL,NULL,NULL,NULL,NULL,NULL,NULL,%s,%s,0,0,0,0,1,NULL,NULL,NULL,NULL,NULL,NULL,%s,%s,%s)""", (project_binding_id, run_row_id, source_id, proxy["address"], int(project["projectId"]), token["address"], payment["address"], state_projection["status"], state_projection["terminalReason"], storage["saleTokenAmount"], storage["totalRaised"], storage["totalTokensDeposited"], storage["totalTokensClaimed"], storage["participantCount"], storage["releasePolicyDigest"][2:], int(storage["releasePolicyVersion"]), state_digest, now, now))

            lease_token = row_digest("JC_FL_SOURCE_MATERIALIZATION_LEASE_V1", {"runId": run_id, "runRowId": run_row_id, "sourceId": source_id, "sourceRevisionId": revision_id})
            cursor.execute("INSERT INTO source_materialization_cursor(source_id,source_revision_id,materialized_through_block,materialized_through_block_hash,materialized_through_tx_index,materialized_through_tx_log_ordinal,last_event_identity_id,lease_token,fence_epoch,status,last_error,updated_at,cursor_generation) VALUES(%s,%s,%s,%s,%s,0,NULL,%s,1,'active','',%s,1)", (source_id, revision_id, int(observation["blockNumber"]), observation["blockHash"], observation["transactionIndex"], lease_token, now))
            coverage_digest = row_digest("JC_FL_SOURCE_COVERAGE_V1", {"sourceId": source_id, "fromBlock": source_plan["fromBlock"], "throughBlock": int(observation["blockNumber"]), "throughBlockHash": observation["blockHash"]})
            cursor.execute("INSERT INTO source_ingestion_coverage(source_id,from_block,through_block,coverage_digest,finalized) VALUES(%s,%s,%s,%s,1)", (source_id, source_plan["fromBlock"], int(observation["blockNumber"]), coverage_digest))

            observations = {
                "mysql_server": {"serverUuid": state["MYSQL_SERVER_UUID"], "version": state["MYSQL_VERSION"], "database": state["DB_NAME"]},
                "mysql_information_schema": {"baselineDigest": state["BASELINE_DIGEST"], "migrationManifestDigest": state["MIGRATION_MANIFEST_DIGEST"], "schemaDigest": state["SCHEMA_DIGEST"], "databaseEvidenceSha256": sha_bytes(database_raw), "schemaReadinessSha256": sha_bytes(readiness_raw)},
                "mysql_grants": {"grantPolicyDigest": state["GRANT_POLICY_DIGEST"], "roleGrantSha256": actual_grants},
                "mysql_container": {"containerId": state["MYSQL_CONTAINER_ID"], "imageId": state["MYSQL_IMAGE_ID"], "ownerLabelDigest": state["OWNER_LABEL_DIGEST"], "transportDigest": state["MYSQL_TRANSPORT_DIGEST"], "containerAuthoritySha256": sha_bytes(container_raw), "infrastructureSha256": sha_bytes(infrastructure_raw)},
                "redis_instance": {"endpoint": f"127.0.0.1:{state['REDIS_PORT']}", "processAuthoritySha256": sha_file(run_dir / "redis" / "process.json")},
                "hardhat_genesis": {"genesisHash": authority["genesisHash"]},
                "hardhat_chain_id": {"chainId": 31337},
                "chain_manifest": {"payloadSha256": authority["payloadSha256"], "signedObservation": observation},
                "deployment_manifest": {"manifestSha256": authority["manifestSha256"], "manifestByteLength": len(manifest_raw)},
                "runtime_admission": {"runtimeAdmissionSha256": runtime_admission_digest, "schemaVersion": authority["schemaVersion"]},
                "source_registry": {"sourceRegistryDigest": source_registry_digest, "projection": registry_projection},
            }
            for kind, value in observations.items():
                value_raw = canonical(value).decode("utf-8")
                cursor.execute("INSERT INTO fair_launch_local_e2e_observation(run_row_id,observation_kind,observed_value,value_digest,collector_version,observed_at) VALUES(%s,%s,%s,%s,%s,%s)", (run_row_id, kind, value_raw, sha_bytes(value_raw.encode()), COLLECTOR, now))

            cursor.execute("SELECT COUNT(*) FROM fair_launch_project_binding_v3 WHERE run_row_id=%s", (run_row_id,))
            if cursor.fetchone()[0] != 3:
                refuse("bootstrap project binding count mismatch")
            cursor.execute("SELECT COUNT(*) FROM fair_launch_source_token_binding_v3 WHERE run_row_id=%s", (run_row_id,))
            if cursor.fetchone()[0] != 4:
                refuse("bootstrap token binding count mismatch")
            cursor.execute("SELECT COUNT(*) FROM fair_launch_local_e2e_observation WHERE run_row_id=%s", (run_row_id,))
            if cursor.fetchone()[0] != 11:
                refuse("bootstrap observation count mismatch")
        connection.commit()
    except Exception:
        connection.rollback()
        raise
    finally:
        connection.close()
    return {
        "authority": AUTHORITY,
        "schemaVersion": "fair-launch-local-e2e-backend-bootstrap-v1",
        "runId": run_id,
        "database": state["DB_NAME"],
        "chainId": 31337,
        "rpcUrl": authority["rpcUrl"],
        "genesisHash": authority["genesisHash"],
        "manifestSha256": authority["manifestSha256"],
        "runtimeAdmissionSha256": sha_bytes(authority_raw),
        "schemaDigest": state["SCHEMA_DIGEST"],
        "grantPolicyDigest": state["GRANT_POLICY_DIGEST"],
        "sourceRegistryDigest": source_registry_digest,
        "runRowId": run_row_id,
        "sourceId": source_id,
        "sourceRevisionId": revision_id,
        "sourceBindingId": source_binding_id,
        "projectIds": [101, 102, 103],
        "signedTip": {"blockNumber": observation["blockNumber"], "blockHash": observation["blockHash"], "transactionIndex": observation["transactionIndex"]},
        "createdAt": now.replace(tzinfo=dt.timezone.utc).isoformat(timespec="microseconds").replace("+00:00", "Z"),
        "expiresAt": expires.isoformat(timespec="milliseconds").replace("+00:00", "Z"),
        "status": "BOOTSTRAPPED_NOT_PRODUCTION_ADMITTED",
    }


def publish_once(path: pathlib.Path, value) -> None:
    if path.exists() or path.is_symlink():
        refuse("bootstrap evidence already exists")
    raw = canonical(value) + b"\n"
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    try:
        os.write(descriptor, raw)
        os.fsync(descriptor)
    finally:
        os.close(descriptor)
    os.chmod(path, 0o600)
    if owned_file(path) != raw:
        refuse("bootstrap evidence read-back mismatch")


def main() -> int:
    if len(sys.argv) != 2 or not RUN_PATTERN.fullmatch(sys.argv[1]):
        refuse("usage: bootstrap-runtime.py <canonical-run-id>")
    run_id = sys.argv[1]
    state_root = pathlib.Path(os.environ.get("JC_FL_E2E_STATE_ROOT", "/tmp/joincare-fair-launch-local-e2e"))
    if not state_root.is_absolute() or state_root != pathlib.Path(os.path.realpath(state_root)):
        refuse("state root is not absolute canonical")
    run_dir = state_root / run_id
    run_stat = run_dir.lstat()
    if not stat.S_ISDIR(run_stat.st_mode) or stat.S_ISLNK(run_stat.st_mode) or run_stat.st_uid != os.getuid() or stat.S_IMODE(run_stat.st_mode) != 0o700:
        refuse("run directory authority invalid")
    state = parse_run_state(run_dir / "state.env", run_id, run_dir)
    authority_path = run_dir / "evidence" / "contract-runtime-admission-v1.json"
    evidence_path = run_dir / "evidence" / "backend-bootstrap-v1.json"
    if evidence_path.exists() or evidence_path.is_symlink():
        refuse("bootstrap is create-once and evidence already exists")
    result = execute_bootstrap(run_id, run_dir, state, authority_path)
    publish_once(evidence_path, result)
    print(f"Fair Launch LOCAL-ONLY Backend bootstrap complete: run_id={run_id} evidence={evidence_path}")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Refusal as exc:
        print(f"fair-launch-local-e2e bootstrap: FAIL: {exc}", file=sys.stderr)
        raise SystemExit(1)

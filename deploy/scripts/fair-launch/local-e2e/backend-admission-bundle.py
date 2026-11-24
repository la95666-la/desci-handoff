#!/usr/bin/env python3
"""Emit one canonical Backend admission bundle to stdout.

The output contains the local_job password and must only be used as an inherited
stdin pipe to the dedicated LOCAL-ONLY Backend binary. It is never evidence and
must never be redirected to a regular file.
"""

import json
import os
import pathlib
import re
import stat
import sys


RUN = re.compile(r"^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$")


def fail(message):
    print(f"fair-launch-local-e2e bundle: FAIL: {message}", file=sys.stderr)
    raise SystemExit(1)


def strict_object(pairs, label):
    value = {}
    for key, item in pairs:
        if key in value:
            raise ValueError(f"{label} contains duplicate JSON keys")
        value[key] = item
    return value


def owned(path, mode=0o600):
    path = pathlib.Path(path)
    if not path.is_absolute() or path != pathlib.Path(os.path.realpath(path)):
        fail("non-canonical authority path")
    before = path.lstat()
    if not stat.S_ISREG(before.st_mode) or stat.S_ISLNK(before.st_mode) or before.st_uid != os.getuid() or (mode is not None and stat.S_IMODE(before.st_mode) != mode) or before.st_nlink != 1:
        fail(f"authority identity invalid: {path.name}")
    fd = os.open(path, os.O_RDONLY | getattr(os, "O_CLOEXEC", 0) | getattr(os, "O_NOFOLLOW", 0))
    try:
        opened = os.fstat(fd)
        fields = lambda row: (row.st_dev, row.st_ino, row.st_uid, row.st_mode, row.st_nlink, row.st_size, row.st_mtime_ns, row.st_ctime_ns)
        if fields(before) != fields(opened):
            fail(f"authority changed before read: {path.name}")
        chunks = []
        while True:
            block = os.read(fd, 65536)
            if not block:
                break
            chunks.append(block)
        raw = b"".join(chunks)
        finished = os.fstat(fd)
        if fields(opened) != fields(finished):
            fail(f"authority changed while read: {path.name}")
    finally:
        os.close(fd)
    after = path.lstat()
    if fields(before) != fields(after):
        fail(f"authority changed while read: {path.name}")
    try:
        return raw.decode("utf-8")
    except UnicodeDecodeError:
        fail(f"authority is not UTF-8: {path.name}")


def windows_path(value):
    if not isinstance(value, str) or not re.fullmatch(r"[A-Za-z]:\\[^\x00]+", value):
        fail("Contract path invalid")
    return pathlib.Path(f"/mnt/{value[0].lower()}/{value[3:].replace(chr(92), '/')}")


if len(sys.argv) != 2 or not RUN.fullmatch(sys.argv[1]):
    fail("usage: backend-admission-bundle.py <run-id>")
run_id = sys.argv[1]
root = pathlib.Path(os.environ.get("JC_FL_E2E_STATE_ROOT", "/tmp/joincare-fair-launch-local-e2e"))
run_dir = root / run_id
if not run_dir.is_dir() or run_dir.is_symlink():
    fail("run directory unavailable")
runtime = owned(run_dir / "evidence" / "contract-runtime-admission-v1.json")
try:
    authority = json.loads(runtime, object_pairs_hook=lambda pairs: strict_object(pairs, "Contract runtime admission"))
except (json.JSONDecodeError, ValueError):
    fail("Contract runtime admission invalid")
if not isinstance(authority, dict) or set(authority) != {"authority", "chainId", "contractStateRoot", "createdAt", "genesisHash", "manifestPath", "manifestSha256", "payloadSha256", "projectId", "rpcUrl", "runId", "schemaVersion", "trustPinPath", "trustPinSha256"}:
    fail("Contract runtime admission fields invalid")
# NTFS permission bits are synthetic under WSL. Exact bytes are still pinned by
# the admitted SHA-256 records, while identity drift is guarded by dev/inode,
# link count, size, mtime and ctime before/after each read.
manifest = owned(windows_path(authority["manifestPath"]), None)
pin = owned(windows_path(authority["trustPinPath"]), None)
trust = owned(windows_path(authority["contractStateRoot"]) / run_id / "public-trust-root.json", None)

# Field order exactly follows FairLaunchLocalE2EAdmissionBundle. Go verifies
# bytes.TrimSpace(stdin) against its own canonical encoding before admission.
bundle = {
    "schemaVersion": "fair-launch-local-e2e-backend-admission-bundle-v1",
    "runId": run_id,
    "contractRuntimeAdmission": runtime,
    "deploymentManifest": manifest,
    "externalTrustPin": pin,
    "publicTrustRoot": trust,
    "backendBootstrap": owned(run_dir / "evidence" / "backend-bootstrap-v1.json"),
    "databaseSchemaEvidence": owned(run_dir / "evidence" / "database-schema.json"),
    "schemaReadiness": owned(run_dir / "evidence" / "schema-readiness.json"),
    "infrastructureEvidence": owned(run_dir / "evidence" / "infrastructure.json"),
    "containerAuthority": owned(run_dir / "evidence" / "docker-created.source.json"),
    "expectedJobGrants": owned(run_dir / "grants" / "expected-job", 0o600),
    "actualJobGrants": owned(run_dir / "grants" / "actual-job", 0o644),
    "expectedIngestGrants": owned(run_dir / "grants" / "expected-ingest", 0o600),
    "actualIngestGrants": owned(run_dir / "grants" / "actual-ingest", 0o644),
    "mysqlJobProfile": owned(run_dir / "credentials" / "mysql-job.cnf", 0o600),
    "mysqlIngestProfile": owned(run_dir / "credentials" / "mysql-ingest.cnf", 0o600),
}
sys.stdout.write(json.dumps(bundle, ensure_ascii=False, separators=(",", ":")))

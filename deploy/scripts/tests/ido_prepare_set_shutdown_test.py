#!/usr/bin/env python3
import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path

root = Path(__file__).resolve().parents[2]
script = root / "scripts" / "ido_prepare_set.py"
tmp = Path(tempfile.mkdtemp(prefix="joincare-set-mode-"))
marker = tmp / "must-not-read.json"

# Every ambiguous or retired grammar must fail before attempting to open any path.
invalid = [
    [],
    [str(marker)],
    ["--dry-run"],
    ["--dry-run", str(marker)],
    [str(marker), "--", "--dry-run"],
    ["--", "--dry-run"],
    ["--dry-run", "--dry-run"],
    [str(marker), "--dry-run", "--dry-run"],
    [str(marker), "--tz", "8", "--dry-run"],
    [str(marker), "--database", "prod", "--dry-run"],
    [str(marker), "--execute", "--dry-run"],
    ["-params.json", "--dry-run"],
]
for argv in invalid:
    result = subprocess.run(
        [sys.executable, str(script), *argv],
        text=True,
        capture_output=True,
        env={**os.environ, "MYSQL_PWD": "must-not-read", "RPC_URL": "must-not-read"},
    )
    assert result.returncode == 78, (argv, result)
    assert result.stderr.strip() == "fair_launch_transformation_unavailable", (argv, result)
    assert result.stdout == "", (argv, result)
    assert not marker.exists(), argv

source = script.read_text(encoding="utf-8")
for forbidden in ("subprocess", "urllib.request", "docker inspect", "docker exec", "MYSQL_ROOT_PASSWORD", "MYSQL_PWD", "--mysql-cmd"):
    assert forbidden not in source, forbidden

params = tmp / "joincare-set-dry-run.json"
params.write_text(json.dumps({
    "project_id": 1, "token_address": "0x1111111111111111111111111111111111111111", "token_decimals": 18,
    "payment_token": "USDT", "payment_token_address": "0x2222222222222222222222222222222222222222", "payment_token_decimals": 18,
    "owner_address": "0x3333333333333333333333333333333333333333", "token_price": "1", "hard_cap": "10",
    "soft_cap": "5", "max_per_user": "2", "start_time": "+1h", "end_time": "+2h", "tge": [{"percent": 100}]
}), encoding="utf-8")
dry = subprocess.run([sys.executable, str(script), str(params), "--dry-run"], text=True, capture_output=True)
assert dry.returncode == 0, dry
assert "OFFLINE EVIDENCE ONLY; DO NOT EXECUTE" in dry.stdout, dry
assert "START TRANSACTION;" in dry.stdout, dry
print("IDO Set exact offline grammar and fail-before-read boundary: PASS")

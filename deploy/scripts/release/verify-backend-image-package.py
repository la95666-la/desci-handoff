#!/usr/bin/env python3
"""Reject cumulative Backend images and unexpectedly large release archives."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import tarfile


DEFAULT_MAX_BYTES = 90 * 1024 * 1024


def fail(message: str) -> None:
    raise SystemExit(f"backend-package-size-guard: {message}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("package", type=Path)
    parser.add_argument("--max-bytes", type=int, default=DEFAULT_MAX_BYTES)
    args = parser.parse_args()

    size = args.package.stat().st_size
    if size > args.max_bytes:
        fail(f"archive is {size} bytes; reviewed ceiling is {args.max_bytes}")

    with tarfile.open(args.package, "r:gz") as archive:
        manifest = json.load(archive.extractfile("manifest.json"))
        if not isinstance(manifest, list) or len(manifest) != 1:
            fail("manifest must contain exactly one image")
        record = manifest[0]
        if set(record) != {"Config", "RepoTags", "Layers"}:
            fail("package must be normalized strict docker-save")
        if len(record["Layers"]) > 7:
            fail(f"image has {len(record['Layers'])} layers; cumulative repack suspected")
        config = json.load(archive.extractfile(record["Config"]))

    history = [str(item.get("created_by", "")) for item in config.get("history", [])]
    for binary in ("core", "job", "notice"):
        matches = [line for line in history if "COPY" in line and f"joincare-{binary}" in line]
        if len(matches) != 1:
            fail(f"expected one joincare-{binary} COPY layer, found {len(matches)}")
    if any("RUN " in line and "chmod" in line and "joincare-" in line for line in history):
        fail("chmod copy-up layer found; use COPY --chmod=0755")

    print(f"backend package size guard passed: bytes={size} layers={len(record['Layers'])}")


if __name__ == "__main__":
    main()

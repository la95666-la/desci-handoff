#!/usr/bin/env python3
"""Normalize Docker 29 OCI-augmented save archives to strict legacy docker-save."""

from __future__ import annotations

import hashlib
import io
import json
import os
from pathlib import Path, PureWindowsPath
import re
import sys
import tarfile


LAYER_MEDIA_TYPE = "application/vnd.oci.image.layer.v1.tar"
TAG = re.compile(r"^joincare/backend:[A-Za-z0-9][A-Za-z0-9._-]{0,127}$")
DIGEST = re.compile(r"^sha256:([0-9a-f]{64})$")


class Refusal(ValueError):
    pass


def strict_object(pairs):
    value = {}
    for key, item in pairs:
        if key in value:
            raise Refusal(f"duplicate JSON key: {key}")
        value[key] = item
    return value


def strict_json(raw: bytes, label: str):
    try:
        return json.loads(
            raw.decode("utf-8", "strict"),
            object_pairs_hook=strict_object,
            parse_constant=lambda value: (_ for _ in ()).throw(Refusal(f"non-finite JSON: {value}")),
        )
    except (UnicodeDecodeError, json.JSONDecodeError, Refusal) as exc:
        raise Refusal(f"{label} is not strict UTF-8 JSON: {exc}") from exc


def name(raw: str) -> tuple[str, ...]:
    windows = PureWindowsPath(raw)
    if not raw or raw in {".", "./"} or "\\" in raw or ":" in raw or raw.startswith("/") or windows.drive or windows.root:
        raise Refusal(f"unsafe archive member: {raw!r}")
    trimmed = raw[:-1] if raw.endswith("/") else raw
    parts = tuple(trimmed.split("/"))
    if not trimmed or any(part in {"", ".", ".."} for part in parts):
        raise Refusal(f"non-normal archive member: {raw!r}")
    return parts


def regular(tf: tarfile.TarFile, member: tarfile.TarInfo, limit: int | None = None) -> bytes:
    if not member.isreg() or member.size < 0 or (limit is not None and member.size > limit):
        raise Refusal(f"invalid regular member: {member.name!r}")
    source = tf.extractfile(member)
    if source is None:
        raise Refusal(f"member has no data: {member.name!r}")
    with source:
        data = source.read(member.size + 1)
    if len(data) != member.size:
        raise Refusal(f"member size mismatch: {member.name!r}")
    return data


def main(source_arg: str, output_arg: str) -> None:
    source, output = Path(source_arg), Path(output_arg)
    if source.resolve() == output.resolve():
        raise Refusal("source and output must differ")
    with tarfile.open(source, "r:gz") as tf:
        members = tf.getmembers()
        if len(members) > 20_000:
            raise Refusal("archive member limit exceeded")
        by_path = {}
        for member in members:
            if tf.pax_headers or member.pax_headers or not (member.isreg() or member.isdir()):
                raise Refusal(f"unsupported archive metadata/type: {member.name!r}")
            parts = name(member.name)
            if parts in by_path:
                raise Refusal(f"duplicate archive member: {member.name!r}")
            by_path[parts] = member

        manifest_member = by_path.get(("manifest.json",))
        if manifest_member is None:
            raise Refusal("manifest.json missing")
        manifest = strict_json(regular(tf, manifest_member, 1024 * 1024), "manifest.json")
        if not isinstance(manifest, list) or len(manifest) != 1 or not isinstance(manifest[0], dict):
            raise Refusal("manifest must contain exactly one record")
        record = manifest[0]
        if set(record) != {"Config", "RepoTags", "Layers", "LayerSources"}:
            raise Refusal("source manifest is not the admitted Docker 29 LayerSources schema")
        tags = record["RepoTags"]
        if not isinstance(tags, list) or len(tags) != 1 or not isinstance(tags[0], str) or not TAG.fullmatch(tags[0]):
            raise Refusal("RepoTags invalid")
        if not isinstance(record["Config"], str) or not isinstance(record["Layers"], list) or not record["Layers"]:
            raise Refusal("Config/Layers invalid")
        config = name(record["Config"])
        layers = [name(item) if isinstance(item, str) else () for item in record["Layers"]]
        if any(not item for item in layers) or len(set(layers)) != len(layers) or config in layers:
            raise Refusal("Config/Layers references invalid")
        references = [config, *layers]
        for ref in references:
            member = by_path.get(ref)
            if member is None or not member.isreg():
                raise Refusal(f"referenced regular member missing: {'/'.join(ref)}")
        config_value = strict_json(regular(tf, by_path[config], 4 * 1024 * 1024), "Config")
        if not isinstance(config_value, dict):
            raise Refusal("Config must be an object")

        sources = record["LayerSources"]
        if not isinstance(sources, dict) or len(sources) != len(layers):
            raise Refusal("LayerSources count mismatch")
        expected = {}
        for layer in layers:
            if len(layer) != 3 or layer[:2] != ("blobs", "sha256") or not re.fullmatch(r"[0-9a-f]{64}", layer[2]):
                raise Refusal("Layer path is not blobs/sha256/<digest>")
            expected[f"sha256:{layer[2]}"] = layer
        if set(sources) != set(expected):
            raise Refusal("LayerSources keys mismatch")
        payloads = {}
        for digest, layer in expected.items():
            item = sources[digest]
            if not isinstance(item, dict) or set(item) != {"digest", "mediaType", "size"}:
                raise Refusal("LayerSources record shape invalid")
            match = DIGEST.fullmatch(item.get("digest", ""))
            data = regular(tf, by_path[layer])
            if not match or item["digest"] != digest or item.get("mediaType") != LAYER_MEDIA_TYPE:
                raise Refusal("LayerSources identity invalid")
            if isinstance(item.get("size"), bool) or item.get("size") != len(data):
                raise Refusal("LayerSources size mismatch")
            if hashlib.sha256(data).hexdigest() != match.group(1):
                raise Refusal("LayerSources content digest mismatch")
            payloads[layer] = data
        config_data = regular(tf, by_path[config], 4 * 1024 * 1024)

    legacy = [{"Config": "/".join(config), "RepoTags": tags, "Layers": ["/".join(layer) for layer in layers]}]
    output.parent.mkdir(parents=True, exist_ok=True)
    temp = output.with_name(output.name + f".tmp-{os.getpid()}")
    try:
        with tarfile.open(temp, "w:gz", format=tarfile.USTAR_FORMAT) as target:
            emitted_dirs = set()
            def emit_dirs(parts):
                for length in range(1, len(parts)):
                    directory = parts[:length]
                    if directory in emitted_dirs:
                        continue
                    info = tarfile.TarInfo("/".join(directory))
                    info.type, info.mode, info.mtime = tarfile.DIRTYPE, 0o755, 0
                    target.addfile(info)
                    emitted_dirs.add(directory)
            def emit_file(parts, data):
                emit_dirs(parts)
                info = tarfile.TarInfo("/".join(parts))
                info.size, info.mode, info.mtime = len(data), 0o644, 0
                target.addfile(info, io.BytesIO(data))
            emit_file(("manifest.json",), json.dumps(legacy, separators=(",", ":"), sort_keys=True).encode())
            emit_file(config, config_data)
            for layer in layers:
                emit_file(layer, payloads[layer])
        os.replace(temp, output)
    finally:
        if temp.exists():
            temp.unlink()


if __name__ == "__main__":
    try:
        if len(sys.argv) != 3:
            raise Refusal("usage: normalize_docker_save.py SOURCE OUTPUT")
        main(sys.argv[1], sys.argv[2])
    except (OSError, tarfile.TarError, Refusal) as exc:
        print(f"normalize-docker-save: {exc}", file=sys.stderr)
        raise SystemExit(1)

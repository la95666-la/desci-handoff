#!/usr/bin/env python3
"""Validate and optionally extract release tar archives from one stable file handle."""

from __future__ import annotations

import argparse
import fnmatch
import gzip
import hashlib
import json
import os
from pathlib import Path, PureWindowsPath
import re
import shutil
import stat
import sys
import tarfile
from dataclasses import dataclass


MIB = 1024 * 1024
GIB = 1024 * MIB
COPY_CHUNK = 1024 * 1024


@dataclass(frozen=True)
class Policy:
    max_compressed: int
    max_members: int
    max_file: int
    max_total: int
    max_ratio: int


# Offline evidence packages are intentionally bounded well below filesystem-scale
# limits. max_total counts regular member payloads; raw preflight separately counts
# every decompressed header, payload and padding byte.
POLICIES = {
    "static": Policy(64 * MIB, 4_096, 32 * MIB, 128 * MIB, 100),
    "runtime": Policy(1 * GIB, 20_000, 1 * GIB, 3 * GIB, 100),
    "docker": Policy(1 * GIB, 20_000, 2 * GIB, 4 * GIB, 100),
}
DOCKER_MANIFEST_MAX = 1024 * 1024
DOCKER_CONFIG_MAX = 4 * MIB
DOCKER_TAG = re.compile(r"^joincare/backend:[A-Za-z0-9][A-Za-z0-9._-]{0,127}$")
RAW_ALLOWED_TYPES = {b"\0", b"0", b"5"}
RAW_REJECTED_EXTENDED_TYPES = {b"x", b"g", b"L", b"K", b"S"}
RUNTIME_DIRECTORIES = {
    ("artifacts",),
    ("artifacts", "backend"),
    ("images",),
    ("packages",),
    ("deploy",),
    ("scripts",),
    ("scripts", "release"),
    ("scripts", "release", "lib"),
}
RUNTIME_EXACT_FILES = {
    ("artifacts", "backend", "joincare-core"),
    ("artifacts", "backend", "joincare-job"),
    ("artifacts", "backend", "joincare-notice"),
    ("deploy", "Dockerfile.backend.runtime"),
    ("scripts", "release", "lib", "safe_tar.py"),
    ("release-manifest.json",),
    ("fair-launch-release-authority.json",),
    ("SHA256SUMS",),
    ("README-DEPLOY.md",),
}
RUNTIME_FILE_PATTERNS = (
    "images/joincare-backend-*.tar.gz",
    "images/joincare-backend-*.tar.gz.sha256",
    "packages/joincare-frontend-*.tar.gz",
    "packages/joincare-frontend-*.tar.gz.sha256",
    "packages/joincare-admin-*.tar.gz",
    "packages/joincare-admin-*.tar.gz.sha256",
)


class ArchiveError(ValueError):
    pass


def _strict_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ArchiveError(f"duplicate JSON object key: {key!r}")
        result[key] = value
    return result


def _reject_json_constant(value: str):
    raise ArchiveError(f"non-finite JSON constant is unsupported: {value}")


def _strict_json_loads(raw: bytes, label: str):
    try:
        return json.loads(
            raw.decode("utf-8", "strict"),
            object_pairs_hook=_strict_object,
            parse_constant=_reject_json_constant,
        )
    except (UnicodeDecodeError, json.JSONDecodeError, ArchiveError) as exc:
        raise ArchiveError(f"{label} is not strict bounded UTF-8 JSON: {exc}") from exc


def validated_name(raw: str) -> tuple[str, ...]:
    if not raw or raw in {".", "./"}:
        raise ArchiveError("empty or dot archive member")
    windows = PureWindowsPath(raw)
    if (
        "\\" in raw
        or ":" in raw
        or raw.startswith("/")
        or windows.drive
        or windows.root
        or windows.is_absolute()
    ):
        raise ArchiveError(f"absolute, rooted, drive-prefixed, or ambiguous archive member: {raw!r}")
    trimmed = raw[:-1] if raw.endswith("/") else raw
    if len(trimmed.encode("utf-8")) > 100:
        raise ArchiveError(f"extended/overlong archive member name is unsupported: {raw!r}")
    parts = tuple(trimmed.split("/"))
    if not trimmed or any(part in {"", ".", ".."} for part in parts):
        raise ArchiveError(f"non-normal archive member: {raw!r}")
    return parts


def _open_stable(path: Path):
    flags = os.O_RDONLY | getattr(os, "O_BINARY", 0)
    if hasattr(os, "O_NOFOLLOW"):
        flags |= os.O_NOFOLLOW
    try:
        fd = os.open(path, flags)
    except OSError as exc:
        raise ArchiveError(f"cannot open archive safely: {exc}") from exc
    handle = os.fdopen(fd, "rb", closefd=True)
    opened = os.fstat(handle.fileno())
    if not stat.S_ISREG(opened.st_mode):
        handle.close()
        raise ArchiveError("archive is not a regular file")
    return handle, opened


def _same_identity(left, right) -> bool:
    return (
        left.st_dev,
        left.st_ino,
        left.st_size,
        left.st_mtime_ns,
        left.st_ctime_ns,
    ) == (
        right.st_dev,
        right.st_ino,
        right.st_size,
        right.st_mtime_ns,
        right.st_ctime_ns,
    )


def _hash_handle(handle, max_compressed: int) -> tuple[str, int]:
    digest = hashlib.sha256()
    total = 0
    handle.seek(0)
    while True:
        block = handle.read(COPY_CHUNK)
        if not block:
            break
        total += len(block)
        if total > max_compressed:
            raise ArchiveError("archive compressed-size limit exceeded")
        digest.update(block)
    handle.seek(0)
    return digest.hexdigest(), total


def _parse_raw_octal(field: bytes, label: str) -> int:
    """Accept only the portable NUL/space padded octal encoding used by ustar."""
    if field and field[0] & 0x80:
        raise ArchiveError(f"base-256 {label} encoding is unsupported")
    stripped = field.strip(b" \0")
    if not stripped:
        return 0
    if any(byte < ord("0") or byte > ord("7") for byte in stripped):
        raise ArchiveError(f"invalid raw tar {label} encoding")
    return int(stripped, 8)


def _raw_header_name(block: bytes) -> str:
    name = block[0:100].split(b"\0", 1)[0]
    prefix = block[345:500].split(b"\0", 1)[0]
    raw = prefix + (b"/" if prefix and name else b"") + name
    try:
        return raw.decode("utf-8", "strict")
    except UnicodeDecodeError as exc:
        raise ArchiveError("raw tar member name is not UTF-8") from exc


def _raw_stream_limit(policy: Policy) -> int:
    # Account for one 512-byte header and up to 511 padding bytes per member,
    # plus the two required terminator blocks and bounded trailing zero padding.
    return policy.max_total + policy.max_members * 1024 + 4 * 512


def preflight_raw_gzip_tar(handle, policy: Policy, compressed_size: int) -> None:
    """Bound and validate raw tar headers before tarfile sees metadata payloads.

    The gzip stream is consumed from the same stable archive handle later rewound for
    tarfile. Unsupported PAX/GNU metadata is rejected immediately after its 512-byte
    header, before its payload is read or interpreted by tarfile.
    """
    handle.seek(0)
    total = 0
    members = 0
    zero_blocks = 0
    stream_limit = _raw_stream_limit(policy)
    ratio_limit = max(compressed_size, 1) * policy.max_ratio

    def read_bounded(size: int) -> bytes:
        nonlocal total
        data = gz.read(size)
        total += len(data)
        if total > stream_limit:
            raise ArchiveError("raw tar decompressed-size limit exceeded")
        if total > ratio_limit:
            raise ArchiveError("raw tar expansion-ratio limit exceeded")
        return data

    try:
        with gzip.GzipFile(fileobj=handle, mode="rb") as gz:
            while True:
                block = read_bounded(512)
                if not block:
                    raise ArchiveError("raw tar ended without two zero terminator blocks")
                if len(block) != 512:
                    raise ArchiveError("truncated raw tar header block")
                if block == b"\0" * 512:
                    zero_blocks += 1
                    if zero_blocks < 2:
                        continue
                    # Account for all decompressed trailing padding and reject any
                    # nonzero concatenated data/member after the terminator.
                    while True:
                        trailing = read_bounded(COPY_CHUNK)
                        if not trailing:
                            break
                        if any(trailing):
                            raise ArchiveError("nonzero data follows raw tar terminator")
                    break
                if zero_blocks:
                    raise ArchiveError("nonzero header follows raw tar zero block")

                members += 1
                if members > policy.max_members:
                    raise ArchiveError("raw tar member limit exceeded")
                stored_checksum = _parse_raw_octal(block[148:156], "checksum")
                checksum_block = block[:148] + (b" " * 8) + block[156:]
                if stored_checksum != sum(checksum_block):
                    raise ArchiveError("raw tar header checksum mismatch")
                magic = block[257:263]
                if magic not in {b"\0" * 6, b"ustar\0", b"ustar "}:
                    raise ArchiveError("unsupported raw tar header magic")
                member_size = _parse_raw_octal(block[124:136], "size")
                typeflag = block[156:157]
                name = _raw_header_name(block)
                validated_name(name)
                if typeflag in RAW_REJECTED_EXTENDED_TYPES:
                    raise ArchiveError(f"unsupported raw tar extended type flag {typeflag!r}: {name!r}")
                if typeflag not in RAW_ALLOWED_TYPES:
                    raise ArchiveError(f"unsupported raw tar type flag {typeflag!r}: {name!r}")
                if typeflag == b"5" and member_size != 0:
                    raise ArchiveError(f"raw tar directory has a nonzero payload: {name!r}")
                if member_size > policy.max_file:
                    raise ArchiveError(f"raw tar per-file size limit exceeded: {name!r}")
                padded = (member_size + 511) & ~511
                remaining = padded
                while remaining:
                    chunk = read_bounded(min(COPY_CHUNK, remaining))
                    if not chunk:
                        raise ArchiveError(f"truncated raw tar member payload: {name!r}")
                    remaining -= len(chunk)
    except (gzip.BadGzipFile, EOFError, OSError) as exc:
        raise ArchiveError(f"invalid bounded gzip tar stream: {exc}") from exc
    finally:
        handle.seek(0)


def _reject_extended_metadata(tf: tarfile.TarFile, member: tarfile.TarInfo) -> None:
    if tf.pax_headers or member.pax_headers:
        raise ArchiveError(f"PAX extended metadata is unsupported: {member.name!r}")
    if getattr(member, "sparse", None):
        raise ArchiveError(f"GNU sparse member is unsupported: {member.name!r}")
    if member.type in {
        tarfile.XHDTYPE,
        tarfile.XGLTYPE,
        tarfile.GNUTYPE_LONGNAME,
        tarfile.GNUTYPE_LONGLINK,
        tarfile.GNUTYPE_SPARSE,
    }:
        raise ArchiveError(f"extended archive header is unsupported: {member.name!r}")


def _read_regular_member(tf: tarfile.TarFile, member: tarfile.TarInfo, limit: int) -> bytes:
    if not member.isreg() or member.size < 0 or member.size > limit:
        raise ArchiveError(f"invalid bounded regular member: {member.name!r}")
    source = tf.extractfile(member)
    if source is None:
        raise ArchiveError(f"regular member has no data: {member.name!r}")
    with source:
        data = source.read(member.size + 1)
    if len(data) != member.size:
        raise ArchiveError(f"member size mismatch: {member.name!r}")
    return data


def _docker_reference(value, label: str) -> tuple[str, ...]:
    if not isinstance(value, str):
        raise ArchiveError(f"docker manifest {label} must be a string")
    parts = validated_name(value)
    if parts == ("manifest.json",):
        raise ArchiveError(f"docker manifest {label} cannot reference manifest.json")
    return parts


def validate_docker_closure(tf: tarfile.TarFile, entries) -> None:
    by_path = {parts: member for member, parts in entries}
    manifest = by_path.get(("manifest.json",))
    if manifest is None or not manifest.isreg():
        raise ArchiveError("docker archive requires exactly one regular root manifest.json")
    raw_manifest = _read_regular_member(tf, manifest, DOCKER_MANIFEST_MAX)
    value = _strict_json_loads(raw_manifest, "docker manifest.json")
    if not isinstance(value, list) or len(value) != 1 or not isinstance(value[0], dict):
        raise ArchiveError("docker manifest must be a single-entry nonempty array")
    record = value[0]
    if set(record) != {"Config", "RepoTags", "Layers"}:
        raise ArchiveError("docker manifest record keys must be exactly Config, RepoTags and Layers")
    tags = record.get("RepoTags")
    if not isinstance(tags, list) or len(tags) != 1 or not isinstance(tags[0], str) or not DOCKER_TAG.fullmatch(tags[0]):
        raise ArchiveError("docker manifest requires exactly one valid joincare/backend:<tag> RepoTag")
    config = _docker_reference(record.get("Config"), "Config")
    layers_value = record.get("Layers")
    if not isinstance(layers_value, list) or not layers_value:
        raise ArchiveError("docker manifest Layers must be a nonempty array")
    layers = [_docker_reference(item, "Layers entry") for item in layers_value]
    if len(set(layers)) != len(layers):
        raise ArchiveError("docker manifest contains a duplicate layer reference")
    references = [config, *layers]
    if len(set(references)) != len(references):
        raise ArchiveError("docker Config and Layers references must be unique")

    required_files = {("manifest.json",), *references}
    required_directories = {
        parts[:length]
        for parts in references
        for length in range(1, len(parts))
    }
    for parts in required_files:
        member = by_path.get(parts)
        if member is None or not member.isreg():
            raise ArchiveError(f"docker manifest reference is not exactly one regular member: {'/'.join(parts)!r}")
    for member, parts in entries:
        if member.isreg() and parts not in required_files:
            raise ArchiveError(f"unexpected docker archive regular member: {member.name!r}")
        if member.isdir() and parts not in required_directories:
            raise ArchiveError(f"unexpected docker archive directory: {member.name!r}")

    config_blob = _read_regular_member(tf, by_path[config], DOCKER_CONFIG_MAX)
    config_value = _strict_json_loads(config_blob, "docker Config member")
    if not isinstance(config_value, dict):
        raise ArchiveError("docker Config JSON must be an object")


def _runtime_member_allowed(parts: tuple[str, ...], is_directory: bool) -> bool:
    if is_directory:
        return parts in RUNTIME_DIRECTORIES
    if parts in RUNTIME_EXACT_FILES:
        return True
    normalized = "/".join(parts)
    return any(fnmatch.fnmatchcase(normalized, pattern) for pattern in RUNTIME_FILE_PATTERNS)


def inspect_open_archive(tf: tarfile.TarFile, policy_name: str, compressed_size: int):
    policy = POLICIES[policy_name]
    seen: dict[tuple[str, ...], str] = {}
    entries: list[tuple[tarfile.TarInfo, tuple[str, ...]]] = []
    total_size = 0
    index_count = 0
    manifest_count = 0
    runtime_pattern_counts = {pattern: 0 for pattern in RUNTIME_FILE_PATTERNS}

    for number, member in enumerate(tf, start=1):
        if number > policy.max_members:
            raise ArchiveError("archive member limit exceeded")
        _reject_extended_metadata(tf, member)
        parts = validated_name(member.name)
        if parts in seen:
            raise ArchiveError(f"duplicate archive member: {member.name!r}")
        if not (member.isdir() or member.isreg()):
            raise ArchiveError(f"unsupported archive member type: {member.name!r}")
        for length in range(1, len(parts)):
            if seen.get(parts[:length]) == "file":
                raise ArchiveError(f"archive path conflicts with file ancestor: {member.name!r}")
        if member.isreg() and any(existing[: len(parts)] == parts for existing in seen):
            raise ArchiveError(f"archive file conflicts with descendant: {member.name!r}")

        if policy_name == "static":
            if parts[0] != "dist":
                raise ArchiveError(f"unexpected static-package member: {member.name!r}")
            if len(parts) == 1 and not member.isdir():
                raise ArchiveError("static-package dist root must be a directory")
            if parts == ("dist", "index.html"):
                if not member.isreg():
                    raise ArchiveError("dist/index.html must be a regular file")
                index_count += 1
        elif policy_name == "runtime":
            if not _runtime_member_allowed(parts, member.isdir()):
                raise ArchiveError(f"unexpected runtime-package member: {member.name!r}")
            if member.isreg():
                normalized = "/".join(parts)
                for pattern in RUNTIME_FILE_PATTERNS:
                    if fnmatch.fnmatchcase(normalized, pattern):
                        runtime_pattern_counts[pattern] += 1
        elif policy_name == "docker" and parts == ("manifest.json",):
            if not member.isreg():
                raise ArchiveError("docker manifest.json must be a regular file")
            if member.size > DOCKER_MANIFEST_MAX:
                raise ArchiveError("docker manifest.json size limit exceeded")
            manifest_count += 1

        seen[parts] = "dir" if member.isdir() else "file"
        if member.isreg():
            if member.size < 0 or member.size > policy.max_file:
                raise ArchiveError(f"archive per-file size limit exceeded: {member.name!r}")
            total_size += member.size
            if total_size > policy.max_total:
                raise ArchiveError("archive uncompressed-size limit exceeded")
        entries.append((member, parts))

    if total_size > max(compressed_size, 1) * policy.max_ratio:
        raise ArchiveError("archive expansion-ratio limit exceeded")
    if policy_name == "static" and index_count != 1:
        raise ArchiveError("static package requires exactly one regular dist/index.html")
    if policy_name == "runtime":
        missing = sorted("/".join(parts) for parts in RUNTIME_EXACT_FILES if seen.get(parts) != "file")
        if missing:
            raise ArchiveError(f"runtime package missing required regular member: {missing[0]}")
        for pattern, count in runtime_pattern_counts.items():
            if count != 1:
                raise ArchiveError(f"runtime package requires exactly one member matching {pattern!r}")
    if policy_name == "docker":
        if manifest_count != 1:
            raise ArchiveError("docker archive requires exactly one regular root manifest.json")
        validate_docker_closure(tf, entries)
    return entries, total_size


def _require_posix_safe_extraction() -> None:
    dir_fd_functions = {os.open, os.mkdir}
    if (
        os.name != "posix"
        or not hasattr(os, "O_NOFOLLOW")
        or not hasattr(os, "O_DIRECTORY")
        or not dir_fd_functions.issubset(os.supports_dir_fd)
        or os.chmod not in os.supports_fd
    ):
        # Windows extraction stays disabled until a native implementation opens
        # every path component with reparse-point-safe semantics.
        raise ArchiveError("archive_extraction_native_nofollow_unavailable")


def _open_directory_chain_no_follow(path: Path) -> int:
    """Open an existing directory through no-follow directory fds for every component."""
    _require_posix_safe_extraction()
    absolute = path.absolute()
    parts = absolute.parts
    if not parts or parts[0] != os.sep:
        raise ArchiveError("extraction destination must resolve from the POSIX root")
    flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW
    current = os.open(os.sep, flags)
    try:
        for part in parts[1:]:
            next_fd = os.open(part, flags, dir_fd=current)
            os.close(current)
            current = next_fd
        return current
    except Exception:
        os.close(current)
        raise


def _prepare_root_fd(destination: Path, total_size: int) -> int:
    _require_posix_safe_extraction()
    absolute = destination.absolute()
    parent_fd = _open_directory_chain_no_follow(absolute.parent)
    try:
        os.mkdir(absolute.name, 0o700, dir_fd=parent_fd)
    except FileExistsError:
        pass
    except OSError as exc:
        raise ArchiveError(f"cannot create extraction destination safely: {exc}") from exc
    finally:
        os.close(parent_fd)
    try:
        root_fd = _open_directory_chain_no_follow(absolute)
    except OSError as exc:
        raise ArchiveError(f"cannot open extraction destination safely: {exc}") from exc
    try:
        if os.listdir(root_fd):
            raise ArchiveError("extraction destination must be empty")
        free = shutil.disk_usage(root_fd).free
        if total_size > free:
            raise ArchiveError("insufficient free space for archive extraction")
        return root_fd
    except Exception:
        os.close(root_fd)
        raise


def _open_relative_directory(root_fd: int, parts: tuple[str, ...]) -> int:
    flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW
    current = os.dup(root_fd)
    try:
        for part in parts:
            next_fd = os.open(part, flags, dir_fd=current)
            os.close(current)
            current = next_fd
        return current
    except Exception:
        os.close(current)
        raise


def _copy_exact(source, output, expected: int) -> None:
    written = 0
    while True:
        block = source.read(min(COPY_CHUNK, expected - written + 1))
        if not block:
            break
        written += len(block)
        if written > expected:
            raise ArchiveError("archive member exceeded declared size while extracting")
        output.write(block)
    if written != expected:
        raise ArchiveError("archive member data shorter than declared size")


def extract_inspected(tf: tarfile.TarFile, entries, destination: Path, total_size: int) -> None:
    root_fd = _prepare_root_fd(destination, total_size)
    explicit_directory_modes = {parts: (member.mode & 0o777) | 0o700 for member, parts in entries if member.isdir()}
    directory_paths = set(explicit_directory_modes)
    for _, parts in entries:
        for length in range(1, len(parts)):
            directory_paths.add(parts[:length])
    try:
        # Directories remain owner-only while populated. Every operation is relative
        # to the stable root descriptor and every reopened component is no-follow.
        for parts in sorted(directory_paths, key=lambda value: (len(value), value)):
            parent_fd = _open_relative_directory(root_fd, parts[:-1])
            try:
                os.mkdir(parts[-1], 0o700, dir_fd=parent_fd)
            except OSError as exc:
                raise ArchiveError(f"cannot create extraction directory safely: {'/'.join(parts)!r}: {exc}") from exc
            finally:
                os.close(parent_fd)
            created_fd = _open_relative_directory(root_fd, parts)
            os.close(created_fd)

        for member, parts in entries:
            if member.isdir():
                continue
            parent_fd = _open_relative_directory(root_fd, parts[:-1])
            source = tf.extractfile(member)
            if source is None:
                os.close(parent_fd)
                raise ArchiveError(f"regular member has no data: {member.name!r}")
            flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | getattr(os, "O_BINARY", 0)
            try:
                fd = os.open(parts[-1], flags, 0o600, dir_fd=parent_fd)
            except OSError as exc:
                source.close()
                os.close(parent_fd)
                raise ArchiveError(f"cannot create extraction file safely: {member.name!r}: {exc}") from exc
            try:
                with source, os.fdopen(fd, "wb", closefd=False) as output:
                    _copy_exact(source, output, member.size)
                    output.flush()
                os.fchmod(fd, member.mode & 0o777)
            finally:
                os.close(fd)
                os.close(parent_fd)

        # Apply archived directory modes only after all children are written.
        for parts in sorted(directory_paths, key=lambda value: (len(value), value), reverse=True):
            directory_fd = _open_relative_directory(root_fd, parts)
            try:
                os.fchmod(directory_fd, explicit_directory_modes.get(parts, 0o755))
            finally:
                os.close(directory_fd)
    finally:
        os.close(root_fd)


def process_archive(
    archive: Path,
    policy_name: str,
    expected_sha256: str | None,
    destination: Path | None,
    read_member: str | None,
) -> bytes | None:
    if destination is not None and expected_sha256 is None:
        raise ArchiveError("expected SHA-256 is required for extraction")
    policy = POLICIES[policy_name]
    handle, opened_stat = _open_stable(archive)
    with handle:
        actual_digest, compressed_size = _hash_handle(handle, policy.max_compressed)
        if expected_sha256 and actual_digest != expected_sha256.lower():
            raise ArchiveError("archive SHA-256 mismatch")
        current = os.fstat(handle.fileno())
        if not _same_identity(current, opened_stat):
            raise ArchiveError("archive identity changed while hashing")
        preflight_raw_gzip_tar(handle, policy, compressed_size)
        if not _same_identity(os.fstat(handle.fileno()), opened_stat):
            raise ArchiveError("archive identity changed during raw preflight")
        handle.seek(0)
        try:
            tf = tarfile.open(fileobj=handle, mode="r:gz")
        except (OSError, tarfile.TarError) as exc:
            raise ArchiveError(f"invalid gzip tar: {exc}") from exc
        data = None
        with tf:
            entries, total_size = inspect_open_archive(tf, policy_name, compressed_size)
            if destination is not None:
                extract_inspected(tf, entries, destination, total_size)
            if read_member is not None:
                requested = validated_name(read_member)
                matches = [member for member, parts in entries if parts == requested and member.isreg()]
                if len(matches) != 1:
                    raise ArchiveError(f"required regular member not found exactly once: {read_member}")
                source = tf.extractfile(matches[0])
                if source is None:
                    raise ArchiveError(f"regular member has no data: {read_member}")
                with source:
                    data = source.read(matches[0].size + 1)
                if len(data) != matches[0].size:
                    raise ArchiveError(f"member size mismatch: {read_member}")
        if not _same_identity(os.fstat(handle.fileno()), opened_stat):
            raise ArchiveError("archive identity changed while inspecting or extracting")
        return data


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--policy", choices=tuple(POLICIES), required=True)
    parser.add_argument("--expected-sha256")
    parser.add_argument("--extract-to", type=Path)
    parser.add_argument("--read-member")
    parser.add_argument("archive", type=Path)
    args = parser.parse_args()
    if args.expected_sha256 and not (
        len(args.expected_sha256) == 64 and all(c in "0123456789abcdefABCDEF" for c in args.expected_sha256)
    ):
        print("safe-tar: invalid expected SHA-256", file=sys.stderr)
        return 1
    try:
        data = process_archive(
            args.archive,
            args.policy,
            args.expected_sha256,
            args.extract_to,
            args.read_member,
        )
        if data is not None:
            sys.stdout.buffer.write(data)
    except (ArchiveError, OSError, tarfile.TarError, json.JSONDecodeError) as exc:
        print(f"safe-tar: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

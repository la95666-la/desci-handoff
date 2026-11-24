#!/usr/bin/env python3
from __future__ import annotations

import importlib.util
import io
import gzip
import os
from pathlib import Path
import shutil
import stat
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[2]
HELPER = ROOT / "scripts/release/lib/safe_tar.py"
spec = importlib.util.spec_from_file_location("safe_tar", HELPER)
assert spec and spec.loader
safe_tar = importlib.util.module_from_spec(spec)
__import__("sys").modules[spec.name] = safe_tar
spec.loader.exec_module(safe_tar)


def archive(path: Path, members, fmt=tarfile.USTAR_FORMAT):
    with tarfile.open(path, "w:gz", format=fmt) as tf:
        for name, kind, data, declared in members:
            item = tarfile.TarInfo(name)
            item.mode = 0o644
            if kind == "dir":
                item.type = tarfile.DIRTYPE
                item.size = 0
                tf.addfile(item)
            elif kind == "file":
                item.size = len(data) if declared is None else declared
                tf.addfile(item, io.BytesIO(data))
            elif kind == "link":
                item.type = tarfile.SYMTYPE
                item.linkname = "dist/index.html"
                tf.addfile(item)


def raw_header(name: bytes, size: int, typeflag: bytes) -> bytes:
    block = bytearray(512)
    block[0 : len(name)] = name
    block[100:108] = b"0000644\0"
    block[108:116] = b"0000000\0"
    block[116:124] = b"0000000\0"
    block[124:136] = f"{size:011o}\0".encode()
    block[136:148] = b"00000000000\0"
    block[148:156] = b"        "
    block[156:157] = typeflag
    block[257:263] = b"ustar\0"
    block[263:265] = b"00"
    block[148:156] = f"{sum(block):06o}\0 ".encode()
    return bytes(block)


def raw_extended_archive(path: Path, typeflag: bytes, payload_size: int) -> None:
    header = raw_header(b"metadata", payload_size, typeflag)
    # Highly compressible metadata intentionally dwarfs the gzip input. Preflight
    # must reject from the header before allocating/feeding this payload to tarfile.
    raw = header + (b"A" * payload_size)
    raw += b"\0" * ((512 - payload_size % 512) % 512)
    raw += b"\0" * 1024
    with gzip.open(path, "wb", compresslevel=9) as stream:
        stream.write(raw)


def rejected(path: Path, policy="static", expected: str | None = None):
    try:
        safe_tar.process_archive(path, policy, None, None, None)
    except safe_tar.ArchiveError as exc:
        if expected is not None:
            assert expected in str(exc), f"expected {expected!r}, got {exc!r}"
        return
    raise AssertionError(f"archive unexpectedly accepted: {path}")


def runtime_members():
    directories = [("/".join(parts), "dir", b"", None) for parts in sorted(safe_tar.RUNTIME_DIRECTORIES)]
    exact_files = [("/".join(parts), "file", b"x", None) for parts in sorted(safe_tar.RUNTIME_EXACT_FILES)]
    patterned_files = [
        ("images/joincare-backend-release-x.tar.gz", "file", b"x", None),
        ("images/joincare-backend-release-x.tar.gz.sha256", "file", b"x", None),
        ("packages/joincare-frontend-release-x.tar.gz", "file", b"x", None),
        ("packages/joincare-frontend-release-x.tar.gz.sha256", "file", b"x", None),
        ("packages/joincare-admin-release-x.tar.gz", "file", b"x", None),
        ("packages/joincare-admin-release-x.tar.gz.sha256", "file", b"x", None),
    ]
    return directories + exact_files + patterned_files


with tempfile.TemporaryDirectory() as td:
    root = Path(td)
    valid = root / "valid.tar.gz"
    archive(valid, [("dist", "dir", b"", None), ("dist/index.html", "file", b"ok", None)])
    digest = __import__("hashlib").sha256(valid.read_bytes()).hexdigest()
    out = root / "out"
    if os.name == "nt":
        out.mkdir()
        try:
            safe_tar.process_archive(valid, "static", digest, out, None)
        except safe_tar.ArchiveError as exc:
            assert "native_nofollow_unavailable" in str(exc)
        else:
            raise AssertionError("Windows extraction did not fail closed")
        assert list(out.iterdir()) == []
    else:
        safe_tar.process_archive(valid, "static", digest, out, None)
        assert (out / "dist/index.html").read_bytes() == b"ok"
        assert stat.S_IMODE((out / "dist/index.html").stat().st_mode) == 0o644

    unbound = root / "unbound"
    try:
        safe_tar.process_archive(valid, "static", None, unbound, None)
    except safe_tar.ArchiveError as exc:
        assert "expected SHA-256" in str(exc)
    else:
        raise AssertionError("extraction without an expected digest was accepted")

    wrong_digest = root / "wrong-digest"
    try:
        safe_tar.process_archive(valid, "static", "0" * 64, wrong_digest, None)
    except safe_tar.ArchiveError as exc:
        assert "SHA-256 mismatch" in str(exc)
    else:
        raise AssertionError("extraction with a wrong expected digest was accepted")

    if os.name != "nt":
        nonempty = root / "nonempty"
        nonempty.mkdir()
        (nonempty / "sentinel").write_text("keep", encoding="utf-8")
        try:
            safe_tar.process_archive(valid, "static", digest, nonempty, None)
        except safe_tar.ArchiveError as exc:
            assert "must be empty" in str(exc)
        else:
            raise AssertionError("extraction into a non-empty directory was accepted")
        assert (nonempty / "sentinel").read_text(encoding="utf-8") == "keep"

    for name in ["C:/escape", "C:escape", "//server/share", r"dist\\escape", "/root", "dist/../x"]:
        bad = root / (str(abs(hash(name))) + ".tar.gz")
        archive(bad, [(name, "file", b"x", None)])
        rejected(bad)

    runtime = root / "runtime.tar.gz"
    archive(runtime, runtime_members())
    safe_tar.process_archive(runtime, "runtime", None, None, None)

    runtime_missing = root / "runtime-missing.tar.gz"
    archive(runtime_missing, [member for member in runtime_members() if member[0] != "README-DEPLOY.md"])
    rejected(runtime_missing, "runtime")

    runtime_duplicate_pattern = root / "runtime-duplicate-pattern.tar.gz"
    archive(runtime_duplicate_pattern, runtime_members() + [
        ("packages/joincare-frontend-release-y.tar.gz", "file", b"x", None),
    ])
    rejected(runtime_duplicate_pattern, "runtime")

    runtime_unexpected = root / "runtime-unexpected.tar.gz"
    archive(runtime_unexpected, runtime_members() + [("unexpected", "file", b"x", None)])
    rejected(runtime_unexpected, "runtime")

    pax = root / "pax.tar.gz"
    with tarfile.open(pax, "w:gz", format=tarfile.PAX_FORMAT) as tf:
        item = tarfile.TarInfo("dist/index.html")
        item.size = 2
        item.pax_headers = {"comment": "unsupported"}
        tf.addfile(item, io.BytesIO(b"ok"))
    rejected(pax)

    for flag, label in [(b"x", "large-pax"), (b"g", "large-global-pax"), (b"L", "large-longname"), (b"K", "large-longlink"), (b"S", "large-sparse")]:
        extended = root / f"{label}.tar.gz"
        raw_extended_archive(extended, flag, 8 * 1024 * 1024)
        assert extended.stat().st_size < 64 * 1024
        # The payload would exceed the expansion-ratio bound if consumed. Seeing
        # the type-flag error proves rejection occurred immediately at its header.
        rejected(extended, expected="extended type flag")

    bad_checksum = root / "bad-checksum.tar.gz"
    raw = bytearray(raw_header(b"dist/index.html", 2, b"0") + b"ok" + b"\0" * 510 + b"\0" * 1024)
    raw[148] ^= 1
    with gzip.open(bad_checksum, "wb") as stream:
        stream.write(raw)
    rejected(bad_checksum)

    base256_size = root / "base256-size.tar.gz"
    header = bytearray(raw_header(b"dist/index.html", 0, b"0"))
    header[124] = 0x80
    header[148:156] = b"        "
    header[148:156] = f"{sum(header):06o}\0 ".encode()
    with gzip.open(base256_size, "wb") as stream:
        stream.write(header + b"\0" * 1024)
    rejected(base256_size)

    link = root / "link.tar.gz"
    archive(link, [("dist", "dir", b"", None), ("dist/index.html", "link", b"", None)])
    rejected(link)

    original = safe_tar.POLICIES["static"]
    try:
        safe_tar.POLICIES["static"] = safe_tar.Policy(original.max_compressed, 2, original.max_file, original.max_total, original.max_ratio)
        count = root / "count.tar.gz"
        archive(count, [("dist", "dir", b"", None), ("dist/index.html", "file", b"ok", None), ("dist/a", "file", b"x", None)])
        rejected(count)

        safe_tar.POLICIES["static"] = safe_tar.Policy(original.max_compressed, original.max_members, 1, original.max_total, original.max_ratio)
        size = root / "size.tar.gz"
        archive(size, [("dist", "dir", b"", None), ("dist/index.html", "file", b"ok", None)])
        rejected(size)

        safe_tar.POLICIES["static"] = safe_tar.Policy(1, original.max_members, original.max_file, original.max_total, original.max_ratio)
        rejected(valid)
    finally:
        safe_tar.POLICIES["static"] = original

    if os.name != "nt":
        symlink_out = root / "symlink-out"
        real_out = root / "real-out"
        real_out.mkdir()
        symlink_out.symlink_to(real_out, target_is_directory=True)
        try:
            safe_tar.process_archive(valid, "static", digest, symlink_out, None)
        except safe_tar.ArchiveError:
            pass
        else:
            raise AssertionError("symlink extraction root was accepted")
        assert list(real_out.iterdir()) == []

        swap = root / "swap.tar.gz"
        shutil.copy2(valid, swap)
        handle, opened = safe_tar._open_stable(swap)
        replacement = root / "replacement.tar.gz"
        archive(replacement, [("dist", "dir", b"", None), ("dist/index.html", "file", b"evil", None)])
        replacement.replace(swap)
        with handle:
            got, _ = safe_tar._hash_handle(handle, original.max_compressed)
        assert got == digest, "stable descriptor followed path replacement"

print("safe_tar adversarial policy: PASS")

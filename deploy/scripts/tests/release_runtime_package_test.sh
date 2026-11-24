#!/usr/bin/env bash
# release_runtime_package_test.sh — runtime-only 发布链路的快速策略测试
# 不执行完整构建；覆盖 profile 校验、包内容策略、bundle 门禁、ELF 门禁。
# 运行环境：bash + node（与 build 脚本同依赖）。
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/release/lib/release_common.sh
source "$ROOT/scripts/release/lib/release_common.sh"

PASS=0
FAIL=0
note() { printf '  %s\n' "$*"; }
passed() { PASS=$((PASS + 1)); printf 'PASS %s\n' "$*"; }
failed() { FAIL=$((FAIL + 1)); printf 'FAIL %s\n' "$*" >&2; }

expect_rl_die() {
    # 断言函数以 rl_die（exit 1）失败，且错误信息包含 $2
    local fn="$1" want="$2" out
    out="$(set +e; eval "$fn" 2>&1)"
    if [ "$?" -eq 0 ]; then
        failed "expected failure: $fn"
        return 1
    fi
    case "$out" in
        *"$want"*) passed "rejects: ${want}" ;;
        *) failed "error message mismatch — want '$want' got: $out" ;;
    esac
}

echo "== 0. deploy provenance fail-closed gate =="
DEPLOY_HEAD="$(git -C "$ROOT" rev-parse HEAD)"
grep -Fq 'FAIR_LAUNCH_REVIEWED_DEPLOY_REF' "$ROOT/scripts/release/build-test-runtime.sh" && passed "builder requires reviewed deploy ref" || failed "builder missing deploy provenance gate"
grep -Fq 'UNADMITTED_OFFLINE_EVIDENCE_ONLY' "$ROOT/scripts/release/build-test-runtime.sh" "$ROOT/scripts/release/verify-test-runtime.sh" && passed "manifest is explicitly unadmitted evidence" || failed "manifest admission label missing"

echo "== 1. profile fail-closed 校验 =="
# 正确 profile（REOWN 用格式合法的假 ID；构建器只验格式）
export VITE_REOWN_PROJECT_ID="0123456789abcdef0123456789abcdef"
export FAIR_LAUNCH_REVIEWED_DEPLOY_REF="$DEPLOY_HEAD"
( set -e
  # shellcheck disable=SC1090
  source "$ROOT/scripts/release/profiles/test-server-mainnet-checkin.env"
  validate_release_profile "test-server-mainnet-checkin"
) && passed "valid profile accepted" || failed "valid profile rejected"

# 各类畸形输入必须被拒
expect_rl_die 'validate_release_profile ""' "profile name is required"
expect_rl_die 'validate_release_profile "../evil"' "path separators"
( set -e
  RELEASE_PROFILE=x VITE_CHECKIN_ADDRESS= VITE_CHECKIN_CHAIN_ID=56 VITE_HARDHAT_RPC= VITE_TRUSTED_PROJECT_CHAINS=97 \
      validate_release_profile x
) >/dev/null 2>&1 && failed "empty address accepted" || passed "rejects: empty address"
( set -e
  RELEASE_PROFILE=x VITE_CHECKIN_ADDRESS=0x1234 VITE_CHECKIN_CHAIN_ID=56 VITE_HARDHAT_RPC= VITE_TRUSTED_PROJECT_CHAINS=97 \
      validate_release_profile x
) >/dev/null 2>&1 && failed "short address accepted" || passed "rejects: malformed address"
( set -e
  RELEASE_PROFILE=x VITE_CHECKIN_ADDRESS=0xE029161be55922edf3eC9D222142edf057d196ee VITE_CHECKIN_CHAIN_ID=97 VITE_HARDHAT_RPC= VITE_TRUSTED_PROJECT_CHAINS=97 \
      validate_release_profile x
) >/dev/null 2>&1 && failed "chain 97 accepted" || passed "rejects: non-56 checkin chain"
( set -e
  RELEASE_PROFILE=x VITE_CHECKIN_ADDRESS=0xE029161be55922edf3eC9D222142edf057d196ee VITE_CHECKIN_CHAIN_ID=56 VITE_HARDHAT_RPC=http://127.0.0.1:8545 VITE_TRUSTED_PROJECT_CHAINS=97 \
      validate_release_profile x
) >/dev/null 2>&1 && failed "hardhat rpc accepted" || passed "rejects: hardhat rpc"
( set -e
  RELEASE_PROFILE=x VITE_CHECKIN_ADDRESS=0xE029161be55922edf3eC9D222142edf057d196ee VITE_CHECKIN_CHAIN_ID=56 VITE_HARDHAT_RPC= VITE_TRUSTED_PROJECT_CHAINS=31337 \
      validate_release_profile x
) >/dev/null 2>&1 && failed "31337 trusted accepted" || passed "rejects: hardhat in trusted chains"
( set -e
  RELEASE_PROFILE=x VITE_CHECKIN_ADDRESS=0xE029161be55922edf3eC9D222142edf057d196ee VITE_CHECKIN_CHAIN_ID=56 VITE_HARDHAT_RPC= VITE_TRUSTED_PROJECT_CHAINS=97 \
      VITE_REOWN_PROJECT_ID=placeholder-no-walletconnect validate_release_profile x
) >/dev/null 2>&1 && failed "placeholder reown accepted" || passed "rejects: placeholder reown id"
( set -e
  RELEASE_PROFILE=x VITE_CHECKIN_ADDRESS=0xE029161be55922edf3eC9D222142edf057d196ee VITE_CHECKIN_CHAIN_ID=56 VITE_HARDHAT_RPC= VITE_TRUSTED_PROJECT_CHAINS=97 \
      unset VITE_REOWN_PROJECT_ID; validate_release_profile x
) >/dev/null 2>&1 && failed "missing reown accepted" || passed "rejects: missing reown id"

echo "== 2. 包内容策略（合成目录） =="
GOOD="$(mktemp -d)"
BAD="$(mktemp -d)"
cleanup() { rm -rf "${GOOD:-}" "${BAD:-}" "${BD:-}" "${IMGDIR:-}" "${IMGS:-}" "${IMG3:-}" "${HANDOFF_DIR:-}"; }
trap cleanup EXIT

mkdir -p "$GOOD/artifacts/backend" "$GOOD/deploy" "$GOOD/images" "$GOOD/packages" "$GOOD/scripts/release/lib"
: > "$GOOD/artifacts/backend/joincare-core"
: > "$GOOD/deploy/Dockerfile.backend.runtime"
: > "$GOOD/release-manifest.json"
: > "$GOOD/SHA256SUMS"
: > "$GOOD/README-DEPLOY.md"
cp "$ROOT/scripts/release/lib/safe_tar.py" "$GOOD/scripts/release/lib/safe_tar.py"
# 合成完整最小 docker-save 镜像闭包（manifest + config JSON + layer）。
IMGDIR="$(mktemp -d)"
mkdir -p "$IMGDIR/docker/layer"
printf '{}' > "$IMGDIR/docker/config.json"
printf 'layer' > "$IMGDIR/docker/layer/layer.tar"
printf '[{"Config":"config.json","RepoTags":["joincare/backend:test"],"Layers":["layer/layer.tar"]}]' > "$IMGDIR/docker/manifest.json"
tar -czf "$GOOD/images/joincare-backend-release-test.tar.gz" -C "$IMGDIR/docker" manifest.json config.json layer/layer.tar
(cd "$GOOD/images" && sha256sum joincare-backend-release-test.tar.gz > joincare-backend-release-test.tar.gz.sha256)
# 合成 frontend / admin 静态包（顶层 dist/index.html）
for comp in frontend admin; do
    mkdir -p "$IMGDIR/$comp/dist"
    : > "$IMGDIR/$comp/dist/index.html"
    tar -czf "$GOOD/packages/joincare-$comp-release-test.tar.gz" -C "$IMGDIR/$comp" dist
    (cd "$GOOD/packages" && sha256sum "joincare-$comp-release-test.tar.gz" > "joincare-$comp-release-test.tar.gz.sha256")
done
trap cleanup EXIT
if ( verify_package_contents "$GOOD" ) 2>/dev/null; then
    passed "clean package passes allowlist"
else
    failed "clean package rejected by allowlist"
fi

# 越界文件（BAD 已存在，rm 后整体复制）
rm -rf "$BAD"; cp -r "$GOOD" "$BAD"
mkdir -p "$BAD/scripts"
: > "$BAD/scripts/deployCheckIn.js"
if ( verify_package_contents "$BAD" ) 2>/dev/null; then
    failed "out-of-allowlist script accepted"
else
    passed "rejects: file outside allowlist"
fi

# allowlist 内但命中 denylist 的文件名
rm "$BAD/scripts/deployCheckIn.js"
: > "$BAD/packages/hardhat-config.js"
if ( verify_package_contents "$BAD" ) 2>/dev/null; then
    failed "denylist hit accepted"
else
    passed "rejects: hardhat-named asset inside dist"
fi
rm "$BAD/packages/hardhat-config.js"
: > "$BAD/artifacts/backend/secret.pem"
if ( verify_package_contents "$BAD" ) 2>/dev/null; then
    failed "pem accepted"
else
    passed "rejects: pem key material"
fi
rm "$BAD/artifacts/backend/secret.pem"
mkdir -p "$BAD/config/secret"
: > "$BAD/config/secret/config.yaml"
if ( verify_package_contents "$BAD" ) 2>/dev/null; then
    failed "secret config accepted"
else
    passed "rejects: secret config path"
fi

echo "== 3. bundle 门禁（合成 dist） =="
BD="$(mktemp -d)"
export VITE_CHECKIN_ADDRESS="0xE029161be55922edf3eC9D222142edf057d196ee" VITE_CHECKIN_CHAIN_ID="56" VITE_TRUSTED_PROJECT_CHAINS="97" VITE_ENVIRONMENT="prod"
mkdir -p "$BD/dist"
printf '<html></html>' > "$BD/dist/index.html"
printf 'var env={VITE_CHECKIN_ADDRESS:"0xE029161be55922edf3eC9D222142edf057d196ee",VITE_CHECKIN_CHAIN_ID:"56",VITE_ENVIRONMENT:"prod",VITE_TRUSTED_PROJECT_CHAINS:"97",VITE_HARDHAT_RPC:""}' > "$BD/dist/app.js"
if ( verify_frontend_bundle "$BD/dist" ) 2>/dev/null; then
    passed "well-formed bundle passes"
else
    failed "well-formed bundle rejected"
fi
printf 'x placeholder-no-walletconnect x' >> "$BD/dist/app.js"
if ( verify_frontend_bundle "$BD/dist" ) 2>/dev/null; then
    failed "placeholder accepted"
else
    passed "rejects: placeholder walletconnect id"
fi
sed -i 's/placeholder-no-walletconnect//' "$BD/dist/app.js"
sed -i 's/VITE_ENVIRONMENT:"prod"/VITE_ENVIRONMENT:"local"/' "$BD/dist/app.js"
if ( verify_frontend_bundle "$BD/dist" ) 2>/dev/null; then
    failed "local wallet build accepted as production"
else
    passed "rejects: local wallet build in production"
fi
sed -i 's/VITE_ENVIRONMENT:"local"/VITE_ENVIRONMENT:"prod"/' "$BD/dist/app.js"
printf 'x testnet-rpc.gochain.io x' >> "$BD/dist/app.js"
if ( verify_frontend_bundle "$BD/dist" ) 2>/dev/null; then
    failed "GoChain testnet RPC accepted in production"
else
    passed "rejects: GoChain testnet RPC in production"
fi
sed -i 's/testnet-rpc.gochain.io//' "$BD/dist/app.js"
printf 'var env2={VITE_HARDHAT_RPC:"http://127.0.0.1:8545"}' >> "$BD/dist/app.js"
if ( verify_frontend_bundle "$BD/dist" ) 2>/dev/null; then
    failed "non-empty hardhat rpc accepted"
else
    passed "rejects: non-empty VITE_HARDHAT_RPC"
fi
sed -i 's|VITE_HARDHAT_RPC:"http://127.0.0.1:8545"||' "$BD/dist/app.js"
sed -i 's/VITE_CHECKIN_CHAIN_ID:"56"/VITE_CHECKIN_CHAIN_ID:"31337"/' "$BD/dist/app.js"
if ( verify_frontend_bundle "$BD/dist" ) 2>/dev/null; then
    failed "wrong chain marker accepted"
else
    passed "rejects: chain identity marker drift"
fi

echo "== 4. backend ELF 门禁 =="
node -e 'const fs=require("fs");const b=Buffer.alloc(20);b[0]=0x7f;b[1]=0x45;b[2]=0x4c;b[3]=0x46;b[4]=0x02;b[5]=0x01;b[6]=0x01;b[18]=0x3e;fs.writeFileSync(process.argv[1],b)' "$BD/joincare-core"
node -e 'require("fs").writeFileSync(process.argv[1],Buffer.from([0x4d,0x5a,0x90,0x00]))' "$BD/joincare-core.exe"
if ( verify_backend_binary "$BD/joincare-core" ) 2>/dev/null; then
    passed "ELF header accepted"
else
    failed "valid ELF rejected"
fi
if ( verify_backend_binary "$BD/joincare-core.exe" ) 2>/dev/null; then
    failed "PE executable accepted"
else
    passed "rejects: non-ELF artifact"
fi

echo "== 5. 脚本语法与实际 handoff 输出 =="
for sh in "$ROOT/scripts/release/build-test-runtime.sh" "$ROOT/scripts/release/verify-test-runtime.sh" "$ROOT/scripts/release/lib/release_common.sh" "$ROOT/scripts/release/render-agent-handoff.sh"; do
    if bash -n "$sh"; then passed "syntax: $(basename "$sh")"; else failed "syntax: $sh"; fi
done
HANDOFF_DIR="$(mktemp -d)"
export PROFILE=test DATE_TAG=20260908 VITE_CHECKIN_CHAIN_ID=56 VITE_CHECKIN_ADDRESS=0x0000000000000000000000000000000000000001 VITE_TRUSTED_PROJECT_CHAINS=56
export BACKEND_COMMIT=1111111111111111111111111111111111111111 FRONTEND_COMMIT=2222222222222222222222222222222222222222 ADMIN_COMMIT=3333333333333333333333333333333333333333
export AGENT_DIR=/archive BACKEND_IMAGE_FILE=backend.tar.gz BACKEND_IMG_PUB_SHA="$(printf 'a%.0s' {1..64})" BACKEND_IMAGE_SIZE=1 BACKEND_IMAGE_TAG=joincare/backend:archive
export FRONTEND_PACKAGE_FILE=frontend.tar.gz FRONTEND_PKG_PUB_SHA="$(printf 'b%.0s' {1..64})" FRONTEND_PACKAGE_SIZE=1
export ADMIN_PACKAGE_FILE=admin.tar.gz ADMIN_PKG_PUB_SHA="$(printf 'c%.0s' {1..64})" ADMIN_PACKAGE_SIZE=1
if bash "$ROOT/scripts/release/render-agent-handoff.sh" "$HANDOFF_DIR/AGENT-HANDOFF.md" \
    && grep -Fq 'fair_launch_activation_unavailable' "$HANDOFF_DIR/AGENT-HANDOFF.md" \
    && grep -Fq '仅归档和离线校验' "$HANDOFF_DIR/AGENT-HANDOFF.md" \
    && ! grep -Eqi 'docker[[:space:]]+(load|start|restart)[^#]*(backend|BACKEND)|compose[^#]*(up|start|restart|recreate)[^#]*(backend|BACKEND)|重打标|retag' "$HANDOFF_DIR/AGENT-HANDOFF.md"; then
    passed "rendered handoff keeps Backend archive-only"
else
    failed "rendered handoff contains Backend activation guidance"
fi
rm -rf "$HANDOFF_DIR"; HANDOFF_DIR=""

printf '\nrelease runtime package policy: %s (pass=%d fail=%d)\n' \
    "$([ "$FAIL" = 0 ] && echo PASS || echo FAIL)" "$PASS" "$FAIL"
[ "$FAIL" = 0 ]

echo "== 6. backend docker-save 镜像门禁 =="
IMGS="$(mktemp -d)"
trap cleanup EXIT
mkdir -p "$IMGS/images"
# 正常最小 docker-save 镜像闭包
mkdir -p "$IMGS/one/layer"
printf '{}' > "$IMGS/one/config.json"
printf 'layer' > "$IMGS/one/layer/layer.tar"
printf '[{"Config":"config.json","RepoTags":["joincare/backend:release-x"],"Layers":["layer/layer.tar"]}]' > "$IMGS/one/manifest.json"
tar -czf "$IMGS/images/joincare-backend-release-x.tar.gz" -C "$IMGS/one" manifest.json config.json layer/layer.tar
(cd "$IMGS/images" && sha256sum joincare-backend-release-x.tar.gz > joincare-backend-release-x.tar.gz.sha256)
out="$( verify_backend_image_package "$IMGS/images" 2>/dev/null )"
if [ "$out" = "joincare-backend-release-x.tar.gz" ]; then
    passed "valid docker-save image accepted"
else
    failed "valid docker-save image rejected"
fi
# 缺 sidecar
rm "$IMGS/images/joincare-backend-release-x.tar.gz.sha256"
if ( verify_backend_image_package "$IMGS/images" ) 2>/dev/null; then
    failed "missing sha256 sidecar accepted"
else
    passed "rejects: missing image sha256 sidecar"
fi
(cd "$IMGS/images" && sha256sum joincare-backend-release-x.tar.gz > joincare-backend-release-x.tar.gz.sha256)
# manifest RepoTag 非平台命名空间
printf '[{"Config":"config.json","RepoTags":["other/x:1"],"Layers":["layer/layer.tar"]}]' > "$IMGS/one/manifest.json"
tar -czf "$IMGS/images/joincare-backend-release-x.tar.gz" -C "$IMGS/one" manifest.json config.json layer/layer.tar
if ( verify_backend_image_package "$IMGS/images" ) 2>/dev/null; then
    failed "foreign RepoTag accepted"
else
    passed "rejects: image without joincare/backend RepoTag"
fi
# 两个镜像
cp "$IMGS/images/joincare-backend-release-x.tar.gz" "$IMGS/images/joincare-backend-release-y.tar.gz"
(cd "$IMGS/images" && sha256sum joincare-backend-release-y.tar.gz > joincare-backend-release-y.tar.gz.sha256)
if ( verify_backend_image_package "$IMGS/images" ) 2>/dev/null; then
    failed "multiple images accepted"
else
    passed "rejects: more than one backend image"
fi
rm "$IMGS/images/joincare-backend-release-y.tar.gz" "$IMGS/images/joincare-backend-release-y.tar.gz.sha256"
# 越界镜像名（allowlist 外）
mv "$IMGS/images/joincare-backend-release-x.tar.gz" "$IMGS/images/evil-image.tar.gz"
mv "$IMGS/images/joincare-backend-release-x.tar.gz.sha256" "$IMGS/images/evil-image.tar.gz.sha256"
if ( verify_package_contents "$IMGS" ) 2>/dev/null; then
    failed "arbitrary image name accepted"
else
    passed "rejects: image name outside allowlist"
fi

echo "== 7. frontend / admin static archive security =="
IMG3="$(mktemp -d)"
trap cleanup EXIT
mkdir -p "$IMG3/packages"
make_static_archive() {
    local archive="$1" kind="$2"
    release_python - "$archive" "$kind" <<'PY'
import io, sys, tarfile
archive, kind = sys.argv[1:]
with tarfile.open(archive, "w:gz", format=tarfile.PAX_FORMAT) as tf:
    def directory(name):
        item = tarfile.TarInfo(name); item.type = tarfile.DIRTYPE; item.mode = 0o755; tf.addfile(item)
    def regular(name, data=b""):
        item = tarfile.TarInfo(name); item.size = len(data); item.mode = 0o644; tf.addfile(item, io.BytesIO(data))
    if kind not in {"absolute", "traversal", "unexpected", "backslash"}:
        directory("dist")
    if kind == "valid": regular("dist/index.html", b"ok")
    elif kind == "substring": regular("dist/index.html.evil", b"bad")
    elif kind == "absolute": regular("/dist/index.html", b"bad")
    elif kind == "traversal": regular("dist/../escape", b"bad")
    elif kind == "unexpected": regular("webroot/index.html", b"bad")
    elif kind == "backslash": regular(r"dist\\index.html", b"bad")
    elif kind in {"symlink", "hardlink", "fifo", "device"}:
        item = tarfile.TarInfo("dist/index.html")
        item.type = {"symlink": tarfile.SYMTYPE, "hardlink": tarfile.LNKTYPE,
                     "fifo": tarfile.FIFOTYPE, "device": tarfile.CHRTYPE}[kind]
        item.linkname = "dist/target" if kind in {"symlink", "hardlink"} else ""
        if kind == "device": item.devmajor = 1; item.devminor = 3
        tf.addfile(item)
    elif kind == "duplicate":
        regular("dist/index.html", b"one"); regular("dist/index.html", b"two")
    elif kind == "conflict":
        regular("dist/file", b"ancestor"); regular("dist/file/child", b"bad")
    else: raise SystemExit("unknown fixture")
PY
}
write_sidecar() {
    local archive="$1" base
    base="$(basename "$archive")"
    (cd "$(dirname "$archive")" && sha256sum "$base" > "$base.sha256")
}
for comp in frontend admin; do
    archive="$IMG3/packages/joincare-$comp-release-x.tar.gz"
    make_static_archive "$archive" valid; write_sidecar "$archive"
    out="$(verify_static_package "$IMG3/packages" "$comp" 2>/dev/null)"
    [ "$out" = "$(basename "$archive")" ] && passed "valid $comp strict static archive accepted" || failed "valid $comp static archive rejected"
done

archive="$IMG3/packages/joincare-frontend-release-x.tar.gz"
for kind in substring traversal absolute symlink hardlink fifo device unexpected duplicate conflict backslash; do
    make_static_archive "$archive" "$kind"; write_sidecar "$archive"
    if ( verify_static_package "$IMG3/packages" frontend ) 2>/dev/null; then
        failed "$kind static archive accepted"
    else
        passed "rejects: $kind static archive"
    fi
done
make_static_archive "$archive" valid
printf '%064d  %s\n' 0 "$(basename "$archive")" > "$archive.sha256"
if ( verify_static_package "$IMG3/packages" frontend ) 2>/dev/null; then failed "wrong sidecar digest accepted"; else passed "rejects: wrong sidecar digest"; fi
write_sidecar "$archive"
sed -i 's/joincare-frontend-release-x.tar.gz/other.tar.gz/' "$archive.sha256"
if ( verify_static_package "$IMG3/packages" frontend ) 2>/dev/null; then failed "sidecar basename mismatch accepted"; else passed "rejects: sidecar basename mismatch"; fi
rm "$archive.sha256"
if ( verify_static_package "$IMG3/packages" frontend ) 2>/dev/null; then failed "missing static sidecar accepted"; else passed "rejects: missing static sidecar"; fi
write_sidecar "$archive"
if ( verify_static_package "$IMG3/packages" tgbot ) 2>/dev/null; then failed "unsupported static component accepted"; else passed "rejects: unsupported static component"; fi

printf '\nrelease runtime package policy final: %s (pass=%d fail=%d)\n' \
    "$([ "$FAIL" = 0 ] && echo PASS || echo FAIL)" "$PASS" "$FAIL"
[ "$FAIL" = 0 ]

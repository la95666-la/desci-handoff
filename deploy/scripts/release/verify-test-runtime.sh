#!/usr/bin/env bash
# verify-test-runtime.sh — 独立校验一个 runtime-only 发布包
#
# 输入：.tar.gz 及其严格相邻 <tarball>.sha256（校验后安全解包到临时目录）
# 校验项：
#   1. 外层 archive：仅规范相对 regular file/directory，无 link/device/FIFO/重复冲突
#   2. 完整性：SHA256SUMS 逐文件核对
#   3. 内容策略：allowlist（每个文件必须在允许清单内）+ denylist（禁止片段）
#   4. manifest：结构、链身份字段
#   5. 静态包：严格 sidecar + 安全 tar member；前端 bundle 链身份门禁
#   6. backend 产物：linux amd64 ELF
#
# 用法：bash verify-test-runtime.sh <release-tarball.tar.gz> [--profile <name>]
#   --profile 缺省用 manifest 里的 profile；显式指定时必须一致
set -euo pipefail

VERIFY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/release/lib/release_common.sh
source "$VERIFY_DIR/lib/release_common.sh"

TARGET=""
EXPECT_PROFILE=""
while [ $# -gt 0 ]; do
    case "$1" in
        --profile) EXPECT_PROFILE="${2:?}"; shift 2 ;;
        -*) rl_die "unknown option: $1" ;;
        *) [ -z "$TARGET" ] && TARGET="$1" || rl_die "multiple targets given"; shift ;;
    esac
done
[ -n "$TARGET" ] || rl_die "usage: verify-test-runtime.sh <release-tarball.tar.gz> [--profile <name>]"

TMP=""
cleanup() { [ -z "$TMP" ] || rm -rf "$TMP"; return 0; }
trap cleanup EXIT

SAFE_TAR_HELPER="$(release_safe_tar_helper)"
[ -f "$SAFE_TAR_HELPER" ] || rl_die "trusted repository safe archive validator unavailable"
if [ ! -f "$TARGET" ] || [[ "$TARGET" != *.tar.gz ]]; then
    rl_die "verifier accepts only a .tar.gz with an adjacent .sha256 sidecar: $TARGET"
fi
TARGET_DIR="$(cd "$(dirname "$TARGET")" && pwd)"
TARGET_BASE="$(basename "$TARGET")"
OUTER_DIGEST="$(verify_package_sidecar_digest "$TARGET_DIR" "$TARGET_BASE")" \
    || rl_die "outer release tarball sidecar verification failed: $TARGET_BASE"
TMP="$(mktemp -d)"
# The external adjacent sidecar is checked before extraction. The trusted helper
# rechecks that digest while inspecting and extracting from one stable handle.
safe_extract_release_tar "$TARGET" "$TMP" runtime "$OUTER_DIGEST"
DIR="$TMP"

v_ok() { printf ' ✓ %s\n' "$*"; }

# ---- 1. checksum + exact inventory ----
verify_checksum_inventory "$DIR"
v_ok "SHA256SUMS: exact regular-file inventory matches"
[ -f "$DIR/scripts/release/lib/safe_tar.py" ] || rl_die "packaged safe archive validator missing"
cmp -s "$SAFE_TAR_HELPER" "$DIR/scripts/release/lib/safe_tar.py" \
    || rl_die "packaged safe archive validator differs from trusted repository helper"

# ---- 2. 内容策略 ----
verify_package_contents "$DIR"
v_ok "package content policy (allowlist + denylist)"

# ---- 3. manifest 结构 ----
MANIFEST_JSON="$(node -e '
const m = require(process.argv[1] + "/release-manifest.json");
const offline = m.schema === "joincare.release-evidence.v1" && m.admission_status === "UNADMITTED_OFFLINE_EVIDENCE_ONLY";
const candidate = m.schema === "joincare.test-server-release-candidate.v1" && m.admission_status === "CANDIDATE_PACKAGE_BUILD_ADMITTED";
if (!offline && !candidate) {
    console.error("manifest admission/schema combination invalid"); process.exit(1);
}
if (!m.profile || !m.built_at) { console.error("manifest missing profile/built_at"); process.exit(1); }
const expectedIdentities = ["admin", "backend", "contract", "deploy", "frontend"];
const identities = m.identities || {};
if (JSON.stringify(Object.keys(identities).sort()) !== JSON.stringify(expectedIdentities)) {
    console.error("manifest identities must be exactly deploy, contract, backend, frontend, admin"); process.exit(1);
}
for (const name of expectedIdentities) {
    const record = identities[name];
    if (!record || JSON.stringify(Object.keys(record).sort()) !== JSON.stringify(["commit"]) || !/^[0-9a-f]{40}$/.test(record.commit)) {
        console.error("manifest identity " + name + " invalid"); process.exit(1);
    }
}
const components = m.components || {};
if (JSON.stringify(Object.keys(components).sort()) !== JSON.stringify(["admin", "backend", "frontend"])) {
    console.error("manifest components must be exactly backend, frontend, admin"); process.exit(1);
}
for (const name of ["backend", "frontend", "admin"]) {
    if (components[name].commit !== identities[name].commit) { console.error("component/identity commit mismatch: " + name); process.exit(1); }
}
const c = m.chain || {};
if (c.checkin_chain_id !== 56) { console.error("manifest checkin_chain_id must be 56, got " + c.checkin_chain_id); process.exit(1); }
if (!/^0x[0-9a-fA-F]{40}$/.test(String(c.checkin_address || ""))) { console.error("manifest checkin_address invalid"); process.exit(1); }
if (!Array.isArray(c.trusted_project_chains) || c.trusted_project_chains.includes(31337) || (offline && c.trusted_project_chains.includes(56)) || (candidate && JSON.stringify(c.trusted_project_chains) !== JSON.stringify([56]))) {
    console.error("manifest trusted_project_chains invalid"); process.exit(1);
}
for (const [name, comp] of Object.entries(components)) {
    if (!/^[0-9a-f]{40}$/.test(comp.commit || "")) { console.error("manifest component " + name + " commit invalid"); process.exit(1); }
    if (!/^[0-9a-f]{64}$/.test(comp.lockfile?.sha256 || "")) { console.error("manifest component " + name + " lockfile sha256 invalid"); process.exit(1); }
}
if (candidate) {
    if (m.profile !== "test-server-fair-launch-bsc-mainnet" || m.environment !== "test-server") { console.error("candidate profile/environment invalid"); process.exit(1); }
    const a=m.admission||{}, f=m.fair_launch||{};
    if (!a.id || !Number.isSafeInteger(a.revision) || !/^[0-9a-f]{64}$/.test(a.sha256||"") || a.package_build_permitted !== true || a.operational_apply_permitted !== false) { console.error("candidate admission invalid"); process.exit(1); }
    if (!f.chain || f.chain.chainId !== 56 || !f.bundle || !Array.isArray(f.components) || f.components.length !== 9 || !f.authorities || !Array.isArray(f.paymentTokens) || !Array.isArray(f.migrations) || f.migrations.length !== 6) { console.error("Fair Launch authority invalid"); process.exit(1); }
    if (f.capabilities?.walletSubmissionPermitted !== false || f.capabilities?.adminDispatchPermitted !== false || f.capabilities?.materializationPermitted !== false) { console.error("candidate operational capability enabled"); process.exit(1); }
}
console.log(m.profile);
' "$DIR")" || rl_die "release-manifest.json failed structural validation"
v_ok "manifest: profile=$MANIFEST_JSON chain=56 trusted=$(node -e 'const m=require(process.argv[1]+"/release-manifest.json");console.log(m.chain.trusted_project_chains.join(","))' "$DIR")"

if [ -n "$EXPECT_PROFILE" ] && [ "$MANIFEST_JSON" != "$EXPECT_PROFILE" ]; then
    rl_die "profile mismatch: manifest=$MANIFEST_JSON expected=$EXPECT_PROFILE"
fi

MANIFEST_SCHEMA="$(node -e 'console.log(require(process.argv[1]+"/release-manifest.json").schema)' "$DIR")"
RELEASE_PROFILE_KIND=checkin
if [ "$MANIFEST_SCHEMA" = joincare.test-server-release-candidate.v1 ]; then
    RELEASE_PROFILE_KIND=fair_launch
    RELEASE_PROFILE="$MANIFEST_JSON"
    RELEASE_ENVIRONMENT="$(node -e 'console.log(require(process.argv[1]+"/release-manifest.json").environment)' "$DIR")"
    [ -f "$DIR/fair-launch-release-authority.json" ] || rl_die "candidate Fair Launch authority missing"
    AUTHORITY_RESULT="$(node "$VERIFY_DIR/validate-fair-launch-release-admission.js" package "$DIR/fair-launch-release-authority.json" "$DIR" "$VERIFY_DIR/../..")" \
        || rl_die "packaged Fair Launch release authority invalid"
    AUTHORITY_SHA="$(node -e 'console.log(JSON.parse(process.argv[1]).admissionSha256)' "$AUTHORITY_RESULT")"
    MANIFEST_AUTHORITY_SHA="$(node -e 'console.log(require(process.argv[1]+"/release-manifest.json").admission.sha256)' "$DIR")"
    [ "$AUTHORITY_SHA" = "$MANIFEST_AUTHORITY_SHA" ] || rl_die "Fair Launch admission digest mismatch"
fi
export RELEASE_PROFILE_KIND

# ---- 4. bundle 门禁（用 manifest 的链身份，防校验器自身被环境带偏）----
VITE_CHECKIN_ADDRESS="$(node -e 'const m=require(process.argv[1]+"/release-manifest.json");console.log(m.chain.checkin_address)' "$DIR")"
VITE_CHECKIN_CHAIN_ID="$(node -e 'const m=require(process.argv[1]+"/release-manifest.json");console.log(m.chain.checkin_chain_id)' "$DIR")"
VITE_TRUSTED_PROJECT_CHAINS="$(node -e 'const m=require(process.argv[1]+"/release-manifest.json");console.log(m.chain.trusted_project_chains.join(","))' "$DIR")"
if [ "$RELEASE_PROFILE_KIND" = fair_launch ]; then
    eval "$(node - "$DIR/release-manifest.json" <<'NODE'
const m=require(process.argv[2]),f=m.fair_launch,c=f.chain,b=f.bundle,t=Object.fromEntries(f.paymentTokens.map(x=>[x.symbol,x]));
const pairs={
VITE_FAIR_LAUNCH_CHAIN_ID:c.chainId,VITE_FAIR_LAUNCH_PROXY_ADDRESS:c.proxyAddress,
VITE_FAIR_LAUNCH_IMPLEMENTATION_ADDRESS:c.implementationAddress,VITE_FAIR_LAUNCH_IMPLEMENTATION_CODE_HASH:c.implementationRuntimeCodeHash,
VITE_FAIR_LAUNCH_PROXY_CODE_HASH:c.proxyRuntimeCodeHash,VITE_FAIR_LAUNCH_START_BLOCK:c.startBlock,
VITE_FAIR_LAUNCH_DEPLOYMENT_BLOCK_HASH:c.deploymentBlockHash,VITE_FAIR_LAUNCH_CONFIRMATION_POLICY:c.confirmationPolicy,
VITE_FAIR_LAUNCH_CONTRACT_VERSION_HASH:c.contractVersionHash,VITE_FAIR_LAUNCH_FULL_SET_DIGEST:b.fullSetDigest,
VITE_FAIR_LAUNCH_AGGREGATE_ABI_DIGEST:b.aggregateAbiDigest,VITE_FAIR_LAUNCH_SELECTOR_MAP_DIGEST:b.selectorMapDigest,
VITE_FAIR_LAUNCH_STORAGE_LAYOUT_DIGEST:b.storageLayoutDigest,
VITE_FAIR_LAUNCH_USDT_ADDRESS:t.USDT.address,VITE_FAIR_LAUNCH_USDC_ADDRESS:t.USDC.address,
VITE_FAIR_LAUNCH_STABLECOIN_DECIMALS:18,VITE_FAIR_LAUNCH_WALLET_SUBMISSION:false,VITE_FAIR_LAUNCH_ADMIN_DISPATCH:false,VITE_FAIR_LAUNCH_MATERIALIZATION:false};
for(const [k,v] of Object.entries(pairs)) console.log(`export ${k}=${JSON.stringify(String(v))}`)
NODE
)"
    FAIR_LAUNCH_RELEASE_ADMISSION_FILE=fair-launch-release-authority.json
    VITE_HARDHAT_RPC=
    VITE_REOWN_PROJECT_ID=0123456789abcdef0123456789abcdef
    VITE_FAIR_LAUNCH_STABLECOIN_DECIMALS=18
    validate_release_profile "$MANIFEST_JSON"
fi
# Validate sidecar and every member before extracting the static archive.
frontend_pkg="$(verify_static_package "$DIR/packages" frontend)"
frontend_digest="$(verify_package_sidecar_digest "$DIR/packages" "$frontend_pkg")"
FE_TMP="$(mktemp -d)"
release_python "$SAFE_TAR_HELPER" --policy static --expected-sha256 "$frontend_digest" \
    --extract-to "$FE_TMP" "$DIR/packages/$frontend_pkg" \
    || rl_die "frontend static archive extraction failed safe validation"
verify_frontend_bundle "$FE_TMP/dist"
rm -rf "$FE_TMP"
v_ok "frontend static archive + bundle gate (verified sidecar, safe dist layout, chain identity embedded)"

# ---- 5. backend 二进制 ----
for bin in joincare-core joincare-job joincare-notice; do
    verify_backend_binary "$DIR/artifacts/backend/$bin"
done
v_ok "backend binaries are linux/amd64 ELF"

# ---- 6. 组件交付包（skill 约定：backend 镜像 + frontend/admin 静态包）----
img="$(verify_backend_image_package "$DIR/images")"
v_ok "backend docker-save image: $img (manifest.json + joincare/backend RepoTag + sha256 sidecar)"
for comp in frontend admin; do
    pkg="$(verify_static_package "$DIR/packages" "$comp")"
    v_ok "$comp static package: $pkg (strict sidecar + normalized regular dist archive)"
done

printf '\nverify: PASS (%s)\n' "$MANIFEST_JSON"

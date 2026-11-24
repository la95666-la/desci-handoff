#!/usr/bin/env bash
# build-test-runtime.sh — 最小权限 runtime-only 发布构建器
#
# 与 ./build.sh + ./pack.sh 的区别：
#   - 只构建 backend归档 / frontend / admin；不构建TGBot，不编译或携带任何合约材料
#   - 从干净的 git 提交树构建（dirty tree 直接拒绝），组件 commit 记入 manifest
#   - 产物写入独立 staging 目录，绝不复用共享 artifacts/（杜绝批次混装）
#   - 前端/Admin 锁文件严格安装（--frozen-lockfile），lockfile SHA256 入 manifest
#   - 构建后立即执行 bundle 门禁（链身份固化、无本地链残留）
#   - 打包使用显式文件清单，不是"复制整个仓库再排除"
#   - 绝不读取 .deploy.env；VITE_REOWN_PROJECT_ID 必须由构建者显式 export
#
# 用法（在 WSL 内执行；源码默认在 /mnt/d 下，可用 env 覆盖）：
#   export VITE_REOWN_PROJECT_ID=<32-hex>
#   bash scripts/release/build-test-runtime.sh [profile]     # 默认 test-server-mainnet-checkin
#
# 输出：release/out/<profile>/joincare-release-<profile>-<date>-<backend8>.tar.gz
set -euo pipefail
[ -z "${JC_FAIR_LAUNCH_TEST_AUTHORITY+x}" ] || { echo 'JC_FAIR_LAUNCH_TEST_AUTHORITY is forbidden in release packaging' >&2; exit 1; }

BUILD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$BUILD_DIR/../.." && pwd)"
# shellcheck source=scripts/release/lib/release_common.sh
source "$BUILD_DIR/lib/release_common.sh"

PROFILE="${1:-test-server-mainnet-checkin}"
PROFILE_FILE="$BUILD_DIR/profiles/$PROFILE.env"
[ -f "$PROFILE_FILE" ] || rl_die "unknown release profile: $PROFILE (expected file at scripts/release/profiles/$PROFILE.env)"
# profile 文件只允许 KEY=VALUE 与注释，杜绝意外执行任意 shell
if grep -Eq '^[^#A-Za-z_]' "$PROFILE_FILE" || grep -Eq '[;&|`$()]' <(grep -v '^#' "$PROFILE_FILE" | grep -v '^$'); then
    rl_die "profile file contains forbidden shell syntax: $PROFILE_FILE"
fi
# shellcheck disable=SC1090
set -a
source "$PROFILE_FILE"
set +a

step()   { printf '\033[1;36m==>\033[0m %s\n' "$*"; }
ok()     { printf '\033[1;32m ✓ \033[0m%s\n' "$*"; }
fail()   { printf '\033[1;31m ✗ \033[0m%s\n' "$*" >&2; exit 1; }

validate_release_profile "$PROFILE"
export RELEASE_PROFILE RELEASE_PROFILE_KIND RELEASE_ENVIRONMENT
require_release_tooling_worktree "$ROOT"

# ---- Deploy provenance and package-build admission gate ----
DEPLOY_COMMIT="$(git -C "$ROOT" rev-parse HEAD)"
if [ "${RELEASE_PROFILE_KIND:-checkin}" = checkin ]; then
    FAIR_LAUNCH_REVIEWED_DEPLOY_REF="${FAIR_LAUNCH_REVIEWED_DEPLOY_REF:-}"
    [[ $FAIR_LAUNCH_REVIEWED_DEPLOY_REF =~ ^[0-9a-f]{40}$ ]] \
        || rl_die 'reviewed Deploy immutable ref is required for offline evidence'
    [ "$FAIR_LAUNCH_REVIEWED_DEPLOY_REF" = "$DEPLOY_COMMIT" ] \
        || rl_die "reviewed Deploy ref must equal current git HEAD: reviewed=$FAIR_LAUNCH_REVIEWED_DEPLOY_REF head=$DEPLOY_COMMIT"
fi

# ---- 源码路径（env 覆盖 > 默认）----
SRC_BACKEND="${SRC_BACKEND:-/mnt/d/LA/desci/joincare_code/joincare_code/_extracted/joincare_server-master}"
SRC_FRONTEND="${SRC_FRONTEND:-/mnt/d/LA/desci/joincare_code/joincare_code/_extracted/joincare_frontend-main/joincare-web}"
SRC_ADMIN="${SRC_ADMIN:-/mnt/d/LA/desci/joincare-admin}"

step "profile: $PROFILE ($RELEASE_DESCRIPTION)"
step "sources:"
printf '  backend : %s\n  frontend: %s\n  admin   : %s\n' "$SRC_BACKEND" "$SRC_FRONTEND" "$SRC_ADMIN"

# ---- 干净树门禁：release 必须从精确提交构建 ----
require_clean_worktree "$SRC_BACKEND" backend
require_clean_worktree "$SRC_FRONTEND" frontend
require_clean_worktree "$SRC_ADMIN" admin

BACKEND_COMMIT="$(git -C "$SRC_BACKEND" rev-parse HEAD)"
FRONTEND_COMMIT="$(git -C "$SRC_FRONTEND" rev-parse HEAD)"
ADMIN_COMMIT="$(git -C "$SRC_ADMIN" rev-parse HEAD)"
FAIR_LAUNCH_REVIEWED_CONTRACT_REF="${FAIR_LAUNCH_REVIEWED_CONTRACT_REF:-}"
[[ $FAIR_LAUNCH_REVIEWED_CONTRACT_REF =~ ^[0-9a-f]{40}$ ]] || rl_die 'reviewed Fair Launch Contract immutable ref is required'

ADMISSION_STATUS="UNADMITTED_OFFLINE_EVIDENCE_ONLY"
ADMISSION_ID=""
ADMISSION_REVISION=""
ADMISSION_SHA256=""
if [ "${RELEASE_PROFILE_KIND:-checkin}" = fair_launch ]; then
    ADMISSION_PATH="$ROOT/$FAIR_LAUNCH_RELEASE_ADMISSION_FILE"
    [ -f "$ADMISSION_PATH" ] || rl_die "Fair Launch release admission not found: $ADMISSION_PATH"
    export FAIR_LAUNCH_RELEASE_COMPONENT_COMMITS
    FAIR_LAUNCH_RELEASE_COMPONENT_COMMITS="$(node -e 'console.log(JSON.stringify({contract:process.argv[1],backend:process.argv[2],frontend:process.argv[3],admin:process.argv[4]}))' \
        "$FAIR_LAUNCH_REVIEWED_CONTRACT_REF" "$BACKEND_COMMIT" "$FRONTEND_COMMIT" "$ADMIN_COMMIT")"
    ADMISSION_RESULT="$(node "$BUILD_DIR/validate-fair-launch-release-admission.js" source "$ADMISSION_PATH" "$ROOT")" \
        || rl_die 'Fair Launch package-build admission validation failed'
    ADMISSION_ID="$(node -e 'const x=JSON.parse(process.argv[1]);console.log(x.admissionId)' "$ADMISSION_RESULT")"
    ADMISSION_REVISION="$(node -e 'const x=JSON.parse(process.argv[1]);console.log(x.revision)' "$ADMISSION_RESULT")"
    ADMISSION_SHA256="$(node -e 'const x=JSON.parse(process.argv[1]);console.log(x.admissionSha256)' "$ADMISSION_RESULT")"
    ADMISSION_STATUS="CANDIDATE_PACKAGE_BUILD_ADMITTED"
    export ADMISSION_PATH
fi
ok "commits: deploy=${DEPLOY_COMMIT:0:12} contract=${FAIR_LAUNCH_REVIEWED_CONTRACT_REF:0:12} backend=${BACKEND_COMMIT:0:12} frontend=${FRONTEND_COMMIT:0:12} admin=${ADMIN_COMMIT:0:12}"

sha256_of() { sha256sum "$1" | awk '{print $1}'; }

# ---- 独立 staging（不复用共享 artifacts/）----
DATE_TAG="$(date -u +%Y%m%d-%H%M%S)"
STAGE_ROOT="${RELEASE_STAGE_ROOT:-/tmp/jc-release-build}"
STAGE="$STAGE_ROOT/$PROFILE-$DATE_TAG"
OUT_DIR="$ROOT/release/out/$PROFILE"
rm -rf "$STAGE" "$OUT_DIR"
# 包目录全程放 WSL 本地 /tmp（或 RELEASE_STAGE_ROOT）：/mnt/d 挂载有写回延迟，
# 大文件（docker save gzip ~100MB）写后立即读回会拿到截断档导致门禁误报。
mkdir -p "$STAGE" "$OUT_DIR" "$STAGE/pkg/artifacts/backend" "$STAGE/pkg/artifacts/frontend" "$STAGE/pkg/artifacts/admin"
mkdir -p "$STAGE/pkg/deploy"
# Never package generated repository dist trees or the unrelated Project #5
# manager-test documents. Only the explicit allowlist at final tar creation ships.
PKG="$STAGE/pkg"

# ============================================================
step "building backend (go, linux/amd64)"
# ============================================================
(
    cd "$SRC_BACKEND"
    export GOEXPERIMENT=jsonv2 CGO_ENABLED=0 GOOS=linux GOARCH=amd64
    # go.mod 声明 1.25.1；若本机 go 较旧，允许 GOTOOLCHAIN 自动取用所需工具链
    export GOTOOLCHAIN="${GOTOOLCHAIN:-auto}"
    for cmd in core job notice; do
        printf '  go build cmd/%s\n' "$cmd"
        go build -trimpath -ldflags="-s -w" -o "$PKG/artifacts/backend/joincare-$cmd" "./cmd/$cmd"
        verify_backend_binary "$PKG/artifacts/backend/joincare-$cmd"
    done
)
ok "backend binaries (linux/amd64 ELF)"

BACKEND_GO_SUM_SHA="$(sha256_of "$SRC_BACKEND/go.sum")"

# ============================================================
step "building frontend (vite, frozen lockfile, WSL-local stage)"
# ============================================================
FE_STAGE="$STAGE/frontend"
rsync -a --delete \
    --exclude node_modules --exclude dist --exclude .git \
    --exclude .env.local --exclude .env.*.local \
    "$SRC_FRONTEND/" "$FE_STAGE/"
(
    cd "$FE_STAGE"
    export CI=true
    pnpm install --frozen-lockfile --prefer-offline
    # profile 提供链身份；REOWN ID 已在环境里通过 validate_release_profile 校验
    export VITE_ENVIRONMENT VITE_CHECKIN_CHAIN_ID VITE_CHECKIN_CHAIN_NAME \
           VITE_CHECKIN_CURRENCY VITE_CHECKIN_ADDRESS VITE_TRUSTED_PROJECT_CHAINS \
           VITE_API_BASE_URL VITE_WS_BASE_URL
    if [ "${RELEASE_PROFILE_KIND:-checkin}" = fair_launch ]; then
        export VITE_FAIR_LAUNCH_CHAIN_ID VITE_FAIR_LAUNCH_PROXY_ADDRESS \
            VITE_FAIR_LAUNCH_IMPLEMENTATION_ADDRESS VITE_FAIR_LAUNCH_IMPLEMENTATION_CODE_HASH \
            VITE_FAIR_LAUNCH_PROXY_CODE_HASH VITE_FAIR_LAUNCH_START_BLOCK \
            VITE_FAIR_LAUNCH_DEPLOYMENT_BLOCK_HASH VITE_FAIR_LAUNCH_CONFIRMATION_POLICY \
            VITE_FAIR_LAUNCH_CONTRACT_VERSION_HASH VITE_FAIR_LAUNCH_FULL_SET_DIGEST \
            VITE_FAIR_LAUNCH_AGGREGATE_ABI_DIGEST VITE_FAIR_LAUNCH_SELECTOR_MAP_DIGEST \
            VITE_FAIR_LAUNCH_STORAGE_LAYOUT_DIGEST VITE_FAIR_LAUNCH_USDT_ADDRESS \
            VITE_FAIR_LAUNCH_USDC_ADDRESS VITE_FAIR_LAUNCH_STABLECOIN_DECIMALS \
            VITE_FAIR_LAUNCH_WALLET_SUBMISSION VITE_FAIR_LAUNCH_ADMIN_DISPATCH \
            VITE_FAIR_LAUNCH_MATERIALIZATION RELEASE_PROFILE_KIND
    fi
    export VITE_HARDHAT_RPC=""
    unset VITE_LOCAL_API_URL || true
    pnpm build
    [ -d dist ] || fail "vite did not produce dist/"
    verify_frontend_bundle dist
    rm -rf "$PKG/artifacts/frontend/dist"
    cp -r dist "$PKG/artifacts/frontend/dist"
)
ok "frontend dist + bundle gate"

FRONTEND_LOCK_SHA="$(sha256_of "$SRC_FRONTEND/pnpm-lock.yaml")"

# ============================================================
step "building admin (vben monorepo, frozen lockfile)"
# ============================================================
AD_STAGE="$STAGE/admin"
rsync -a --delete \
    --exclude node_modules --exclude .git \
    --exclude 'apps/*/dist' \
    "$SRC_ADMIN/" "$AD_STAGE/"
(
    cd "$AD_STAGE"
    export CI=true
    printf 'VITE_GLOB_API_URL=/admin\nVITE_NITRO_MOCK=false\n' > apps/web-antd/.env.production
    pnpm install --frozen-lockfile --prefer-offline
    pnpm build:antd
    [ -d apps/web-antd/dist ] || fail "vben build did not produce dist/"
    rm -rf "$PKG/artifacts/admin/dist"
    cp -r apps/web-antd/dist "$PKG/artifacts/admin/dist"
)
ok "admin dist"

ADMIN_LOCK_SHA="$(sha256_of "$SRC_ADMIN/pnpm-lock.yaml")"

# ============================================================
step "assembling minimal runtime-only package"
# ============================================================
install -m 0644 "$ROOT/deploy/Dockerfile.backend.runtime" "$PKG/deploy/"

# ============================================================
step "packaging components (skill 交付方式)"
#   backend  → images/joincare-backend-*.tar.gz   docker save | gzip（仅归档和离线校验）
#   frontend → packages/joincare-frontend-*.tar.gz 顶层 dist/（仅归档和离线校验）
#   admin    → packages/joincare-admin-*.tar.gz    顶层 dist/（仅归档和离线校验）
# ============================================================
mkdir -p "$PKG/images" "$PKG/packages"

# ---- backend docker-save image (archive/verification-only in this release) ----
# It remains in the evidence package, but no generated guidance may instruct
# operators to load, retag, Apply, Rollback, recreate or start it.
BACKEND_IMAGE_TAG="joincare/backend:release-${DATE_TAG}-${BACKEND_COMMIT:0:8}"
BACKEND_IMAGE_FILE="joincare-backend-release-${DATE_TAG}-${BACKEND_COMMIT:0:8}.tar.gz"
mkdir -p "$STAGE/img-backend/artifacts/backend" "$STAGE/img-backend/deploy"
install -m 0644 "$ROOT/deploy/Dockerfile.backend.runtime" "$STAGE/img-backend/deploy/"
cp "$PKG/artifacts/backend/joincare-core" "$PKG/artifacts/backend/joincare-job" "$PKG/artifacts/backend/joincare-notice" "$STAGE/img-backend/artifacts/backend/"
(
    set -e
    cd "$STAGE/img-backend"
    docker build -f deploy/Dockerfile.backend.runtime -t "$BACKEND_IMAGE_TAG" .
    docker save "$BACKEND_IMAGE_TAG" | gzip > "$STAGE/backend-image-source.tar.gz"
    release_python "$ROOT/scripts/release/lib/normalize_docker_save.py" \
        "$STAGE/backend-image-source.tar.gz" "$PKG/images/$BACKEND_IMAGE_FILE"
    rm -f "$STAGE/backend-image-source.tar.gz"
)
sha256_of "$PKG/images/$BACKEND_IMAGE_FILE" | awk -v f="$BACKEND_IMAGE_FILE" '{printf "%s  %s\n", $1, f}' > "$PKG/images/$BACKEND_IMAGE_FILE.sha256"
verify_backend_image_package "$PKG/images" >/dev/null
ok "backend image: $BACKEND_IMAGE_FILE ($(du -h "$PKG/images/$BACKEND_IMAGE_FILE" | cut -f1), RepoTag $BACKEND_IMAGE_TAG)"

# ---- frontend 静态 dist 包（skill：tar 顶层 dist/，含 dist/index.html）----
FRONTEND_PACKAGE_FILE="joincare-frontend-release-${DATE_TAG}-${FRONTEND_COMMIT:0:8}.tar.gz"
(
    set -e
    cd "$PKG/artifacts/frontend"
    [ -f dist/index.html ] || fail "frontend dist/index.html missing"
    tar -czf "$PKG/packages/$FRONTEND_PACKAGE_FILE" dist
)
sha256_of "$PKG/packages/$FRONTEND_PACKAGE_FILE" | awk -v f="$FRONTEND_PACKAGE_FILE" '{printf "%s  %s\n", $1, f}' > "$PKG/packages/$FRONTEND_PACKAGE_FILE.sha256"
verify_static_package "$PKG/packages" frontend >/dev/null
# dist 已装进静态包；裸 dist 不再属于最终包（allowlist 之外）
rm -rf "$PKG/artifacts/frontend"
ok "frontend static package: $FRONTEND_PACKAGE_FILE ($(du -h "$PKG/packages/$FRONTEND_PACKAGE_FILE" | cut -f1), top-level dist/)"

# ---- admin 静态 dist 包 ----
ADMIN_PACKAGE_FILE="joincare-admin-release-${DATE_TAG}-${ADMIN_COMMIT:0:8}.tar.gz"
(
    set -e
    cd "$PKG/artifacts/admin"
    [ -f dist/index.html ] || fail "admin dist/index.html missing"
    tar -czf "$PKG/packages/$ADMIN_PACKAGE_FILE" dist
)
sha256_of "$PKG/packages/$ADMIN_PACKAGE_FILE" | awk -v f="$ADMIN_PACKAGE_FILE" '{printf "%s  %s\n", $1, f}' > "$PKG/packages/$ADMIN_PACKAGE_FILE.sha256"
verify_static_package "$PKG/packages" admin >/dev/null
rm -rf "$PKG/artifacts/admin"
ok "admin static package: $ADMIN_PACKAGE_FILE ($(du -h "$PKG/packages/$ADMIN_PACKAGE_FILE" | cut -f1), top-level dist/)"

cat > "$PKG/README-DEPLOY.md" <<EOF
JoinCare runtime-only release · $PROFILE
=========================================
built_at:               $DATE_TAG
checkin_chain_id:       $VITE_CHECKIN_CHAIN_ID
checkin_address:        $VITE_CHECKIN_ADDRESS
trusted_project_chains: $VITE_TRUSTED_PROJECT_CHAINS

backend  commit: $BACKEND_COMMIT  (go.sum sha256 $BACKEND_GO_SUM_SHA)
frontend commit: $FRONTEND_COMMIT (pnpm-lock sha256 $FRONTEND_LOCK_SHA)
admin    commit: $ADMIN_COMMIT    (pnpm-lock sha256 $ADMIN_LOCK_SHA)
deploy reviewed commit:   $DEPLOY_COMMIT
contract reviewed commit: $FAIR_LAUNCH_REVIEWED_CONTRACT_REF
admission_status:         $ADMISSION_STATUS
admission_id:             ${ADMISSION_ID:-none}
operational_apply:        false

包内不含合约源码 / 部署与升级脚本 / Hardhat 编排 / 任何密钥。
CheckIn 复用既有 BSC 主网 proxy，禁止重新部署。

交付边界：
  允许构建和保存 checksum-bound candidate package。
  candidate package 不等于服务器 Apply admission。
  Backend/Job/Lang 激活返回 fair_launch_activation_unavailable。
  Frontend/Admin 上传、Apply、restart、upgrade、rollback 返回 static_release_admission_unavailable。
  本交付包不提供任何安装或容器变更命令。

组件包完整性 sidecar：images/*.sha256 与 packages/*.sha256（同目录逐一校验）
整包完整性：sha256sum -c SHA256SUMS
EOF

if [ "${RELEASE_PROFILE_KIND:-checkin}" = fair_launch ]; then
    cp "$ADMISSION_PATH" "$PKG/fair-launch-release-authority.json"
fi

BACKEND_SHORT="${BACKEND_COMMIT:0:8}"
printf '%s\n' \
    "artifacts/backend/joincare-core" \
    "artifacts/backend/joincare-job" \
    "artifacts/backend/joincare-notice" \
    "backend_commit=$BACKEND_COMMIT" \
    "frontend_commit=$FRONTEND_COMMIT" \
    "admin_commit=$ADMIN_COMMIT" \
    "fair_launch_deploy_commit=$DEPLOY_COMMIT" \
    "fair_launch_contract_commit=$FAIR_LAUNCH_REVIEWED_CONTRACT_REF" \
    "release_profile_kind=${RELEASE_PROFILE_KIND:-checkin}" \
    "release_environment=${RELEASE_ENVIRONMENT:-}" \
    "admission_status=$ADMISSION_STATUS" \
    "admission_id=$ADMISSION_ID" \
    "admission_revision=$ADMISSION_REVISION" \
    "admission_sha256=$ADMISSION_SHA256" \
    "frontend_pnpm_lock_sha256=$FRONTEND_LOCK_SHA" \
    "admin_pnpm_lock_sha256=$ADMIN_LOCK_SHA" \
    "backend_go_sum_sha256=$BACKEND_GO_SUM_SHA" \
    "checkin_chain_id=$VITE_CHECKIN_CHAIN_ID" \
    "checkin_address=$VITE_CHECKIN_ADDRESS" \
    "trusted_project_chains=$VITE_TRUSTED_PROJECT_CHAINS" \
    "profile=$PROFILE" \
    "built_at=$DATE_TAG" > "$STAGE/manifest.kv"

if [ "${RELEASE_PROFILE_KIND:-checkin}" = fair_launch ]; then
    export VITE_FAIR_LAUNCH_CHAIN_ID VITE_FAIR_LAUNCH_PROXY_ADDRESS \
        VITE_FAIR_LAUNCH_IMPLEMENTATION_ADDRESS VITE_FAIR_LAUNCH_IMPLEMENTATION_CODE_HASH \
        VITE_FAIR_LAUNCH_PROXY_CODE_HASH VITE_FAIR_LAUNCH_START_BLOCK \
        VITE_FAIR_LAUNCH_DEPLOYMENT_BLOCK_HASH VITE_FAIR_LAUNCH_CONFIRMATION_POLICY \
        VITE_FAIR_LAUNCH_CONTRACT_VERSION_HASH VITE_FAIR_LAUNCH_FULL_SET_DIGEST \
        VITE_FAIR_LAUNCH_AGGREGATE_ABI_DIGEST VITE_FAIR_LAUNCH_SELECTOR_MAP_DIGEST \
        VITE_FAIR_LAUNCH_STORAGE_LAYOUT_DIGEST
fi
node - "$STAGE/manifest.kv" "$PKG/release-manifest.json" <<'NODE'
const fs = require('fs');
const [kvPath, outPath] = process.argv.slice(2);
const flat = {};
for (const line of fs.readFileSync(kvPath, 'utf8').split('\n')) {
    if (!line || line.startsWith('artifacts/')) continue;
    const i = line.indexOf('=');
    if (i > 0) flat[line.slice(0, i)] = line.slice(i + 1);
}
const manifest = {
    schema: flat.release_profile_kind === 'fair_launch' ? 'joincare.test-server-release-candidate.v1' : 'joincare.release-evidence.v1',
    admission_status: flat.admission_status,
    profile: flat.profile,
    built_at: flat.built_at,
    environment: flat.release_environment || null,
    chain: {
        checkin_chain_id: Number(flat.checkin_chain_id),
        checkin_address: flat.checkin_address,
        trusted_project_chains: flat.trusted_project_chains.split(',').map(Number),
    },
    identities: {
        deploy: { commit: flat.fair_launch_deploy_commit },
        contract: { commit: flat.fair_launch_contract_commit },
        backend: { commit: flat.backend_commit },
        frontend: { commit: flat.frontend_commit },
        admin: { commit: flat.admin_commit },
    },
    components: {
        backend: { commit: flat.backend_commit, lockfile: { file: 'go.sum', sha256: flat.backend_go_sum_sha256 } },
        frontend: { commit: flat.frontend_commit, lockfile: { file: 'pnpm-lock.yaml', sha256: flat.frontend_pnpm_lock_sha256 } },
        admin: { commit: flat.admin_commit, lockfile: { file: 'pnpm-lock.yaml', sha256: flat.admin_pnpm_lock_sha256 } },
    },
};
if (flat.release_profile_kind === 'fair_launch') {
    const authority = JSON.parse(fs.readFileSync(process.env.ADMISSION_PATH, 'utf8'));
    manifest.admission = {
        id: flat.admission_id,
        revision: Number(flat.admission_revision),
        sha256: flat.admission_sha256,
        package_build_permitted: true,
        operational_apply_permitted: false,
    };
    manifest.fair_launch = {
        chain: authority.chain,
        bundle: authority.bundle,
        components: authority.components,
        authorities: authority.authorities,
        paymentTokens: authority.paymentTokens,
        migrations: authority.migrations,
        capabilities: {
            walletSubmissionPermitted: false,
            adminDispatchPermitted: false,
            materializationPermitted: false,
        },
    };
}
fs.writeFileSync(outPath, JSON.stringify(manifest, null, 2) + '\n');
NODE

# The helper is packaged only as review evidence. The verifier always executes its
# repository-owned adjacent helper and compares this copy byte-for-byte.
mkdir -p "$PKG/scripts/release/lib"
cp "$BUILD_DIR/lib/safe_tar.py" "$PKG/scripts/release/lib/safe_tar.py"

# Generate checksums only after every staged regular file exists so the manifest is
# an exact inventory, including the packaged helper evidence.
(
    cd "$PKG"
    find . -type f ! -name SHA256SUMS -printf '%P\0' | sort -z | xargs -0 sha256sum > SHA256SUMS
)

# ---- 打包前内容策略自检 ----
verify_package_contents "$PKG"

TARBALL_NAME="joincare-release-$PROFILE-$DATE_TAG-$BACKEND_SHORT.tar.gz"
STAGED_TARBALL="$STAGE/$TARBALL_NAME"
(
    cd "$PKG"
    tar -czf "$STAGED_TARBALL" \
        artifacts/backend/joincare-core artifacts/backend/joincare-job artifacts/backend/joincare-notice \
        images packages deploy scripts/release/lib/safe_tar.py \
        $([ "${RELEASE_PROFILE_KIND:-checkin}" = fair_launch ] && printf '%s ' fair-launch-release-authority.json) \
        release-manifest.json SHA256SUMS README-DEPLOY.md
)
sha256_of "$STAGED_TARBALL" | awk -v f="$TARBALL_NAME" '{printf "%s  %s\n", $1, f}' > "$STAGED_TARBALL.sha256"
ok "tarball (staged, WSL-local): $STAGED_TARBALL"
printf '  size: %s\n' "$(du -h "$STAGED_TARBALL" | cut -f1)"

step "verifying staged tarball with external sidecar"
bash "$BUILD_DIR/verify-test-runtime.sh" "$STAGED_TARBALL" \
    || fail "staged tarball failed verification"

# 发布助手：staged(WSL 本地) → /mnt/d，sync + 哈希重试兜底写回延迟
publish_file() {
    local src="$1" dst="$2" want got ok=0 attempt
    want="$(sha256_of "$src")"
    cp -f "$src" "$dst"
    sync "$dst" 2>/dev/null || sync
    for attempt in 1 2 3 4 5 6; do
        got="$(sha256_of "$dst" 2>/dev/null || echo none)"
        if [ "$got" = "$want" ]; then ok=1; break; fi
        printf '  copy not yet flushed (attempt %d): %s\n' "$attempt" "$(basename "$dst")" >&2
        sleep 10
    done
    [ "$ok" = 1 ] || fail "published file hash mismatch after retries: $dst (staged=$want copied=$got)"
    printf '%s' "$want"
}

step "publishing tarball to $OUT_DIR"
TARBALL="$OUT_DIR/$TARBALL_NAME"
TARBALL_SHA="$(publish_file "$STAGED_TARBALL" "$TARBALL")"
publish_file "$STAGED_TARBALL.sha256" "$TARBALL.sha256" >/dev/null
ok "published tarball and external sidecar verified: $TARBALL ($TARBALL_SHA)"

# ============================================================
step "publishing standalone component packages (agent 交付入口)"
# 安装 agent 直接消费这三个独立包（skill Component Matrix 约定的布局）；
# 无需解开大 tarball。
# ============================================================
AGENT_DIR="$OUT_DIR/agent-packages"
rm -rf "$AGENT_DIR"; mkdir -p "$AGENT_DIR"

BACKEND_IMG_PUB_SHA="$(publish_file "$PKG/images/$BACKEND_IMAGE_FILE" "$AGENT_DIR/$BACKEND_IMAGE_FILE")"
publish_file "$PKG/images/$BACKEND_IMAGE_FILE.sha256" "$AGENT_DIR/$BACKEND_IMAGE_FILE.sha256" >/dev/null
FRONTEND_PKG_PUB_SHA="$(publish_file "$PKG/packages/$FRONTEND_PACKAGE_FILE" "$AGENT_DIR/$FRONTEND_PACKAGE_FILE")"
publish_file "$PKG/packages/$FRONTEND_PACKAGE_FILE.sha256" "$AGENT_DIR/$FRONTEND_PACKAGE_FILE.sha256" >/dev/null
ADMIN_PKG_PUB_SHA="$(publish_file "$PKG/packages/$ADMIN_PACKAGE_FILE" "$AGENT_DIR/$ADMIN_PACKAGE_FILE")"
publish_file "$PKG/packages/$ADMIN_PACKAGE_FILE.sha256" "$AGENT_DIR/$ADMIN_PACKAGE_FILE.sha256" >/dev/null

bytes_of() { stat -c '%s' "$1"; }

export PROFILE DATE_TAG VITE_CHECKIN_CHAIN_ID VITE_CHECKIN_ADDRESS VITE_TRUSTED_PROJECT_CHAINS
export BACKEND_COMMIT FRONTEND_COMMIT ADMIN_COMMIT AGENT_DIR BACKEND_IMAGE_FILE BACKEND_IMG_PUB_SHA BACKEND_IMAGE_TAG
export FRONTEND_PACKAGE_FILE FRONTEND_PKG_PUB_SHA ADMIN_PACKAGE_FILE ADMIN_PKG_PUB_SHA
export BACKEND_IMAGE_SIZE="$(bytes_of "$AGENT_DIR/$BACKEND_IMAGE_FILE")"
export FRONTEND_PACKAGE_SIZE="$(bytes_of "$AGENT_DIR/$FRONTEND_PACKAGE_FILE")"
export ADMIN_PACKAGE_SIZE="$(bytes_of "$AGENT_DIR/$ADMIN_PACKAGE_FILE")"
bash "$BUILD_DIR/render-agent-handoff.sh" "$OUT_DIR/AGENT-HANDOFF.md"
ok "agent packages + AGENT-HANDOFF.md published to $AGENT_DIR"
ok "release package verified: $PROFILE"
printf '\n  agent 交付入口: %s\n  交接单: %s\n\n  完整 tarball: %s\n' \
    "$AGENT_DIR" "$OUT_DIR/AGENT-HANDOFF.md" "$TARBALL"
printf '  manifest commits — backend %s / frontend %s / admin %s\n' \
    "${BACKEND_COMMIT:0:12}" "${FRONTEND_COMMIT:0:12}" "${ADMIN_COMMIT:0:12}"

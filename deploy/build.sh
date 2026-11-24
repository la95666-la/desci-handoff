#!/usr/bin/env bash
# 本地 build 所有 artifacts → ./artifacts/
# 生成产物给 runtime Dockerfile 只 COPY 不再编译
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"
export JC_ROOT="$SCRIPT_DIR"
export JC_LIB="$SCRIPT_DIR/scripts/lib"

# shellcheck source=scripts/lib/colors.sh
source "$JC_LIB/colors.sh"

# 保存调用者显式传入的配置；source .deploy.env 不得覆盖显式环境变量。
CALLER_VITE_REOWN_PROJECT_ID="${VITE_REOWN_PROJECT_ID:-}"
CALLER_JC_REOWN_PROJECT_ID="${JC_REOWN_PROJECT_ID:-}"
CALLER_SRC_BACKEND="${SRC_BACKEND:-}"
CALLER_SRC_FRONTEND="${SRC_FRONTEND:-}"
CALLER_SRC_ADMIN="${SRC_ADMIN:-}"

# Frontend/Admin artifact builds are offline-verification only. Reject retired targets before
# reading .deploy.env so contract/TGBot requests cannot touch local secrets.
TARGET="${1:-all}"
case "$TARGET" in
    all|frontend|admin) ;;
    backend) error "fair_launch_activation_unavailable"; exit 78 ;;
    tgbot) error "tgbot_release_unavailable"; exit 78 ;;
    contracts) error "chain_mutation_unavailable"; exit 78 ;;
    *) error "未知 target: $TARGET"; info "可用: all / frontend / admin"; exit 2 ;;
esac

# 读源码路径（优先级：env > .deploy.env > default）
if [ -f "$JC_ROOT/.deploy.env" ]; then
    # shellcheck disable=SC1091
    source "$JC_ROOT/.deploy.env"
fi
SRC_BACKEND="${CALLER_SRC_BACKEND:-${JC_SRC_BACKEND:-/mnt/d/la/desci/joincare_code/joincare_code/_extracted/joincare_server-master}}"
SRC_FRONTEND="${CALLER_SRC_FRONTEND:-${JC_SRC_FRONTEND:-/mnt/d/la/desci/joincare_code/joincare_code/_extracted/joincare_frontend-main/joincare-web}}"
SRC_ADMIN="${CALLER_SRC_ADMIN:-${JC_SRC_ADMIN:-/mnt/d/la/desci/joincare-admin}}"

ARTIFACTS="$JC_ROOT/artifacts"

# Current build surface: ./build.sh [frontend|admin|all].

# 前端 build 时嵌入的 Vite 变量（可通过 export 传入）
export VITE_ENVIRONMENT="${VITE_ENVIRONMENT:-${JC_ENV:-prod}}"
# API/WS baseURL：默认空 → 走同源（前端 nginx / Caddy 反代 /client/* → backend:13004）
# 跨域场景才设：export VITE_API_BASE_URL=https://api.joincarelabs.com
export VITE_API_BASE_URL="${VITE_API_BASE_URL:-}"
export VITE_WS_BASE_URL="${VITE_WS_BASE_URL:-}"
# Reown Project ID 在 Vite 编译期固化。解析优先级：
# 调用者 VITE_* > 调用者 JC_* > .deploy.env JC_* > frontend/.env.local。
REOWN_PROJECT_ID_FROM_FILE=""
if [ -f "$SRC_FRONTEND/.env.local" ]; then
    REOWN_PROJECT_ID_FROM_FILE="$(
        sed -n 's/^[[:space:]]*VITE_REOWN_PROJECT_ID[[:space:]]*=[[:space:]]*//p' \
            "$SRC_FRONTEND/.env.local" | tail -n 1 | tr -d '\r'
    )"
    REOWN_PROJECT_ID_FROM_FILE="${REOWN_PROJECT_ID_FROM_FILE%\"}"
    REOWN_PROJECT_ID_FROM_FILE="${REOWN_PROJECT_ID_FROM_FILE#\"}"
    REOWN_PROJECT_ID_FROM_FILE="${REOWN_PROJECT_ID_FROM_FILE%\'}"
    REOWN_PROJECT_ID_FROM_FILE="${REOWN_PROJECT_ID_FROM_FILE#\'}"
fi
export VITE_REOWN_PROJECT_ID="${CALLER_VITE_REOWN_PROJECT_ID:-${CALLER_JC_REOWN_PROJECT_ID:-${JC_REOWN_PROJECT_ID:-$REOWN_PROJECT_ID_FROM_FILE}}}"
export VITE_CHECKIN_CHAIN_ID="${VITE_CHECKIN_CHAIN_ID:-${JC_CHAIN_ID:-31337}}"
export VITE_CHECKIN_CHAIN_NAME="${VITE_CHECKIN_CHAIN_NAME:-${JC_CHAIN_NAME:-JoinCare}}"
export VITE_CHECKIN_CURRENCY="${VITE_CHECKIN_CURRENCY:-${JC_CHAIN_CURRENCY:-ETH}}"
# No build profile may embed or activate a Hardhat RPC in this release.
export VITE_HARDHAT_RPC=""
# CheckIn address is read-only build input. Contract mutation is unavailable.
export VITE_CHECKIN_ADDRESS="${VITE_CHECKIN_ADDRESS:-${JC_CHECKIN_CONTRACT:-}}"

validate_reown_project_id() {
    if [ -z "$VITE_REOWN_PROJECT_ID" ]; then
        error "VITE_REOWN_PROJECT_ID 未配置，拒绝构建 frontend"
        info "请设置 JC_REOWN_PROJECT_ID（.deploy.env）或 VITE_REOWN_PROJECT_ID（环境变量）"
        return 1
    fi
    if [ "$VITE_REOWN_PROJECT_ID" = "placeholder-no-walletconnect" ]; then
        error "Reown Project ID 仍是 placeholder，拒绝构建 frontend"
        return 1
    fi
    if ! [[ "$VITE_REOWN_PROJECT_ID" =~ ^[[:xdigit:]]{32}$ ]]; then
        error "Reown Project ID 格式无效：应为 32 位十六进制字符串"
        return 1
    fi
}

verify_frontend_reown_bundle() {
    local dist_dir="$1"
    if grep -R -a -F -q "placeholder-no-walletconnect" "$dist_dir"; then
        error "frontend bundle 包含 placeholder-no-walletconnect，已拒绝生成发布产物"
        return 1
    fi
    if ! grep -R -a -F -q "$VITE_REOWN_PROJECT_ID" "$dist_dir"; then
        error "frontend bundle 未包含配置的 Reown Project ID，Vite 变量可能未注入"
        return 1
    fi
}

# ============================================================
build_backend() {
    step "编译 backend (Go)"
    [ -d "$SRC_BACKEND" ] || { error "找不到 backend 源码: $SRC_BACKEND"; exit 1; }
    command -v go >/dev/null || { error "本地缺 go —— 请装 Go 1.25+"; exit 1; }
    local out="$ARTIFACTS/backend"
    rm -rf "$out" && mkdir -p "$out"
    (
        cd "$SRC_BACKEND"
        export GOEXPERIMENT=jsonv2 CGO_ENABLED=0 GOOS=linux GOARCH=amd64
        info "  go build cmd/core → joincare-core"
        go build -trimpath -ldflags="-s -w" -o "$out/joincare-core"   ./cmd/core
        info "  go build cmd/job → joincare-job"
        go build -trimpath -ldflags="-s -w" -o "$out/joincare-job"    ./cmd/job
        info "  go build cmd/notice → joincare-notice"
        go build -trimpath -ldflags="-s -w" -o "$out/joincare-notice" ./cmd/notice
    )
    success "backend → $(du -sh "$out" | cut -f1) at $out"
}

# ============================================================
build_frontend() {
    step "编译 frontend (Vite)"
    [ -d "$SRC_FRONTEND" ] || { error "找不到 frontend 源码: $SRC_FRONTEND"; exit 1; }
    command -v pnpm >/dev/null 2>&1 || command -v npm >/dev/null || { error "本地缺 pnpm/npm"; exit 1; }
    validate_reown_project_id || exit 1
    local out="$ARTIFACTS/frontend"
    local staged_out="$ARTIFACTS/.frontend-build-$$"
    rm -rf "$staged_out" && mkdir -p "$staged_out"
    info "  Vite vars: ENV=$VITE_ENVIRONMENT REOWN=${VITE_REOWN_PROJECT_ID:0:8}... CHAIN=$VITE_CHECKIN_CHAIN_ID"
    if ! (
        cd "$SRC_FRONTEND" || exit 1
        # pnpm 在无人值守环境可能需要重建 node_modules；CI=true 禁止等待 TTY 确认。
        export CI=true
        # 禁止 build 失败后误用上一次遗留的 dist。
        rm -rf dist
        if command -v pnpm >/dev/null 2>&1; then
            CI=true pnpm install --frozen-lockfile=false --prefer-offline || exit 1
            pnpm build || exit 1
        else
            npm install --no-audit --no-fund || exit 1
            npm run build || exit 1
        fi
        [ -d dist ] || { error "Vite 未生成 dist 目录"; exit 1; }
        verify_frontend_reown_bundle dist || exit 1
        cp -r dist "$staged_out/dist" || exit 1
    ); then
        rm -rf "$staged_out"
        error "frontend 构建或 Reown bundle 校验失败；旧发布产物保持不变"
        return 1
    fi
    rm -rf "$out"
    mv "$staged_out" "$out"
    success "frontend → $(du -sh "$out" | cut -f1) at $out"
}

# ============================================================
build_admin() {
    step "编译 admin (vben monorepo · @vben/web-antd)"
    [ -d "$SRC_ADMIN" ] || { error "找不到 admin 源码: $SRC_ADMIN"; exit 1; }
    command -v pnpm >/dev/null 2>&1 || { error "admin 必须用 pnpm (monorepo)"; exit 1; }
    local out="$ARTIFACTS/admin"
    rm -rf "$out" && mkdir -p "$out"
    (
        cd "$SRC_ADMIN"
        # 生产环境 API URL 走同源 /admin/*（nginx 反代到 backend:13004），关 mock
        # vben 默认没设时会用 mock-napi.vben.pro，会导致登录 429
        printf "VITE_GLOB_API_URL=/admin\nVITE_NITRO_MOCK=false\n" \
            > apps/web-antd/.env.production
        pnpm install --frozen-lockfile=false --prefer-offline
        pnpm build:antd
        cp -r apps/web-antd/dist "$out/dist"
    )
    success "admin → $(du -sh "$out" | cut -f1) at $out"
}

# ============================================================
# TGBot and contract artifact builders are intentionally absent. TGBot has no
# official checksum-bound release input; contract mutation tooling must not be
# copied into deploy artifacts. Contract source/audit builds remain in the
# independently reviewed contract repository and Fair Launch CI lane.

# ============================================================
write_build_info() {
    local info="$ARTIFACTS/build-info.json"
    cat > "$info" <<EOF
{
  "built_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "built_by": "$(whoami)@$(hostname)",
  "targets":  "$TARGET",
  "env":      "${JC_ENV:-unknown}"
}
EOF
    success "build info → $info"
}

# ============================================================
mkdir -p "$ARTIFACTS"

case "$TARGET" in
    all)
        build_frontend
        build_admin
        ;;
    frontend) build_frontend ;;
    admin)    build_admin ;;
esac

write_build_info

echo
success "所有 artifacts 已就绪 → $ARTIFACTS"
info "Frontend/Admin artifacts are ready for archive/offline verification only; static_release_admission_unavailable."

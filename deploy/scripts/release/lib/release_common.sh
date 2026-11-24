#!/usr/bin/env bash
# runtime-only 发布共享库：profile 校验、包 allowlist/denylist、bundle 门禁。
# 被 build-test-runtime.sh / verify-test-runtime.sh / scripts/tests 复用；
# 纯函数式（除明确的 die 输出），便于 node/bash 测试直接断言。

# ---- 输出助手（build 脚本注入 step/error/success 时优先用注入版） ----
rl_die() { printf 'release: %s\n' "$*" >&2; exit 1; }

# ---- profile fail-closed 校验 ----
# 参数：profile 名；校验当前已 source 的变量。
validate_release_profile() {
    local profile="$1"
    [ -n "$profile" ] || rl_die "profile name is required"
    case "$profile" in
        */*|.*|*..*) rl_die "profile name must not contain path separators: $profile" ;;
    esac

    [ -n "${RELEASE_PROFILE:-}" ] || rl_die "RELEASE_PROFILE is not set"
    [ "$RELEASE_PROFILE" = "$profile" ] || rl_die "profile identity mismatch: requested=$profile declared=$RELEASE_PROFILE"
    RELEASE_PROFILE_KIND="${RELEASE_PROFILE_KIND:-checkin}"
    case "$RELEASE_PROFILE_KIND" in checkin|fair_launch) : ;; *) rl_die "unsupported RELEASE_PROFILE_KIND: $RELEASE_PROFILE_KIND" ;; esac
    [ -n "${VITE_CHECKIN_ADDRESS:-}" ] || rl_die "VITE_CHECKIN_ADDRESS is empty (fail-closed: refusing to build without an approved CheckIn proxy)"
    [ "${VITE_CHECKIN_CHAIN_ID:-}" = "56" ] || rl_die "VITE_CHECKIN_CHAIN_ID must be 56 for mainnet-checkin profiles, got '${VITE_CHECKIN_CHAIN_ID:-}'"
    [ -z "${VITE_HARDHAT_RPC:-}" ] || rl_die "VITE_HARDHAT_RPC must be empty; a browser-facing Hardhat RPC is forbidden in this release"
    [[ $VITE_CHECKIN_ADDRESS =~ ^0x[0-9a-fA-F]{40}$ ]] \
        || rl_die "VITE_CHECKIN_ADDRESS is not a 20-byte hex address: $VITE_CHECKIN_ADDRESS"
    [ -n "${VITE_TRUSTED_PROJECT_CHAINS:-}" ] || rl_die "VITE_TRUSTED_PROJECT_CHAINS is empty; the dual-chain profile must declare the trusted project chain explicitly"
    case ",${VITE_TRUSTED_PROJECT_CHAINS}," in
        *,31337,*) rl_die "VITE_TRUSTED_PROJECT_CHAINS must not include the local hardhat chain" ;;
    esac
    if [ "$RELEASE_PROFILE_KIND" = checkin ]; then
        case ",${VITE_TRUSTED_PROJECT_CHAINS}," in
            *,56,*) rl_die "VITE_TRUSTED_PROJECT_CHAINS must not duplicate the mainnet checkin chain" ;;
        esac
    else
        [ "${RELEASE_ENVIRONMENT:-}" = test-server ] || rl_die "Fair Launch release environment must be test-server"
        [ "$VITE_TRUSTED_PROJECT_CHAINS" = 56 ] || rl_die "Fair Launch test-server profile must trust exactly project chain 56"
        [ "${VITE_FAIR_LAUNCH_CHAIN_ID:-}" = 56 ] || rl_die "VITE_FAIR_LAUNCH_CHAIN_ID must be 56"
        for name in VITE_FAIR_LAUNCH_PROXY_ADDRESS VITE_FAIR_LAUNCH_IMPLEMENTATION_ADDRESS; do
            [[ ${!name:-} =~ ^0x[0-9a-fA-F]{40}$ ]] || rl_die "$name is not a 20-byte address"
        done
        for name in VITE_FAIR_LAUNCH_IMPLEMENTATION_CODE_HASH VITE_FAIR_LAUNCH_PROXY_CODE_HASH \
            VITE_FAIR_LAUNCH_DEPLOYMENT_BLOCK_HASH VITE_FAIR_LAUNCH_CONTRACT_VERSION_HASH \
            VITE_FAIR_LAUNCH_FULL_SET_DIGEST VITE_FAIR_LAUNCH_AGGREGATE_ABI_DIGEST \
            VITE_FAIR_LAUNCH_SELECTOR_MAP_DIGEST VITE_FAIR_LAUNCH_STORAGE_LAYOUT_DIGEST; do
            [[ ${!name:-} =~ ^0x[0-9a-fA-F]{64}$ ]] || rl_die "$name is not a 32-byte hash"
        done
        [[ ${VITE_FAIR_LAUNCH_START_BLOCK:-} =~ ^[1-9][0-9]*$ ]] || rl_die "VITE_FAIR_LAUNCH_START_BLOCK is invalid"
        [ "${VITE_FAIR_LAUNCH_CONFIRMATION_POLICY:-}" = 20_blocks ] || rl_die "Fair Launch confirmation policy must be 20_blocks"
        [ "${VITE_FAIR_LAUNCH_STABLECOIN_DECIMALS:-}" = 18 ] || rl_die "Fair Launch stablecoin decimals must be 18"
        for name in VITE_FAIR_LAUNCH_USDT_ADDRESS VITE_FAIR_LAUNCH_USDC_ADDRESS; do
            [[ ${!name:-} =~ ^0x[0-9a-fA-F]{40}$ ]] || rl_die "$name is not a 20-byte address"
        done
        for name in VITE_FAIR_LAUNCH_WALLET_SUBMISSION VITE_FAIR_LAUNCH_ADMIN_DISPATCH VITE_FAIR_LAUNCH_MATERIALIZATION; do
            [ "${!name:-}" = false ] || rl_die "$name must remain false in candidate releases"
        done
        [ -n "${FAIR_LAUNCH_RELEASE_ADMISSION_FILE:-}" ] || rl_die "Fair Launch release admission file is required"
    fi

    [ -n "${VITE_REOWN_PROJECT_ID:-}" ] || rl_die "VITE_REOWN_PROJECT_ID is not exported; export it explicitly (this builder never reads .deploy.env)"
    [ "${VITE_REOWN_PROJECT_ID}" != "placeholder-no-walletconnect" ] || rl_die "VITE_REOWN_PROJECT_ID is still the placeholder"
    [[ $VITE_REOWN_PROJECT_ID =~ ^[0-9a-fA-F]{32}$ ]] \
        || rl_die "VITE_REOWN_PROJECT_ID must be a 32-hex-char project id"
}

# ---- 源仓清洁度 ----
require_clean_worktree() {
    local repo="$1" name="$2"
    git -C "$repo" rev-parse --is-inside-work-tree >/dev/null 2>&1 || rl_die "$name is not a git work tree: $repo"
    local dirty
    dirty="$(git -C "$repo" status --porcelain)"
    [ -z "$dirty" ] || rl_die "$name worktree is dirty (commit or stash first; a release must be built from an exact committed tree):
$dirty"
}

# Deploy may contain only the four explicitly out-of-scope Project #5 manuals as
# untracked files. Any tracked modification or other untracked path refuses a
# release so the tooling commit and admission are stable and reviewable.
require_release_tooling_worktree() {
    local repo="$1" dirty line failures=0
    dirty="$(git -C "$repo" status --porcelain)"
    while IFS= read -r line; do
        [ -z "$line" ] && continue
        case "$line" in
            '?? docs/IDO-BSC-MAINNET-PROJECT-5-MANAGER-TEST-GUIDE-ZH-CN.docx'|\
            '?? docs/IDO-BSC-MAINNET-PROJECT-5-MANAGER-TEST-GUIDE-ZH-CN.pdf'|\
            '?? docs/IDO-BSC-MAINNET-PROJECT-5-MANAGER-TEST-GUIDE.md'|\
            '?? docs/IDO-BSC-MAINNET-PROJECT-5-MANAGER-TEST-GUIDE.pdf') : ;;
            *) printf 'release: deploy release-tooling worktree has an unadmitted change: %s\n' "$line" >&2; failures=$((failures + 1)) ;;
        esac
    done <<< "$dirty"
    [ "$failures" = 0 ] || rl_die "deploy release-tooling worktree is not release-clean"
}

# ---- 包内容策略 ----
# allowlist 的唯一事实来源是 verify_package_contents 里的 case 语句；
# denylist 是第二道防线（作用于文件相对路径，按 grep 正则匹配）。
# 明确禁止的文件名/路径片段（逐个 grep）。即使 allowlist 意外放宽也不得出现。
release_denylist_patterns() {
    cat <<'EOF'
hardhat
Hardhat
deployCheckIn
upgradeCheckIn
deployIDOv2
configureIDOv2
preflightCheckIn
contracts-checkin
artifacts/contracts/
\.deploy\.env$
deploy\.env$
^config/secret/
secrets?\.
\.pem$
\.key$
id_rsa
mnemonic
seed phrase
\.sql$
docker-compose
\.github/
^scripts/
^src/
node_modules
\.git/
NUL
\.bak$
project1-params
project4-params
contracts-model/
contracts-audit/
JoinCareFairLaunchV1ArtifactBaseline
fair_launch_migration
migrations/
recovery-portal/deployment-manifest.example.json
raw_tx
EOF
}

# 校验一个已解包的 release 目录内容策略；参数：目录。
# 1) allowlist：每个文件必须命中允许的精确路径或前缀
# 2) denylist：任何文件路径不得命中禁止片段（双重防线）
verify_package_contents() {
    local dir="$1"
    [ -d "$dir" ] || rl_die "release dir not found: $dir"
    [ -f "$dir/release-manifest.json" ] || rl_die "release-manifest.json missing"
    [ -f "$dir/SHA256SUMS" ] || rl_die "SHA256SUMS missing"

    local failures=0 rel pattern
    local files=() patterns=()
    while IFS= read -r rel; do
        [ -n "$rel" ] && files+=("$rel")
    done < <(cd "$dir" && find . -type f | sed 's|^\./||')
    while IFS= read -r pattern; do
        [ -n "$pattern" ] && patterns+=("$pattern")
    done <<< "$(release_denylist_patterns)"

    # 1) allowlist
    for rel in "${files[@]}"; do
        case "$rel" in
            artifacts/backend/joincare-core|artifacts/backend/joincare-job|artifacts/backend/joincare-notice|\
            release-manifest.json|fair-launch-release-authority.json|SHA256SUMS|SHA256SUMS.asc|README-DEPLOY.md|\
            deploy/Dockerfile.backend.runtime|\
            scripts/release/lib/safe_tar.py|\
            packages/joincare-frontend-*.tar.gz|packages/joincare-frontend-*.tar.gz.sha256|\
            packages/joincare-admin-*.tar.gz|packages/joincare-admin-*.tar.gz.sha256|\
            images/joincare-backend-*.tar.gz|images/joincare-backend-*.tar.gz.sha256)
                : ;;
            *)
                printf 'release: file outside allowlist: %s\n' "$rel" >&2
                failures=$((failures + 1)) ;;
        esac
    done

    # 2) denylist（第二道防线；作用于文件相对路径）。
    #    用 bash 内建 =~ 而不是 grep 管道：避免 MSYS/跨平台 fork 管道在循环体内的挂起。
    for rel in "${files[@]}"; do
        [ "$rel" = "scripts/release/lib/safe_tar.py" ] && continue
        for pattern in "${patterns[@]}"; do
            if [[ $rel =~ $pattern ]]; then
                printf 'release: denylist hit (%s): %s\n' "$pattern" "$rel" >&2
                failures=$((failures + 1))
            fi
        done
    done

    [ "$failures" = 0 ] || rl_die "package content policy failed with $failures violation(s)"
    return 0
}
# ---- backend docker-save 镜像门禁（skill 规定的 backend 交付方式） ----
# 校验 images/ 下恰有一个 joincare-backend-*.tar.gz 的 docker-save gzip：
# 含 manifest.json、RepoTags 必须是 joincare/backend:<tag>（服务器侧 docker load
# 后按 "Loaded image:" 解析重打标），且带 .sha256 sidecar。成功时输出镜像文件名。
verify_backend_image_package() {
    local dir="$1"
    [ -d "$dir" ] || rl_die "backend image dir not found: $dir"
    local images=() f
    while IFS= read -r f; do
        [ -n "$f" ] && images+=("$f")
    done < <(cd "$dir" && find . -maxdepth 1 -type f -name 'joincare-backend-*.tar.gz' | sed 's|^\./||')
    if [ "${#images[@]}" -ne 1 ]; then
        rl_die "expected exactly one joincare-backend-*.tar.gz docker-save image, found ${#images[@]}"
    fi
    local img="${images[0]}" digest manifest_blob
    # Verify the real sidecar before parsing or decompressing any archive byte. The
    # trusted helper then hashes, inspects and reads manifest.json from one handle.
    digest="$(verify_package_sidecar_digest "$dir" "$img")" \
        || rl_die "backend image sidecar verification failed: $img"
    manifest_blob="$(release_python "$(release_safe_tar_helper)" --policy docker \
        --expected-sha256 "$digest" --read-member manifest.json "$dir/$img")" \
        || rl_die "backend docker-save archive failed safe validation: $img"
    # safe_tar's docker policy validates the complete docker-save closure and
    # returns the exact bounded manifest bytes from the same stable handle.
    [ -n "$manifest_blob" ] || rl_die "backend image manifest evidence is empty after safe validation: $img"
    printf '%s' "$img"
}

release_safe_tar_helper() {
    local lib_dir helper
    lib_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    helper="$lib_dir/safe_tar.py"
    [ -f "$helper" ] || rl_die "trusted repository safe archive validator unavailable"
    printf '%s' "$helper"
}

release_python() {
    if command -v python3 >/dev/null 2>&1 && python3 --version >/dev/null 2>&1; then
        python3 "$@"
    elif command -v python >/dev/null 2>&1 && python --version >/dev/null 2>&1; then
        python "$@"
    else
        rl_die "Python 3 is required for safe archive validation"
    fi
}

safe_extract_release_tar() {
    local archive="$1" destination="$2" policy="${3:-runtime}" expected="${4:-}"
    [ -n "$expected" ] || rl_die "expected SHA-256 is required for archive extraction"
    release_python "$(release_safe_tar_helper)" --policy "$policy" --expected-sha256 "$expected" \
        --extract-to "$destination" "$archive" \
        || rl_die "unsafe or invalid $policy archive: $(basename "$archive")"
}

verify_package_sidecar_digest() {
    local dir="$1" pkg="$2" sidecar line
    sidecar="$dir/${pkg}.sha256"
    [ -f "$sidecar" ] || rl_die "package is missing its .sha256 sidecar: $pkg"
    IFS= read -r line < "$sidecar" || rl_die "invalid sha256 sidecar: $pkg.sha256"
    [[ "$line" =~ ^([0-9a-f]{64})([[:space:]][[:space:]]|[[:space:]]\*)([^/[:space:]]+)$ ]] \
        || rl_die "invalid sha256 sidecar format: $pkg.sha256"
    [ "${BASH_REMATCH[3]}" = "$pkg" ] || rl_die "sha256 sidecar basename mismatch: $pkg.sha256"
    [ "$(wc -l < "$sidecar")" -eq 1 ] || rl_die "sha256 sidecar must contain exactly one line: $pkg.sha256"
    (cd "$dir" && sha256sum -c --strict "$pkg.sha256" >/dev/null) \
        || rl_die "sha256 sidecar verification failed: $pkg"
    printf '%s' "${BASH_REMATCH[1]}"
}

verify_package_sidecar() {
    verify_package_sidecar_digest "$1" "$2" >/dev/null
}

verify_checksum_inventory() {
    local dir="$1" expected actual
    [ -f "$dir/SHA256SUMS" ] || rl_die "SHA256SUMS missing"
    expected="$(mktemp)"; actual="$(mktemp)"
    awk '
      !/^[0-9a-f]{64}  [^/][^[:cntrl:]]*$/ { exit 2 }
      { name=substr($0,67); if (name=="SHA256SUMS" || name ~ /(^|\/)\.\.($|\/)/ || name ~ /\\/ || seen[name]++) exit 3; print name }
    ' "$dir/SHA256SUMS" | LC_ALL=C sort > "$expected" \
        || { rm -f "$expected" "$actual"; rl_die "SHA256SUMS inventory format invalid"; }
    (cd "$dir" && find . -type f ! -name SHA256SUMS -printf '%P\n' | LC_ALL=C sort) > "$actual"
    if ! cmp -s "$expected" "$actual"; then
        rm -f "$expected" "$actual"
        rl_die "SHA256SUMS inventory does not exactly cover regular files"
    fi
    rm -f "$expected" "$actual"
    (cd "$dir" && sha256sum -c --strict SHA256SUMS --quiet) \
        || rl_die "SHA256SUMS verification failed"
}

# ---- frontend / admin 静态包门禁（offline archive evidence only） ----
# The archive must have a strict, verified sidecar and contain only normalized
# regular files/directories below dist/, including exactly one dist/index.html.
verify_static_package() {
    local dir="$1" component="$2"
    case "$component" in frontend|admin) : ;;
        *) rl_die "unsupported static package component: $component" ;;
    esac
    [ -d "$dir" ] || rl_die "$component package dir not found: $dir"
    local pkgs=() f
    while IFS= read -r f; do
        [ -n "$f" ] && pkgs+=("$f")
    done < <(cd "$dir" && find . -maxdepth 1 -type f -name "joincare-$component-*.tar.gz" | sed 's|^\./||')
    if [ "${#pkgs[@]}" -ne 1 ]; then
        rl_die "expected exactly one joincare-$component-*.tar.gz static package, found ${#pkgs[@]}"
    fi
    local pkg="${pkgs[0]}" digest
    digest="$(verify_package_sidecar_digest "$dir" "$pkg")" \
        || rl_die "$component static package sidecar verification failed: $pkg"
    release_python "$(release_safe_tar_helper)" --policy static --expected-sha256 "$digest" "$dir/$pkg" \
        || rl_die "$component static package failed safe archive validation: $pkg"
    printf '%s' "$pkg"
}

# ---- 前端 bundle 门禁 ----
# 参数：frontend dist 目录；校验链身份已固化且无本地链/占位符残留。
verify_frontend_bundle() {
    local dist="$1"
    [ -d "$dist" ] || rl_die "frontend dist not found: $dist"
    [ -f "$dist/index.html" ] || rl_die "frontend dist/index.html missing"

    local js_files
    js_files="$(find "$dist" -type f -name '*.js')"
    [ -n "$js_files" ] || rl_die "frontend dist has no js assets"

    # 必需标记
    grep -R -a -F -q "VITE_CHECKIN_ADDRESS:\"${VITE_CHECKIN_ADDRESS}\"" "$dist" \
        || rl_die "bundle does not embed the approved CheckIn address (expected VITE_CHECKIN_ADDRESS:\"$VITE_CHECKIN_ADDRESS\")"
    grep -R -a -F -q "VITE_CHECKIN_CHAIN_ID:\"${VITE_CHECKIN_CHAIN_ID}\"" "$dist" \
        || rl_die "bundle does not embed VITE_CHECKIN_CHAIN_ID:\"$VITE_CHECKIN_CHAIN_ID\""
    grep -R -a -F -q "VITE_ENVIRONMENT:\"${VITE_ENVIRONMENT}\"" "$dist" \
        || rl_die "bundle does not embed VITE_ENVIRONMENT:\"$VITE_ENVIRONMENT\""
    grep -R -a -F -q "VITE_TRUSTED_PROJECT_CHAINS:\"${VITE_TRUSTED_PROJECT_CHAINS}\"" "$dist" \
        || rl_die "bundle does not embed VITE_TRUSTED_PROJECT_CHAINS:\"$VITE_TRUSTED_PROJECT_CHAINS\""

    # 禁止项
    if grep -R -a -F -q "placeholder-no-walletconnect" "$dist"; then
        rl_die "bundle contains the placeholder WalletConnect project id"
    fi
    # VITE_HARDHAT_RPC 必须以空串固化（Vite 全量内联 import.meta.env，非空即配置泄漏；
    # 缺失该标记同样视为异常构建）。依赖内置的 hardhat 链注册表常量不是本构建配置，
    # 不做字符串封杀——精确校验我们自己的注入值。
    local hardhatMarker
    hardhatMarker="$(grep -R -a -h -o 'VITE_HARDHAT_RPC:"[^"]*"' "$dist" | sort -u)"
    if [ "$hardhatMarker" != 'VITE_HARDHAT_RPC:""' ]; then
        rl_die "bundle embeds a non-empty or missing VITE_HARDHAT_RPC: ${hardhatMarker:-<marker absent>}"
    fi
    # AppKit 的本地链与公共 GoChain Testnet 共用 31337。主网包不得引导钱包尝试该链。
    if [ "$VITE_ENVIRONMENT" = prod ] && [ "$VITE_CHECKIN_CHAIN_ID" = 56 ]; then
        if grep -R -a -E -q 'VITE_ENVIRONMENT:"local"|VITE_CHECKIN_CHAIN_ID:"31337"|testnet-rpc\.gochain\.io' "$dist"; then
            rl_die 'production frontend bundle contains local/GoChain network configuration'
        fi
    fi
    return 0
}

# ---- backend 二进制门禁：linux ELF64 LSB x86-64 ----
# 只依赖文件头（不依赖 file 命令）：
#   bytes 0-3   7f 45 4c 46   ELF magic
#   byte  4     02            ELFCLASS64
#   byte  5     01            ELFDATA2LSB（linux 小端）
#   bytes 18-19 3e 00         EM_X86_64
verify_backend_binary() {
    local file="$1"
    [ -f "$file" ] || rl_die "backend binary missing: $file"
    local header
    header="$(head -c 20 "$file" | od -An -tx1 | tr -d ' \n')"
    case "$header" in
        7f454c46020101*) : ;;
        *) rl_die "backend artifact is not a little-endian 64-bit ELF: $file" ;;
    esac
    local machine="${header:36:4}"
    [ "$machine" = "3e00" ] || rl_die "backend artifact is not x86-64 ELF (e_machine=${machine}): $file"
}

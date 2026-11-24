#!/usr/bin/env bash
# Render the release handoff from already verified package metadata. Backend is
# archive/verification-only until the reviewed activation lease is implemented.
set -euo pipefail

[ $# -eq 1 ] || { echo 'usage: render-agent-handoff.sh OUTPUT' >&2; exit 2; }
out="$1"
for name in PROFILE DATE_TAG VITE_CHECKIN_CHAIN_ID VITE_CHECKIN_ADDRESS VITE_TRUSTED_PROJECT_CHAINS \
    BACKEND_COMMIT FRONTEND_COMMIT ADMIN_COMMIT AGENT_DIR BACKEND_IMAGE_FILE BACKEND_IMG_PUB_SHA \
    BACKEND_IMAGE_SIZE BACKEND_IMAGE_TAG FRONTEND_PACKAGE_FILE FRONTEND_PKG_PUB_SHA FRONTEND_PACKAGE_SIZE \
    ADMIN_PACKAGE_FILE ADMIN_PKG_PUB_SHA ADMIN_PACKAGE_SIZE; do
    [ -n "${!name:-}" ] || { echo "handoff metadata missing: $name" >&2; exit 2; }
done

cat > "$out" <<EOF
# Agent 交付单 · $PROFILE

构建时间: $DATE_TAG
链身份: CheckIn chain $VITE_CHECKIN_CHAIN_ID / proxy $VITE_CHECKIN_ADDRESS / trusted project chains $VITE_TRUSTED_PROJECT_CHAINS

组件 commit：backend ${BACKEND_COMMIT:0:12} · frontend ${FRONTEND_COMMIT:0:12} · admin ${ADMIN_COMMIT:0:12}
交付目录：$AGENT_DIR

---

## backend（仅归档和离线校验，禁止激活）

\`\`\`text
Package: $AGENT_DIR/$BACKEND_IMAGE_FILE
Size: $BACKEND_IMAGE_SIZE
SHA256: $BACKEND_IMG_PUB_SHA
Layout: docker save gzip（manifest.json + RepoTag $BACKEND_IMAGE_TAG）
Status: fair_launch_activation_unavailable
\`\`\`

当前 release 没有经审查的数据库 activation lease。Backend 镜像不得激活、部署或回滚；仅允许保存和离线校验摘要。

## frontend（仅归档和离线校验，禁止安装）

\`\`\`text
Package: $AGENT_DIR/$FRONTEND_PACKAGE_FILE
Size: $FRONTEND_PACKAGE_SIZE
SHA256: $FRONTEND_PKG_PUB_SHA
Layout: tar -tzf 含顶层 dist/index.html
Status: static_release_admission_unavailable
\`\`\`

仅允许离线执行 SHA-256、成员 allowlist、链接和 bundle identity 校验。不得上传、安装、Apply、restart、upgrade 或 rollback。

## admin（仅归档和离线校验，禁止安装）

\`\`\`text
Package: $AGENT_DIR/$ADMIN_PACKAGE_FILE
Size: $ADMIN_PACKAGE_SIZE
SHA256: $ADMIN_PKG_PUB_SHA
Layout: tar -tzf 含顶层 dist/index.html
Status: static_release_admission_unavailable
\`\`\`

仅允许离线执行 SHA-256、成员 allowlist 和链接校验。不得上传、安装、Apply、restart、upgrade 或 rollback。

---

所有 operational release/deployment 均不可用。Frontend/Admin 写操作返回 \`static_release_admission_unavailable\`；Backend/Job/Lang 返回 \`fair_launch_activation_unavailable\`。
本发布不含合约 / Hardhat / 部署升级脚本 / 密钥；CheckIn 复用既有 BSC 主网 proxy。
EOF

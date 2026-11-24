#!/usr/bin/env bash
# jc-release-server 一键安装（sudo）
#
#   假设：
#   - 你已经在 tools/jc-release 里 go build ./cmd/jc-release-server 得到二进制
#   - Ubuntu 22.04+ / Debian 12+ 系统，systemd

set -euo pipefail

BIN="${1:-./cmd/jc-release-server/jc-release-server}"

if [ "$(id -u)" != "0" ]; then
    echo "需要 sudo 执行" >&2
    exit 1
fi

if [ ! -x "$BIN" ]; then
    echo "找不到二进制 $BIN"
    echo "先在 tools/jc-release/ 里跑:"
    echo "  go build -o cmd/jc-release-server/jc-release-server ./cmd/jc-release-server"
    exit 1
fi

# --- 1. user + 数据目录 ---
if ! id jc-release >/dev/null 2>&1; then
    useradd -r -s /sbin/nologin -d /var/lib/jc-release jc-release
fi
mkdir -p /var/lib/jc-release /etc/jc-release
chown -R jc-release:jc-release /var/lib/jc-release
chmod 750 /var/lib/jc-release

# --- 2. 复制二进制 ---
cp "$BIN" /usr/local/bin/jc-release-server
chmod 755 /usr/local/bin/jc-release-server

# --- 3. 配置模板 ---
CFG=/etc/jc-release/server.yaml
if [ ! -f "$CFG" ]; then
    cp deploy/server-config.example.yaml "$CFG"
    chmod 600 "$CFG"
    chown jc-release:jc-release "$CFG"
    echo ""
    echo "已复制 $CFG"
    echo ""
    echo "== 下一步：编辑 $CFG 填入以下三项 =="
    echo ""
    echo "1) admin_password_bcrypt : 生成:"
    echo "     htpasswd -bnBC 10 \"\" 'YourStrongPassword!' | tr -d ':\\n'"
    echo ""
    echo "2) session_secret :"
    echo "     openssl rand -hex 32"
    echo ""
    echo "3) listen_addr : 保留 127.0.0.1:8090 (nginx 反代) 或改成 0.0.0.0:port"
    echo ""
    echo "编辑好后跑："
    echo "  sudo systemctl enable --now jc-release-server"
    echo ""
else
    echo "$CFG 已存在，跳过配置生成"
fi

# --- 4. systemd ---
cp deploy/systemd/jc-release-server.service /etc/systemd/system/
systemctl daemon-reload

echo ""
echo "==============================="
echo "安装完成。启动命令："
echo "  sudo systemctl enable --now jc-release-server"
echo "  sudo journalctl -fu jc-release-server"
echo "==============================="

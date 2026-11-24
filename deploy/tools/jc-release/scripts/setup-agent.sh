#!/usr/bin/env bash
# jc-agent 一键安装（sudo）—— 在生产/测试服务器上运行
#
# 用法：
#   sudo ./scripts/setup-agent.sh <path-to-jc-agent-binary>
# 例：
#   sudo ./scripts/setup-agent.sh ./cmd/jc-agent/jc-agent

set -euo pipefail

BIN="${1:-./cmd/jc-agent/jc-agent}"

if [ "$(id -u)" != "0" ]; then
    echo "需要 sudo 执行" >&2
    exit 1
fi

if [ ! -x "$BIN" ]; then
    echo "找不到二进制 $BIN"
    echo "先跑: go build -o cmd/jc-agent/jc-agent ./cmd/jc-agent"
    exit 1
fi

# --- 1. 数据目录 ---
mkdir -p /var/lib/jc-agent /etc/jc-agent
chmod 700 /var/lib/jc-agent

# --- 2. 复制二进制 ---
cp "$BIN" /usr/local/bin/jc-agent
chmod 755 /usr/local/bin/jc-agent

# --- 3. 交互填 server_url + enroll_token ---
CFG=/etc/jc-agent/agent.yaml
if [ ! -f "$CFG" ]; then
    cp deploy/agent-config.example.yaml "$CFG"
    chmod 600 "$CFG"
    echo ""
    read -p "jc-release-server 地址 (含 https://): " URL
    read -p "Enroll token (在 Web UI 里生成，15 分钟有效): " TOK
    # sed 替换
    sed -i "s|^server_url:.*|server_url: \"${URL}\"|" "$CFG"
    sed -i "s|^enroll_token:.*|enroll_token: \"${TOK}\"|" "$CFG"
    # All release activation is statically unavailable in this release;
    # setup intentionally installs no activation executable or DB credential.
    echo "配置已写入 $CFG"
else
    echo "$CFG 已存在，跳过交互填写"
fi

# --- 4. systemd ---
cp deploy/systemd/jc-agent.service /etc/systemd/system/
systemctl daemon-reload

# --- 5. 启动 ---
systemctl enable --now jc-agent
sleep 3

echo ""
echo "==============================="
echo "jc-agent 已启动。首次运行会用 enroll_token 换永久 agent_token 并落盘 state.json"
echo ""
echo "跟日志: sudo journalctl -fu jc-agent"
echo "查状态: sudo jc-agent -config $CFG status"
echo "查最新版本: sudo jc-agent -config $CFG check"
echo "==============================="

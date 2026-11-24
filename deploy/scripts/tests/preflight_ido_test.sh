#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

cat > "$TMP_DIR/deploy.env" <<'EOF'
JC_HOST='example.invalid'
JC_REMOTE='/opt/joincare-test'
JC_SSH_USER='root'
JC_COMPOSE_PROJECT='joincare-test'
JC_IDO_JOB_ENABLED='1'
JC_PROJECT_CHAIN_RPC='https://rpc.example.invalid'
JC_PROJECT_CHAIN_ID='97'
JC_PROJECT_V1_ENABLED='1'
JC_PROJECT_CONTRACT='0x4444444444444444444444444444444444444444'
JC_PROJECT_JOB_KEY='test-only-key'
JC_PROJECT_KEEPER_KEY='different-test-only-key'
JC_IDO_V2_CONTRACT='0x5555555555555555555555555555555555555555'
JC_IDO_V2_START_BLOCK='100'
JC_IDO_V2_IMPLEMENTATION='0x8888888888888888888888888888888888888888'
JC_IDO_V2_IMPLEMENTATION_CODE_HASH='0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
JC_IDO_V2_OPERATOR='0x9999999999999999999999999999999999999999'
JC_IDO_V2_FEE_RECEIVER='0x3333333333333333333333333333333333333333'
JC_IDO_V2_SUCCESS_FEE_RATE='200'
JC_IDO_V2_USDT_ADDRESS='0x6666666666666666666666666666666666666666'
JC_IDO_V2_USDT_DECIMALS='6'
JC_IDO_V2_USDC_ADDRESS='0x7777777777777777777777777777777777777777'
JC_IDO_V2_USDC_DECIMALS='18'
EOF

run_case() {
    local name="$1" expected_rc="$2" expected_text="$3" output rc
    set +e
    output=$(PATH="$ROOT/scripts/tests/bin:$PATH" \
        JC_PREFLIGHT_TEST_CASE="$name" \
        JC_ROOT="$ROOT" \
        JC_LIB="$ROOT/scripts/lib" \
        JC_CMD="$ROOT/scripts/cmd" \
        JC_TEMPLATE="$ROOT/config/template" \
        JC_ENV_FILE="$TMP_DIR/deploy.env" \
        bash "$ROOT/scripts/cmd/preflight_ido.sh" 2>&1)
    rc=$?
    set -e
    if [ "$rc" -ne "$expected_rc" ]; then
        echo "case=$name rc=$rc want=$expected_rc" >&2
        echo "$output" >&2
        exit 1
    fi
    if ! grep -q "$expected_text" <<<"$output"; then
        echo "case=$name missing output: $expected_text" >&2
        echo "$output" >&2
        exit 1
    fi
}

run_case ok 0 "预检通过"
run_case wrong-chain 1 "chainId 不匹配"
run_case wrong-contract 1 "jccToken()"
run_case v2-no-code 1 "V2_CONTRACT 地址没有 bytecode"
run_case wrong-v2-implementation 1 "implementation 与批准地址不一致"
run_case v2-token-disabled 1 "治理decimals或allowed无效"
run_case wrong-v2-operator 1 "operator与批准配置不一致"

cp "$TMP_DIR/deploy.env" "$TMP_DIR/deploy-v1.env"
sed -i "s/JC_PROJECT_V1_ENABLED='1'/JC_PROJECT_V1_ENABLED='0'/; s/JC_PROJECT_CONTRACT='[^']*'/JC_PROJECT_CONTRACT=''/" "$TMP_DIR/deploy.env"
run_case v2-only 0 "V1 enabled=0"
mv "$TMP_DIR/deploy-v1.env" "$TMP_DIR/deploy.env"

sed -i "s/JC_PROJECT_V1_ENABLED='1'/JC_PROJECT_V1_ENABLED='0'/; s/JC_PROJECT_CONTRACT='[^']*'/JC_PROJECT_CONTRACT=''/; s/JC_IDO_V2_CONTRACT='[^']*'/JC_IDO_V2_CONTRACT=''/" "$TMP_DIR/deploy.env"
run_case v2-only-missing-v2 1 "禁用V1时必须完整配置"
cp "$TMP_DIR/deploy.env" "$TMP_DIR/deploy-missing.env"
sed -i "s/JC_PROJECT_V1_ENABLED='0'/JC_PROJECT_V1_ENABLED='1'/; s/JC_PROJECT_CONTRACT=''/JC_PROJECT_CONTRACT='0x4444444444444444444444444444444444444444'/; s/JC_IDO_V2_CONTRACT=''/JC_IDO_V2_CONTRACT='0x5555555555555555555555555555555555555555'/" "$TMP_DIR/deploy.env"

sed -i "s/JC_PROJECT_KEEPER_KEY='different-test-only-key'/JC_PROJECT_KEEPER_KEY='test-only-key'/" "$TMP_DIR/deploy.env"
run_case same-relayer-key 1 "必须由 outbox/job 独占"

echo "preflight_ido tests passed"

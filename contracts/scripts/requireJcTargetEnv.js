const required = ["JC_DEPLOY_RPC", "JC_DEPLOY_CHAIN_ID", "JC_DEPLOY_PRIVATE_KEY"];
const missing = required.filter((name) => !String(process.env[name] || "").trim());

if (missing.length > 0) {
  console.error(`jcTarget 配置缺失: ${missing.join(", ")}`);
  console.error("请显式设置 RPC、chainId 和部署私钥；拒绝回退到 localhost。");
  process.exit(1);
}

if (!/^\d+$/.test(process.env.JC_DEPLOY_CHAIN_ID) || BigInt(process.env.JC_DEPLOY_CHAIN_ID) === 0n) {
  console.error("JC_DEPLOY_CHAIN_ID 必须是十进制正整数");
  process.exit(1);
}

if (!/^(0x)?[0-9a-fA-F]{64}$/.test(process.env.JC_DEPLOY_PRIVATE_KEY)) {
  console.error("JC_DEPLOY_PRIVATE_KEY 必须是 64 位 hex（可带 0x 前缀）");
  process.exit(1);
}

const expectedChainId = process.argv[2];
if (expectedChainId && process.env.JC_DEPLOY_CHAIN_ID !== expectedChainId) {
  console.error(`命令目标链不匹配: 配置=${process.env.JC_DEPLOY_CHAIN_ID} 命令要求=${expectedChainId}`);
  process.exit(1);
}

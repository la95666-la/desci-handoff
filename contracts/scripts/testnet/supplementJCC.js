const hre = require("hardhat");
const fs = require("fs");
const path = require("path");

async function main() {
  console.log("\n=== 补充 JCC 代币到测试账户 ===\n");

  // 加载配置
  const accountsPath = path.join(__dirname, "config", "accounts.json");
  const contractsPath = path.join(__dirname, "config", "contracts.json");
  const testParamsPath = path.join(__dirname, "config", "testParams.json");

  const accountsConfig = JSON.parse(fs.readFileSync(accountsPath, "utf8"));
  const contractsConfig = JSON.parse(fs.readFileSync(contractsPath, "utf8"));
  const testParams = JSON.parse(fs.readFileSync(testParamsPath, "utf8"));

  // 获取 deployer 账户
  const [deployer] = await hre.ethers.getSigners();
  console.log("Deployer 地址:", deployer.address);

  // 获取 JCC 代币合约
  const jccToken = await hre.ethers.getContractAt(
    "JCC",
    contractsConfig.contracts.jccToken
  );

  console.log("JCC 代币地址:", contractsConfig.contracts.jccToken);

  // 检查 deployer 的 JCC 余额
  const deployerBalance = await jccToken.balanceOf(deployer.address);
  console.log("Deployer JCC 余额:", hre.ethers.formatEther(deployerBalance), "JCC\n");

  const accounts = accountsConfig.accounts;
  const requiredPerUser = hre.ethers.parseEther(testParams.jccDistribution.perUser.toString());

  let totalNeeded = 0n;
  const transfers = [];

  // 检查每个用户账户需要补充多少
  for (const [role, account] of Object.entries(accounts)) {
    if (role === "deployer" || role === "projectOwner" || role === "feeReceiver") {
      continue; // 跳过这些账户
    }

    const currentBalance = await jccToken.balanceOf(account.address);
    const currentBalanceFormatted = hre.ethers.formatEther(currentBalance);

    console.log(`检查 ${role} (${account.address})`);
    console.log(`  当前余额: ${currentBalanceFormatted} JCC`);
    console.log(`  需要余额: ${testParams.jccDistribution.perUser} JCC`);

    if (currentBalance < requiredPerUser) {
      const needed = requiredPerUser - currentBalance;
      const neededFormatted = hre.ethers.formatEther(needed);
      console.log(`  需要补充: ${neededFormatted} JCC ✅`);

      transfers.push({
        role,
        address: account.address,
        amount: needed,
        amountFormatted: neededFormatted
      });

      totalNeeded += needed;
    } else {
      console.log(`  余额充足 ✅`);
    }
    console.log("");
  }

  if (transfers.length === 0) {
    console.log("✅ 所有账户 JCC 余额充足，无需补充！\n");
    return;
  }

  const totalNeededFormatted = hre.ethers.formatEther(totalNeeded);
  console.log("=".repeat(80));
  console.log(`总共需要补充: ${totalNeededFormatted} JCC`);
  console.log(`Deployer 当前余额: ${hre.ethers.formatEther(deployerBalance)} JCC`);

  if (deployerBalance < totalNeeded) {
    console.log("\n❌ Deployer JCC 余额不足，无法完成补充！");
    console.log(`需要: ${totalNeededFormatted} JCC`);
    console.log(`当前: ${hre.ethers.formatEther(deployerBalance)} JCC`);
    console.log(`缺少: ${hre.ethers.formatEther(totalNeeded - deployerBalance)} JCC\n`);
    process.exit(1);
  }

  console.log("=".repeat(80));
  console.log("\n开始转账...\n");

  // 执行转账
  for (const transfer of transfers) {
    console.log(`转账给 ${transfer.role} (${transfer.address})`);
    console.log(`  金额: ${transfer.amountFormatted} JCC`);

    try {
      const tx = await jccToken.transfer(transfer.address, transfer.amount);
      console.log(`  交易哈希: ${tx.hash}`);

      const receipt = await tx.wait();
      console.log(`  ✅ 转账成功 (Gas 使用: ${receipt.gasUsed.toString()})`);

      // 验证余额
      const newBalance = await jccToken.balanceOf(transfer.address);
      console.log(`  新余额: ${hre.ethers.formatEther(newBalance)} JCC`);
    } catch (error) {
      console.log(`  ❌ 转账失败: ${error.message}`);
    }
    console.log("");
  }

  // 最终检查
  console.log("=".repeat(80));
  console.log("\n最终余额检查:\n");

  for (const [role, account] of Object.entries(accounts)) {
    if (role === "deployer" || role === "projectOwner" || role === "feeReceiver") {
      continue;
    }

    const balance = await jccToken.balanceOf(account.address);
    const balanceFormatted = hre.ethers.formatEther(balance);
    const sufficient = balance >= requiredPerUser ? "✅" : "⚠️";

    console.log(`${role}: ${balanceFormatted} JCC ${sufficient}`);
  }

  const finalDeployerBalance = await jccToken.balanceOf(deployer.address);
  console.log(`\nDeployer 剩余: ${hre.ethers.formatEther(finalDeployerBalance)} JCC`);

  console.log("\n✅ JCC 补充完成！\n");
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });

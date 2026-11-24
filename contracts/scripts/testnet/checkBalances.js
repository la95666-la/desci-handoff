const hre = require("hardhat");
const fs = require("fs");
const path = require("path");

async function main() {
  console.log("\n=== 检查测试账户余额 ===\n");

  // 加载配置
  const accountsPath = path.join(__dirname, "config", "accounts.json");
  const contractsPath = path.join(__dirname, "config", "contracts.json");
  const testParamsPath = path.join(__dirname, "config", "testParams.json");

  const accountsConfig = JSON.parse(fs.readFileSync(accountsPath, "utf8"));
  const contractsConfig = JSON.parse(fs.readFileSync(contractsPath, "utf8"));
  const testParams = JSON.parse(fs.readFileSync(testParamsPath, "utf8"));

  // 获取 JCC 代币合约
  const jccToken = await hre.ethers.getContractAt(
    "JCC",
    contractsConfig.contracts.jccToken
  );

  console.log("📋 账户余额检查\n");
  console.log("JCC 代币地址:", contractsConfig.contracts.jccToken);
  console.log("平台合约地址:", contractsConfig.contracts.joinCarePlatform);
  console.log("\n" + "=".repeat(100) + "\n");

  const accounts = accountsConfig.accounts;
  let totalBNB = 0n;
  let totalJCC = 0n;
  const insufficientAccounts = [];

  // 检查每个账户
  for (const [role, account] of Object.entries(accounts)) {
    if (role === "deployer") continue; // 跳过 deployer，因为私钥不在配置中

    console.log(`👤 ${role.toUpperCase()} (${account.description})`);
    console.log(`   地址: ${account.address}`);

    try {
      // 获取 BNB 余额
      const bnbBalance = await hre.ethers.provider.getBalance(account.address);
      const bnbFormatted = hre.ethers.formatEther(bnbBalance);

      // 获取 JCC 余额
      const jccBalance = await jccToken.balanceOf(account.address);
      const jccFormatted = hre.ethers.formatEther(jccBalance);

      console.log(`   BNB 余额: ${bnbFormatted} BNB`);
      console.log(`   JCC 余额: ${jccFormatted} JCC`);

      totalBNB += bnbBalance;
      totalJCC += jccBalance;

      // 检查余额是否充足
      const requiredBNB = role.startsWith("user")
        ? testParams.bnbDistribution.users
        : (testParams.bnbDistribution[role] || "0.01");
      const requiredJCC = testParams.jccDistribution.perUser || "10000";

      const bnbSufficient = parseFloat(bnbFormatted) >= parseFloat(requiredBNB);
      const jccSufficient = role === "projectOwner" || role === "feeReceiver"
        ? true
        : parseFloat(jccFormatted) >= parseFloat(requiredJCC);

      if (!bnbSufficient || !jccSufficient) {
        insufficientAccounts.push({
          role,
          address: account.address,
          bnbCurrent: bnbFormatted,
          bnbRequired: requiredBNB,
          jccCurrent: jccFormatted,
          jccRequired: requiredJCC,
          bnbSufficient,
          jccSufficient
        });
        console.log(`   ⚠️  余额不足！`);
        if (!bnbSufficient) {
          console.log(`      需要 BNB: ${requiredBNB}, 当前: ${bnbFormatted}`);
        }
        if (!jccSufficient) {
          console.log(`      需要 JCC: ${requiredJCC}, 当前: ${jccFormatted}`);
        }
      } else {
        console.log(`   ✅ 余额充足`);
      }

    } catch (error) {
      console.log(`   ❌ 查询失败: ${error.message}`);
      insufficientAccounts.push({
        role,
        address: account.address,
        error: error.message
      });
    }

    console.log("");
  }

  // 检查 deployer 账户
  console.log(`👤 DEPLOYER (合约部署者和资产分发者)`);
  console.log(`   地址: ${accounts.deployer.address}`);
  try {
    const bnbBalance = await hre.ethers.provider.getBalance(accounts.deployer.address);
    const bnbFormatted = hre.ethers.formatEther(bnbBalance);

    const jccBalance = await jccToken.balanceOf(accounts.deployer.address);
    const jccFormatted = hre.ethers.formatEther(jccBalance);

    console.log(`   BNB 余额: ${bnbFormatted} BNB`);
    console.log(`   JCC 余额: ${jccFormatted} JCC`);

    totalBNB += bnbBalance;
    totalJCC += jccBalance;

    const requiredBNB = testParams.budgets.reserveBNB || "0.7535";
    const bnbSufficient = parseFloat(bnbFormatted) >= parseFloat(requiredBNB);

    if (!bnbSufficient) {
      insufficientAccounts.push({
        role: "deployer",
        address: accounts.deployer.address,
        bnbCurrent: bnbFormatted,
        bnbRequired: requiredBNB,
        bnbSufficient: false
      });
      console.log(`   ⚠️  BNB 余额不足！需要: ${requiredBNB}, 当前: ${bnbFormatted}`);
    } else {
      console.log(`   ✅ 余额充足`);
    }
  } catch (error) {
    console.log(`   ❌ 查询失败: ${error.message}`);
  }

  console.log("\n" + "=".repeat(100) + "\n");

  // 总计
  console.log("📊 总计");
  console.log(`   总 BNB: ${hre.ethers.formatEther(totalBNB)} BNB`);
  console.log(`   总 JCC: ${hre.ethers.formatEther(totalJCC)} JCC`);
  console.log("");

  // 余额不足的账户
  if (insufficientAccounts.length > 0) {
    console.log("⚠️  以下账户余额不足:\n");
    for (const account of insufficientAccounts) {
      if (account.error) {
        console.log(`   ${account.role}: 查询失败 - ${account.error}`);
      } else {
        console.log(`   ${account.role} (${account.address}):`);
        if (!account.bnbSufficient) {
          console.log(`      BNB: 需要 ${account.bnbRequired}, 当前 ${account.bnbCurrent}`);
        }
        if (account.jccSufficient === false) {
          console.log(`      JCC: 需要 ${account.jccRequired}, 当前 ${account.jccCurrent}`);
        }
      }
    }
    console.log("\n请先为这些账户充值后再运行测试！\n");
    process.exit(1);
  } else {
    console.log("✅ 所有账户余额充足，可以开始测试！\n");
  }
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });

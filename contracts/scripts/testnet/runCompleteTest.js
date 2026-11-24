const hre = require("hardhat");
const fs = require("fs");
const path = require("path");
const { validateE2EEnvironment } = require("./utils/testHelper");

/**
 * 完整测试流程 - 快速版本（20分钟内完成）
 *
 * 测试流程：
 * 1. 部署验证
 * 2. 创建项目（1分钟后开始，持续5分钟）
 * 3. 项目方充值代币
 * 4. 等待项目激活
 * 5. 用户认购（确保达到软顶）
 * 6. 等待项目结束
 * 7. 代币领取测试（TGE + 3次vesting）
 * 8. 资金提取测试
 *
 * 时间线：
 * - 项目开始延迟: 60秒（1分钟）
 * - 项目持续时间: 300秒（5分钟）
 * - TGE后 180秒（3分钟）→ 释放 50%
 * - TGE后 480秒（8分钟）→ 释放 30%
 * - TGE后 780秒（13分钟）→ 释放 20%
 *
 * 总时间：约 19 分钟
 */

async function main() {
  console.log("\n" + "=".repeat(100));
  console.log("=== JoinCare IDO 平台 - 完整测试流程（快速版本）===");
  console.log("=".repeat(100) + "\n");

  const startTime = Date.now();

  // 加载配置
  const configDir = path.join(__dirname, "config");
  const accountsConfig = JSON.parse(fs.readFileSync(path.join(configDir, "accounts.json"), "utf8"));
  const contractsConfig = JSON.parse(fs.readFileSync(path.join(configDir, "contracts.json"), "utf8"));
  const testParams = JSON.parse(fs.readFileSync(path.join(configDir, "testParams.json"), "utf8"));
	const environment = await validateE2EEnvironment(contractsConfig);

  console.log("📋 测试配置");
	console.log(`  网络: chainId=${environment.network.chainId}`);
  console.log("  JCC 代币:", contractsConfig.contracts.jccToken);
  console.log("  平台合约:", contractsConfig.contracts.joinCarePlatform);
  console.log("  项目代币:", contractsConfig.contracts.projectToken);
  console.log("\n  时间配置:");
  console.log(`    项目开始延迟: ${testParams.timeConfig.projectStartDelay}秒`);
  console.log(`    项目持续时间: ${testParams.timeConfig.projectDuration}秒`);
  console.log(`    Vesting 延迟: ${testParams.timeConfig.vestingDelays.join(", ")}秒`);
  console.log(`    Vesting 比例: ${testParams.timeConfig.vestingPercents.map(p => p / 100 + "%").join(", ")}`);
  console.log("\n" + "=".repeat(100) + "\n");

  // 获取账户
  const [deployer] = await hre.ethers.getSigners();
  const projectOwner = new hre.ethers.Wallet(
    accountsConfig.accounts.projectOwner.privateKey,
    hre.ethers.provider
  );
  const users = [
    new hre.ethers.Wallet(accountsConfig.accounts.user1.privateKey, hre.ethers.provider),
    new hre.ethers.Wallet(accountsConfig.accounts.user2.privateKey, hre.ethers.provider),
    new hre.ethers.Wallet(accountsConfig.accounts.user3.privateKey, hre.ethers.provider),
    new hre.ethers.Wallet(accountsConfig.accounts.user4.privateKey, hre.ethers.provider),
    new hre.ethers.Wallet(accountsConfig.accounts.user5.privateKey, hre.ethers.provider),
  ];

  // 获取合约实例
  const jccToken = await hre.ethers.getContractAt("JCC", contractsConfig.contracts.jccToken);
  const platform = await hre.ethers.getContractAt("JoinCare_v1", contractsConfig.contracts.joinCarePlatform);
  const projectToken = await hre.ethers.getContractAt("JCC", contractsConfig.contracts.projectToken);

  // 测试结果记录
  const testResults = {
    startTime: new Date().toISOString(),
    tests: [],
    summary: {
      total: 0,
      passed: 0,
      failed: 0
    }
  };

  function logTest(module, name, passed, details = {}) {
    testResults.tests.push({ module, name, passed, details, timestamp: new Date().toISOString() });
    testResults.summary.total++;
    if (passed) {
      testResults.summary.passed++;
      console.log(`  ✅ ${name}`);
    } else {
      testResults.summary.failed++;
      console.log(`  ❌ ${name}`);
      if (details.error) {
        console.log(`     错误: ${details.error}`);
      }
    }
  }

  try {
    // ========== 预检：确保账户余额满足测试需求 ==========
    console.log("🔍 预检：检查并补充账户余额\n");

    // 每个 user 需要至少 maxPerUser 数量的 JCC
    const requiredJCC = hre.ethers.parseEther(testParams.projectParams.maxPerUser);
    // projectOwner 需要至少 hardCap / tokenPrice 数量的 projectToken（即与硬顶等量）
    const tokenPrice_pre = hre.ethers.parseEther(testParams.projectParams.tokenPrice);
    const hardCap_pre = hre.ethers.parseEther(testParams.projectParams.hardCap);
    const requiredPT = (hardCap_pre * hre.ethers.parseEther("1")) / tokenPrice_pre;

    // 1. 检查并补充每个 user 的 JCC 余额
    for (let i = 0; i < users.length; i++) {
      const balance = await jccToken.balanceOf(users[i].address);
      console.log(`  user${i + 1} JCC 余额: ${hre.ethers.formatEther(balance)} JCC (最低需求: ${hre.ethers.formatEther(requiredJCC)} JCC)`);
      if (balance < requiredJCC) {
        const mintAmount = requiredJCC - balance;
        console.log(`    ⚠️  余额不足，正在补充 ${hre.ethers.formatEther(mintAmount)} JCC...`);
        const mintTx = await jccToken.connect(deployer).mint(users[i].address, mintAmount);
        await mintTx.wait();
        console.log(`    ✅ 补充完成`);
      } else {
        console.log(`    ✅ 余额充足`);
      }
    }

    // 2. 检查并补充 projectOwner 的 projectToken 余额
    const ptBalance = await projectToken.balanceOf(projectOwner.address);
    console.log(`\n  projectOwner PT 余额: ${hre.ethers.formatEther(ptBalance)} PT (最低需求: ${hre.ethers.formatEther(requiredPT)} PT)`);
    if (ptBalance < requiredPT) {
      const mintAmount = requiredPT - ptBalance;
      console.log(`    ⚠️  余额不足，正在补充 ${hre.ethers.formatEther(mintAmount)} PT...`);
      const mintTx = await projectToken.connect(deployer).mint(projectOwner.address, mintAmount);
      await mintTx.wait();
      console.log(`    ✅ 补充完成`);
    } else {
      console.log(`    ✅ 余额充足`);
    }

    console.log("\n  ✅ 账户余额预检完成\n");
    console.log("=".repeat(100) + "\n");

    // ========== 模块 1: 部署验证 ==========
    console.log("📦 模块 1: 部署验证\n");

    try {
      const jccName = await jccToken.name();
      const jccSymbol = await jccToken.symbol();
      logTest("部署验证", "JCC 代币配置正确", jccName === "JoinCare Coin" && jccSymbol === "JCC", {
        name: jccName,
        symbol: jccSymbol
      });
    } catch (error) {
      logTest("部署验证", "JCC 代币配置正确", false, { error: error.message });
    }

    try {
      const platformFees = await platform.platformFees();
      logTest("部署验证", "平台费用配置正确", platformFees.successFeeRate === 200n, {
        feeRate: platformFees.successFeeRate.toString()
      });
    } catch (error) {
      logTest("部署验证", "平台费用配置正确", false, { error: error.message });
    }

    try {
      const ptName = await projectToken.name();
      const ptSymbol = await projectToken.symbol();
      const ptDecimals = await projectToken.decimals();
      logTest("部署验证", "项目代币部署正确", ptDecimals === 18n && ptSymbol.length > 0, {
        name: ptName,
        symbol: ptSymbol,
        decimals: ptDecimals.toString()
      });
    } catch (error) {
      logTest("部署验证", "项目代币部署正确", false, { error: error.message });
    }

    console.log("");

    // ========== 模块 2: 创建项目 ==========
    console.log("🚀 模块 2: 创建项目\n");

    const currentTime = Math.floor(Date.now() / 1000);
    const startTime_project = currentTime + testParams.timeConfig.projectStartDelay;
    const endTime = startTime_project + testParams.timeConfig.projectDuration;

    const tokenPrice = hre.ethers.parseEther(testParams.projectParams.tokenPrice);
    const hardCap = hre.ethers.parseEther(testParams.projectParams.hardCap);
    const softCap = hre.ethers.parseEther(testParams.projectParams.softCap);
    const maxPerUser = hre.ethers.parseEther(testParams.projectParams.maxPerUser);

    // 计算 vesting 时间（绝对时间戳，从项目结束时间开始）
    const vestingTimes = testParams.timeConfig.vestingDelays.map(delay => endTime + delay);
    const vestingDuration = testParams.timeConfig.vestingDelays[testParams.timeConfig.vestingDelays.length - 1];

    let projectId;
    try {
      console.log("  创建项目参数:");
      console.log(`    开始时间: ${new Date(startTime_project * 1000).toLocaleString()}`);
      console.log(`    结束时间: ${new Date(endTime * 1000).toLocaleString()}`);
      console.log(`    代币价格: ${testParams.projectParams.tokenPrice} JCC = 1 PT`);
      console.log(`    硬顶: ${testParams.projectParams.hardCap} JCC`);
      console.log(`    软顶: ${testParams.projectParams.softCap} JCC`);
      console.log(`    单用户限额: ${testParams.projectParams.maxPerUser} JCC\n`);

      // 生成项目ID
      projectId = BigInt(Date.now()) * 1000000n + BigInt(Math.floor(Math.random() * 1000000));
      console.log(`  生成项目 ID: ${projectId.toString()}\n`);

      const tx = await platform.connect(deployer).createProject(
        projectId,
        projectToken.target,
        projectOwner.address,
        tokenPrice,
        hardCap,
        softCap,
        maxPerUser,
        startTime_project,
        endTime,
        vestingTimes,
        testParams.timeConfig.vestingPercents,
        vestingDuration
      );

      const receipt = await tx.wait();

      console.log(`  项目 ID: ${projectId.toString()}`);
      console.log(`  交易哈希: ${tx.hash}`);
      console.log(`  Gas 使用: ${receipt.gasUsed.toString()}\n`);
      logTest("项目创建", "创建项目成功", true, { projectId: projectId.toString(), txHash: tx.hash });
    } catch (error) {
      logTest("项目创建", "创建项目成功", false, { error: error.message });
      console.log("\n❌ 项目创建失败，无法继续测试\n");
      return;
    }

    console.log("");

    // ========== 模块 3: 代币充值 ==========
    console.log("💰 模块 3: 代币充值\n");

    try {
      const requiredTokens = (hardCap * hre.ethers.parseEther("1")) / tokenPrice;
      console.log(`  需要充值: ${hre.ethers.formatEther(requiredTokens)} PT\n`);

      // 授权
      const approveTx = await projectToken.connect(projectOwner).approve(platform.target, requiredTokens);
      await approveTx.wait();
      console.log(`  ✅ 授权成功`);

      // 充值
      const depositTx = await platform.connect(projectOwner).depositProjectTokens(projectId, requiredTokens);
      const depositReceipt = await depositTx.wait();
      console.log(`  ✅ 充值成功 (Gas: ${depositReceipt.gasUsed.toString()})\n`);

      // 验证（字段名：totalTokensDeposited）
      const project = await platform.projects(projectId);
      logTest("代币充值", "充值金额正确", project.totalTokensDeposited === requiredTokens, {
        deposited: hre.ethers.formatEther(project.totalTokensDeposited),
        required: hre.ethers.formatEther(requiredTokens)
      });
    } catch (error) {
      logTest("代币充值", "充值金额正确", false, { error: error.message });
    }

    console.log("");

    // ========== 模块 4: 手动激活项目 ==========
    console.log("⏳ 模块 4: 手动激活项目\n");

    const waitTime = startTime_project - Math.floor(Date.now() / 1000) + 5;
    if (waitTime > 0) {
      console.log(`  等待 ${waitTime} 秒直到项目开始时间...\n`);
      await new Promise(resolve => setTimeout(resolve, waitTime * 1000));
    }

    try {
      // 调用合约手动激活方法（仅 owner 可调用）
      const activateTx = await platform.connect(deployer).activateProject(projectId);
      await activateTx.wait();
      console.log(`  ✅ activateProject() 交易已确认\n`);

      // 验证状态
      const project = await platform.projects(projectId);
      const statusNames = ["Pending", "Active", "Ended", "Successful", "Failed", "Cancelled"];
      const status = statusNames[project.status];
      logTest("项目激活", "手动激活项目", status === "Active", { status });
    } catch (error) {
      logTest("项目激活", "手动激活项目", false, { error: error.message });
    }

    console.log("");

    // ========== 模块 5: 用户认购 ==========
    console.log("🛒 模块 5: 用户认购\n");

    // 确保达到软顶：5个用户各认购 1000 JCC = 5000 JCC = 软顶
    const purchaseAmount = hre.ethers.parseEther("1000");
    let totalPurchased = 0n;

    for (let i = 0; i < users.length; i++) {
      try {
        // 授权（等待链上确认，避免 approve 未生效就发 purchase）
        const approveTx = await jccToken.connect(users[i]).approve(platform.target, purchaseAmount);
        await approveTx.wait();

        // 认购
        const tx = await platform.connect(users[i]).purchase(projectId, purchaseAmount);
        await tx.wait();

        totalPurchased += purchaseAmount;
        console.log(`  ✅ user${i + 1} 认购 ${hre.ethers.formatEther(purchaseAmount)} JCC`);
      } catch (error) {
        console.log(`  ❌ user${i + 1} 认购失败: ${error.message}`);
      }
    }

    console.log(`\n  总认购: ${hre.ethers.formatEther(totalPurchased)} JCC`);
    logTest("用户认购", "达到软顶", totalPurchased >= softCap, {
      purchased: hre.ethers.formatEther(totalPurchased),
      softCap: hre.ethers.formatEther(softCap)
    });

    console.log("");

    // ========== 模块 6: 等待项目结束 ==========
    console.log("⏳ 模块 6: 等待项目结束\n");

    const waitTimeEnd = endTime - Math.floor(Date.now() / 1000) + 5;
    if (waitTimeEnd > 0) {
      console.log(`  等待 ${waitTimeEnd} 秒直到项目结束...\n`);
      await new Promise(resolve => setTimeout(resolve, waitTimeEnd * 1000));
    }

    try {
      // 调用 endProject
      const endTx = await platform.endProject(projectId);
      await endTx.wait();
      console.log(`  ✅ 项目结束交易已确认\n`);

      const project = await platform.projects(projectId);
      const statusNames = ["Pending", "Active", "Ended", "Successful", "Failed", "Cancelled"];
      const status = statusNames[project.status];
      console.log(`  项目状态: ${status}\n`);
      logTest("项目结束", "项目标记为成功", status === "Successful", { status });
    } catch (error) {
      logTest("项目结束", "项目标记为成功", false, { error: error.message });
    }

    console.log("");

    // ========== 模块 7: 代币领取测试 ==========
    // 合约无 TGE 立即释放，vestingTimes[0] 是第一个释放时间点（项目结束后 180 秒）
    // 释放比例：[50%, 30%, 20%]，对应 3 次独立的 Claim
    console.log("🎁 模块 7: 代币领取测试\n");

    // user1 每次认购 1000 JCC，tokenPrice = 1e18 JCC/PT，总获得 1000 PT
    const totalTokensBought = (purchaseAmount * hre.ethers.parseEther("1")) / tokenPrice;

    // 第一次 vesting（项目结束后 180 秒，释放 50%）
    console.log("  📍 第一次 Vesting（项目结束后 3 分钟，释放 50%）\n");
    const vestingWait1 = vestingTimes[0] - Math.floor(Date.now() / 1000) + 5;
    if (vestingWait1 > 0) {
      console.log(`  等待 ${vestingWait1} 秒到达第一次释放时间...\n`);
      await new Promise(resolve => setTimeout(resolve, vestingWait1 * 1000));
    }

    try {
      const user = users[0];
      const balanceBefore = await projectToken.balanceOf(user.address);

      const tx = await platform.connect(user).claim(projectId);
      await tx.wait();

      const balanceAfter = await projectToken.balanceOf(user.address);
      const claimed = balanceAfter - balanceBefore;
      const expected = totalTokensBought * BigInt(testParams.timeConfig.vestingPercents[0]) / 10000n;

      console.log(`  user1 领取 ${hre.ethers.formatEther(claimed)} PT（预期 ${hre.ethers.formatEther(expected)} PT）`);
      logTest("代币领取", `第一次 Vesting ${testParams.timeConfig.vestingPercents[0] / 100}%`, claimed === expected, {
        claimed: hre.ethers.formatEther(claimed),
        expected: hre.ethers.formatEther(expected)
      });
    } catch (error) {
      logTest("代币领取", `第一次 Vesting ${testParams.timeConfig.vestingPercents[0] / 100}%`, false, { error: error.message });
    }

    // 第二次 vesting（项目结束后 480 秒，释放 30%）
    console.log(`\n  📍 第二次 Vesting（项目结束后 8 分钟，释放 30%）\n`);
    const vestingWait2 = vestingTimes[1] - Math.floor(Date.now() / 1000) + 5;
    if (vestingWait2 > 0) {
      console.log(`  等待 ${vestingWait2} 秒到达第二次释放时间...\n`);
      await new Promise(resolve => setTimeout(resolve, vestingWait2 * 1000));
    }

    try {
      const user = users[0];
      const balanceBefore = await projectToken.balanceOf(user.address);

      const tx = await platform.connect(user).claim(projectId);
      await tx.wait();

      const balanceAfter = await projectToken.balanceOf(user.address);
      const claimed = balanceAfter - balanceBefore;
      const expected = totalTokensBought * BigInt(testParams.timeConfig.vestingPercents[1]) / 10000n;

      console.log(`  user1 领取 ${hre.ethers.formatEther(claimed)} PT（预期 ${hre.ethers.formatEther(expected)} PT）`);
      logTest("代币领取", `第二次 Vesting ${testParams.timeConfig.vestingPercents[1] / 100}%`, claimed === expected, {
        claimed: hre.ethers.formatEther(claimed),
        expected: hre.ethers.formatEther(expected)
      });
    } catch (error) {
      logTest("代币领取", `第二次 Vesting ${testParams.timeConfig.vestingPercents[1] / 100}%`, false, { error: error.message });
    }

    // 第三次 vesting（项目结束后 780 秒，释放 20%）
    console.log(`\n  📍 第三次 Vesting（项目结束后 13 分钟，释放 20%）\n`);
    const vestingWait3 = vestingTimes[2] - Math.floor(Date.now() / 1000) + 5;
    if (vestingWait3 > 0) {
      console.log(`  等待 ${vestingWait3} 秒到达第三次释放时间...\n`);
      await new Promise(resolve => setTimeout(resolve, vestingWait3 * 1000));
    }

    try {
      const user = users[0];
      const balanceBefore = await projectToken.balanceOf(user.address);

      const tx = await platform.connect(user).claim(projectId);
      await tx.wait();

      const balanceAfter = await projectToken.balanceOf(user.address);
      const claimed = balanceAfter - balanceBefore;
      const expected = totalTokensBought * BigInt(testParams.timeConfig.vestingPercents[2]) / 10000n;

      console.log(`  user1 领取 ${hre.ethers.formatEther(claimed)} PT（预期 ${hre.ethers.formatEther(expected)} PT）`);
      logTest("代币领取", `第三次 Vesting ${testParams.timeConfig.vestingPercents[2] / 100}%`, claimed === expected, {
        claimed: hre.ethers.formatEther(claimed),
        expected: hre.ethers.formatEther(expected)
      });
    } catch (error) {
      logTest("代币领取", `第三次 Vesting ${testParams.timeConfig.vestingPercents[2] / 100}%`, false, { error: error.message });
    }

    console.log("");

    // ========== 模块 8: 资金提取测试 ==========
    console.log("💸 模块 8: 资金提取测试\n");

    try {
      const project = await platform.projects(projectId);
      const platformFees = await platform.platformFees();
      const feeAmount = (project.totalRaised * platformFees.successFeeRate) / 10000n;
      const netAmount = project.totalRaised - feeAmount;

      console.log(`  总募集: ${hre.ethers.formatEther(project.totalRaised)} JCC`);
      console.log(`  平台费用 (2%): ${hre.ethers.formatEther(feeAmount)} JCC`);
      console.log(`  净收入: ${hre.ethers.formatEther(netAmount)} JCC\n`);

      // 项目方提取募集资金
      const balanceBefore = await jccToken.balanceOf(projectOwner.address);
      const tx = await platform.connect(projectOwner).withdrawRaisedFunds(projectId);
      await tx.wait();
      const balanceAfter = await jccToken.balanceOf(projectOwner.address);
      const withdrawn = balanceAfter - balanceBefore;

      console.log(`  ✅ 项目方提取 ${hre.ethers.formatEther(withdrawn)} JCC`);
      logTest("资金提取", "提取募集资金", withdrawn === netAmount, {
        withdrawn: hre.ethers.formatEther(withdrawn),
        expected: hre.ethers.formatEther(netAmount)
      });

      // 项目方提取剩余代币（字段名：totalTokensDeposited, totalTokensSold）
      const remainingTokens = project.totalTokensDeposited - project.totalTokensSold;
      if (remainingTokens > 0n) {
        const ptBalanceBefore = await projectToken.balanceOf(projectOwner.address);
        const tx2 = await platform.connect(projectOwner).withdrawProjectTokens(projectId);
        await tx2.wait();
        const ptBalanceAfter = await projectToken.balanceOf(projectOwner.address);
        const withdrawnPT = ptBalanceAfter - ptBalanceBefore;

        console.log(`  ✅ 项目方提取剩余代币 ${hre.ethers.formatEther(withdrawnPT)} PT`);
        logTest("资金提取", "提取剩余代币", withdrawnPT === remainingTokens, {
          withdrawn: hre.ethers.formatEther(withdrawnPT),
          expected: hre.ethers.formatEther(remainingTokens)
        });
      } else {
        console.log(`  ℹ️  无剩余代币需要提取`);
        logTest("资金提取", "提取剩余代币", true, { remaining: "0" });
      }
    } catch (error) {
      logTest("资金提取", "提取募集资金", false, { error: error.message });
    }

    console.log("");

  } catch (error) {
    console.error("\n❌ 测试过程中发生错误:", error);
  }

  // ========== 测试总结 ==========
  const endTime_test = Date.now();
  const duration = Math.floor((endTime_test - startTime) / 1000);

  console.log("\n" + "=".repeat(100));
  console.log("📊 测试总结");
  console.log("=".repeat(100) + "\n");

  console.log(`总测试数: ${testResults.summary.total}`);
  console.log(`通过: ${testResults.summary.passed} ✅`);
  console.log(`失败: ${testResults.summary.failed} ❌`);
  console.log(`通过率: ${((testResults.summary.passed / testResults.summary.total) * 100).toFixed(1)}%`);
  console.log(`\n总耗时: ${Math.floor(duration / 60)} 分 ${duration % 60} 秒`);

  // 保存测试报告
  testResults.endTime = new Date().toISOString();
  testResults.duration = duration;

  const reportPath = path.join(__dirname, "reports", "fullTestReport.json");
  fs.writeFileSync(reportPath, JSON.stringify(testResults, null, 2));
  console.log(`\n测试报告已保存: ${reportPath}`);

  console.log("\n" + "=".repeat(100) + "\n");
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });

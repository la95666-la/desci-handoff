/**
 * 触发所有合约事件的测试脚本
 * 
 * 事件列表:
 * 1. PlatformFeesUpdated - 更新平台费率
 * 2. ProjectCreated - 创建项目
 * 3. ProjectTokensDeposited - 充值项目代币
 * 4. ProjectActivated - 激活项目
 * 5. Purchase - 用户认购
 * 6. ProjectEnded - 项目结束
 * 7. Claimed - 领取代币（需要项目成功+释放时间到）
 * 8. Refunded - 退款（需要项目失败）
 * 9. RaisedFundsWithdrawn - 提取募集资金
 * 10. ProjectTokensWithdrawn - 提取项目代币
 * 11. ProjectCancelled - 取消项目（合约里没有这个函数，跳过）
 */

const { ethers } = require("hardhat");

// 合约地址
const JOINCARE_PROXY = "0xe6F01eC4dEA815c9dAAb79d7A7D0FA16b2cB368A";
const JCC_ADDRESS = "0xFc6C84ac73c11f6C162B9dd5B4A94e7bfb3308EA";

async function main() {
  const [owner] = await ethers.getSigners();
  console.log("Owner 地址:", owner.address);
  console.log("BNB 余额:", ethers.formatEther(await ethers.provider.getBalance(owner.address)));

  // 获取合约实例
  const joincare = await ethers.getContractAt("JoinCare_v1", JOINCARE_PROXY);
  const jcc = await ethers.getContractAt("JCC", JCC_ADDRESS);

  const jccBalance = await jcc.balanceOf(owner.address);
  console.log("JCC 余额:", ethers.formatEther(jccBalance));

  // ============ 1. PlatformFeesUpdated ============
  console.log("\n--- 1. 触发 PlatformFeesUpdated ---");
  try {
    const tx1 = await joincare.setPlatformFees(200, owner.address);
    await tx1.wait();
    console.log("✅ PlatformFeesUpdated 触发成功, tx:", tx1.hash);
  } catch (e) {
    console.log("❌ PlatformFeesUpdated 失败:", e.message?.substring(0, 100));
  }

  // ============ 2. ProjectCreated (项目A - 成功流程) ============
  console.log("\n--- 2. 触发 ProjectCreated (项目 100) ---");
  const projectIdA = 100;
  const now = Math.floor(Date.now() / 1000);
  const startTime = now + 5; // 5秒后开始
  const endTime = now + 120; // 2分钟后结束
  const vestingTime1 = endTime + 10; // 结束后10秒释放
  const tokenPrice = ethers.parseEther("1"); // 1 JCC = 1 token
  const hardCap = ethers.parseEther("100"); // 100 JCC 硬顶
  const softCap = ethers.parseEther("10"); // 10 JCC 软顶
  const maxPerUser = ethers.parseEther("100");

  // 需要一个18位精度的项目代币。这里复用JCC作为项目代币（简化测试）
  try {
    const tx2 = await joincare.createProject(
      projectIdA,
      JCC_ADDRESS, // 用JCC当项目代币（18 decimals）
      owner.address, // projectOwner
      tokenPrice,
      hardCap,
      softCap,
      maxPerUser,
      startTime,
      endTime,
      [vestingTime1], // 一次性全部释放
      [10000], // 100%
      0 // vestingDuration
    );
    await tx2.wait();
    console.log("✅ ProjectCreated 触发成功, tx:", tx2.hash);
  } catch (e) {
    console.log("❌ ProjectCreated 失败:", e.message?.substring(0, 200));
  }

  // ============ 3. ProjectTokensDeposited ============
  console.log("\n--- 3. 触发 ProjectTokensDeposited ---");
  try {
    // 先approve
    const approveAmount = ethers.parseEther("1000");
    const txApprove = await jcc.approve(JOINCARE_PROXY, approveAmount);
    await txApprove.wait();
    console.log("  JCC approve 完成");

    const depositAmount = ethers.parseEther("200");
    const tx3 = await joincare.depositProjectTokens(projectIdA, depositAmount);
    await tx3.wait();
    console.log("✅ ProjectTokensDeposited 触发成功, tx:", tx3.hash);
  } catch (e) {
    console.log("❌ ProjectTokensDeposited 失败:", e.message?.substring(0, 200));
  }

  // ============ 4. ProjectActivated ============
  console.log("\n--- 4. 触发 ProjectActivated ---");
  try {
    const tx4 = await joincare.activateProject(projectIdA);
    await tx4.wait();
    console.log("✅ ProjectActivated 触发成功, tx:", tx4.hash);
  } catch (e) {
    console.log("❌ ProjectActivated 失败:", e.message?.substring(0, 200));
  }

  // ============ 5. Purchase ============
  console.log("\n--- 5. 等待认购开始时间... ---");
  // 等待 startTime 到达
  while (Math.floor(Date.now() / 1000) < startTime + 2) {
    await new Promise(r => setTimeout(r, 1000));
    process.stdout.write(".");
  }
  console.log("\n--- 5. 触发 Purchase ---");
  try {
    const purchaseAmount = ethers.parseEther("50"); // 认购50 JCC
    const tx5 = await joincare.purchase(projectIdA, purchaseAmount);
    await tx5.wait();
    console.log("✅ Purchase 触发成功, tx:", tx5.hash);
  } catch (e) {
    console.log("❌ Purchase 失败:", e.message?.substring(0, 200));
  }

  // ============ 6. ProjectEnded (owner可以随时结束) ============
  console.log("\n--- 6. 触发 ProjectEnded ---");
  try {
    const tx6 = await joincare.endProject(projectIdA);
    await tx6.wait();
    console.log("✅ ProjectEnded 触发成功, tx:", tx6.hash);
  } catch (e) {
    console.log("❌ ProjectEnded 失败:", e.message?.substring(0, 200));
  }

  // ============ 7. RaisedFundsWithdrawn (项目成功时) ============
  console.log("\n--- 7. 触发 RaisedFundsWithdrawn ---");
  try {
    const tx7 = await joincare.withdrawRaisedFunds(projectIdA);
    await tx7.wait();
    console.log("✅ RaisedFundsWithdrawn 触发成功, tx:", tx7.hash);
  } catch (e) {
    console.log("❌ RaisedFundsWithdrawn 失败:", e.message?.substring(0, 200));
  }

  // ============ 8. Claimed (等释放时间到) ============
  console.log("\n--- 8. 等待释放时间... ---");
  while (Math.floor(Date.now() / 1000) < vestingTime1 + 2) {
    await new Promise(r => setTimeout(r, 2000));
    process.stdout.write(".");
  }
  console.log("\n--- 8. 触发 Claimed ---");
  try {
    const tx8 = await joincare.claim(projectIdA);
    await tx8.wait();
    console.log("✅ Claimed 触发成功, tx:", tx8.hash);
  } catch (e) {
    console.log("❌ Claimed 失败:", e.message?.substring(0, 200));
  }

  // ============ 9. ProjectTokensWithdrawn ============
  console.log("\n--- 9. 触发 ProjectTokensWithdrawn ---");
  try {
    const tx9 = await joincare.withdrawProjectTokens(projectIdA);
    await tx9.wait();
    console.log("✅ ProjectTokensWithdrawn 触发成功, tx:", tx9.hash);
  } catch (e) {
    console.log("❌ ProjectTokensWithdrawn 失败:", e.message?.substring(0, 200));
  }

  // ============ 10. 创建项目B（失败流程 → Refunded） ============
  console.log("\n--- 10. 创建项目 101（失败流程，触发 Refunded） ---");
  const projectIdB = 101;
  const now2 = Math.floor(Date.now() / 1000);
  const startTimeB = now2 + 5;
  const endTimeB = now2 + 60;
  const vestingTimeB = endTimeB + 10;

  try {
    // 创建项目B（软顶设得很高，认购一点点就结束 → 失败）
    const txB1 = await joincare.createProject(
      projectIdB,
      JCC_ADDRESS,
      owner.address,
      tokenPrice,
      ethers.parseEther("1000"), // hardCap 1000
      ethers.parseEther("500"),  // softCap 500（设很高，不会达到）
      ethers.parseEther("1000"),
      startTimeB,
      endTimeB,
      [vestingTimeB],
      [10000],
      0
    );
    await txB1.wait();
    console.log("  项目 101 创建成功");

    // approve + 充值代币
    const txApproveB = await jcc.approve(JOINCARE_PROXY, ethers.parseEther("2000"));
    await txApproveB.wait();
    const txDepB = await joincare.depositProjectTokens(projectIdB, ethers.parseEther("1000"));
    await txDepB.wait();
    console.log("  项目 101 代币充值完成");

    // 激活
    const txActB = await joincare.activateProject(projectIdB);
    await txActB.wait();

    // 等待开始
    while (Math.floor(Date.now() / 1000) < startTimeB + 2) {
      await new Promise(r => setTimeout(r, 1000));
    }

    // 认购少量（不会达到softCap）
    const txPurB = await joincare.purchase(projectIdB, ethers.parseEther("10"));
    await txPurB.wait();
    console.log("  项目 101 认购了 10 JCC（未达到软顶500）");

    // owner结束项目 → 失败
    const txEndB = await joincare.endProject(projectIdB);
    await txEndB.wait();
    console.log("  项目 101 已结束（状态：Failed）");

    // 退款
    const txRefund = await joincare.refund(projectIdB);
    await txRefund.wait();
    console.log("✅ Refunded 触发成功, tx:", txRefund.hash);
  } catch (e) {
    console.log("❌ 项目B流程失败:", e.message?.substring(0, 200));
  }

  console.log("\n========== 全部事件触发完成 ==========");
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });

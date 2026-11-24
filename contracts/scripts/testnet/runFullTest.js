const hre = require("hardhat");
const fs = require("fs");
const path = require("path");
const { createTestRecorder, expectRevert, preflight, validateE2EEnvironment, waitUntil } = require("./utils/testHelper");

/**
 * JoinCare IDO 平台 - 全面测试脚本
 *
 * 测试数量：约 40 个
 *
 * Phase 0:  预检 - 余额补充
 * Phase 1:  参数验证 & 权限控制（无需等待时间的 revert 测试）
 * Phase 2:  创建"成功项目"和"失败项目"
 * Phase 2.5:在项目开始时间前测试权限控制
 * Phase 3:  认购阶段（等待 startTime 后）
 * Phase 4:  失败项目结束 → 退款测试
 * Phase 5:  成功项目结束 → Claim 测试
 * Phase 6:  资金提取测试（含双花防护）
 *
 * 时间设计（从脚本启动计算）：
 *   T+60s   : 认购开始（两个项目同时开始）
 *   T+240s  : 失败项目结束（总募集 < softcap）
 *   T+360s  : 成功项目结束（总募集 >= softcap）
 *   T+420s  : Vesting[0]（+60s after p1 end）⟶ 50%
 *   T+600s  : Vesting[1]（+240s after p1 end）⟶ 30%
 *   T+780s  : Vesting[2]（+420s after p1 end）⟶ 20%
 *
 * 总耗时：约 15-18 分钟
 */

async function main() {
    console.log("\n" + "=".repeat(100));
    console.log("=== JoinCare IDO 平台 - 全面测试（约 40 个测试用例）===");
    console.log("=".repeat(100) + "\n");

    const runStartMs = Date.now();

    // ── 加载配置 ──────────────────────────────────────────────────────────────
    const configDir = path.join(__dirname, "config");
    const accountsConfig = JSON.parse(fs.readFileSync(path.join(configDir, "accounts.json"), "utf8"));
    const contractsConfig = JSON.parse(fs.readFileSync(path.join(configDir, "contracts.json"), "utf8"));
    const testParams = JSON.parse(fs.readFileSync(path.join(configDir, "testParams.json"), "utf8"));
	await validateE2EEnvironment(contractsConfig);

    // ── 账户 ──────────────────────────────────────────────────────────────────
    const [deployer] = await hre.ethers.getSigners();
    const projectOwner = new hre.ethers.Wallet(accountsConfig.accounts.projectOwner.privateKey, hre.ethers.provider);
    const users = [
        new hre.ethers.Wallet(accountsConfig.accounts.user1.privateKey, hre.ethers.provider),
        new hre.ethers.Wallet(accountsConfig.accounts.user2.privateKey, hre.ethers.provider),
        new hre.ethers.Wallet(accountsConfig.accounts.user3.privateKey, hre.ethers.provider),
        new hre.ethers.Wallet(accountsConfig.accounts.user4.privateKey, hre.ethers.provider),
        new hre.ethers.Wallet(accountsConfig.accounts.user5.privateKey, hre.ethers.provider),
    ];

    // ── 合约实例 ───────────────────────────────────────────────────────────────
    const jccToken = await hre.ethers.getContractAt("JCC", contractsConfig.contracts.jccToken);
    const platform = await hre.ethers.getContractAt("JoinCare_v1", contractsConfig.contracts.joinCarePlatform);
    const projectToken = await hre.ethers.getContractAt("JCC", contractsConfig.contracts.projectToken);

    // ── 参数常量（从 testParams.json 读取）────────────────────────────────────
    const tokenPrice = hre.ethers.parseEther(testParams.projectParams.tokenPrice);
    const hardCap = hre.ethers.parseEther(testParams.projectParams.hardCap);
    const softCap = hre.ethers.parseEther(testParams.projectParams.softCap);
    const maxPerUser = hre.ethers.parseEther(testParams.projectParams.maxPerUser);
    const depositRequired = (hardCap * hre.ethers.parseEther("1")) / tokenPrice; // = hardCap（1:1价格）

    // ── 测试记录 ───────────────────────────────────────────────────────────────
    const { testResults, logTest } = createTestRecorder();
    const STATUS = ["Pending", "Active", "Ended", "Successful", "Failed", "Cancelled"];

    // ============================================================================
    //  Phase 0: 预检
    // ============================================================================
    await preflight({
        deployer, jccToken, projectToken, projectOwner, users,
        requiredJCC: maxPerUser,               // 每个 user 至少有 maxPerUser JCC
        requiredPT: depositRequired * 2n,     // 两个项目各需一份
    });
    console.log("=".repeat(100) + "\n");

    try {
        // ============================================================================
        //  Phase 1: 参数验证 & 权限控制（无需等待）
        // ============================================================================
        console.log("🔐 Phase 1: 参数验证 & 权限控制测试\n");

        const now = Math.floor(Date.now() / 1000);
        const fStart = now + 300;
        const fEnd = now + 600;
        const fVT = [fEnd + 60, fEnd + 180];
        const fVP = [5000, 5000];
        const dId = BigInt(Date.now()) * 10000n; // 不会真正使用的 dummy ID

        // helper： 快速创建项目调用（用于参数验证）
        const badCreate = (overrides) => platform.connect(deployer).createProject(
            dId,
            projectToken.target,
            projectOwner.address,
            tokenPrice, hardCap, softCap, maxPerUser,
            fStart, fEnd, fVT, fVP, 0,
            ...Object.values(overrides)
        );

        // 1. 过期 startTime
        let r = await expectRevert(
            () => platform.connect(deployer).createProject(
                dId, projectToken.target, projectOwner.address,
                tokenPrice, hardCap, softCap, maxPerUser,
                now - 3600, fEnd, fVT, fVP, 0
            ), "Start time must be in future");
        logTest("参数验证", "拒绝过期的 startTime", r.reverted, r.error ? { error: r.error } : {});

        // 2. endTime ≤ startTime
        r = await expectRevert(
            () => platform.connect(deployer).createProject(
                dId, projectToken.target, projectOwner.address,
                tokenPrice, hardCap, softCap, maxPerUser,
                fStart, fStart - 1, fVT, fVP, 0
            ), "End time must be after start time");
        logTest("参数验证", "拒绝 endTime ≤ startTime", r.reverted, r.error ? { error: r.error } : {});

        // 3. 空 vesting 数组
        r = await expectRevert(
            () => platform.connect(deployer).createProject(
                dId, projectToken.target, projectOwner.address,
                tokenPrice, hardCap, softCap, maxPerUser,
                fStart, fEnd, [], [], 0
            ), "Empty vesting schedule");
        logTest("参数验证", "拒绝空的 vesting 数组", r.reverted, r.error ? { error: r.error } : {});

        // 4. vesting 数组长度不匹配
        r = await expectRevert(
            () => platform.connect(deployer).createProject(
                dId, projectToken.target, projectOwner.address,
                tokenPrice, hardCap, softCap, maxPerUser,
                fStart, fEnd, [fEnd + 60, fEnd + 120], [10000], 0
            ), "Vesting arrays length mismatch");
        logTest("参数验证", "拒绝 vesting 数组长度不一致", r.reverted, r.error ? { error: r.error } : {});

        // 5. vesting 总比例 ≠ 100%
        r = await expectRevert(
            () => platform.connect(deployer).createProject(
                dId, projectToken.target, projectOwner.address,
                tokenPrice, hardCap, softCap, maxPerUser,
                fStart, fEnd, [fEnd + 60, fEnd + 120], [3000, 6000], 0
            ), "Total vesting percent must be 100%");
        logTest("参数验证", "拒绝 vesting 总比例不等于100%", r.reverted, r.error ? { error: r.error } : {});

        // 6. vesting 时间不递增
        r = await expectRevert(
            () => platform.connect(deployer).createProject(
                dId, projectToken.target, projectOwner.address,
                tokenPrice, hardCap, softCap, maxPerUser,
                fStart, fEnd, [fEnd + 180, fEnd + 60], [5000, 5000], 0
            ), "Vesting times must be ascending");
        logTest("参数验证", "拒绝 vesting 时间不递增", r.reverted, r.error ? { error: r.error } : {});

        // 7. 第一个 vesting 时间早于 endTime
        r = await expectRevert(
            () => platform.connect(deployer).createProject(
                dId, projectToken.target, projectOwner.address,
                tokenPrice, hardCap, softCap, maxPerUser,
                fStart, fEnd, [fEnd - 1, fEnd + 120], [5000, 5000], 0
            ), "First vesting time must be after endTime");
        logTest("参数验证", "拒绝第一个 vesting 时间早于 endTime", r.reverted, r.error ? { error: r.error } : {});

        // 8. softCap > hardCap
        r = await expectRevert(
            () => platform.connect(deployer).createProject(
                dId, projectToken.target, projectOwner.address,
                tokenPrice, softCap, hardCap, maxPerUser, // 软顶和硬顶对调
                fStart, fEnd, fVT, fVP, 0
            ), "Soft cap must <= hard cap");
        logTest("参数验证", "拒绝 softCap > hardCap", r.reverted, r.error ? { error: r.error } : {});

        // 9. 非 owner 创建项目
        r = await expectRevert(
            () => platform.connect(users[0]).createProject(
                dId, projectToken.target, projectOwner.address,
                tokenPrice, hardCap, softCap, maxPerUser,
                fStart, fEnd, fVT, fVP, 0
            ), "OwnableUnauthorizedAccount");
        logTest("权限控制", "非 owner 不能创建项目", r.reverted, r.error ? { error: r.error } : {});

        console.log("");

        // ============================================================================
        //  Phase 2: 创建两个测试项目（成功项目 + 失败项目）
        // ============================================================================
        console.log("🚀 Phase 2: 创建测试项目\n");

        // ── 设计说明 ─────────────────────────────────────────────────────────────────
        // Phase 1 + Phase 2 的充值/激活/权限验证操作总计约需 8-9 分钟。
        // 为确保两个项目的 startTime/endTime 基于相同的时基并留有足够余量，
        // 策略如下：
        //   1. 先用大 delay（startDelay）创建两个项目（startTime 在未来足够远）
        //   2. 完成所有充值/激活/权限测试（消耗约 7 分钟）
        //   3. 等到 waitUntil(p1Start) 后再开始认购（Phase 3）
        // startDelay 配置于 testParams.json，runFullTest 建议使用 600s（10分钟）。
        // p2（失败项目）与 p1 共用相同的 cur + startDelay 作为 startTime，
        // 确保两个项目并行运行。
        // ─────────────────────────────────────────────────────────────────────────────

        const cur = Math.floor(Date.now() / 1000);
        const startDelay = testParams.timeConfig.projectStartDelay; // 来自 testParams.json，应 ≥ 600s

        // 成功项目（p1）时间参数
        const p1Start = cur + startDelay;
        const p1End = p1Start + testParams.timeConfig.projectDuration;
        const p1VTimes = testParams.timeConfig.vestingDelays.map(d => p1End + d);
        const p1VPct = testParams.timeConfig.vestingPercents;
        const p1VDur = testParams.timeConfig.vestingDelays[testParams.timeConfig.vestingDelays.length - 1];

        // 失败项目（p2）时间参数 —— 与 p1 相同 startTime，持续仅 180s（比 p1 先结束）
        const p2Start = cur + startDelay;   // 与 p1 同步开始，共用同一时基
        const p2End = p2Start + 180;       // 仅 3 分钟，确保先于 p1End 结束
        const p2VTimes = [p2End + 60, p2End + 120];
        const p2VPct = [5000, 5000];

        let p1Id, p2Id;

        // 创建成功项目
        try {
            p1Id = BigInt(Date.now()) * 1000000n + BigInt(Math.floor(Math.random() * 500000));
            const tx = await platform.connect(deployer).createProject(
                p1Id, projectToken.target, projectOwner.address,
                tokenPrice, hardCap, softCap, maxPerUser,
                p1Start, p1End, p1VTimes, p1VPct, p1VDur
            );
            await tx.wait();
            console.log(`  成功项目 ID: ${p1Id}`);
            logTest("项目创建", "创建成功项目", true, { projectId: p1Id.toString() });
        } catch (e) {
            logTest("项目创建", "创建成功项目", false, { error: e.message });
            throw e;
        }

        // 创建失败项目
        try {
            p2Id = BigInt(Date.now()) * 1000000n + BigInt(Math.floor(Math.random() * 500000)) + 999999n;
            const tx = await platform.connect(deployer).createProject(
                p2Id, projectToken.target, projectOwner.address,
                tokenPrice, hardCap, softCap, maxPerUser,
                p2Start, p2End, p2VTimes, p2VPct, 0
            );
            await tx.wait();
            console.log(`  失败项目 ID: ${p2Id}`);
            logTest("项目创建", "创建失败项目", true, { projectId: p2Id.toString() });
        } catch (e) {
            logTest("项目创建", "创建失败项目", false, { error: e.message });
            throw e;
        }

        // 10. 重复 projectId 被拒绝
        r = await expectRevert(
            () => platform.connect(deployer).createProject(
                p1Id, projectToken.target, projectOwner.address,
                tokenPrice, hardCap, softCap, maxPerUser,
                fStart, fEnd, fVT, fVP, 0
            ), "Project ID already exists");
        logTest("参数验证", "拒绝重复的 projectId", r.reverted, r.error ? { error: r.error } : {});

        // 11. 非 projectOwner 充值
        r = await expectRevert(
            () => platform.connect(users[0]).depositProjectTokens(p1Id, hre.ethers.parseEther("1")),
            "Only project owner can deposit");
        logTest("权限控制", "非 projectOwner 不能充值", r.reverted, r.error ? { error: r.error } : {});

        // 12. 未充值时激活
        r = await expectRevert(
            () => platform.connect(deployer).activateProject(p1Id),
            "Project tokens not deposited");
        logTest("参数验证", "未充值不能激活项目", r.reverted, r.error ? { error: r.error } : {});

        // 充值成功项目
        try {
            let tx = await projectToken.connect(projectOwner).approve(platform.target, depositRequired);
            await tx.wait();
            tx = await platform.connect(projectOwner).depositProjectTokens(p1Id, depositRequired);
            await tx.wait();
            const p = await platform.projects(p1Id);
            logTest("代币充值", "成功项目充值金额正确", p.totalTokensDeposited === depositRequired, {
                deposited: hre.ethers.formatEther(p.totalTokensDeposited)
            });
        } catch (e) {
            logTest("代币充值", "成功项目充值金额正确", false, { error: e.message });
        }

        // 14. 非 owner 激活
        r = await expectRevert(
            () => platform.connect(users[0]).activateProject(p1Id),
            "OwnableUnauthorizedAccount");
        logTest("权限控制", "非 owner 不能激活项目", r.reverted, r.error ? { error: r.error } : {});

        // 充值失败项目
        try {
            let tx = await projectToken.connect(projectOwner).approve(platform.target, depositRequired);
            await tx.wait();
            tx = await platform.connect(projectOwner).depositProjectTokens(p2Id, depositRequired);
            await tx.wait();
            logTest("代币充值", "失败项目充值成功", true, {});
        } catch (e) {
            logTest("代币充值", "失败项目充值成功", false, { error: e.message });
        }

        // 激活成功项目
        try {
            const tx = await platform.connect(deployer).activateProject(p1Id);
            await tx.wait();
            const p = await platform.projects(p1Id);
            logTest("项目激活", "手动激活成功项目", STATUS[p.status] === "Active", { status: STATUS[p.status] });
        } catch (e) {
            logTest("项目激活", "手动激活成功项目", false, { error: e.message });
        }

        // 15. Active 状态下再次充值被拒绝
        r = await expectRevert(
            () => platform.connect(projectOwner).depositProjectTokens(p1Id, hre.ethers.parseEther("1")),
            "Project not in Pending status");
        logTest("参数验证", "Active 状态不能再次充值", r.reverted, r.error ? { error: r.error } : {});

        // 激活失败项目
        try {
            const tx = await platform.connect(deployer).activateProject(p2Id);
            await tx.wait();
            const p = await platform.projects(p2Id);
            logTest("项目激活", "手动激活失败项目", STATUS[p.status] === "Active", { status: STATUS[p.status] });
        } catch (e) {
            logTest("项目激活", "手动激活失败项目", false, { error: e.message });
        }

        // 16. 非 owner 在 endTime 前结束项目（此时距 p1End 仍有大量余量）
        r = await expectRevert(
            () => platform.connect(users[0]).endProject(p1Id),
            "Project not ended yet");
        logTest("权限控制", "非 owner 不能在 endTime 前结束项目", r.reverted, r.error ? { error: r.error } : {});

        console.log("");

        // ============================================================================
        //  Phase 3: 等待 startTime → 认购阶段
        // ============================================================================
        await waitUntil(p1Start, "等待项目开始时间");
        console.log("🛒 Phase 3: 用户认购\n");

        // 成功项目：5 个用户各购 maxPerUser → 达到软顶
        let p1Purchased = 0n;
        for (let i = 0; i < users.length; i++) {
            try {
                let tx = await jccToken.connect(users[i]).approve(platform.target, maxPerUser);
                await tx.wait();
                tx = await platform.connect(users[i]).purchase(p1Id, maxPerUser);
                await tx.wait();
                p1Purchased += maxPerUser;
                console.log(`  ✅ [成功项目] user${i + 1} 认购 ${hre.ethers.formatEther(maxPerUser)} JCC`);
            } catch (e) {
                console.log(`  ❌ [成功项目] user${i + 1} 认购失败: ${e.message.substring(0, 100)}`);
            }
        }
        logTest("用户认购", "[成功项目] 5用户达到软顶", p1Purchased >= softCap, {
            purchased: hre.ethers.formatEther(p1Purchased),
            softCap: hre.ethers.formatEther(softCap)
        });

        // 17. 超过单用户上限
        r = await expectRevert(
            () => platform.connect(users[0]).purchase(p1Id, hre.ethers.parseEther("1")), // user1 已到上限
            "Exceeds max per user");
        logTest("参数验证", "拒绝超过单用户认购上限", r.reverted, r.error ? { error: r.error } : {});

        // 失败项目：只有 user1 购买（远低于软顶 5000）
        try {
            let tx = await jccToken.connect(users[0]).approve(platform.target, maxPerUser);
            await tx.wait();
            tx = await platform.connect(users[0]).purchase(p2Id, maxPerUser);
            await tx.wait();
            logTest("用户认购", "[失败项目] 少量认购（将低于软顶）", true, {
                purchased: hre.ethers.formatEther(maxPerUser),
                softCap: hre.ethers.formatEther(softCap)
            });
        } catch (e) {
            logTest("用户认购", "[失败项目] 少量认购", false, { error: e.message });
        }

        // 18. 非失败项目不能退款
        r = await expectRevert(
            () => platform.connect(users[0]).refund(p1Id), // 成功项目进行中
            "Project not failed");
        logTest("参数验证", "Non-failed 项目不能退款", r.reverted, r.error ? { error: r.error } : {});

        console.log("");

        // ============================================================================
        //  Phase 4: 等待失败项目结束 → 退款测试
        // ============================================================================
        await waitUntil(p2End, "等待失败项目结束时间");
        console.log("💸 Phase 4: 失败项目退款测试\n");

        // 结束失败项目
        try {
            const tx = await platform.connect(deployer).endProject(p2Id);
            await tx.wait();
            const p = await platform.projects(p2Id);
            logTest("项目结束", "[失败项目] 状态为 Failed", STATUS[p.status] === "Failed", { status: STATUS[p.status] });
        } catch (e) {
            logTest("项目结束", "[失败项目] 状态为 Failed", false, { error: e.message });
        }

        // 19. user1 退款
        const u1BalBefore = await jccToken.balanceOf(users[0].address);
        try {
            const tx = await platform.connect(users[0]).refund(p2Id);
            await tx.wait();
            const u1BalAfter = await jccToken.balanceOf(users[0].address);
            const refunded = u1BalAfter - u1BalBefore;
            logTest("退款", "[失败项目] user1 退款成功", refunded === maxPerUser, {
                refunded: hre.ethers.formatEther(refunded), expected: hre.ethers.formatEther(maxPerUser)
            });
        } catch (e) {
            logTest("退款", "[失败项目] user1 退款成功", false, { error: e.message });
        }

        // 20. 重复退款被拒绝
        r = await expectRevert(
            () => platform.connect(users[0]).refund(p2Id),
            "Already refunded");
        logTest("退款", "拒绝重复退款", r.reverted, r.error ? { error: r.error } : {});

        // 21. 无认购记录退款被拒绝
        r = await expectRevert(
            () => platform.connect(users[1]).refund(p2Id), // user2 没有购买
            "No purchase to refund");
        logTest("退款", "无认购记录不能退款", r.reverted, r.error ? { error: r.error } : {});

        // 22. 失败项目 projectOwner 提取全部代币
        const po2BalBefore = await projectToken.balanceOf(projectOwner.address);
        try {
            const tx = await platform.connect(projectOwner).withdrawProjectTokens(p2Id);
            await tx.wait();
            const po2BalAfter = await projectToken.balanceOf(projectOwner.address);
            const withdrawn = po2BalAfter - po2BalBefore;
            const p2data = await platform.projects(p2Id);
            logTest("代币提取", "[失败项目] 提取全部项目代币", withdrawn === p2data.totalTokensDeposited, {
                withdrawn: hre.ethers.formatEther(withdrawn)
            });
        } catch (e) {
            logTest("代币提取", "[失败项目] 提取全部项目代币", false, { error: e.message });
        }

        // 25. 失败项目 无法提取募集资金（项目未成功）
        r = await expectRevert(
            () => platform.connect(projectOwner).withdrawRaisedFunds(p2Id),
            "Project not successful");
        logTest("参数验证", "[失败项目] 不能提取募集资金", r.reverted, r.error ? { error: r.error } : {});

        // 26. 重复提取代币被拒绝
        r = await expectRevert(
            () => platform.connect(projectOwner).withdrawProjectTokens(p2Id),
            "Tokens already withdrawn");
        logTest("代币提取", "拒绝重复提取失败项目代币", r.reverted, r.error ? { error: r.error } : {});

        console.log("");

        // ============================================================================
        //  Phase 5: 等待成功项目结束 → Claim 测试
        // ============================================================================
        await waitUntil(p1End, "等待成功项目结束时间");
        console.log("🎁 Phase 5: 代币领取测试\n");

        // 结束成功项目
        try {
            const tx = await platform.connect(deployer).endProject(p1Id);
            await tx.wait();
            const p = await platform.projects(p1Id);
            logTest("项目结束", "[成功项目] 状态为 Successful", STATUS[p.status] === "Successful", { status: STATUS[p.status] });
        } catch (e) {
            logTest("项目结束", "[成功项目] 状态为 Successful", false, { error: e.message });
        }

        // 27. Vesting 开始前不能 Claim
        r = await expectRevert(
            () => platform.connect(users[0]).claim(p1Id),
            "First vesting not started yet");
        logTest("代币领取", "Vesting 开始前不能 Claim", r.reverted, r.error ? { error: r.error } : {});

        // 获取 user1 购买量
        const u1Purchase = await platform.getUserPurchase(p1Id, users[0].address);
        const totalBought = u1Purchase.totalTokensBought;

        // 28. 第一次 Vesting（50%）
        await waitUntil(p1VTimes[0], "等待第一次 Vesting 时间");
        {
            const before = await projectToken.balanceOf(users[0].address);
            try {
                const tx = await platform.connect(users[0]).claim(p1Id);
                await tx.wait();
                const claimed = (await projectToken.balanceOf(users[0].address)) - before;
                const expected = totalBought * BigInt(p1VPct[0]) / 10000n;
                logTest("代币领取", `第一次 Vesting (${p1VPct[0] / 100}%)`, claimed === expected, {
                    claimed: hre.ethers.formatEther(claimed), expected: hre.ethers.formatEther(expected)
                });
            } catch (e) {
                logTest("代币领取", `第一次 Vesting (${p1VPct[0] / 100}%)`, false, { error: e.message });
            }
        }

        // 29. 重复 Claim 被拒绝（无新释放）
        r = await expectRevert(
            () => platform.connect(users[0]).claim(p1Id),
            "No tokens to claim");
        logTest("代币领取", "拒绝重复 Claim（无新释放）", r.reverted, r.error ? { error: r.error } : {});

        // 30. 无认购不能 Claim （未参与的账户）
        // 注意：这里用 projectOwner 来测，其没有购买过
        r = await expectRevert(
            () => platform.connect(projectOwner).claim(p1Id),
            "No tokens to claim");
        logTest("参数验证", "无认购记录不能 Claim", r.reverted, r.error ? { error: r.error } : {});

        // 31. 第二次 Vesting（30%）
        await waitUntil(p1VTimes[1], "等待第二次 Vesting 时间");
        {
            const before = await projectToken.balanceOf(users[0].address);
            try {
                const tx = await platform.connect(users[0]).claim(p1Id);
                await tx.wait();
                const claimed = (await projectToken.balanceOf(users[0].address)) - before;
                const expected = totalBought * BigInt(p1VPct[1]) / 10000n;
                logTest("代币领取", `第二次 Vesting (${p1VPct[1] / 100}%)`, claimed === expected, {
                    claimed: hre.ethers.formatEther(claimed), expected: hre.ethers.formatEther(expected)
                });
            } catch (e) {
                logTest("代币领取", `第二次 Vesting (${p1VPct[1] / 100}%)`, false, { error: e.message });
            }
        }

        // 32. 第三次 Vesting（20%）
        await waitUntil(p1VTimes[2], "等待第三次 Vesting 时间");
        {
            const before = await projectToken.balanceOf(users[0].address);
            try {
                const tx = await platform.connect(users[0]).claim(p1Id);
                await tx.wait();
                const claimed = (await projectToken.balanceOf(users[0].address)) - before;
                const expected = totalBought * BigInt(p1VPct[2]) / 10000n;
                logTest("代币领取", `第三次 Vesting (${p1VPct[2] / 100}%)`, claimed === expected, {
                    claimed: hre.ethers.formatEther(claimed), expected: hre.ethers.formatEther(expected)
                });
            } catch (e) {
                logTest("代币领取", `第三次 Vesting (${p1VPct[2] / 100}%)`, false, { error: e.message });
            }
        }

        console.log("");

        // ============================================================================
        //  Phase 6: 资金提取测试（含双花防护）
        // ============================================================================
        console.log("💰 Phase 6: 资金提取测试\n");

        // 33. 非 projectOwner 不能提取募集资金
        r = await expectRevert(
            () => platform.connect(users[1]).withdrawRaisedFunds(p1Id),
            "Only project owner can withdraw");
        logTest("权限控制", "非 projectOwner 不能提取募集资金", r.reverted, r.error ? { error: r.error } : {});

        // 34. projectOwner 提取募集资金（扣2%手续费）
        const p1data = await platform.projects(p1Id);
        const platformFeeData = await platform.platformFees();
        const fee = p1data.totalRaised * platformFeeData.successFeeRate / 10000n;
        const netAmount = p1data.totalRaised - fee;
        const poBefore = await jccToken.balanceOf(projectOwner.address);
        try {
            const tx = await platform.connect(projectOwner).withdrawRaisedFunds(p1Id);
            await tx.wait();
            const poAfter = await jccToken.balanceOf(projectOwner.address);
            const received = poAfter - poBefore;
            logTest("资金提取", "提取募集资金（含手续费）", received === netAmount, {
                received: hre.ethers.formatEther(received),
                expected: hre.ethers.formatEther(netAmount),
                fee: hre.ethers.formatEther(fee)
            });
        } catch (e) {
            logTest("资金提取", "提取募集资金（含手续费）", false, { error: e.message });
        }

        // 35. 重复提取募集资金被拒绝
        r = await expectRevert(
            () => platform.connect(projectOwner).withdrawRaisedFunds(p1Id),
            "Funds already withdrawn");
        logTest("资金提取", "拒绝重复提取募集资金", r.reverted, r.error ? { error: r.error } : {});

        // 36. 非 projectOwner 不能提取项目代币
        r = await expectRevert(
            () => platform.connect(users[1]).withdrawProjectTokens(p1Id),
            "Only project owner can withdraw");
        logTest("权限控制", "非 projectOwner 不能提取项目代币", r.reverted, r.error ? { error: r.error } : {});

        // 37. projectOwner 提取未售出代币
        const p1Refresh = await platform.projects(p1Id);
        const unsold = p1Refresh.totalTokensDeposited - p1Refresh.totalTokensSold;
        const ptBefore = await projectToken.balanceOf(projectOwner.address);
        try {
            const tx = await platform.connect(projectOwner).withdrawProjectTokens(p1Id);
            await tx.wait();
            const ptAfter = await projectToken.balanceOf(projectOwner.address);
            const received = ptAfter - ptBefore;
            logTest("代币提取", "[成功项目] 提取未售出代币", received === unsold, {
                received: hre.ethers.formatEther(received),
                expected: hre.ethers.formatEther(unsold)
            });
        } catch (e) {
            logTest("代币提取", "[成功项目] 提取未售出代币", false, { error: e.message });
        }

        // 38. 重复提取项目代币被拒绝
        r = await expectRevert(
            () => platform.connect(projectOwner).withdrawProjectTokens(p1Id),
            "Tokens already withdrawn");
        logTest("代币提取", "拒绝重复提取项目代币", r.reverted, r.error ? { error: r.error } : {});

        console.log("");

    } catch (err) {
        console.error("\n❌ 测试过程中发生致命错误:", err.message);
    }

    // ============================================================================
    //  测试总结
    // ============================================================================
    const elapsed = Math.floor((Date.now() - runStartMs) / 1000);
    console.log("=".repeat(100));
    console.log("📊 测试总结");
    console.log("=".repeat(100) + "\n");
    console.log(`总测试数: ${testResults.summary.total}`);
    console.log(`通过:     ${testResults.summary.passed} ✅`);
    console.log(`失败:     ${testResults.summary.failed} ❌`);
    console.log(`通过率:   ${((testResults.summary.passed / testResults.summary.total) * 100).toFixed(1)}%`);
    console.log(`总耗时:   ${Math.floor(elapsed / 60)} 分 ${elapsed % 60} 秒`);

    // 保存报告
    testResults.endTime = new Date().toISOString();
    testResults.duration = elapsed;
    const reportPath = path.join(__dirname, "reports", "fullTestReport_advanced.json");
    fs.writeFileSync(reportPath, JSON.stringify(testResults, null, 2));
    console.log(`\n报告已保存: ${reportPath}`);
    console.log("\n" + "=".repeat(100) + "\n");
}

main()
    .then(() => process.exit(0))
    .catch(e => { console.error(e); process.exit(1); });

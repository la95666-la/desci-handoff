/**
 * testHelper.js
 * 测试公共工具：日志记录、预期 revert 检测、余额预检与补充
 */

const hre = require("hardhat");

async function validateE2EEnvironment(contractsConfig) {
    const network = await hre.ethers.provider.getNetwork();
    const expectedChainId = BigInt(contractsConfig.chainId);
    if (network.chainId !== expectedChainId) {
        throw new Error(`测试链不匹配: RPC=${network.chainId} contracts.json=${expectedChainId}`);
    }

    const addresses = contractsConfig.contracts || {};
    for (const name of ["jccToken", "joinCarePlatform", "projectToken"]) {
        const address = addresses[name];
        if (!hre.ethers.isAddress(address) || address === hre.ethers.ZeroAddress) {
            throw new Error(`contracts.json ${name} 不是有效非零地址`);
        }
        if ((await hre.ethers.provider.getCode(address)) === "0x") {
            throw new Error(`contracts.json ${name} 地址没有合约代码: ${address}`);
        }
    }

    const platform = await hre.ethers.getContractAt("JoinCare_v1", addresses.joinCarePlatform);
    const [owner, jccToken, platformFees] = await Promise.all([
        platform.owner(),
        platform.jccToken(),
        platform.platformFees(),
    ]);
    if (owner === hre.ethers.ZeroAddress || platformFees.feeReceiver === hre.ethers.ZeroAddress) {
        throw new Error("IDO 平台 getter 返回零地址");
    }
    if (jccToken.toLowerCase() !== addresses.jccToken.toLowerCase()) {
        throw new Error(`JCC 地址不一致: platform=${jccToken} contracts.json=${addresses.jccToken}`);
    }
    return { network, owner, jccToken, platformFees };
}

/**
 * 创建测试结果记录器
 */
function createTestRecorder() {
    const testResults = {
        startTime: new Date().toISOString(),
        tests: [],
        summary: { total: 0, passed: 0, failed: 0 }
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
            if (details.error) console.log(`     错误: ${details.error.substring(0, 200)}`);
        }
    }

    return { testResults, logTest };
}

/**
 * 测试预期会 revert 的调用
 * @param {Function} fn - 要调用的异步函数
 * @param {string} expectedMsg - 期望包含的 revert 原因字符串（支持普通字符串和 OZ v5 Custom Error 名称）
 * @returns {{ reverted: boolean, error: string|null }}
 *
 * 兼容两种 revert 格式：
 * 1. 传统字符串 reason："Project not failed" 等
 * 2. OZ v5 Custom Error：OwnableUnauthorizedAccount、EnforcedPause 等
 *    链上返回 ABI 编码的 hex，需要从 err.data 或 err.message 中提取识别
 */
async function expectRevert(fn, expectedMsg) {
    try {
        await fn();
        return { reverted: false, error: `Expected revert with "${expectedMsg}" but tx succeeded` };
    } catch (err) {
        const msg = (err.message || "") + (err.reason || "");

        // 1. 常规字符串匹配（适用于 require 字符串 reason）
        if (msg.includes(expectedMsg)) {
            return { reverted: true, error: null };
        }

        // 2. OZ v5 Custom Error 匹配：从错误的 data 字段提取 Custom Error 名称
        //    OZ v5 错误示例: OwnableUnauthorizedAccount(address)  selector = 0x118cdaa7
        //    EnforcedPause()                                       selector = 0xd93c0665
        const customErrorName = extractCustomErrorName(err);
        if (customErrorName && customErrorName.includes(expectedMsg)) {
            return { reverted: true, error: null };
        }

        return {
            reverted: false,
            error: `Expected "${expectedMsg}" but got: ${msg.substring(0, 300)}`
        };
    }
}

/**
 * 从 ethers.js 错误对象中尝试提取 Custom Error 名称
 * 支持通过 err.data（4字节 selector hex）或 err.message 中的 hex 字串来识别
 */
function extractCustomErrorName(err) {
    // OZ v5 常见 Custom Error 的 selector 映射表
    const knownSelectors = {
        "0x118cdaa7": "OwnableUnauthorizedAccount",
        "0x179b8571": "OwnableInvalidOwner",
        "0xd93c0665": "EnforcedPause",
        "0x8dfc202b": "ExpectedPause",
        "0x3ee5aeb5": "ReentrancyGuardReentrantCall",
    };

    // 尝试从 err.data 中获取 selector
    let selector = null;
    if (err.data && typeof err.data === "string" && err.data.startsWith("0x")) {
        selector = err.data.substring(0, 10).toLowerCase();
    }

    // 也尝试从 err.message 中提取 0x 开头的 hex（BSC testnet 有时嵌在消息里）
    if (!selector) {
        const hexMatch = (err.message || "").match(/execution reverted:\s*(0x[0-9a-fA-F]{8,})/i);
        if (hexMatch) {
            selector = hexMatch[1].substring(0, 10).toLowerCase();
        }
    }

    if (selector && knownSelectors[selector]) {
        return knownSelectors[selector];
    }

    return null;
}

/**
 * 执行余额预检，不足时由 deployer 调用 mint() 补充
 * @param {object} params
 */
async function preflight({ deployer, jccToken, projectToken, projectOwner, users, requiredJCC, requiredPT }) {
    console.log("🔍 预检：检查并补充账户余额\n");

    for (let i = 0; i < users.length; i++) {
        const balance = await jccToken.balanceOf(users[i].address);
        const label = `user${i + 1} JCC: ${hre.ethers.formatEther(balance)} (需≥${hre.ethers.formatEther(requiredJCC)})`;
        if (balance < requiredJCC) {
            const mintTx = await jccToken.connect(deployer).mint(users[i].address, requiredJCC - balance);
            await mintTx.wait();
            console.log(`  ${label}  ➜ 已补充`);
        } else {
            console.log(`  ${label}  ✅`);
        }
    }

    const ptBal = await projectToken.balanceOf(projectOwner.address);
    const ptLabel = `projectOwner PT: ${hre.ethers.formatEther(ptBal)} (需≥${hre.ethers.formatEther(requiredPT)})`;
    if (ptBal < requiredPT) {
        const mintTx = await projectToken.connect(deployer).mint(projectOwner.address, requiredPT - ptBal);
        await mintTx.wait();
        console.log(`  ${ptLabel}  ➜ 已补充`);
    } else {
        console.log(`  ${ptLabel}  ✅`);
    }

    console.log("\n  ✅ 预检完成\n");
}

/**
 * 等待到指定的 Unix 时间戳
 */
async function waitUntil(targetTimestamp, label) {
    const wait = targetTimestamp - Math.floor(Date.now() / 1000) + 5;
    if (wait > 0) {
        console.log(`  ⏳ ${label}：等待 ${wait} 秒...\n`);
        await new Promise(r => setTimeout(r, wait * 1000));
    }
}

module.exports = { createTestRecorder, expectRevert, preflight, validateE2EEnvironment, waitUntil };

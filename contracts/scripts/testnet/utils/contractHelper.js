/**
 * 合约交互工具类
 * 简化合约调用、交易处理和事件验证
 */

const { ethers } = require("ethers");
const fs = require("fs");
const path = require("path");

class ContractHelper {
    constructor(provider) {
        this.provider = provider;
        this.contracts = {};
        this.abis = {};
    }

    /**
     * 初始化合约实例
     */
    async initContracts(signer) {
        const configPath = path.join(__dirname, "../config/contracts.json");
        const config = JSON.parse(fs.readFileSync(configPath, "utf8"));

        try {
            // 尝试加载 ABI 文件
            const jccArtifact = require("../../../artifacts/contracts/JCC.sol/JCC.json");
            const platformArtifact = require("../../../artifacts/contracts/JoinCare_v1.sol/JoinCare_v1.json");
            const erc20Artifact = require("../../../artifacts/contracts/mocks/MockERC20.sol/MockERC20.json");

            this.abis = {
                jccABI: jccArtifact.abi,
                platformABI: platformArtifact.abi,
                erc20ABI: erc20Artifact.abi
            };

            // 创建合约实例
            this.contracts.jccToken = new ethers.Contract(
                config.contracts.jccToken,
                this.abis.jccABI,
                signer
            );

            this.contracts.platform = new ethers.Contract(
                config.contracts.joinCarePlatform,
                this.abis.platformABI,
                signer
            );

            // 如果项目代币已部署，也初始化
            if (config.contracts.projectToken) {
                this.contracts.projectToken = new ethers.Contract(
                    config.contracts.projectToken,
                    this.abis.erc20ABI,
                    signer
                );
            }
        } catch (error) {
            console.error("Error loading contract ABIs:", error.message);
            throw new Error("Failed to load contract ABIs. Make sure contracts are compiled.");
        }

        return this.contracts;
    }

    /**
     * 等待交易确认
     */
    async waitForTx(tx, confirmations = 2) {
        console.log(`  ⏳ 交易已发送: ${tx.hash}`);
        console.log(`     等待 ${confirmations} 个确认...`);

        const receipt = await tx.wait(confirmations);

        if (receipt.status === 1) {
            console.log(`  ✅ 交易成功确认`);
            console.log(`     Gas 使用: ${receipt.gasUsed.toString()}`);
        } else {
            console.log(`  ❌ 交易失败`);
        }

        return receipt;
    }

    /**
     * 从交易回执中提取事件日志
     */
    getEventLogs(receipt, contract, eventName) {
        const events = [];

        for (const log of receipt.logs) {
            try {
                const parsed = contract.interface.parseLog({
                    topics: log.topics,
                    data: log.data
                });

                if (parsed && parsed.name === eventName) {
                    events.push(parsed);
                }
            } catch (e) {
                // 忽略无法解析的日志
            }
        }

        return events;
    }

    /**
     * 验证事件是否被触发
     */
    verifyEvent(receipt, contract, eventName, expectedArgs = {}) {
        const events = this.getEventLogs(receipt, contract, eventName);

        if (events.length === 0) {
            throw new Error(`事件未触发: ${eventName}`);
        }

        const event = events[0];

        // 验证事件参数
        for (const [key, expectedValue] of Object.entries(expectedArgs)) {
            const actualValue = event.args[key];

            if (actualValue === undefined) {
                throw new Error(`事件参数不存在: ${key}`);
            }

            // 对于 BigInt 类型，转换为字符串比较
            const actual = typeof actualValue === "bigint" ? actualValue.toString() : actualValue;
            const expected = typeof expectedValue === "bigint" ? expectedValue.toString() : expectedValue;

            if (actual !== expected) {
                throw new Error(
                    `事件参数不匹配: ${key}\n` +
                    `  期望: ${expected}\n` +
                    `  实际: ${actual}`
                );
            }
        }

        console.log(`  ✅ 事件验证通过: ${eventName}`);
        return event;
    }

    /**
     * 获取交易的 Gas 使用情况
     */
    getGasUsed(receipt) {
        return {
            gasUsed: receipt.gasUsed.toString(),
            effectiveGasPrice: receipt.gasPrice ? receipt.gasPrice.toString() : "0",
            totalCost: receipt.gasUsed * (receipt.gasPrice || 0n)
        };
    }

    /**
     * 批量检查余额
     */
    async checkBalances(accounts, tokens = []) {
        const balances = {};

        for (const account of accounts) {
            const address = typeof account === "string" ? account : account.address;
            const name = typeof account === "string" ? address.slice(0, 6) : account.name;

            balances[name] = {
                address,
                bnb: await this.provider.getBalance(address)
            };

            // 检查代币余额
            for (const token of tokens) {
                const tokenContract = new ethers.Contract(
                    token.address,
                    this.abis.erc20ABI,
                    this.provider
                );
                balances[name][token.symbol] = await tokenContract.balanceOf(address);
            }
        }

        return balances;
    }

    /**
     * 格式化余额输出
     */
    formatBalances(balances) {
        console.log("\n📊 账户余额:");
        console.log("─".repeat(80));

        for (const [name, balance] of Object.entries(balances)) {
            console.log(`\n${name} (${balance.address.slice(0, 6)}...${balance.address.slice(-4)}):`);
            console.log(`  BNB: ${ethers.formatEther(balance.bnb)}`);

            for (const [key, value] of Object.entries(balance)) {
                if (key !== "address" && key !== "bnb") {
                    console.log(`  ${key}: ${ethers.formatEther(value)}`);
                }
            }
        }

        console.log("─".repeat(80));
    }

    /**
     * 重试机制包装器
     */
    async withRetry(fn, maxRetries = 3, delay = 2000) {
        for (let i = 0; i < maxRetries; i++) {
            try {
                return await fn();
            } catch (error) {
                if (i === maxRetries - 1) throw error;

                console.log(`  ⚠️  尝试 ${i + 1} 失败，${delay / 1000}秒后重试...`);
                console.log(`     错误: ${error.message}`);

                await new Promise(resolve => setTimeout(resolve, delay));
            }
        }
    }

    /**
     * 获取项目信息
     */
    async getProject(projectId) {
        const project = await this.contracts.platform.getProject(projectId);

        return {
            projectId: project.projectId.toString(),
            projectToken: project.projectToken,
            projectOwner: project.projectOwner,
            tokenPrice: project.tokenPrice.toString(),
            hardCap: project.hardCap.toString(),
            softCap: project.softCap.toString(),
            maxPerUser: project.maxPerUser.toString(),
            startTime: Number(project.startTime),
            endTime: Number(project.endTime),
            totalRaised: project.totalRaised.toString(),
            totalTokensSold: project.totalTokensSold.toString(),
			totalTokensDeposited: project.totalTokensDeposited.toString(),
			fundsWithdrawn: project.fundsWithdrawn,
			projectTokensWithdrawn: project.projectTokensWithdrawn,
            status: Number(project.status),
            vestingTimes: project.vestingTimes.map(t => Number(t)),
            vestingPercents: project.vestingPercents.map(p => Number(p))
        };
    }

    /**
     * 获取用户认购信息
     */
    async getUserPurchase(projectId, userAddress) {
		const purchase = await this.contracts.platform.getUserPurchase(projectId, userAddress);

        return {
			totalJCCSpent: purchase.totalJCCSpent.toString(),
			totalTokensBought: purchase.totalTokensBought.toString(),
			claimedTokens: purchase.claimedTokens.toString(),
            refunded: purchase.refunded
        };
    }

	async userPurchases(projectId, userAddress) {
		return this.getUserPurchase(projectId, userAddress);
	}

    /**
     * 获取区块链浏览器链接
     */
    getExplorerLink(type, value) {
        const baseUrl = "https://testnet.bscscan.com";

        switch (type) {
            case "tx":
                return `${baseUrl}/tx/${value}`;
            case "address":
                return `${baseUrl}/address/${value}`;
            case "token":
                return `${baseUrl}/token/${value}`;
            default:
                return baseUrl;
        }
    }
}

module.exports = ContractHelper;

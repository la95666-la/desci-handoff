/**
 * Gas 追踪工具类
 * 记录和分析 Gas 使用情况
 */

const { ethers } = require("ethers");
const fs = require("fs");
const path = require("path");

class GasTracker {
    constructor() {
        this.transactions = [];
        this.startTime = Date.now();
    }

    /**
     * 记录交易的 Gas 使用
     */
    recordTransaction(name, receipt, category = "其他") {
        const gasUsed = receipt.gasUsed;
        const gasPrice = receipt.gasPrice || receipt.effectiveGasPrice || 0n;
        const gasCost = gasUsed * gasPrice;

        const record = {
            name,
            category,
            hash: receipt.hash,
            gasUsed: gasUsed.toString(),
            gasPrice: gasPrice.toString(),
            gasCost: gasCost.toString(),
            gasCostBNB: ethers.formatEther(gasCost),
            timestamp: new Date().toISOString(),
            blockNumber: receipt.blockNumber
        };

        this.transactions.push(record);

        console.log(`  💰 Gas 使用: ${gasUsed.toString()} (${ethers.formatEther(gasCost)} BNB)`);

        return record;
    }

    /**
     * 获取统计信息
     */
    getStats() {
        if (this.transactions.length === 0) {
            return {
                totalTransactions: 0,
                totalGasUsed: "0",
                totalCostBNB: "0",
                averageGasUsed: "0",
                averageCostBNB: "0",
                byCategory: {}
            };
        }

        const totalGasUsed = this.transactions.reduce(
            (sum, tx) => sum + BigInt(tx.gasUsed),
            0n
        );

        const totalCost = this.transactions.reduce(
            (sum, tx) => sum + BigInt(tx.gasCost),
            0n
        );

        const averageGasUsed = totalGasUsed / BigInt(this.transactions.length);
        const averageCost = totalCost / BigInt(this.transactions.length);

        // 按类别统计
        const byCategory = {};
        for (const tx of this.transactions) {
            if (!byCategory[tx.category]) {
                byCategory[tx.category] = {
                    count: 0,
                    totalGasUsed: 0n,
                    totalCost: 0n
                };
            }

            byCategory[tx.category].count++;
            byCategory[tx.category].totalGasUsed += BigInt(tx.gasUsed);
            byCategory[tx.category].totalCost += BigInt(tx.gasCost);
        }

        // 转换为可读格式
        const categoryStats = {};
        for (const [category, stats] of Object.entries(byCategory)) {
            categoryStats[category] = {
                count: stats.count,
                totalGasUsed: stats.totalGasUsed.toString(),
                totalCostBNB: ethers.formatEther(stats.totalCost),
                averageGasUsed: (stats.totalGasUsed / BigInt(stats.count)).toString(),
                averageCostBNB: ethers.formatEther(stats.totalCost / BigInt(stats.count))
            };
        }

        return {
            totalTransactions: this.transactions.length,
            totalGasUsed: totalGasUsed.toString(),
            totalCostBNB: ethers.formatEther(totalCost),
            averageGasUsed: averageGasUsed.toString(),
            averageCostBNB: ethers.formatEther(averageCost),
            byCategory: categoryStats
        };
    }

    /**
     * 打印 Gas 报告
     */
    printReport() {
        const stats = this.getStats();

        console.log("\n" + "═".repeat(80));
        console.log("                           Gas 使用报告");
        console.log("═".repeat(80));

        console.log("\n📊 总体统计:");
        console.log(`   交易数量: ${stats.totalTransactions}`);
        console.log(`   总 Gas 使用: ${stats.totalGasUsed}`);
        console.log(`   总成本: ${stats.totalCostBNB} BNB`);
        console.log(`   平均 Gas: ${stats.averageGasUsed}`);
        console.log(`   平均成本: ${stats.averageCostBNB} BNB`);

        if (Object.keys(stats.byCategory).length > 0) {
            console.log("\n📋 分类统计:");
            for (const [category, categoryStats] of Object.entries(stats.byCategory)) {
                console.log(`\n   ${category}:`);
                console.log(`     交易数: ${categoryStats.count}`);
                console.log(`     总 Gas: ${categoryStats.totalGasUsed}`);
                console.log(`     总成本: ${categoryStats.totalCostBNB} BNB`);
                console.log(`     平均 Gas: ${categoryStats.averageGasUsed}`);
                console.log(`     平均成本: ${categoryStats.averageCostBNB} BNB`);
            }
        }

        console.log("\n💸 成本估算 (按当前 BNB 价格 $500):");
        const bnbPrice = 500;
        const totalCostUSD = parseFloat(stats.totalCostBNB) * bnbPrice;
        console.log(`   总成本: $${totalCostUSD.toFixed(2)} USD`);

        console.log("\n" + "═".repeat(80) + "\n");

        return stats;
    }

    /**
     * 获取最昂贵的交易
     */
    getMostExpensive(count = 5) {
        return [...this.transactions]
            .sort((a, b) => BigInt(b.gasCost) - BigInt(a.gasCost))
            .slice(0, count);
    }

    /**
     * 打印最昂贵的交易
     */
    printMostExpensive(count = 5) {
        const expensive = this.getMostExpensive(count);

        console.log(`\n💰 最昂贵的 ${count} 笔交易:`);
        console.log("─".repeat(80));

        for (let i = 0; i < expensive.length; i++) {
            const tx = expensive[i];
            console.log(`\n${i + 1}. ${tx.name} (${tx.category})`);
            console.log(`   Gas: ${tx.gasUsed}`);
            console.log(`   成本: ${tx.gasCostBNB} BNB`);
            console.log(`   交易: ${tx.hash}`);
        }

        console.log("─".repeat(80) + "\n");
    }

    /**
     * 保存报告到文件
     */
    saveReport(filename = "gasReport.json") {
        const stats = this.getStats();
        const reportPath = path.join(__dirname, "../reports", filename);

        // 确保目录存在
        const dir = path.dirname(reportPath);
        if (!fs.existsSync(dir)) {
            fs.mkdirSync(dir, { recursive: true });
        }

        const report = {
            generatedAt: new Date().toISOString(),
            duration: Date.now() - this.startTime,
            stats,
            transactions: this.transactions,
            mostExpensive: this.getMostExpensive(10)
        };

        fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
        console.log(`\n💾 Gas 报告已保存: ${reportPath}\n`);

        return reportPath;
    }

    /**
     * 检查是否超出预算
     */
    checkBudget(budgetBNB) {
        const stats = this.getStats();
        const totalCost = parseFloat(stats.totalCostBNB);
        const budget = parseFloat(budgetBNB);

        const remaining = budget - totalCost;
        const percentUsed = (totalCost / budget) * 100;

        console.log("\n💰 预算检查:");
        console.log(`   预算: ${budget} BNB`);
        console.log(`   已用: ${totalCost.toFixed(6)} BNB (${percentUsed.toFixed(2)}%)`);
        console.log(`   剩余: ${remaining.toFixed(6)} BNB`);

        if (remaining < 0) {
            console.log(`   ⚠️  警告: 已超出预算 ${Math.abs(remaining).toFixed(6)} BNB`);
            return false;
        } else if (percentUsed > 80) {
            console.log(`   ⚠️  警告: 已使用超过 80% 的预算`);
        } else {
            console.log(`   ✅ 预算充足`);
        }

        return true;
    }

    /**
     * 重置追踪器
     */
    reset() {
        this.transactions = [];
        this.startTime = Date.now();
    }
}

module.exports = GasTracker;

/**
 * 验证工具类
 * 用于验证链上状态、余额和事件
 */

const { ethers } = require("ethers");

class VerificationHelper {
    constructor(contractHelper) {
        this.contractHelper = contractHelper;
        this.testResults = [];
    }

    /**
     * 记录测试结果
     */
    recordTest(category, name, passed, details = "") {
        const result = {
            category,
            name,
            passed,
            details,
            timestamp: new Date().toISOString()
        };

        this.testResults.push(result);

        const icon = passed ? "✅" : "❌";
        console.log(`${icon} ${category} - ${name}`);
        if (details) {
            console.log(`   ${details}`);
        }

        return passed;
    }

    /**
     * 验证项目状态
     */
    async verifyProjectState(projectId, expectedStatus, description = "") {
        try {
            const project = await this.contractHelper.getProject(projectId);
			const statusNames = ["Pending", "Active", "Ended", "Successful", "Failed", "Cancelled"];
            const actualStatus = statusNames[project.status];
            const expectedStatusName = statusNames[expectedStatus];

            const passed = project.status === expectedStatus;

            this.recordTest(
                "项目状态",
                description || `项目 ${projectId} 状态应为 ${expectedStatusName}`,
                passed,
                passed ? "" : `实际状态: ${actualStatus}`
            );

            return passed;
        } catch (error) {
            this.recordTest(
                "项目状态",
                description || `项目 ${projectId} 状态验证`,
                false,
                `错误: ${error.message}`
            );
            return false;
        }
    }

    /**
     * 验证余额
     */
    async verifyBalance(address, tokenContract, expectedBalance, tolerance = 0n, description = "") {
        try {
            const actualBalance = await tokenContract.balanceOf(address);
            const expected = ethers.parseEther(expectedBalance.toString());

            // 允许一定的误差范围
            const diff = actualBalance > expected ? actualBalance - expected : expected - actualBalance;
            const passed = diff <= tolerance;

            this.recordTest(
                "余额验证",
                description || `地址 ${address.slice(0, 6)}... 余额`,
                passed,
                passed
                    ? `余额: ${ethers.formatEther(actualBalance)}`
                    : `期望: ${ethers.formatEther(expected)}, 实际: ${ethers.formatEther(actualBalance)}`
            );

            return passed;
        } catch (error) {
            this.recordTest(
                "余额验证",
                description || `地址 ${address.slice(0, 6)}... 余额`,
                false,
                `错误: ${error.message}`
            );
            return false;
        }
    }

    /**
     * 验证用户认购信息
     */
    async verifyUserPurchase(projectId, userAddress, expectedPaid, expectedTokens, description = "") {
        try {
            const purchase = await this.contractHelper.contracts.platform.userPurchases(projectId, userAddress);

			const paidMatch = BigInt(purchase.totalJCCSpent) === ethers.parseEther(expectedPaid.toString());
			const tokensMatch = BigInt(purchase.totalTokensBought) === ethers.parseEther(expectedTokens.toString());
            const passed = paidMatch && tokensMatch;

            this.recordTest(
                "认购验证",
                description || `用户 ${userAddress.slice(0, 6)}... 认购信息`,
                passed,
                passed
                    ? `支付: ${ethers.formatEther(purchase.totalJCCSpent)} JCC, 获得: ${ethers.formatEther(purchase.totalTokensBought)} PT`
                    : `期望支付: ${expectedPaid}, 实际: ${ethers.formatEther(purchase.totalJCCSpent)}`
            );

            return passed;
        } catch (error) {
            this.recordTest(
                "认购验证",
                description || `用户 ${userAddress.slice(0, 6)}... 认购信息`,
                false,
                `错误: ${error.message}`
            );
            return false;
        }
    }

    /**
     * 验证项目募集金额
     */
    async verifyProjectRaised(projectId, expectedRaised, description = "") {
        try {
            const project = await this.contractHelper.getProject(projectId);
            const expected = ethers.parseEther(expectedRaised.toString());
            const passed = project.totalRaised === expected.toString();

            this.recordTest(
                "募集验证",
                description || `项目 ${projectId} 募集金额`,
                passed,
                passed
                    ? `募集: ${ethers.formatEther(project.totalRaised)} JCC`
                    : `期望: ${ethers.formatEther(expected)}, 实际: ${ethers.formatEther(project.totalRaised)}`
            );

            return passed;
        } catch (error) {
            this.recordTest(
                "募集验证",
                description || `项目 ${projectId} 募集金额`,
                false,
                `错误: ${error.message}`
            );
            return false;
        }
    }

    /**
     * 验证交易成功
     */
    verifyTransaction(receipt, description = "") {
        const passed = receipt.status === 1;

        this.recordTest(
            "交易验证",
            description || `交易 ${receipt.hash.slice(0, 10)}...`,
            passed,
            passed ? `Gas: ${receipt.gasUsed.toString()}` : "交易失败"
        );

        return passed;
    }

    /**
     * 验证合约配置
     */
    async verifyContractConfig(contract, expectedConfig, description = "") {
        try {
            const results = [];

            for (const [key, expectedValue] of Object.entries(expectedConfig)) {
                const actualValue = await contract[key]();
                const match = actualValue.toString() === expectedValue.toString();
                results.push({ key, match, actualValue, expectedValue });
            }

            const allMatch = results.every(r => r.match);

            this.recordTest(
                "配置验证",
                description || "合约配置",
                allMatch,
                allMatch
                    ? "所有配置正确"
                    : results
                          .filter(r => !r.match)
                          .map(r => `${r.key}: 期望 ${r.expectedValue}, 实际 ${r.actualValue}`)
                          .join("; ")
            );

            return allMatch;
        } catch (error) {
            this.recordTest(
                "配置验证",
                description || "合约配置",
                false,
                `错误: ${error.message}`
            );
            return false;
        }
    }

    /**
     * 生成测试报告
     */
    generateReport() {
        const total = this.testResults.length;
        const passed = this.testResults.filter(r => r.passed).length;
        const failed = total - passed;
        const passRate = total > 0 ? ((passed / total) * 100).toFixed(2) : 0;

        const report = {
            summary: {
                total,
                passed,
                failed,
                passRate: `${passRate}%`
            },
            byCategory: {},
            details: this.testResults
        };

        // 按类别统计
        for (const result of this.testResults) {
            if (!report.byCategory[result.category]) {
                report.byCategory[result.category] = {
                    total: 0,
                    passed: 0,
                    failed: 0
                };
            }

            report.byCategory[result.category].total++;
            if (result.passed) {
                report.byCategory[result.category].passed++;
            } else {
                report.byCategory[result.category].failed++;
            }
        }

        return report;
    }

    /**
     * 打印测试报告
     */
    printReport() {
        const report = this.generateReport();

        console.log("\n" + "═".repeat(80));
        console.log("                           测试报告");
        console.log("═".repeat(80));

        console.log("\n📊 总体统计:");
        console.log(`   总测试数: ${report.summary.total}`);
        console.log(`   通过: ${report.summary.passed} ✅`);
        console.log(`   失败: ${report.summary.failed} ❌`);
        console.log(`   通过率: ${report.summary.passRate}`);

        console.log("\n📋 分类统计:");
        for (const [category, stats] of Object.entries(report.byCategory)) {
            const rate = ((stats.passed / stats.total) * 100).toFixed(1);
            console.log(`   ${category}: ${stats.passed}/${stats.total} (${rate}%)`);
        }

        if (report.summary.failed > 0) {
            console.log("\n❌ 失败的测试:");
            const failedTests = this.testResults.filter(r => !r.passed);
            for (const test of failedTests) {
                console.log(`   • ${test.category} - ${test.name}`);
                if (test.details) {
                    console.log(`     ${test.details}`);
                }
            }
        }

        console.log("\n" + "═".repeat(80) + "\n");

        return report;
    }

    /**
     * 保存报告到文件
     */
    saveReport(filename) {
        const fs = require("fs");
        const path = require("path");

        const report = this.generateReport();
        const reportPath = path.join(__dirname, "../reports", filename);

        // 确保目录存在
        const dir = path.dirname(reportPath);
        if (!fs.existsSync(dir)) {
            fs.mkdirSync(dir, { recursive: true });
        }

        fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
        console.log(`\n💾 报告已保存: ${reportPath}\n`);

        return reportPath;
    }

    /**
     * 清空测试结果
     */
    reset() {
        this.testResults = [];
    }
}

module.exports = VerificationHelper;

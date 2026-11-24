/**
 * 生成 BNB Chain 钱包地址脚本
 *
 * 用法:
 *   node scripts/generateWallet.js                          # 生成单个钱包
 *   node scripts/generateWallet.js 5                        # 生成5个钱包
 *   node scripts/generateWallet.js 1 save                   # 生成钱包并保存到文件
 *   node scripts/generateWallet.js 4 save --roles deployer,relayer,job,user1
 *                                                            # 按用途标注生成并保存（顺序对应）
 *
 * --roles 说明：
 *   - 逗号分隔的用途标签，数量必须与生成数量一致，按顺序一一对应
 *   - 用途会写入 JSON（wallets[].purpose）、控制台标题与保存文件名
 *   - 常用组合（BSC 主网 IDO 测试，见 deploy 仓 IDO-BSC-MAINNET-DEPLOYMENT-GUIDE.md）：
 *       deployer,relayer,job,user1
 */

const { ethers } = require("ethers");
const fs = require("fs");
const path = require("path");

/**
 * 生成单个钱包
 * @returns {Object} 钱包信息对象
 */
function generateSingleWallet() {
    // 生成随机钱包
    const wallet = ethers.Wallet.createRandom();

    return {
        address: wallet.address,           // 公钥地址
        privateKey: wallet.privateKey,     // 私钥
        mnemonic: wallet.mnemonic.phrase,  // 助记词
        path: wallet.mnemonic.path,        // 派生路径
    };
}

/**
 * 格式化输出钱包信息
 * @param {Object} wallet 钱包对象
 * @param {number} index 钱包索引
 * @returns {string} 格式化的字符串
 */
function formatWalletOutput(wallet, index = 1, purpose = "") {
    const title = purpose ? `钱包 #${index} · ${purpose}` : `钱包 #${index}`;
    return `
╔════════════════════════════════════════════════════════════════╗
║                    ${title}
╚════════════════════════════════════════════════════════════════╝

📍 公钥地址 (Address):
   ${wallet.address}

🔑 私钥 (Private Key):
   ${wallet.privateKey}

📝 助记词 (Mnemonic):
   ${wallet.mnemonic}

🛣️  派生路径 (Derivation Path):
   ${wallet.path}

⚠️  安全提示:
   - 请妥善保管私钥，不要分享给任何人
   - 不要在公开场合暴露私钥
   - 建议将私钥保存在安全的地方（如硬件钱包）
   - 助记词可以恢复钱包，请妥善保管

`;
}

/**
 * 保存钱包信息到文件
 * @param {Array} wallets 钱包数组
 * @param {string} filename 文件名
 */
function saveWalletsToFile(wallets, purposes = [], filename = "") {
    const suffix = purposes.length ? `-${purposes.join("-")}` : "";
    const target = filename || `wallets${suffix}.json`;
    const filePath = path.join(__dirname, target);

    const data = {
        generatedAt: new Date().toISOString(),
        count: wallets.length,
        wallets: wallets.map((wallet, index) => ({
            index: index + 1,
            purpose: purposes[index] || "",
            address: wallet.address,
            privateKey: wallet.privateKey,
            mnemonic: wallet.mnemonic,
            path: wallet.path,
        })),
    };

    fs.writeFileSync(filePath, JSON.stringify(data, null, 2));
    console.log(`\n✅ 钱包信息已保存到: ${filePath}`);
    console.log(`⚠️  请妥善保管此文件，不要上传到版本控制系统！\n`);
}

/**
 * 主函数
 */
async function main() {
    try {
        // 获取命令行参数
        const args = process.argv.slice(2);
        const count = parseInt(args[0]) || 1;
        const shouldSave = args.includes("save");

        // --roles deployer,relayer,job,user1（可选；数量必须与 count 一致）
        const rolesIndex = args.indexOf("--roles");
        let purposes = [];
        if (rolesIndex !== -1) {
            purposes = String(args[rolesIndex + 1] || "")
                .split(",")
                .map((role) => role.trim())
                .filter(Boolean);
            if (purposes.length !== count) {
                throw new Error(`--roles 数量(${purposes.length})必须与生成数量(${count})一致：${purposes.join(",") || "(空)"}`);
            }
        }

        console.log(`\n🔐 正在生成 ${count} 个 BNB Chain 钱包...\n`);

        const wallets = [];

        // 生成指定数量的钱包
        for (let i = 0; i < count; i++) {
            const wallet = generateSingleWallet();
            wallets.push(wallet);
            console.log(formatWalletOutput(wallet, i + 1, purposes[i]));
        }

        // 如果指定了 save 参数，保存到文件
        if (shouldSave) {
            saveWalletsToFile(wallets, purposes);
        }

        // 显示总结
        console.log(`╔════════════════════════════════════════════════════════════════╗`);
        console.log(`║                        生成完成`);
        console.log(`╚════════════════════════════════════════════════════════════════╝`);
        console.log(`✅ 成功生成 ${count} 个钱包`);

        if (shouldSave) {
            console.log(`💾 钱包信息已保存到文件`);
        } else {
            console.log(`💡 提示: 使用 'node scripts/generateWallet.js ${count} save --roles deployer,relayer,job,user1' 保存到文件并标注用途\n`);
        }

    } catch (error) {
        console.error("❌ 生成钱包失败:", error.message);
        process.exit(1);
    }
}

// 运行主函数
main();

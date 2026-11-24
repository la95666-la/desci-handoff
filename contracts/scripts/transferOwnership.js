const { ethers } = require("hardhat");
require("dotenv").config();

/**
 * 将平台合约 owner 迁移到多签（安全审计 0.2）
 *
 * JC 代币使用 Ownable2Step：本脚本发起 transferOwnership 后，
 * 还需多签钱包执行 acceptOwnership() 才生效（防误转）。
 * JoinCare_v1 / JoinCareCheckIn 为 OZ Ownable 单步转移，发起即生效，请二次确认地址。
 *
 * 用法：
 *   MULTISIG_ADDRESS=0x<Safe> JC_TOKEN_ADDRESS=0x... IDO_PROXY_ADDRESS=0x... CHECKIN_PROXY_ADDRESS=0x... \
 *   JC_DEPLOY_RPC=<rpc> JC_DEPLOY_CHAIN_ID=<id> JC_DEPLOY_PRIVATE_KEY=<key> \
 *   npx hardhat run scripts/transferOwnership.js --network jcTarget
 *
 * 三个地址变量均可选，只迁移提供的合约。
 */
async function main() {
    const multisig = process.env.MULTISIG_ADDRESS;
    if (!multisig || !ethers.isAddress(multisig)) {
        throw new Error("请通过 MULTISIG_ADDRESS 提供多签（Gnosis Safe）地址");
    }

    const [deployer] = await ethers.getSigners();
    console.log("操作账户:", deployer.address);
    console.log("目标多签:", multisig, "\n");

    // JC 代币（Ownable2Step，需 Safe 后续 acceptOwnership）
    if (process.env.JC_TOKEN_ADDRESS) {
        const jc = await ethers.getContractAt("JC", process.env.JC_TOKEN_ADDRESS);
        const owner = await jc.owner();
        if (owner.toLowerCase() === multisig.toLowerCase()) {
            console.log("[JC] owner 已是多签，跳过");
        } else if (owner.toLowerCase() !== deployer.address.toLowerCase()) {
            console.log(`[JC] 当前 owner=${owner} 不是操作账户，跳过（请用当前 owner 账户执行）`);
        } else {
            const tx = await jc.transferOwnership(multisig);
            await tx.wait();
            console.log("[JC] 已发起 transferOwnership:", tx.hash);
            console.log("     ⚠️ 需多签执行 acceptOwnership() 后才生效（pendingOwner 查询确认）");
        }
    }

    // JoinCare_v1（OZ Ownable，单步生效）
    if (process.env.IDO_PROXY_ADDRESS) {
        const ido = await ethers.getContractAt("Ownable", process.env.IDO_PROXY_ADDRESS);
        const tx = await ido.transferOwnership(multisig);
        await tx.wait();
        console.log("[IDO] 已转移 owner:", tx.hash);
    }

    // JoinCareCheckIn（OZ Ownable，单步生效）
    if (process.env.CHECKIN_PROXY_ADDRESS) {
        const checkin = await ethers.getContractAt("Ownable", process.env.CHECKIN_PROXY_ADDRESS);
        const tx = await checkin.transferOwnership(multisig);
        await tx.wait();
        console.log("[CheckIn] 已转移 owner:", tx.hash);
    }

    console.log("\n完成。请确认所有合约 owner 已指向多签。");
}

main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error("执行失败:", error);
        process.exit(1);
    });

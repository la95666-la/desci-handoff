// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable2Step.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @dev JC 平台代币（2026-08-07 A3 口径：符号由 JCC 统一为 JC）
 * 标准 ERC20 代币，用于 JoinCare IDO 平台。
 *
 * 安全设计（安全审计 0.2）：
 * - MAX_SUPPLY 硬顶 10 亿（对齐白皮书 Tokenomics：总供应 1,000,000,000 固定）
 * - mint 仅 owner，且累计总量不得超过硬顶
 * - Ownable2Step：owner 变更需新 owner 显式接受，防误转到错误/失控地址；
 *   生产 owner 应为多签（Gnosis Safe），部署后执行 transferOwnership 迁移
 */
contract JC is ERC20, Ownable2Step {
    /// @dev 总量硬顶（18 decimals）
    uint256 public constant MAX_SUPPLY = 1_000_000_000 * 10 ** 18;

    /**
     * @dev 初始化 JC 代币
     * @param initialSupply 初始供应量（单位：token，不包括 decimals）
     */
    constructor(uint256 initialSupply) ERC20("JoinCare Coin", "JC") Ownable(msg.sender) {
        require(initialSupply * 10 ** decimals() <= MAX_SUPPLY, "JC: initial supply exceeds cap");
        // 铸造初始供应量（18 decimals）
        _mint(msg.sender, initialSupply * 10 ** decimals());
    }

    /**
     * @dev 铸造新代币（仅 owner，受总量硬顶约束）
     * @param to 接收地址
     * @param amount 铸造数量
     */
    function mint(address to, uint256 amount) public onlyOwner {
        require(totalSupply() + amount <= MAX_SUPPLY, "JC: mint exceeds max supply");
        _mint(to, amount);
    }

    /**
     * @dev 销毁代币
     * @param amount 销毁数量
     */
    function burn(uint256 amount) public {
        _burn(msg.sender, amount);
    }

    /**
     * @dev 销毁他人的代币（需要授权）
     * @param account 账户地址
     * @param amount 销毁数量
     */
    function burnFrom(address account, uint256 amount) public {
        uint256 currentAllowance = allowance(account, msg.sender);
        require(currentAllowance >= amount, "ERC20: burn amount exceeds allowance");
        unchecked {
            _approve(account, msg.sender, currentAllowance - amount);
        }
        _burn(account, amount);
    }
}

// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

import "@openzeppelin/contracts/token/ERC20/extensions/ERC20Burnable.sol";
import "@openzeppelin/contracts/access/Ownable2Step.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title TestProjectToken
 * @notice JoinCare IDO 测试环境专用的项目方 ERC20 代币。
 * @dev
 * - ERC20 默认使用 18 decimals，满足 JoinCare_v1.createProject 的精度要求；
 * - 初始供应量直接铸造给 initialOwner；
 * - owner 可在 maxSupply 硬顶内增发，便于重复执行充值/认购/Vesting 演练；
 * - Ownable2Step 防止测试项目方权限误转；
 * - 支持持有人 burn 与基于 allowance 的 burnFrom。
 *
 * 构造函数中的供应量参数均为最小单位（wei），部署脚本负责将人类可读数量
 * 按 18 decimals 转换。本合约仅用于本地链或公开测试网，不应作为生产项目代币。
 */
contract TestProjectToken is ERC20Burnable, Ownable2Step {
    error InvalidMaxSupply();
    error InitialSupplyExceedsMaxSupply(uint256 initialSupply, uint256 maxSupply);
    error MintExceedsMaxSupply(uint256 requested, uint256 available);

    /// @notice 代币总供应量硬顶（最小单位）
    uint256 public immutable maxSupply;

    constructor(
        string memory name_,
        string memory symbol_,
        uint256 initialSupply_,
        uint256 maxSupply_,
        address initialOwner_
    ) ERC20(name_, symbol_) Ownable(initialOwner_) {
        if (maxSupply_ == 0) revert InvalidMaxSupply();
        if (initialSupply_ > maxSupply_) {
            revert InitialSupplyExceedsMaxSupply(initialSupply_, maxSupply_);
        }

        maxSupply = maxSupply_;
        _mint(initialOwner_, initialSupply_);
    }

    /**
     * @notice 在供应量硬顶内增发测试代币。
     * @param to 接收地址
     * @param amount 增发数量（最小单位）
     */
    function mint(address to, uint256 amount) external onlyOwner {
        uint256 available = maxSupply - totalSupply();
        if (amount > available) revert MintExceedsMaxSupply(amount, available);
        _mint(to, amount);
    }
}

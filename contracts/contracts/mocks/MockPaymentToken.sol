// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable2Step.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title MockPaymentToken
 * @notice JoinCare 本地链和公开测试网专用的稳定币测试基类。
 * @dev 供应量参数使用最小单位。owner 可在不可变供应硬顶内增发，权限迁移使用 Ownable2Step。
 */
abstract contract MockPaymentToken is ERC20, Ownable2Step {
    error InvalidDecimals(uint8 decimals);
    error InvalidMaxSupply();
    error InitialSupplyExceedsMaxSupply(uint256 initialSupply, uint256 maxSupply);
    error MintExceedsMaxSupply(uint256 requested, uint256 available);

    uint8 private immutable _tokenDecimals;
    uint256 public immutable maxSupply;

    constructor(
        string memory name_,
        string memory symbol_,
        uint8 decimals_,
        uint256 initialSupply_,
        uint256 maxSupply_,
        address initialOwner_
    ) ERC20(name_, symbol_) Ownable(initialOwner_) {
        if (decimals_ == 0 || decimals_ > 18) revert InvalidDecimals(decimals_);
        if (maxSupply_ == 0) revert InvalidMaxSupply();
        if (initialSupply_ > maxSupply_) {
            revert InitialSupplyExceedsMaxSupply(initialSupply_, maxSupply_);
        }

        _tokenDecimals = decimals_;
        maxSupply = maxSupply_;
        _mint(initialOwner_, initialSupply_);
    }

    function decimals() public view virtual override returns (uint8) {
        return _tokenDecimals;
    }

    function mint(address to, uint256 amount) external onlyOwner {
        uint256 available = maxSupply - totalSupply();
        if (amount > available) revert MintExceedsMaxSupply(amount, available);
        _mint(to, amount);
    }
}

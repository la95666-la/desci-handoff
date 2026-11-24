// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

import "./MockERC20.sol";

/**
 * @title MockJCC
 * @dev JCC平台代币（用于测试）
 */
contract MockJCC is MockERC20 {
    constructor(uint256 initialSupply)
        MockERC20("JoinCare Coin", "JC", initialSupply)
    {}
}

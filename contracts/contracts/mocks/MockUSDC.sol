// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

import "./MockPaymentToken.sol";

/**
 * @title MockUSDC
 * @notice 18 decimals 的测试支付币，用于覆盖本期第二种稳定币配置。
 */
contract MockUSDC is MockPaymentToken {
    constructor(uint256 initialSupply_, uint256 maxSupply_, address initialOwner_)
        MockPaymentToken(
            "JoinCare Mock USDC",
            "mUSDC",
            18,
            initialSupply_,
            maxSupply_,
            initialOwner_
        )
    {}
}

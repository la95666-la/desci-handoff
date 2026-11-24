// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

import "./MockPaymentToken.sol";

/**
 * @title MockUSDT
 * @notice 6 decimals 的测试支付币，用于覆盖 USDT 精度路径。
 */
contract MockUSDT is MockPaymentToken {
    constructor(uint256 initialSupply_, uint256 maxSupply_, address initialOwner_)
        MockPaymentToken(
            "JoinCare Mock USDT",
            "mUSDT",
            6,
            initialSupply_,
            maxSupply_,
            initialOwner_
        )
    {}
}

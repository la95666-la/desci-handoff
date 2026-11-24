// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

import "../JoinCareIDOv3.sol";

/** @dev Storage-compatible local regression fixture, NOT an audited production implementation. */
/// @custom:oz-upgrades-unsafe-allow missing-initializer
contract JoinCareIDOv3UpgradeMock is JoinCareIDOv3 {
    function implementationVersion() external pure returns (uint256) {
        return 2;
    }
}

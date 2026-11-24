// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

import "../JoinCareIDOv2.sol";

/**
 * @notice Storage-compatible implementation used only by the V2 upgrade regression test.
 */
/// @custom:oz-upgrades-unsafe-allow missing-initializer
contract JoinCareIDOv2UpgradeMock is JoinCareIDOv2 {
    function implementationVersion() external pure returns (uint256) {
        return 2;
    }
}

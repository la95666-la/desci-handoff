// SPDX-License-Identifier: MIT
pragma solidity 0.8.30;

import "../JoinCareCheckIn.sol";

contract JoinCareCheckInV2 is JoinCareCheckIn {
    function version() external pure returns (uint256) {
        return 2;
    }
}

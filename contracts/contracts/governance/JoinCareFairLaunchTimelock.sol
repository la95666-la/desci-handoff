// SPDX-License-Identifier: MIT
pragma solidity 0.8.22;

import {TimelockController} from "@openzeppelin/contracts/governance/TimelockController.sol";

/// @notice Fair Launch governance timelock with no deployer or external admin.
/// @dev The TimelockController remains self-administered. Proposers also receive
///      the canceller role through the OpenZeppelin constructor.
contract JoinCareFairLaunchTimelock is TimelockController {
    constructor(uint256 minDelay, address[] memory proposers, address[] memory executors)
        TimelockController(minDelay, proposers, executors, address(0))
    {}
}

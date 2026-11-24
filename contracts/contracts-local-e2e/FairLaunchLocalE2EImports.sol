// SPDX-License-Identifier: MIT
pragma solidity 0.8.22;

// TEST ONLY compile root. Production configuration continues to compile from
// contracts/ and never sees FairLaunchSafeExecutor.
import "../contracts/JoinCareFairLaunchV1.sol";
import "../contracts/fairlaunch/JoinCareFairLaunchV1CoreFacet.sol";
import "../contracts/fairlaunch/JoinCareFairLaunchV1ReleaseFacet.sol";
import "../contracts/fairlaunch/JoinCareFairLaunchV1FundsFacet.sol";
import "../contracts/fairlaunch/JoinCareFairLaunchV1AdminFacet.sol";
import "../contracts/fairlaunch/JoinCareFairLaunchV1ViewsFacet.sol";
import "../contracts/libraries/JoinCareFairLaunchV1Logic.sol";
import "../contracts/libraries/JoinCareFairLaunchV1Runtime.sol";
import "../contracts/mocks/FairLaunchERC1967Proxy.sol";
import "../contracts/mocks/FairLaunchTokenMock.sol";
import "./FairLaunchSafeExecutor.sol";

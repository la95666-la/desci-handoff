// SPDX-License-Identifier: MIT
pragma solidity 0.8.22;

import {Math} from "@openzeppelin/contracts/utils/math/Math.sol";

/// @notice Stateless deterministic helpers for JoinCareFairLaunchV1.
/// @dev This linked library MUST NOT read/write contract storage, call tokens,
///      authorize callers, selfdestruct, or retain configuration. A library
///      change requires a new implementation and the normal governed upgrade.
library JoinCareFairLaunchV1Logic {
    error InvalidAddress();
    error InvalidAmount();
    error InvalidProjectParameters();
    error InvalidVestingSchedule();
    error BeneficiarySafeNotApproved(address account);
    error BeneficiarySafeAttestationMismatch(address account, bytes32 expectedDigest, uint32 expectedVersion, bytes32 actualDigest, uint32 actualVersion);
    error TokenNotConfigured(address token);
    error TokenNotAllowed(address token);
    error TokenEmergencyBlocked(address token);
    error TokenPolicySnapshotChanged(address token, bytes32 expectedDigest, uint32 expectedVersion, bytes32 actualDigest, uint32 actualVersion);
    error UnsupportedTokenBehavior(address token, uint256 requested, uint256 observed);
    error BelowMinimumContribution(uint256 minimum, uint256 actual);
    error ExceedsMaxContribution(uint256 maximum, uint256 resultingTotal);
    error ExceedsSaleTokenAmount(uint256 saleTokenAmount, uint256 resultingDeposit);
    error InvalidClaimsPauseUntil(uint64 requestedUntil, uint64 maximumUntil);
    error UnsupportedOutgoingTokenBehavior(address token, uint256 requested, uint256 senderDecrease, uint256 recipientIncrease);

    function validateCreateBasics(
        uint256 projectId,
        address projectToken,
        address paymentToken,
        address projectOwner,
        address proceedsReceiver,
        address feeReceiver,
        uint256 saleTokenAmount,
        uint256 softCap,
        uint256 minContribution,
        uint256 maxContribution,
        uint64 startTime,
        uint64 endTime,
        uint256 nowTime
    ) external pure {
        if (projectId == 0 || projectToken == address(0) || paymentToken == address(0) || projectOwner == address(0) || proceedsReceiver == address(0) || feeReceiver == address(0)) revert InvalidAddress();
        if (projectToken == paymentToken || saleTokenAmount == 0 || softCap == 0 || minContribution == 0 || (maxContribution != 0 && maxContribution < minContribution) || startTime <= nowTime || endTime <= startTime) revert InvalidProjectParameters();
    }

    function validateVesting(uint64[] calldata times, uint16[] calldata bps, uint64 endTime) external pure {
        if (times.length == 0 || times.length > 10 || times.length != bps.length) revert InvalidVestingSchedule();
        uint256 totalBps;
        uint64 previous;
        for (uint256 i; i < times.length; ++i) {
            if (times[i] < endTime || times[i] <= previous || bps[i] == 0) revert InvalidVestingSchedule();
            previous = times[i];
            totalBps += bps[i];
        }
        if (totalBps != 10_000) revert InvalidVestingSchedule();
    }

    function validateBeneficiary(address account, bool approved, bytes32 expectedDigest, uint32 expectedVersion, bytes32 actualDigest, uint32 actualVersion) external pure {
        if (!approved) revert BeneficiarySafeNotApproved(account);
        if (actualDigest != expectedDigest || actualVersion != expectedVersion) revert BeneficiarySafeAttestationMismatch(account, expectedDigest, expectedVersion, actualDigest, actualVersion);
    }

    function validatePolicy(address token, uint32 actualVersion, bool enabled, bool blocked, bytes32 actualDigest, bytes32 expectedDigest, uint32 expectedVersion) external pure {
        if (actualVersion == 0) revert TokenNotConfigured(token);
        if (!enabled) revert TokenNotAllowed(token);
        if (blocked) revert TokenEmergencyBlocked(token);
        if (actualDigest != expectedDigest || actualVersion != expectedVersion) revert TokenPolicySnapshotChanged(token, expectedDigest, expectedVersion, actualDigest, actualVersion);
    }

    function allocation(uint256 contribution, uint256 saleTokenAmount, uint256 finalTotalRaised) external pure returns (uint256) {
        if (finalTotalRaised == 0) return 0;
        return Math.mulDiv(contribution, saleTokenAmount, finalTotalRaised);
    }

    function proportional(uint256 amount, uint256 bps) external pure returns (uint256) {
        return Math.mulDiv(amount, bps, 10_000);
    }

    function validateDeposit(uint256 saleTokenAmount, uint256 currentDeposit, uint256 amount) external pure {
        if (amount == 0) revert InvalidAmount();
        uint256 resulting = currentDeposit + amount;
        if (resulting > saleTokenAmount) revert ExceedsSaleTokenAmount(saleTokenAmount, resulting);
    }

    function validateContribution(uint256 minimum, uint256 maximum, uint256 amount, uint256 resultingTotal) external pure {
        if (amount == 0) revert InvalidAmount();
        if (amount < minimum) revert BelowMinimumContribution(minimum, amount);
        if (maximum != 0 && resultingTotal > maximum) revert ExceedsMaxContribution(maximum, resultingTotal);
    }

    function validateClaimsPause(uint64 requested, uint64 nowTime, uint64 maximum) external pure {
        if (requested <= nowTime || requested > maximum) revert InvalidClaimsPauseUntil(requested, maximum);
    }

    function validateIncoming(address token, uint256 requested, uint256 observed) external pure {
        if (observed != requested) revert UnsupportedTokenBehavior(token, requested, observed);
    }

    function validateOutgoing(address token, uint256 requested, uint256 senderDecrease, uint256 recipientIncrease) external pure {
        if (senderDecrease != requested || recipientIncrease != requested) revert UnsupportedOutgoingTokenBehavior(token, requested, senderDecrease, recipientIncrease);
    }
}

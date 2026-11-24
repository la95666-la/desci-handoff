// SPDX-License-Identifier: MIT
pragma solidity 0.8.30;

import "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";

import "./interfaces/IJoinCareCheckIn.sol";

/**
 * @title JoinCareCheckIn
 * @dev Task-center daily check-in. Users call `checkIn()` directly and pay
 *      their own gas. Each (user, UTC day) can be checked in at most once.
 */
contract JoinCareCheckIn is
    IJoinCareCheckIn,
    Initializable,
    UUPSUpgradeable,
    OwnableUpgradeable,
    PausableUpgradeable
{
    using SafeERC20 for IERC20;

    error OwnershipRenunciationDisabled();

    // ═══════════════════════════════════════════════════════════════════
    // Storage (append-only; DO NOT reorder / remove for UUPS compatibility)
    // ═══════════════════════════════════════════════════════════════════

    // (user, epochDay) → checked-in.
    // epochDay = block.timestamp / 86400 (UTC day count since 1970-01-01).
    mapping(address => mapping(uint256 => bool)) public checkedDay;

    uint256[50] private __gap;

    /// @custom:oz-upgrades-unsafe-allow constructor
    constructor() {
        _disableInitializers();
    }

    // ═══════════════════════════════════════════════════════════════════
    // Init
    // ═══════════════════════════════════════════════════════════════════

    function initialize() public initializer {
        __Ownable_init(msg.sender);
        __Pausable_init();
        // OZ contracts-upgradeable v5.2+ re-exports UUPSUpgradeable from the
        // non-upgradeable package (no storage, no init) — no __UUPSUpgradeable_init call.
    }

    function _authorizeUpgrade(address newImplementation)
        internal
        override
        onlyOwner
    {
        // Only owner can upgrade the check-in implementation.
    }

    function renounceOwnership() public view override onlyOwner {
        revert OwnershipRenunciationDisabled();
    }

    // ═══════════════════════════════════════════════════════════════════
    // Check-in (user self-paid)
    // ═══════════════════════════════════════════════════════════════════

    /// @dev Anyone can call this once per UTC day for themselves. Gas paid by msg.sender.
    function checkIn() external whenNotPaused {
        uint256 day = block.timestamp / 86400;
        require(!checkedDay[msg.sender][day], "Already checked in");
        checkedDay[msg.sender][day] = true;
        emit UserCheckedIn(msg.sender, day, block.timestamp);
    }

    /// @dev Convenience view: whether the given user has checked in today (UTC).
    function hasCheckedInToday(address user) external view returns (bool) {
        return checkedDay[user][block.timestamp / 86400];
    }

    // ═══════════════════════════════════════════════════════════════════
    // Owner controls
    // ═══════════════════════════════════════════════════════════════════

    function pause() external onlyOwner {
        _pause();
    }

    function unpause() external onlyOwner {
        _unpause();
    }

    /**
     * @dev Rescue ERC20 tokens accidentally sent to this contract.
     *      The contract holds no operational balance, so any ERC20 on the
     *      address is considered mis-sent and can be recovered by owner.
     *      Uses SafeERC20 so non-standard tokens (missing return value) work.
     */
    function rescueERC20(IERC20 token, address to, uint256 amount) external onlyOwner {
        require(to != address(0), "Invalid recipient");
        token.safeTransfer(to, amount);
        emit TokensRescued(address(token), to, amount);
    }
}

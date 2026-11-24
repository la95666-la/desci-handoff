// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

// NON-PRODUCTION BEHAVIORAL MODEL — MUST NOT DEPLOY.
// This deliberately omits UUPS, governance, token-policy attestations, release
// evidence, pause controls, payout flows and recovery controls. It exists only
// to make the frozen Fair Launch arithmetic, lifecycle and reserve invariants
// executable in local tests. It is not the candidate production implementation.

import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import {Math} from "@openzeppelin/contracts/utils/math/Math.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

contract JoinCareFairLaunchV1BehavioralModel is ReentrancyGuard {
    using SafeERC20 for IERC20;

    uint16 private constant BPS_DENOMINATOR = 10_000;
    uint8 private constant MAX_VESTING_PERIODS = 10;

    // Order and meaning intentionally match the frozen candidate baseline.
    enum ProjectStatus {
        Pending,
        Active,
        SuccessfulPendingRelease,
        Successful,
        Failed,
        Cancelled
    }

    enum TerminalReason {
        None,
        SoftCapNotReached,
        CancelledByOperator,
        TokenRuntimeInvalid,
        GovernanceInvalidation,
        PaymentTokenGovernanceInvalidation
    }

    struct ModelCreateProjectParams {
        uint256 projectId;
        address projectToken;
        address paymentToken;
        address projectOwner;
        uint256 saleTokenAmount;
        uint256 softCap;
        uint256 minContribution;
        uint256 maxContribution;
        uint64 startTime;
        uint64 endTime;
    }

    // This is the executable subset of the frozen Project semantics.
    struct Project {
        uint256 projectId;
        address projectToken;
        address paymentToken;
        address projectOwner;
        ProjectStatus status;
        TerminalReason terminalReason;
        uint64 startTime;
        uint64 endTime;
        uint256 saleTokenAmount;
        uint256 softCap;
        uint256 minContribution;
        uint256 maxContribution;
        uint256 totalRaised;
        uint256 totalTokensDeposited;
        uint256 totalTokensClaimed;
        uint256 participantCount;
        uint256 finalTotalRaised;
        uint256 finalPriceNumerator;
        uint256 finalPriceDenominator;
        bool failedProjectTokensWithdrawn;
    }

    struct UserPosition {
        uint256 contributionAmount;
        uint256 claimedTokens;
        bool refunded;
    }

    address public immutable operator;
    mapping(uint256 => Project) private _projects;
    mapping(uint256 => mapping(address => UserPosition)) private _positions;
    mapping(uint256 => mapping(address => bool)) public hasParticipated;
    mapping(uint256 => uint64[]) private _vestingTimes;
    mapping(uint256 => uint16[]) private _vestingBps;
    mapping(address => uint256) private _reservedBalances;

    error InvalidAddress();
    error InvalidAmount();
    error InvalidProjectParameters();
    error InvalidVestingSchedule();
    error ProjectAlreadyExists(uint256 projectId);
    error ProjectDoesNotExist(uint256 projectId);
    error InvalidProjectStatus(ProjectStatus expected, ProjectStatus actual);
    error UnauthorizedOperator(address caller);
    error UnauthorizedProjectOwner(address caller);
    error ProjectNotStarted(uint64 startTime);
    error ProjectNotEnded(uint64 endTime);
    error ProjectEndedAlready(uint64 endTime);
    error BelowMinimumContribution(uint256 minimum, uint256 actual);
    error ExceedsMaxContribution(uint256 maximum, uint256 resultingTotal);
    error ExceedsSaleTokenAmount(uint256 saleTokenAmount, uint256 resultingDeposit);
    error InsufficientProjectTokens(uint256 required, uint256 deposited);
    error UnsupportedTokenBehavior(address token, uint256 requested, uint256 observed);
    error UnsupportedOutgoingTokenBehavior(address token, uint256 requested, uint256 senderDecrease, uint256 recipientIncrease);
    error TokenReserveInsolvent(address token, uint256 balance, uint256 reserved);
    error NoContribution(address user);
    error AlreadyRefunded(address user);
    error NoTokensToClaim(address user);
    error AlreadyWithdrawn();
    error CancellationWindowClosed(uint64 endTime);

    event ModelProjectCreated(uint256 indexed projectId, uint256 saleTokenAmount, uint256 softCap);
    event ModelTokensDeposited(uint256 indexed projectId, uint256 amount, uint256 totalDeposited);
    event ModelActivated(uint256 indexed projectId);
    event ModelContribution(uint256 indexed projectId, address indexed user, uint256 amount, uint256 userTotal, uint256 totalRaised);
    event ModelEnded(uint256 indexed projectId, ProjectStatus status, TerminalReason reason, uint256 finalTotalRaised);
    event ModelSuccessfulReleaseAcknowledged(uint256 indexed projectId);
    event ModelCancelled(uint256 indexed projectId, ProjectStatus previousStatus);
    event ModelRefunded(uint256 indexed projectId, address indexed user, uint256 amount);
    event ModelClaimed(uint256 indexed projectId, address indexed user, uint256 amount);
    event ModelFailedTokensWithdrawn(uint256 indexed projectId, uint256 amount);

    constructor(address modelOperator) {
        if (modelOperator == address(0)) revert InvalidAddress();
        operator = modelOperator;
    }

    modifier onlyOperator() {
        if (msg.sender != operator) revert UnauthorizedOperator(msg.sender);
        _;
    }

    function createProject(
        ModelCreateProjectParams calldata params,
        uint64[] calldata vestingTimes,
        uint16[] calldata vestingBps
    ) external onlyOperator {
        if (params.projectId == 0) revert InvalidProjectParameters();
        if (_projects[params.projectId].projectId != 0) revert ProjectAlreadyExists(params.projectId);
        if (
            params.projectToken == address(0) || params.paymentToken == address(0) ||
            params.projectOwner == address(0)
        ) revert InvalidAddress();
        if (
            params.projectToken == params.paymentToken || params.saleTokenAmount == 0 ||
            params.softCap == 0 || params.minContribution == 0 ||
            (params.maxContribution != 0 && params.maxContribution < params.minContribution) ||
            params.startTime <= block.timestamp || params.endTime <= params.startTime
        ) revert InvalidProjectParameters();
        _validateVesting(params.endTime, vestingTimes, vestingBps);

        Project storage project = _projects[params.projectId];
        project.projectId = params.projectId;
        project.projectToken = params.projectToken;
        project.paymentToken = params.paymentToken;
        project.projectOwner = params.projectOwner;
        project.status = ProjectStatus.Pending;
        project.startTime = params.startTime;
        project.endTime = params.endTime;
        project.saleTokenAmount = params.saleTokenAmount;
        project.softCap = params.softCap;
        project.minContribution = params.minContribution;
        project.maxContribution = params.maxContribution;
        _vestingTimes[params.projectId] = vestingTimes;
        _vestingBps[params.projectId] = vestingBps;

        emit ModelProjectCreated(params.projectId, params.saleTokenAmount, params.softCap);
    }

    function depositProjectTokens(uint256 projectId, uint256 amount) external nonReentrant {
        Project storage project = _requireProject(projectId);
        _requireStatus(project, ProjectStatus.Pending);
        if (msg.sender != project.projectOwner) revert UnauthorizedProjectOwner(msg.sender);
        if (amount == 0) revert InvalidAmount();
        uint256 resultingDeposit = project.totalTokensDeposited + amount;
        if (resultingDeposit > project.saleTokenAmount) {
            revert ExceedsSaleTokenAmount(project.saleTokenAmount, resultingDeposit);
        }

        // Fail closed before either an external token call or a new reserve obligation.
        _assertSolvent(project.projectToken);
        _pullExact(project.projectToken, msg.sender, amount);
        project.totalTokensDeposited = resultingDeposit;
        _reservedBalances[project.projectToken] += amount;
        _assertSolvent(project.projectToken);
        emit ModelTokensDeposited(projectId, amount, resultingDeposit);
    }

    function activateProject(uint256 projectId) external onlyOperator {
        Project storage project = _requireProject(projectId);
        _requireStatus(project, ProjectStatus.Pending);
        if (block.timestamp >= project.startTime) revert ProjectEndedAlready(project.startTime);
        if (project.totalTokensDeposited != project.saleTokenAmount) {
            revert InsufficientProjectTokens(project.saleTokenAmount, project.totalTokensDeposited);
        }
        project.status = ProjectStatus.Active;
        emit ModelActivated(projectId);
    }

    function contribute(uint256 projectId, uint256 amount) external nonReentrant {
        Project storage project = _requireProject(projectId);
        _requireStatus(project, ProjectStatus.Active);
        if (block.timestamp < project.startTime) revert ProjectNotStarted(project.startTime);
        if (block.timestamp >= project.endTime) revert ProjectEndedAlready(project.endTime);
        if (amount < project.minContribution) revert BelowMinimumContribution(project.minContribution, amount);

        UserPosition storage position = _positions[projectId][msg.sender];
        uint256 resultingTotal = position.contributionAmount + amount;
        if (project.maxContribution != 0 && resultingTotal > project.maxContribution) {
            revert ExceedsMaxContribution(project.maxContribution, resultingTotal);
        }

        // Existing aggregate reserves must be solvent before transfer or obligation growth.
        _assertSolvent(project.paymentToken);
        _pullExact(project.paymentToken, msg.sender, amount);
        position.contributionAmount = resultingTotal;
        project.totalRaised += amount; // Intentionally no hard-cap branch.
        _reservedBalances[project.paymentToken] += amount;
        if (!hasParticipated[projectId][msg.sender]) {
            hasParticipated[projectId][msg.sender] = true;
            project.participantCount += 1;
        }
        _assertSolvent(project.paymentToken);
        emit ModelContribution(projectId, msg.sender, amount, resultingTotal, project.totalRaised);
    }

    // O(1): settlement writes project aggregates only and never iterates users.
    function endProject(uint256 projectId) external {
        Project storage project = _requireProject(projectId);
        _requireStatus(project, ProjectStatus.Active);
        if (block.timestamp < project.endTime) revert ProjectNotEnded(project.endTime);

        project.finalTotalRaised = project.totalRaised;
        if (project.totalRaised >= project.softCap) {
            project.status = ProjectStatus.SuccessfulPendingRelease;
            project.finalPriceNumerator = project.totalRaised;
            project.finalPriceDenominator = project.saleTokenAmount;
        } else {
            project.status = ProjectStatus.Failed;
            project.terminalReason = TerminalReason.SoftCapNotReached;
        }
        emit ModelEnded(projectId, project.status, project.terminalReason, project.finalTotalRaised);
    }

    // Test-only stand-in for the omitted canonical release-evidence workflow.
    function acknowledgeSuccessfulReleaseForModel(uint256 projectId) external onlyOperator {
        Project storage project = _requireProject(projectId);
        _requireStatus(project, ProjectStatus.SuccessfulPendingRelease);
        project.status = ProjectStatus.Successful;
        emit ModelSuccessfulReleaseAcknowledged(projectId);
    }

    function cancelProject(uint256 projectId) external onlyOperator {
        Project storage project = _requireProject(projectId);
        ProjectStatus previous = project.status;
        if (previous == ProjectStatus.Active && block.timestamp >= project.endTime) {
            revert CancellationWindowClosed(project.endTime);
        }
        if (previous != ProjectStatus.Pending && previous != ProjectStatus.Active) {
            revert InvalidProjectStatus(ProjectStatus.Active, previous);
        }
        project.status = ProjectStatus.Cancelled;
        project.terminalReason = TerminalReason.CancelledByOperator;
        emit ModelCancelled(projectId, previous);
    }

    function refund(uint256 projectId) external nonReentrant {
        _refundFor(projectId, msg.sender);
    }

    function refundFor(uint256 projectId, address user) external nonReentrant {
        _refundFor(projectId, user);
    }

    function claim(uint256 projectId) external nonReentrant {
        _claimFor(projectId, msg.sender);
    }

    function claimFor(uint256 projectId, address user) external nonReentrant {
        _claimFor(projectId, user);
    }

    function withdrawFailedProjectTokens(uint256 projectId) external nonReentrant {
        Project storage project = _requireProject(projectId);
        if (project.status != ProjectStatus.Failed && project.status != ProjectStatus.Cancelled) {
            revert InvalidProjectStatus(ProjectStatus.Failed, project.status);
        }
        if (msg.sender != project.projectOwner) revert UnauthorizedProjectOwner(msg.sender);
        if (project.failedProjectTokensWithdrawn) revert AlreadyWithdrawn();

        uint256 amount = project.totalTokensDeposited;
        // Check before changing the withdrawal flag/reserve or calling the token.
        _assertSolvent(project.projectToken);
        project.failedProjectTokensWithdrawn = true;
        _reservedBalances[project.projectToken] -= amount;
        _pushExact(project.projectToken, msg.sender, amount);
        _assertSolvent(project.projectToken);
        emit ModelFailedTokensWithdrawn(projectId, amount);
    }

    function calculateFinalAllocation(uint256 projectId, address user) public view returns (uint256) {
        Project storage project = _requireProjectView(projectId);
        if (
            project.status != ProjectStatus.SuccessfulPendingRelease &&
            project.status != ProjectStatus.Successful
        ) return 0;
        if (project.finalTotalRaised == 0) return 0;
        return Math.mulDiv(
            _positions[projectId][user].contributionAmount,
            project.saleTokenAmount,
            project.finalTotalRaised
        );
    }

    function calculateClaimable(uint256 projectId, address user) public view returns (uint256) {
        Project storage project = _requireProjectView(projectId);
        if (project.status != ProjectStatus.Successful) return 0;
        uint256 vested = Math.mulDiv(calculateFinalAllocation(projectId, user), _unlockedBps(projectId), BPS_DENOMINATOR);
        uint256 claimed = _positions[projectId][user].claimedTokens;
        return vested > claimed ? vested - claimed : 0;
    }

    function getProject(uint256 projectId) external view returns (Project memory) {
        return _requireProjectView(projectId);
    }

    function getPosition(uint256 projectId, address user) external view returns (UserPosition memory) {
        _requireProjectView(projectId);
        return _positions[projectId][user];
    }

    function getVestingSchedule(uint256 projectId) external view returns (uint64[] memory, uint16[] memory) {
        _requireProjectView(projectId);
        return (_vestingTimes[projectId], _vestingBps[projectId]);
    }

    function reservedBalance(address token) external view returns (uint256) {
        return _reservedBalances[token];
    }

    function modelDisclaimer() external pure returns (string memory) {
        return "NON_PRODUCTION_BEHAVIORAL_MODEL_DO_NOT_DEPLOY";
    }

    function _refundFor(uint256 projectId, address user) private {
        Project storage project = _requireProject(projectId);
        if (project.status != ProjectStatus.Failed && project.status != ProjectStatus.Cancelled) {
            revert InvalidProjectStatus(ProjectStatus.Failed, project.status);
        }
        UserPosition storage position = _positions[projectId][user];
        uint256 amount = position.contributionAmount;
        if (amount == 0) revert NoContribution(user);
        if (position.refunded) revert AlreadyRefunded(user);

        // Check before changing refund state/reserve or calling the token.
        _assertSolvent(project.paymentToken);
        position.refunded = true;
        _reservedBalances[project.paymentToken] -= amount;
        _pushExact(project.paymentToken, user, amount);
        _assertSolvent(project.paymentToken);
        emit ModelRefunded(projectId, user, amount);
    }

    function _claimFor(uint256 projectId, address user) private {
        Project storage project = _requireProject(projectId);
        _requireStatus(project, ProjectStatus.Successful);
        uint256 amount = calculateClaimable(projectId, user);
        if (amount == 0) revert NoTokensToClaim(user);

        // Check before changing entitlement accounting/reserve or calling the token.
        _assertSolvent(project.projectToken);
        UserPosition storage position = _positions[projectId][user];
        position.claimedTokens += amount;
        project.totalTokensClaimed += amount;
        _reservedBalances[project.projectToken] -= amount;
        _pushExact(project.projectToken, user, amount);
        _assertSolvent(project.projectToken);
        emit ModelClaimed(projectId, user, amount);
    }

    function _validateVesting(uint64 endTime, uint64[] calldata times, uint16[] calldata bps) private pure {
        if (times.length == 0 || times.length > MAX_VESTING_PERIODS || times.length != bps.length) {
            revert InvalidVestingSchedule();
        }
        uint256 totalBps;
        uint64 previous;
        for (uint256 i; i < times.length; ++i) {
            if (times[i] < endTime || (i != 0 && times[i] <= previous) || bps[i] == 0) {
                revert InvalidVestingSchedule();
            }
            previous = times[i];
            totalBps += bps[i];
        }
        if (totalBps != BPS_DENOMINATOR) revert InvalidVestingSchedule();
    }

    function _unlockedBps(uint256 projectId) private view returns (uint256 unlocked) {
        uint64[] storage times = _vestingTimes[projectId];
        uint16[] storage bps = _vestingBps[projectId];
        for (uint256 i; i < times.length; ++i) {
            if (block.timestamp < times[i]) break;
            unlocked += bps[i];
        }
    }

    function _pullExact(address token, address from, uint256 amount) private {
        IERC20 asset = IERC20(token);
        uint256 contractBefore = asset.balanceOf(address(this));
        uint256 senderBefore = asset.balanceOf(from);
        asset.safeTransferFrom(from, address(this), amount);
        uint256 observed = asset.balanceOf(address(this)) - contractBefore;
        uint256 senderDecrease = senderBefore - asset.balanceOf(from);
        if (observed != amount || senderDecrease != amount) {
            revert UnsupportedTokenBehavior(token, amount, observed);
        }
    }

    function _pushExact(address token, address recipient, uint256 amount) private {
        IERC20 asset = IERC20(token);
        uint256 senderBefore = asset.balanceOf(address(this));
        uint256 recipientBefore = asset.balanceOf(recipient);
        asset.safeTransfer(recipient, amount);
        uint256 senderDecrease = senderBefore - asset.balanceOf(address(this));
        uint256 recipientIncrease = asset.balanceOf(recipient) - recipientBefore;
        if (senderDecrease != amount || recipientIncrease != amount) {
            revert UnsupportedOutgoingTokenBehavior(token, amount, senderDecrease, recipientIncrease);
        }
    }

    function _assertSolvent(address token) private view {
        uint256 balance = IERC20(token).balanceOf(address(this));
        uint256 reserved = _reservedBalances[token];
        if (balance < reserved) revert TokenReserveInsolvent(token, balance, reserved);
    }

    function _requireProject(uint256 projectId) private view returns (Project storage project) {
        project = _projects[projectId];
        if (project.projectId == 0) revert ProjectDoesNotExist(projectId);
    }

    function _requireProjectView(uint256 projectId) private view returns (Project storage project) {
        project = _projects[projectId];
        if (project.projectId == 0) revert ProjectDoesNotExist(projectId);
    }

    function _requireStatus(Project storage project, ProjectStatus expected) private view {
        if (project.status != expected) revert InvalidProjectStatus(expected, project.status);
    }
}

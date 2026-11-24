// SPDX-License-Identifier: MIT
pragma solidity 0.8.22;

// AUDIT SPECIMEN — MUST NOT DEPLOY.
// This contract exists only to generate ABI, selector, topic, error and
// ERC-7201 storage-layout evidence for the Fair Launch candidate freeze.

import {Initializable} from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import {UUPSUpgradeable} from "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import {Ownable2StepUpgradeable} from "@openzeppelin/contracts-upgradeable/access/Ownable2StepUpgradeable.sol";
import {ReentrancyGuardUpgradeable} from "@openzeppelin/contracts-upgradeable/utils/ReentrancyGuardUpgradeable.sol";

contract JoinCareFairLaunchV1ArtifactBaseline is
    Initializable,
    UUPSUpgradeable,
    Ownable2StepUpgradeable,
    ReentrancyGuardUpgradeable
{
    uint16 private constant MAX_FEE_RATE = 1_000;
    bytes32 private constant FAIR_LAUNCH_STORAGE_LOCATION =
        0x7ebb4c309dc14c823b32233ea7cfb602ee106e1bc7e105581a751cd0bcb46700;

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

    enum GovernanceInvalidationClass {
        ProjectTokenNonObjectiveRisk,
        ProtocolAccountingRisk,
        LegalOrComplianceHold
    }

    struct PaymentTokenPolicyRevision {
        bool exists;
        uint8 decimals;
        uint32 policyVersion;
        bytes32 runtimeCodeHash;
        bytes32 policyDigest;
    }

    struct PaymentTokenAdmission {
        uint32 currentPolicyVersion;
        bool enabledForNewProjects;
        bool emergencyBlocked;
    }

    struct ProjectTokenPolicy {
        bool enabledForNewProjects;
        bool emergencyBlocked;
        uint8 decimals;
        uint32 policyVersion;
        bytes32 runtimeCodeHash;
        bytes32 policyDigest;
    }

    struct CreateProjectParams {
        uint256 projectId;
        address projectToken;
        address paymentToken;
        address projectOwner;
        address projectProceedsReceiver;
        bytes32 projectProceedsReceiverAttestationDigest;
        uint32 projectProceedsReceiverAttestationVersion;
        address expectedPlatformFeeReceiver;
        bytes32 expectedPlatformFeeReceiverAttestationDigest;
        uint32 expectedPlatformFeeReceiverAttestationVersion;
        uint16 expectedPlatformFeeBps;
        uint256 saleTokenAmount;
        uint256 softCap;
        uint256 minContribution;
        uint256 maxContribution;
        uint64 startTime;
        uint64 endTime;
        bytes32 expectedProjectTokenPolicyDigest;
        uint32 expectedProjectTokenPolicyVersion;
        bytes32 expectedPaymentTokenPolicyDigest;
        uint32 expectedPaymentTokenPolicyVersion;
        bytes32 expectedReleasePolicyDigest;
        uint32 expectedReleasePolicyVersion;
    }

    struct Project {
        uint256 projectId;
        address projectToken;
        address paymentToken;
        address projectOwner;
        address projectProceedsReceiver;
        bytes32 projectProceedsReceiverAttestationDigest;
        uint32 projectProceedsReceiverAttestationVersion;
        address platformFeeReceiver;
        bytes32 platformFeeReceiverAttestationDigest;
        uint32 platformFeeReceiverAttestationVersion;
        uint8 projectTokenDecimals;
        uint8 paymentTokenDecimals;
        uint16 platformFeeBps;
        ProjectStatus status;
        TerminalReason terminalReason;
        uint32 projectTokenPolicyVersion;
        uint32 paymentTokenPolicyVersion;
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
        uint256 settlementBlockNumber;
        uint256 settlementAnchorBlockNumber;
        bytes32 settlementAnchorBlockHash;
        bytes32 settlementTxHash;
        uint32 settlementTxLogOrdinal;
        bytes32 releaseManifestDigest;
        bytes32 projectTokenCodeHash;
        bytes32 projectTokenPolicyDigest;
        bytes32 paymentTokenCodeHash;
        bytes32 paymentTokenPolicyDigest;
        bytes32 releasePolicyDigest;
        uint32 releasePolicyVersion;
        bool projectProceedsWithdrawn;
        bool platformFeeWithdrawn;
        bool failedProjectTokensWithdrawn;
    }

    struct UserPosition {
        uint256 contributionAmount;
        uint256 claimedTokens;
        bool refunded;
    }

    /// @custom:storage-location erc7201:joincare.storage.JoinCareFairLaunchV1
    struct FairLaunchStorage {
        address operator;
        address pauseGuardian;
        address unpauseTimelock;
        address treasury;
        address platformFeeReceiver;
        uint16 platformFeeBps;
        bool contributionsPaused;
        bool payoutsPaused;
        uint64 claimsPauseUntil;
        mapping(address => PaymentTokenAdmission) paymentTokenAdmissions;
        mapping(address => mapping(uint32 => PaymentTokenPolicyRevision)) paymentTokenPolicyRevisions;
        mapping(address => ProjectTokenPolicy) projectTokenPolicies;
        mapping(address => uint256) reservedBalances;
        mapping(address => bool) approvedBeneficiarySafes;
        mapping(address => bytes32) beneficiarySafeAttestations;
        mapping(address => uint32) beneficiarySafeAttestationVersions;
        mapping(bytes32 => bool) releasePolicies;
        mapping(bytes32 => uint32) releasePolicyVersions;
        mapping(uint256 => Project) projects;
        mapping(uint256 => mapping(address => UserPosition)) positions;
        mapping(uint256 => mapping(address => bool)) hasParticipated;
        mapping(uint256 => uint64[]) vestingTimes;
        mapping(uint256 => uint16[]) vestingBps;
        uint256[32] __gap;
    }

    event FairLaunchProjectCreated(uint256 indexed projectId, address indexed projectToken, address indexed paymentToken, address projectOwner, address projectProceedsReceiver, bytes32 projectProceedsReceiverAttestationDigest, uint32 projectProceedsReceiverAttestationVersion, address platformFeeReceiver, bytes32 platformFeeReceiverAttestationDigest, uint32 platformFeeReceiverAttestationVersion, uint16 platformFeeBps, uint256 saleTokenAmount, uint256 softCap, uint256 minContribution, uint256 maxContribution, uint64 startTime, uint64 endTime, bytes32 projectTokenPolicyDigest, uint32 projectTokenPolicyVersion, bytes32 paymentTokenPolicyDigest, uint32 paymentTokenPolicyVersion, bytes32 releasePolicyDigest, uint32 releasePolicyVersion);
    event FairLaunchTokensDeposited(uint256 indexed projectId, address indexed projectToken, address indexed from, uint256 amount, uint256 totalDeposited);
    event FairLaunchActivated(uint256 indexed projectId);
    event FairLaunchContribution(uint256 indexed projectId, address indexed user, address indexed paymentToken, uint256 paymentAmount, uint256 userContribution, uint256 totalRaised);
    event FairLaunchEnded(uint256 indexed projectId, ProjectStatus status, TerminalReason terminalReason, address indexed paymentToken, uint256 finalTotalRaised, uint256 finalPriceNumerator, uint256 finalPriceDenominator, uint256 settlementBlockNumber, uint256 settlementAnchorBlockNumber, bytes32 settlementAnchorBlockHash);
    event FairLaunchSuccessfulReleased(uint256 indexed projectId, uint256 indexed settlementBlockNumber, address indexed attester, uint256 settlementAnchorBlockNumber, bytes32 settlementAnchorBlockHash, bytes32 settlementTxHash, uint32 settlementTxLogOrdinal, bytes32 releaseManifestDigest, bytes32 releasePolicyDigest, uint32 releasePolicyVersion);
    event FairLaunchPendingReleaseInvalidated(uint256 indexed projectId, TerminalReason terminalReason, GovernanceInvalidationClass invalidationClass, bytes32 reasonCode, bytes32 evidenceHash, address indexed caller);
    event FairLaunchPaymentTokenInvalidated(uint256 indexed projectId, address indexed paymentToken, address indexed timelock, bytes32 expectedRuntimeCodeHash, bytes32 observedRuntimeCodeHash, bytes32 expectedPolicyDigest, uint32 expectedPolicyVersion, bytes32 observedPolicyDigest, uint32 observedPolicyVersion, bool emergencyBlocked, bytes32 reasonCode, bytes32 evidenceHash);
    event FairLaunchCancelled(uint256 indexed projectId, ProjectStatus previousStatus, bytes32 reasonCode, bytes32 evidenceHash, address indexed operator);
    event FairLaunchRefunded(uint256 indexed projectId, address indexed user, address indexed paymentToken, uint256 amount, address caller);
    event FairLaunchClaimed(uint256 indexed projectId, address indexed user, address indexed projectToken, uint256 amount, uint256 totalUserClaimed, address caller);
    event FairLaunchProjectProceedsWithdrawn(uint256 indexed projectId, address indexed paymentToken, address indexed projectProceedsReceiver, uint256 amount, uint256 platformFeeReserved);
    event FairLaunchPlatformFeeWithdrawn(uint256 indexed projectId, address indexed paymentToken, address indexed platformFeeReceiver, uint256 amount);
    event FairLaunchFailedProjectTokensWithdrawn(uint256 indexed projectId, address indexed projectToken, address indexed projectOwner, uint256 amount);
    event FairLaunchContributionsPaused(address indexed guardian, bytes32 reasonCode, bytes32 evidenceHash);
    event FairLaunchContributionsUnpaused(address indexed timelock, bytes32 operationId);
    event FairLaunchPayoutsPaused(address indexed guardian, bytes32 reasonCode, bytes32 evidenceHash);
    event FairLaunchPayoutsUnpaused(address indexed timelock, bytes32 operationId);
    event FairLaunchClaimsPaused(address indexed guardian, uint64 pauseUntil, bytes32 reasonCode, bytes32 evidenceHash);
    event FairLaunchClaimsUnpaused(address indexed timelock, bytes32 operationId);
    event FairLaunchOperatorUpdated(address indexed previousOperator, address indexed newOperator);
    event FairLaunchPauseGuardianUpdated(address indexed previousGuardian, address indexed newGuardian);
    event FairLaunchUnpauseTimelockUpdated(address indexed previousTimelock, address indexed newTimelock);
    event FairLaunchTreasuryUpdated(address indexed previousTreasury, address indexed newTreasury);
    event FairLaunchPlatformFeeUpdated(uint16 previousFeeBps, uint16 newFeeBps, address indexed previousReceiver, address indexed newReceiver);
    event FairLaunchPaymentTokenPolicyRevisionRegistered(address indexed token, uint32 indexed policyVersion, uint8 decimals, bytes32 runtimeCodeHash, bytes32 policyDigest);
    event FairLaunchPaymentTokenAdmissionUpdated(address indexed token, uint32 indexed currentPolicyVersion, bool enabledForNewProjects);
    event FairLaunchPaymentTokenEmergencyBlockUpdated(address indexed token, bool emergencyBlocked);
    event FairLaunchProjectTokenConfigured(address indexed token, bool enabledForNewProjects, bool emergencyBlocked, uint8 decimals, uint32 policyVersion, bytes32 runtimeCodeHash, bytes32 policyDigest);
    event FairLaunchBeneficiarySafeConfigured(address indexed account, bool approved, bytes32 attestationDigest, uint32 attestationVersion);
    event FairLaunchReleasePolicyConfigured(bytes32 indexed policyDigest, uint32 policyVersion, bool enabled);
    event FairLaunchExcessTokensRecovered(address indexed token, address indexed treasury, uint256 amount);

    error InvalidAddress();
    error InvalidAmount();
    error InvalidFeeBps(uint16 actual, uint16 maximum);
    error InvalidTokenDecimals(uint8 decimals);
    error InvalidProjectParameters();
    error InvalidVestingSchedule();
    error ProjectAlreadyExists(uint256 projectId);
    error ProjectDoesNotExist(uint256 projectId);
    error InvalidProjectStatus(ProjectStatus expected, ProjectStatus actual);
    error UnauthorizedOperator(address caller);
    error UnauthorizedPauseGuardian(address caller);
    error UnauthorizedUnpauseTimelock(address caller);
    error UnauthorizedProjectOwner(address caller);
    error UnauthorizedBeneficiary(address caller, address expected);
    error BeneficiarySafeNotApproved(address account);
    error BeneficiarySafeAttestationMismatch(address account, bytes32 expectedDigest, uint32 expectedVersion, bytes32 actualDigest, uint32 actualVersion);
    error TokenNotConfigured(address token);
    error TokenNotAllowed(address token);
    error TokenEmergencyBlocked(address token);
    error TokenHasNoCode(address token);
    error TokenRuntimeMismatch(address token, bytes32 expected, bytes32 actual);
    error TokenDecimalsChanged(address token, uint8 expected, uint8 actual);
    error TokenPolicySnapshotChanged(address token, bytes32 expectedDigest, uint32 expectedVersion, bytes32 actualDigest, uint32 actualVersion);
    error PaymentTokenPolicyRevisionAlreadyExists(address token, uint32 policyVersion);
    error PaymentTokenPolicyRevisionDoesNotExist(address token, uint32 policyVersion);
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
    error ContributionsArePaused();
    error PayoutsArePaused();
    error ClaimsArePaused(uint64 pauseUntil);
    error InvalidClaimsPauseUntil(uint64 requestedUntil, uint64 maximumUntil);
    error CancellationWindowClosed(uint64 endTime);
    error InvalidReleaseEvidence();
    error SettlementAnchorMismatch(uint256 expectedBlockNumber, bytes32 expectedBlockHash, uint256 actualBlockNumber, bytes32 actualBlockHash);
    error ReleaseConfirmationDepthNotReached(uint256 requiredBlock, uint256 currentBlock);
    error InvalidReconciliationDigest();
    error ReleasePolicyNotEnabled(bytes32 policyDigest);
    error ReleasePolicySnapshotMismatch(bytes32 expectedDigest, uint32 expectedVersion, uint32 actualVersion);
    error ProjectTokenRuntimeStillMatches();
    error PaymentTokenNotInvalidatable();
    error PaymentTokenGovernanceInvalidationRequired();
    error InvalidGovernanceInvalidationClass(GovernanceInvalidationClass invalidationClass);
    error ExcessiveRecoveryRequested(address token, uint256 recoverable, uint256 requested);
    error OwnershipRenunciationDisabled();

    constructor() {
        _disableInitializers();
    }

    function initialize(address governanceTimelock, address initialOperator, address initialPauseGuardian, address initialUnpauseTimelock, address initialTreasury, address initialPlatformFeeReceiver, uint16 initialPlatformFeeBps) external initializer {
        if (governanceTimelock == address(0) || initialOperator == address(0) || initialPauseGuardian == address(0) || initialUnpauseTimelock == address(0) || initialTreasury == address(0) || initialPlatformFeeReceiver == address(0)) revert InvalidAddress();
        if (initialPlatformFeeBps > MAX_FEE_RATE) revert InvalidFeeBps(initialPlatformFeeBps, MAX_FEE_RATE);
        __Ownable_init(governanceTimelock);
        __Ownable2Step_init();
        __ReentrancyGuard_init();
        FairLaunchStorage storage $ = _getFairLaunchStorage();
        $.operator = initialOperator;
        $.pauseGuardian = initialPauseGuardian;
        $.unpauseTimelock = initialUnpauseTimelock;
        $.treasury = initialTreasury;
        $.platformFeeReceiver = initialPlatformFeeReceiver;
        $.platformFeeBps = initialPlatformFeeBps;
    }

    function createProject(CreateProjectParams calldata, uint64[] calldata, uint16[] calldata) external { _mechanicalOnly(); }
    function depositProjectTokens(uint256, uint256) external { _mechanicalOnly(); }
    function activateProject(uint256) external { _mechanicalOnly(); }
    function contribute(uint256, uint256) external { _mechanicalOnly(); }
    function endProject(uint256) external { _mechanicalOnly(); }
    function releaseSuccessfulProject(uint256, uint256, uint256, bytes32, bytes32, uint32, bytes32) external { _mechanicalOnly(); }
    function failPendingReleaseForProjectTokenRuntimeMismatch(uint256) external { _mechanicalOnly(); }
    function invalidatePendingRelease(uint256, GovernanceInvalidationClass, bytes32, bytes32) external { _mechanicalOnly(); }
    function invalidatePendingReleaseForPaymentToken(uint256, bytes32, bytes32) external { _mechanicalOnly(); }
    function cancelProject(uint256, bytes32, bytes32) external { _mechanicalOnly(); }
    function refund(uint256) external { _mechanicalOnly(); }
    function refundFor(uint256, address) external { _mechanicalOnly(); }
    function claim(uint256) external { _mechanicalOnly(); }
    function claimFor(uint256, address) external { _mechanicalOnly(); }
    function withdrawProjectProceeds(uint256) external { _mechanicalOnly(); }
    function withdrawPlatformFee(uint256) external { _mechanicalOnly(); }
    function withdrawFailedProjectTokens(uint256) external { _mechanicalOnly(); }
    function pauseContributions(bytes32, bytes32) external { _mechanicalOnly(); }
    function unpauseContributions(bytes32) external { _mechanicalOnly(); }
    function pausePayouts(bytes32, bytes32) external { _mechanicalOnly(); }
    function unpausePayouts(bytes32) external { _mechanicalOnly(); }
    function pauseClaims(uint64, bytes32, bytes32) external { _mechanicalOnly(); }
    function unpauseClaims(bytes32) external { _mechanicalOnly(); }
    function setOperator(address) external { _mechanicalOnly(); }
    function setPauseGuardian(address) external { _mechanicalOnly(); }
    function setUnpauseTimelock(address) external { _mechanicalOnly(); }
    function setTreasury(address) external { _mechanicalOnly(); }
    function setPlatformFee(uint16, address) external { _mechanicalOnly(); }
    function registerPaymentTokenPolicyRevision(address, PaymentTokenPolicyRevision calldata) external { _mechanicalOnly(); }
    function setPaymentTokenAdmission(address, uint32, bool) external { _mechanicalOnly(); }
    function setPaymentTokenEmergencyBlocked(address, bool) external { _mechanicalOnly(); }
    function configureProjectToken(address, ProjectTokenPolicy calldata) external { _mechanicalOnly(); }
    function configureBeneficiarySafe(address, bool, bytes32, uint32) external { _mechanicalOnly(); }
    function configureReleasePolicy(bytes32, uint32, bool) external { _mechanicalOnly(); }
    function recoverExcessTokens(address, uint256) external { _mechanicalOnly(); }

    function projectExists(uint256 projectId) external view returns (bool) { return _getFairLaunchStorage().projects[projectId].projectId == projectId && projectId != 0; }
    function getProject(uint256 projectId) external view returns (Project memory) { return _getFairLaunchStorage().projects[projectId]; }
    function getPosition(uint256 projectId, address user) external view returns (UserPosition memory) { return _getFairLaunchStorage().positions[projectId][user]; }
    function getVestingSchedule(uint256 projectId) external view returns (uint64[] memory times, uint16[] memory bps) { FairLaunchStorage storage $ = _getFairLaunchStorage(); return ($.vestingTimes[projectId], $.vestingBps[projectId]); }
    function calculateFinalAllocation(uint256, address) public view returns (uint256) { return 0; }
    function calculateClaimable(uint256, address) public view returns (uint256) { return 0; }
    function isClaimsPaused() public view returns (bool) { return block.timestamp < _getFairLaunchStorage().claimsPauseUntil; }
    function projectTokenRuntimeMatches(uint256) external view returns (bool) { return false; }
    function paymentTokenRuntimeMatches(uint256) external view returns (bool) { return false; }
    function contractVersion() external pure returns (bytes32) { return keccak256("fair_launch_v1"); }
    function operator() external view returns (address) { return _getFairLaunchStorage().operator; }
    function pauseGuardian() external view returns (address) { return _getFairLaunchStorage().pauseGuardian; }
    function unpauseTimelock() external view returns (address) { return _getFairLaunchStorage().unpauseTimelock; }
    function treasury() external view returns (address) { return _getFairLaunchStorage().treasury; }
    function platformFeeConfig() external view returns (uint16 feeBps, address receiver) { FairLaunchStorage storage $ = _getFairLaunchStorage(); return ($.platformFeeBps, $.platformFeeReceiver); }
    function paymentTokenAdmission(address token) external view returns (PaymentTokenAdmission memory) { return _getFairLaunchStorage().paymentTokenAdmissions[token]; }
    function paymentTokenPolicyRevision(address token, uint32 policyVersion) external view returns (PaymentTokenPolicyRevision memory) { return _getFairLaunchStorage().paymentTokenPolicyRevisions[token][policyVersion]; }
    function projectTokenPolicy(address token) external view returns (ProjectTokenPolicy memory) { return _getFairLaunchStorage().projectTokenPolicies[token]; }
    function reservedBalance(address token) external view returns (uint256) { return _getFairLaunchStorage().reservedBalances[token]; }
    function beneficiarySafeConfig(address account) external view returns (bool approved, bytes32 attestationDigest, uint32 attestationVersion) { FairLaunchStorage storage $ = _getFairLaunchStorage(); return ($.approvedBeneficiarySafes[account], $.beneficiarySafeAttestations[account], $.beneficiarySafeAttestationVersions[account]); }
    function releasePolicyConfig(bytes32 policyDigest) external view returns (bool enabled, uint32 policyVersion) { FairLaunchStorage storage $ = _getFairLaunchStorage(); return ($.releasePolicies[policyDigest], $.releasePolicyVersions[policyDigest]); }

    function renounceOwnership() public pure override { revert OwnershipRenunciationDisabled(); }

    function _authorizeUpgrade(address) internal override onlyOwner {}

    function _mechanicalOnly() private pure { assembly { revert(0, 0) } }

    function _getFairLaunchStorage() private pure returns (FairLaunchStorage storage $) {
        assembly { $.slot := FAIR_LAUNCH_STORAGE_LOCATION }
    }
}

// SPDX-License-Identifier: MIT
pragma solidity 0.8.22;
import {Initializable} from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import {Ownable2StepUpgradeable} from "@openzeppelin/contracts-upgradeable/access/Ownable2StepUpgradeable.sol";
import {ReentrancyGuardUpgradeable} from "@openzeppelin/contracts-upgradeable/utils/ReentrancyGuardUpgradeable.sol";
import {IERC20Metadata} from "@openzeppelin/contracts/token/ERC20/extensions/IERC20Metadata.sol";
import {JoinCareFairLaunchV1Logic as Logic} from "../libraries/JoinCareFairLaunchV1Logic.sol";
import {JoinCareFairLaunchV1Runtime as Runtime} from "../libraries/JoinCareFairLaunchV1Runtime.sol";

/// @custom:oz-upgrades-unsafe-allow external-library-linking
abstract contract JoinCareFairLaunchV1Shared is
 Initializable, Ownable2StepUpgradeable, ReentrancyGuardUpgradeable {
    uint16 internal constant MAX_FEE_RATE = 1_000;
    bytes32 internal constant FAIR_LAUNCH_STORAGE_LOCATION = 0x7ebb4c309dc14c823b32233ea7cfb602ee106e1bc7e105581a751cd0bcb46700;
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

    modifier onlyOperator() { FairLaunchStorage storage $ = _getFairLaunchStorage(); if (msg.sender != $.operator) revert UnauthorizedOperator(msg.sender); _; }
    modifier onlyPauseGuardian() { FairLaunchStorage storage $ = _getFairLaunchStorage(); if (msg.sender != $.pauseGuardian) revert UnauthorizedPauseGuardian(msg.sender); _; }
    modifier onlyUnpauseTimelock() { FairLaunchStorage storage $ = _getFairLaunchStorage(); if (msg.sender != $.unpauseTimelock) revert UnauthorizedUnpauseTimelock(msg.sender); _; }

    function _requireContributionsNotPaused(FairLaunchStorage storage $) internal view { if ($.contributionsPaused) revert ContributionsArePaused(); }

    function _refund(uint256 id,address user) internal {FairLaunchStorage storage $=_getFairLaunchStorage();Project storage p=_project($,id);if(p.status!=ProjectStatus.Failed&&p.status!=ProjectStatus.Cancelled)revert InvalidProjectStatus(ProjectStatus.Failed,p.status);UserPosition storage pos=$.positions[id][user];if(pos.contributionAmount==0)revert NoContribution(user);if(pos.refunded)revert AlreadyRefunded(user);uint256 amount=pos.contributionAmount;pos.refunded=true;$.reservedBalances[p.paymentToken]-=amount;_out($,p.paymentToken,user,amount);emit FairLaunchRefunded(id,user,p.paymentToken,amount,msg.sender);}
    function _claim(uint256 id,address user) internal {FairLaunchStorage storage $=_getFairLaunchStorage();Project storage p=_project($,id);_status(p,ProjectStatus.Successful);if(block.timestamp<$.claimsPauseUntil)revert ClaimsArePaused($.claimsPauseUntil);_checkProjectRuntime($,p);uint256 unlocked;uint64[] storage times=$.vestingTimes[id];uint16[] storage bps=$.vestingBps[id];for(uint256 i;i<times.length;++i){if(block.timestamp<times[i])break;unlocked+=bps[i];}uint256 allocation=Logic.allocation($.positions[id][user].contributionAmount,p.saleTokenAmount,p.finalTotalRaised);uint256 vested=Logic.proportional(allocation,unlocked);uint256 claimed=$.positions[id][user].claimedTokens;uint256 amount=vested<=claimed?0:vested-claimed;if(amount==0)revert NoTokensToClaim(user);UserPosition storage pos=$.positions[id][user];pos.claimedTokens+=amount;p.totalTokensClaimed+=amount;$.reservedBalances[p.projectToken]-=amount;_out($,p.projectToken,user,amount);emit FairLaunchClaimed(id,user,p.projectToken,amount,pos.claimedTokens,msg.sender);}
    function _project(FairLaunchStorage storage $,uint256 id) internal view returns(Project storage p){p=$.projects[id];if(id==0||p.projectId!=id)revert ProjectDoesNotExist(id);}
    function _status(Project storage p,ProjectStatus expected) internal view {if(p.status!=expected)revert InvalidProjectStatus(expected,p.status);}
    function _checkBeneficiary(FairLaunchStorage storage $,address account,bytes32 digest,uint32 version) internal view {if(!$.approvedBeneficiarySafes[account])revert BeneficiarySafeNotApproved(account);bytes32 actual=$.beneficiarySafeAttestations[account];uint32 actualVersion=$.beneficiarySafeAttestationVersions[account];if(actual!=digest||actualVersion!=version)revert BeneficiarySafeAttestationMismatch(account,digest,version,actual,actualVersion);}
    function _checkProjectRuntime(FairLaunchStorage storage $ ,Project storage p) internal view {ProjectTokenPolicy memory policy=$.projectTokenPolicies[p.projectToken];if(policy.emergencyBlocked)revert TokenEmergencyBlocked(p.projectToken);if(policy.policyDigest!=p.projectTokenPolicyDigest||policy.policyVersion!=p.projectTokenPolicyVersion)revert TokenPolicySnapshotChanged(p.projectToken,p.projectTokenPolicyDigest,p.projectTokenPolicyVersion,policy.policyDigest,policy.policyVersion);Runtime.validateRuntime(p.projectToken,p.projectTokenCodeHash,p.projectTokenDecimals);}
    function _checkPaymentRuntime(FairLaunchStorage storage $,Project storage p) internal view {PaymentTokenAdmission memory admission=$.paymentTokenAdmissions[p.paymentToken];if(admission.emergencyBlocked)revert TokenEmergencyBlocked(p.paymentToken);PaymentTokenPolicyRevision memory policy=$.paymentTokenPolicyRevisions[p.paymentToken][p.paymentTokenPolicyVersion];if(!policy.exists||policy.policyDigest!=p.paymentTokenPolicyDigest||policy.policyVersion!=p.paymentTokenPolicyVersion)revert TokenPolicySnapshotChanged(p.paymentToken,p.paymentTokenPolicyDigest,p.paymentTokenPolicyVersion,policy.policyDigest,policy.policyVersion);Runtime.validateRuntime(p.paymentToken,p.paymentTokenCodeHash,p.paymentTokenDecimals);}
    function _projectRuntimeMatches(FairLaunchStorage storage $,Project storage p) internal view returns(bool){ProjectTokenPolicy memory policy=$.projectTokenPolicies[p.projectToken];return !policy.emergencyBlocked&&policy.policyDigest==p.projectTokenPolicyDigest&&policy.policyVersion==p.projectTokenPolicyVersion&&Runtime.runtimeMatches(p.projectToken,p.projectTokenCodeHash,p.projectTokenDecimals);}
    function _paymentRuntimeMatches(FairLaunchStorage storage $,Project storage p) internal view returns(bool){PaymentTokenAdmission memory a=$.paymentTokenAdmissions[p.paymentToken];PaymentTokenPolicyRevision memory policy=$.paymentTokenPolicyRevisions[p.paymentToken][p.paymentTokenPolicyVersion];return !a.emergencyBlocked&&policy.exists&&policy.policyDigest==p.paymentTokenPolicyDigest&&policy.policyVersion==p.paymentTokenPolicyVersion&&Runtime.runtimeMatches(p.paymentToken,p.paymentTokenCodeHash,p.paymentTokenDecimals);}
    function _in(address token,uint256 amount) internal {uint256 beforeBalance=Runtime.balance(token,address(this));_callToken(token,abi.encodeWithSelector(bytes4(keccak256("transferFrom(address,address,uint256)")),msg.sender,address(this),amount));uint256 observed=Runtime.balance(token,address(this))-beforeBalance;Logic.validateIncoming(token,amount,observed);}
    function _out(FairLaunchStorage storage $,address token,address recipient,uint256 amount) internal { _solvent($,token);uint256 senderBefore=Runtime.balance(token,address(this));uint256 recipientBefore=Runtime.balance(token,recipient);_callToken(token,abi.encodeWithSelector(bytes4(keccak256("transfer(address,uint256)")),recipient,amount));uint256 senderAfter=Runtime.balance(token,address(this));uint256 recipientAfter=Runtime.balance(token,recipient);uint256 decrease=senderBefore>=senderAfter?senderBefore-senderAfter:0;uint256 increase=recipientAfter>=recipientBefore?recipientAfter-recipientBefore:0;Logic.validateOutgoing(token,amount,decrease,increase);_solvent($,token);}
    function _solvent(FairLaunchStorage storage $,address token) internal view {Runtime.validateSolvent(token,address(this),$.reservedBalances[token]);}
    function _callToken(address token,bytes memory data) internal { (bool ok,bytes memory result)=token.call(data);if(!ok||(result.length!=0&&!abi.decode(result,(bool))))revert UnsupportedTokenBehavior(token,0,0);}
    function _getFairLaunchStorage() internal pure returns (FairLaunchStorage storage $) { assembly { $.slot := FAIR_LAUNCH_STORAGE_LOCATION } }
}

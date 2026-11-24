// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

import "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/access/Ownable2StepUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/extensions/IERC20Metadata.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/utils/math/Math.sol";
import "./utils/ReentrancyGuardUpgradeable.sol";

/**
 * @title JoinCareIDOv2
 * @notice Project-scoped USDT/USDC settlement IDO deployed behind a fresh UUPS proxy.
 * @dev V2 intentionally has no V1/JCC storage fallback. All accounting uses token raw units.
 */
contract JoinCareIDOv2 is
    Initializable,
    UUPSUpgradeable,
    Ownable2StepUpgradeable,
    PausableUpgradeable,
    ReentrancyGuardUpgradeable
{
    using SafeERC20 for IERC20;

    enum ProjectStatus {
        Pending,
        Active,
        Successful,
        Failed,
        Cancelled
    }

    struct PaymentTokenConfig {
        bool allowed;
        uint8 decimals;
    }

    struct Project {
        uint256 projectId;
        address projectToken;
        address paymentToken;
        address projectOwner;
        address feeReceiver;
        uint8 projectTokenDecimals;
        uint8 paymentTokenDecimals;
        uint16 successFeeRate;
        ProjectStatus status;
        uint256 tokenPrice;
        uint256 hardCap;
        uint256 softCap;
        uint256 maxPerUser;
        uint256 startTime;
        uint256 endTime;
        uint256 totalRaised;
        uint256 totalTokensSold;
        uint256 totalTokensDeposited;
        bool fundsWithdrawn;
        bool platformFeeWithdrawn;
        bool projectTokensWithdrawn;
        uint256[] vestingTimes;
        uint256[] vestingPercents;
    }

    struct UserPurchase {
        uint256 paymentAmount;
        uint256 tokensBought;
        uint256 claimedTokens;
        bool refunded;
    }

    uint256 public constant BASIS_POINTS = 10_000;
    uint256 public constant MAX_FEE_RATE = 1_000;
    uint8 public constant MAX_SUPPORTED_DECIMALS = 18;

    address public feeReceiver;
    uint16 public successFeeRate;

    mapping(address => PaymentTokenConfig) public paymentTokenConfigs;
    mapping(address => PaymentTokenConfig) public projectTokenConfigs;
    mapping(address => uint256) public reservedBalances;
    mapping(uint256 => Project) private _projects;
    mapping(uint256 => mapping(address => UserPurchase)) private _userPurchases;
    mapping(uint256 => address[]) private _projectParticipants;
    mapping(uint256 => mapping(address => bool)) public hasParticipated;
    address public operator;

    event PaymentTokenConfigured(address indexed token, bool allowed, uint8 decimals);
    event ProjectTokenConfigured(address indexed token, bool allowed, uint8 decimals);
    event PlatformFeesUpdated(uint16 successFeeRate, address indexed feeReceiver);
    event OperatorUpdated(address indexed previousOperator, address indexed newOperator);
    event ProjectCreated(
        uint256 indexed projectId,
        address indexed projectToken,
        address indexed paymentToken,
        address projectOwner,
        address feeReceiver,
        uint8 projectTokenDecimals,
        uint8 paymentTokenDecimals,
        uint16 successFeeRate,
        uint256 tokenPrice,
        uint256 hardCap,
        uint256 softCap,
        uint256 maxPerUser,
        uint256 startTime,
        uint256 endTime
    );
    event ProjectTokensDeposited(uint256 indexed projectId, address indexed projectToken, uint256 amount);
    event ProjectActivated(uint256 indexed projectId);
    event Purchased(
        uint256 indexed projectId,
        address indexed user,
        address indexed paymentToken,
        uint256 paymentAmount,
        address projectToken,
        uint256 projectTokenAmount
    );
    event ProjectEnded(
        uint256 indexed projectId,
        ProjectStatus status,
        address indexed paymentToken,
        uint256 totalRaised
    );
    event ProjectCancelled(uint256 indexed projectId, address indexed paymentToken, uint256 totalRaised);
    event Refunded(
        uint256 indexed projectId,
        address indexed user,
        address indexed paymentToken,
        uint256 paymentAmount
    );
    event Claimed(
        uint256 indexed projectId,
        address indexed user,
        address indexed projectToken,
        uint256 projectTokenAmount
    );
    event ProjectProceedsWithdrawn(
        uint256 indexed projectId,
        address indexed paymentToken,
        address indexed projectOwner,
        uint256 projectOwnerAmount,
        uint256 platformFeeReserved
    );
    event PlatformFeeWithdrawn(
        uint256 indexed projectId,
        address indexed paymentToken,
        address indexed feeReceiver,
        uint256 platformFee
    );
    event ProjectTokensWithdrawn(
        uint256 indexed projectId,
        address indexed projectToken,
        address indexed projectOwner,
        uint256 amount
    );
    event ExcessTokensRecovered(address indexed token, address indexed recipient, uint256 amount);

    error InvalidAddress();
    error InvalidAmount();
    error InvalidFeeRate();
    error InvalidTokenDecimals(uint8 decimals);
    error TokenDecimalsChanged(address token, uint8 configured, uint8 current);
    error TokenHasNoCode(address token);
    error PaymentTokenNotConfigured(address token);
    error PaymentTokenNotAllowed(address token);
    error ProjectTokenNotConfigured(address token);
    error ProjectTokenNotAllowed(address token);
    error ProjectAlreadyExists(uint256 projectId);
    error ProjectDoesNotExist(uint256 projectId);
    error InvalidProjectStatus(ProjectStatus actual);
    error InvalidProjectParameters();
    error FeeSnapshotChanged(address expectedReceiver, address actualReceiver, uint16 expectedRate, uint16 actualRate);
    error InvalidVestingSchedule();
    error UnauthorizedProjectOwner();
    error ProjectNotStarted();
    error ProjectEndedAlready();
    error ExceedsHardCap();
    error ExceedsMaxPerUser();
    error InsufficientProjectTokens();
    error TokenAmountTooSmall();
    error UnsupportedTokenBehavior(address token, uint256 requested, uint256 received);
    error UnsupportedOutgoingTokenBehavior(
        address token,
        uint256 requested,
        uint256 senderDecrease,
        uint256 recipientIncrease
    );
    error TokenReserveInsolvent(address token, uint256 balance, uint256 reserved);
    error OwnershipRenunciationDisabled();
    error NoPurchase();
    error AlreadyRefunded();
    error AlreadyWithdrawn();
    error NoTokensToClaim();
    error UnauthorizedOperator();
    error ExcessiveRecoveryRequested(address token, uint256 excess);

    /// @custom:oz-upgrades-unsafe-allow constructor
    constructor() {
        _disableInitializers();
    }

    function initialize(address initialOwner, address initialOperator, address initialFeeReceiver, uint16 initialFeeRate)
        external
        initializer
    {
        if (
            initialOwner == address(0) || initialOwner == address(this) ||
            initialOperator == address(0) || initialOperator == address(this) ||
            initialFeeReceiver == address(0) || initialFeeReceiver == address(this)
        ) revert InvalidAddress();
        if (initialFeeRate > MAX_FEE_RATE) revert InvalidFeeRate();

        __Ownable_init(initialOwner);
        __Ownable2Step_init();
        __Pausable_init();
        __ReentrancyGuard_init();
        feeReceiver = initialFeeReceiver;
        successFeeRate = initialFeeRate;
        operator = initialOperator;
    }

    function _authorizeUpgrade(address) internal override onlyOwner {}

    function renounceOwnership() public view override onlyOwner {
        revert OwnershipRenunciationDisabled();
    }

    function pause() external onlyOwner {
        _pause();
    }

    function unpause() external onlyOwner {
        _unpause();
    }

    function setPlatformFees(uint16 newFeeRate, address newFeeReceiver) external onlyOwner {
        if (newFeeReceiver == address(0) || newFeeReceiver == address(this)) revert InvalidAddress();
        if (newFeeRate > MAX_FEE_RATE) revert InvalidFeeRate();
        successFeeRate = newFeeRate;
        feeReceiver = newFeeReceiver;
        emit PlatformFeesUpdated(newFeeRate, newFeeReceiver);
    }

    function setOperator(address newOperator) external onlyOwner {
        if (newOperator == address(0) || newOperator == address(this)) revert InvalidAddress();
        address previousOperator = operator;
        operator = newOperator;
        emit OperatorUpdated(previousOperator, newOperator);
    }

    function configurePaymentToken(address token, bool allowed) external onlyOwner {
        if (token == address(0)) revert InvalidAddress();
        if (!allowed) {
            PaymentTokenConfig storage existingConfig = paymentTokenConfigs[token];
            if (existingConfig.decimals == 0) revert PaymentTokenNotConfigured(token);
            existingConfig.allowed = false;
            emit PaymentTokenConfigured(token, false, existingConfig.decimals);
            return;
        }

        if (token.code.length == 0) revert TokenHasNoCode(token);
        uint8 tokenDecimals = _readSupportedDecimals(token);
        uint8 configuredDecimals = paymentTokenConfigs[token].decimals;
        if (configuredDecimals != 0 && configuredDecimals != tokenDecimals) {
            revert TokenDecimalsChanged(token, configuredDecimals, tokenDecimals);
        }
        paymentTokenConfigs[token] = PaymentTokenConfig({allowed: true, decimals: tokenDecimals});
        emit PaymentTokenConfigured(token, true, tokenDecimals);
    }

    function configureProjectToken(address token, bool allowed) external onlyOwner {
        if (token == address(0)) revert InvalidAddress();
        if (!allowed) {
            PaymentTokenConfig storage existingConfig = projectTokenConfigs[token];
            if (existingConfig.decimals == 0) revert ProjectTokenNotConfigured(token);
            existingConfig.allowed = false;
            emit ProjectTokenConfigured(token, false, existingConfig.decimals);
            return;
        }

        if (token.code.length == 0) revert TokenHasNoCode(token);
        uint8 tokenDecimals = _readSupportedDecimals(token);
        uint8 configuredDecimals = projectTokenConfigs[token].decimals;
        if (configuredDecimals != 0 && configuredDecimals != tokenDecimals) {
            revert TokenDecimalsChanged(token, configuredDecimals, tokenDecimals);
        }
        projectTokenConfigs[token] = PaymentTokenConfig({allowed: true, decimals: tokenDecimals});
        emit ProjectTokenConfigured(token, true, tokenDecimals);
    }

    function createProject(
        uint256 projectId,
        address projectToken,
        address paymentToken,
        address projectOwner,
        address expectedFeeReceiver,
        uint16 expectedSuccessFeeRate,
        uint256 tokenPrice,
        uint256 hardCap,
        uint256 softCap,
        uint256 maxPerUser,
        uint256 startTime,
        uint256 endTime,
        uint256[] calldata vestingTimes,
        uint256[] calldata vestingPercents
    ) external whenNotPaused {
        _requireOperatorOrOwner();
        if (
            projectId == 0 || projectToken == address(0) ||
            projectOwner == address(0) || projectOwner == address(this)
        ) {
            revert InvalidAddress();
        }
        if (_projects[projectId].projectToken != address(0)) revert ProjectAlreadyExists(projectId);
        if (expectedFeeReceiver != feeReceiver || expectedSuccessFeeRate != successFeeRate) {
            revert FeeSnapshotChanged(expectedFeeReceiver, feeReceiver, expectedSuccessFeeRate, successFeeRate);
        }
        PaymentTokenConfig memory paymentConfig = paymentTokenConfigs[paymentToken];
        if (!paymentConfig.allowed) revert PaymentTokenNotAllowed(paymentToken);
        PaymentTokenConfig memory projectConfig = projectTokenConfigs[projectToken];
        if (!projectConfig.allowed) revert ProjectTokenNotAllowed(projectToken);
        _requireRuntimeDecimals(paymentToken, paymentConfig.decimals);
        _requireRuntimeDecimals(projectToken, projectConfig.decimals);
        if (projectToken == paymentToken) revert InvalidProjectParameters();
        if (
            tokenPrice == 0 || hardCap == 0 || softCap == 0 || softCap > hardCap ||
            maxPerUser == 0 || maxPerUser > hardCap || startTime <= block.timestamp || endTime <= startTime
        ) revert InvalidProjectParameters();

        _validateVesting(endTime, vestingTimes, vestingPercents);

        Project storage project = _projects[projectId];
        project.projectId = projectId;
        project.projectToken = projectToken;
        project.paymentToken = paymentToken;
        project.projectOwner = projectOwner;
        project.feeReceiver = feeReceiver;
        project.projectTokenDecimals = projectConfig.decimals;
        project.paymentTokenDecimals = paymentConfig.decimals;
        project.successFeeRate = successFeeRate;
        project.status = ProjectStatus.Pending;
        project.tokenPrice = tokenPrice;
        project.hardCap = hardCap;
        project.softCap = softCap;
        project.maxPerUser = maxPerUser;
        project.startTime = startTime;
        project.endTime = endTime;
        project.vestingTimes = vestingTimes;
        project.vestingPercents = vestingPercents;

        emit ProjectCreated(
            projectId,
            projectToken,
            paymentToken,
            projectOwner,
            project.feeReceiver,
            projectConfig.decimals,
            paymentConfig.decimals,
            project.successFeeRate,
            tokenPrice,
            hardCap,
            softCap,
            maxPerUser,
            startTime,
            endTime
        );
    }

    function depositProjectTokens(uint256 projectId, uint256 amount)
        external
        nonReentrant
        whenNotPaused
    {
        Project storage project = _requireProject(projectId);
        if (project.status != ProjectStatus.Pending) revert InvalidProjectStatus(project.status);
        if (!projectTokenConfigs[project.projectToken].allowed) {
            revert ProjectTokenNotAllowed(project.projectToken);
        }
        _requireRuntimeDecimals(project.projectToken, projectTokenConfigs[project.projectToken].decimals);
        if (msg.sender != project.projectOwner) revert UnauthorizedProjectOwner();
        if (amount == 0) revert InvalidAmount();

        uint256 received = _transferInExact(project.projectToken, msg.sender, amount);
        project.totalTokensDeposited += received;
        _increaseReserve(project.projectToken, received);
        emit ProjectTokensDeposited(projectId, project.projectToken, received);
    }

    function activateProject(uint256 projectId) external whenNotPaused {
        _requireOperatorOrOwner();
        Project storage project = _requireProject(projectId);
        if (project.status != ProjectStatus.Pending) revert InvalidProjectStatus(project.status);
        if (!projectTokenConfigs[project.projectToken].allowed) {
            revert ProjectTokenNotAllowed(project.projectToken);
        }
        _requireRuntimeDecimals(project.projectToken, projectTokenConfigs[project.projectToken].decimals);
        uint256 softCapTokenAmount = Math.mulDiv(
            project.softCap,
            10 ** uint256(project.projectTokenDecimals),
            project.tokenPrice
        );
        if (project.totalTokensDeposited < softCapTokenAmount) revert InsufficientProjectTokens();
        project.status = ProjectStatus.Active;
        emit ProjectActivated(projectId);
    }

    function purchase(uint256 projectId, uint256 paymentAmount)
        external
        nonReentrant
        whenNotPaused
    {
        Project storage project = _requireProject(projectId);
        if (project.status != ProjectStatus.Active) revert InvalidProjectStatus(project.status);
        if (!paymentTokenConfigs[project.paymentToken].allowed) {
            revert PaymentTokenNotAllowed(project.paymentToken);
        }
        if (!projectTokenConfigs[project.projectToken].allowed) {
            revert ProjectTokenNotAllowed(project.projectToken);
        }
        _requireRuntimeDecimals(project.paymentToken, paymentTokenConfigs[project.paymentToken].decimals);
        _requireRuntimeDecimals(project.projectToken, projectTokenConfigs[project.projectToken].decimals);
        if (block.timestamp < project.startTime) revert ProjectNotStarted();
        if (block.timestamp >= project.endTime) revert ProjectEndedAlready();
        if (paymentAmount == 0) revert InvalidAmount();
        if (project.totalRaised + paymentAmount > project.hardCap) revert ExceedsHardCap();

        UserPurchase storage purchaseRecord = _userPurchases[projectId][msg.sender];
        if (purchaseRecord.paymentAmount + paymentAmount > project.maxPerUser) revert ExceedsMaxPerUser();

        uint256 projectTokenAmount = Math.mulDiv(
            paymentAmount,
            10 ** uint256(project.projectTokenDecimals),
            project.tokenPrice
        );
        if (projectTokenAmount == 0) revert TokenAmountTooSmall();
        if (project.totalTokensSold + projectTokenAmount > project.totalTokensDeposited) {
            revert InsufficientProjectTokens();
        }

        _transferInExact(project.paymentToken, msg.sender, paymentAmount);
        _increaseReserve(project.paymentToken, paymentAmount);
        project.totalRaised += paymentAmount;
        project.totalTokensSold += projectTokenAmount;
        purchaseRecord.paymentAmount += paymentAmount;
        purchaseRecord.tokensBought += projectTokenAmount;

        if (!hasParticipated[projectId][msg.sender]) {
            hasParticipated[projectId][msg.sender] = true;
            _projectParticipants[projectId].push(msg.sender);
        }

        emit Purchased(
            projectId,
            msg.sender,
            project.paymentToken,
            paymentAmount,
            project.projectToken,
            projectTokenAmount
        );
        if (project.totalRaised == project.hardCap) _finalizeProject(project);
    }

    function endProject(uint256 projectId) external {
        Project storage project = _requireProject(projectId);
        if (project.status != ProjectStatus.Active) revert InvalidProjectStatus(project.status);
        if (block.timestamp < project.endTime) revert ProjectNotStarted();
        _finalizeProject(project);
    }

    function cancelProject(uint256 projectId) external {
        Project storage project = _requireProject(projectId);
        if (project.status != ProjectStatus.Pending && project.status != ProjectStatus.Active) {
            revert InvalidProjectStatus(project.status);
        }
        if (project.status == ProjectStatus.Pending) {
            if (msg.sender != owner() && msg.sender != project.projectOwner) revert UnauthorizedProjectOwner();
        } else if (msg.sender != owner()) {
            revert OwnableUnauthorizedAccount(msg.sender);
        }
        project.status = ProjectStatus.Cancelled;
        emit ProjectCancelled(projectId, project.paymentToken, project.totalRaised);
    }

    function refund(uint256 projectId) external nonReentrant {
        _refund(projectId, msg.sender);
    }

    function batchRefund(uint256 projectId, address[] calldata users)
        external
        onlyOwner
        nonReentrant
    {
        for (uint256 i = 0; i < users.length; ++i) {
            UserPurchase storage purchaseRecord = _userPurchases[projectId][users[i]];
            if (purchaseRecord.paymentAmount > 0 && !purchaseRecord.refunded) {
                _refund(projectId, users[i]);
            }
        }
    }

    function withdrawRaisedFunds(uint256 projectId) external nonReentrant {
        Project storage project = _requireProject(projectId);
        if (project.status != ProjectStatus.Successful) revert InvalidProjectStatus(project.status);
        if (msg.sender != project.projectOwner) revert UnauthorizedProjectOwner();
        if (project.fundsWithdrawn) revert AlreadyWithdrawn();
        project.fundsWithdrawn = true;

        uint256 platformFee = Math.mulDiv(project.totalRaised, project.successFeeRate, BASIS_POINTS);
        uint256 projectOwnerAmount = project.totalRaised - platformFee;
        _transferOutExact(project.paymentToken, project.projectOwner, projectOwnerAmount);

        emit ProjectProceedsWithdrawn(
            projectId,
            project.paymentToken,
            project.projectOwner,
            projectOwnerAmount,
            platformFee
        );
    }

    function withdrawPlatformFee(uint256 projectId) external nonReentrant {
        Project storage project = _requireProject(projectId);
        if (project.status != ProjectStatus.Successful) revert InvalidProjectStatus(project.status);
        if (project.platformFeeWithdrawn) revert AlreadyWithdrawn();
        project.platformFeeWithdrawn = true;

        uint256 platformFee = Math.mulDiv(project.totalRaised, project.successFeeRate, BASIS_POINTS);
        if (platformFee > 0) _transferOutExact(project.paymentToken, project.feeReceiver, platformFee);
        emit PlatformFeeWithdrawn(projectId, project.paymentToken, project.feeReceiver, platformFee);
    }

    function withdrawProjectTokens(uint256 projectId) external nonReentrant {
        Project storage project = _requireProject(projectId);
        if (
            project.status != ProjectStatus.Successful && project.status != ProjectStatus.Failed &&
            project.status != ProjectStatus.Cancelled
        ) revert InvalidProjectStatus(project.status);
        if (msg.sender != project.projectOwner) revert UnauthorizedProjectOwner();
        if (project.projectTokensWithdrawn) revert AlreadyWithdrawn();
        project.projectTokensWithdrawn = true;

        uint256 amount = project.status == ProjectStatus.Successful
            ? project.totalTokensDeposited - project.totalTokensSold
            : project.totalTokensDeposited;
        if (amount > 0) _transferOutExact(project.projectToken, project.projectOwner, amount);
        emit ProjectTokensWithdrawn(projectId, project.projectToken, project.projectOwner, amount);
    }

    function recoverExcessTokens(address tokenAddress, address recipient, uint256 amount)
        external
        onlyOwner
        nonReentrant
    {
        if (tokenAddress == address(0) || recipient == address(0) || recipient == address(this)) {
            revert InvalidAddress();
        }
        if (amount == 0) revert InvalidAmount();
        uint256 balance = IERC20(tokenAddress).balanceOf(address(this));
        uint256 reserve = reservedBalances[tokenAddress];
        uint256 excess = balance > reserve ? balance - reserve : 0;
        if (amount > excess) revert ExcessiveRecoveryRequested(tokenAddress, excess);
        IERC20(tokenAddress).safeTransfer(recipient, amount);
        emit ExcessTokensRecovered(tokenAddress, recipient, amount);
    }

    function calculateClaimable(uint256 projectId, address user) public view returns (uint256) {
        Project storage project = _projects[projectId];
        UserPurchase storage purchaseRecord = _userPurchases[projectId][user];
        if (project.status != ProjectStatus.Successful || purchaseRecord.tokensBought == 0) return 0;

        uint256 unlocked;
        for (uint256 i = 0; i < project.vestingTimes.length; ++i) {
            if (block.timestamp < project.vestingTimes[i]) break;
            unlocked += project.vestingPercents[i];
        }
        uint256 vested = Math.mulDiv(purchaseRecord.tokensBought, unlocked, BASIS_POINTS);
        return vested > purchaseRecord.claimedTokens ? vested - purchaseRecord.claimedTokens : 0;
    }

    function claim(uint256 projectId) external nonReentrant {
        Project storage project = _requireProject(projectId);
        uint256 amount = calculateClaimable(projectId, msg.sender);
        if (amount == 0) revert NoTokensToClaim();
        _userPurchases[projectId][msg.sender].claimedTokens += amount;
        _transferOutExact(project.projectToken, msg.sender, amount);
        emit Claimed(projectId, msg.sender, project.projectToken, amount);
    }

    function getProject(uint256 projectId) external view returns (Project memory) {
        return _projects[projectId];
    }

    function getUserPurchase(uint256 projectId, address user) external view returns (UserPurchase memory) {
        return _userPurchases[projectId][user];
    }

    function getProjectParticipants(uint256 projectId) external view returns (address[] memory) {
        return _projectParticipants[projectId];
    }

    function _refund(uint256 projectId, address user) private {
        Project storage project = _requireProject(projectId);
        if (project.status != ProjectStatus.Failed && project.status != ProjectStatus.Cancelled) {
            revert InvalidProjectStatus(project.status);
        }
        UserPurchase storage purchaseRecord = _userPurchases[projectId][user];
        if (purchaseRecord.paymentAmount == 0) revert NoPurchase();
        if (purchaseRecord.refunded) revert AlreadyRefunded();
        purchaseRecord.refunded = true;
        uint256 amount = purchaseRecord.paymentAmount;
        _transferOutExact(project.paymentToken, user, amount);
        emit Refunded(projectId, user, project.paymentToken, amount);
    }

    function _finalizeProject(Project storage project) private {
        project.status = project.totalRaised >= project.softCap ? ProjectStatus.Successful : ProjectStatus.Failed;
        emit ProjectEnded(project.projectId, project.status, project.paymentToken, project.totalRaised);
    }

    function _transferInExact(address tokenAddress, address from, uint256 amount) private returns (uint256) {
        IERC20 token = IERC20(tokenAddress);
        uint256 beforeBalance = token.balanceOf(address(this));
        token.safeTransferFrom(from, address(this), amount);
        uint256 received = token.balanceOf(address(this)) - beforeBalance;
        if (received != amount) revert UnsupportedTokenBehavior(tokenAddress, amount, received);
        return received;
    }

    function _increaseReserve(address tokenAddress, uint256 amount) private {
        uint256 newReserve = reservedBalances[tokenAddress] + amount;
        uint256 balance = IERC20(tokenAddress).balanceOf(address(this));
        if (balance < newReserve) revert TokenReserveInsolvent(tokenAddress, balance, newReserve);
        reservedBalances[tokenAddress] = newReserve;
    }

    function _transferOutExact(address tokenAddress, address recipient, uint256 amount) private {
        if (recipient == address(0) || recipient == address(this)) revert InvalidAddress();
        IERC20 token = IERC20(tokenAddress);
        uint256 senderBefore = token.balanceOf(address(this));
        uint256 recipientBefore = token.balanceOf(recipient);
        uint256 reserveBefore = reservedBalances[tokenAddress];
        if (reserveBefore < amount || senderBefore < reserveBefore) {
            revert TokenReserveInsolvent(tokenAddress, senderBefore, reserveBefore);
        }

        reservedBalances[tokenAddress] = reserveBefore - amount;
        token.safeTransfer(recipient, amount);

        uint256 senderAfter = token.balanceOf(address(this));
        uint256 recipientAfter = token.balanceOf(recipient);
        uint256 senderDecrease = senderBefore >= senderAfter ? senderBefore - senderAfter : 0;
        uint256 recipientIncrease = recipientAfter >= recipientBefore ? recipientAfter - recipientBefore : 0;
        if (senderDecrease != amount || recipientIncrease != amount) {
            revert UnsupportedOutgoingTokenBehavior(tokenAddress, amount, senderDecrease, recipientIncrease);
        }
        uint256 reserveAfter = reservedBalances[tokenAddress];
        if (senderAfter < reserveAfter) revert TokenReserveInsolvent(tokenAddress, senderAfter, reserveAfter);
    }

    function _requireRuntimeDecimals(address tokenAddress, uint8 configuredDecimals) private view {
        if (tokenAddress.code.length == 0) revert TokenHasNoCode(tokenAddress);
        uint8 currentDecimals = _readSupportedDecimals(tokenAddress);
        if (currentDecimals != configuredDecimals) {
            revert TokenDecimalsChanged(tokenAddress, configuredDecimals, currentDecimals);
        }
    }

    function _readSupportedDecimals(address token) private view returns (uint8 tokenDecimals) {
        try IERC20Metadata(token).decimals() returns (uint8 value) {
            tokenDecimals = value;
        } catch {
            revert InvalidTokenDecimals(0);
        }
        if (tokenDecimals == 0 || tokenDecimals > MAX_SUPPORTED_DECIMALS) {
            revert InvalidTokenDecimals(tokenDecimals);
        }
    }

    function _validateVesting(
        uint256 endTime,
        uint256[] calldata vestingTimes,
        uint256[] calldata vestingPercents
    ) private pure {
        if (vestingTimes.length == 0 || vestingTimes.length > 10 || vestingTimes.length != vestingPercents.length) {
            revert InvalidVestingSchedule();
        }
        uint256 totalPercent;
        for (uint256 i = 0; i < vestingTimes.length; ++i) {
            if (
                vestingTimes[i] <= endTime || vestingPercents[i] == 0 ||
                (i > 0 && vestingTimes[i] <= vestingTimes[i - 1])
            ) revert InvalidVestingSchedule();
            totalPercent += vestingPercents[i];
        }
        if (totalPercent != BASIS_POINTS) revert InvalidVestingSchedule();
    }

    function _requireProject(uint256 projectId) private view returns (Project storage project) {
        project = _projects[projectId];
        if (project.projectToken == address(0)) revert ProjectDoesNotExist(projectId);
    }

    function _requireOperatorOrOwner() private view {
        if (msg.sender != operator && msg.sender != owner()) revert UnauthorizedOperator();
    }

    uint256[49] private __gap;
}

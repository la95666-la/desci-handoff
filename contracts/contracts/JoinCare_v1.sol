// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

import "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import "./utils/ReentrancyGuardUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/token/ERC20/extensions/IERC20Metadata.sol";

/**
 * @title JoinCare_v1
 * @dev JoinCare IDO平台主合约（UUPS可升级模式）
 *
 * 核心功能：
 * - 单合约管理多个IDO项目
 * - 用户使用JCC代币认购项目代币
 * - 支持TGE释放 + 线性释放
 * - 平台收取2%成功抽成
 *
 * 安全特性：
 * - ReentrancyGuard: 防重入攻击
 * - Pausable: 紧急暂停
 * - Ownable: 权限控制
 * - SafeERC20: 安全的ERC20交互
 * - Fee-on-transfer防御: balanceBefore/balanceAfter
 * - 18 decimals强制验证
 * - 释放参数验证（防除零）
 */
contract JoinCare_v1 is
    Initializable,
    UUPSUpgradeable,
    OwnableUpgradeable,
    ReentrancyGuardUpgradeable,
    PausableUpgradeable
{
    using SafeERC20 for IERC20;

    // ============ 枚举定义 ============

    /**
     * @dev 项目状态枚举
     */
    enum ProjectStatus {
        Pending,    // 待激活（项目方充值代币后）
        Active,     // 进行中（用户可以认购）
        Ended,      // 已结束（等待判定成功/失败）
        Successful, // 成功（达到软顶，可提取资金）
        Failed,     // 失败（未达软顶，可退款）
        Cancelled   // 已取消（管理员取消）
    }

    // ============ 结构体定义 ============

    /**
     * @dev 项目信息结构体
     */
    struct Project {
        uint256 projectId;              // 项目ID
        address projectToken;           // 项目代币地址（必须18 decimals）
        address projectOwner;           // 项目方地址
        uint256 tokenPrice;             // 代币价格（1 token = tokenPrice JCC，18 decimals）
        uint256 hardCap;                // 硬顶（JCC数量，18 decimals）
        uint256 softCap;                // 软顶（JCC数量，18 decimals）
        uint256 maxPerUser;             // 单用户认购上限（JCC数量，18 decimals）
        uint256 startTime;              // 认购开始时间（Unix时间戳）
        uint256 endTime;                // 认购结束时间（Unix时间戳）
        uint256[] vestingTimes;         // 释放时间点数组（Unix时间戳）
        uint256[] vestingPercents;      // 释放百分比数组（基点，总和=10000）
        uint256 vestingDuration;        // 释放总时长（秒，用于验证）
        ProjectStatus status;           // 项目状态
        uint256 totalRaised;            // 已募集的JCC总量（18 decimals）
        uint256 totalTokensSold;        // 已售出的项目代币总量（18 decimals）
        uint256 totalTokensDeposited;   // 项目方充值的代币总量（18 decimals）
        bool fundsWithdrawn;            // 募集资金是否已提取
        bool projectTokensWithdrawn;    // 项目代币是否已提取
    }

    /**
     * @dev 用户认购信息结构体
     */
    struct UserPurchase {
        uint256 totalJCCSpent;      // 用户投入的JCC总量（18 decimals）
        uint256 totalTokensBought;  // 用户购买的项目代币总量（18 decimals）
        uint256 claimedTokens;      // 已领取的代币数量（18 decimals）
        bool refunded;              // 是否已退款
    }

    /**
     * @dev 平台费用结构体
     */
    struct PlatformFees {
        uint256 successFeeRate;     // 成功抽成比例（基点，200 = 2%）
        address feeReceiver;        // 费用接收地址
    }

    // ============ 状态变量 ============

    /// @dev JCC平台代币地址
    address public jccToken;

    /// @dev 平台费用配置
    PlatformFees public platformFees;

    /// @dev 项目ID => 项目信息
    mapping(uint256 => Project) public projects;

    /// @dev 项目ID => 用户地址 => 认购信息
    mapping(uint256 => mapping(address => UserPurchase)) public userPurchases;

    /// @dev 项目ID => 参与者地址列表
    mapping(uint256 => address[]) public projectParticipants;

    /// @dev 项目ID => 用户地址 => 是否已参与（用于去重）
    mapping(uint256 => mapping(address => bool)) public hasParticipated;

    // ============ 常量 ============

    /// @dev 基点分母（10000 = 100%）
    uint256 public constant BASIS_POINTS = 10000;

    /// @dev 最大平台费率（1000 = 10%）
    uint256 public constant MAX_FEE_RATE = 1000;

    /// @dev 必须的代币精度（18 decimals）
    uint8 public constant REQUIRED_DECIMALS = 18;

    // ============ 事件定义 ============

    /**
     * @dev 项目创建事件
     */
    event ProjectCreated(
        uint256 indexed projectId,
        address indexed projectToken,
        address indexed projectOwner,
        uint256 tokenPrice,
        uint256 hardCap,
        uint256 softCap
    );

    /**
     * @dev 项目方充值代币事件
     */
    event ProjectTokensDeposited(
        uint256 indexed projectId,
        uint256 amount
    );

    /**
     * @dev 项目激活事件
     */
    event ProjectActivated(uint256 indexed projectId);

    /**
     * @dev 用户认购事件
     */
    event Purchase(
        uint256 indexed projectId,
        address indexed user,
        uint256 jccAmount,
        uint256 tokenAmount
    );

    /**
     * @dev 项目结束事件
     */
    event ProjectEnded(
        uint256 indexed projectId,
        ProjectStatus status,
        uint256 totalRaised
    );

    /**
     * @dev 用户领取代币事件
     */
    event Claimed(
        uint256 indexed projectId,
        address indexed user,
        uint256 amount
    );

    /**
     * @dev 用户退款事件
     */
    event Refunded(
        uint256 indexed projectId,
        address indexed user,
        uint256 jccAmount
    );

    /**
     * @dev 项目方提取募集资金事件
     */
    event RaisedFundsWithdrawn(
        uint256 indexed projectId,
        address indexed projectOwner,
        uint256 amount,
        uint256 platformFee
    );

    /**
     * @dev 项目方提取项目代币事件
     */
    event ProjectTokensWithdrawn(
        uint256 indexed projectId,
        address indexed projectOwner,
        uint256 amount
    );

    /**
     * @dev 平台费用更新事件
     */
    event PlatformFeesUpdated(
        uint256 successFeeRate,
        address feeReceiver
    );

    /**
     * @dev 项目取消事件
     */
    event ProjectCancelled(uint256 indexed projectId);

    // ============ 初始化和升级函数 ============

    /**
     * @dev 初始化函数（替代constructor）
     * @param _jccToken JCC代币地址
     * @param _feeReceiver 平台费用接收地址
     * @param _successFeeRate 成功抽成比例（基点，200 = 2%）
     */
    function initialize(
        address _jccToken,
        address _feeReceiver,
        uint256 _successFeeRate
    ) public initializer {
        require(_jccToken != address(0), "Invalid JC token address");
        require(_feeReceiver != address(0), "Invalid fee receiver address");
        require(_successFeeRate <= MAX_FEE_RATE, "Fee rate too high");

        // 初始化继承的合约
        __Ownable_init(msg.sender);
        __Pausable_init();
        __ReentrancyGuard_init();

        // 设置状态变量
        jccToken = _jccToken;
        platformFees = PlatformFees({
            successFeeRate: _successFeeRate,
            feeReceiver: _feeReceiver
        });
    }

    /**
     * @dev 授权升级函数（仅owner可调用）
     * @param newImplementation 新实现合约地址
     */
    function _authorizeUpgrade(address newImplementation)
        internal
        override
        onlyOwner
    {
        // 仅owner可以升级合约
    }

    // ============ 项目管理模块 ============

    /**
     * @dev 创建新项目（仅owner）
     * @param _projectId 项目ID（由服务端生成）
     * @param _projectToken 项目代币地址
     * @param _projectOwner 项目方地址
     * @param _tokenPrice 代币价格（1 token = _tokenPrice JCC）
     * @param _hardCap 硬顶（JCC数量）
     * @param _softCap 软顶（JCC数量）
     * @param _maxPerUser 单用户认购上限（JCC数量）
     * @param _startTime 认购开始时间
     * @param _endTime 认购结束时间
     * @param _vestingTimes 释放时间点数组
     * @param _vestingPercents 释放百分比数组
     * @param _vestingDuration 释放总时长（秒）
     */
    function createProject(
        uint256 _projectId,
        address _projectToken,
        address _projectOwner,
        uint256 _tokenPrice,
        uint256 _hardCap,
        uint256 _softCap,
        uint256 _maxPerUser,
        uint256 _startTime,
        uint256 _endTime,
        uint256[] calldata _vestingTimes,
        uint256[] calldata _vestingPercents,
        uint256 _vestingDuration
    ) external onlyOwner whenNotPaused returns (uint256) {
        // 1. 验证 projectId
        require(_projectId > 0, "Invalid project ID");
        require(projects[_projectId].projectToken == address(0), "Project ID already exists");

        // 2. 基础参数验证
        require(_projectToken != address(0), "Invalid project token");
        require(_projectOwner != address(0), "Invalid project owner");
        require(_tokenPrice > 0, "Token price must be positive");
        require(_hardCap > 0, "Hard cap must be positive");
        require(_softCap > 0, "Soft cap must be positive");
        require(_softCap <= _hardCap, "Soft cap must <= hard cap");
        require(_maxPerUser > 0, "Max per user must be positive");
        require(_maxPerUser <= _hardCap, "Max per user must <= hard cap");

        // 3. 时间参数验证
        require(_startTime > block.timestamp, "Start time must be in future");
        require(_endTime > _startTime, "End time must be after start time");

        // 4. 项目代币必须是18 decimals（防止精度错误）
        uint8 decimals = IERC20Metadata(_projectToken).decimals();
        require(decimals == REQUIRED_DECIMALS, "Project token must be 18 decimals");

        // 5. 释放参数验证
        _validateVestingSchedule(_endTime, _vestingTimes, _vestingPercents, _vestingDuration);

        // 6. 创建项目
        Project storage project = projects[_projectId];
        project.projectId = _projectId;
        project.projectToken = _projectToken;
        project.projectOwner = _projectOwner;
        project.tokenPrice = _tokenPrice;
        project.hardCap = _hardCap;
        project.softCap = _softCap;
        project.maxPerUser = _maxPerUser;
        project.startTime = _startTime;
        project.endTime = _endTime;
        project.vestingTimes = _vestingTimes;
        project.vestingPercents = _vestingPercents;
        project.vestingDuration = _vestingDuration;
        project.status = ProjectStatus.Pending;
        project.totalRaised = 0;
        project.totalTokensSold = 0;
        project.totalTokensDeposited = 0;
        project.fundsWithdrawn = false;
        project.projectTokensWithdrawn = false;

        emit ProjectCreated(
            _projectId,
            _projectToken,
            _projectOwner,
            _tokenPrice,
            _hardCap,
            _softCap
        );

        return _projectId;
    }

    /**
     * @dev 验证释放计划的有效性
     * @param _endTime 认购结束时间
     * @param _vestingTimes 释放时间点数组
     * @param _vestingPercents 释放百分比数组
     * @param _vestingDuration 释放总时长
     */
    function _validateVestingSchedule(
        uint256 _endTime,
        uint256[] calldata _vestingTimes,
        uint256[] calldata _vestingPercents,
        uint256 _vestingDuration
    ) private pure {
        // 1. 数组不能为空
        require(_vestingTimes.length > 0, "Empty vesting schedule");

        // 2. 数组长度必须一致
        require(
            _vestingTimes.length == _vestingPercents.length,
            "Vesting arrays length mismatch"
        );

        // 3. 数组长度不能超过10
        require(_vestingTimes.length <= 10, "Too many vesting periods");

        // 4. 验证时间递增和百分比总和
        uint256 totalPercent = 0;
        for (uint256 i = 0; i < _vestingTimes.length; i++) {
            // 百分比必须>0
            require(_vestingPercents[i] > 0, "Vesting percent must be > 0");
            totalPercent += _vestingPercents[i];

            // 时间必须递增
            if (i > 0) {
                require(
                    _vestingTimes[i] > _vestingTimes[i - 1],
                    "Vesting times must be ascending"
                );
            }
        }

        // 5. 总百分比必须等于100%
        require(totalPercent == BASIS_POINTS, "Total vesting percent must be 100%");

        // 6. 第一个释放时间必须晚于认购结束时间
        require(
            _vestingTimes[0] > _endTime,
            "First vesting time must be after endTime"
        );

        // 7. 如果设置了vestingDuration，最后一个时间不能超过
        if (_vestingDuration > 0) {
            require(
                _vestingTimes[_vestingTimes.length - 1] <= _vestingTimes[0] + _vestingDuration,
                "Last vesting time exceeds duration"
            );
        }
    }

    /**
     * @dev 项目方充值项目代币
     * @param _projectId 项目ID
     * @param _amount 充值数量
     *
     * 安全特性：
     * - 使用balanceBefore/balanceAfter模式防御fee-on-transfer代币
     * - 记录实际到账数量
     */
    function depositProjectTokens(uint256 _projectId, uint256 _amount)
        external
        nonReentrant
        whenNotPaused
    {
        Project storage project = projects[_projectId];

        // 1. 验证项目状态
        require(project.projectToken != address(0), "Project does not exist");
        require(project.status == ProjectStatus.Pending, "Project not in Pending status");
        require(msg.sender == project.projectOwner, "Only project owner can deposit");
        require(_amount > 0, "Amount must be positive");

        // 2. 使用balanceBefore/balanceAfter模式（防御fee-on-transfer代币）
        IERC20 token = IERC20(project.projectToken);
        uint256 balanceBefore = token.balanceOf(address(this));

        // 3. 转移代币到合约
        token.safeTransferFrom(msg.sender, address(this), _amount);

        // 4. 计算实际到账数量
        uint256 balanceAfter = token.balanceOf(address(this));
        uint256 actualAmount = balanceAfter - balanceBefore;

        // 5. 更新项目状态
        project.totalTokensDeposited += actualAmount;

        emit ProjectTokensDeposited(_projectId, actualAmount);
    }

    /**
     * @dev 手动激活项目（仅owner）
     * @param _projectId 项目ID
     */
    function activateProject(uint256 _projectId)
        external
        onlyOwner
        whenNotPaused
    {
        Project storage project = projects[_projectId];

        require(project.projectToken != address(0), "Project does not exist");
        require(project.status == ProjectStatus.Pending, "Project not in Pending status");
        require(project.totalTokensDeposited > 0, "Project tokens not deposited");

        project.status = ProjectStatus.Active;

        emit ProjectActivated(_projectId);
    }

    /**
     * @dev 内部函数：结束项目并判定成功/失败
     * @param _projectId 项目ID
     */
    function _endProject(uint256 _projectId) internal {
        Project storage project = projects[_projectId];

        // 判定项目成功或失败
        if (project.totalRaised >= project.softCap) {
            project.status = ProjectStatus.Successful;
        } else {
            project.status = ProjectStatus.Failed;
        }

        emit ProjectEnded(_projectId, project.status, project.totalRaised);
    }

    /**
     * @dev 手动结束项目（去中心化）
     * @param _projectId 项目ID
     *
     * 权限说明：
     * - 任何人都可以在 endTime 之后调用此函数
     * - owner 可以在任何时候调用（用于紧急情况，如发现项目方欺诈）
     */
    function endProject(uint256 _projectId)
        external
        whenNotPaused
    {
        Project storage project = projects[_projectId];

        require(project.projectToken != address(0), "Project does not exist");
        require(project.status == ProjectStatus.Active, "Project not active");

        // 如果不是 owner，必须等到 endTime 之后才能调用
        if (msg.sender != owner()) {
            require(block.timestamp >= project.endTime, "Project not ended yet");
        }
        // owner 可以随时结束项目（用于紧急情况）

        _endProject(_projectId);
    }

    // ============ 认购模块 ============

    /**
     * @dev 用户认购项目代币
     * @param _projectId 项目ID
     * @param _jccAmount 认购的JCC数量
     *
     * 特性：
     * - 自动激活：如果项目处于Pending状态且已充值代币，首次认购时自动激活
     * - 自动结束：如果达到硬顶，自动结束项目
     */
    function purchase(uint256 _projectId, uint256 _jccAmount)
        external
        nonReentrant
        whenNotPaused
    {
        Project storage project = projects[_projectId];

        // 1. 验证项目存在
        require(project.projectToken != address(0), "Project does not exist");

        // 2. 自动激活逻辑（双重保障机制）
        if (project.status == ProjectStatus.Pending) {
            require(project.totalTokensDeposited > 0, "Project tokens not deposited");
            require(block.timestamp >= project.startTime, "Project not started yet");
            project.status = ProjectStatus.Active;
            emit ProjectActivated(_projectId);
        }

        // 3. 验证项目状态
        require(project.status == ProjectStatus.Active, "Project not active");

        // 4. 验证时间窗口
        require(block.timestamp >= project.startTime, "Project not started yet");
        require(block.timestamp <= project.endTime, "Project ended");

        // 5. 验证认购金额
        require(_jccAmount > 0, "Amount must be positive");

        // 6. 验证不超过硬顶
        require(
            project.totalRaised + _jccAmount <= project.hardCap,
            "Exceeds hard cap"
        );

        // 7. 验证单用户上限
        UserPurchase storage userPurchase = userPurchases[_projectId][msg.sender];
        require(
            userPurchase.totalJCCSpent + _jccAmount <= project.maxPerUser,
            "Exceeds max per user"
        );

        // 8. 计算可购买的项目代币数量
        // tokenAmount = jccAmount / tokenPrice
        uint256 tokenAmount = (_jccAmount * 1e18) / project.tokenPrice;
        require(tokenAmount > 0, "Token amount too small");

        // 9. 验证项目方充值的代币足够
        require(
            project.totalTokensSold + tokenAmount <= project.totalTokensDeposited,
            "Insufficient project tokens"
        );

        // 10. 转移JCC到合约
        IERC20(jccToken).safeTransferFrom(msg.sender, address(this), _jccAmount);

        // 11. 更新项目状态
        project.totalRaised += _jccAmount;
        project.totalTokensSold += tokenAmount;

        // 12. 更新用户认购信息
        userPurchase.totalJCCSpent += _jccAmount;
        userPurchase.totalTokensBought += tokenAmount;

        // 13. 记录参与者（去重）
        if (!hasParticipated[_projectId][msg.sender]) {
            projectParticipants[_projectId].push(msg.sender);
            hasParticipated[_projectId][msg.sender] = true;
        }

        emit Purchase(_projectId, msg.sender, _jccAmount, tokenAmount);

        // 14. 自动结束检查：如果达到硬顶，自动结束项目
        if (project.totalRaised >= project.hardCap) {
            _endProject(_projectId);
        }
    }

    // ============ 释放与领取模块 ============

    /**
     * @dev 计算用户可领取的代币数量（view函数）
     * @param _projectId 项目ID
     * @param _user 用户地址
     * @return 可领取的代币数量
     *
     * 释放逻辑：
     * - 遍历vestingTimes数组，累加已到期的vestingPercents
     * - 总可领取 = totalTokensBought × 累计百分比 / 10000
     * - 减去已领取数量 claimedTokens
     */
    function calculateClaimable(uint256 _projectId, address _user)
        public
        view
        returns (uint256)
    {
        Project storage project = projects[_projectId];
        UserPurchase storage userPurchase = userPurchases[_projectId][_user];

        // 1. 验证项目成功
        if (project.status != ProjectStatus.Successful) {
            return 0;
        }

        // 2. 验证用户有认购
        if (userPurchase.totalTokensBought == 0) {
            return 0;
        }

        // 3. 计算已解锁的总百分比
        uint256 unlockedPercent = 0;
        for (uint256 i = 0; i < project.vestingTimes.length; i++) {
            if (block.timestamp >= project.vestingTimes[i]) {
                unlockedPercent += project.vestingPercents[i];
            } else {
                break; // 时间是递增的，后面的都不满足
            }
        }

        // 4. 如果没有解锁，返回0
        if (unlockedPercent == 0) {
            return 0;
        }

        // 5. 计算总可领取数量
        uint256 totalClaimable = (userPurchase.totalTokensBought * unlockedPercent) / BASIS_POINTS;

        // 6. 减去已领取数量
        if (totalClaimable <= userPurchase.claimedTokens) {
            return 0;
        }

        return totalClaimable - userPurchase.claimedTokens;
    }

    /**
     * @dev 用户领取代币
     * @param _projectId 项目ID
     */
    function claim(uint256 _projectId)
        external
        nonReentrant
        whenNotPaused
    {
        Project storage project = projects[_projectId];
        UserPurchase storage userPurchase = userPurchases[_projectId][msg.sender];

        // 1. 验证项目成功
        require(project.status == ProjectStatus.Successful, "Project not successful");

        // 2. 验证第一个释放时间已到
        require(block.timestamp >= project.vestingTimes[0], "First vesting not started yet");

        // 3. 计算可领取数量
        uint256 claimableAmount = calculateClaimable(_projectId, msg.sender);
        require(claimableAmount > 0, "No tokens to claim");

        // 4. 更新已领取数量
        userPurchase.claimedTokens += claimableAmount;

        // 5. 转移代币给用户
        IERC20(project.projectToken).safeTransfer(msg.sender, claimableAmount);

        emit Claimed(_projectId, msg.sender, claimableAmount);
    }

    // ============ 退款模块 ============

    /**
     * @dev 用户申请退款（项目失败时）
     * @param _projectId 项目ID
     */
    function refund(uint256 _projectId)
        external
        nonReentrant
        whenNotPaused
    {
        Project storage project = projects[_projectId];
        UserPurchase storage userPurchase = userPurchases[_projectId][msg.sender];

        // 1. 验证项目失败
        require(project.status == ProjectStatus.Failed, "Project not failed");

        // 2. 验证用户有认购
        require(userPurchase.totalJCCSpent > 0, "No purchase to refund");

        // 3. 验证未退款
        require(!userPurchase.refunded, "Already refunded");

        // 4. 标记已退款
        userPurchase.refunded = true;

        // 5. 退还JCC
        uint256 refundAmount = userPurchase.totalJCCSpent;
        IERC20(jccToken).safeTransfer(msg.sender, refundAmount);

        emit Refunded(_projectId, msg.sender, refundAmount);
    }

    /**
     * @dev 批量退款（仅owner，紧急情况使用）
     * @param _projectId 项目ID
     * @param _users 用户地址列表
     */
    function batchRefund(uint256 _projectId, address[] calldata _users)
        external
        onlyOwner
        nonReentrant
        whenNotPaused
    {
        Project storage project = projects[_projectId];

        // 验证项目失败
        require(project.status == ProjectStatus.Failed, "Project not failed");

        for (uint256 i = 0; i < _users.length; i++) {
            address user = _users[i];
            UserPurchase storage userPurchase = userPurchases[_projectId][user];

            // 跳过已退款或无认购的用户
            if (userPurchase.refunded || userPurchase.totalJCCSpent == 0) {
                continue;
            }

            // 标记已退款
            userPurchase.refunded = true;

            // 退还JCC
            uint256 refundAmount = userPurchase.totalJCCSpent;
            IERC20(jccToken).safeTransfer(user, refundAmount);

            emit Refunded(_projectId, user, refundAmount);
        }
    }

    // ============ 资金流管理模块 ============

    /**
     * @dev 项目方提取募集资金（项目成功时）
     * @param _projectId 项目ID
     *
     * 资金分配：
     * - 平台费用：totalRaised × successFeeRate / 10000 → feeReceiver
     * - 项目方：totalRaised - 平台费用 → projectOwner
     */
    function withdrawRaisedFunds(uint256 _projectId)
        external
        nonReentrant
        whenNotPaused
    {
        Project storage project = projects[_projectId];

        // 1. 验证项目成功
        require(project.status == ProjectStatus.Successful, "Project not successful");

        // 2. 验证调用者是项目方
        require(msg.sender == project.projectOwner, "Only project owner can withdraw");

        // 3. 验证未提取过（防止双花）
        require(!project.fundsWithdrawn, "Funds already withdrawn");

        // 4. 标记已提取
        project.fundsWithdrawn = true;

        // 5. 计算平台费用
        uint256 platformFee = (project.totalRaised * platformFees.successFeeRate) / BASIS_POINTS;
        uint256 projectOwnerAmount = project.totalRaised - platformFee;

        // 6. 转账给平台
        if (platformFee > 0) {
            IERC20(jccToken).safeTransfer(platformFees.feeReceiver, platformFee);
        }

        // 7. 转账给项目方
        IERC20(jccToken).safeTransfer(project.projectOwner, projectOwnerAmount);

        emit RaisedFundsWithdrawn(_projectId, project.projectOwner, projectOwnerAmount, platformFee);
    }

    /**
     * @dev 项目方提取项目代币
     * @param _projectId 项目ID
     *
     * 可提取数量：
     * - 成功：totalTokensDeposited - totalTokensSold（未售出部分）
     * - 失败：totalTokensDeposited（全部）
     */
    function withdrawProjectTokens(uint256 _projectId)
        external
        nonReentrant
        whenNotPaused
    {
        Project storage project = projects[_projectId];

        // 1. 验证项目已结束（成功或失败）
        require(
            project.status == ProjectStatus.Successful || project.status == ProjectStatus.Failed,
            "Project not ended"
        );

        // 2. 验证调用者是项目方
        require(msg.sender == project.projectOwner, "Only project owner can withdraw");

        // 3. 验证未提取过（防止双花）
        require(!project.projectTokensWithdrawn, "Tokens already withdrawn");

        // 4. 标记已提取
        project.projectTokensWithdrawn = true;

        // 5. 计算可提取数量
        uint256 withdrawAmount;
        if (project.status == ProjectStatus.Successful) {
            // 成功：提取未售出部分
            withdrawAmount = project.totalTokensDeposited - project.totalTokensSold;
        } else {
            // 失败：提取全部
            withdrawAmount = project.totalTokensDeposited;
        }

        // 6. 转账给项目方
        if (withdrawAmount > 0) {
            IERC20(project.projectToken).safeTransfer(project.projectOwner, withdrawAmount);
        }

        emit ProjectTokensWithdrawn(_projectId, project.projectOwner, withdrawAmount);
    }

    /**
     * @dev 设置平台费用（仅owner）
     * @param _successFeeRate 成功抽成比例（基点）
     * @param _feeReceiver 费用接收地址
     */
    function setPlatformFees(uint256 _successFeeRate, address _feeReceiver)
        external
        onlyOwner
    {
        require(_feeReceiver != address(0), "Invalid fee receiver");
        require(_successFeeRate <= MAX_FEE_RATE, "Fee rate too high");

        platformFees.successFeeRate = _successFeeRate;
        platformFees.feeReceiver = _feeReceiver;

        emit PlatformFeesUpdated(_successFeeRate, _feeReceiver);
    }

    // ============ 查询和控制模块 ============

    /**
     * @dev 查询项目信息
     * @param _projectId 项目ID
     * @return 项目信息
     */
    function getProject(uint256 _projectId)
        external
        view
        returns (Project memory)
    {
        return projects[_projectId];
    }

    /**
     * @dev 查询用户认购信息
     * @param _projectId 项目ID
     * @param _user 用户地址
     * @return 用户认购信息
     */
    function getUserPurchase(uint256 _projectId, address _user)
        external
        view
        returns (UserPurchase memory)
    {
        return userPurchases[_projectId][_user];
    }

    /**
     * @dev 查询项目参与者数量
     * @param _projectId 项目ID
     * @return 参与者数量
     */
    function getParticipantCount(uint256 _projectId)
        external
        view
        returns (uint256)
    {
        return projectParticipants[_projectId].length;
    }

    /**
     * @dev 暂停合约（仅owner，紧急情况）
     */
    function pause() external onlyOwner {
        _pause();
    }

    /**
     * @dev 恢复合约（仅owner）
     */
    function unpause() external onlyOwner {
        _unpause();
    }

    // ============ 存储间隙 ============

    /**
     * @dev 预留存储空间，用于未来升级时添加新的状态变量
     * 重要：升级时可以在__gap之前添加新变量，但必须相应减少__gap的大小
     */
    uint256[50] private __gap;
}


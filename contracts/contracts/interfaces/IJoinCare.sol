// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

/**
 * @title IJoinCare
 * @dev JoinCare IDO平台接口定义
 */
interface IJoinCare {
    // ============ 枚举定义 ============

    enum ProjectStatus {
        Pending,
        Active,
        Ended,
        Successful,
        Failed,
        Cancelled
    }

    // ============ 结构体定义 ============

    struct Project {
        uint256 projectId;
        address projectToken;
        address projectOwner;
        uint256 tokenPrice;
        uint256 hardCap;
        uint256 softCap;
        uint256 maxPerUser;
        uint256 startTime;
        uint256 endTime;
        uint256[] vestingTimes;      // 释放时间点数组
        uint256[] vestingPercents;   // 释放百分比数组（基点，总和=10000）
        uint256 vestingDuration;     // 释放总时长（用于验证）
        ProjectStatus status;
        uint256 totalRaised;
        uint256 totalTokensSold;
        uint256 totalTokensDeposited;
        bool fundsWithdrawn;
        bool projectTokensWithdrawn;
    }

    struct UserPurchase {
        uint256 totalJCCSpent;
        uint256 totalTokensBought;
        uint256 claimedTokens;
        bool refunded;
    }

    struct PlatformFees {
        uint256 successFeeRate;
        address feeReceiver;
    }

    // ============ 事件定义 ============

    event ProjectCreated(
        uint256 indexed projectId,
        address indexed projectToken,
        address indexed projectOwner,
        uint256 tokenPrice,
        uint256 hardCap,
        uint256 softCap
    );

    event ProjectTokensDeposited(
        uint256 indexed projectId,
        uint256 amount
    );

    event ProjectActivated(uint256 indexed projectId);

    event Purchase(
        uint256 indexed projectId,
        address indexed user,
        uint256 jccAmount,
        uint256 tokenAmount
    );

    event ProjectEnded(
        uint256 indexed projectId,
        ProjectStatus status,
        uint256 totalRaised
    );

    event Claimed(
        uint256 indexed projectId,
        address indexed user,
        uint256 amount
    );

    event Refunded(
        uint256 indexed projectId,
        address indexed user,
        uint256 jccAmount
    );

    event RaisedFundsWithdrawn(
        uint256 indexed projectId,
        address indexed projectOwner,
        uint256 amount,
        uint256 platformFee
    );

    event ProjectTokensWithdrawn(
        uint256 indexed projectId,
        address indexed projectOwner,
        uint256 amount
    );

    event PlatformFeesUpdated(
        uint256 successFeeRate,
        address feeReceiver
    );

    event ProjectCancelled(uint256 indexed projectId);

    // ============ 函数定义 ============

    // 初始化函数
    function initialize(
        address _jccToken,
        address _feeReceiver,
        uint256 _successFeeRate
    ) external;

    // 项目管理函数
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
    ) external returns (uint256);

    function depositProjectTokens(uint256 _projectId, uint256 _amount) external;

    function activateProject(uint256 _projectId) external;

    function endProject(uint256 _projectId) external;

    // 认购函数
    function purchase(uint256 _projectId, uint256 _jccAmount) external;

    // 领取函数
    function calculateClaimable(uint256 _projectId, address _user)
        external
        view
        returns (uint256);

    function claim(uint256 _projectId) external;

    // 退款函数
    function refund(uint256 _projectId) external;

    function batchRefund(uint256 _projectId, address[] calldata _users) external;

    // 资金管理函数
    function withdrawRaisedFunds(uint256 _projectId) external;

    function withdrawProjectTokens(uint256 _projectId) external;

    function setPlatformFees(uint256 _successFeeRate, address _feeReceiver) external;

    // 查询函数
    function getProject(uint256 _projectId) external view returns (Project memory);

    function getUserPurchase(uint256 _projectId, address _user)
        external
        view
        returns (UserPurchase memory);

    function getParticipantCount(uint256 _projectId) external view returns (uint256);

    // 控制函数
    function pause() external;

    function unpause() external;

    // 状态变量查询
    function jccToken() external view returns (address);

    function platformFees() external view returns (PlatformFees memory);
}

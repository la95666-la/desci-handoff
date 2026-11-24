// SPDX-License-Identifier: MIT
pragma solidity 0.8.22;

/// @notice TEST ONLY 2-of-3 Safe-like executor for localhost Hardhat E2E.
/// @dev This is intentionally outside contracts/ and therefore absent from the
/// production compiler/source tree. Owners approve a call digest on-chain and
/// any caller may execute after the threshold is reached. The target observes
/// this contract as msg.sender; no Hardhat account impersonation is needed.
contract FairLaunchSafeExecutor {
    error InvalidOwner();
    error InvalidThreshold();
    error DuplicateOwner();
    error NotOwner();
    error AlreadyApproved();
    error AlreadyExecuted();
    error InsufficientApprovals(uint256 approvals, uint256 threshold);
    error InvalidTarget();
    error CallFailed(bytes reason);

    event TransactionApproved(bytes32 indexed digest, address indexed owner);
    event TransactionExecuted(bytes32 indexed digest, address indexed target, uint256 value, bytes data, bytes result);

    address[3] public owners;
    uint256 public immutable threshold;
    mapping(address => bool) public isOwner;
    mapping(bytes32 => mapping(address => bool)) public approvedBy;
    mapping(bytes32 => uint256) public approvalCount;
    mapping(bytes32 => bool) public executed;

    constructor(address[3] memory owners_, uint256 threshold_) {
        if (threshold_ != 2) revert InvalidThreshold();
        for (uint256 i = 0; i < owners_.length; i++) {
            address owner = owners_[i];
            if (owner == address(0)) revert InvalidOwner();
            if (isOwner[owner]) revert DuplicateOwner();
            isOwner[owner] = true;
            owners[i] = owner;
        }
        threshold = threshold_;
    }

    receive() external payable {}

    function transactionDigest(address target, uint256 value, bytes calldata data, bytes32 salt)
        public
        view
        returns (bytes32)
    {
        return keccak256(abi.encode(block.chainid, address(this), target, value, keccak256(data), salt));
    }

    function approveTransaction(address target, uint256 value, bytes calldata data, bytes32 salt)
        external
        returns (bytes32 digest)
    {
        if (!isOwner[msg.sender]) revert NotOwner();
        digest = transactionDigest(target, value, data, salt);
        if (executed[digest]) revert AlreadyExecuted();
        if (approvedBy[digest][msg.sender]) revert AlreadyApproved();
        approvedBy[digest][msg.sender] = true;
        approvalCount[digest] += 1;
        emit TransactionApproved(digest, msg.sender);
    }

    function executeTransaction(address target, uint256 value, bytes calldata data, bytes32 salt)
        external
        returns (bytes memory result)
    {
        if (target == address(0)) revert InvalidTarget();
        bytes32 digest = transactionDigest(target, value, data, salt);
        if (executed[digest]) revert AlreadyExecuted();
        uint256 approvals = approvalCount[digest];
        if (approvals < threshold) revert InsufficientApprovals(approvals, threshold);
        executed[digest] = true;
        (bool ok, bytes memory response) = target.call{value: value}(data);
        if (!ok) revert CallFailed(response);
        emit TransactionExecuted(digest, target, value, data, response);
        return response;
    }
}

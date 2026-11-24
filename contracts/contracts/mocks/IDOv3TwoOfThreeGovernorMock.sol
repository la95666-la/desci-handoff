// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

/// @dev Local test fixture only. NOT an audited production Safe or deployable governance.
contract IDOv3TwoOfThreeGovernorMock {
    address[3] public signers;
    uint256 public nextProposal;

    struct Proposal {
        address target;
        bytes data;
        uint8 approvals;
        bool executed;
    }

    mapping(uint256 => Proposal) public proposals;
    mapping(uint256 => mapping(address => bool)) public approvedBy;

    constructor(address[3] memory authorizedSigners) {
        for (uint256 i; i < 3; ++i) {
            require(authorizedSigners[i] != address(0), "zero signer");
            for (uint256 j; j < i; ++j) require(authorizedSigners[i] != authorizedSigners[j], "duplicate signer");
        }
        signers = authorizedSigners;
    }

    function _requireSigner() private view {
        require(msg.sender == signers[0] || msg.sender == signers[1] || msg.sender == signers[2], "not signer");
    }

    function propose(address target, bytes calldata data) external returns (uint256 id) {
        _requireSigner();
        require(target != address(0), "zero target");
        id = ++nextProposal;
        Proposal storage proposal = proposals[id];
        proposal.target = target;
        proposal.data = data;
        proposal.approvals = 1;
        approvedBy[id][msg.sender] = true;
    }

    function approve(uint256 id) external {
        _requireSigner();
        Proposal storage proposal = proposals[id];
        require(proposal.target != address(0) && !proposal.executed, "invalid proposal");
        require(!approvedBy[id][msg.sender], "already approved");
        approvedBy[id][msg.sender] = true;
        proposal.approvals += 1;
    }

    function execute(uint256 id) external returns (bytes memory result) {
        _requireSigner();
        Proposal storage proposal = proposals[id];
        require(proposal.approvals >= 2 && !proposal.executed, "threshold not met");
        proposal.executed = true;
        (bool ok, bytes memory data) = proposal.target.call(proposal.data);
        if (!ok) assembly { revert(add(data, 0x20), mload(data)) }
        return data;
    }
}

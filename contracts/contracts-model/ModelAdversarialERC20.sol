// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

// NON-PRODUCTION TEST TOKEN. It exposes an explicit model-only balance corruption
// hook and transfer counters so tests can prove reserve-solvency checks run before
// any ERC-20 transfer/transferFrom call.
import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract ModelAdversarialERC20 is ERC20 {
    uint8 private immutable _modelDecimals;
    uint256 public transferCalls;
    uint256 public transferFromCalls;
    bool public trapTransfer;
    bool public trapTransferFrom;

    error TransferCallReached(bool transferFromCall);

    constructor(string memory name_, string memory symbol_, uint8 decimals_) ERC20(name_, symbol_) {
        _modelDecimals = decimals_;
    }

    function decimals() public view override returns (uint8) {
        return _modelDecimals;
    }

    function mint(address to, uint256 amount) external {
        _mint(to, amount);
    }

    function forceBurn(address from, uint256 amount) external {
        _burn(from, amount);
    }

    function resetTransferCounters() external {
        transferCalls = 0;
        transferFromCalls = 0;
    }

    function setTransferTraps(bool trapTransfer_, bool trapTransferFrom_) external {
        trapTransfer = trapTransfer_;
        trapTransferFrom = trapTransferFrom_;
    }

    function transfer(address to, uint256 value) public override returns (bool) {
        if (trapTransfer) revert TransferCallReached(false);
        transferCalls += 1;
        return super.transfer(to, value);
    }

    function transferFrom(address from, address to, uint256 value) public override returns (bool) {
        if (trapTransferFrom) revert TransferCallReached(true);
        transferFromCalls += 1;
        return super.transferFrom(from, to, value);
    }
}

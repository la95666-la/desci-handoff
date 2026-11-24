// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

// NON-PRODUCTION TEST TOKEN.
import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract ModelExactERC20 is ERC20 {
    uint8 private immutable _modelDecimals;

    constructor(string memory name_, string memory symbol_, uint8 decimals_) ERC20(name_, symbol_) {
        _modelDecimals = decimals_;
    }

    function decimals() public view override returns (uint8) {
        return _modelDecimals;
    }

    function mint(address to, uint256 amount) external {
        _mint(to, amount);
    }
}

// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

// NON-PRODUCTION ADVERSARIAL TEST TOKEN: taxes both transfer directions.
import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract ModelFeeOnTransferERC20 is ERC20 {
    constructor() ERC20("Model Fee Token", "MFEE") {}

    function mint(address to, uint256 amount) external {
        _mint(to, amount);
    }

    function _update(address from, address to, uint256 amount) internal override {
        if (from == address(0) || to == address(0)) {
            super._update(from, to, amount);
            return;
        }
        uint256 fee = amount / 50;
        super._update(from, to, amount - fee);
        if (fee != 0) super._update(from, address(0), fee);
    }
}

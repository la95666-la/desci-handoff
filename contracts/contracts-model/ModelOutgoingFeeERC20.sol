// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

// NON-PRODUCTION ADVERSARIAL TEST TOKEN: transferFrom is exact while direct
// transfer taxes the recipient, exercising fail-closed outgoing checks.
import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract ModelOutgoingFeeERC20 is ERC20 {
    constructor() ERC20("Model Outgoing Fee Token", "MOFEE") {}

    function mint(address to, uint256 amount) external {
        _mint(to, amount);
    }

    function transfer(address to, uint256 amount) public override returns (bool) {
        uint256 fee = amount / 50;
        _transfer(_msgSender(), to, amount - fee);
        if (fee != 0) _burn(_msgSender(), fee);
        return true;
    }
}

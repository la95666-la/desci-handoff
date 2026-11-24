// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

/**
 * @notice Adversarial test token that can reduce an account balance after deposit.
 */
contract MockNegativeRebaseToken is ERC20 {
    constructor() ERC20("Negative Rebase Test Token", "NRT") {
        _mint(msg.sender, 1_000_000 ether);
    }

    function slash(address account, uint256 amount) external {
        _burn(account, amount);
    }
}

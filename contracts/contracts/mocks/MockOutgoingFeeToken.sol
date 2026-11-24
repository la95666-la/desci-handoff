// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

/**
 * @notice Adversarial test token: transferFrom is exact, but direct transfer burns 2%.
 * This models a token that passes deposit checks and taxes refunds/claims/withdrawals.
 */
contract MockOutgoingFeeToken is ERC20 {
    uint8 private immutable _configuredDecimals;

    constructor(string memory name_, string memory symbol_, uint8 decimals_)
        ERC20(name_, symbol_)
    {
        _configuredDecimals = decimals_;
        _mint(msg.sender, 1_000_000 * 10 ** uint256(decimals_));
    }

    function decimals() public view override returns (uint8) {
        return _configuredDecimals;
    }

    function mint(address to, uint256 amount) external {
        _mint(to, amount);
    }

    function transfer(address to, uint256 amount) public override returns (bool) {
        uint256 fee = (amount * 2) / 100;
        _transfer(_msgSender(), to, amount - fee);
        if (fee > 0) _burn(_msgSender(), fee);
        return true;
    }
}

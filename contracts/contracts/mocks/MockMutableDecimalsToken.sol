// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

/**
 * @notice Test-only token used to prove whitelist decimals cannot drift after registration.
 */
contract MockMutableDecimalsToken is ERC20 {
    uint8 private _configuredDecimals;

    constructor(uint8 initialDecimals) ERC20("Mutable Decimals Token", "MDT") {
        _configuredDecimals = initialDecimals;
        _mint(msg.sender, 1_000_000 * 10 ** uint256(initialDecimals));
    }

    function decimals() public view override returns (uint8) {
        return _configuredDecimals;
    }

    function setDecimals(uint8 newDecimals) external {
        _configuredDecimals = newDecimals;
    }
}

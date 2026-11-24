// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

/**
 * @title MockERC20
 * @dev 用于测试的标准ERC20代币（18 decimals）
 */
contract MockERC20 is ERC20 {
    constructor(
        string memory name,
        string memory symbol,
        uint256 initialSupply
    ) ERC20(name, symbol) {
        _mint(msg.sender, initialSupply * 10 ** decimals());
    }

    /**
     * @dev 允许任何人铸造代币（仅用于测试）
     */
    function mint(address to, uint256 amount) public {
        _mint(to, amount);
    }

    /**
     * @dev 允许任何人销毁代币（仅用于测试）
     */
    function burn(address from, uint256 amount) public {
        _burn(from, amount);
    }

    /**
     * @dev 返回代币的小数位数（18）
     */
    function decimals() public view override returns (uint8) {
        return 18;
    }
}

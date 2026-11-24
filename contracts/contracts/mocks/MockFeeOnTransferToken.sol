// SPDX-License-Identifier: MIT
pragma solidity ^0.8.22;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

/**
 * @title MockFeeOnTransferToken
 * @dev 带转账税的ERC20代币（用于测试fee-on-transfer防御）
 * 每次转账收取2%的手续费
 */
contract MockFeeOnTransferToken is ERC20 {
    uint256 public constant FEE_PERCENT = 2; // 2%转账税

    constructor(uint256 initialSupply)
        ERC20("Fee On Transfer Token", "FOT")
    {
        _mint(msg.sender, initialSupply * 10 ** decimals());
    }

    /**
     * @dev 重写transfer函数，添加转账税
     */
    function transfer(address to, uint256 amount)
        public
        override
        returns (bool)
    {
        uint256 fee = (amount * FEE_PERCENT) / 100;
        uint256 amountAfterFee = amount - fee;

        // 转账扣费后的金额给接收者
        _transfer(_msgSender(), to, amountAfterFee);

        // 转账费用给合约（销毁）
        _burn(_msgSender(), fee);

        return true;
    }

    /**
     * @dev 重写transferFrom函数，添加转账税
     */
    function transferFrom(
        address from,
        address to,
        uint256 amount
    ) public override returns (bool) {
        uint256 fee = (amount * FEE_PERCENT) / 100;
        uint256 amountAfterFee = amount - fee;

        // 检查授权额度
        uint256 currentAllowance = allowance(from, _msgSender());
        require(
            currentAllowance >= amount,
            "ERC20: insufficient allowance"
        );

        // 减少授权额度
        _approve(from, _msgSender(), currentAllowance - amount);

        // 转账扣费后的金额给接收者
        _transfer(from, to, amountAfterFee);

        // 转账费用给合约（销毁）
        _burn(from, fee);

        return true;
    }

    /**
     * @dev 允许任何人铸造代币（仅用于测试）
     */
    function mint(address to, uint256 amount) public {
        _mint(to, amount);
    }

    /**
     * @dev 返回代币的小数位数（18）
     */
    function decimals() public view override returns (uint8) {
        return 18;
    }
}

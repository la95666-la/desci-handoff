// SPDX-License-Identifier: MIT
pragma solidity 0.8.30;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";

/**
 * @title IJoinCareCheckIn
 * @dev JoinCare task center check-in interface.
 */
interface IJoinCareCheckIn {
    event UserCheckedIn(
        address indexed user,
        uint256 indexed day,
        uint256 timestamp
    );

    event TokensRescued(address indexed token, address indexed to, uint256 amount);

    function initialize() external;

    function checkIn() external;

    function hasCheckedInToday(address user) external view returns (bool);

    function pause() external;

    function unpause() external;

    function rescueERC20(IERC20 token, address to, uint256 amount) external;

    function checkedDay(address user, uint256 day) external view returns (bool);
}

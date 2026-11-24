// SPDX-License-Identifier: MIT
pragma solidity 0.8.22;
import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
contract FairLaunchTokenMock is ERC20 { uint8 private _d; constructor(string memory n,string memory s,uint8 d) ERC20(n,s){_d=d;} function decimals() public view override returns(uint8){return _d;} function setDecimals(uint8 d) external {_d=d;} function mint(address a,uint256 v) external {_mint(a,v);} }

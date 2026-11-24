// SPDX-License-Identifier: MIT
pragma solidity 0.8.22;
contract FairLaunchStaticcallAdversary {
 enum Mode { Normal, RevertCall, ShortReturn, OversizedDecimals }
 Mode public mode; uint256 public balanceValue;
 function setMode(Mode value) external { mode=value; }
 function setBalance(uint256 value) external { balanceValue=value; }
 function decimals() external view returns(uint256){if(mode==Mode.RevertCall)revert();if(mode==Mode.ShortReturn){assembly{return(0,1)}}if(mode==Mode.OversizedDecimals)return 256;return 18;}
 function balanceOf(address) external view returns(uint256){if(mode==Mode.RevertCall)revert();if(mode==Mode.ShortReturn){assembly{return(0,1)}}return balanceValue;}
}

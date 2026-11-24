// SPDX-License-Identifier: MIT
pragma solidity 0.8.22;
library JoinCareFairLaunchV1Runtime {
 error TokenHasNoCode(address token); error TokenRuntimeMismatch(address token,bytes32 expected,bytes32 actual); error TokenDecimalsChanged(address token,uint8 expected,uint8 actual); error TokenReserveInsolvent(address token,uint256 balance,uint256 reserved);
 function codeHash(address token) external view returns(bytes32 h){assembly{h:=extcodehash(token)}}
 function validateRuntime(address token,bytes32 expected,uint8 decimals_) external view {bytes32 a;assembly{a:=extcodehash(token)}if(a==0)revert TokenHasNoCode(token);if(a!=expected)revert TokenRuntimeMismatch(token,expected,a);(bool ok,bytes memory o)=token.staticcall(abi.encodeWithSelector(0x313ce567));if(!ok||o.length!=32)revert TokenDecimalsChanged(token,decimals_,type(uint8).max);uint256 d=abi.decode(o,(uint256));if(d>255||uint8(d)!=decimals_)revert TokenDecimalsChanged(token,decimals_,d>255?255:uint8(d));}
 function runtimeMatches(address token,bytes32 expected,uint8 decimals_) external view returns(bool){bytes32 a;assembly{a:=extcodehash(token)}if(a==0||a!=expected)return false;(bool ok,bytes memory o)=token.staticcall(abi.encodeWithSelector(0x313ce567));if(!ok||o.length!=32)return false;uint256 d=abi.decode(o,(uint256));return d<=255&&uint8(d)==decimals_;}
 function balance(address token,address account) external view returns(uint256 v){(bool ok,bytes memory o)=token.staticcall(abi.encodeWithSelector(0x70a08231,account));if(!ok||o.length!=32)revert TokenReserveInsolvent(token,0,1);v=abi.decode(o,(uint256));}
 function validateSolvent(address token,address account,uint256 reserved) external view returns(uint256 v){(bool ok,bytes memory o)=token.staticcall(abi.encodeWithSelector(0x70a08231,account));if(!ok||o.length!=32)revert TokenReserveInsolvent(token,0,reserved);v=abi.decode(o,(uint256));if(v<reserved)revert TokenReserveInsolvent(token,v,reserved);}
}

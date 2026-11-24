// SPDX-License-Identifier: MIT
pragma solidity 0.8.22;
import {JoinCareFairLaunchV1Shared} from "./JoinCareFairLaunchV1Shared.sol";
import {JoinCareFairLaunchV1Logic as Logic} from "../libraries/JoinCareFairLaunchV1Logic.sol";
import {JoinCareFairLaunchV1Runtime as Runtime} from "../libraries/JoinCareFairLaunchV1Runtime.sol";
contract JoinCareFairLaunchV1ViewsFacet is JoinCareFairLaunchV1Shared {
    function projectExists(uint256 projectId) external view returns (bool) { return _getFairLaunchStorage().projects[projectId].projectId == projectId && projectId != 0; }
    function getProject(uint256 projectId) external view returns (Project memory) { return _project(_getFairLaunchStorage(),projectId); }
    function getPosition(uint256 projectId,address user) external view returns (UserPosition memory) { _project(_getFairLaunchStorage(),projectId); return _getFairLaunchStorage().positions[projectId][user]; }
    function getVestingSchedule(uint256 projectId) external view returns(uint64[] memory times,uint16[] memory bps){FairLaunchStorage storage $=_getFairLaunchStorage();_project($,projectId);return($.vestingTimes[projectId],$.vestingBps[projectId]);}
    function calculateFinalAllocation(uint256 id,address user) public view returns(uint256){FairLaunchStorage storage $=_getFairLaunchStorage();Project storage p=_project($,id);if(p.status!=ProjectStatus.Successful||p.finalTotalRaised==0)return 0;return Logic.allocation($.positions[id][user].contributionAmount,p.saleTokenAmount,p.finalTotalRaised);}
    function calculateClaimable(uint256 id,address user) public view returns(uint256){FairLaunchStorage storage $=_getFairLaunchStorage();Project storage p=_project($,id);if(p.status!=ProjectStatus.Successful)return 0;uint256 unlocked;uint64[] storage times=$.vestingTimes[id];uint16[] storage bps=$.vestingBps[id];for(uint256 i;i<times.length;++i){if(block.timestamp<times[i])break;unlocked+=bps[i];}uint256 vested=Logic.proportional(calculateFinalAllocation(id,user),unlocked);uint256 claimed=$.positions[id][user].claimedTokens;return vested>claimed?vested-claimed:0;}
    function isClaimsPaused() public view returns(bool){return block.timestamp<_getFairLaunchStorage().claimsPauseUntil;}
    function projectTokenRuntimeMatches(uint256 id) external view returns(bool){FairLaunchStorage storage $=_getFairLaunchStorage();return _projectRuntimeMatches($,_project($,id));}
    function paymentTokenRuntimeMatches(uint256 id) external view returns(bool){FairLaunchStorage storage $=_getFairLaunchStorage();Project storage p=_project($,id);return _paymentRuntimeMatches($,p);}
    function contractVersion() external pure returns(bytes32){return keccak256("fair_launch_v1");}
    function operator() external view returns(address){return _getFairLaunchStorage().operator;}
    function pauseGuardian() external view returns(address){return _getFairLaunchStorage().pauseGuardian;}
    function unpauseTimelock() external view returns(address){return _getFairLaunchStorage().unpauseTimelock;}
    function treasury() external view returns(address){return _getFairLaunchStorage().treasury;}
    function platformFeeConfig() external view returns(uint16 feeBps,address receiver){FairLaunchStorage storage $=_getFairLaunchStorage();return($.platformFeeBps,$.platformFeeReceiver);}
    function paymentTokenAdmission(address token) external view returns(PaymentTokenAdmission memory){return _getFairLaunchStorage().paymentTokenAdmissions[token];}
    function paymentTokenPolicyRevision(address token,uint32 policyVersion) external view returns(PaymentTokenPolicyRevision memory){return _getFairLaunchStorage().paymentTokenPolicyRevisions[token][policyVersion];}
    function projectTokenPolicy(address token) external view returns(ProjectTokenPolicy memory){return _getFairLaunchStorage().projectTokenPolicies[token];}
    function reservedBalance(address token) external view returns(uint256){return _getFairLaunchStorage().reservedBalances[token];}
    function beneficiarySafeConfig(address account) external view returns(bool approved,bytes32 attestationDigest,uint32 attestationVersion){FairLaunchStorage storage $=_getFairLaunchStorage();return($.approvedBeneficiarySafes[account],$.beneficiarySafeAttestations[account],$.beneficiarySafeAttestationVersions[account]);}
    function releasePolicyConfig(bytes32 policyDigest) external view returns(bool enabled,uint32 policyVersion){FairLaunchStorage storage $=_getFairLaunchStorage();return($.releasePolicies[policyDigest],$.releasePolicyVersions[policyDigest]);}
}

// Frozen against JoinCareIDOv3.getProject/getWhitelistPolicy; never reuse V2 ABI.
export const IDO_V3_ABI = [
  'function contractVersion() view returns (bytes32)',
  'function getWhitelistPolicy(uint256 projectId) view returns (tuple(bool enabled,bytes32 root,uint64 version))',
  'function getProject(uint256 projectId) view returns (tuple(uint256 projectId,address projectToken,address paymentToken,address projectOwner,address feeReceiver,uint8 projectTokenDecimals,uint8 paymentTokenDecimals,uint16 successFeeRate,uint8 status,uint256 tokenPrice,uint256 hardCap,uint256 softCap,uint256 maxPerUser,uint256 startTime,uint256 endTime,uint256 totalRaised,uint256 totalTokensSold,uint256 totalTokensDeposited,bool fundsWithdrawn,bool platformFeeWithdrawn,bool projectTokensWithdrawn,uint256[] vestingTimes,uint256[] vestingPercents))',
  'function getUserPurchase(uint256 projectId,address user) view returns (tuple(uint256 paymentAmount,uint256 tokensBought,uint256 claimedTokens,bool refunded))',
  'function calculateClaimable(uint256 projectId,address user) view returns (uint256)',
  'function purchase(uint256 projectId,uint256 paymentAmount)',
  'function purchaseWithProof(uint256 projectId,uint256 paymentAmount,uint64 expectedVersion,bytes32[] proof)',
  'function claim(uint256 projectId)',
  'function refund(uint256 projectId)',
  'function depositProjectTokens(uint256 projectId,uint256 amount)',
  'function withdrawProjectTokens(uint256 projectId)',
  'function withdrawRaisedFunds(uint256 projectId)',
]

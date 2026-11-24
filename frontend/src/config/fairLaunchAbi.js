// Generated from the reviewer-owned Fair Launch candidate ABI baseline.
// No deployment address belongs in this module: runtime identity always comes from Backend.
export const FAIR_LAUNCH_V1_ABI = [
  'function getProject(uint256 projectId) view returns (tuple(uint256 projectId,address projectToken,address paymentToken,address projectOwner,address projectProceedsReceiver,bytes32 projectProceedsReceiverAttestationDigest,uint32 projectProceedsReceiverAttestationVersion,address platformFeeReceiver,bytes32 platformFeeReceiverAttestationDigest,uint32 platformFeeReceiverAttestationVersion,uint8 projectTokenDecimals,uint8 paymentTokenDecimals,uint16 platformFeeBps,uint8 status,uint8 terminalReason,uint32 projectTokenPolicyVersion,uint32 paymentTokenPolicyVersion,uint64 startTime,uint64 endTime,uint256 saleTokenAmount,uint256 softCap,uint256 minContribution,uint256 maxContribution,uint256 totalRaised,uint256 totalTokensDeposited,uint256 totalTokensClaimed,uint256 participantCount,uint256 finalTotalRaised,uint256 finalPriceNumerator,uint256 finalPriceDenominator,uint256 settlementBlockNumber,uint256 settlementAnchorBlockNumber,bytes32 settlementAnchorBlockHash,bytes32 settlementTxHash,uint32 settlementTxLogOrdinal,bytes32 releaseManifestDigest,bytes32 projectTokenCodeHash,bytes32 projectTokenPolicyDigest,bytes32 paymentTokenCodeHash,bytes32 paymentTokenPolicyDigest,bytes32 releasePolicyDigest,uint32 releasePolicyVersion,bool projectProceedsWithdrawn,bool platformFeeWithdrawn,bool failedProjectTokensWithdrawn))',
  'function getPosition(uint256 projectId,address user) view returns (tuple(uint256 contributionAmount,uint256 claimedTokens,bool refunded))',
  'function calculateFinalAllocation(uint256 projectId,address user) view returns (uint256)',
  'function calculateClaimable(uint256 projectId,address user) view returns (uint256)',
  'function projectTokenRuntimeMatches(uint256 projectId) view returns (bool)',
  'function paymentTokenRuntimeMatches(uint256 projectId) view returns (bool)',
  'function contribute(uint256 projectId,uint256 paymentAmount)',
  'function claim(uint256 projectId)',
  'function claimFor(uint256 projectId,address user)',
  'function refund(uint256 projectId)',
  'function refundFor(uint256 projectId,address user)',
  'function depositProjectTokens(uint256 projectId,uint256 amount)',
  'function withdrawProjectProceeds(uint256 projectId)',
  'function withdrawPlatformFee(uint256 projectId)',
  'function withdrawFailedProjectTokens(uint256 projectId)',
]

export const FAIR_LAUNCH_STATUS = Object.freeze({
  PENDING: 0,
  ACTIVE: 1,
  SUCCESSFUL_PENDING_RELEASE: 2,
  SUCCESSFUL: 3,
  FAILED: 4,
  CANCELLED: 5,
})

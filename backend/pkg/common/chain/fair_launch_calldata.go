package chain

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// FairLaunchCreateProjectParams mirrors the frozen aggregate proxy ABI tuple.
// The backend always targets the proxy; dispatcher/facet addresses are metadata only.
type FairLaunchCreateProjectParams struct {
	ProjectID                                     uint64
	ProjectToken                                  common.Address
	PaymentToken                                  common.Address
	ProjectOwner                                  common.Address
	ProjectProceedsReceiver                       common.Address
	ProjectProceedsReceiverAttestationDigest      common.Hash
	ProjectProceedsReceiverAttestationVersion     uint32
	ExpectedPlatformFeeReceiver                   common.Address
	ExpectedPlatformFeeReceiverAttestationDigest  common.Hash
	ExpectedPlatformFeeReceiverAttestationVersion uint32
	ExpectedPlatformFeeBps                        uint16
	SaleTokenAmount                               *big.Int
	SoftCap                                       *big.Int
	MinContribution                               *big.Int
	MaxContribution                               *big.Int
	StartTime                                     uint64
	EndTime                                       uint64
	ExpectedProjectTokenPolicyDigest              common.Hash
	ExpectedProjectTokenPolicyVersion             uint32
	ExpectedPaymentTokenPolicyDigest              common.Hash
	ExpectedPaymentTokenPolicyVersion             uint32
	ExpectedReleasePolicyDigest                   common.Hash
	ExpectedReleasePolicyVersion                  uint32
}

type FairLaunchProjectTokenPolicy struct {
	EnabledForNewProjects bool
	EmergencyBlocked      bool
	Decimals              uint8
	PolicyVersion         uint32
	RuntimeCodeHash       common.Hash
	PolicyDigest          common.Hash
}
type FairLaunchPaymentTokenPolicyRevision struct {
	Exists          bool
	Decimals        uint8
	PolicyVersion   uint32
	RuntimeCodeHash common.Hash
	PolicyDigest    common.Hash
}

func PackFairLaunchCreate(params FairLaunchCreateProjectParams, vestingTimes []uint64, vestingBps []uint16) ([]byte, error) {
	if params.ProjectID == 0 || params.ProjectToken == (common.Address{}) || params.PaymentToken == (common.Address{}) || params.ProjectOwner == (common.Address{}) || params.ProjectProceedsReceiver == (common.Address{}) || params.ExpectedPlatformFeeReceiver == (common.Address{}) || params.SaleTokenAmount == nil || params.SaleTokenAmount.Sign() <= 0 || params.SoftCap == nil || params.SoftCap.Sign() <= 0 || params.MinContribution == nil || params.MinContribution.Sign() <= 0 || params.MaxContribution == nil || params.MaxContribution.Sign() < 0 || params.StartTime == 0 || params.EndTime <= params.StartTime || len(vestingTimes) == 0 || len(vestingTimes) != len(vestingBps) {
		return nil, errors.New("invalid fair launch create parameters")
	}
	return fairLaunchV1ABI.Pack("createProject", params, vestingTimes, vestingBps)
}
func PackFairLaunchRelease(projectID, settlementBlock, anchorBlock *big.Int, anchorHash, manifestDigest common.Hash, manifestVersion uint32, policyDigest common.Hash) ([]byte, error) {
	return fairLaunchV1ABI.Pack("releaseSuccessfulProject", projectID, settlementBlock, anchorBlock, anchorHash, manifestDigest, manifestVersion, policyDigest)
}
func PackFairLaunchWithdrawFailedProjectTokens(projectID *big.Int) ([]byte, error) {
	return fairLaunchV1ABI.Pack("withdrawFailedProjectTokens", projectID)
}
func PackFairLaunchConfigureBeneficiarySafe(a common.Address, approved bool, digest common.Hash, version uint32) ([]byte, error) {
	return fairLaunchV1ABI.Pack("configureBeneficiarySafe", a, approved, digest, version)
}
func PackFairLaunchConfigureProjectToken(a common.Address, p FairLaunchProjectTokenPolicy) ([]byte, error) {
	return fairLaunchV1ABI.Pack("configureProjectToken", a, p)
}
func PackFairLaunchConfigureReleasePolicy(d common.Hash, v uint32, enabled bool) ([]byte, error) {
	return fairLaunchV1ABI.Pack("configureReleasePolicy", d, v, enabled)
}
func PackFairLaunchInvalidatePaymentToken(projectID *big.Int, reason, evidence common.Hash) ([]byte, error) {
	return fairLaunchV1ABI.Pack("invalidatePendingReleaseForPaymentToken", projectID, reason, evidence)
}
func PackFairLaunchInvalidateProject(projectID *big.Int, class uint8, reason, evidence common.Hash) ([]byte, error) {
	return fairLaunchV1ABI.Pack("invalidatePendingRelease", projectID, class, reason, evidence)
}
func PackFairLaunchRegisterPaymentTokenPolicy(a common.Address, p FairLaunchPaymentTokenPolicyRevision) ([]byte, error) {
	return fairLaunchV1ABI.Pack("registerPaymentTokenPolicyRevision", a, p)
}
func PackFairLaunchSetOperator(a common.Address) ([]byte, error) {
	return fairLaunchV1ABI.Pack("setOperator", a)
}
func PackFairLaunchSetPaymentTokenAdmission(a common.Address, v uint32, approved bool) ([]byte, error) {
	return fairLaunchV1ABI.Pack("setPaymentTokenAdmission", a, v, approved)
}
func PackFairLaunchSetPaymentTokenEmergencyBlocked(a common.Address, blocked bool) ([]byte, error) {
	return fairLaunchV1ABI.Pack("setPaymentTokenEmergencyBlocked", a, blocked)
}
func PackFairLaunchSetPlatformFee(bps uint16, receiver common.Address) ([]byte, error) {
	return fairLaunchV1ABI.Pack("setPlatformFee", bps, receiver)
}
func PackFairLaunchRecoverExcessTokens(token common.Address, amount *big.Int) ([]byte, error) {
	return fairLaunchV1ABI.Pack("recoverExcessTokens", token, amount)
}
func PackFairLaunchPauseClaims(until uint64, reason, evidence common.Hash) ([]byte, error) {
	return fairLaunchV1ABI.Pack("pauseClaims", until, reason, evidence)
}
func PackFairLaunchPauseContributions(reason, evidence common.Hash) ([]byte, error) {
	return fairLaunchV1ABI.Pack("pauseContributions", reason, evidence)
}
func PackFairLaunchPausePayouts(reason, evidence common.Hash) ([]byte, error) {
	return fairLaunchV1ABI.Pack("pausePayouts", reason, evidence)
}
func PackFairLaunchUnpauseClaims(evidence common.Hash) ([]byte, error) {
	return fairLaunchV1ABI.Pack("unpauseClaims", evidence)
}
func PackFairLaunchUnpauseContributions(evidence common.Hash) ([]byte, error) {
	return fairLaunchV1ABI.Pack("unpauseContributions", evidence)
}
func PackFairLaunchUnpausePayouts(evidence common.Hash) ([]byte, error) {
	return fairLaunchV1ABI.Pack("unpausePayouts", evidence)
}
func PackFairLaunchUpgrade(newImplementation common.Address, data []byte) ([]byte, error) {
	return fairLaunchV1ABI.Pack("upgradeToAndCall", newImplementation, data)
}

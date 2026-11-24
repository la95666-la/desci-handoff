package chain

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"joincare_server/pkg/common/idov3whitelist"
)

// V3LifecycleRole describes the contract caller, not a verified signer. A
// preview never chooses a wallet, grants an Admin permission, builds an
// outbox action, or submits a Safe proposal.
type V3LifecycleRole uint8

const (
	V3Operator V3LifecycleRole = iota + 1
	V3OwnerSafe
	V3ProjectOwner
	V3PublicCaller
)

// V3CallerIdentity is an independently reviewed, versioned expectation, NOT
// authority derived from a log, a candidate row or a role label. Callers must
// verify owner/operator on chain and signer custody again at execution time.
type V3CallerIdentity struct {
	Operator  common.Address
	OwnerSafe common.Address
}

func matchV3Caller(role V3LifecycleRole, caller common.Address, projectOwner common.Address, expected V3CallerIdentity, proxy common.Address) bool {
	if caller == (common.Address{}) || caller == proxy {
		return false
	}
	if (role == V3Operator || role == V3OwnerSafe) && (expected.Operator == (common.Address{}) || expected.OwnerSafe == (common.Address{}) ||
		expected.Operator == expected.OwnerSafe || expected.Operator == proxy || expected.OwnerSafe == proxy) {
		return false
	}
	switch role {
	case V3Operator:
		return caller == expected.Operator
	case V3OwnerSafe:
		return caller == expected.OwnerSafe
	case V3ProjectOwner:
		return projectOwner != (common.Address{}) && projectOwner != proxy && caller == projectOwner
	case V3PublicCaller:
		return true // only methods without a contract role guard may accept this
	default:
		return false
	}
}

// PreviewV3LifecycleCall only checks ABI, observed state and the on-chain
// caller role of an independently admitted V3 project. Project-owner actions
// additionally compare the proposed caller address with the pinned owner.
// The caller must separately authenticate the binding/runtime, the fixed block, custody and
// permissions at execution time. This value must not be sent to V2 outbox.
func PreviewV3LifecycleCall(binding idov3whitelist.ExpectedProjectBinding, observed idov3whitelist.ProjectIdentityObservation, method string, caller common.Address, role V3LifecycleRole, expected V3CallerIdentity) ([]byte, error) {
	if idov3whitelist.MatchProjectIdentityObservation(binding, observed) != nil ||
		observed.ProjectOwner == (common.Address{}) || observed.FeeReceiver == (common.Address{}) ||
		observed.ProjectOwner == binding.ProxyAddress || observed.FeeReceiver == binding.ProxyAddress ||
		!matchV3Caller(role, caller, observed.ProjectOwner, expected, binding.ProxyAddress) {
		return nil, errors.New("V3 lifecycle project observation mismatched")
	}
	allowed := false
	switch method {
	case "activateProject":
		allowed = observed.State == idov3whitelist.ProjectPending && (role == V3Operator || role == V3OwnerSafe)
	case "cancelProject":
		allowed = (observed.State == idov3whitelist.ProjectPending && (role == V3ProjectOwner || role == V3OwnerSafe)) ||
			(observed.State == idov3whitelist.ProjectActive && role == V3OwnerSafe)
	case "endProject":
		// endProject has no caller-role modifier; a known owner/operator
		// remains a valid caller. Time and pause checks are NOT established
		// by this state-only preview and must be simulated at execution.
		allowed = observed.State == idov3whitelist.ProjectActive && validV3Role(role)
	case "withdrawRaisedFunds", "withdrawProjectTokens":
		allowed = role == V3ProjectOwner && (observed.State == idov3whitelist.ProjectSuccessful ||
			(method == "withdrawProjectTokens" && (observed.State == idov3whitelist.ProjectFailed || observed.State == idov3whitelist.ProjectCancelled)))
	case "withdrawPlatformFee":
		// Anyone may trigger the fee transfer, but the contract always pays
		// its stored feeReceiver, never the transaction sender.
		allowed = observed.State == idov3whitelist.ProjectSuccessful && validV3Role(role)
	}
	if !allowed {
		return nil, errors.New("V3 lifecycle state or caller role invalid")
	}
	return PackV3Lifecycle(method, new(big.Int).SetUint64(binding.ProjectID))
}

func validV3Role(role V3LifecycleRole) bool {
	return role >= V3Operator && role <= V3PublicCaller
}

// PreviewV3CreateCall binds the already prepared V3 Set parameters to an
// independently approved project/runtime identity and an explicit operator
// or owner contract caller. It intentionally does not read a DB candidate,
// create a durable intent, or unlock the existing V2 create route.
func PreviewV3CreateCall(binding idov3whitelist.ExpectedProjectBinding, params V3CreateParams, caller common.Address, role V3LifecycleRole, expected V3CallerIdentity) ([]byte, error) {
	if binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) ||
		binding.PaymentToken == (common.Address{}) || binding.ProjectToken == (common.Address{}) ||
		binding.ProxyAddress == binding.PaymentToken || binding.ProxyAddress == binding.ProjectToken || binding.PaymentToken == binding.ProjectToken ||
		params.ProjectID == nil || !params.ProjectID.IsUint64() || params.ProjectID.Uint64() != binding.ProjectID ||
		params.PaymentToken != binding.PaymentToken || params.ProjectToken != binding.ProjectToken ||
		params.ProjectOwner == binding.ProxyAddress || params.FeeReceiver == binding.ProxyAddress ||
		(role != V3Operator && role != V3OwnerSafe) ||
		!matchV3Caller(role, caller, params.ProjectOwner, expected, binding.ProxyAddress) {
		return nil, errors.New("V3 create identity or caller role invalid")
	}
	return PackV3Create(params)
}

// PreviewV3ProjectDepositCall compares the separately authenticated wallet
// caller against the observed project owner. It neither authenticates that
// wallet nor approves a token, and must not be queued on the shared
// relayer/owner-Safe outbox. A successful fixed-block read is not a substitute
// for execution-time state simulation.
func PreviewV3ProjectDepositCall(binding idov3whitelist.ExpectedProjectBinding, observed idov3whitelist.ProjectIdentityObservation,
	caller common.Address, amount *big.Int, role V3LifecycleRole) ([]byte, error) {
	if idov3whitelist.MatchProjectIdentityObservation(binding, observed) != nil ||
		observed.State != idov3whitelist.ProjectPending || role != V3ProjectOwner ||
		observed.ProjectOwner == (common.Address{}) || observed.ProjectOwner == binding.ProxyAddress ||
		observed.ProjectOwner == binding.ProjectToken || observed.ProjectOwner == binding.PaymentToken || caller != observed.ProjectOwner {
		return nil, errors.New("V3 project deposit identity, state or caller role invalid")
	}
	return PackV3ProjectDeposit(new(big.Int).SetUint64(binding.ProjectID), amount)
}

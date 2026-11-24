package chain

import (
	"bytes"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// Dedicated V3 ABI: NEVER borrow the V2 Client/ABI or its configured proxy.
// The packed bytes are inputs to an independently authorized durable action,
// not permission to sign, broadcast or present a Safe signature.
const idoV3LifecycleABIJSON = `[
	 {"type":"function","name":"createProject","inputs":[{"name":"projectId","type":"uint256"},{"name":"projectToken","type":"address"},{"name":"paymentToken","type":"address"},{"name":"projectOwner","type":"address"},{"name":"expectedFeeReceiver","type":"address"},{"name":"expectedSuccessFeeRate","type":"uint16"},{"name":"tokenPrice","type":"uint256"},{"name":"hardCap","type":"uint256"},{"name":"softCap","type":"uint256"},{"name":"maxPerUser","type":"uint256"},{"name":"startTime","type":"uint256"},{"name":"endTime","type":"uint256"},{"name":"vestingTimes","type":"uint256[]"},{"name":"vestingPercents","type":"uint256[]"}]},
	 {"type":"function","name":"activateProject","inputs":[{"name":"projectId","type":"uint256"}]},
	 {"type":"function","name":"depositProjectTokens","inputs":[{"name":"projectId","type":"uint256"},{"name":"amount","type":"uint256"}]},
	 {"type":"function","name":"cancelProject","inputs":[{"name":"projectId","type":"uint256"}]},
	 {"type":"function","name":"endProject","inputs":[{"name":"projectId","type":"uint256"}]},
	 {"type":"function","name":"withdrawRaisedFunds","inputs":[{"name":"projectId","type":"uint256"}]},
	 {"type":"function","name":"withdrawPlatformFee","inputs":[{"name":"projectId","type":"uint256"}]},
	 {"type":"function","name":"withdrawProjectTokens","inputs":[{"name":"projectId","type":"uint256"}]}
	]`

var idoV3LifecycleABI = func() abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(idoV3LifecycleABIJSON))
	if err != nil {
		panic("invalid embedded V3 lifecycle ABI")
	}
	return parsed
}()

type V3CreateParams struct {
	ProjectID                                             *big.Int
	ProjectToken, PaymentToken, ProjectOwner, FeeReceiver common.Address
	FeeRate                                               uint16
	Price, HardCap, SoftCap, MaxPerUser, Start, End       *big.Int
	VestingTimes, VestingPercents                         []*big.Int
}

// PackV3ProjectDeposit is a project-owner wallet call, not an operator,
// relayer or Safe outbox action. The owner must separately approve the
// project token and sign in their own wallet; this packs only isolated ABI.
func PackV3ProjectDeposit(projectID, amount *big.Int) ([]byte, error) {
	if projectID == nil || projectID.Sign() <= 0 || projectID.BitLen() > 256 ||
		amount == nil || amount.Sign() <= 0 || amount.BitLen() > 256 {
		return nil, errors.New("V3 project deposit parameters invalid")
	}
	return idoV3LifecycleABI.Pack("depositProjectTokens", projectID, amount)
}

func PackV3Create(params V3CreateParams) ([]byte, error) {
	positive := func(v *big.Int) bool { return v != nil && v.Sign() > 0 && v.BitLen() <= 256 }
	if !positive(params.ProjectID) || params.ProjectToken == (common.Address{}) || params.PaymentToken == (common.Address{}) || params.ProjectOwner == (common.Address{}) || params.FeeReceiver == (common.Address{}) ||
		params.ProjectToken == params.PaymentToken || params.FeeRate > 1000 || !positive(params.Price) || !positive(params.HardCap) || !positive(params.SoftCap) || !positive(params.MaxPerUser) || !positive(params.Start) || !positive(params.End) ||
		params.SoftCap.Cmp(params.HardCap) > 0 || params.MaxPerUser.Cmp(params.HardCap) > 0 || params.End.Cmp(params.Start) <= 0 || len(params.VestingTimes) == 0 || len(params.VestingTimes) > 10 || len(params.VestingTimes) != len(params.VestingPercents) {
		return nil, errors.New("V3 create parameters invalid")
	}
	sum := new(big.Int)
	previous := params.End
	for i := range params.VestingTimes {
		if !positive(params.VestingTimes[i]) || !positive(params.VestingPercents[i]) || params.VestingTimes[i].Cmp(previous) <= 0 || params.VestingPercents[i].Cmp(big.NewInt(10000)) > 0 {
			return nil, errors.New("V3 vesting schedule invalid")
		}
		sum.Add(sum, params.VestingPercents[i])
		previous = params.VestingTimes[i]
	}
	if sum.Cmp(big.NewInt(10000)) != 0 {
		return nil, errors.New("V3 vesting total invalid")
	}
	return idoV3LifecycleABI.Pack("createProject", params.ProjectID, params.ProjectToken, params.PaymentToken, params.ProjectOwner, params.FeeReceiver, params.FeeRate, params.Price, params.HardCap, params.SoftCap, params.MaxPerUser, params.Start, params.End, params.VestingTimes, params.VestingPercents)
}

func PackV3Lifecycle(method string, projectID *big.Int) ([]byte, error) {
	if projectID == nil || projectID.Sign() <= 0 || projectID.BitLen() > 256 {
		return nil, errors.New("V3 project ID invalid")
	}
	switch method {
	case "activateProject", "cancelProject", "endProject", "withdrawRaisedFunds", "withdrawPlatformFee", "withdrawProjectTokens":
		return idoV3LifecycleABI.Pack(method, projectID)
	default:
		return nil, errors.New("V3 lifecycle method unavailable")
	}
}

// VerifyV3ActionCall checks a persisted V3 outbox action against the dedicated
// ABI, including the exact project ID and canonical argument encoding. It is
// only a shape check: it does not admit a runtime, signer, receipt or event.
func VerifyV3ActionCall(action string, projectID uint64, data []byte) error {
	invalid := errors.New("V3 action calldata invalid")
	methods := map[string]string{
		"ido_v3_create": "createProject", "ido_v3_activate": "activateProject",
		"ido_v3_end": "endProject", "ido_v3_withdraw_platform_fee": "withdrawPlatformFee",
	}
	name, ok := methods[action]
	if !ok || projectID == 0 || len(data) < 36 {
		return invalid
	}
	method := idoV3LifecycleABI.Methods[name]
	if !bytes.Equal(data[:4], method.ID) {
		return invalid
	}
	args, err := method.Inputs.Unpack(data[4:])
	if err != nil || len(args) != len(method.Inputs) || len(args) == 0 {
		return invalid
	}
	id, ok := args[0].(*big.Int)
	if !ok || !id.IsUint64() || id.Uint64() != projectID {
		return invalid
	}
	canonical, err := idoV3LifecycleABI.Pack(name, args...)
	if err != nil || !bytes.Equal(canonical, data) {
		return invalid
	}
	return nil
}

package chaintx

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/projectruntime"
)

var fairLaunchOperatorActions = map[string]struct{}{
	constant.TransactionTypeFairLaunchCreate:   {},
	constant.TransactionTypeFairLaunchActivate: {},
	constant.TransactionTypeFairLaunchEnd:      {},
	constant.TransactionTypeFairLaunchCancel:   {},
	constant.TransactionTypeFairLaunchRelease:  {},
}

var fairLaunchWalletActions = map[string]struct{}{
	constant.TransactionTypeFairLaunchProjectProceedsWithdraw: {},
	constant.TransactionTypeFairLaunchPlatformFeeWithdraw:     {},
}

// FairLaunchIntent builds a durable intent only from the trusted runtime. It
// does not sign or broadcast and rejects governance/wallet actions from the
// Operator domain. The shared nonce fence remains keyed by chain and sender.
func FairLaunchIntent(runtime projectruntime.Runtime, projectID uint64, action string, callData []byte) (Intent, error) {
	if !runtime.Enabled {
		return Intent{}, errors.New("fair launch runtime is disabled")
	}
	if runtime.Version != constant.ContractVersionFairLaunchV1 || runtime.FundingMode != constant.FundingModeFairLaunch {
		return Intent{}, errors.New("fair launch runtime mode/version mismatch")
	}
	if projectID == 0 || len(callData) < 4 || runtime.ChainID == 0 || !common.IsHexAddress(runtime.ContractAddress) || !common.IsHexAddress(runtime.OperatorAddress) {
		return Intent{}, errors.New("fair launch intent identity is invalid")
	}
	if _, walletAction := fairLaunchWalletActions[action]; walletAction {
		return Intent{}, fmt.Errorf("fair launch action %q must be signed by its beneficiary wallet", action)
	}
	if _, operatorAction := fairLaunchOperatorActions[action]; !operatorAction {
		return Intent{}, fmt.Errorf("fair launch action %q is not an Operator durable action", action)
	}
	return Intent{
		ProjectID:       projectID,
		Action:          action,
		ChainID:         runtime.ChainID,
		SenderAddress:   common.HexToAddress(runtime.OperatorAddress),
		ContractAddress: common.HexToAddress(runtime.ContractAddress),
		CallData:        append([]byte(nil), callData...),
	}, nil
}

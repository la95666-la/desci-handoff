package idov3whitelist

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const safeExecutionABI = `[{"type":"function","name":"execTransaction","inputs":[{"name":"to","type":"address"},{"name":"value","type":"uint256"},{"name":"data","type":"bytes"},{"name":"operation","type":"uint8"},{"name":"safeTxGas","type":"uint256"},{"name":"baseGas","type":"uint256"},{"name":"gasPrice","type":"uint256"},{"name":"gasToken","type":"address"},{"name":"refundReceiver","type":"address"},{"name":"signatures","type":"bytes"}],"outputs":[{"type":"bool"}]}]`

var safeExecutionSuccess = crypto.Keccak256Hash([]byte("ExecutionSuccess(bytes32,uint256)"))

// SafeExecutionRPC cannot sign or send a transaction. The exact outer tx
// and inner calldata must be read from the chain, not an Admin submission.
type SafeExecutionRPC interface {
	EvidenceRPC
	TransactionByHash(context.Context, common.Hash) (*types.Transaction, bool, error)
}

// CheckSafePolicyExecution validates the original outer Safe tx, successful
// Safe execution log and exactly one policy event in its canonical receipt.
// Safe itself checks its signatures/nonce. This helper also checks the
// on-chain threshold at that block; custody/owner independence remains a
// separately pinned governance release decision.
func CheckSafePolicyExecution(ctx context.Context, rpc SafeExecutionRPC, scope Scope,
	governor common.Address, expected PolicyChange, txHash, safeHash common.Hash,
	receipt *types.Receipt) error {
	invalid := errors.New("V3 policy Safe execution unavailable")
	if ctx == nil || rpc == nil || validateScope(scope) != nil || governor == (common.Address{}) ||
		governor == scope.ProxyAddress || txHash == (common.Hash{}) || safeHash == (common.Hash{}) ||
		receipt == nil || receipt.TxHash != txHash || receipt.Status != types.ReceiptStatusSuccessful ||
		receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Sign() <= 0 ||
		receipt.BlockHash == (common.Hash{}) {
		return invalid
	}
	transaction, pending, err := rpc.TransactionByHash(ctx, txHash)
	if err != nil || pending || transaction == nil || transaction.Hash() != txHash ||
		transaction.To() == nil || *transaction.To() != governor || transaction.Value().Sign() != 0 ||
		transaction.ChainId().Cmp(scope.ChainID) != 0 {
		return invalid
	}
	// The outer EVM transaction must carry a valid signer on this exact
	// chain. An RPC-supplied transaction object is not authority on its own.
	if _, err := types.Sender(types.LatestSignerForChainID(scope.ChainID), transaction); err != nil {
		return invalid
	}
	parsed, err := abi.JSON(strings.NewReader(safeExecutionABI))
	if err != nil || len(transaction.Data()) < 4 || !bytes.Equal(transaction.Data()[:4], parsed.Methods["execTransaction"].ID) {
		return invalid
	}
	values, err := parsed.Methods["execTransaction"].Inputs.Unpack(transaction.Data()[4:])
	if err != nil || len(values) != 10 {
		return invalid
	}
	canonical, err := parsed.Methods["execTransaction"].Inputs.Pack(values...)
	if err != nil || !bytes.Equal(canonical, transaction.Data()[4:]) {
		return invalid
	}
	to, okTo := values[0].(common.Address)
	value, okValue := values[1].(*big.Int)
	inner, okInner := values[2].([]byte)
	operation, okOp := values[3].(uint8)
	signatures, okSignatures := values[9].([]byte)
	expectedCall, err := PackPolicyChangeCall(scope, expected)
	if err != nil || !okTo || to != scope.ProxyAddress || !okValue || value.Sign() != 0 ||
		!okInner || !bytes.Equal(inner, expectedCall) || !okOp || operation != 0 ||
		!okSignatures || len(signatures) < 130 {
		return invalid
	}
	// The Safe may use a relayer to pay for execution, but cannot be a
	// one-of-one EOA. Require the threshold on the receipt block itself.
	block := new(big.Int).Set(receipt.BlockNumber)
	thresholdCall := crypto.Keccak256([]byte("getThreshold()"))[:4]
	threshold, err := rpc.CallContract(ctx, ethereum.CallMsg{To: &governor, Data: thresholdCall}, block)
	if err != nil || len(threshold) != 32 || new(big.Int).SetBytes(threshold).Cmp(big.NewInt(2)) < 0 ||
		new(big.Int).SetBytes(threshold).Cmp(big.NewInt(3)) > 0 {
		return invalid
	}
	// Safe signature and nonce validity are proved by its successful event;
	// this does not certify independent natural persons or custody.
	var success, policy int
	for _, log := range receipt.Logs {
		if log == nil || log.Removed || log.BlockHash != receipt.BlockHash || log.TxHash != txHash ||
			log.BlockNumber != receipt.BlockNumber.Uint64() || len(log.Topics) == 0 {
			return invalid
		}
		if log.Address == governor && log.Topics[0] == safeExecutionSuccess {
			success++
			// The deployed Policy Safe indexes txHash in topic[1] and emits
			// payment in one data word. Older Safe layouts leave both words
			// unindexed. Authenticate the exact supplied Safe hash and zero
			// payment in either layout, never a topic-only match.
			indexed := len(log.Topics) == 2 && log.Topics[1] == safeHash && len(log.Data) == 32 &&
				new(big.Int).SetBytes(log.Data).Sign() == 0
			unindexed := len(log.Topics) == 1 && len(log.Data) == 64 && bytes.Equal(log.Data[:32], safeHash[:]) &&
				new(big.Int).SetBytes(log.Data[32:]).Sign() == 0
			if !indexed && !unindexed {
				return invalid
			}
		}
		if log.Address == scope.ProxyAddress && log.Topics[0] == policyUpdatedTopic &&
			len(log.Topics) > 1 && log.Topics[1] == common.BigToHash(scope.ProjectID) {
			policy++
		}
	}
	if success != 1 || policy != 1 {
		return invalid
	}
	return nil
}

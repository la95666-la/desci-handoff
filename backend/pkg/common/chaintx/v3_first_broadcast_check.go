package chaintx

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/v3diagnostic"
)

type V3FirstBroadcastRPC interface {
	ChainID(context.Context) (*big.Int, error)
	NonceAt(context.Context, common.Address, *big.Int) (uint64, error)
	PendingNonceAt(context.Context, common.Address) (uint64, error)
	BalanceAt(context.Context, common.Address, *big.Int) (*big.Int, error)
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
	TransactionByHash(context.Context, common.Hash) (*types.Transaction, bool, error)
	CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)
}

// Both independent providers must affirm absence, exact nonce, balance and
// current simulation using the persisted gas/value/data. Unknown != absent.
// This function cannot change an outbox row or invoke a send/sign RPC.
func CheckV3OriginalFirstBroadcast(ctx context.Context, row entity.ChainTxSubmission, b idov3whitelist.ExpectedProjectBinding, providers ...V3FirstBroadcastRPC) (resultErr error) {
	stage := "original_signature"
	defer func() {
		if resultErr != nil {
			resultErr = v3diagnostic.New("PREFLIGHT_REJECTED", stage, nil)
		}
	}()
	invalid := errors.New("V3 original first broadcast check unavailable")
	if ctx == nil || ctx.Err() != nil || len(providers) != 2 || row.State != "signed" || row.ActiveSlot == nil || *row.ActiveSlot != 1 || row.BroadcastCount != 0 || row.LastBroadcastAt != nil || row.ReceiptStatus != nil || row.ReceiptBlockNumber != nil || row.FinalEventRecordID != nil || row.V3SignedAt == nil || row.ReplacementGeneration != 0 || VerifyV3PersistedAppliedAttempt(row, b) != nil {
		return invalid
	}
	var signed types.Transaction
	if signed.UnmarshalBinary(row.RawTx) != nil || signed.Gas() != *row.GasLimit || signed.GasPrice().Sign() <= 0 || signed.GasPrice().String() != row.GasPriceRaw || signed.Type() != types.LegacyTxType {
		return invalid
	}
	operator := common.HexToAddress(row.SenderAddress)
	a, aOK := providers[0].(SenderNonceRPC)
	bRPC, bOK := providers[1].(SenderNonceRPC)
	if !aOK || !bOK {
		return invalid
	}
	if floor, e := ConfirmV3SenderNonce(ctx, b.ChainID, operator, signed.Nonce(), a, bRPC); e != nil || floor != signed.Nonce() {
		return invalid
	}
	for _, rpc := range providers {
		stage = "original_rpc_chain"
		if rpc == nil {
			return invalid
		}
		id, e := rpc.ChainID(ctx)
		if e != nil || id == nil || id.Cmp(new(big.Int).SetUint64(b.ChainID)) != 0 {
			return invalid
		}
		stage = "original_nonce"
		latest, e := rpc.NonceAt(ctx, operator, nil)
		if e != nil || latest != signed.Nonce() {
			return invalid
		}
		pending, e := rpc.PendingNonceAt(ctx, operator)
		if e != nil || pending != signed.Nonce() {
			return invalid
		}
		stage = "original_hash_absence"
		if receipt, e := rpc.TransactionReceipt(ctx, signed.Hash()); !errors.Is(e, ethereum.NotFound) || receipt != nil {
			return invalid
		}
		if tx, _, e := rpc.TransactionByHash(ctx, signed.Hash()); !errors.Is(e, ethereum.NotFound) || tx != nil {
			return invalid
		}
		stage = "original_balance"
		balance, e := rpc.BalanceAt(ctx, operator, nil)
		if e != nil || balance == nil || balance.Cmp(signed.Cost()) < 0 {
			return invalid
		}
		stage = "original_exact_simulation"
		result, e := rpc.CallContract(ctx, ethereum.CallMsg{From: operator, To: signed.To(), Gas: signed.Gas(), GasPrice: signed.GasPrice(), Value: signed.Value(), Data: signed.Data()}, nil)
		if e != nil || len(result) != 0 {
			return invalid
		}
	}
	return nil
}

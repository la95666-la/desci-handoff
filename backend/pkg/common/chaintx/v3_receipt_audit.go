package chaintx

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// V3ReceiptSource is deliberately read-only. It cannot send, replace, or sign
// an outbox transaction. The audit does not advance a submission or finance.
type V3ReceiptSource interface {
	ChainID(context.Context) (*big.Int, error)
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	BlockNumber(context.Context) (uint64, error)
}

type V3ReceiptAudit struct {
	Hash      common.Hash
	Nonce     uint64
	Block     uint64
	BlockHash common.Hash
	Reverted  bool
	GasUsed   uint64
}

// AuditV3PersistedReceipt is a reconcile-only building block for an already
// durably signed V3 attempt. The binding MUST come from independent signed
// admission; a submission row cannot authenticate its own proxy or version.
// Unknown RPC outcomes fail closed: NEVER resend
// this raw transaction, sign a replacement, release its nonce, or mark it
// applied based on this result. A separate authenticated event/financial
// materializer must independently match the canonical event to the intent.
func AuditV3PersistedReceipt(ctx context.Context, row entity.ChainTxSubmission, binding idov3whitelist.ExpectedProjectBinding, source V3ReceiptSource, confirmations uint64) (V3ReceiptAudit, error) {
	var empty V3ReceiptAudit
	invalid := errors.New("V3 receipt audit unavailable")
	if ctx == nil || source == nil || !IsIDOV3TransactionAction(row.Action) ||
		binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) ||
		binding.ProjectToken == (common.Address{}) || binding.PaymentToken == (common.Address{}) ||
		binding.ProjectToken == binding.PaymentToken || row.ChainID != binding.ChainID ||
		row.ProjectID != binding.ProjectID || !common.IsHexAddress(row.ContractAddress) ||
		common.HexToAddress(row.ContractAddress) != binding.ProxyAddress ||
		row.Nonce == nil || row.TxHash == nil || !validHash(*row.TxHash) ||
		len(row.RawTx) == 0 || row.ChainID == 0 || confirmations == 0 ||
		row.ActiveSlot == nil || *row.ActiveSlot != 1 {
		return empty, invalid
	}
	if basechain.VerifyV3ActionCall(row.Action, row.ProjectID, row.CallData) != nil {
		return empty, invalid
	}
	if err := validateRaw(row, row.RawTx, *row.TxHash, *row.Nonce); err != nil {
		return empty, invalid
	}
	chainID, err := source.ChainID(ctx)
	if err != nil || chainID == nil || !chainID.IsUint64() || chainID.Uint64() != row.ChainID {
		return empty, invalid
	}
	hash := common.HexToHash(*row.TxHash)
	receipt, err := source.TransactionReceipt(ctx, hash)
	if err != nil || !validV3Receipt(receipt, hash) {
		return empty, invalid
	}
	head, err := source.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(head, receipt.BlockNumber.Uint64(), confirmations) {
		return empty, invalid
	}
	block := new(big.Int).Set(receipt.BlockNumber)
	header, err := source.HeaderByNumber(ctx, block)
	if err != nil || header == nil || header.Number == nil || header.Number.Cmp(block) != 0 || header.Hash() != receipt.BlockHash {
		return empty, invalid
	}
	// A receipt/header race must not be mistaken for a final result. Fail closed
	// even if the transaction hash remains the same after a block replacement.
	again, err := source.TransactionReceipt(ctx, hash)
	if err != nil || !validV3Receipt(again, hash) || again.BlockHash != receipt.BlockHash ||
		again.BlockNumber.Cmp(block) != 0 || again.Status != receipt.Status || again.TransactionIndex != receipt.TransactionIndex {
		return empty, invalid
	}
	stable, err := source.HeaderByNumber(ctx, block)
	if err != nil || stable == nil || stable.Number == nil || stable.Number.Cmp(block) != 0 || stable.Hash() != receipt.BlockHash {
		return empty, invalid
	}
	return V3ReceiptAudit{Hash: hash, Nonce: *row.Nonce, Block: block.Uint64(),
		BlockHash: receipt.BlockHash, Reverted: receipt.Status == types.ReceiptStatusFailed,
		GasUsed: receipt.GasUsed}, nil
}

func validV3Receipt(r *types.Receipt, hash common.Hash) bool {
	return r != nil && r.TxHash == hash && r.BlockHash != (common.Hash{}) &&
		r.BlockNumber != nil && r.BlockNumber.Sign() > 0 && r.BlockNumber.IsUint64() &&
		(r.Status == types.ReceiptStatusSuccessful || r.Status == types.ReceiptStatusFailed)
}

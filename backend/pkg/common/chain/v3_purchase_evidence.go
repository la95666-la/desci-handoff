package chain

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// V3PurchaseEvidenceRPC is read-only; it cannot sign or broadcast. Its caller
// must independently authenticate the configured V3 runtime and project row.
type V3PurchaseEvidenceRPC interface {
	ChainID(context.Context) (*big.Int, error)
	BlockNumber(context.Context) (uint64, error)
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
}

// V3PurchaseObservation is transient evidence, not a durable financial fact.
// Recheck canonicality and materialization state on every reorg/replay path.
type V3PurchaseObservation struct {
	Purchase      V3PurchaseEvent
	ConfirmedHead uint64
	HeadHash      common.Hash
}

// ReconcileV3PurchaseObservation re-reads a supplied receipt and verifies its
// canonical block, configured project and token identity, receipt contents,
// confirmation depth, and stable head/receipt block hashes during this call.
// It does not establish the runtime's code identity, approve an Admin policy,
// persist a purchase, or grant a proof. minConfirmations includes the receipt
// block. Raw RPC errors never escape this boundary.
func ReconcileV3PurchaseObservation(ctx context.Context, rpc V3PurchaseEvidenceRPC, binding V3PurchaseBinding, txHash common.Hash, logIndex uint, minConfirmations uint64) (V3PurchaseObservation, error) {
	if rpc == nil || binding.ChainID == 0 || binding.ProxyAddress == (common.Address{}) || binding.ProjectID == 0 ||
		binding.PaymentToken == (common.Address{}) || binding.ProjectToken == (common.Address{}) ||
		txHash == (common.Hash{}) || minConfirmations == 0 {
		return V3PurchaseObservation{}, errors.New("invalid V3 purchase observation request")
	}
	chainID, err := rpc.ChainID(ctx)
	if err != nil || chainID == nil || !chainID.IsUint64() || chainID.Uint64() != binding.ChainID {
		return V3PurchaseObservation{}, errors.New("V3 purchase chain identity unavailable or mismatched")
	}
	receipt, err := rpc.TransactionReceipt(ctx, txHash)
	if err != nil || receipt == nil || receipt.TxHash != txHash || receipt.Status != types.ReceiptStatusSuccessful ||
		receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Sign() <= 0 || receipt.BlockHash == (common.Hash{}) {
		return V3PurchaseObservation{}, errors.New("V3 purchase receipt unavailable or unsuccessful")
	}
	block := receipt.BlockNumber.Uint64()
	head, err := rpc.BlockNumber(ctx)
	if err != nil || head < block || head-block+1 < minConfirmations {
		return V3PurchaseObservation{}, errors.New("V3 purchase confirmation depth unavailable")
	}
	blockNumber := new(big.Int).SetUint64(block)
	canonical, err := rpc.HeaderByNumber(ctx, blockNumber)
	if err != nil || canonical == nil || canonical.Number == nil || canonical.Number.Cmp(blockNumber) != 0 || canonical.Hash() != receipt.BlockHash {
		return V3PurchaseObservation{}, errors.New("V3 purchase receipt block is not canonical")
	}
	found := 0
	var event V3PurchaseEvent
	for _, log := range receipt.Logs {
		if log == nil || log.Address != binding.ProxyAddress || log.Index != logIndex {
			continue
		}
		found++
		decoded, decodeErr := DecodeV3PurchaseLog(*log, binding.ChainID, binding.ProxyAddress)
		if decodeErr != nil {
			return V3PurchaseObservation{}, errors.New("V3 purchase receipt log malformed")
		}
		event = *decoded
	}
	if found != 1 || MatchV3PurchaseReceipt(&event, binding, receipt) != nil {
		return V3PurchaseObservation{}, errors.New("V3 purchase receipt log or binding mismatch")
	}
	headNumber := new(big.Int).SetUint64(head)
	headHeader, err := rpc.HeaderByNumber(ctx, headNumber)
	if err != nil || headHeader == nil || headHeader.Number == nil || headHeader.Number.Cmp(headNumber) != 0 {
		return V3PurchaseObservation{}, errors.New("V3 purchase confirmation head unavailable")
	}
	confirmedHash := headHeader.Hash()
	// Catch in-flight replacement of either anchor. Later reorganizations must
	// be handled by the separate, still-unimplemented durable materializer.
	canonical, err = rpc.HeaderByNumber(ctx, blockNumber)
	if err != nil || canonical == nil || canonical.Number == nil || canonical.Number.Cmp(blockNumber) != 0 || canonical.Hash() != receipt.BlockHash {
		return V3PurchaseObservation{}, errors.New("V3 purchase receipt block changed during read")
	}
	headHeader, err = rpc.HeaderByNumber(ctx, headNumber)
	if err != nil || headHeader == nil || headHeader.Number == nil || headHeader.Number.Cmp(headNumber) != 0 || headHeader.Hash() != confirmedHash {
		return V3PurchaseObservation{}, errors.New("V3 purchase head changed during read")
	}
	return V3PurchaseObservation{Purchase: event, ConfirmedHead: head, HeadHash: confirmedHash}, nil
}

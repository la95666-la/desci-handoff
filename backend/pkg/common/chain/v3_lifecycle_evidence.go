package chain

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"joincare_server/pkg/common/idov3whitelist"
)

// V3LifecycleObservation is transient, read-only evidence, not a durable Job
// event, financial fact, proof, or finality certificate. Later reorgs still
// require an independent, persistent rollback/replay mechanism.
type V3LifecycleObservation struct {
	Event         V3LifecycleEvent
	ConfirmedHead uint64
	HeadHash      common.Hash
}

// ReconcileV3LifecycleObservation checks an already-known transaction on a
// read-only RPC. binding MUST come from independently authenticated V3 project
// identity, never from this event, an inert candidate row or request input.
// minConfirmations counts the receipt block. RPC errors are masked.
func ReconcileV3LifecycleObservation(ctx context.Context, rpc V3PurchaseEvidenceRPC,
	binding idov3whitelist.ExpectedProjectBinding, txHash common.Hash, logIndex uint, minConfirmations uint64,
) (V3LifecycleObservation, error) {
	var empty V3LifecycleObservation
	if rpc == nil || txHash == (common.Hash{}) || minConfirmations == 0 || binding.ChainID == 0 || binding.ProjectID == 0 ||
		binding.ProxyAddress == (common.Address{}) || binding.ProjectToken == (common.Address{}) || binding.PaymentToken == (common.Address{}) ||
		binding.ProjectToken == binding.PaymentToken {
		return empty, errors.New("V3 lifecycle observation request invalid")
	}
	chainID, err := rpc.ChainID(ctx)
	if err != nil || chainID == nil || !chainID.IsUint64() || chainID.Uint64() != binding.ChainID {
		return empty, errors.New("V3 lifecycle chain identity unavailable or mismatched")
	}
	receipt, err := rpc.TransactionReceipt(ctx, txHash)
	if err != nil || receipt == nil || receipt.TxHash != txHash || receipt.Status != types.ReceiptStatusSuccessful ||
		receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Sign() <= 0 || receipt.BlockHash == (common.Hash{}) {
		return empty, errors.New("V3 lifecycle receipt unavailable or unsuccessful")
	}
	block := receipt.BlockNumber.Uint64()
	head, err := rpc.BlockNumber(ctx)
	if err != nil || head < block || head-block+1 < minConfirmations {
		return empty, errors.New("V3 lifecycle confirmation depth unavailable")
	}
	blockNumber := new(big.Int).SetUint64(block)
	headNumber := new(big.Int).SetUint64(head)
	readAnchor := func(number *big.Int, want common.Hash) error {
		header, readErr := rpc.HeaderByNumber(ctx, number)
		if readErr != nil || header == nil || header.Number == nil || header.Number.Cmp(number) != 0 || header.Hash() != want {
			return errors.New("V3 lifecycle block anchor unavailable or changed")
		}
		return nil
	}
	if err := readAnchor(blockNumber, receipt.BlockHash); err != nil {
		return empty, err
	}
	var event *V3LifecycleEvent
	for _, log := range receipt.Logs {
		if log == nil || log.Index != logIndex {
			continue
		}
		if event != nil || log.Address != binding.ProxyAddress {
			return empty, errors.New("V3 lifecycle log index ambiguous or source mismatched")
		}
		event, err = DecodeV3LifecycleLog(*log, binding.ChainID, binding.ProxyAddress)
		if err != nil {
			return empty, errors.New("V3 lifecycle log malformed")
		}
	}
	if MatchV3LifecycleReceipt(event, binding, receipt) != nil {
		return empty, errors.New("V3 lifecycle log or binding mismatched")
	}
	confirmed, err := rpc.HeaderByNumber(ctx, headNumber)
	if err != nil || confirmed == nil || confirmed.Number == nil || confirmed.Number.Cmp(headNumber) != 0 {
		return empty, errors.New("V3 lifecycle confirmation head unavailable")
	}
	headHash := confirmed.Hash()
	// Re-read the receipt and both canonical anchors after the first checks.
	// This narrows provider inconsistency during this call, not future reorgs.
	again, err := rpc.TransactionReceipt(ctx, txHash)
	if err != nil || MatchV3LifecycleReceipt(event, binding, again) != nil {
		return empty, errors.New("V3 lifecycle receipt changed during read")
	}
	if err := readAnchor(blockNumber, receipt.BlockHash); err != nil {
		return empty, err
	}
	if err := readAnchor(headNumber, headHash); err != nil {
		return empty, err
	}
	return V3LifecycleObservation{Event: *event, ConfirmedHead: head, HeadHash: headHash}, nil
}

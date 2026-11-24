package project

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"joincare_server/pkg/common/idov3whitelist"
)

// A disabled policy is a canonical versioned event, never a Merkle snapshot.
// This read-only reconciler exists solely for recording its Safe execution;
// it cannot be used by the wallet proof path.
func v3ReconcileDisabledPolicy(ctx context.Context, rpc *ethclient.Client, scope idov3whitelist.Scope,
	runtime idov3whitelist.RuntimeIdentity, version uint64, hash common.Hash, confirmations uint64) (idov3whitelist.PolicyEvidence, error) {
	invalid := errors.New("V3 disabled policy evidence unavailable")
	if rpc == nil || ctx == nil || version == 0 || hash == (common.Hash{}) || confirmations == 0 {
		return idov3whitelist.PolicyEvidence{}, invalid
	}
	receipt, err := rpc.TransactionReceipt(ctx, hash)
	if err != nil || receipt == nil || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Sign() <= 0 || receipt.BlockHash == (common.Hash{}) {
		return idov3whitelist.PolicyEvidence{}, invalid
	}
	block := receipt.BlockNumber.Uint64()
	latest, err := rpc.BlockNumber(ctx)
	if err != nil || !idov3whitelist.HasConfirmations(latest, block, confirmations) {
		return idov3whitelist.PolicyEvidence{}, invalid
	}
	anchor, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(block))
	if err != nil || anchor == nil || anchor.Number == nil || !anchor.Number.IsUint64() || anchor.Number.Uint64() != block || anchor.Hash() != receipt.BlockHash {
		return idov3whitelist.PolicyEvidence{}, invalid
	}
	atReceipt, err := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, runtime, block, anchor.Hash())
	if err != nil || idov3whitelist.MatchDisabledPolicyReceipt(scope, version, hash, receipt, atReceipt) != nil {
		return idov3whitelist.PolicyEvidence{}, invalid
	}
	head, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(latest))
	if err != nil || head == nil || head.Number == nil || !head.Number.IsUint64() || head.Number.Uint64() != latest {
		return idov3whitelist.PolicyEvidence{}, invalid
	}
	atHead, err := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, runtime, latest, head.Hash())
	if err != nil || atHead.Policy.Enabled || atHead.Policy.Root != (common.Hash{}) || atHead.Policy.Version != version {
		return idov3whitelist.PolicyEvidence{}, invalid
	}
	for _, point := range []struct {
		number uint64
		hash   common.Hash
	}{{block, anchor.Hash()}, {latest, head.Hash()}} {
		again, e := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(point.number))
		if e != nil || again == nil || again.Hash() != point.hash {
			return idov3whitelist.PolicyEvidence{}, invalid
		}
	}
	return idov3whitelist.PolicyEvidence{ReceiptBlock: atReceipt, Head: atHead, TxHash: hash}, nil
}

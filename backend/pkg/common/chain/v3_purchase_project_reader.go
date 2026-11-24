package chain

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"joincare_server/pkg/common/idov3whitelist"
)

// V3PurchaseProjectRPC is read-only. The provider must support historical
// state reads at the receipt block; it cannot sign or submit transactions.
type V3PurchaseProjectRPC interface {
	V3PurchaseEvidenceRPC
	idov3whitelist.PolicyRPC
}

// V3PurchaseProjectEvidence is an ephemeral observation. Its project field
// carries no proof, policy or durable purchase entitlement.
type V3PurchaseProjectEvidence struct {
	Purchase V3PurchaseObservation
	Project  idov3whitelist.ProjectIdentityObservation
}

// ReconcileV3PurchaseProjectEvidence adds a historical runtime/project read to
// the successful receipt check. expected and runtime MUST come from separately
// authenticated V3 configuration, not from an event or untrusted request. A
// subsequent reorg remains possible; no scanner, ledger or proof is updated.
func ReconcileV3PurchaseProjectEvidence(ctx context.Context, rpc V3PurchaseProjectRPC,
	binding V3PurchaseBinding, expected idov3whitelist.ExpectedProjectBinding,
	runtime idov3whitelist.RuntimeIdentity, txHash common.Hash, logIndex uint, minConfirmations uint64,
) (V3PurchaseProjectEvidence, error) {
	if rpc == nil || expected.ChainID != binding.ChainID || expected.ProxyAddress != binding.ProxyAddress ||
		expected.ProjectID != binding.ProjectID || expected.PaymentToken != binding.PaymentToken ||
		expected.ProjectToken != binding.ProjectToken {
		return V3PurchaseProjectEvidence{}, errors.New("V3 purchase project binding mismatch")
	}
	observed, err := ReconcileV3PurchaseObservation(ctx, rpc, binding, txHash, logIndex, minConfirmations)
	if err != nil {
		return V3PurchaseProjectEvidence{}, err
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(binding.ChainID),
		ProjectID: new(big.Int).SetUint64(binding.ProjectID), ProxyAddress: binding.ProxyAddress}
	project, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, runtime,
		observed.Purchase.BlockNumber, observed.Purchase.BlockHash)
	if err != nil || idov3whitelist.MatchProjectIdentityObservation(expected, project) != nil ||
		project.State != idov3whitelist.ProjectActive && project.State != idov3whitelist.ProjectSuccessful {
		return V3PurchaseProjectEvidence{}, errors.New("V3 purchase project observation unavailable or mismatched")
	}
	// A stable header alone does not prove that a provider kept returning the
	// same receipt while historical state was read. Re-read the exact tx and
	// reject a dropped, reverted or log-replaced response before returning.
	again, err := rpc.TransactionReceipt(ctx, txHash)
	if err != nil || MatchV3PurchaseProjectObservation(&observed.Purchase, binding, again, project) != nil {
		return V3PurchaseProjectEvidence{}, errors.New("V3 purchase receipt changed during project read")
	}
	// Check both anchors once more, after all historical eth_call/code reads.
	for _, anchor := range []struct {
		number uint64
		hash   common.Hash
	}{
		{observed.Purchase.BlockNumber, observed.Purchase.BlockHash}, {observed.ConfirmedHead, observed.HeadHash},
	} {
		number := new(big.Int).SetUint64(anchor.number)
		header, readErr := rpc.HeaderByNumber(ctx, number)
		if readErr != nil || header == nil || header.Number == nil || header.Number.Cmp(number) != 0 || header.Hash() != anchor.hash {
			return V3PurchaseProjectEvidence{}, errors.New("V3 purchase project anchor changed during read")
		}
	}
	return V3PurchaseProjectEvidence{Purchase: observed, Project: project}, nil
}

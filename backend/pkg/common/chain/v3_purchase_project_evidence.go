package chain

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"joincare_server/pkg/common/idov3whitelist"
)

// MatchV3PurchaseProjectObservation cross-checks a supplied successful V3
// receipt against a project identity read at precisely its block. It does
// not authenticate the source of binding or runtime, fetch canonical headers,
// provide ongoing reorg recovery, or write a purchase/entitlement.
func MatchV3PurchaseProjectObservation(event *V3PurchaseEvent, binding V3PurchaseBinding, receipt *types.Receipt, project idov3whitelist.ProjectIdentityObservation) error {
	if event == nil || receipt == nil || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() ||
		receipt.BlockNumber.Sign() <= 0 || receipt.BlockHash == (common.Hash{}) ||
		project.BlockNumber != receipt.BlockNumber.Uint64() || project.BlockHash != receipt.BlockHash ||
		project.State != idov3whitelist.ProjectActive && project.State != idov3whitelist.ProjectSuccessful {
		return errors.New("V3 purchase project block or state mismatch")
	}
	expected := idov3whitelist.ExpectedProjectBinding{ChainID: binding.ChainID, ProxyAddress: binding.ProxyAddress,
		ProjectID: binding.ProjectID, PaymentToken: binding.PaymentToken, ProjectToken: binding.ProjectToken}
	if idov3whitelist.MatchProjectIdentityObservation(expected, project) != nil || MatchV3PurchaseReceipt(event, binding, receipt) != nil {
		return errors.New("V3 purchase project evidence mismatch")
	}
	// getProject is read at the end of the receipt block, so these totals must
	// include this purchase. They can also include other purchases in the same
	// block; equality is neither required nor evidence of ledger finality.
	payment, paymentOK := new(big.Int).SetString(event.PaymentRaw, 10)
	sold, soldOK := new(big.Int).SetString(event.ProjectTokenRaw, 10)
	raised, raisedOK := new(big.Int).SetString(project.RaisedAtBlockRaw, 10)
	totalSold, totalSoldOK := new(big.Int).SetString(project.TokensSoldAtBlockRaw, 10)
	if !paymentOK || !soldOK || !raisedOK || !totalSoldOK || payment.Sign() <= 0 || sold.Sign() <= 0 ||
		raised.Sign() < 0 || totalSold.Sign() < 0 || payment.Cmp(raised) > 0 || sold.Cmp(totalSold) > 0 {
		return errors.New("V3 purchase amount exceeds project block totals")
	}
	return nil
}

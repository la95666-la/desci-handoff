package chaintx

import (
	"github.com/ethereum/go-ethereum/common"
	"joincare_server/pkg/common/entity"
	"testing"
)

func TestActiveRetirementOriginalFence(t *testing.T) {
	row, b, _, _ := signedActivation45Fixture(t)
	copy := row
	if !sameActiveOriginal(row, copy, b) {
		t.Fatal("valid original")
	}
	for _, change := range []func(){func() { copy.BroadcastCount = 1 }, func() { copy.State = "broadcast_unknown" }, func() { copy.ID++ }, func() { copy.RawTx = []byte{1} }, func() { copy.Nonce = nil }, func() { copy.ActiveSlot = nil }, func() { n := uint64(1); copy.ReceiptStatus = &n }} {
		copy = row
		change()
		if sameActiveOriginal(row, copy, b) {
			t.Fatal("mutated active admitted")
		}
	}
	active := row.ID
	f := entity.ChainRelayerNonce{ChainID: row.ChainID, SenderAddress: row.SenderAddress, NextNonce: *row.Nonce + 1, ActiveSubmissionID: &active}
	if !activeRetirementFence(row, f) {
		t.Fatal("exact fence rejected")
	}
	for _, change := range []func(){func() { f.NextNonce++ }, func() { f.ActiveSubmissionID = nil }, func() { f.SenderAddress = common.HexToAddress("0x99").Hex() }, func() { f.ChainID++ }} {
		f = entity.ChainRelayerNonce{ChainID: row.ChainID, SenderAddress: row.SenderAddress, NextNonce: *row.Nonce + 1, ActiveSubmissionID: &active}
		change()
		if activeRetirementFence(row, f) {
			t.Fatal("foreign fence accepted")
		}
	}
}
func TestIndependentRetiredAuditAgreement(t *testing.T) {
	a := retiredNonceEvidence{SubmissionID: 41, Nonce: 35, ConsumedNonce: 58, TxHash: "original", ReceiptHash: "canonical", ConfirmedBlock: 100, ConfirmedHash: "a"}
	b := a
	b.ConfirmedBlock = 101
	b.ConfirmedHash = "b"
	if !sameRetiredAudit(a, b) {
		t.Fatal("normal head progress rejected")
	}
	b.ReceiptHash = "reorg"
	if sameRetiredAudit(a, b) {
		t.Fatal("different receipt accepted")
	}
	b = a
	b.ConsumedNonce++
	if sameRetiredAudit(a, b) {
		t.Fatal("nonce disagreement")
	}
}

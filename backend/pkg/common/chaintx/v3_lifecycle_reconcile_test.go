package chaintx

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestV3StoredReceiptCannotBeRewrittenByOrdinaryRecovery(t *testing.T) {
	row := entity.ChainTxSubmission{State: constant.ChainTxStateBroadcastUnknown}
	audit := V3ReceiptAudit{Block: 42, BlockHash: common.HexToHash("0x1234"), GasUsed: 90000}
	if !v3StoredReceiptAgrees(row, audit) {
		t.Fatal("unobserved receipt rejected")
	}
	status, block, gas := uint64(1), uint64(42), uint64(90000)
	row.ReceiptStatus, row.ReceiptBlockNumber, row.ReceiptBlockHash, row.ReceiptGasUsed = &status, &block, audit.BlockHash.Hex(), &gas
	if !v3StoredReceiptAgrees(row, audit) {
		t.Fatal("identical original-hash observation rejected")
	}
	wrong := audit
	wrong.BlockHash = common.HexToHash("0x9999")
	if v3StoredReceiptAgrees(row, wrong) {
		t.Fatal("replaced block overwrote persisted receipt")
	}
	wrong = audit
	wrong.Reverted = true
	if v3StoredReceiptAgrees(row, wrong) {
		t.Fatal("reverted receipt overwrote successful observation")
	}
	wrong = audit
	wrong.GasUsed++
	if v3StoredReceiptAgrees(row, wrong) {
		t.Fatal("changed gas observation overwrote persisted receipt")
	}
	row.State = constant.ChainTxStateMinedRevert
	if v3StoredReceiptAgrees(row, audit) {
		t.Fatal("mined revert state accepted successful observation")
	}
	row.ReceiptStatus = nil
	if v3StoredReceiptAgrees(row, audit) {
		t.Fatal("partial persisted receipt accepted")
	}
}

func TestV3ReconciliationRejectsOtherRolesAndCrossScopeBeforeRPC(t *testing.T) {
	b := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 42,
		ProxyAddress: common.HexToAddress("0x2222222222222222222222222222222222222222")}
	active := uint8(1)
	nonce := uint64(7)
	hash := "0x" + "1111111111111111111111111111111111111111111111111111111111111111"
	row := entity.ChainTxSubmission{ID: 1, ProjectID: 42, ChainID: 31337, ContractAddress: b.ProxyAddress.Hex(),
		SenderAddress: "0x3333333333333333333333333333333333333333", Action: "ido_v3_activate", ActiveSlot: &active, Nonce: &nonce, TxHash: &hash, RawTx: []byte{1}, CallData: []byte{1}}
	if !v3ReconcilableRow(row, b) {
		t.Fatal("valid identity rejected")
	}
	for _, action := range []string{"ido_v3_cancel", "ido_v3_withdraw_raised_funds", "ido_v3_withdraw_project_tokens", "ido_v3_future", "v2_activate"} {
		wrong := row
		wrong.Action = action
		if v3ReconcilableRow(wrong, b) {
			t.Fatalf("wrong role/action accepted: %s", action)
		}
	}
	creation := row
	creation.Action = "ido_v3_create"
	if !v3ReconcilableRow(creation, b) {
		t.Fatal("signed creation not eligible for separate signed-Set recovery")
	}
	wrong := b
	wrong.ProjectID++
	if v3ReconcilableRow(row, wrong) {
		t.Fatal("cross project accepted")
	}
	wrong = b
	wrong.ProxyAddress = common.HexToAddress("0x4444444444444444444444444444444444444444")
	if v3ReconcilableRow(row, wrong) {
		t.Fatal("cross proxy accepted")
	}
	changed := row
	changed.RawTx = []byte{2}
	if sameV3SignedAttempt(row, changed) {
		t.Fatal("raw transaction replacement accepted")
	}
	changed = row
	changed.CallData = []byte{2}
	if sameV3SignedAttempt(row, changed) {
		t.Fatal("calldata replacement accepted")
	}
	changed = row
	changed.TxHash = new(string)
	*changed.TxHash = hash
	if !sameV3SignedAttempt(row, changed) {
		t.Fatal("matching attempt rejected")
	}
}

func TestV3FinalizationRejectsMismatchedEvidenceWithoutQueries(t *testing.T) {
	db, mock := shutdownMockDB(t)
	proxy := common.HexToAddress("0x2222222222222222222222222222222222222222")
	operator := common.HexToAddress("0x3333333333333333333333333333333333333333")
	b := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 42, ProxyAddress: proxy}
	data, err := basechain.PackV3Lifecycle("activateProject", big.NewInt(42))
	if err != nil {
		t.Fatal(err)
	}
	slot := uint8(1)
	nonce := uint64(7)
	hash := "0x" + "1111111111111111111111111111111111111111111111111111111111111111"
	row := entity.ChainTxSubmission{ID: 1, Action: "ido_v3_activate", ChainID: 31337, ProjectID: 42,
		ContractAddress: proxy.Hex(), SenderAddress: operator.Hex(), ActiveSlot: &slot, Nonce: &nonce,
		TxHash: &hash, RawTx: []byte{1}, CallData: data}
	audit := V3ReceiptAudit{Hash: common.HexToHash(hash), Nonce: 7, Block: 42,
		BlockHash: common.HexToHash("0x42")}
	for _, tc := range []struct {
		name   string
		mutate func(*entity.ChainTxSubmission, *V3ReceiptAudit, *common.Address)
	}{
		{"wrong nonce", func(_ *entity.ChainTxSubmission, a *V3ReceiptAudit, _ *common.Address) { a.Nonce++ }},
		{"missing block", func(_ *entity.ChainTxSubmission, a *V3ReceiptAudit, _ *common.Address) { a.Block = 0 }},
		{"foreign sender", func(_ *entity.ChainTxSubmission, _ *V3ReceiptAudit, c *common.Address) { *c = proxy }},
		{"changed action", func(r *entity.ChainTxSubmission, _ *V3ReceiptAudit, _ *common.Address) { r.Action = "ido_v3_cancel" }},
		{"changed project argument", func(r *entity.ChainTxSubmission, _ *V3ReceiptAudit, _ *common.Address) { r.ProjectID = 43 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, a, c := row, audit, operator
			tc.mutate(&r, &a, &c)
			if _, err := finalizeV3LifecycleReceiptTx(db, r, b, c, a); err == nil {
				t.Fatal("mismatched evidence passed")
			}
		})
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

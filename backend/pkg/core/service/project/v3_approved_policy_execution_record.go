package project

import (
	"errors"
	"strings"

	"gorm.io/gorm"
	"joincare_server/pkg/common/idov3whitelist"
)

// approvedV3PolicyExecutionRecord is a DB comparison of a reviewed policy
// change and an already observed Safe transaction. It is NOT proof eligibility:
// publication callers independently check the original Safe transaction and
// canonical receipt. Investment proof combines this immutable publication
// with wallet membership; actual chain version/proof enforcement remains in
// the purchase contract, not the DB-only proof request.
type approvedV3PolicyExecutionRecord struct {
	Preview      V3PolicyPreview
	ReviewID     uint64
	InitiatorID  uint64
	ApproverID   uint64
	TxHash       string
	SafeTxHash   string
	Digest       string
	ReceiptBlock uint64
	ReceiptHash  string
}

// readApprovedV3PolicyExecutionTx accepts only the new four-eyes digest
// domain. Historical offline-attested execution rows cannot silently borrow
// this admission. The caller owns the transaction and must not commit a
// financial or proof write merely because this comparison succeeds.
func readApprovedV3PolicyExecutionTx(tx *gorm.DB, projectID, reviewID uint64) (approvedV3PolicyExecutionRecord, error) {
	invalid := errors.New("V3 approved policy execution unavailable")
	if tx == nil || projectID == 0 || reviewID == 0 {
		return approvedV3PolicyExecutionRecord{}, invalid
	}
	set, err := idov3whitelist.ReadApprovedSetRecord(tx.Statement.Context, tx, projectID)
	if err != nil || set.Identity.ProjectID != projectID {
		return approvedV3PolicyExecutionRecord{}, invalid
	}
	preview, err := V3PolicyReviewReadTx(tx, projectID, reviewID)
	if err != nil || preview.ChainID != "56" || preview.Proxy != set.Identity.Proxy || !strings.EqualFold(preview.Proxy, v3TokenProxy) {
		return approvedV3PolicyExecutionRecord{}, invalid
	}
	var rows []struct {
		InitiatorID  uint64 `gorm:"column:initiator_admin_id"`
		ApproverID   uint64 `gorm:"column:approver_admin_id"`
		TxHash       string `gorm:"column:tx_hash"`
		SafeTxHash   string `gorm:"column:safe_tx_hash"`
		Digest       string `gorm:"column:attestation_digest"`
		ReceiptBlock uint64 `gorm:"column:receipt_block"`
		ReceiptHash  string `gorm:"column:receipt_hash"`
	}
	read := tx.Raw(`SELECT r.initiator_admin_id,d.approver_admin_id,e.tx_hash,e.safe_tx_hash,
		e.attestation_digest,e.receipt_block,e.receipt_hash FROM ido_v3_policy_review r
		JOIN ido_v3_policy_review_decision d ON d.review_id=r.id
		 AND d.chain_id=r.chain_id AND BINARY d.proxy_address=BINARY r.proxy_address
		 AND d.project_id=r.project_id AND BINARY d.contract_version=BINARY r.contract_version
		 AND d.target_version=r.target_version AND BINARY d.decision=BINARY 'approved'
		JOIN ido_v3_policy_review_execution e ON e.review_id=d.review_id
		WHERE r.id=? AND r.project_id=? AND r.chain_id=56
		 AND BINARY r.proxy_address=BINARY ? AND BINARY r.contract_version=BINARY 'ido_v3_stablecoin'
		 LIMIT 2`, reviewID, projectID, preview.Proxy).Scan(&rows)
	if read.Error != nil || len(rows) != 1 {
		return approvedV3PolicyExecutionRecord{}, invalid
	}
	r := rows[0]
	want, err := digestV3ApprovedPolicyExecution(preview, set.Identity, reviewID, r.InitiatorID, r.ApproverID,
		r.TxHash, r.SafeTxHash, r.ReceiptBlock, r.ReceiptHash)
	if err != nil || want != r.Digest {
		return approvedV3PolicyExecutionRecord{}, invalid
	}
	return approvedV3PolicyExecutionRecord{Preview: preview, ReviewID: reviewID,
		InitiatorID: r.InitiatorID, ApproverID: r.ApproverID, TxHash: r.TxHash,
		SafeTxHash: r.SafeTxHash, Digest: r.Digest, ReceiptBlock: r.ReceiptBlock, ReceiptHash: r.ReceiptHash}, nil
}

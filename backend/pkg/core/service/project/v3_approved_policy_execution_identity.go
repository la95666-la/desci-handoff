package project

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"joincare_server/pkg/common/idov3whitelist"
)

// v3ApprovedPolicyExecutionIdentity binds an existing immutable, two-admin
// policy review to an actual original Safe transaction and canonical receipt.
// The digest is a cross-process comparison value, NOT a signature, approval
// of a supplied hash, or permission to apply a financial effect. Publication
// validates original Safe/chain evidence; membership requests verify this DB
// digest and full list locally without repeating the on-chain audit.
type v3ApprovedPolicyExecutionIdentity struct {
	ChainID      uint64 `json:"chain_id"`
	ProjectID    uint64 `json:"project_id"`
	Proxy        string `json:"proxy"`
	Governor     string `json:"governor"`
	SetDigest    string `json:"set_digest"`
	ReviewID     uint64 `json:"review_id"`
	ReviewDigest string `json:"review_digest"`
	InitiatorID  uint64 `json:"initiator_id"`
	ApproverID   uint64 `json:"approver_id"`
	TxHash       string `json:"tx_hash"`
	SafeTxHash   string `json:"safe_tx_hash"`
	ReceiptBlock uint64 `json:"receipt_block"`
	ReceiptHash  string `json:"receipt_hash"`
}

func digestV3ApprovedPolicyExecution(preview V3PolicyPreview, set idov3whitelist.ApprovedSetIdentity, reviewID, initiatorID, approverID uint64,
	txHash, safeHash string, receiptBlock uint64, receiptHash string) (string, error) {
	invalid := errors.New("V3 approved policy execution identity unavailable")
	setDigest, setErr := idov3whitelist.DigestApprovedSetIdentity(set)
	if reviewID == 0 || initiatorID == 0 || approverID == 0 || initiatorID == approverID ||
		receiptBlock == 0 || preview.ChainID != "56" || !v3CanonicalHash(preview.PreviewDigest) ||
		!v3CanonicalHash(txHash) || !v3CanonicalHash(safeHash) || !v3CanonicalHash(receiptHash) ||
		txHash == safeHash || preview.Proxy != strings.ToLower(preview.Proxy) ||
		common.HexToAddress(preview.Proxy) != common.HexToAddress(v3TokenProxy) ||
		setErr != nil || set.ChainID != 56 || set.ProjectID != mustParseV3Version(preview.ProjectID) ||
		set.Proxy != preview.Proxy || set.ApprovalID == 0 || set.InitiatorID == set.ApproverID {
		return "", invalid
	}
	projectID := mustParseV3Version(preview.ProjectID)
	if projectID == 0 || receiptBlock < 128805696 {
		return "", invalid
	}
	digest, err := V3PolicyPreviewDigest(preview)
	if err != nil || digest != preview.PreviewDigest {
		return "", invalid
	}
	if _, _, err := v3PolicyIdentity(preview, projectID); err != nil {
		return "", invalid
	}
	identity := v3ApprovedPolicyExecutionIdentity{ChainID: 56, ProjectID: projectID,
		Proxy: preview.Proxy, Governor: strings.ToLower(v3DiagnosticGovernor), SetDigest: strings.ToLower(setDigest.Hex()),
		ReviewID: reviewID, ReviewDigest: digest, InitiatorID: initiatorID, ApproverID: approverID,
		TxHash: txHash, SafeTxHash: safeHash, ReceiptBlock: receiptBlock, ReceiptHash: receiptHash}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return "", invalid
	}
	sum := sha256.Sum256(append([]byte("joincare:ido-v3:approved-policy-execution:v1\x00"), encoded...))
	return strings.ToLower(common.BytesToHash(sum[:]).Hex()), nil
}

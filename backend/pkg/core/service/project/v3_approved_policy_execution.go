package project

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/db"
)

// RecordApprovedV3PolicyExecution accepts two hashes only as *locators* for
// an externally executed policy Safe transaction. They are not approval or
// attestation: the immutable two-admin policy decision, exact inner calldata,
// successful outer Safe execution, receipt, finality and latest policy must
// be independently checked before writing the insert-only observation. This
// candidate is not routed until the proof reader uses this same digest.
func RecordApprovedV3PolicyExecution(projectID, reviewID uint64, rawTxHash, rawSafeHash string) error {
	return RecordApprovedV3PolicyExecutionContext(context.Background(), projectID, reviewID, rawTxHash, rawSafeHash)
}

// The stage is a finite diagnostic, not the underlying database/RPC error.
// It never contains a hash, URL, wallet, project identity or credential.
type v3PolicyObservationStageError string

func (e v3PolicyObservationStageError) Error() string {
	return "V3 approved policy receipt unavailable"
}

func V3PolicyObservationFailureStage(err error) string {
	var stage v3PolicyObservationStageError
	if errors.As(err, &stage) {
		return string(stage)
	}
	return "unavailable"
}

// Only a finite enumeration crosses the Admin boundary. In particular, do
// not format errors returned by the commercial RPC or approved member reader.
func V3PolicyObservationFailureDetail(err error) string {
	var stage v3PolicyObservationDetailError
	if errors.As(err, &stage) {
		switch stage {
		case "request", "scope", "receipt_lookup", "confirmations", "receipt_header",
			"receipt_lookup_deadline", "receipt_lookup_rpc", "receipt_lookup_missing", "receipt_lookup_malformed",
			"receipt_policy_read", "receipt_policy_match", "head_header", "head_policy_read",
			"head_policy_match", "receipt_header_recheck", "head_header_recheck",
			"policy_evidence_unavailable", "disabled_policy_evidence", "receipt_recheck", "receipt_logs":
			return string(stage)
		}
	}
	return ""
}

type v3PolicyObservationDetailError string

func (e v3PolicyObservationDetailError) Error() string { return "V3 policy receipt check unavailable" }

type v3PolicyObservationError struct {
	stage  v3PolicyObservationStageError
	detail v3PolicyObservationDetailError
}

// Below the Admin client's 170s and Nginx's 175s budgets, and the Core
// server's 180s write deadline. No work continues after request cancellation.
const v3PolicyObservationBudget = 150 * time.Second

func (e v3PolicyObservationError) Error() string { return e.stage.Error() }
func (e v3PolicyObservationError) Unwrap() []error {
	return []error{e.stage, e.detail}
}

func RecordApprovedV3PolicyExecutionContext(parent context.Context, projectID, reviewID uint64, rawTxHash, rawSafeHash string) error {
	if parent == nil || parent.Err() != nil || !viper.GetBool("idoV3PolicyExecution.enabled") || !viper.GetBool("idoV3PolicyJournal.enabled") ||
		projectID == 0 || reviewID == 0 || db.Mysql() == nil ||
		!v3CanonicalHash(rawTxHash) || !v3CanonicalHash(rawSafeHash) || rawTxHash == rawSafeHash {
		return v3PolicyObservationStageError("request_gate")
	}
	ownerHash, governorHash, err := approvedV3SafeHashes()
	if err != nil {
		return v3PolicyObservationStageError("safe_identity")
	}
	ctx, cancel := context.WithTimeout(parent, v3PolicyObservationBudget)
	defer cancel()
	rpc, err := v3TokenGateRPC(ctx)
	if err != nil {
		return v3PolicyObservationStageError("rpc_setup")
	}
	defer rpc.Close()
	return recordApprovedV3PolicyExecution(ctx, db.Mysql(), rpc, projectID, reviewID,
		rawTxHash, rawSafeHash, ownerHash, governorHash)
}

func recordApprovedV3PolicyExecution(ctx context.Context, database *gorm.DB, rpc *ethclient.Client,
	projectID, reviewID uint64, txHash, safeHash string, ownerHash, governorHash common.Hash) (resultErr error) {
	invalid := errors.New("V3 approved policy receipt unavailable")
	started, phase, complete := time.Now(), "input", false
	detail := ""
	defer func() {
		if !complete {
			if phase == "canonical_policy_receipt" && detail != "" {
				resultErr = v3PolicyObservationError{v3PolicyObservationStageError(phase), v3PolicyObservationDetailError(detail)}
			} else {
				resultErr = v3PolicyObservationStageError(phase)
			}
			if logger := log.GetLogger(); logger != nil {
				logger.WithFields(map[string]interface{}{
					"phase": phase, "elapsed_ms": time.Since(started).Milliseconds(),
					"check":    V3PolicyObservationFailureDetail(resultErr),
					"deadline": ctx != nil && errors.Is(ctx.Err(), context.DeadlineExceeded),
					"canceled": ctx != nil && errors.Is(ctx.Err(), context.Canceled),
				}).Warn("V3 approved policy execution observation unavailable")
			}
		}
	}()
	if ctx == nil || database == nil || rpc == nil || projectID == 0 || reviewID == 0 ||
		ownerHash == (common.Hash{}) || governorHash == (common.Hash{}) ||
		!v3CanonicalHash(txHash) || !v3CanonicalHash(safeHash) || txHash == safeHash {
		return invalid
	}
	phase = "creation_admission"
	admitted, initialCandidate, err := chaintx.AdmitApprovedLockedProjectForPolicyObservation(ctx, database, rpc, projectID, ownerHash, governorHash)
	if err != nil {
		return invalid
	}
	phase = "policy_evidence_store"
	if requireV3PolicyEvidenceStore(ctx, database, projectID) != nil {
		return invalid
	}
	phase = "approved_set"
	set, err := idov3whitelist.ReadApprovedSetRecord(ctx, database, projectID)
	if err != nil {
		return invalid
	}
	var preview V3PolicyPreview
	var initiator, approver uint64
	phase = "immutable_review"
	err = database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var e error
		preview, e = V3PolicyReviewReadTx(tx, projectID, reviewID)
		if e != nil {
			return invalid
		}
		var rows []struct {
			Initiator uint64 `gorm:"column:initiator_admin_id"`
			Approver  uint64 `gorm:"column:approver_admin_id"`
		}
		read := tx.Raw(`SELECT r.initiator_admin_id,d.approver_admin_id FROM ido_v3_policy_review r
			JOIN ido_v3_policy_review_decision d ON d.review_id=r.id AND d.chain_id=r.chain_id
			 AND BINARY d.proxy_address=BINARY r.proxy_address AND d.project_id=r.project_id
			 AND BINARY d.contract_version=BINARY r.contract_version AND d.target_version=r.target_version
			 WHERE r.id=? AND r.project_id=? AND r.chain_id=56 AND BINARY r.proxy_address=BINARY ?
			 AND BINARY r.contract_version=BINARY 'ido_v3_stablecoin' AND BINARY d.decision=BINARY 'approved'
				 AND NOT EXISTS (SELECT 1 FROM ido_v3_policy_review newer
					JOIN ido_v3_policy_review_decision nd ON nd.review_id=newer.id AND BINARY nd.decision=BINARY 'approved'
					WHERE newer.chain_id=r.chain_id AND BINARY newer.proxy_address=BINARY r.proxy_address
					AND newer.project_id=r.project_id AND BINARY newer.contract_version=BINARY r.contract_version
					AND newer.target_version=r.target_version AND newer.id>r.id)
			 LIMIT 2`, reviewID, projectID, set.Identity.Proxy).Scan(&rows)
		if read.Error != nil || len(rows) != 1 || rows[0].Initiator == 0 || rows[0].Approver == 0 ||
			rows[0].Initiator == rows[0].Approver {
			return invalid
		}
		initiator, approver = rows[0].Initiator, rows[0].Approver
		return nil
	})
	if err != nil || preview.Proxy != set.Identity.Proxy || preview.ChainID != "56" {
		return invalid
	}
	phase = "review_commitment"
	scope, commitment, err := v3PolicyIdentity(preview, projectID)
	if err != nil || scope.ProxyAddress != admitted.Binding.ProxyAddress ||
		scope.ChainID.Uint64() != admitted.Binding.ChainID {
		return invalid
	}
	governor := common.HexToAddress(v3DiagnosticGovernor)
	phase = "prior_policy"
	if preview.OldEnabled && v3VerifyPriorReceipt(ctx, rpc, admitted, preview, projectID) != nil {
		return invalid
	}
	if preview.OldEnabled && verifyApprovedV3PriorPolicy(ctx, database, rpc, admitted, preview, projectID) != nil {
		return invalid
	}
	outer, inner := common.HexToHash(txHash), common.HexToHash(safeHash)
	var evidence idov3whitelist.PolicyEvidence
	phase = "canonical_policy_receipt"
	if preview.Enabled {
		evidence, err = idov3whitelist.ReconcilePolicyEvidence(ctx, rpc, scope, admitted.Runtime,
			commitment, outer, admitted.Confirmations)
	} else {
		evidence, err = v3ReconcileDisabledPolicy(ctx, rpc, scope, admitted.Runtime,
			mustParseV3Version(preview.TargetVersion), outer, admitted.Confirmations)
	}
	if err != nil {
		if preview.Enabled {
			detail = idov3whitelist.PolicyEvidenceFailureStage(err)
		} else {
			detail = "disabled_policy_evidence"
		}
		return invalid
	}
	receipt, err := rpc.TransactionReceipt(ctx, outer)
	if err != nil || receipt == nil || receipt.TxHash != outer ||
		receipt.BlockHash != evidence.ReceiptBlock.BlockHash || receipt.BlockNumber == nil ||
		!receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Uint64() != evidence.ReceiptBlock.BlockNumber {
		detail = "receipt_recheck"
		return invalid
	}
	// Keep an immutable copy of the complete original receipt before any
	// subsequent RPC reads. A provider may reuse its nested Logs slice, and a
	// matching block/hash alone cannot authenticate changed Safe/policy logs.
	initialLogs, err := json.Marshal(receipt.Logs)
	if err != nil || len(initialLogs) == 0 || len(initialLogs) > 1<<20 {
		detail = "receipt_logs"
		return invalid
	}
	initialLogsDigest := sha256.Sum256(initialLogs)
	phase = "safe_execution"
	change := idov3whitelist.PolicyChange{ExpectedVersion: mustParseV3Version(preview.OldVersion),
		TargetVersion: mustParseV3Version(preview.TargetVersion), Enabled: preview.Enabled,
		Root: common.HexToHash(preview.Root)}
	if preview.Enabled {
		change.Candidate = &commitment
	}
	if idov3whitelist.CheckSafePolicyExecution(ctx, rpc, scope, governor, change, outer, inner, receipt) != nil {
		return invalid
	}
	governance := idov3whitelist.GovernanceIdentity{FollowOwner: true,
		Governor: governor, OwnerCodeHash: ownerHash, GovernorCodeHash: governorHash}
	phase = "governance_identity"
	for _, observation := range []idov3whitelist.PolicyObservation{evidence.ReceiptBlock, evidence.Head} {
		if _, err := idov3whitelist.ReadGovernanceAt(ctx, rpc, scope, admitted.Runtime,
			governance, observation.BlockNumber, observation.BlockHash); err != nil {
			return invalid
		}
		project, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, admitted.Runtime,
			observation.BlockNumber, observation.BlockHash)
		if err != nil || idov3whitelist.MatchProjectIdentityObservation(admitted.Binding, project) != nil ||
			(project.State != idov3whitelist.ProjectPending && project.State != idov3whitelist.ProjectActive) ||
			(project.State == idov3whitelist.ProjectActive && !preview.Enabled) {
			return invalid
		}
	}
	// Reject a newer unconfirmed policy change, even while the receipt is
	// already final. An observation is never a Safe proposal or an execution.
	phase = "current_policy"
	if checkApprovedV3ObservationCurrentPolicy(ctx, rpc, scope, admitted, preview) != nil {
		return invalid
	}
	phase = "repeat_receipt"
	confirmation, err := rpc.TransactionReceipt(ctx, outer)
	var repeatedLogsDigest [sha256.Size]byte
	if err == nil && confirmation != nil {
		repeatedLogs, marshalErr := json.Marshal(confirmation.Logs)
		if marshalErr != nil || len(repeatedLogs) == 0 || len(repeatedLogs) > 1<<20 {
			return invalid
		}
		repeatedLogsDigest = sha256.Sum256(repeatedLogs)
	}
	if err != nil || confirmation == nil || confirmation.TxHash != outer ||
		confirmation.BlockHash != receipt.BlockHash || confirmation.Status != receipt.Status ||
		confirmation.BlockNumber == nil || confirmation.BlockNumber.Cmp(receipt.BlockNumber) != 0 ||
		confirmation.TransactionIndex != receipt.TransactionIndex ||
		repeatedLogsDigest != initialLogsDigest {
		return invalid
	}
	phase = "final_admission"
	if chaintx.RecheckApprovedPolicyObservation(ctx, database, rpc, projectID, initialCandidate) != nil {
		return invalid
	}
	phase = "final_policy_evidence_store"
	if requireV3PolicyEvidenceStore(ctx, database, projectID) != nil {
		return invalid
	}
	phase = "execution_digest"
	digest, err := digestV3ApprovedPolicyExecution(preview, set.Identity, reviewID, initiator, approver,
		txHash, safeHash, evidence.ReceiptBlock.BlockNumber, strings.ToLower(evidence.ReceiptBlock.BlockHash.Hex()))
	if err != nil {
		return invalid
	}
	phase = "atomic_cursor_incident"
	err = database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if chainreorg.LockNoActiveIncidentTx(tx, admitted.Binding.ChainID, preview.Proxy) != nil ||
			chaintx.LockV3RunningCursorTx(tx, admitted.Binding) != nil {
			return invalid
		}
		phase = "atomic_project_lock"
		var p []struct {
			ID uint64 `gorm:"column:project_id"`
		}
		read := tx.Raw(`SELECT project_id FROM projects WHERE project_id=? AND chain_id=?
			AND BINARY contract_address=BINARY ? AND BINARY funding_mode=BINARY 'capped_presale'
			AND BINARY contract_version=BINARY 'ido_v3_stablecoin' AND onchain_locked=1
			AND status=? LIMIT 2 FOR UPDATE`, projectID, admitted.Binding.ChainID,
			preview.Proxy, strings.ToUpper(preview.State[:1])+preview.State[1:]).Scan(&p)
		if read.Error != nil || len(p) != 1 || p[0].ID != projectID {
			return invalid
		}
		phase = "atomic_review_identity"
		fresh, e := idov3whitelist.ReadApprovedSetRecord(ctx, tx, projectID)
		if e != nil || fresh.Identity != set.Identity {
			return invalid
		}
		review, e := V3PolicyReviewReadTx(tx, projectID, reviewID)
		if e != nil || review.PreviewDigest != preview.PreviewDigest {
			return invalid
		}
		var newer int64
		if tx.Raw(`SELECT COUNT(*) FROM ido_v3_policy_review r
			JOIN ido_v3_policy_review_execution e ON e.review_id=r.id
			WHERE r.chain_id=? AND BINARY r.proxy_address=BINARY ? AND r.project_id=?
			AND r.target_version>=? AND r.id<>?`, admitted.Binding.ChainID, preview.Proxy,
			projectID, mustParseV3Version(preview.TargetVersion), reviewID).Scan(&newer).Error != nil || newer != 0 {
			return invalid
		}
		var decisions []struct {
			Approver uint64 `gorm:"column:approver_admin_id"`
		}
		read = tx.Raw(`SELECT approver_admin_id FROM ido_v3_policy_review_decision WHERE review_id=?
			AND BINARY decision=BINARY 'approved' LIMIT 2`, reviewID).Scan(&decisions)
		if read.Error != nil || len(decisions) != 1 || decisions[0].Approver != approver {
			return invalid
		}
		phase = "atomic_initiator_permission"
		if _, e := middleware.HasLivePermissionTx(tx, int64(initiator), middleware.ModProject, middleware.ActWrite); e != nil {
			return invalid
		}
		phase = "atomic_approver_permission"
		if _, e := middleware.HasLivePermissionTx(tx, int64(approver), middleware.ModProject, middleware.ActApprove); e != nil {
			return invalid
		}
		phase = "atomic_execution_insert"
		var existing int64
		if tx.Raw(`SELECT COUNT(*) FROM ido_v3_policy_review_execution WHERE review_id=?`, reviewID).Scan(&existing).Error != nil || existing != 0 {
			return invalid
		}
		insert := tx.Exec(`INSERT INTO ido_v3_policy_review_execution
			(review_id,tx_hash,safe_tx_hash,attestation_digest,receipt_block,receipt_hash)
			VALUES (?,?,?,?,?,?)`, reviewID, txHash, safeHash, digest,
			evidence.ReceiptBlock.BlockNumber, strings.ToLower(evidence.ReceiptBlock.BlockHash.Hex()))
		if insert.Error != nil || insert.RowsAffected != 1 {
			return invalid
		}
		return nil
	})
	if err != nil {
		return invalid
	}
	complete = true
	return nil
}

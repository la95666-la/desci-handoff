package project

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/db"
)

// Offline Safe attestation is signed in a domain separate from the Set and
// scanner releases. It binds the exact already-executed outer transaction,
// Safe transaction hash, review digest and expiry. Safe code and threshold
// are independently read at the receipt block, not signed in this payload. It
// cannot be supplied by the Admin who enters a transaction hash. A release
// must pin an independently reviewed public key before observing any tx.
type V3SafeAttestation struct {
	ChainID      uint64 `json:"chain_id"`
	ProjectID    uint64 `json:"project_id"`
	Proxy        string `json:"proxy"`
	Governor     string `json:"governor"`
	ReviewDigest string `json:"review_digest"`
	TxHash       string `json:"tx_hash"`
	SafeTxHash   string `json:"safe_tx_hash"`
	ExpiresAt    int64  `json:"expires_at"`
}

func pinnedV3SafeAttestation(p V3PolicyPreview, governor common.Address, requireFresh bool) (V3SafeAttestation, string, error) {
	invalid := errors.New("V3 Safe attestation unavailable")
	key, e1 := base64.StdEncoding.Strict().DecodeString(viper.GetString("idoV3PolicyExecution.pinnedPublicKey"))
	sig, e2 := base64.StdEncoding.Strict().DecodeString(viper.GetString("idoV3PolicyExecution.signature"))
	var a V3SafeAttestation
	raw := []byte(viper.GetString("idoV3PolicyExecution.manifest"))
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var extra any
	if !viper.GetBool("idoV3PolicyExecution.enabled") || e1 != nil || e2 != nil || len(key) != ed25519.PublicKeySize ||
		len(sig) != ed25519.SignatureSize || decoder.Decode(&a) != nil || decoder.Decode(&extra) != io.EOF ||
		(requireFresh && a.ExpiresAt <= time.Now().Unix()) || a.ExpiresAt <= 0 || a.ChainID == 0 || a.ProjectID == 0 ||
		a.Proxy != p.Proxy || a.Governor != strings.ToLower(governor.Hex()) || a.ReviewDigest != p.PreviewDigest ||
		a.ProjectID != mustParseV3Version(p.ProjectID) || a.ChainID != mustParseV3Version(p.ChainID) ||
		!v3CanonicalHash(a.TxHash) || !v3CanonicalHash(a.SafeTxHash) || a.TxHash == a.SafeTxHash {
		return a, "", invalid
	}
	// JSON decoder must contain exactly one value. A second value or malformed
	// trailing bytes cannot be accepted as a signed authorization.
	encoded, err := json.Marshal(a)
	if err != nil || !ed25519.Verify(key, append([]byte("joincare:ido-v3:safe-execution:v1\x00"), encoded...), sig) {
		return a, "", invalid
	}
	sum := sha256.Sum256(encoded)
	return a, "0x" + common.Bytes2Hex(sum[:]), nil
}

func v3CanonicalHash(s string) bool {
	if len(s) != 66 || !strings.HasPrefix(s, "0x") || s != strings.ToLower(s) || common.HexToHash(s) == (common.Hash{}) {
		return false
	}
	for _, c := range s[2:] {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

// V3PolicyRecordExecution never signs, proposes, retries or broadcasts. A
// verified Safe must have already executed externally; the original hash is
// audited against both independent signatures and the canonical chain.
func V3PolicyRecordExecution(projectID, reviewID uint64) error {
	invalid := errors.New("V3 policy execution unavailable")
	if projectID == 0 || reviewID == 0 || db.Mysql() == nil || !viper.GetBool("idoV3PolicyExecution.enabled") ||
		!viper.GetBool("idoV3PolicyJournal.enabled") {
		return invalid
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	admitted, set, _, _, _, err := pinnedV3Admission(projectID, false)
	if err != nil {
		return invalid
	}
	var preview V3PolicyPreview
	var approver uint64
	var initiator uint64
	err = db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		preview, err = V3PolicyReviewReadTx(tx, projectID, reviewID)
		if err != nil {
			return invalid
		}
		var decisions []struct {
			Approver  uint64 `gorm:"column:approver_admin_id"`
			Initiator uint64 `gorm:"column:initiator_admin_id"`
		}
		read := tx.Raw(`SELECT d.approver_admin_id,r.initiator_admin_id FROM ido_v3_policy_review_decision d
			JOIN ido_v3_policy_review r ON r.id=d.review_id WHERE d.review_id=? AND d.decision='approved'
			AND d.chain_id=? AND BINARY d.proxy_address=BINARY ? AND d.project_id=? AND d.target_version=?
			AND NOT EXISTS (SELECT 1 FROM ido_v3_policy_review newer
			 JOIN ido_v3_policy_review_decision nd ON nd.review_id=newer.id AND BINARY nd.decision=BINARY 'approved'
			 WHERE newer.chain_id=r.chain_id AND BINARY newer.proxy_address=BINARY r.proxy_address
			 AND newer.project_id=r.project_id AND BINARY newer.contract_version=BINARY r.contract_version
			 AND newer.target_version=r.target_version AND newer.id>r.id) LIMIT 2`,
			reviewID, admitted.Binding.ChainID, preview.Proxy, projectID, mustParseV3Version(preview.TargetVersion)).Scan(&decisions)
		if read.Error != nil || len(decisions) != 1 || decisions[0].Initiator == 0 || decisions[0].Approver == 0 || decisions[0].Initiator == decisions[0].Approver {
			return invalid
		}
		initiator, approver = decisions[0].Initiator, decisions[0].Approver
		return nil
	})
	if err != nil {
		return invalid
	}
	governor := common.HexToAddress(set.Governor)
	att, digest, err := pinnedV3SafeAttestation(preview, governor, true)
	if err != nil {
		return invalid
	}
	rpc, err := chainrpc.Dial(ctx, viper.GetString("chain.project.url"))
	if err != nil {
		return invalid
	}
	defer rpc.Close()
	scope, commitment, err := v3PolicyIdentity(preview, projectID)
	if err != nil || scope.ChainID.Uint64() != admitted.Binding.ChainID || scope.ProxyAddress != admitted.Binding.ProxyAddress {
		return invalid
	}
	if preview.OldEnabled && v3VerifyPriorReceipt(ctx, rpc, admitted, preview, projectID) != nil {
		return invalid
	}
	transactionHash, safeHash := common.HexToHash(att.TxHash), common.HexToHash(att.SafeTxHash)
	var evidence idov3whitelist.PolicyEvidence
	if preview.Enabled {
		evidence, err = idov3whitelist.ReconcilePolicyEvidence(ctx, rpc, scope, admitted.Runtime, commitment, transactionHash, admitted.Confirmations)
	} else {
		evidence, err = v3ReconcileDisabledPolicy(ctx, rpc, scope, admitted.Runtime, mustParseV3Version(preview.TargetVersion), transactionHash, admitted.Confirmations)
	}
	if err != nil {
		return invalid
	}
	receipt, err := rpc.TransactionReceipt(ctx, transactionHash)
	if err != nil || receipt == nil || receipt.BlockHash != evidence.ReceiptBlock.BlockHash {
		return invalid
	}
	change := idov3whitelist.PolicyChange{ExpectedVersion: mustParseV3Version(preview.OldVersion), TargetVersion: mustParseV3Version(preview.TargetVersion),
		Enabled: preview.Enabled, Root: common.HexToHash(preview.Root)}
	if preview.Enabled {
		change.Candidate = &commitment
	}
	if idov3whitelist.CheckSafePolicyExecution(ctx, rpc, scope, governor, change, transactionHash, safeHash, receipt) != nil {
		return invalid
	}
	governance := idov3whitelist.GovernanceIdentity{Owner: common.HexToAddress(set.OwnerSafe), Governor: governor,
		OwnerCodeHash: common.HexToHash(set.OwnerCodeHash), GovernorCodeHash: common.HexToHash(set.GovernorCodeHash)}
	for i, obs := range []idov3whitelist.PolicyObservation{evidence.ReceiptBlock, evidence.Head} {
		if _, err = idov3whitelist.ReadGovernanceAt(ctx, rpc, scope, admitted.Runtime, governance, obs.BlockNumber, obs.BlockHash); err != nil {
			return invalid
		}
		project, readErr := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, admitted.Runtime, obs.BlockNumber, obs.BlockHash)
		if readErr != nil || idov3whitelist.MatchProjectIdentityObservation(admitted.Binding, project) != nil ||
			project.State != idov3whitelist.ProjectPending && project.State != idov3whitelist.ProjectActive ||
			(project.State == idov3whitelist.ProjectActive && !preview.Enabled) ||
			(i == 0 && preview.State == "active" && project.State != idov3whitelist.ProjectActive) {
			return invalid
		}
	}
	if checkV3ExecutionCursor(ctx, rpc, admitted) != nil {
		return invalid
	}
	// Do not publish a historical receipt observation after another policy
	// version has already become visible at the current tip.
	latest, err := rpc.BlockNumber(ctx)
	if err != nil {
		return invalid
	}
	latestHeader, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(latest))
	if err != nil || latestHeader == nil || latestHeader.Number == nil || !latestHeader.Number.IsUint64() || latestHeader.Number.Uint64() != latest {
		return invalid
	}
	latestPolicy, err := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, admitted.Runtime, latest, latestHeader.Hash())
	if err != nil || latestPolicy.Policy.Version != mustParseV3Version(preview.TargetVersion) ||
		latestPolicy.Policy.Enabled != preview.Enabled || strings.ToLower(latestPolicy.Policy.Root.Hex()) != preview.Root {
		return invalid
	}
	latestProject, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, admitted.Runtime, latest, latestHeader.Hash())
	if err != nil || idov3whitelist.MatchProjectIdentityObservation(admitted.Binding, latestProject) != nil ||
		(latestProject.State != idov3whitelist.ProjectPending && latestProject.State != idov3whitelist.ProjectActive) ||
		(latestProject.State == idov3whitelist.ProjectActive && !preview.Enabled) {
		return invalid
	}
	latestAgain, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(latest))
	if err != nil || latestAgain == nil || latestAgain.Hash() != latestHeader.Hash() {
		return invalid
	}
	// Normal chain growth does not invalidate the pinned policy/identity and
	// canonical hash checked above. The observation is not a frozen-head claim.
	receiptAgain, err := rpc.TransactionReceipt(ctx, transactionHash)
	if err != nil || receiptAgain == nil || receiptAgain.TxHash != transactionHash ||
		receiptAgain.BlockHash != evidence.ReceiptBlock.BlockHash || receiptAgain.BlockNumber == nil ||
		!receiptAgain.BlockNumber.IsUint64() || receiptAgain.BlockNumber.Uint64() != evidence.ReceiptBlock.BlockNumber ||
		receiptAgain.Status != receipt.Status {
		return invalid
	}
	return db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if chainreorg.LockNoActiveIncidentTx(tx, admitted.Binding.ChainID, preview.Proxy) != nil || chaintx.LockV3RunningCursorTx(tx, admitted.Binding) != nil {
			return invalid
		}
		var projects []struct {
			ID uint64 `gorm:"column:project_id"`
		}
		read := tx.Raw(`SELECT project_id FROM projects WHERE project_id=? AND chain_id=? AND BINARY contract_address=BINARY ?
			AND funding_mode='capped_presale' AND contract_version='ido_v3_stablecoin' AND onchain_locked=1
			AND status=? LIMIT 2 FOR UPDATE`, projectID, admitted.Binding.ChainID, preview.Proxy,
			strings.ToUpper(preview.State[:1])+preview.State[1:]).Scan(&projects)
		if read.Error != nil || len(projects) != 1 {
			return invalid
		}
		p, readErr := V3PolicyReviewReadTx(tx, projectID, reviewID)
		if readErr != nil || p.PreviewDigest != preview.PreviewDigest {
			return invalid
		}
		// A later executed version cannot be followed by observing an older
		// receipt as an effective current policy.
		var newer int64
		if tx.Raw(`SELECT COUNT(*) FROM ido_v3_policy_review r JOIN ido_v3_policy_review_execution e ON e.review_id=r.id
			WHERE r.chain_id=? AND BINARY r.proxy_address=BINARY ? AND r.project_id=? AND r.target_version>=?
			AND r.id<>?`, admitted.Binding.ChainID, preview.Proxy, projectID, mustParseV3Version(preview.TargetVersion), reviewID).
			Scan(&newer).Error != nil || newer != 0 {
			return invalid
		}
		var decisions []struct {
			Approver uint64 `gorm:"column:approver_admin_id"`
		}
		read = tx.Raw(`SELECT approver_admin_id FROM ido_v3_policy_review_decision WHERE review_id=? AND decision='approved' LIMIT 2`, reviewID).Scan(&decisions)
		if read.Error != nil || len(decisions) != 1 || decisions[0].Approver != approver || initiator == approver {
			return invalid
		}
		// Admission and approval were historical, but revocation of either
		// current Admin permission closes the materialization gate.
		if _, err := middleware.HasLivePermissionTx(tx, int64(initiator), middleware.ModProject, middleware.ActWrite); err != nil {
			return invalid
		}
		if _, err := middleware.HasLivePermissionTx(tx, int64(approver), middleware.ModProject, middleware.ActApprove); err != nil {
			return invalid
		}
		var duplicates int64
		if tx.Raw(`SELECT COUNT(*) FROM ido_v3_policy_review_execution WHERE review_id=?`, reviewID).Scan(&duplicates).Error != nil || duplicates != 0 {
			return invalid
		}
		insert := tx.Exec(`INSERT INTO ido_v3_policy_review_execution
			(review_id,tx_hash,safe_tx_hash,attestation_digest,receipt_block,receipt_hash)
			VALUES (?,?,?,?,?,?)`, reviewID, att.TxHash, att.SafeTxHash, digest, evidence.ReceiptBlock.BlockNumber,
			strings.ToLower(evidence.ReceiptBlock.BlockHash.Hex()))
		if insert.Error != nil || insert.RowsAffected != 1 {
			return invalid
		}
		return nil
	})
}

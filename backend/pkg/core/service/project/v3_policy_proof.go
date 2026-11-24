package project

import (
	"context"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/core/db"
)

type V3WalletProof struct {
	ChainID   string   `json:"chain_id"`
	ProjectID string   `json:"project_id"`
	Proxy     string   `json:"proxy"`
	Wallet    string   `json:"wallet"`
	Version   string   `json:"version"`
	Root      string   `json:"root"`
	Proof     []string `json:"proof"`
}

// ProveV3Wallet re-audits *every* admission and the original Safe receipt on
// each request. The immutable review/decision/observation are necessary but
// cannot become an authority without current canonical chain, exact runtime,
// cursor, member set and signed-in verified wallet. Never cache this result.
func ProveV3Wallet(uid int, projectID uint64, rawWallet string, sessionVersion uint64) (V3WalletProof, error) {
	return proveV3WalletWithAdmission(uid, projectID, rawWallet, sessionVersion, false)
}

// ProveApprovedV3Wallet checks wallet ownership and the approved full member
// snapshot locally. Chain/Safe/runtime audits belong to publication and actual
// execution, not every proof read; this route uses no cursor or RPC.
func ProveApprovedV3Wallet(uid int, projectID uint64, rawWallet string, sessionVersion uint64) (V3WalletProof, error) {
	return proveApprovedV3Membership(uid, projectID, rawWallet, sessionVersion)
}

func v3WalletProofReadBudget(approved bool) time.Duration {
	if approved {
		// Approved membership proofs use only DB + local computation.
		return 10 * time.Second
	}
	return 15 * time.Second
}

func proveV3WalletWithAdmission(uid int, projectID uint64, rawWallet string, sessionVersion uint64, approved bool) (V3WalletProof, error) {
	if approved {
		return proveApprovedV3Membership(uid, projectID, rawWallet, sessionVersion)
	}
	invalid := errors.New("V3 wallet qualification unavailable")
	if !viper.GetBool("idoV3PolicyProof.enabled") || !viper.GetBool("idoV3PolicyJournal.enabled") || uid <= 0 || projectID == 0 || sessionVersion == 0 || db.Mysql() == nil ||
		!common.IsHexAddress(rawWallet) || common.HexToAddress(rawWallet) == (common.Address{}) {
		return V3WalletProof{}, invalid
	}
	wallet := strings.ToLower(common.HexToAddress(rawWallet).Hex())
	ctx, cancel := context.WithTimeout(context.Background(), v3WalletProofReadBudget(approved))
	defer cancel()
	var admitted idov3whitelist.AdmittedProject
	var set idov3whitelist.SetManifest
	var ownerHash, governorHash common.Hash
	var rpc *ethclient.Client
	var err error
	if approved {
		ownerHash, governorHash, err = approvedV3SafeHashes()
		if err != nil {
			return V3WalletProof{}, invalid
		}
		rpc, err = v3TokenGateRPC(ctx)
		if err != nil {
			return V3WalletProof{}, invalid
		}
	} else {
		admitted, set, _, _, _, err = pinnedV3Admission(projectID, false)
		if err != nil {
			return V3WalletProof{}, invalid
		}
		rpc, err = chainrpc.Dial(ctx, viper.GetString("chain.project.url"))
	}
	if err != nil {
		return V3WalletProof{}, invalid
	}
	defer rpc.Close()
	if approved {
		admitted, err = chaintx.AdmitApprovedLockedProject(ctx, db.Mysql(), rpc, projectID, ownerHash, governorHash)
		if err != nil || idov3whitelist.RequireApprovedV3ProjectScan(ctx, db.Mysql(), projectID, false) != nil {
			return V3WalletProof{}, invalid
		}
	}
	var reviewID uint64
	var p V3PolicyPreview
	var txHash, safeHash, receiptHash, attDigest string
	var receiptBlock uint64
	var initiator, approver uint64
	err = db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if chainreorg.LockNoActiveIncidentTx(tx, admitted.Binding.ChainID, strings.ToLower(admitted.Binding.ProxyAddress.Hex())) != nil ||
			chaintx.LockV3RunningCursorTx(tx, admitted.Binding) != nil {
			return invalid
		}
		var identity []entity.UserWalletIdentity
		read := tx.Raw(`SELECT * FROM user_wallet_identity WHERE BINARY wallet_address=BINARY ? AND uid=? AND chain='evm' LIMIT 2`, wallet, uid).Scan(&identity)
		if read.Error != nil || len(identity) != 1 || identity[0].ClaimMethod == "" {
			return invalid
		}
		var bound []entity.UserWallet
		read = tx.Raw(`SELECT * FROM user_wallet WHERE uid=? AND LOWER(wallet_address)=? AND chain='evm'
			AND verified_at IS NOT NULL LIMIT 2`, uid, wallet).Scan(&bound)
		if read.Error != nil || len(bound) != 1 || bound[0].VerificationMethod == "" {
			return invalid
		}
		var user entity.User
		if tx.Select("id,is_enable,session_version").Where("id=? AND is_enable=1", uid).Take(&user).Error != nil || user.SessionVersion != sessionVersion {
			return invalid
		}
		var records []struct {
			ID        uint64 `gorm:"column:id"`
			Tx        string `gorm:"column:tx_hash"`
			Safe      string `gorm:"column:safe_tx_hash"`
			Receipt   string `gorm:"column:receipt_hash"`
			Block     uint64 `gorm:"column:receipt_block"`
			Digest    string `gorm:"column:attestation_digest"`
			Initiator uint64 `gorm:"column:initiator_admin_id"`
			Approver  uint64 `gorm:"column:approver_admin_id"`
		}
		read = tx.Raw(`SELECT r.id,e.tx_hash,e.safe_tx_hash,e.receipt_hash,e.receipt_block,e.attestation_digest,
			r.initiator_admin_id,d.approver_admin_id FROM ido_v3_policy_review r
			JOIN ido_v3_policy_review_decision d ON d.review_id=r.id AND d.decision='approved'
			JOIN ido_v3_policy_review_execution e ON e.review_id=r.id
			WHERE r.project_id=? AND r.chain_id=? AND BINARY r.proxy_address=BINARY ?
			AND r.target_version=(SELECT MAX(newest.target_version) FROM ido_v3_policy_review newest
				JOIN ido_v3_policy_review_decision nd ON nd.review_id=newest.id AND nd.decision='approved'
				JOIN ido_v3_policy_review_execution ne ON ne.review_id=newest.id
				WHERE newest.project_id=r.project_id AND newest.chain_id=r.chain_id AND BINARY newest.proxy_address=BINARY r.proxy_address)
			AND r.enabled=1 LIMIT 2`, projectID, admitted.Binding.ChainID, strings.ToLower(admitted.Binding.ProxyAddress.Hex())).Scan(&records)
		if read.Error != nil || len(records) != 1 || records[0].ID == 0 || records[0].Initiator == 0 ||
			records[0].Approver == 0 || records[0].Initiator == records[0].Approver {
			return invalid
		}
		row := records[0]
		reviewID, txHash, safeHash, receiptHash, receiptBlock, attDigest, initiator, approver =
			row.ID, row.Tx, row.Safe, row.Receipt, row.Block, row.Digest, row.Initiator, row.Approver
		p, err = V3PolicyReviewReadTx(tx, projectID, reviewID)
		if err != nil {
			return invalid
		}
		return nil
	})
	if err != nil {
		return V3WalletProof{}, invalid
	}
	scope, commitment, err := v3PolicyIdentity(p, projectID)
	if err != nil || scope.ChainID.Uint64() != admitted.Binding.ChainID || scope.ProxyAddress != admitted.Binding.ProxyAddress ||
		!v3CanonicalHash(txHash) || !v3CanonicalHash(safeHash) || !v3CanonicalHash(receiptHash) || !v3CanonicalHash(attDigest) || receiptBlock == 0 {
		return V3WalletProof{}, invalid
	}
	if approved {
		var execution approvedV3PolicyExecutionRecord
		err = db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var e error
			execution, e = readApprovedV3PolicyExecutionTx(tx, projectID, reviewID)
			return e
		})
		if err != nil || execution.Preview.PreviewDigest != p.PreviewDigest ||
			execution.InitiatorID != initiator || execution.ApproverID != approver ||
			execution.TxHash != txHash || execution.SafeTxHash != safeHash ||
			execution.Digest != attDigest || execution.ReceiptHash != receiptHash || execution.ReceiptBlock != receiptBlock {
			return V3WalletProof{}, invalid
		}
	} else {
		att, digest, e := pinnedV3SafeAttestation(p, common.HexToAddress(set.Governor), false)
		if e != nil || att.TxHash != txHash || att.SafeTxHash != safeHash || digest != attDigest {
			return V3WalletProof{}, invalid
		}
	}
	if p.OldEnabled && v3VerifyPriorReceipt(ctx, rpc, admitted, p, projectID) != nil {
		return V3WalletProof{}, invalid
	}
	if approved && p.OldEnabled && verifyApprovedV3PriorPolicy(ctx, db.Mysql(), rpc, admitted, p, projectID) != nil {
		return V3WalletProof{}, invalid
	}
	evidence, err := idov3whitelist.ReconcilePolicyEvidence(ctx, rpc, scope, admitted.Runtime, commitment, common.HexToHash(txHash), admitted.Confirmations)
	if err != nil || evidence.ReceiptBlock.BlockNumber != receiptBlock ||
		strings.ToLower(evidence.ReceiptBlock.BlockHash.Hex()) != receiptHash {
		return V3WalletProof{}, invalid
	}
	receipt, err := rpc.TransactionReceipt(ctx, common.HexToHash(txHash))
	if err != nil || receipt == nil || receipt.BlockHash != evidence.ReceiptBlock.BlockHash {
		return V3WalletProof{}, invalid
	}
	change := idov3whitelist.PolicyChange{ExpectedVersion: mustParseV3Version(p.OldVersion), TargetVersion: commitment.Version,
		Enabled: true, Root: commitment.Root, Candidate: &commitment}
	governor := common.HexToAddress(set.Governor)
	if approved {
		governor = common.HexToAddress(v3DiagnosticGovernor)
	}
	if idov3whitelist.CheckSafePolicyExecution(ctx, rpc, scope, governor, change,
		common.HexToHash(txHash), common.HexToHash(safeHash), receipt) != nil {
		return V3WalletProof{}, invalid
	}
	governance := idov3whitelist.GovernanceIdentity{Owner: common.HexToAddress(set.OwnerSafe), Governor: common.HexToAddress(set.Governor),
		OwnerCodeHash: common.HexToHash(set.OwnerCodeHash), GovernorCodeHash: common.HexToHash(set.GovernorCodeHash)}
	if approved {
		governance = idov3whitelist.GovernanceIdentity{FollowOwner: true, Governor: governor,
			OwnerCodeHash: ownerHash, GovernorCodeHash: governorHash}
	}
	for i, obs := range []idov3whitelist.PolicyObservation{evidence.ReceiptBlock, evidence.Head} {
		if _, err = idov3whitelist.ReadGovernanceAt(ctx, rpc, scope, admitted.Runtime, governance, obs.BlockNumber, obs.BlockHash); err != nil {
			return V3WalletProof{}, invalid
		}
		project, readErr := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, admitted.Runtime, obs.BlockNumber, obs.BlockHash)
		// Initial policy enable is deliberately allowed while Pending. Only
		// the current confirmed head (and the latest tip below) must be Active
		// before an actual buyer receives a proof.
		if readErr != nil || idov3whitelist.MatchProjectIdentityObservation(admitted.Binding, project) != nil ||
			(i == 0 && project.State != idov3whitelist.ProjectPending && project.State != idov3whitelist.ProjectActive) ||
			(i == 1 && project.State != idov3whitelist.ProjectActive) {
			return V3WalletProof{}, invalid
		}
	}
	// The confirmed observation establishes finality of the original receipt;
	// the latest block independently closes the window in which a newer policy,
	// project termination or proxy upgrade is visible but not yet confirmed.
	latest, err := rpc.BlockNumber(ctx)
	if err != nil || latest < evidence.Head.BlockNumber {
		return V3WalletProof{}, invalid
	}
	latestNumber := new(big.Int).SetUint64(latest)
	latestHeader, err := rpc.HeaderByNumber(ctx, latestNumber)
	if err != nil || latestHeader == nil || latestHeader.Number == nil || latestHeader.Number.Cmp(latestNumber) != 0 {
		return V3WalletProof{}, invalid
	}
	latestPolicy, err := idov3whitelist.ReadPolicyAt(ctx, rpc, scope, admitted.Runtime, latest, latestHeader.Hash())
	if err != nil || !latestPolicy.Policy.Enabled || latestPolicy.Policy.Version != commitment.Version || latestPolicy.Policy.Root != commitment.Root {
		return V3WalletProof{}, invalid
	}
	if _, err := idov3whitelist.ReadGovernanceAt(ctx, rpc, scope, admitted.Runtime, governance, latest, latestHeader.Hash()); err != nil {
		return V3WalletProof{}, invalid
	}
	latestProject, err := idov3whitelist.ReadProjectIdentityAt(ctx, rpc, scope, admitted.Runtime, latest, latestHeader.Hash())
	if err != nil || idov3whitelist.MatchProjectIdentityObservation(admitted.Binding, latestProject) != nil ||
		latestProject.State != idov3whitelist.ProjectActive {
		return V3WalletProof{}, invalid
	}
	if checkV3ExecutionCursor(ctx, rpc, admitted) != nil {
		return V3WalletProof{}, invalid
	}
	var anchor []struct {
		Block uint64 `gorm:"column:cursor_block"`
		Hash  string `gorm:"column:cursor_hash"`
		State string `gorm:"column:state"`
	}
	read := db.Mysql().WithContext(ctx).Raw(`SELECT cursor_block,cursor_hash,state FROM ido_v3_scan_cursor WHERE chain_id=? AND proxy_address=? AND project_id=?
		AND contract_version='ido_v3_stablecoin' LIMIT 2`, admitted.Binding.ChainID, p.Proxy, projectID).Scan(&anchor)
	if read.Error != nil || len(anchor) != 1 || anchor[0].State != "running" || anchor[0].Block < evidence.ReceiptBlock.BlockNumber ||
		!v3CanonicalHash(anchor[0].Hash) {
		return V3WalletProof{}, invalid
	}
	// The V3 Job cursor must have consumed the policy event itself, not just
	// project purchase events, before a proof can be served.
	var observed int64
	read = db.Mysql().WithContext(ctx).Raw(`SELECT COUNT(*) FROM ido_v3_policy_event_journal WHERE chain_id=? AND proxy_address=?
		AND project_id=? AND tx_hash=? AND block_number=? AND block_hash=? AND version=? AND root=? AND enabled=1 AND state='canonical'`,
		admitted.Binding.ChainID, p.Proxy, projectID, txHash, receiptBlock, receiptHash, commitment.Version, strings.ToLower(commitment.Root.Hex())).Scan(&observed)
	if read.Error != nil || observed != 1 {
		return V3WalletProof{}, invalid
	}
	// The exact scanned log must be backed by a canonical project checkpoint,
	// not merely by an insertable policy-journal row. Recheck this fence again
	// under the final cursor lock before returning the single-wallet proof.
	var checkpoint int64
	read = db.Mysql().WithContext(ctx).Raw(`SELECT COUNT(*) FROM ido_v3_block_checkpoint WHERE chain_id=?
		AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'
		AND block_number=? AND block_hash=? AND status='canonical'`,
		admitted.Binding.ChainID, p.Proxy, projectID, receiptBlock, receiptHash).Scan(&checkpoint)
	if read.Error != nil || checkpoint != 1 {
		return V3WalletProof{}, invalid
	}
	record := idov3whitelist.ApprovedSnapshot{Scope: scope, ExpectedVersion: mustParseV3Version(p.OldVersion), Commitment: commitment,
		Members: p.CanonicalMembers, InitiatorID: initiator, ApproverID: approver, ApprovalID: reviewID, ExecutionTx: common.HexToHash(txHash)}
	// VerifyApprovedSnapshot expects EIP-55 canonical member addresses; the
	// append-only DB stores lower-case values and they are normalized solely
	// for this complete-set verification, never sourced from Warmup.
	for i, w := range record.Members {
		record.Members[i] = common.HexToAddress(w).Hex()
	}
	tree, err := idov3whitelist.VerifyApprovedSnapshot(record, evidence)
	if err != nil {
		return V3WalletProof{}, invalid
	}
	version, siblings, err := tree.ProofForPolicy(scope, evidence.Head.Policy, wallet)
	if err != nil {
		return V3WalletProof{}, invalid
	}
	proof := make([]string, len(siblings))
	for i, s := range siblings {
		proof[i] = s.Hex()
	}
	// A final head/cursor read bounds the response window. Any subsequent
	// reorg invalidates it; wallet transaction must recheck version/root.
	head, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(evidence.Head.BlockNumber))
	if err != nil || head == nil || head.Hash() != evidence.Head.BlockHash ||
		checkV3ExecutionCursor(ctx, rpc, admitted) != nil {
		return V3WalletProof{}, invalid
	}
	latestAgain, err := rpc.HeaderByNumber(ctx, latestNumber)
	if err != nil || latestAgain == nil || latestAgain.Hash() != latestHeader.Hash() {
		return V3WalletProof{}, invalid
	}
	// A later Safe observation written during this request invalidates the
	// earlier selected version even if the scanner is still catching up.
	// Recheck publication under the same durable cursor/reorg locks that the
	// scanner uses. A halt between the earlier reads and response must not
	// leave a previously validated proof available to this request.
	if err := db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if chainreorg.LockNoActiveIncidentTx(tx, admitted.Binding.ChainID, p.Proxy) != nil ||
			chaintx.LockV3RunningCursorTx(tx, admitted.Binding) != nil {
			return invalid
		}
		// Wallet revocation, unbinding or session invalidation while the RPC
		// reads were in flight must close this response, too.
		var owned int64
		if tx.Raw(`SELECT COUNT(*) FROM user_wallet_identity i JOIN user_wallet w
			ON w.uid=i.uid AND BINARY LOWER(w.wallet_address)=BINARY i.wallet_address AND w.chain=i.chain
			JOIN user u ON u.id=i.uid WHERE i.uid=? AND BINARY i.wallet_address=BINARY ?
			AND i.chain='evm' AND i.claim_method<>'' AND w.verified_at IS NOT NULL
			AND w.verification_method<>'' AND u.is_enable=1 AND u.session_version=?`, uid, wallet, sessionVersion).
			Scan(&owned).Error != nil || owned != 1 {
			return invalid
		}
		var superseding int64
		if tx.Raw(`SELECT COUNT(*) FROM ido_v3_policy_review r JOIN ido_v3_policy_review_execution e ON e.review_id=r.id
			WHERE r.chain_id=? AND BINARY r.proxy_address=BINARY ? AND r.project_id=? AND r.target_version>?`,
			admitted.Binding.ChainID, p.Proxy, projectID, commitment.Version).Scan(&superseding).Error != nil || superseding != 0 {
			return invalid
		}
		var count int64
		if tx.Raw(`SELECT COUNT(*) FROM ido_v3_policy_event_journal WHERE chain_id=? AND proxy_address=?
			AND project_id=? AND tx_hash=? AND block_number=? AND block_hash=? AND version=? AND root=? AND enabled=1 AND state='canonical'`,
			admitted.Binding.ChainID, p.Proxy, projectID, txHash, receiptBlock, receiptHash, commitment.Version,
			strings.ToLower(commitment.Root.Hex())).Scan(&count).Error != nil || count != 1 {
			return invalid
		}
		if tx.Raw(`SELECT COUNT(*) FROM ido_v3_block_checkpoint WHERE chain_id=?
			AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'
			AND block_number=? AND block_hash=? AND status='canonical'`,
			admitted.Binding.ChainID, p.Proxy, projectID, receiptBlock, receiptHash).Scan(&checkpoint).Error != nil || checkpoint != 1 {
			return invalid
		}
		if approved {
			record, e := readApprovedV3PolicyExecutionTx(tx, projectID, reviewID)
			if e != nil || record.Preview.PreviewDigest != p.PreviewDigest ||
				record.TxHash != txHash || record.SafeTxHash != safeHash ||
				record.Digest != attDigest ||
				record.ReceiptBlock != receiptBlock || record.ReceiptHash != receiptHash ||
				record.InitiatorID != initiator || record.ApproverID != approver {
				return invalid
			}
		}
		return nil
	}); err != nil {
		return V3WalletProof{}, invalid
	}
	// Normal head growth is not evidence of policy change. The pinned policy
	// and header were checked above; the contract checks version at purchase.
	if approved {
		fresh, e := chaintx.AdmitApprovedLockedProject(ctx, db.Mysql(), rpc, projectID, ownerHash, governorHash)
		if e != nil || fresh != admitted || idov3whitelist.RequireApprovedV3ProjectScan(ctx, db.Mysql(), projectID, false) != nil {
			return V3WalletProof{}, invalid
		}
	}
	return V3WalletProof{ChainID: strconv.FormatUint(admitted.Binding.ChainID, 10), ProjectID: strconv.FormatUint(projectID, 10),
		Proxy: p.Proxy, Wallet: wallet, Version: strconv.FormatUint(version, 10), Root: commitment.Root.Hex(), Proof: proof}, nil
}

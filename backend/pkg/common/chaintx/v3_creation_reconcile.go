package chaintx

import (
	"context"
	"crypto/ed25519"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/gorm"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// ReconcileV3CreationReceipt audits an ALREADY signed creation using its
// original hash and the two independent offline admission domains. It never
// signs, broadcasts, resends, locks a project or marks finance applied.
// A revert releases the original sender slot only after confirmed receipt;
// successful creation remains final_success until the canonical Job bridge.
func ReconcileV3CreationReceipt(ctx context.Context, database *gorm.DB, rpc V3LifecycleRecoveryRPC,
	setKey ed25519.PublicKey, set idov3whitelist.SetManifest, setSignature []byte,
	scanKey ed25519.PublicKey, scan idov3whitelist.AdmissionManifest, scanSignature []byte,
	operator common.Address, submissionID uint64) (*entity.ChainTxSubmission, error) {
	invalid := errors.New("V3 creation receipt reconciliation unavailable")
	if ctx == nil || database == nil || rpc == nil || submissionID == 0 || operator == (common.Address{}) ||
		operator != common.HexToAddress(set.Operator) || scan.ChainID != set.ChainID || scan.ProjectID != set.ProjectID ||
		scan.ProxyAddress != set.ProxyAddress || scan.ProjectToken != set.ProjectToken || scan.PaymentToken != set.PaymentToken ||
		set.ExpiresAt <= 1 || idov3whitelist.VerifySignedSetManifest(setKey, set, setSignature,
		common.HexToHash(set.RequestDigest), time.Unix(set.ExpiresAt-1, 0)) != nil ||
		idov3whitelist.CheckCreationSetCommitment(ctx, database, set, scan) != nil {
		return nil, invalid
	}
	admitted, err := idov3whitelist.AdmitSignedCreation(ctx, database, scanKey, scan, scanSignature, time.Now())
	if err != nil {
		return nil, invalid
	}
	row := entity.ChainTxSubmission{}
	if database.WithContext(ctx).Where("id = ?", submissionID).Take(&row).Error != nil ||
		row.Action != "ido_v3_create" || !v3ReconcilableRow(row, admitted.Binding) ||
		!strings.EqualFold(row.SenderAddress, operator.Hex()) {
		return nil, invalid
	}
	if row.State == constant.ChainTxStateFinalSuccess || row.State == constant.ChainTxStateFinalRevert ||
		row.State == constant.ChainTxStateApplied || row.State == ChainTxStateV3ReorgSuspended ||
		!V3SignedBeforeSet(row, set.ExpiresAt) ||
		basechain.VerifyV3ActionCall(row.Action, admitted.Binding.ProjectID, row.CallData) != nil {
		return nil, invalid
	}
	audit, err := AuditV3PersistedReceipt(ctx, row, admitted.Binding, rpc, admitted.Confirmations)
	if err != nil {
		return nil, invalid
	}
	// Neither a receipt nor a database candidate authenticates the V3 runtime.
	// Verify the signed Set's runtime, owner/governor and token code at a fixed
	// confirmed block before changing an already signed row.
	if _, err := idov3whitelist.InspectSetRuntimeAt(ctx, rpc, set); err != nil {
		return nil, invalid
	}
	scope := idov3whitelist.Scope{ChainID: new(big.Int).SetUint64(admitted.Binding.ChainID),
		ProjectID: new(big.Int).SetUint64(admitted.Binding.ProjectID), ProxyAddress: admitted.Binding.ProxyAddress}
	if idov3whitelist.ReadScanOperatorAt(ctx, rpc, scope, admitted.Runtime, operator, audit.Block, audit.BlockHash) != nil {
		return nil, invalid
	}
	// A reverted create has no ProjectCreated log, so its confirmed block is
	// not required to be the separately signed successful-creation height.
	// It still must be audited by original hash before releasing the nonce.
	if (!audit.Reverted && audit.Block != scan.CreationBlock) ||
		idov3whitelist.VerifySignedSetManifest(setKey, set, setSignature,
			common.HexToHash(set.RequestDigest), time.Unix(set.ExpiresAt-1, 0)) != nil ||
		idov3whitelist.CheckCreationSetCommitment(ctx, database, set, scan) != nil {
		return nil, invalid
	}
	if _, err := idov3whitelist.AdmitSignedCreation(ctx, database, scanKey, scan, scanSignature, time.Now()); err != nil {
		return nil, invalid
	}
	header, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(audit.Block))
	if err != nil || header == nil || header.Number == nil || !header.Number.IsUint64() ||
		header.Number.Uint64() != audit.Block || header.Hash() != audit.BlockHash {
		return nil, invalid
	}
	var updated entity.ChainTxSubmission
	if database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Recheck exact pre-lock project identity and immutable Set operator under
		// the shared reorg gate before the submission and sender nonce locks.
		if chainreorg.LockNoActiveIncidentTx(tx, admitted.Binding.ChainID, scan.ProxyAddress) != nil {
			return invalid
		}
		var rows []struct {
			ProjectID uint64 `gorm:"column:project_id"`
			CallHash  string `gorm:"column:create_calldata_hash"`
		}
		read := tx.Raw(`SELECT p.project_id,c.create_calldata_hash FROM projects p JOIN ido_v3_project_binding b
			ON b.project_id=p.project_id AND b.chain_id=p.chain_id AND BINARY b.proxy_address=BINARY p.contract_address
			AND BINARY b.contract_version=BINARY p.contract_version
			JOIN ido_v3_set_create_commitment c ON c.project_id=b.project_id AND c.chain_id=b.chain_id
			AND BINARY c.proxy_address=BINARY b.proxy_address AND BINARY c.contract_version=BINARY b.contract_version
			WHERE p.project_id=? AND p.chain_id=? AND BINARY p.contract_address=BINARY ?
			AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin' AND BINARY p.funding_mode=BINARY 'capped_presale'
			AND p.onchain_locked=0 AND BINARY p.status=BINARY 'Set'
			AND BINARY p.token_address=BINARY ? AND BINARY p.payment_token_address=BINARY ?
			AND BINARY c.operator_address=BINARY ? LIMIT 2 FOR UPDATE`, admitted.Binding.ProjectID, admitted.Binding.ChainID,
			scan.ProxyAddress, scan.ProjectToken, scan.PaymentToken, strings.ToLower(operator.Hex())).Scan(&rows)
		if read.Error != nil || len(rows) != 1 || rows[0].ProjectID != admitted.Binding.ProjectID ||
			rows[0].CallHash != strings.ToLower(crypto.Keccak256Hash(row.CallData).Hex()) ||
			scan.ExpiresAt <= time.Now().Unix() {
			return invalid
		}
		// The finalizer below locks and compares this original signed attempt.
		var finishErr error
		updated, finishErr = finalizeV3CreationReceiptLockedTx(tx, row, admitted.Binding, operator, audit, set.ExpiresAt)
		return finishErr
	}) != nil {
		return nil, invalid
	}
	return &updated, nil
}

package chainevent

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/idov3whitelist"
)

func historicalV3SetReviewTime(expires int64) time.Time {
	if expires <= 1 {
		return time.Time{}
	}
	return time.Unix(expires-1, 0)
}

func (j *V3ScannerJob) reconcilePrelockCreate(ctx context.Context) error {
	if j == nil || ctx == nil || strings.TrimSpace(j.RPCURL) == "" {
		return errors.New("V3 creation recovery RPC unavailable")
	}
	// No signer or broadcast capability is constructed here. The receipt
	// wrapper authenticates chain, runtime, Set, scan and persisted raw bytes.
	rpc, err := chainrpc.Dial(ctx, j.RPCURL)
	if err != nil {
		return errors.New("V3 creation recovery RPC unavailable")
	}
	defer rpc.Close()
	return j.reconcileSignedReceipt(ctx, rpc, "ido_v3_create")
}

// A crashed Core may leave a nonce-free intent/ready row behind. The Job
// holds the distinct signed scanner/Set admission before entering this path;
// the storage primitive locks reorg, cursor, project and the exact unsigned
// row, and requires a DB-local five-minute age before cancelling it. This
// path never handles signed bytes, and cannot create a new attempt.
func (j *V3ScannerJob) cancelStaleUnsignedIntent(ctx context.Context) error {
	invalid := errors.New("V3 stale unsigned cleanup unavailable")
	if j == nil || ctx == nil || j.Database == nil || j.Operator == (common.Address{}) ||
		j.Admission.Manifest.ChainID == 0 || j.Admission.Manifest.ProjectID == 0 ||
		!common.IsHexAddress(j.Admission.Manifest.ProxyAddress) ||
		!common.IsHexAddress(j.Admission.Manifest.ProjectToken) ||
		!common.IsHexAddress(j.Admission.Manifest.PaymentToken) ||
		common.HexToAddress(j.Admission.Manifest.ProxyAddress) == (common.Address{}) ||
		common.HexToAddress(j.Admission.Manifest.ProjectToken) == common.HexToAddress(j.Admission.Manifest.PaymentToken) {
		return invalid
	}
	m := j.Admission.Manifest
	var rows []struct {
		ID uint64 `gorm:"column:id"`
	}
	read := j.Database.WithContext(ctx).Raw(`SELECT id FROM chain_tx_submission WHERE chain_id=? AND project_id=?
		AND BINARY contract_address=BINARY ? AND BINARY sender_address=BINARY ?
		AND action IN ('ido_v3_create','ido_v3_activate','ido_v3_end','ido_v3_withdraw_platform_fee')
		AND active_slot=1 AND state IN ('intent','ready') AND nonce IS NULL AND gas_limit IS NULL
		AND raw_tx IS NULL AND tx_hash IS NULL
		AND updated_at < CURRENT_TIMESTAMP(6) - INTERVAL 5 MINUTE ORDER BY id LIMIT 1`, m.ChainID,
		m.ProjectID, strings.ToLower(m.ProxyAddress), strings.ToLower(j.Operator.Hex())).Scan(&rows)
	if read.Error != nil {
		return invalid
	}
	if len(rows) == 0 {
		return nil
	}
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: m.ChainID, ProjectID: m.ProjectID,
		ProxyAddress: common.HexToAddress(m.ProxyAddress), ProjectToken: common.HexToAddress(m.ProjectToken),
		PaymentToken: common.HexToAddress(m.PaymentToken)}
	if err := j.Database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return chaintx.CancelStaleUnsignedV3IntentTx(tx, binding, j.Operator, rows[0].ID)
	}); err != nil {
		return invalid
	}
	return nil
}

// reconcileSignedReceipt never sends a transaction. Only the exact active
// persisted signed attempt for this release-pinned sender may be audited.
// Empty action selects lifecycle attempts after a successful scanner tick;
// create is checked even before the first ProjectCreated journal exists.
func (j *V3ScannerJob) reconcileSignedReceipt(ctx context.Context, rpc chaintx.V3LifecycleRecoveryRPC, action string) error {
	if j == nil || ctx == nil || j.Database == nil || rpc == nil || j.Operator == (common.Address{}) ||
		!common.IsHexAddress(j.Admission.Manifest.ProxyAddress) || j.Admission.Manifest.ProjectID == 0 ||
		j.Admission.Manifest.ChainID == 0 || (action != "" && action != "ido_v3_create") {
		return errors.New("V3 receipt recovery scope unavailable")
	}
	m := j.Admission.Manifest
	var rows []struct {
		ID     uint64 `gorm:"column:id"`
		Action string `gorm:"column:action"`
	}
	query := `SELECT id,action FROM chain_tx_submission WHERE chain_id=? AND project_id=?
		AND BINARY contract_address=BINARY ? AND BINARY sender_address=BINARY ?
		AND active_slot=1 AND state IN ? AND action IN ? ORDER BY id LIMIT 2`
	actions := []string{"ido_v3_activate", "ido_v3_end", "ido_v3_withdraw_platform_fee"}
	if action == "ido_v3_create" {
		actions = []string{action}
	}
	read := j.Database.WithContext(ctx).Raw(query, m.ChainID, m.ProjectID,
		strings.ToLower(m.ProxyAddress), strings.ToLower(j.Operator.Hex()),
		[]string{constant.ChainTxStateSigned, constant.ChainTxStateSubmitted, constant.ChainTxStateBroadcastUnknown,
			constant.ChainTxStateMinedSuccess, constant.ChainTxStateMinedRevert}, actions).Scan(&rows)
	if read.Error != nil || len(rows) > 1 {
		return errors.New("V3 signed attempt selection unavailable")
	}
	if len(rows) == 0 {
		return nil
	}
	if action == "ido_v3_create" {
		_, err := chaintx.ReconcileV3CreationReceipt(ctx, j.Database, rpc, j.SetKey, j.SetManifest, j.SetSignature,
			j.Admission.PublicKey, m, j.Admission.Signature, j.Operator, rows[0].ID)
		return err
	}
	set := j.SetManifest
	// A historical Set is an immutable identity commitment, not a new
	// lifecycle authorization. The current signed scanner decision and
	// independent Set commitment must both still match this project.
	if idov3whitelist.VerifySignedSetManifest(j.SetKey, set, j.SetSignature,
		common.HexToHash(set.RequestDigest), historicalV3SetReviewTime(set.ExpiresAt)) != nil ||
		idov3whitelist.CheckCreationSetCommitment(ctx, j.Database, set, m) != nil ||
		common.HexToAddress(set.Operator) != j.Operator {
		return errors.New("V3 lifecycle identity commitment unavailable")
	}
	governance := idov3whitelist.GovernanceIdentity{Owner: common.HexToAddress(set.OwnerSafe),
		Governor: common.HexToAddress(set.Governor), OwnerCodeHash: common.HexToHash(set.OwnerCodeHash),
		GovernorCodeHash: common.HexToHash(set.GovernorCodeHash)}
	_, err := chaintx.ReconcileV3LifecycleReceipt(ctx, j.Database, rpc, j.Admission.PublicKey,
		m, j.Admission.Signature, governance, j.Operator, rows[0].ID)
	return err
}

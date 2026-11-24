package chainevent

import (
	"context"
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/log"
)

// Bounded receipt-only lane: does not consume project ranges, publish a
// freshness certificate, bridge a successful create, sign, send, replace or
// refresh exemptions. Existing reconcilers retain every authority check.
// Shared j.mu prevents concurrent writes with this job's project consumer.
// Default off; source deployment alone does not enable production writes.
func (j *ApprovedV3ScannerJob) runPriorityReceipts() {
	if j == nil || j.Database == nil || !viper.GetBool("idoV3OriginalReconciliation.enabled") || viper.GetString("application.environment") != "prod" || j.Stream.Mode != "stream" || j.OwnerHash == (common.Hash{}) || j.GovernorHash == (common.Hash{}) || idov3whitelist.ValidateApprovedV3RPCEndpoint(j.RPCURL) != nil {
		return
	}
	if !j.mu.TryLock() {
		return
	}
	defer j.mu.Unlock()
	parent := j.Context
	if parent == nil {
		parent = context.Background()
	}
	j.runPriorityReceiptsLocked(parent)
}

// Also called at the start of Run while mu is held: cron registration order
// cannot starve receipt work behind the full 50s consumer turn.
func (j *ApprovedV3ScannerJob) runPriorityReceiptsLocked(parent context.Context) {
	if !viper.GetBool("idoV3OriginalReconciliation.enabled") || viper.GetString("application.environment") != "prod" || j.Stream.Mode != "stream" || j.Database == nil || j.OwnerHash == (common.Hash{}) || j.GovernorHash == (common.Hash{}) || idov3whitelist.ValidateApprovedV3RPCEndpoint(j.RPCURL) != nil {
		return
	}
	if time.Now().Before(j.receiptsNext) {
		return
	}
	// At most one active original for the fixed production sender. Ambiguous
	// selection refuses work; neither LIMIT nor ORDER picks an arbitrary row.
	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	defer cancel()
	j.receiptsNext = time.Now().Add(30 * time.Second)
	var identity struct{ UUID, DB string }
	if j.Database.WithContext(ctx).Raw(`SELECT @@server_uuid AS uuid,DATABASE() AS db`).Scan(&identity).Error != nil || identity.UUID != "78dbed8b-77a9-11f1-ac1c-06eab0296b63" || identity.DB != "joincare" {
		return
	}
	target, err := selectV3PriorityReceipt(ctx, j.Database)
	if err != nil || target == nil {
		return
	}
	rpc, err := chainrpc.DialScanner(ctx, j.RPCURL)
	if err != nil {
		return
	}
	defer rpc.Close()
	started := time.Now()
	// A missing receipt is ordinary unknown, NOT an audit authorization. This
	// cheap first read avoids expensive historical runtime admission every poll.
	outcome := runV3PriorityReceipt(ctx, *target, rpc, func() error {
		if target.Action == "ido_v3_create" {
			_, err = chaintx.ReconcileApprovedV3CreationReceipt(ctx, j.Database, rpc, target.ProjectID, target.ID, j.OwnerHash, j.GovernorHash)
		} else {
			_, err = chaintx.ReconcileApprovedV3LifecycleReceipt(ctx, j.Database, rpc, target.ProjectID, target.ID, j.OwnerHash, j.GovernorHash)
		}
		return err
	})
	if logger := log.GetLogger(); logger != nil {
		logger.WithFields(map[string]any{"submission_id": target.ID, "project_id": target.ProjectID, "outcome": outcome, "elapsed_ms": time.Since(started).Milliseconds()}).Info("V3 priority original receipt")
	}
}

// The callback can only be reached for a well-formed receipt of this exact
// original hash. It repeats canonical/finality/runtime checks in the existing
// reconciler; the cheap read itself is never finalization evidence.
func runV3PriorityReceipt(ctx context.Context, target v3PriorityReceiptTarget, rpc interface {
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
}, reconcile func() error) string {
	if ctx == nil || ctx.Err() != nil || rpc == nil || reconcile == nil {
		return "receipt_unavailable"
	}
	hash := common.HexToHash(target.Hash)
	r, err := rpc.TransactionReceipt(ctx, hash)
	if err != nil || r == nil {
		return "receipt_unavailable"
	}
	if r.TxHash != hash || r.BlockNumber == nil || r.BlockNumber.Sign() <= 0 || !r.BlockNumber.IsUint64() || r.BlockHash == (common.Hash{}) || (r.Status != 0 && r.Status != 1) {
		return "receipt_mismatch"
	}
	if err := reconcile(); err != nil {
		return "reconcile_rejected"
	}
	return "reconciled"
}

type v3PriorityReceiptTarget struct {
	ID, ProjectID uint64
	Action, Hash  string
}

func selectV3PriorityReceipt(ctx context.Context, db *gorm.DB) (*v3PriorityReceiptTarget, error) {
	invalid := errors.New("V3 original receipt selection unavailable")
	if ctx == nil || db == nil {
		return nil, invalid
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var rows []v3PriorityReceiptTarget
	r := db.WithContext(ctx).Raw(`SELECT s.id,s.project_id,s.action,s.tx_hash AS hash
 FROM chain_relayer_nonce n JOIN chain_tx_submission s ON s.id=n.active_submission_id
 AND s.chain_id=n.chain_id AND BINARY LOWER(s.sender_address)=BINARY LOWER(n.sender_address)
 WHERE n.chain_id=56 AND BINARY LOWER(n.sender_address)=BINARY '0xfd712061ee08e070a930ecd45e935ae5d7f5445c'
 AND s.active_slot=1 AND s.nonce IS NOT NULL AND n.next_nonce=s.nonce+1
 AND BINARY s.contract_address=BINARY '0x0d4040875742975269c55e16c8884d3ee9510b8e'
 AND BINARY s.action IN ('ido_v3_create','ido_v3_activate','ido_v3_end','ido_v3_withdraw_platform_fee')
 AND BINARY s.state IN ('signed','submitted','broadcast_unknown','mined_success','mined_revert')
 AND s.tx_hash IS NOT NULL LIMIT 2`).Scan(&rows)
	if r.Error != nil || len(rows) > 1 {
		return nil, invalid
	}
	if len(rows) == 0 {
		return nil, nil
	}
	row := rows[0]
	if row.ID == 0 || row.ProjectID == 0 || len(row.Hash) != 66 {
		return nil, invalid
	}
	// Hash parsing is strict; common.HexToHash silently accepts malformed text.
	for i, c := range row.Hash {
		if i < 2 {
			continue
		}
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return nil, invalid
		}
	}
	if row.Hash[:2] != "0x" || common.HexToHash(row.Hash) == (common.Hash{}) {
		return nil, invalid
	}
	return &row, nil
}

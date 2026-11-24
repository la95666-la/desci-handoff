package chainevent

import (
	"context"
	"errors"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	baseChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/job/chain"
	"joincare_server/pkg/job/db"
	"joincare_server/pkg/job/model"
)

type ChainTxRecoveryJob struct{}

func activeRecoverySubmission(database *gorm.DB, activeSubmissionID *uint64) (entity.ChainTxSubmission, bool, error) {
	if activeSubmissionID == nil {
		return entity.ChainTxSubmission{}, false, nil
	}
	active := entity.ChainTxSubmission{}
	if err := database.Select("id,action").Where("id = ?", *activeSubmissionID).Take(&active).Error; err != nil {
		return entity.ChainTxSubmission{}, false, err
	}
	return active, true, nil
}

func NewChainTxRecoveryJob() model.Add {
	var job cron.Job = new(ChainTxRecoveryJob)
	return job.(model.Add)
}

func (v *ChainTxRecoveryJob) Add(crontab *cron.Cron) {
	crontab.AddJob("15,45 * * * * *", cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(v))
}

func (v *ChainTxRecoveryJob) Run() {
	client := chain.GetClient()
	if client == nil {
		log.GetLogger().Error("chain transaction recovery client is not configured")
		return
	}
	if chain.V2Enabled() {
		if err := chainreorg.RequireNoActiveIncident(db.Mysql(), client.ChainID.Uint64(), chain.V2ContractAddress().Hex()); err != nil {
			log.GetLogger().Errorf("chain transaction recovery blocked by reorg gate: %s", err)
			return
		}
	}
	nonceState := entity.ChainRelayerNonce{}
	err := db.Mysql().Where("chain_id = ? AND sender_address = ?", client.ChainID.Uint64(), client.Address.Hex()).Take(&nonceState).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.GetLogger().Errorf("query relayer nonce fence err: %s", err)
		return
	}
	active, hasActive, activeErr := activeRecoverySubmission(db.Mysql(), nonceState.ActiveSubmissionID)
	if activeErr != nil {
		log.GetLogger().Errorf("query active durable chain transaction err: %s", activeErr)
		return
	}
	if hasActive && (chaintx.IsFairLaunchTransactionAction(active.Action) || chaintx.IsIDOV3TransactionAction(active.Action)) {
		// An active sender fence is exclusive. A disabled Fair Launch submission
		// must stop this run before any other progression query or RPC; skipping it
		// and dispatching another row would violate nonce ownership.
		return
	}
	// 兜底（HIGH-1 配套）：全局治理 submission 已 final_success 但 Paused/Unpaused
	// 事件因故未被消费时，signer fence 会一直被占住。Only run this after the
	// disabled Fair Launch active-fence check above.
	v.releaseSettledGlobalActions(client)
	if hasActive && !v.recoverOne(client, active.ID) {
		return
	}
	ready, err := nextReadyRecoverySubmission(db.Mysql(), client.ChainID.Uint64(), client.Address.Hex())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	if err != nil {
		log.GetLogger().Errorf("query durable chain transactions err: %s", err)
		return
	}
	v.recoverOne(client, ready.ID)
}

func nextReadyRecoverySubmission(database *gorm.DB, chainID uint64, sender string) (entity.ChainTxSubmission, error) {
	ready := entity.ChainTxSubmission{}
	err := database.Where("chain_id = ? AND sender_address = ? AND active_slot = 1 AND state = ? AND LOWER(TRIM(action)) NOT LIKE ? AND LOWER(TRIM(action)) NOT LIKE ? AND LOWER(TRIM(action)) NOT LIKE ?", chainID, sender, constant.ChainTxStateReady, "fair\\_launch\\_%", "v3\\_%", "ido\\_v3\\_%").Order("id ASC").Take(&ready).Error
	return ready, err
}

// releaseSettledGlobalActions 把已 final_success 的全局治理（pause/unpause）
// submission 标记 applied。交易已确认即治理动作已生效，推进状态是安全的。
func (v *ChainTxRecoveryJob) releaseSettledGlobalActions(client *baseChain.Client) {
	rows := make([]entity.ChainTxSubmission, 0, 4)
	err := db.Mysql().Where(
		"chain_id = ? AND sender_address = ? AND project_id = 0 AND state = ? AND action IN ?",
		client.ChainID.Uint64(), client.Address.Hex(), constant.ChainTxStateFinalSuccess,
		[]string{constant.TransactionTypeGlobalPause, constant.TransactionTypeGlobalUnpause},
	).Order("id ASC").Find(&rows).Error
	if err != nil {
		log.GetLogger().Errorf("query settled global governance submissions err: %s", err)
		return
	}
	for _, row := range rows {
		if err := db.Mysql().Transaction(func(tx *gorm.DB) error {
			// final_event_record_id=0：由恢复作业直接终结，无 material 事件记录。
			return chaintx.MarkAppliedTx(tx, row.ID, 0)
		}); err != nil {
			log.GetLogger().Errorf("release settled global governance submission err: submission_id=%d err=%s", row.ID, err)
		}
	}
}

func (v *ChainTxRecoveryJob) recoverOne(client *baseChain.Client, submissionID uint64) bool {
	row := entity.ChainTxSubmission{}
	if err := db.Mysql().Select("id,action").Where("id = ?", submissionID).Take(&row).Error; err != nil {
		log.GetLogger().Errorf("query durable chain transaction identity err: submission_id=%d err=%s", submissionID, err)
		return false
	}
	if chaintx.IsFairLaunchTransactionAction(row.Action) || chaintx.IsIDOV3TransactionAction(row.Action) {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	updated, reconcileErr := chaintx.ReconcileReceipt(ctx, db.Mysql(), client, submissionID, client.Confirmations)
	if reconcileErr != nil {
		log.GetLogger().Errorf("reconcile durable chain transaction err: submission_id=%d err=%s", submissionID, reconcileErr)
		return false
	}
	if !eligibleForExactRebroadcast(*updated, time.Now()) {
		return updated.State == constant.ChainTxStateApplied || updated.State == constant.ChainTxStateFinalRevert
	}
	updated, dispatchErr := chaintx.Dispatch(ctx, db.Mysql(), client, submissionID)
	if dispatchErr != nil {
		if errors.Is(dispatchErr, chaintx.ErrSignerBusy) || errors.Is(dispatchErr, chaintx.ErrNonceDrift) {
			return false
		}
		// Dispatch 失败时可能返回 nil（例如 Prepare 被拒），禁止解引用
		if updated != nil {
			log.GetLogger().Errorf("rebroadcast durable chain transaction err: submission_id=%d project_id=%d action=%s tx_hash=%s err=%s", submissionID, updated.ProjectID, updated.Action, chainTxHash(updated), dispatchErr)
		} else {
			log.GetLogger().Errorf("rebroadcast durable chain transaction err: submission_id=%d err=%s", submissionID, dispatchErr)
		}
	}
	return false
}

func eligibleForExactRebroadcast(row entity.ChainTxSubmission, now time.Time) bool {
	if row.State == constant.ChainTxStateReady || row.State == constant.ChainTxStateSigned {
		return true
	}
	if row.State != constant.ChainTxStateSubmitted && row.State != constant.ChainTxStateBroadcastUnknown {
		return false
	}
	return row.LastBroadcastAt == nil || now.Sub(*row.LastBroadcastAt) >= 30*time.Second
}

func chainTxHash(row *entity.ChainTxSubmission) string {
	if row == nil || row.TxHash == nil {
		return ""
	}
	return *row.TxHash
}

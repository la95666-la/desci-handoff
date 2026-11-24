package project

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/core/db"
	chainManager "joincare_server/pkg/core/third/chain"
)

// 全局暂停/恢复（安全审计 HIGH-1 修复）。
//
// 旧行为：Admin 端点直接用 relayer 私钥签名并广播 pause/unpause，绕过 durable
// outbox —— 链上 nonce 前进但 chain_relayer_nonce 栅栏不感知，导致后续所有
// durable Prepare 触发 ErrNonceDrift，全部 IDO V2 上链操作被阻断。
//
// 新行为：与 create/activate/end/cancel/platform_fee 一致，走 durable intent
// （project_id=0 的全局 action），Paused/Unpaused 链事件负责把 submission 推进
// applied 并释放 signer fence；ChainTxRecoveryJob 兜底回收已确认但事件被跳过的
// 全局 submission。

// PauseIDOV2WithGuard 全局暂停 V2 合约（仅平台 owner；挡住认购/激活/充值，不影响退款/领取）。
// 幂等：已暂停时直接返回当前状态，不产生新的 durable intent。
func PauseIDOV2WithGuard(guard OnChainMutationGuard) (paused bool, txHash string, err error) {
	client := chainManagerGetV2()
	if client == nil {
		return false, "", errors.New("IDO V2 relayer client is not configured")
	}
	current, err := client.IsPaused()
	if err != nil {
		return false, "", err
	}
	if current {
		return true, "", nil
	}
	submission, err := dispatchGlobalV2Action(constant.TransactionTypeGlobalPause, client.PackPause, guard)
	if err != nil {
		return false, "", err
	}
	return true, submission.TxHash, nil
}

// UnpauseIDOV2WithGuard 解除全局暂停。幂等：未暂停时直接返回当前状态。
func UnpauseIDOV2WithGuard(guard OnChainMutationGuard) (paused bool, txHash string, err error) {
	client := chainManagerGetV2()
	if client == nil {
		return false, "", errors.New("IDO V2 relayer client is not configured")
	}
	current, err := client.IsPaused()
	if err != nil {
		return false, "", err
	}
	if !current {
		return false, "", nil
	}
	submission, err := dispatchGlobalV2Action(constant.TransactionTypeGlobalUnpause, client.PackUnpause, guard)
	if err != nil {
		return false, "", err
	}
	return false, submission.TxHash, nil
}

func dispatchGlobalV2Action(action string, pack func() ([]byte, error), guard OnChainMutationGuard) (*OnChainSubmission, error) {
	client := chainManager.GetClient(constant.ChainProjectV2)
	if client == nil {
		return nil, errors.New("IDO V2 relayer client is not configured")
	}
	data, err := pack()
	if err != nil {
		return nil, fmt.Errorf("pack %s calldata: %w", action, err)
	}

	var recordID uint64
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := chainreorg.LockNoActiveIncidentTx(tx, client.ChainID.Uint64(), client.ContractAddress.Hex()); err != nil {
			return err
		}
		if guard != nil {
			if err := guard(tx); err != nil {
				return err
			}
		}
		intent, err := chaintx.CreateIntentTx(tx, chaintx.Intent{
			Action: action, ChainID: client.ChainID.Uint64(),
			SenderAddress: client.Address, ContractAddress: client.ContractAddress,
			CallData: data, Global: true,
		})
		if err != nil {
			return err
		}
		recordID = intent.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := db.Mysql().Transaction(func(tx *gorm.DB) error { return chaintx.MarkReadyTx(tx, recordID) }); err != nil {
		return nil, fmt.Errorf("mark %s intent ready: %w", action, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	submission, sendErr := chaintx.Dispatch(ctx, db.Mysql(), client, recordID)
	hash := submissionHash(submission)
	if sendErr != nil {
		return nil, fmt.Errorf("%s durable dispatch incomplete; tx_hash=%s: %w", action, hash, sendErr)
	}
	return &OnChainSubmission{TxHash: hash, Status: action, Submitted: true}, nil
}

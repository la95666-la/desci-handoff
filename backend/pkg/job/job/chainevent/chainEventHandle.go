package chainevent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/robfig/cron/v3"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	commonChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/job/chain"
	"joincare_server/pkg/job/db"
	"joincare_server/pkg/job/model"
	"joincare_server/pkg/job/notice"
)

type GetChainEventJob struct {
	runMu    sync.Mutex
	readHead func(context.Context) (uint64, uint64, error) // nil uses the admitted chain client
	ctx      context.Context                               // nil for legacy direct calls
}

func NewGetChainEventJob() model.Add {
	var job cron.Job = new(GetChainEventJob)

	return job.(model.Add)
}

func (v *GetChainEventJob) Add(crontab *cron.Cron) {
	crontab.AddJob("0 * * * * *", cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(v))
}

func (v *GetChainEventJob) Run() {
	ctx := v.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if v.readHead != nil {
		v.runWithHeadContext(ctx, v.readHead)
		return
	}
	v.runWithHeadContext(ctx, func(ctx context.Context) (uint64, uint64, error) {
		return chain.GetClient().LatestAndSafeBlock(ctx)
	})
}

// The starter binds its stop context before registering this job with cron.
// The immediate and scheduled scanner invocations then share shutdown intent.
func (v *GetChainEventJob) SetRunContext(ctx context.Context) {
	v.ctx = ctx
}

// Keep the chain-head read injectable so a real Redis + MySQL scanner run can
// verify the failed-RPC path without opening an external RPC connection.
func (v *GetChainEventJob) runWithHead(readHead func(context.Context) (uint64, uint64, error)) {
	v.runWithHeadContext(context.Background(), readHead)
}

func (v *GetChainEventJob) runWithHeadContext(ctx context.Context, readHead func(context.Context) (uint64, uint64, error)) {
	if !v.runMu.TryLock() {
		log.GetLogger().Warn("skip overlapping chain event sync run")
		return
	}
	defer v.runMu.Unlock()
	if err := ctx.Err(); err != nil {
		return
	}
	lockToken, err := newRunLockToken()
	if err != nil {
		log.GetLogger().Errorf("create chain event run lock token err: %s", err)
		return
	}
	lockTTL := 2 * time.Minute
	locked, err := db.Redis().TryTokenLock(ctx, constant.KEY_CHAIN_EVENT_JOB_LOCK, lockToken, lockTTL)
	if err != nil {
		log.GetLogger().Errorf("acquire chain event distributed lock err: %s", err)
		return
	}
	if !locked {
		log.GetLogger().Info("skip chain event sync because another job instance holds the lock")
		return
	}
	leaseLost, stopLease := maintainRunLockLease(ctx, lockToken, lockTTL)
	defer func() {
		unlockCtx, cancelUnlock := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelUnlock()
		if err := db.Redis().UnlockTokenLock(unlockCtx, constant.KEY_CHAIN_EVENT_JOB_LOCK, lockToken); err != nil {
			log.GetLogger().Warnf("release chain event distributed lock err: %s", err)
		}
	}()
	defer stopLease()
	if err := ctx.Err(); err != nil {
		return
	}
	syncState, err := claimSyncLease(lockToken)
	if err != nil {
		log.GetLogger().Errorf("claim chain event DB lease err: %s", err)
		return
	}
	if err := activeReorgIncidentError(); err != nil {
		_ = updateSyncFailure(lockToken, syncState.CursorBlock, syncState.LatestBlock, syncState.SafeBlock, err)
		log.GetLogger().Error(err)
		return
	}
	if err := ctx.Err(); err != nil {
		_ = updateSyncFailure(lockToken, syncState.CursorBlock, syncState.LatestBlock, syncState.SafeBlock, err)
		return
	}
	latestBlock, safeBlock, err := readHead(ctx)
	if err != nil {
		_ = updateSyncFailure(lockToken, syncState.CursorBlock, syncState.LatestBlock, syncState.SafeBlock, err)
		log.GetLogger().Errorf("query chain sync head err: %s", err)
		return
	}
	registeredState, registerErr := registerConfiguredV2(ctx, lockToken, syncState, safeBlock)
	if registerErr != nil {
		_ = updateSyncFailure(lockToken, syncState.CursorBlock, latestBlock, safeBlock, registerErr)
		log.GetLogger().Errorf("register V2 chain event source err: %s", registerErr)
		return
	}
	syncState = registeredState
	blockNum := int(syncState.CursorBlock)
	if !syncState.Initialized {
		var source string
		blockNum, source, err = determineInitialCursor(ctx, safeBlock)
		if err != nil {
			_ = updateSyncFailure(lockToken, syncState.CursorBlock, latestBlock, safeBlock, err)
			log.GetLogger().Errorf("determine initial chain event cursor err: %s", err)
			return
		}
		log.GetLogger().Infof("initialize chain event cursor=%d source=%s", blockNum, source)
		initialHash, hashErr := cursorBlockHash(ctx, uint64(blockNum))
		if hashErr != nil {
			_ = updateSyncFailure(lockToken, syncState.CursorBlock, latestBlock, safeBlock, hashErr)
			log.GetLogger().Errorf("query initial chain event cursor hash err: %s", hashErr)
			return
		}
		if err := initializeSyncState(lockToken, uint64(blockNum), latestBlock, safeBlock, initialHash); err != nil {
			_ = updateSyncFailure(lockToken, syncState.CursorBlock, latestBlock, safeBlock, err)
			log.GetLogger().Errorf("initialize chain event DB checkpoint err: %s", err)
			return
		}
		cached, cacheErr := db.Redis().SetIfTokenLock(ctx, constant.KEY_CHAIN_EVENT_JOB_LOCK, lockToken, constant.KEY_BLOCK_NUMBER, blockNum)
		if cacheErr != nil || !cached {
			log.GetLogger().Warnf("cache initialized chain event cursor failed (DB remains authoritative): updated=%t err=%v", cached, cacheErr)
		}
		syncState.Initialized = true
		syncState.CursorBlock = uint64(blockNum)
		syncState.CursorBlockHash = initialHash
	}
	if err := verifyCursorCanonicality(ctx, syncState); err != nil {
		var mismatch *cursorCanonicalityError
		if errors.As(err, &mismatch) {
			if persistErr := persistDetectedReorgIncident(lockToken, syncState, mismatch); persistErr != nil {
				err = fmt.Errorf("%w; persist durable reorg incident: %v", err, persistErr)
			}
		}
		_ = updateSyncFailure(lockToken, uint64(blockNum), latestBlock, safeBlock, err)
		log.GetLogger().Errorf("verify chain cursor canonicality err: %s", err)
		return
	}
	if uint64(blockNum) > safeBlock {
		err := fmt.Errorf("durable cursor=%d is ahead of current safe block=%d", blockNum, safeBlock)
		_ = updateSyncFailure(lockToken, uint64(blockNum), latestBlock, safeBlock, err)
		log.GetLogger().Error(err)
		return
	}
	startCursorHash, err := cursorBlockHash(ctx, uint64(blockNum))
	if err != nil {
		_ = updateSyncFailure(lockToken, uint64(blockNum), latestBlock, safeBlock, err)
		log.GetLogger().Errorf("query initial cursor block hash err: %s", err)
		return
	}
	if err := updateSyncStarted(lockToken, uint64(blockNum), latestBlock, safeBlock, startCursorHash); err != nil {
		_ = updateSyncFailure(lockToken, uint64(blockNum), latestBlock, safeBlock, err)
		log.GetLogger().Errorf("persist chain sync start err: %s", err)
		return
	}
	for {
		if err := currentLeaseError(leaseLost); err != nil {
			_ = updateSyncFailure(lockToken, uint64(blockNum), latestBlock, safeBlock, err)
			log.GetLogger().Error(err)
			return
		}
		logs, newNum, hasMore, err := chain.GetClient().PollEvents(ctx, uint64(blockNum))
		if err != nil {
			_ = updateSyncFailure(lockToken, uint64(blockNum), latestBlock, safeBlock, err)
			log.GetLogger().Errorf("PollEvents err: %s", err)
			return
		}
		if newNum == 0 {
			latestBlock, safeBlock, err = chain.GetClient().LatestAndSafeBlock(ctx)
			if err != nil {
				_ = updateSyncFailure(lockToken, uint64(blockNum), latestBlock, safeBlock, err)
				log.GetLogger().Errorf("refresh caught-up chain head err: %s", err)
				return
			}
			cursorHash, hashErr := cursorBlockHash(ctx, uint64(blockNum))
			if hashErr != nil {
				_ = updateSyncFailure(lockToken, uint64(blockNum), latestBlock, safeBlock, hashErr)
				log.GetLogger().Errorf("refresh caught-up chain sync state err: %s", hashErr)
				return
			}
			if !strings.EqualFold(cursorHash, syncState.CursorBlockHash) {
				mismatch := &cursorCanonicalityError{BlockNumber: uint64(blockNum), Expected: strings.ToLower(syncState.CursorBlockHash), Observed: strings.ToLower(cursorHash)}
				if persistErr := persistDetectedReorgIncident(lockToken, syncState, mismatch); persistErr != nil {
					mismatch = &cursorCanonicalityError{BlockNumber: uint64(blockNum), Expected: mismatch.Expected, Observed: mismatch.Observed}
					log.GetLogger().Errorf("persist caught-up reorg incident err: %s", persistErr)
				}
				_ = updateSyncFailure(lockToken, uint64(blockNum), latestBlock, safeBlock, mismatch)
				log.GetLogger().Error(mismatch)
				return
			}
			refreshed, refreshErr := db.Redis().RefreshTokenLock(ctx, constant.KEY_CHAIN_EVENT_JOB_LOCK, lockToken, lockTTL)
			if refreshErr != nil || !refreshed {
				err := fmt.Errorf("caught-up checkpoint fencing refresh failed: refreshed=%t err=%v", refreshed, refreshErr)
				_ = updateSyncFailure(lockToken, uint64(blockNum), latestBlock, safeBlock, err)
				log.GetLogger().Error(err)
				return
			}
			if err := updateSyncSuccess(lockToken, uint64(blockNum), latestBlock, safeBlock, cursorHash); err != nil {
				log.GetLogger().Errorf("persist caught-up chain sync state err: %s", err)
				return
			}
			cached, cacheErr := db.Redis().SetIfTokenLock(ctx, constant.KEY_CHAIN_EVENT_JOB_LOCK, lockToken, constant.KEY_BLOCK_NUMBER, blockNum)
			if cacheErr != nil || !cached {
				log.GetLogger().Warnf("repair caught-up cursor cache failed (DB remains authoritative): updated=%t err=%v", cached, cacheErr)
			}
			break
		}
		if err := revalidateScannedLogs(ctx, logs, 0, ""); err != nil {
			_ = updateSyncFailure(lockToken, uint64(blockNum), latestBlock, safeBlock, err)
			log.GetLogger().Errorf("preflight scanned chain logs err: %s", err)
			return
		}
		if chain.V2Enabled() {
			runtimeCtx, cancelRuntime := context.WithTimeout(ctx, 8*time.Second)
			runtimeErr := chain.VerifyV2RuntimeAt(runtimeCtx, new(big.Int).SetUint64(uint64(newNum)))
			cancelRuntime()
			if runtimeErr != nil {
				_ = updateSyncFailure(lockToken, uint64(blockNum), latestBlock, safeBlock, runtimeErr)
				log.GetLogger().Errorf("verify V2 runtime at batch boundary=%d: %s", newNum, runtimeErr)
				return
			}
		}
		if err := ensureUnresolvedFailuresAppearInLogs(uint64(blockNum)+1, uint64(newNum), logs); err != nil {
			_ = updateSyncFailure(lockToken, uint64(blockNum), latestBlock, safeBlock, err)
			log.GetLogger().Error(err)
			return
		}
		batchNotifications := make([]struct {
			event     string
			projectID int64
			items     []eventNotification
		}, 0)
		cursorHash := ""
		batchErr := db.Mysql().Transaction(func(batchTx *gorm.DB) error {
			if err := lockChainEventSyncFenceTx(batchTx, lockToken); err != nil {
				return err
			}
			if err := processLogBatch(logs, func(info types.Log) error {
				if err := currentLeaseError(leaseLost); err != nil {
					return err
				}
				hasFailure, err := ensureEventCanRun(info)
				if err != nil {
					return err
				}
				if chain.IsV2Log(info.Address) {
					v2Data, decodeErr := commonChain.DecodeV2Log(info)
					if decodeErr != nil {
						return &eventBatchError{log: info, event: "decode_v2", err: decodeErr}
					}
					if v2Data == nil {
						if len(info.Topics) == 0 {
							return &eventBatchError{log: info, event: "decode_v2", err: errors.New("V2 proxy log has no event topic")}
						}
						if info.Topics[0] == commonChain.V2UpgradedTopic {
							return &eventBatchError{log: info, event: "Upgraded", err: errors.New("V2 proxy implementation upgrade requires an approved runtime and controlled replay")}
						}
						if info.Topics[0] == commonChain.V2PausedTopic || info.Topics[0] == commonChain.V2UnpausedTopic {
							// 全局治理暂停事件（HIGH-1 配套）：不产生账务记录，但要把对应的
							// global_pause/global_unpause durable submission 推进 applied 并释放
							// relayer signer fence；若曾进入失败队列则在此结案。
							if err := v.applyV2GlobalPauseStateTx(batchTx, info); err != nil {
								return &eventBatchError{log: info, event: "global_pause_state", err: err}
							}
							if err := resolveRetriedV2NonMaterialTx(batchTx, info, hasFailure); err != nil {
								return &eventBatchError{log: info, event: "global_pause_state", err: fmt.Errorf("resolve governance event failure: %w", err)}
							}
							return nil
						}
						if commonChain.IsApprovedV2NonMaterialTopic(info.Topics[0]) {
							// 非实质事件（治理/配置类）不产生账务影响；若它曾进入失败/重试队列，
							// 在此结案，避免 replay 后因 hasFailure 永久循环死信。
							if err := resolveRetriedV2NonMaterialTx(batchTx, info, hasFailure); err != nil {
								return &eventBatchError{log: info, event: "unknown_v2_topic", err: fmt.Errorf("resolve non-material event failure: %w", err)}
							}
							return nil
						}
						return &eventBatchError{log: info, event: "unknown_v2_topic", err: fmt.Errorf("unapproved V2 event topic %s", info.Topics[0].Hex())}
					}
					if err := v.processV2LogTx(batchTx, v2Data); err != nil {
						return &eventBatchError{log: info, event: v2Data.Event, err: err}
					}
					return nil
				}
				if !chain.V1Enabled() {
					return &eventBatchError{log: info, event: "unexpected_source", err: errors.New("non-V2 log reached a V2-only event filter")}
				}
				data, decodeErr := decodeChainLog(info)
				if decodeErr != nil {
					return &eventBatchError{log: info, event: "decode", err: decodeErr}
				}
				if data == nil {
					if hasFailure {
						err := errors.New("previously failed chain event is no longer decodable; reconciliation required")
						return &eventBatchError{log: info, event: "decode", err: err}
					}
					return nil
				}
				notifications, err := v.processLogTx(batchTx, data)
				if err != nil {
					return &eventBatchError{log: info, event: data.Event, err: err}
				}
				if len(notifications) > 0 {
					batchNotifications = append(batchNotifications, struct {
						event     string
						projectID int64
						items     []eventNotification
					}{data.Event, data.ProjectID, notifications})
				}
				return nil
			}); err != nil {
				return err
			}
			if err := ensureNoUnresolvedFailuresTx(batchTx, uint64(blockNum)+1, uint64(newNum)); err != nil {
				return err
			}
			var hashErr error
			cursorHash, hashErr = cursorBlockHash(ctx, uint64(newNum))
			if hashErr != nil {
				return hashErr
			}
			if err := revalidateScannedLogs(ctx, logs, uint64(newNum), cursorHash); err != nil {
				return &canonicalBatchError{err: err}
			}
			if err := revalidateCommittedCursorTx(ctx, batchTx, uint64(blockNum)); err != nil {
				return &canonicalBatchError{err: err}
			}
			latestBlock, safeBlock, err = chain.GetClient().LatestAndSafeBlock(ctx)
			if err != nil {
				return err
			}
			refreshed, refreshErr := db.Redis().RefreshTokenLock(ctx, constant.KEY_CHAIN_EVENT_JOB_LOCK, lockToken, lockTTL)
			if refreshErr != nil || !refreshed {
				return fmt.Errorf("checkpoint fencing refresh failed: refreshed=%t err=%v", refreshed, refreshErr)
			}
			return updateSyncSuccessTx(batchTx, lockToken, uint64(newNum), latestBlock, safeBlock, cursorHash)
		})
		if batchErr != nil {
			var eventErr *eventBatchError
			if errors.As(batchErr, &eventErr) {
				if persistErr := recordEventFailure(lockToken, eventErr.log, eventErr.event, eventErr.err); persistErr != nil {
					batchErr = fmt.Errorf("%v; persist failure: %w", batchErr, persistErr)
				}
			} else {
				var canonicalErr *canonicalBatchError
				if errors.As(batchErr, &canonicalErr) {
					var mismatch *cursorCanonicalityError
					if errors.As(canonicalErr, &mismatch) {
						currentState := entity.ChainEventSyncState{}
						if stateErr := db.Mysql().Where("id = 1 AND lease_token = ?", lockToken).Take(&currentState).Error; stateErr != nil {
							batchErr = fmt.Errorf("%v; reload reorg cursor state: %w", batchErr, stateErr)
						} else if persistErr := persistDetectedReorgIncident(lockToken, &currentState, mismatch); persistErr != nil {
							batchErr = fmt.Errorf("%v; persist durable reorg incident: %w", batchErr, persistErr)
						}
					}
					if persistErr := recordReconciliationFailures(lockToken, logs, batchErr); persistErr != nil {
						batchErr = fmt.Errorf("%v; persist reconciliation: %w", batchErr, persistErr)
					}
				}
			}
			_ = updateSyncFailure(lockToken, uint64(blockNum), latestBlock, safeBlock, batchErr)
			log.GetLogger().Errorf("process chain event batch cursor=%d target=%d err: %s", blockNum, newNum, batchErr)
			return
		}
		for _, pending := range batchNotifications {
			v.pushEventNotifications(pending.event, pending.projectID, pending.items)
		}
		cached, cacheErr := db.Redis().SetIfTokenLock(ctx, constant.KEY_CHAIN_EVENT_JOB_LOCK, lockToken, constant.KEY_BLOCK_NUMBER, newNum)
		if cacheErr != nil || !cached {
			log.GetLogger().Warnf("cache fenced chain event cursor failed (DB checkpoint committed): updated=%t err=%v", cached, cacheErr)
		}
		blockNum = newNum
		syncState.CursorBlock = uint64(newNum)
		syncState.CursorBlockHash = cursorHash
		// 已追上最新区块，退出循环
		if !hasMore {
			break
		}
	}
}

func newRunLockToken() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func maintainRunLockLease(ctx context.Context, token string, ttl time.Duration) (<-chan error, func()) {
	leaseCtx, cancel := context.WithCancel(ctx)
	lost := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(ttl / 4)
		defer ticker.Stop()
		for {
			select {
			case <-leaseCtx.Done():
				return
			case <-ticker.C:
				refreshed, err := db.Redis().RefreshTokenLock(leaseCtx, constant.KEY_CHAIN_EVENT_JOB_LOCK, token, ttl)
				if err != nil || !refreshed {
					select {
					case lost <- fmt.Errorf("chain event distributed lock lease lost: refreshed=%t err=%v", refreshed, err):
					default:
					}
					return
				}
			}
		}
	}()
	return lost, cancel
}

func currentLeaseError(lost <-chan error) error {
	select {
	case err := <-lost:
		return err
	default:
		return nil
	}
}

func decodeChainLog(info types.Log) (data *commonChain.Data, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			data = nil
			err = fmt.Errorf("decode chain log panic: %v", recovered)
		}
	}()
	return chain.GetClient().ProcessLog(info), nil
}

func initialCursorForStartBlock(startBlock uint64) int {
	if startBlock == 0 {
		return 0
	}
	return int(startBlock - 1)
}

func processLogBatch(logs []types.Log, handle func(types.Log) error) error {
	for _, info := range logs {
		if info.Removed {
			continue
		}
		if err := handle(info); err != nil {
			return fmt.Errorf("block=%d tx=%s log_index=%d: %w", info.BlockNumber, info.TxHash.Hex(), info.Index, err)
		}
	}
	return nil
}

// A retry is resolved inside the same fenced batch transaction as its cursor.
// Using db.Mysql() here starts a second transaction that can wait on the
// outer transaction's chain_event_sync_state row lock and commit independently.
func resolveRetriedV2NonMaterialTx(batchTx *gorm.DB, info types.Log, hasFailure bool) error {
	if !hasFailure {
		return nil
	}
	return resolveEventFailure(batchTx, info.BlockNumber, info.TxHash.Hex(), info.Index)
}

type eventNotification struct {
	uid              int
	relatedID        int
	notificationType string
}

type eventBatchError struct {
	log   types.Log
	event string
	err   error
}

func (e *eventBatchError) Error() string { return e.err.Error() }
func (e *eventBatchError) Unwrap() error { return e.err }

type canonicalBatchError struct{ err error }

func (e *canonicalBatchError) Error() string { return e.err.Error() }
func (e *canonicalBatchError) Unwrap() error { return e.err }

func (v *GetChainEventJob) ProcessLog(data *commonChain.Data) error {
	var notifications []eventNotification
	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, ""); err != nil {
			return err
		}
		if err := requireNoActiveReorgIncidentTx(tx); err != nil {
			return err
		}
		var err error
		notifications, err = v.processLogTx(tx, data)
		return err
	})
	if err != nil {
		return err
	}
	v.pushEventNotifications(data.Event, data.ProjectID, notifications)
	return nil
}

func lockChainEventSyncFenceTx(tx *gorm.DB, leaseToken string) error {
	state := entity.ChainEventSyncState{}
	query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id,lease_token").Where("id = 1")
	if leaseToken != "" {
		query = query.Where("lease_token = ?", leaseToken)
	}
	if err := query.Take(&state).Error; err != nil {
		return fmt.Errorf("lock chain event sync fence: %w", err)
	}
	return nil
}

func requireNoActiveReorgIncidentTx(tx *gorm.DB) error {
	if !chain.V2Enabled() || chain.GetClient() == nil || chain.GetClient().ChainID == nil || !chain.GetClient().ChainID.IsUint64() {
		return nil
	}
	active, err := hasActiveReorgIncidentTx(tx, chain.GetClient().ChainID.Uint64(), chain.V2ContractAddress().Hex())
	if err != nil {
		return fmt.Errorf("query active chain event reorg incident: %w", err)
	}
	if active {
		return errors.New("active chain event reorg incident blocks standalone event processing")
	}
	return nil
}

func (v *GetChainEventJob) processLogTx(tx *gorm.DB, data *commonChain.Data) ([]eventNotification, error) {
	info := entity.ChainEventRecord{}
	if err := tx.Select("id").Model(&entity.ChainEventRecord{}).
		Where("block_number = ? and tx_hash = ? and log_index = ?", data.BlockNumber, data.TxHash, data.LogIndex).
		Scan(&info).Error; err != nil {
		return nil, fmt.Errorf("query chain event record: %w", err)
	}
	if info.ID != 0 {
		return nil, resolveEventFailure(tx, data.BlockNumber, data.TxHash, data.LogIndex)
	}
	data = v.ProcessData(data)
	realtimeNotifications := make([]eventNotification, 0, 1)
	var err error
	switch data.Event {
	case commonChain.ProjectCreated:
	case commonChain.Purchase:
		var notification *eventNotification
		notification, err = v.purchase(data, tx)
		if notification != nil {
			realtimeNotifications = append(realtimeNotifications, *notification)
		}
	case commonChain.Claimed:
		var notification *eventNotification
		notification, err = v.tokensClaimed(data, tx)
		if notification != nil {
			realtimeNotifications = append(realtimeNotifications, *notification)
		}
	case commonChain.Refunded:
		var notification *eventNotification
		notification, err = v.tokensRefund(data, tx)
		if notification != nil {
			realtimeNotifications = append(realtimeNotifications, *notification)
		}
	case commonChain.ProjectTokensDeposited:
		err = v.projectTokensDeposited(data, tx)
	case commonChain.ProjectActivated:
	case commonChain.ProjectEnded:
		var notifications []eventNotification
		notifications, err = v.projectEnded(data, tx)
		realtimeNotifications = append(realtimeNotifications, notifications...)
	case commonChain.RaisedFundsWithdrawn:
		err = v.raisedFundsWithdrawn(data, tx)
	case commonChain.ProjectTokensWithdrawn:
		err = v.projectTokensWithdrawn(data, tx)
	case commonChain.PlatformFeesUpdated:
	case commonChain.ProjectCancelled:
	default:
		return nil, fmt.Errorf("unsupported decoded event %q", data.Event)
	}
	if err != nil {
		return nil, fmt.Errorf("handle %s: %w", data.Event, err)
	}
	content, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal %s event: %w", data.Event, err)
	}
	insert := entity.ChainEventRecord{
		Event: data.Event, BlockNumber: int(data.BlockNumber), BlockHash: data.BlockHash, LogIndex: data.LogIndex,
		ContractAddress: chain.GetClient().ContractAddress.Hex(), ContractVersion: constant.ContractVersionV1Legacy,
		TxHash: data.TxHash, Data: string(content), CreatedAt: time.Now(),
	}
	if err := tx.Create(&insert).Error; err != nil {
		return nil, fmt.Errorf("create chain_event_record: %w", err)
	}
	if data.Event == commonChain.ProjectEnded {
		submission, err := chaintx.FindEventSubmissionTx(tx, uint64(data.ProjectID), constant.TransactionTypeProjectEnd, data.TxHash)
		if err != nil {
			return nil, fmt.Errorf("find project end durable submission: %w", err)
		}
		if submission != nil {
			if err := chaintx.MarkAppliedTx(tx, submission.ID, insert.ID); err != nil {
				return nil, fmt.Errorf("finalize project end durable submission: %w", err)
			}
		} else if err := chaintx.CancelUnsignedBusinessTx(tx, uint64(data.ProjectID), constant.TransactionTypeProjectEnd); err != nil {
			return nil, fmt.Errorf("cancel superseded unsigned project end intent: %w", err)
		}
	}
	if err := resolveEventFailure(tx, data.BlockNumber, data.TxHash, data.LogIndex); err != nil {
		return nil, err
	}
	return realtimeNotifications, nil
}

func (v *GetChainEventJob) pushEventNotifications(event string, projectID int64, realtimeNotifications []eventNotification) {
	for _, notification := range realtimeNotifications {
		if err := notice.Push(notification.uid, notification.relatedID, notification.notificationType); err != nil {
			log.GetLogger().Warnf(
				"chain event realtime push failed: event=%s project_id=%d uid=%d related_id=%d err=%s",
				event, projectID, notification.uid, notification.relatedID, err,
			)
		}
	}
}

func (v *GetChainEventJob) purchase(data *commonChain.Data, tx *gorm.DB) (*eventNotification, error) {
	uid, err := getUIDByAddressTx(tx, data.User.String())
	if err != nil {
		return nil, err
	}
	// 获取项目的提取时间
	info := entity.ProjectTge{}
	err = tx.Model(&entity.ProjectTge{}).
		Select("project_id,`index`,tge_percent,vesting_percent,vesting_interval,vesting_time,vesting_duration,created_at").
		Where("project_id = ? and `index` = 1", data.ProjectID).
		Scan(&info).Error
	if err != nil {
		return nil, err
	}
	err = tx.Model(&entity.Project{}).
		Where("project_id = ?", data.ProjectID).Updates(map[string]interface{}{
		"total_raised": gorm.Expr("total_raised + ?", data.JccAmount),
	}).Error
	if err != nil {
		return nil, err
	}
	now := time.Now()

	insert := entity.UserProject{
		UID:           uid,
		ProjectID:     int(data.ProjectID),
		PurchaseToken: data.JccAmount,
		NextClaimTime: info.VestingTime,
		ClaimTime:     now,
		ClaimToken:    decimal.Zero,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	err = tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "uid"}, {Name: "project_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"purchase_token": gorm.Expr("purchase_token + ?", data.JccAmount),
		}),
	}).Create(&insert).Error
	if err != nil {
		return nil, err
	}
	userProject := entity.UserProject{}
	if err := tx.Select("id").Where("uid = ? AND project_id = ?", uid, data.ProjectID).Take(&userProject).Error; err != nil {
		return nil, err
	}
	projectInfo, err := getProjectInfoTx(tx, int(data.ProjectID))
	if err != nil {
		return nil, fmt.Errorf("project not found for purchase notification: %w", err)
	}
	if err := notice.CreateWithDB(tx, uid, int(userProject.ID), constant.PurchaseSuccessful, map[string][]string{
		constant.NoticeLangCn: {projectInfo.NameCn, data.JccAmount.String(), data.TokensAmount.String(), projectInfo.TokenSymbol},
		constant.NoticeLangTw: {projectInfo.NameCn, data.JccAmount.String(), data.TokensAmount.String(), projectInfo.TokenSymbol},
		constant.NoticeLangEn: {projectInfo.NameEn, data.JccAmount.String(), data.TokensAmount.String(), projectInfo.TokenSymbol},
	}); err != nil {
		return nil, err
	}
	return &eventNotification{uid: uid, relatedID: int(userProject.ID), notificationType: constant.PurchaseSuccessful}, nil
}

func (v *GetChainEventJob) tokensClaimed(data *commonChain.Data, tx *gorm.DB) (*eventNotification, error) {
	uid, err := getUIDByAddressTx(tx, data.User.String())
	if err != nil {
		return nil, err
	}
	// 获取当前用户的项目状态
	projectInfo := entity.UserProject{}
	err = tx.Select("uid,project_id,purchase_token,next_claim_time,claim_time,claim_token,created_at").
		Model(&entity.UserProject{}).Where("project_id = ? and uid = ?", data.ProjectID, uid).
		Scan(&projectInfo).Error
	if err != nil {
		return nil, err
	}
	now := time.Now()

	// 获取项目的提取时间
	info := entity.ProjectTge{}
	err = tx.Model(&entity.ProjectTge{}).
		Select("project_id,`index`,tge_percent,vesting_percent,vesting_interval,vesting_time,vesting_duration,created_at").
		Where("project_id = ? and vesting_time > ?", data.ProjectID, now).
		Order("`index` asc").
		First(&info).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		info.VestingTime = now
	}

	err = tx.Model(&entity.UserProject{}).
		Where("project_id = ? and uid = ?", data.ProjectID, uid).Updates(map[string]interface{}{
		"next_claim_time": info.VestingTime,
		"claim_time":      now,
		"claim_token":     gorm.Expr("claim_token + ?", data.Amount),
	}).Error
	if err != nil {
		return nil, err
	}
	insert := entity.UserClaimRecord{
		UID:         uid,
		ProjectID:   int(data.ProjectID),
		ClaimToken:  data.Amount,
		BlockNumber: int(data.BlockNumber),
		TxHash:      data.TxHash,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	err = tx.Create(&insert).Error
	if err != nil {
		return nil, err
	}
	pInfo, err := getProjectInfoTx(tx, int(data.ProjectID))
	if err != nil {
		return nil, fmt.Errorf("project not found for claim notification: %w", err)
	}
	if err := notice.CreateWithDB(tx, uid, int(insert.ID), constant.ClaimSuccessful, map[string][]string{
		constant.NoticeLangCn: {data.Amount.String(), pInfo.TokenSymbol, pInfo.NameCn},
		constant.NoticeLangTw: {data.Amount.String(), pInfo.TokenSymbol, pInfo.NameCn},
		constant.NoticeLangEn: {data.Amount.String(), pInfo.TokenSymbol, pInfo.NameEn},
	}); err != nil {
		return nil, err
	}
	return &eventNotification{uid: uid, relatedID: int(insert.ID), notificationType: constant.ClaimSuccessful}, nil
}

func getUIDByAddressTx(tx *gorm.DB, address string) (int, error) {
	normalized := strings.ToLower(strings.TrimSpace(address))
	if !common.IsHexAddress(normalized) {
		return 0, errors.New("invalid event wallet address")
	}
	var identity entity.UserWalletIdentity
	if err := tx.Select("uid,chain").Where("wallet_address = ? AND chain = 'evm'", normalized).Take(&identity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, errors.New("can not found verified wallet owner")
		}
		return 0, err
	}
	if identity.UID <= 0 {
		return 0, errors.New("verified wallet owner is invalid")
	}
	return identity.UID, nil
}

func resolveUniqueWalletUID(primaryUIDs, secondaryUIDs []uint64, address string) (int, error) {
	unique := make(map[uint64]struct{}, len(primaryUIDs)+len(secondaryUIDs))
	for _, uid := range append(append([]uint64{}, primaryUIDs...), secondaryUIDs...) {
		if uid != 0 {
			unique[uid] = struct{}{}
		}
	}
	if len(unique) == 0 {
		return 0, errors.New("can not found user id")
	}
	if len(unique) != 1 {
		return 0, fmt.Errorf("wallet address belongs to multiple users: %s", address)
	}
	for uid := range unique {
		return int(uid), nil
	}
	return 0, errors.New("can not found user id")
}

func (v *GetChainEventJob) tokensRefund(data *commonChain.Data, tx *gorm.DB) (*eventNotification, error) {
	uid, err := getUIDByAddressTx(tx, data.User.String())
	if err != nil {
		return nil, err
	}
	now := time.Now()
	insert := entity.UserRefundRecord{
		UID:         uid,
		ProjectID:   int(data.ProjectID),
		RefundToken: data.JccAmount,
		BlockNumber: int(data.BlockNumber),
		TxHash:      data.TxHash,
		CreatedAt:   now,
	}
	err = tx.Create(&insert).Error
	if err != nil {
		return nil, err
	}
	pInfo, err := getProjectInfoTx(tx, int(data.ProjectID))
	if err != nil {
		return nil, fmt.Errorf("project not found for refund notification: %w", err)
	}
	if err := notice.CreateWithDB(tx, uid, int(insert.ID), constant.RefundSuccessful, map[string][]string{
		constant.NoticeLangCn: {pInfo.NameCn, data.JccAmount.String()},
		constant.NoticeLangTw: {pInfo.NameCn, data.JccAmount.String()},
		constant.NoticeLangEn: {pInfo.NameEn, data.JccAmount.String()},
	}); err != nil {
		return nil, err
	}
	return &eventNotification{uid: uid, relatedID: int(insert.ID), notificationType: constant.RefundSuccessful}, nil
}

func (v *GetChainEventJob) projectTokensDeposited(data *commonChain.Data, tx *gorm.DB) error {
	err := tx.Model(&entity.Project{}).
		Where("project_id = ?", data.ProjectID).Updates(map[string]interface{}{
		"total_deposited": gorm.Expr("total_deposited + ?", data.Amount),
	}).Error
	if err != nil {
		return err
	}

	return nil
}

func getProjectInfoTx(tx *gorm.DB, projectID int) (*entity.Project, error) {
	info := entity.Project{}
	err := tx.Model(&entity.Project{}).
		Select("project_id,owner_address,name_en,name_cn,token_address,token_symbol,token_decimals,payment_token,token_price,hard_cap,soft_cap,max_per_user,start_time,end_time,tge_time,total_raised,total_deposited,participant_count,status").
		Where("project_id = ?", projectID).
		Take(&info).Error
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (v *GetChainEventJob) ProcessData(data *commonChain.Data) *commonChain.Data {
	data.TokenPrice = v.ProcessChainToken(data.TokenPrice)
	data.HardCap = v.ProcessChainToken(data.HardCap)
	data.SoftCap = v.ProcessChainToken(data.SoftCap)
	data.JccAmount = v.ProcessChainToken(data.JccAmount)
	data.TokensAmount = v.ProcessChainToken(data.TokensAmount)
	data.Amount = v.ProcessChainToken(data.Amount)
	data.TotalRaised = v.ProcessChainToken(data.TotalRaised)
	data.PlatformFee = v.ProcessChainToken(data.PlatformFee)
	data.FeeRate = v.ProcessChainToken(data.FeeRate)
	return data
}
func (v *GetChainEventJob) ProcessChainToken(a decimal.Decimal) decimal.Decimal {
	if !a.IsZero() {
		return a.Shift(-18)
	}
	return a
}

func (v *GetChainEventJob) raisedFundsWithdrawn(data *commonChain.Data, tx *gorm.DB) error {
	err := tx.Model(&entity.Project{}).
		Where("project_id = ?", data.ProjectID).Updates(map[string]interface{}{
		"fund_withdrawn": 1,
	}).Error
	if err != nil {
		return err
	}
	return nil
}
func (v *GetChainEventJob) projectTokensWithdrawn(data *commonChain.Data, tx *gorm.DB) error {
	err := tx.Model(&entity.Project{}).
		Where("project_id = ?", data.ProjectID).Updates(map[string]interface{}{
		"token_withdrawn": 1,
	}).Error
	if err != nil {
		return err
	}
	return nil
}
func (v *GetChainEventJob) projectEnded(data *commonChain.Data, tx *gorm.DB) ([]eventNotification, error) {
	updates := make(map[string]interface{}, 2)
	if data.NewStatus == constant.ProjectStatusFailed {
		updates["token_withdrawn"] = 1
	}
	updates["status"] = data.NewStatus
	err := tx.Model(&entity.Project{}).
		Where("project_id = ?", data.ProjectID).Updates(updates).Error
	if err != nil {
		return nil, err
	}
	notifications := make([]eventNotification, 0)
	if data.NewStatus == constant.ProjectStatusFailed {
		userProjects := make([]entity.UserProject, 0, 20)
		err := tx.Select("id,uid,project_id,purchase_token,next_claim_time,claim_time,claim_token,created_at").
			Model(&entity.UserProject{}).Where("project_id = ?", data.ProjectID).Find(&userProjects).Error
		if err != nil {
			return nil, err
		}
		projectInfo, err := getProjectInfoTx(tx, int(data.ProjectID))
		if err != nil {
			return nil, fmt.Errorf("project not found for funding failure notification: %w", err)
		}
		for _, info := range userProjects {
			if err := notice.CreateWithDB(tx, info.UID, int(info.ID), constant.ProjectFundingFailed, map[string][]string{
				constant.NoticeLangCn: {projectInfo.NameCn, info.PurchaseToken.String()},
				constant.NoticeLangTw: {projectInfo.NameCn, info.PurchaseToken.String()},
				constant.NoticeLangEn: {projectInfo.NameEn, info.PurchaseToken.String()},
			}); err != nil {
				return nil, err
			}
			notifications = append(notifications, eventNotification{uid: info.UID, relatedID: int(info.ID), notificationType: constant.ProjectFundingFailed})
		}
	}
	return notifications, nil
}

package chainevent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math/big"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	baseChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/common/projectruntime"
	"joincare_server/pkg/job/chain"
	"joincare_server/pkg/job/db"
	"joincare_server/pkg/job/model"
)

type ProjectEndJob struct{}

func NewProjectEndJob() model.Add {

	var job cron.Job = new(ProjectEndJob)

	return job.(model.Add)
}

func (v *ProjectEndJob) Add(crontab *cron.Cron) {
	crontab.AddJob("0 * * * * *", cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(v))
	//crontab.AddJob("10 0 8 * * *", cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(v))
	//crontab.AddJob("30 0 8 * * *", cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(v))
	//crontab.AddJob("0 1 8 * * *", cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(v))
}

func (v *ProjectEndJob) Run() {
	ctx, cancelJob := context.WithCancel(context.Background())
	defer cancelJob()
	lockToken, err := newProjectEndLockToken()
	if err != nil {
		log.GetLogger().Errorf("create project end lock token err: %s", err)
		return
	}
	locked, err := db.Redis().TryTokenLock(ctx, constant.KEY_PROJECT_END_JOB_LOCK, lockToken, 2*time.Minute)
	if err != nil {
		log.GetLogger().Errorf("acquire project end job lock err: %s", err)
		return
	}
	if !locked {
		return
	}
	leaseLost := make(chan struct{})
	stopRenew := make(chan struct{})
	go renewProjectEndLease(ctx, lockToken, stopRenew, leaseLost, cancelJob)
	defer func() {
		close(stopRenew)
		unlockCtx, cancelUnlock := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelUnlock()
		if err := db.Redis().UnlockTokenLock(unlockCtx, constant.KEY_PROJECT_END_JOB_LOCK, lockToken); err != nil {
			log.GetLogger().Warnf("release project end job lock err: %s", err)
		}
	}()

	client := chain.GetClient()
	if client == nil {
		log.GetLogger().Error("project end job chain client is not configured")
		return
	}
	// For V2 the contract compares block.timestamp against endTime. A local
	// Hardhat chain may advance ahead of the host clock; conversely a fast host
	// clock must not dispatch a transaction before the confirmed chain is due.
	now := time.Now()
	if chain.V2Enabled() && !chain.V1Enabled() {
		clockCtx, cancelClock := context.WithTimeout(ctx, 8*time.Second)
		chainTime, clockErr := confirmedV2EndTime(clockCtx, client)
		cancelClock()
		if clockErr != nil {
			log.GetLogger().Errorf("query confirmed V2 project end clock: %s", clockErr)
			return
		}
		now = chainTime
	}
	infos := make([]entity.Project, 0, 8)
	err = db.Mysql().Model(&entity.Project{}).
		Select("project_id").
		Where("end_time <= ? AND status = ? AND contract_version IN ?", now, constant.ProjectStatusActive, eligibleEndVersions()).
		Find(&infos).Error
	if err != nil {
		log.GetLogger().Errorf("query projects due to end err: %s", err)
		return
	}
	for _, info := range infos {
		select {
		case <-leaseLost:
			log.GetLogger().Error("project end job lease lost; stop lifecycle submissions")
			return
		default:
		}
		refreshed, refreshErr := db.Redis().RefreshTokenLock(ctx, constant.KEY_PROJECT_END_JOB_LOCK, lockToken, 2*time.Minute)
		if refreshErr != nil || !refreshed {
			log.GetLogger().Errorf("project end job lost lock: refreshed=%t err=%v", refreshed, refreshErr)
			return
		}
		operationCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		refreshed, refreshErr = db.Redis().RefreshTokenLock(operationCtx, constant.KEY_PROJECT_END_JOB_LOCK, lockToken, 2*time.Minute)
		if refreshErr != nil || !refreshed {
			cancel()
			log.GetLogger().Errorf("project end job lost lock before broadcast: refreshed=%t err=%v", refreshed, refreshErr)
			return
		}

		submission, reserveErr := reserveDueProjectEnd(info.ProjectID, client, now)
		if reserveErr != nil {
			cancel()
			log.GetLogger().Errorf("reserve project end submission err: project_id=%d err=%s", info.ProjectID, reserveErr)
			continue
		}
		submission, reconcileErr := chaintx.ReconcileReceipt(operationCtx, db.Mysql(), client, submission.ID, client.Confirmations)
		if reconcileErr != nil {
			cancel()
			log.GetLogger().Errorf("reconcile project end submission err: project_id=%d submission_id=%d err=%s", info.ProjectID, submission.ID, reconcileErr)
			continue
		}
		if submission.State == constant.ChainTxStateFinalSuccess || submission.State == constant.ChainTxStateApplied {
			cancel()
			continue
		}
		if submission.State == constant.ChainTxStateAmbiguous {
			cancel()
			log.GetLogger().Errorf("project end submission is ambiguous and requires manual reconciliation: project_id=%d submission_id=%d tx_hash=%s", info.ProjectID, submission.ID, submissionHashValue(submission))
			continue
		}
		if submission.State == constant.ChainTxStateMinedSuccess || submission.State == constant.ChainTxStateMinedRevert {
			cancel()
			continue
		}
		if submission.State == constant.ChainTxStateFinalRevert {
			submission, reserveErr = reserveDueProjectEnd(info.ProjectID, client, now)
			if reserveErr != nil {
				cancel()
				log.GetLogger().Errorf("reserve project end retry err: project_id=%d err=%s", info.ProjectID, reserveErr)
				continue
			}
		}
		submission, endErr := chaintx.Dispatch(operationCtx, db.Mysql(), client, submission.ID)
		cancel()
		txHash := submissionHashValue(submission)
		if endErr != nil {
			log.GetLogger().Errorf("project end durable dispatch incomplete: project_id=%d submission_id=%d tx_hash=%s err=%s", info.ProjectID, submission.ID, txHash, endErr)
			continue
		}
		log.GetLogger().Infof("submitted project end tx: project_id=%d tx_hash=%s", info.ProjectID, txHash)
	}
	// Do not perform runtime lookup, project query, or reservation while the
	// authoritative Fair Launch materializer is disabled.
	if !fairLaunchEndSchedulingEnabled() {
		return
	}
	// Fair Launch uses the same scheduler lease but only reserves an unsigned
	// Operator intent. Remote Gateway dispatch remains a separate recovery loop.
	runtime, runtimeErr := projectruntime.FairLaunchV1()
	if runtimeErr != nil {
		log.GetLogger().Errorf("load Fair Launch runtime: %s", runtimeErr)
		return
	}
	if !runtime.Enabled {
		return
	}
	fairProjects := make([]entity.Project, 0, 8)
	if err := db.Mysql().Where("end_time <= ? AND status = ? AND contract_version = ? AND funding_mode = ?", time.Now().UTC(), constant.ProjectStatusActive, constant.ContractVersionFairLaunchV1, constant.FundingModeFairLaunch).Find(&fairProjects).Error; err != nil {
		log.GetLogger().Errorf("query Fair Launch projects due to end: %s", err)
		return
	}
	for _, project := range fairProjects {
		if _, err := func() (*entity.ChainTxSubmission, error) {
			var submission *entity.ChainTxSubmission
			err := db.Mysql().Transaction(func(tx *gorm.DB) error {
				var reserveErr error
				submission, reserveErr = reserveFairLaunchEndTx(tx, project.ProjectID, runtime, time.Now().UTC())
				return reserveErr
			})
			return submission, err
		}(); err != nil {
			log.GetLogger().Errorf("reserve Fair Launch end: project_id=%d err=%s", project.ProjectID, err)
		}
	}
}

func fairLaunchEndSchedulingEnabled() bool {
	return constant.FairLaunchMaterializationEnabled
}

func eligibleEndVersions() []string {
	if chain.V2Enabled() && !chain.V1Enabled() {
		return []string{constant.ContractVersionV2Stablecoin}
	}
	// The mixed V1/V2 job retains legacy auto-end only. V2 auto-end
	// requires the V2-only signer client and its independent nonce lane.
	return []string{constant.ContractVersionV1Legacy}
}

func confirmedV2EndTime(ctx context.Context, client *baseChain.Client) (time.Time, error) {
	if client == nil || client.Client == nil {
		return time.Time{}, errors.New("V2 end chain client is unavailable")
	}
	_, safeBlock, err := client.LatestAndSafeBlock(ctx)
	if err != nil {
		return time.Time{}, err
	}
	// Block zero is not an admitted lifecycle source. With insufficient
	// confirmations, wait instead of borrowing the unconfirmed head clock.
	if safeBlock == 0 {
		return time.Time{}, errors.New("V2 end has no confirmed block")
	}
	header, err := client.Client.HeaderByNumber(ctx, new(big.Int).SetUint64(safeBlock))
	if err != nil {
		return time.Time{}, err
	}
	if header == nil || header.Time > uint64(1<<63-1) {
		return time.Time{}, errors.New("V2 end confirmed block timestamp is invalid")
	}
	return time.Unix(int64(header.Time), 0).UTC(), nil
}

func reserveDueProjectEnd(projectID int, client *baseChain.Client, now time.Time) (*entity.ChainTxSubmission, error) {
	if chain.V2Enabled() && !chain.V1Enabled() {
		return reserveIDOV2End(projectID, client, now)
	}
	return reserveProjectEnd(projectID, client)
}

func reserveProjectEnd(projectID int, client *baseChain.Client) (*entity.ChainTxSubmission, error) {
	data, err := client.PackEndProject(big.NewInt(int64(projectID)))
	if err != nil {
		return nil, err
	}
	var submission *entity.ChainTxSubmission
	err = db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := chainreorg.LockNoActiveIncidentTx(tx, client.ChainID.Uint64(), client.ContractAddress.Hex()); err != nil {
			return err
		}
		project := entity.Project{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ?", projectID).Take(&project).Error; err != nil {
			return err
		}
		if project.Status != constant.ProjectStatusActive || project.ContractVersion != constant.ContractVersionV1Legacy || project.EndTime.After(time.Now()) {
			return errors.New("project is no longer eligible for legacy end submission")
		}
		var createErr error
		submission, createErr = chaintx.CreateIntentTx(tx, chaintx.Intent{ProjectID: uint64(projectID), Action: constant.TransactionTypeProjectEnd, ChainID: client.ChainID.Uint64(), SenderAddress: client.Address, ContractAddress: client.ContractAddress, CallData: data})
		if createErr != nil {
			return createErr
		}
		return chaintx.MarkReadyTx(tx, submission.ID)
	})
	return submission, err
}

func submissionHashValue(submission *entity.ChainTxSubmission) string {
	if submission == nil || submission.TxHash == nil {
		return ""
	}
	return *submission.TxHash
}

func newProjectEndLockToken() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func renewProjectEndLease(ctx context.Context, token string, stop <-chan struct{}, lost chan<- struct{}, cancel context.CancelFunc) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			refreshed, err := db.Redis().RefreshTokenLock(ctx, constant.KEY_PROJECT_END_JOB_LOCK, token, 2*time.Minute)
			if err != nil || !refreshed {
				log.GetLogger().Errorf("project end job lease renewal failed: refreshed=%t err=%v", refreshed, err)
				close(lost)
				cancel()
				return
			}
		}
	}
}

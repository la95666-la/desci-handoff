package idoreconciliation

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	baseChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	commonReconciliation "joincare_server/pkg/common/idoreconciliation"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/job/chain"
	"joincare_server/pkg/job/db"
	"joincare_server/pkg/job/model"
)

const (
	pageSize         = 25
	leaseDuration    = 90 * time.Second
	pageTimeout      = 60 * time.Second
	maxAttempts      = 5
	activeLeaseWhere = "run_id = ? AND lease_token = ? AND state IN ? AND lease_expires_at > CURRENT_TIMESTAMP(6)"
)

var errReconciliationCancelled = errors.New("reconciliation was cancelled")
var errFinalSnapshotInvalidated = errors.New("final reconciliation snapshot was invalidated")

type Worker struct{}

func NewWorker() model.Add {
	var job cron.Job = new(Worker)
	return job.(model.Add)
}

func (v *Worker) Add(crontab *cron.Cron) {
	crontab.AddJob("30 * * * * *", cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(v))
}

func (v *Worker) Run() {
	reader := chain.GetV2SnapshotClient()
	if reader == nil {
		log.GetLogger().Error("IDO reconciliation V2 snapshot client is not configured")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), pageTimeout)
	defer cancel()
	if err := reapOneExpired(ctx); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.GetLogger().Errorf("reap expired IDO reconciliation lease err: %s", err)
		return
	}
	run, job, token, err := claimOne(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	if err != nil {
		log.GetLogger().Errorf("claim IDO reconciliation job err: %s", err)
		return
	}
	if err := processClaimed(ctx, reader, run, job, token); err != nil {
		log.GetLogger().Errorf("process IDO reconciliation err: run_id=%d err=%s", run.ID, err)
	}
}

func claimOne(ctx context.Context) (entity.IDOReconciliationRun, entity.IDOReconciliationJob, string, error) {
	run := entity.IDOReconciliationRun{}
	job := entity.IDOReconciliationJob{}
	token, err := randomToken()
	if err != nil {
		return run, job, "", err
	}
	now := time.Now()
	err = db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&entity.IDOReconciliationJob{}).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Joins("JOIN ido_reconciliation_run r ON r.id = ido_reconciliation_job.run_id").
			Joins("JOIN projects p ON p.project_id = r.project_id").
			Where("ido_reconciliation_job.state IN ? AND ido_reconciliation_job.next_attempt_at <= ? AND (ido_reconciliation_job.lease_expires_at IS NULL OR ido_reconciliation_job.lease_expires_at < ?) AND p.funding_mode = ? AND p.contract_version = ?", []string{"queued", "retrying"}, now, now, constant.FundingModeCappedPresale, constant.ContractVersionV2Stablecoin).
			Order("ido_reconciliation_job.run_id ASC").Take(&job).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", job.RunID).Take(&run).Error; err != nil {
			return err
		}
		startedAt := job.StartedAt
		if startedAt == nil {
			startedAt = &now
		}
		result := tx.Model(&entity.IDOReconciliationJob{}).Where("run_id = ? AND state = ?", job.RunID, job.State).Updates(map[string]interface{}{
			"state": "running", "attempt_count": job.AttemptCount + 1, "lease_token": token, "lease_expires_at": gorm.Expr("TIMESTAMPADD(SECOND, ?, CURRENT_TIMESTAMP(6))", int(leaseDuration/time.Second)),
			"started_at": startedAt, "last_error_code": "", "last_error_message": "", "updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		job.State, job.AttemptCount, job.LeaseToken = "running", job.AttemptCount+1, token
		return appendEventTx(tx, run.ID, "STARTED", "collect", "", "", nil, token)
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	return run, job, token, err
}

func processClaimed(ctx context.Context, reader commonReconciliation.Reader, run entity.IDOReconciliationRun, job entity.IDOReconciliationJob, token string) error {
	source := commonReconciliation.SourceEnvelope{}
	sourceHash := sha256.Sum256(run.SourcePayload)
	if hex.EncodeToString(sourceHash[:]) != run.SourceDigest || commonReconciliation.DecodeCanonicalJSON(run.SourcePayload, &source) != nil {
		return finishFailure(run.ID, token, "SOURCE_CHANGED", "source", "frozen source payload digest or encoding is invalid", "invalidated")
	}
	if source.Request.Project.ProjectID != run.ProjectID || source.Request.BlockNumber != run.EvidenceBlockNumber || !strings.EqualFold(source.Request.BlockHash, run.EvidenceBlockHash) || len(source.Request.Wallets) != int(run.ExpectedWalletCount) {
		return finishFailure(run.ID, token, "SOURCE_CHANGED", "source", "frozen source identity does not match run", "invalidated")
	}
	if source.Scope != "database_ledger_plus_onchain_snapshot" || run.AlgorithmVersion != commonReconciliation.AlgorithmVersion || run.CanonicalizationVersion != commonReconciliation.CanonicalizationVersion || !commonReconciliation.ValidDigest(source.FundLedgerFacts.RootDigest) || !commonReconciliation.ValidDigest(source.PurchaseEntitlements.RootDigest) {
		return finishFailure(run.ID, token, "SOURCE_CHANGED", "source", "frozen evidence envelope identity is invalid", "invalidated")
	}
	expectedWalletIdentityDigest, expectedCheckIdentityDigest, expectedCheckCount := commonReconciliation.ExpectedEvidenceIdentity(source.DatabaseChecks, source.Request)
	if source.ExpectedWalletIdentityDigest != expectedWalletIdentityDigest || source.ExpectedCheckIdentityDigest != expectedCheckIdentityDigest || source.ExpectedCheckCount != expectedCheckCount || run.ExpectedWalletIdentityDigest != expectedWalletIdentityDigest || run.ExpectedCheckIdentityDigest != expectedCheckIdentityDigest || run.ExpectedCheckCount != expectedCheckCount {
		return finishFailure(run.ID, token, "SOURCE_CHANGED", "source", "frozen evidence identity set does not match run", "invalidated")
	}
	if err := validateOperationalDB(ctx, run, source); err != nil {
		return finishFailure(run.ID, token, "SNAPSHOT_INVALIDATED", "database", err.Error(), "invalidated")
	}
	current, err := loadFencedJob(ctx, run.ID, token)
	if err != nil {
		return err
	}
	if current.State == "cancel_requested" {
		return finishCancelled(run.ID, token)
	}
	start := int(current.NextWalletOrdinal)
	end := start + pageSize
	if end > len(source.Request.Wallets) {
		end = len(source.Request.Wallets)
	}
	base := commonReconciliation.BaseSnapshot{}
	if start == 0 || end == len(source.Request.Wallets) {
		base, err = commonReconciliation.CollectBase(ctx, reader, source.Request)
		if err != nil {
			return handleExecutionError(run.ID, token, current.ConsecutiveErrorCount+1, err)
		}
	}
	if start < end {
		wallets, _, err := commonReconciliation.ObserveWalletRange(ctx, reader, source.Request, start, end)
		if err != nil {
			return handleExecutionError(run.ID, token, current.ConsecutiveErrorCount+1, err)
		}
		if err := persistWalletPage(ctx, run, token, source.Request.Wallets[start:end], wallets, uint(start), uint(end)); err != nil {
			return err
		}
		if end < len(source.Request.Wallets) {
			return releaseForNextPage(run.ID, token, uint(end))
		}
	}
	return finalize(ctx, reader, run, token, source, base, current.ConsecutiveErrorCount)
}

func persistWalletPage(ctx context.Context, run entity.IDOReconciliationRun, token string, expected []commonReconciliation.ExpectedWallet, actual []commonReconciliation.WalletSnapshot, start, end uint) error {
	if len(expected) != len(actual) || uint(len(actual)) != end-start {
		return errors.New("wallet page result count mismatch")
	}
	return db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		job, err := lockFencedJob(tx, run.ID, token)
		if err != nil {
			return err
		}
		if job.State == "cancel_requested" {
			return finishCancelledTx(tx, run.ID, token)
		}
		if job.State != "running" || job.NextWalletOrdinal != start {
			return errors.New("wallet page fencing rejected stale progress")
		}
		if err := renewLeaseTx(tx, run.ID, token, "running"); err != nil {
			return err
		}
		previous := commonReconciliation.ZeroDigest()
		previousIdentity := commonReconciliation.ZeroDigest()
		if start > 0 {
			prior := entity.IDOReconciliationWallet{}
			if err := tx.Select("ordinal,row_digest,identity_digest").Where("run_id = ? AND ordinal = ?", run.ID, start-1).Take(&prior).Error; err != nil {
				return err
			}
			if prior.Ordinal != start-1 {
				return errors.New("wallet evidence chain is not contiguous")
			}
			previous = prior.RowDigest
			previousIdentity = prior.IdentityDigest
		}
		for i := range actual {
			ordinal := start + uint(i)
			wallet := strings.ToLower(actual[i].Wallet)
			row := entity.IDOReconciliationWallet{
				RunID: run.ID, Ordinal: ordinal, WalletAddress: wallet,
				ExpectedPaymentRaw: expected[i].PaymentAmountRaw, ActualPaymentRaw: actual[i].PaymentAmountRaw,
				ExpectedTokensRaw: expected[i].TokensBoughtRaw, ActualTokensRaw: actual[i].TokensBoughtRaw,
				ExpectedClaimedRaw: expected[i].ClaimedTokensRaw, ActualClaimedRaw: actual[i].ClaimedTokensRaw,
				ExpectedRefunded: expected[i].Refunded, ActualRefunded: actual[i].Refunded, ObservedAt: time.Now(),
			}
			row.PreviousRowDigest = previous
			row.IdentityDigest = commonReconciliation.WalletIdentityDigest(row.Ordinal, row.WalletAddress, row.ExpectedPaymentRaw, row.ExpectedTokensRaw, row.ExpectedClaimedRaw, row.ExpectedRefunded, previousIdentity)
			row.RowDigest = commonReconciliation.WalletDigest(row.RunID, row.Ordinal, row.WalletAddress, row.ExpectedPaymentRaw, row.ActualPaymentRaw, row.ExpectedTokensRaw, row.ActualTokensRaw, row.ExpectedClaimedRaw, row.ActualClaimedRaw, row.ExpectedRefunded, row.ActualRefunded, row.PreviousRowDigest)
			existing := entity.IDOReconciliationWallet{}
			err := tx.Where("run_id = ? AND ordinal = ?", row.RunID, row.Ordinal).Take(&existing).Error
			if err == nil {
				if existing.RowDigest != row.RowDigest || existing.WalletAddress != row.WalletAddress {
					return errors.New("existing wallet evidence does not match replayed page")
				}
			} else if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(&row).Error; err != nil {
					return err
				}
			} else {
				return err
			}
			previous = row.RowDigest
			previousIdentity = row.IdentityDigest
		}
		result := tx.Model(&entity.IDOReconciliationJob{}).Where("run_id = ? AND lease_token = ? AND state = 'running' AND next_wallet_ordinal = ? AND lease_expires_at > CURRENT_TIMESTAMP(6)", run.ID, token, start).Updates(map[string]interface{}{"next_wallet_ordinal": end, "consecutive_error_count": 0, "updated_at": time.Now()})
		if result.Error != nil || result.RowsAffected != 1 {
			return errors.New("wallet page checkpoint fencing rejected stale lease")
		}
		return appendEventTx(tx, run.ID, "PAGE_COMPLETED", "wallets", "", fmt.Sprintf("wallet ordinals [%d,%d) persisted", start, end), nil, token)
	})
}

func releaseForNextPage(runID uint64, token string, next uint) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		job, err := lockFencedJob(tx, runID, token)
		if err != nil {
			return err
		}
		if job.State == "cancel_requested" {
			return finishCancelledTx(tx, runID, token)
		}
		result := tx.Model(&entity.IDOReconciliationJob{}).Where("run_id = ? AND lease_token = ? AND state = 'running' AND next_wallet_ordinal = ? AND lease_expires_at > CURRENT_TIMESTAMP(6)", runID, token, next).Updates(map[string]interface{}{"state": "retrying", "lease_token": "", "lease_expires_at": nil, "next_attempt_at": time.Now(), "updated_at": time.Now()})
		return expectOne(result, "release reconciliation page")
	})
}

func finalize(ctx context.Context, reader commonReconciliation.Reader, run entity.IDOReconciliationRun, token string, source commonReconciliation.SourceEnvelope, base commonReconciliation.BaseSnapshot, consecutiveErrors uint) error {
	walletRows := make([]entity.IDOReconciliationWallet, 0, run.ExpectedWalletCount)
	if err := db.Mysql().WithContext(ctx).Where("run_id = ?", run.ID).Order("ordinal ASC").Find(&walletRows).Error; err != nil {
		return err
	}
	wallets := make([]commonReconciliation.WalletSnapshot, len(walletRows))
	walletChecks := make([]commonReconciliation.Check, 0, len(walletRows)*4)
	walletEvidenceDigest := commonReconciliation.ZeroDigest()
	walletIdentityDigest := commonReconciliation.ZeroDigest()
	for i, row := range walletRows {
		expectedIdentity := commonReconciliation.WalletIdentityDigest(row.Ordinal, row.WalletAddress, row.ExpectedPaymentRaw, row.ExpectedTokensRaw, row.ExpectedClaimedRaw, row.ExpectedRefunded, walletIdentityDigest)
		if row.Ordinal != uint(i) || row.PreviousRowDigest != walletEvidenceDigest || row.IdentityDigest != expectedIdentity || row.RowDigest != commonReconciliation.WalletDigest(row.RunID, row.Ordinal, row.WalletAddress, row.ExpectedPaymentRaw, row.ActualPaymentRaw, row.ExpectedTokensRaw, row.ActualTokensRaw, row.ExpectedClaimedRaw, row.ActualClaimedRaw, row.ExpectedRefunded, row.ActualRefunded, row.PreviousRowDigest) {
			return finishFailure(run.ID, token, "SOURCE_CHANGED", "wallets", "wallet evidence digest or ordinal mismatch", "invalidated")
		}
		walletEvidenceDigest = row.RowDigest
		walletIdentityDigest = row.IdentityDigest
		wallets[i] = commonReconciliation.WalletSnapshot{Wallet: row.WalletAddress, PaymentAmountRaw: row.ActualPaymentRaw, TokensBoughtRaw: row.ActualTokensRaw, ClaimedTokensRaw: row.ActualClaimedRaw, Refunded: row.ActualRefunded}
		walletChecks = append(walletChecks, commonReconciliation.WalletChecks(source.Request.Wallets[i], wallets[i])...)
	}
	result, err := commonReconciliation.Complete(ctx, reader, source.Request, base, wallets, walletChecks)
	if err != nil {
		return handleExecutionError(run.ID, token, consecutiveErrors+1, err)
	}
	if err := renewClaimLease(run.ID, token); err != nil {
		if errors.Is(err, errReconciliationCancelled) {
			return nil
		}
		return err
	}
	allChecks := append(append([]commonReconciliation.Check(nil), source.DatabaseChecks...), result.Checks...)
	if uint(len(allChecks)) != run.ExpectedCheckCount || walletIdentityDigest != run.ExpectedWalletIdentityDigest {
		return finishFailure(run.ID, token, "SOURCE_CHANGED", "evidence", "collected evidence identity count does not match frozen source", "invalidated")
	}
	databaseBalanced := commonReconciliation.AllChecksOK(source.DatabaseChecks)
	balanced := databaseBalanced && commonReconciliation.AllChecksOK(result.Checks)
	result.Balanced = balanced
	failed := uint(0)
	for _, check := range allChecks {
		if !check.OK {
			failed++
		}
	}
	outcome := "UNBALANCED"
	if failed == 0 {
		outcome = "BALANCED"
	}
	manifestOnchain := result
	manifestOnchain.Wallets = nil
	manifestOnchain.Checks = make([]commonReconciliation.Check, 0, len(result.Checks)-len(walletChecks))
	for _, check := range result.Checks {
		if !strings.HasPrefix(check.Code, "chain_wallet_") {
			manifestOnchain.Checks = append(manifestOnchain.Checks, check)
		}
	}
	materializedChecks := append(append([]commonReconciliation.Check(nil), source.DatabaseChecks...), manifestOnchain.Checks...)
	checkEvidenceDigest := commonReconciliation.ZeroDigest()
	for ordinal, check := range materializedChecks {
		scope := "onchain"
		if ordinal < len(source.DatabaseChecks) {
			scope = "database"
		}
		checkEvidenceDigest = commonReconciliation.CheckDigest(run.ID, uint(ordinal), scope, check.Code, check.OK, truncate(check.Expected, 500), truncate(check.Actual, 500), truncate(check.Message, 500), checkEvidenceDigest)
	}
	manifestPayload, payloadDigest, err := commonReconciliation.CanonicalJSON(commonReconciliation.ManifestEnvelope{Scope: "durable_operational_reconciliation", DatabaseBalanced: databaseBalanced, DatabaseChecks: source.DatabaseChecks, Onchain: manifestOnchain, WalletEvidenceDigest: walletEvidenceDigest, CheckEvidenceDigest: checkEvidenceDigest})
	if err != nil {
		return err
	}
	finalCtx, cancelFinal := context.WithTimeout(context.Background(), pageTimeout)
	defer cancelFinal()
	finalizeErr := db.Mysql().WithContext(finalCtx).Transaction(func(tx *gorm.DB) error {
		job, err := lockFencedJob(tx, run.ID, token)
		if err != nil {
			return err
		}
		if job.State == "cancel_requested" {
			return finishCancelledTx(tx, run.ID, token)
		}
		if job.State != "running" || job.NextWalletOrdinal != run.ExpectedWalletCount {
			return errors.New("finalization fencing rejected incomplete wallet progress")
		}
		if err := renewLeaseTx(tx, run.ID, token, "running"); err != nil {
			return err
		}
		if err := validateFinalDBFenceTx(tx, run, source); err != nil {
			return err
		}
		if err := verifyRunDigestTx(tx, run); err != nil {
			return err
		}
		if result := tx.Model(&entity.IDOReconciliationJob{}).Where("run_id = ? AND lease_token = ? AND state = 'running' AND lease_expires_at > CURRENT_TIMESTAMP(6)", run.ID, token).Update("state", "finalizing"); result.Error != nil || result.RowsAffected != 1 {
			return errors.New("enter finalizing state rejected stale lease")
		}
		previousCheckDigest := commonReconciliation.ZeroDigest()
		previousCheckIdentity := commonReconciliation.ZeroDigest()
		for ordinal, check := range materializedChecks {
			scope := "onchain"
			if ordinal < len(source.DatabaseChecks) {
				scope = "database"
			}
			row := entity.IDOReconciliationCheck{RunID: run.ID, Ordinal: uint(ordinal), Scope: scope, Code: check.Code, OK: check.OK, ExpectedExact: truncate(check.Expected, 500), ActualExact: truncate(check.Actual, 500), Message: truncate(check.Message, 500), PreviousRowDigest: previousCheckDigest, CreatedAt: time.Now()}
			row.IdentityDigest = commonReconciliation.CheckIdentityDigest(row.Ordinal, row.Scope, row.Code, previousCheckIdentity)
			row.RowDigest = commonReconciliation.CheckDigest(row.RunID, row.Ordinal, row.Scope, row.Code, row.OK, row.ExpectedExact, row.ActualExact, row.Message, row.PreviousRowDigest)
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			previousCheckDigest = row.RowDigest
			previousCheckIdentity = row.IdentityDigest
		}
		if previousCheckIdentity != run.ExpectedCheckIdentityDigest {
			return errors.New("materialized check identity set does not match frozen source")
		}
		previous := commonReconciliation.ZeroDigest()
		prior := entity.IDOReconciliationManifest{}
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("manifest_digest").Where("project_id = ?", run.ProjectID).Order("run_id DESC").Take(&prior).Error
		if err == nil {
			previous = prior.ManifestDigest
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		manifest := entity.IDOReconciliationManifest{RunID: run.ID, ProjectID: run.ProjectID, Outcome: outcome, WalletCount: uint(len(wallets)), CheckCount: uint(len(allChecks)), FailedCheckCount: failed, Payload: manifestPayload, PayloadDigest: payloadDigest, WalletEvidenceDigest: walletEvidenceDigest, CheckEvidenceDigest: checkEvidenceDigest, PreviousManifestDigest: previous, CompletedAt: time.Now()}
		manifest.ManifestDigest = commonReconciliation.ManifestDigest(manifest.RunID, manifest.ProjectID, manifest.Outcome, manifest.WalletCount, manifest.CheckCount, manifest.FailedCheckCount, manifest.PayloadDigest, manifest.WalletEvidenceDigest, manifest.CheckEvidenceDigest, manifest.PreviousManifestDigest)
		if err := tx.Create(&manifest).Error; err != nil {
			return err
		}
		completedAt := manifest.CompletedAt
		update := tx.Model(&entity.IDOReconciliationJob{}).Where("run_id = ? AND lease_token = ? AND state = 'finalizing' AND lease_expires_at > CURRENT_TIMESTAMP(6)", run.ID, token).Updates(map[string]interface{}{"state": "completed", "outcome": outcome, "active_slot": nil, "lease_token": "", "lease_expires_at": nil, "completed_at": completedAt, "updated_at": completedAt})
		if err := expectOne(update, "complete reconciliation"); err != nil {
			return err
		}
		return appendEventTx(tx, run.ID, "FINALIZED", "manifest", "", outcome, nil, token)
	})
	if errors.Is(finalizeErr, errFinalSnapshotInvalidated) {
		return finishFailure(run.ID, token, "SNAPSHOT_INVALIDATED", "database", finalizeErr.Error(), "invalidated")
	}
	return finalizeErr
}

func handleExecutionError(runID uint64, token string, consecutiveErrors uint, err error) error {
	code, terminal := classifyError(err)
	if terminal == "invalidated" || terminal == "failed" || consecutiveErrors >= maxAttempts {
		if terminal == "" {
			terminal, code = "failed", "INTERNAL_ERROR"
		}
		return finishFailure(runID, token, code, "collect", err.Error(), terminal)
	}
	return scheduleRetry(runID, token, code, err.Error(), consecutiveErrors)
}

func classifyError(err error) (string, string) {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "canonical") || strings.Contains(message, "implementation") || strings.Contains(message, "contract address") || strings.Contains(message, "chain id mismatch") {
		return "SNAPSHOT_INVALIDATED", "invalidated"
	}
	if strings.Contains(message, "deadline exceeded") || strings.Contains(message, "timeout") {
		return "RPC_TIMEOUT", ""
	}
	if strings.Contains(message, "participant count") {
		return "PARTICIPANT_LIMIT", "failed"
	}
	return "RPC_UNAVAILABLE", ""
}

func scheduleRetry(runID uint64, token, code, detail string, consecutiveErrors uint) error {
	delay := time.Duration(consecutiveErrors*consecutiveErrors) * time.Minute
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		job, err := lockFencedJob(tx, runID, token)
		if err != nil {
			return err
		}
		if job.State == "cancel_requested" {
			return finishCancelledTx(tx, runID, token)
		}
		message := truncate(detail, 500)
		result := tx.Model(&entity.IDOReconciliationJob{}).Where("run_id = ? AND lease_token = ? AND state = 'running' AND lease_expires_at > CURRENT_TIMESTAMP(6)", runID, token).Updates(map[string]interface{}{"state": "retrying", "consecutive_error_count": consecutiveErrors, "lease_token": "", "lease_expires_at": nil, "next_attempt_at": time.Now().Add(delay), "last_error_code": code, "last_error_message": message, "updated_at": time.Now()})
		if err := expectOne(result, "schedule reconciliation retry"); err != nil {
			return err
		}
		return appendEventTx(tx, runID, "RETRY_SCHEDULED", "collect", code, "retry scheduled", nil, token)
	})
}

func finishFailure(runID uint64, token, code, stage, detail, state string) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		job, err := lockFencedJob(tx, runID, token)
		if err != nil {
			return err
		}
		if job.State == "cancel_requested" {
			return finishCancelledTx(tx, runID, token)
		}
		now := time.Now()
		message := truncate(detail, 500)
		result := tx.Model(&entity.IDOReconciliationJob{}).Where("run_id = ? AND lease_token = ? AND state IN ? AND lease_expires_at > CURRENT_TIMESTAMP(6)", runID, token, []string{"running", "finalizing"}).Updates(map[string]interface{}{"state": state, "active_slot": nil, "lease_token": "", "lease_expires_at": nil, "last_error_code": code, "last_error_message": message, "completed_at": now, "updated_at": now})
		if err := expectOne(result, "finish reconciliation failure"); err != nil {
			return err
		}
		event := "FAILED"
		if state == "invalidated" {
			event = "INVALIDATED"
		}
		return appendEventTx(tx, runID, event, stage, code, "reconciliation collection did not finalize", nil, token)
	})
}

func finishCancelled(runID uint64, token string) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error { return finishCancelledTx(tx, runID, token) })
}

func finishCancelledTx(tx *gorm.DB, runID uint64, token string) error {
	job, err := lockFencedJob(tx, runID, token)
	if err != nil {
		return err
	}
	if job.State != "cancel_requested" {
		return errors.New("reconciliation cancellation fencing rejected current state")
	}
	now := time.Now()
	result := tx.Model(&entity.IDOReconciliationJob{}).Where("run_id = ? AND lease_token = ? AND state = 'cancel_requested' AND lease_expires_at > CURRENT_TIMESTAMP(6)", runID, token).Updates(map[string]interface{}{"state": "cancelled", "active_slot": nil, "lease_token": "", "lease_expires_at": nil, "completed_at": now, "updated_at": now})
	if err := expectOne(result, "finish reconciliation cancellation"); err != nil {
		return err
	}
	return appendEventTx(tx, runID, "CANCELLED", "cancel", "", "cancelled by administrator request", job.CancelRequestedByAdminID, token)
}

func loadFencedJob(ctx context.Context, runID uint64, token string) (entity.IDOReconciliationJob, error) {
	job := entity.IDOReconciliationJob{}
	err := db.Mysql().WithContext(ctx).Where(activeLeaseWhere, runID, token, []string{"running", "cancel_requested", "finalizing"}).Take(&job).Error
	return job, err
}

func lockFencedJob(tx *gorm.DB, runID uint64, token string) (entity.IDOReconciliationJob, error) {
	job := entity.IDOReconciliationJob{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(activeLeaseWhere, runID, token, []string{"running", "cancel_requested", "finalizing"}).Take(&job).Error
	return job, err
}

func renewLeaseTx(tx *gorm.DB, runID uint64, token, state string) error {
	result := tx.Model(&entity.IDOReconciliationJob{}).Where("run_id = ? AND lease_token = ? AND state = ? AND lease_expires_at > CURRENT_TIMESTAMP(6)", runID, token, state).Update("lease_expires_at", gorm.Expr("TIMESTAMPADD(SECOND, ?, CURRENT_TIMESTAMP(6))", int(leaseDuration/time.Second)))
	return expectOne(result, "renew reconciliation lease")
}

func renewClaimLease(runID uint64, token string) error {
	cancelled := false
	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		job, err := lockFencedJob(tx, runID, token)
		if err != nil {
			return err
		}
		if job.State == "cancel_requested" {
			if err := finishCancelledTx(tx, runID, token); err != nil {
				return err
			}
			cancelled = true
			return nil
		}
		return renewLeaseTx(tx, runID, token, "running")
	})
	if err == nil && cancelled {
		return errReconciliationCancelled
	}
	return err
}

func verifyRunDigestTx(tx *gorm.DB, expected entity.IDOReconciliationRun) error {
	current := entity.IDOReconciliationRun{}
	if err := tx.Select("id,project_id,source_payload,source_digest").Where("id = ?", expected.ID).Take(&current).Error; err != nil {
		return err
	}
	sum := sha256.Sum256(current.SourcePayload)
	if current.ProjectID != expected.ProjectID || current.SourceDigest != hex.EncodeToString(sum[:]) || current.SourceDigest != expected.SourceDigest {
		return errors.New("frozen reconciliation source digest changed")
	}
	return nil
}

func appendEventTx(tx *gorm.DB, runID uint64, eventType, stage, errorCode, message string, actorAdminID *uint64, leaseToken string) error {
	previous := commonReconciliation.ZeroDigest()
	sequence := uint(1)
	last := entity.IDOReconciliationEvent{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("sequence_no,event_hash").Where("run_id = ?", runID).Order("sequence_no DESC").Take(&last).Error
	if err == nil {
		sequence, previous = last.SequenceNo+1, last.EventHash
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	actor := uint64(0)
	if actorAdminID != nil {
		actor = *actorAdminID
	}
	leaseHash := ""
	if leaseToken != "" {
		sum := sha256.Sum256([]byte(leaseToken))
		leaseHash = hex.EncodeToString(sum[:])
	}
	row := entity.IDOReconciliationEvent{RunID: runID, SequenceNo: sequence, EventType: eventType, Stage: stage, ErrorCode: errorCode, Message: truncate(message, 500), ActorAdminID: actorAdminID, LeaseTokenHash: leaseHash, PreviousEventHash: previous, CreatedAt: time.Now()}
	row.EventHash = commonReconciliation.EventDigest(row.RunID, row.SequenceNo, row.EventType, row.Stage, row.ErrorCode, row.Message, actor, row.LeaseTokenHash, row.PreviousEventHash)
	return tx.Create(&row).Error
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func expectOne(result *gorm.DB, action string) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("%s rejected stale lease", action)
	}
	return nil
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}

func validateOperationalDB(ctx context.Context, run entity.IDOReconciliationRun, source commonReconciliation.SourceEnvelope) error {
	request := source.Request
	project := entity.Project{}
	if err := db.Mysql().WithContext(ctx).Select("project_id,funding_mode,contract_version,chain_id,contract_address").Where("project_id = ?", run.ProjectID).Take(&project).Error; err != nil {
		return err
	}
	if !constant.IsExactV2ProjectIdentity(project.FundingMode, project.ContractVersion) || project.ChainID != run.ChainID || !strings.EqualFold(project.ContractAddress, run.ContractAddress) {
		return errors.New("reconciliation project is no longer an exact V2 identity")
	}
	state := entity.ChainEventSyncState{}
	if err := db.Mysql().WithContext(ctx).Where("id = 1").Take(&state).Error; err != nil {
		return err
	}
	maxLag := request.MaxHealthyLagBlocks
	if !state.Initialized || !state.V2Registered || state.LastSuccessAt == nil || state.LastError != "" || state.CursorBlock < run.EvidenceBlockNumber || state.CursorBlock > state.SafeBlock || state.LagBlocks > maxLag || state.UpdatedAt.IsZero() || time.Since(state.UpdatedAt) > 3*time.Minute {
		return errors.New("chain event sync is unhealthy or moved behind the frozen source")
	}
	if !strings.EqualFold(state.V2ContractAddress, run.ContractAddress) || !strings.EqualFold(state.V2ImplementationAddress, run.ImplementationAddress) || !strings.EqualFold(state.V2ImplementationCodeHash, run.ImplementationCodeHash) {
		return errors.New("registered V2 source identity drifted after task creation")
	}
	var activeReorgs int64
	if err := db.Mysql().WithContext(ctx).Model(&entity.ChainEventReorgIncident{}).Where("chain_id = ? AND LOWER(contract_address) = ? AND active_slot = 1 AND status = ?", run.ChainID, strings.ToLower(run.ContractAddress), "detected").Count(&activeReorgs).Error; err != nil {
		return err
	}
	if activeReorgs != 0 {
		return errors.New("active chain event reorg incident invalidates the frozen source")
	}
	if viper.GetUint64("chain.project.chainID") != run.ChainID || !strings.EqualFold(viper.GetString("chain.project.v2.contractAddress"), run.ContractAddress) || !strings.EqualFold(viper.GetString("chain.project.v2.implementationAddress"), run.ImplementationAddress) || !strings.EqualFold(viper.GetString("chain.project.v2.implementationCodeHash"), run.ImplementationCodeHash) {
		return errors.New("trusted V2 configuration drifted after task creation")
	}
	confirmations, err := baseChain.ResolveConfirmations(run.ChainID, viper.GetString("chain.project.confirmations"))
	if err != nil || confirmations != run.ConfirmationDepth || confirmations != request.LatestConfirmationDepth {
		return errors.New("trusted confirmation depth drifted after task creation")
	}
	var unresolved int64
	if err := db.Mysql().WithContext(ctx).Model(&entity.ChainEventFailure{}).Where(
		"block_number <= ? AND LOWER(contract_address) = ? AND contract_version = ? AND status IN ?",
		run.EvidenceBlockNumber, strings.ToLower(run.ContractAddress), constant.ContractVersionV2Stablecoin,
		[]string{constant.ChainEventFailureRetrying, constant.ChainEventFailureDeadLetter},
	).Count(&unresolved).Error; err != nil {
		return err
	}
	if unresolved != 0 {
		return errors.New("unresolved chain event failure affects the frozen evidence range")
	}
	for _, factSet := range []struct {
		table string
		want  uint64
	}{
		{"fund_ledger", source.FundLedgerFacts.RowCount},
		{"ido_purchase_entitlement", source.PurchaseEntitlements.RowCount},
	} {
		if err := validateJobFactCount(ctx, db.Mysql(), factSet.table, run.ProjectID, factSet.want); err != nil {
			return err
		}
	}
	return nil
}

func validateJobFactCount(ctx context.Context, database *gorm.DB, table string, projectID, want uint64) error {
	var count int64
	// The frozen source commits every child fact for the admitted project, including
	// malformed version/status rows. The worker must use the same universe so a
	// corrupt fact cannot disappear from final evidence through a read filter.
	if err := database.WithContext(ctx).Table(table).Where("project_id = ?", projectID).Count(&count).Error; err != nil {
		return err
	}
	if count < 0 || uint64(count) != want {
		return errors.New("financial evidence set changed after source creation")
	}
	return nil
}

func validateFinalDBFenceTx(tx *gorm.DB, run entity.IDOReconciliationRun, source commonReconciliation.SourceEnvelope) error {
	project := entity.Project{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("project_id,funding_mode,contract_version,chain_id,contract_address").Where("project_id = ?", run.ProjectID).Take(&project).Error; err != nil {
		return err
	}
	if !constant.IsExactV2ProjectIdentity(project.FundingMode, project.ContractVersion) || project.ChainID != run.ChainID || !strings.EqualFold(project.ContractAddress, run.ContractAddress) {
		return fmt.Errorf("%w: reconciliation project is no longer an exact V2 identity", errFinalSnapshotInvalidated)
	}
	state := entity.ChainEventSyncState{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = 1").Take(&state).Error; err != nil {
		return err
	}
	request := source.Request
	if !state.Initialized || !state.V2Registered || state.LastSuccessAt == nil || state.LastError != "" || state.CursorBlock < run.EvidenceBlockNumber || state.CursorBlock > state.SafeBlock || state.LagBlocks > request.MaxHealthyLagBlocks || state.UpdatedAt.IsZero() || time.Since(state.UpdatedAt) > 3*time.Minute {
		return fmt.Errorf("%w: chain event sync is unhealthy", errFinalSnapshotInvalidated)
	}
	if !strings.EqualFold(state.V2ContractAddress, run.ContractAddress) || !strings.EqualFold(state.V2ImplementationAddress, run.ImplementationAddress) || !strings.EqualFold(state.V2ImplementationCodeHash, run.ImplementationCodeHash) {
		return fmt.Errorf("%w: registered V2 source identity drifted", errFinalSnapshotInvalidated)
	}
	var activeReorgs int64
	if err := tx.Model(&entity.ChainEventReorgIncident{}).Where("chain_id = ? AND LOWER(contract_address) = ? AND active_slot = 1 AND status = ?", run.ChainID, strings.ToLower(run.ContractAddress), "detected").Count(&activeReorgs).Error; err != nil {
		return err
	}
	if activeReorgs != 0 {
		return fmt.Errorf("%w: active chain event reorg incident", errFinalSnapshotInvalidated)
	}
	confirmations, confirmationErr := baseChain.ResolveConfirmations(run.ChainID, viper.GetString("chain.project.confirmations"))
	if confirmationErr != nil || confirmations != run.ConfirmationDepth || confirmations != request.LatestConfirmationDepth || viper.GetUint64("chain.project.chainID") != run.ChainID || !strings.EqualFold(viper.GetString("chain.project.v2.contractAddress"), run.ContractAddress) || !strings.EqualFold(viper.GetString("chain.project.v2.implementationAddress"), run.ImplementationAddress) || !strings.EqualFold(viper.GetString("chain.project.v2.implementationCodeHash"), run.ImplementationCodeHash) {
		return fmt.Errorf("%w: trusted V2 runtime configuration drifted", errFinalSnapshotInvalidated)
	}
	var unresolved int64
	if err := tx.Model(&entity.ChainEventFailure{}).Where(
		"block_number <= ? AND LOWER(contract_address) = ? AND contract_version = ? AND status IN ?",
		run.EvidenceBlockNumber, strings.ToLower(run.ContractAddress), constant.ContractVersionV2Stablecoin,
		[]string{constant.ChainEventFailureRetrying, constant.ChainEventFailureDeadLetter},
	).Count(&unresolved).Error; err != nil {
		return err
	}
	if unresolved != 0 {
		return fmt.Errorf("%w: unresolved chain event failure affects final evidence", errFinalSnapshotInvalidated)
	}
	ledger, err := jobFundLedgerFactCommitmentTx(tx, run.ProjectID)
	if err != nil {
		return err
	}
	entitlements, err := jobEntitlementFactCommitmentTx(tx, run.ProjectID)
	if err != nil {
		return err
	}
	if ledger != source.FundLedgerFacts || entitlements != source.PurchaseEntitlements {
		return fmt.Errorf("%w: financial evidence commitment changed within the frozen block range", errFinalSnapshotInvalidated)
	}
	return nil
}

func jobFundLedgerFactCommitmentTx(database *gorm.DB, projectID uint64) (commonReconciliation.FactSetCommitment, error) {
	result := commonReconciliation.FactSetCommitment{RootDigest: commonReconciliation.ZeroDigest()}
	rows, err := database.Model(&entity.FundLedger{}).Where("project_id = ?", projectID).Order("id ASC").Rows()
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		fact := entity.FundLedger{}
		if err := database.ScanRows(rows, &fact); err != nil {
			return result, err
		}
		_, payloadDigest, err := commonReconciliation.CanonicalJSON(fact)
		if err != nil {
			return result, err
		}
		result.RootDigest = commonReconciliation.FactRowDigest("fund_ledger", result.RowCount, payloadDigest, result.RootDigest)
		result.RowCount++
	}
	return result, rows.Err()
}

func jobEntitlementFactCommitmentTx(database *gorm.DB, projectID uint64) (commonReconciliation.FactSetCommitment, error) {
	result := commonReconciliation.FactSetCommitment{RootDigest: commonReconciliation.ZeroDigest()}
	rows, err := database.Model(&entity.IDOPurchaseEntitlement{}).Where("project_id = ?", projectID).Order("id ASC").Rows()
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		fact := entity.IDOPurchaseEntitlement{}
		if err := database.ScanRows(rows, &fact); err != nil {
			return result, err
		}
		_, payloadDigest, err := commonReconciliation.CanonicalJSON(fact)
		if err != nil {
			return result, err
		}
		result.RootDigest = commonReconciliation.FactRowDigest("purchase_entitlement", result.RowCount, payloadDigest, result.RootDigest)
		result.RowCount++
	}
	return result, rows.Err()
}

func reapOneExpired(ctx context.Context) error {
	now := time.Now()
	return db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		job := entity.IDOReconciliationJob{}
		if err := tx.Model(&entity.IDOReconciliationJob{}).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Joins("JOIN ido_reconciliation_run r ON r.id = ido_reconciliation_job.run_id").
			Joins("JOIN projects p ON p.project_id = r.project_id").
			Where("ido_reconciliation_job.state IN ? AND ido_reconciliation_job.lease_expires_at < CURRENT_TIMESTAMP(6) AND p.funding_mode = ? AND p.contract_version = ?", []string{"running", "cancel_requested", "finalizing"}, constant.FundingModeCappedPresale, constant.ContractVersionV2Stablecoin).
			Order("ido_reconciliation_job.lease_expires_at ASC,ido_reconciliation_job.run_id ASC").Take(&job).Error; err != nil {
			return err
		}
		if job.State == "cancel_requested" {
			return finishExpiredCancellationTx(tx, job, now)
		}
		var manifestCount int64
		if err := tx.Model(&entity.IDOReconciliationManifest{}).Where("run_id = ?", job.RunID).Count(&manifestCount).Error; err != nil {
			return err
		}
		if manifestCount != 0 {
			return errors.New("expired reconciliation lease has a manifest but is not completed")
		}
		if job.ConsecutiveErrorCount+1 >= maxAttempts {
			completedAt := now
			result := tx.Model(&entity.IDOReconciliationJob{}).Where("run_id = ? AND state = ? AND lease_token = ?", job.RunID, job.State, job.LeaseToken).Updates(map[string]interface{}{
				"state": "failed", "active_slot": nil, "consecutive_error_count": job.ConsecutiveErrorCount + 1, "lease_token": "", "lease_expires_at": nil,
				"last_error_code": "INTERNAL_ERROR", "last_error_message": "worker lease expired repeatedly before checkpoint", "completed_at": completedAt, "updated_at": now,
			})
			if err := expectOne(result, "fail repeatedly expired reconciliation lease"); err != nil {
				return err
			}
			return appendEventTx(tx, job.RunID, "FAILED", "lease", "INTERNAL_ERROR", "worker lease repeatedly expired before checkpoint", nil, job.LeaseToken)
		}
		result := tx.Model(&entity.IDOReconciliationJob{}).Where("run_id = ? AND state = ? AND lease_token = ?", job.RunID, job.State, job.LeaseToken).Updates(map[string]interface{}{
			"state": "retrying", "consecutive_error_count": job.ConsecutiveErrorCount + 1, "lease_token": "", "lease_expires_at": nil,
			"next_attempt_at": now, "last_error_code": "INTERNAL_ERROR", "last_error_message": "worker lease expired before checkpoint", "updated_at": now,
		})
		if err := expectOne(result, "reap reconciliation lease"); err != nil {
			return err
		}
		return appendEventTx(tx, job.RunID, "RETRY_SCHEDULED", "lease", "INTERNAL_ERROR", "expired worker lease was recovered", nil, job.LeaseToken)
	})
}

func finishExpiredCancellationTx(tx *gorm.DB, job entity.IDOReconciliationJob, now time.Time) error {
	result := tx.Model(&entity.IDOReconciliationJob{}).Where("run_id = ? AND state = 'cancel_requested' AND lease_token = ?", job.RunID, job.LeaseToken).Updates(map[string]interface{}{"state": "cancelled", "active_slot": nil, "lease_token": "", "lease_expires_at": nil, "completed_at": now, "updated_at": now})
	if err := expectOne(result, "reap reconciliation cancellation"); err != nil {
		return err
	}
	return appendEventTx(tx, job.RunID, "CANCELLED", "lease", "", "cancelled after worker lease expiry", job.CancelRequestedByAdminID, job.LeaseToken)
}

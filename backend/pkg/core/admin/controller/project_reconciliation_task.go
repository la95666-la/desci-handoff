package controller

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	driverMysql "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	baseChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idoreconciliation"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/core/admin/middleware"
	"joincare_server/pkg/core/db"
)

const (
	defaultAsyncReconciliationParticipants = 2_000
	hardAsyncReconciliationParticipants    = 10_000
	maxAsyncReconciliationFactRows         = 50_000
	maxAsyncReconciliationSourceBytes      = 12 * 1024 * 1024
)

var (
	errIDOReconciliationActive         = errors.New("an active on-chain reconciliation already exists for this project")
	errIDOReconciliationDuplicate      = errors.New("a concurrent idempotent reconciliation request won")
	errIDOReconciliationNotCancellable = errors.New("on-chain reconciliation is not cancellable in its current state")
)

type createIDOReconciliationRequest struct {
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotency_key"`
}

type idoReconciliationSummary struct {
	RunID                uint64     `json:"run_id"`
	RunUUID              string     `json:"run_uuid"`
	ProjectID            uint64     `json:"project_id"`
	State                string     `json:"state"`
	Outcome              *string    `json:"outcome"`
	Balanced             *bool      `json:"balanced"`
	EvidenceBlockNumber  uint64     `json:"evidence_block_number"`
	EvidenceBlockHash    string     `json:"evidence_block_hash"`
	ExpectedWalletCount  uint       `json:"expected_wallet_count"`
	ProcessedWalletCount uint       `json:"processed_wallet_count"`
	AttemptCount         uint       `json:"attempt_count"`
	LastErrorCode        string     `json:"last_error_code"`
	LastErrorMessage     string     `json:"last_error_message"`
	SourceDigest         string     `json:"source_digest"`
	ManifestDigest       string     `json:"manifest_digest,omitempty"`
	InvalidatedByReorg   bool       `json:"invalidated_by_reorg"`
	RequestedByAdminID   uint64     `json:"requested_by_admin_id"`
	Reason               string     `json:"reason"`
	RequestedAt          time.Time  `json:"requested_at"`
	StartedAt            *time.Time `json:"started_at"`
	CompletedAt          *time.Time `json:"completed_at"`
}

type idoReconciliationDetail struct {
	Summary          idoReconciliationSummary         `json:"summary"`
	DatabaseBalanced *bool                            `json:"database_balanced"`
	DatabaseChecks   []idoreconciliation.Check        `json:"database_checks"`
	Onchain          *idoReconciliationOnchainSummary `json:"onchain"`
}

type idoReconciliationOnchainSummary struct {
	Scope         string                                   `json:"scope"`
	Balanced      *bool                                    `json:"balanced"`
	BlockNumber   uint64                                   `json:"block_number"`
	BlockHash     string                                   `json:"block_hash"`
	LatestBlock   uint64                                   `json:"latest_block"`
	Project       idoreconciliation.ProjectSnapshot        `json:"project"`
	TokenReserves []idoreconciliation.TokenReserveSnapshot `json:"token_reserves"`
	Checks        []idoreconciliation.Check                `json:"checks"`
}

// CreateProjectOnchainReconciliation POST /admin/v1/projects/:id/onchain-reconciliations
func CreateProjectOnchainReconciliation(c *gin.Context) {
	projectID, ok := adminProjectID(c)
	if !ok {
		return
	}
	if _, err := loadV2FundProject(db.Mysql().WithContext(c.Request.Context()), uint64(projectID)); err != nil {
		fundProjectReadError(c, err)
		return
	}
	request := createIDOReconciliationRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "IDO_RECONCILIATION_REQUEST_INVALID", "message": "reason and idempotency_key are required"})
		return
	}
	request.Reason = strings.TrimSpace(request.Reason)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if utf8.RuneCountInString(request.Reason) < 3 || utf8.RuneCountInString(request.Reason) > 500 || len(request.IdempotencyKey) < 16 || len(request.IdempotencyKey) > 100 || !asciiToken(request.IdempotencyKey) {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "IDO_RECONCILIATION_REQUEST_INVALID", "message": "reason must be 3-500 characters and idempotency_key must be a 16-100 character ASCII token"})
		return
	}
	adminID := currentAdminID(c)
	if adminID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "code": "IDO_RECONCILIATION_ADMIN_REQUIRED", "message": "authenticated administrator is required"})
		return
	}
	roles := currentAdminRoles(c)
	run, created, err := createIDOReconciliationRun(c.Request.Context(), uint64(projectID), adminID, roles, request)
	if err != nil {
		log.GetLogger().WithError(err).WithField("project_id", projectID).Error("IDO reconciliation create failed")
		status, code := http.StatusInternalServerError, "IDO_RECONCILIATION_CREATE_FAILED"
		publicMessage := "reconciliation task could not be created; inspect restricted service logs"
		if errors.Is(err, errAdminProjectExecuteRevoked) {
			status, code = http.StatusForbidden, "IDO_RECONCILIATION_ADMIN_REVOKED"
			publicMessage = "project execute permission is no longer available"
		} else if errors.Is(err, errIDOReconciliationActive) {
			status, code = http.StatusConflict, "IDO_RECONCILIATION_ACTIVE"
			publicMessage = errIDOReconciliationActive.Error()
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			status, code = http.StatusNotFound, "IDO_RECONCILIATION_PROJECT_NOT_FOUND"
			publicMessage = "project was not found"
		} else if strings.Contains(err.Error(), "participant limit") || strings.Contains(err.Error(), "on-chain created") {
			status, code = http.StatusConflict, "IDO_RECONCILIATION_NOT_READY"
			publicMessage = "project is not ready for durable on-chain reconciliation"
		}
		c.JSON(status, gin.H{"status": false, "code": code, "message": publicMessage})
		return
	}
	status := http.StatusAccepted
	if !created {
		status = http.StatusOK
	}
	summary, summaryErr := loadIDOReconciliationSummary(db.Mysql().WithContext(c.Request.Context()), run.ID)
	if summaryErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "IDO_RECONCILIATION_READ_FAILED", "message": "reconciliation was accepted but its status could not be read"})
		return
	}
	c.JSON(status, gin.H{"status": true, "data": gin.H{"run_id": run.ID, "run_uuid": run.RunUUID, "state": summary.State, "created": created}})
}

func createIDOReconciliationRun(ctx context.Context, projectID, adminID uint64, roles string, request createIDOReconciliationRequest) (entity.IDOReconciliationRun, bool, error) {
	database := db.Mysql().WithContext(ctx)
	result := entity.IDOReconciliationRun{}
	created := false
	err := database.Transaction(func(tx *gorm.DB) error {
		if _, err := loadV2FundProject(tx, projectID); err != nil {
			return err
		}
		syncState := entity.ChainEventSyncState{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = 1").Take(&syncState).Error; err != nil {
			return err
		}
		if err := requireCurrentProjectExecuteAdmin(tx, adminID); err != nil {
			return err
		}
		existing := entity.IDOReconciliationRun{}
		err := tx.Select("id,run_uuid,project_id,reason").Where("requested_by_admin_id = ? AND idempotency_key = ?", adminID, request.IdempotencyKey).Take(&existing).Error
		if err == nil {
			if existing.ProjectID != projectID || existing.Reason != request.Reason {
				return errors.New("idempotency key was already used for a different reconciliation request")
			}
			result = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		snapshot, err := loadOnchainDBSnapshotForTask(tx, projectID)
		if err != nil {
			return err
		}
		limit := asyncReconciliationParticipantLimit()
		if len(snapshot.accumulator.purchaseWallets) > limit {
			return errors.New("project participant count exceeds configured asynchronous reconciliation participant limit")
		}
		confirmations, err := baseChain.ResolveConfirmations(snapshot.project.ChainID, viper.GetString("chain.project.confirmations"))
		if err != nil {
			return err
		}
		maxLag := viper.GetUint64("chainEvent.maxHealthyLagBlocks")
		if maxLag == 0 {
			maxLag = 1000
		}
		reconcileRequest := buildOnchainReconciliationRequest(snapshot, confirmations, maxLag, limit)
		_, dbChecks, _ := snapshot.accumulator.Result()
		expectedWalletIdentityDigest, expectedCheckIdentityDigest, expectedCheckCount := idoreconciliation.ExpectedEvidenceIdentity(dbChecks, reconcileRequest)
		dbBalanced := allReconciliationChecksOK(dbChecks)
		ledgerFacts, entitlementFacts, err := loadFundSourceFactCommitmentsTx(tx, projectID)
		if err != nil {
			return err
		}
		sourcePayload, sourceDigest, err := idoreconciliation.CanonicalJSON(idoreconciliation.SourceEnvelope{Scope: "database_ledger_plus_onchain_snapshot", DatabaseBalanced: dbBalanced, DatabaseChecks: dbChecks, FundLedgerFacts: ledgerFacts, PurchaseEntitlements: entitlementFacts, ExpectedWalletIdentityDigest: expectedWalletIdentityDigest, ExpectedCheckIdentityDigest: expectedCheckIdentityDigest, ExpectedCheckCount: expectedCheckCount, Request: reconcileRequest})
		if err != nil {
			return err
		}
		if len(sourcePayload) > maxAsyncReconciliationSourceBytes {
			return errors.New("canonical reconciliation source exceeds configured storage safety limit")
		}
		now := time.Now()
		result = entity.IDOReconciliationRun{
			RunUUID: strings.ToLower(uuid.NewString()), ProjectID: projectID, ChainID: snapshot.project.ChainID, ContractAddress: strings.ToLower(snapshot.project.ContractAddress),
			RequestedByAdminID: adminID, RequestedRoleCodes: roles, Reason: request.Reason, IdempotencyKey: request.IdempotencyKey,
			AlgorithmVersion: idoreconciliation.AlgorithmVersion, CanonicalizationVersion: idoreconciliation.CanonicalizationVersion,
			EvidenceBlockNumber: snapshot.sync.CursorBlock, EvidenceBlockHash: strings.ToLower(snapshot.sync.CursorBlockHash),
			ImplementationAddress: strings.ToLower(snapshot.sync.V2ImplementationAddress), ImplementationCodeHash: strings.ToLower(snapshot.sync.V2ImplementationCodeHash),
			ConfirmationDepth: confirmations, MaxHealthyLagBlocks: maxLag, ExpectedWalletCount: uint(len(reconcileRequest.Wallets)), ExpectedCheckCount: expectedCheckCount, ExpectedWalletIdentityDigest: expectedWalletIdentityDigest, ExpectedCheckIdentityDigest: expectedCheckIdentityDigest, SourcePayload: sourcePayload, SourceDigest: sourceDigest, RequestedAt: now,
		}
		if err := tx.Create(&result).Error; err != nil {
			if isDuplicateKey(err) {
				return errIDOReconciliationDuplicate
			}
			return err
		}
		active := uint8(1)
		job := entity.IDOReconciliationJob{RunID: result.ID, ProjectID: projectID, State: "queued", ActiveSlot: &active, NextAttemptAt: now, UpdatedAt: now}
		if err := tx.Create(&job).Error; err != nil {
			if isDuplicateKey(err) {
				return errIDOReconciliationActive
			}
			return err
		}
		if err := appendIDOReconciliationEventTx(tx, result.ID, "QUEUED", "request", "", "", &adminID, ""); err != nil {
			return err
		}
		created = true
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if errors.Is(err, errIDOReconciliationDuplicate) {
		existing := entity.IDOReconciliationRun{}
		readErr := database.Select("id,run_uuid,project_id,reason").Where("requested_by_admin_id = ? AND idempotency_key = ?", adminID, request.IdempotencyKey).Take(&existing).Error
		if readErr != nil {
			return result, false, readErr
		}
		if existing.ProjectID != projectID || existing.Reason != request.Reason {
			return result, false, errors.New("idempotency key was already used for a different reconciliation request")
		}
		return existing, false, nil
	}
	return result, created, err
}

func loadFundSourceFactCommitmentsTx(tx *gorm.DB, projectID uint64) (idoreconciliation.FactSetCommitment, idoreconciliation.FactSetCommitment, error) {
	ledger, err := fundLedgerFactCommitmentTx(tx, projectID)
	if err != nil {
		return ledger, idoreconciliation.FactSetCommitment{}, err
	}
	entitlements, err := entitlementFactCommitmentTx(tx, projectID)
	return ledger, entitlements, err
}

func fundLedgerFactCommitmentTx(tx *gorm.DB, projectID uint64) (idoreconciliation.FactSetCommitment, error) {
	result := idoreconciliation.FactSetCommitment{RootDigest: idoreconciliation.ZeroDigest()}
	rows, err := tx.Model(&entity.FundLedger{}).Where("project_id = ?", projectID).Order("id ASC").Rows()
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		if result.RowCount >= maxAsyncReconciliationFactRows {
			return result, errors.New("fund ledger fact count exceeds reconciliation safety limit")
		}
		fact := entity.FundLedger{}
		if err := tx.ScanRows(rows, &fact); err != nil {
			return result, err
		}
		_, payloadDigest, err := idoreconciliation.CanonicalJSON(fact)
		if err != nil {
			return result, err
		}
		result.RootDigest = idoreconciliation.FactRowDigest("fund_ledger", result.RowCount, payloadDigest, result.RootDigest)
		result.RowCount++
	}
	return result, rows.Err()
}

func entitlementFactCommitmentTx(tx *gorm.DB, projectID uint64) (idoreconciliation.FactSetCommitment, error) {
	result := idoreconciliation.FactSetCommitment{RootDigest: idoreconciliation.ZeroDigest()}
	rows, err := tx.Model(&entity.IDOPurchaseEntitlement{}).Where("project_id = ?", projectID).Order("id ASC").Rows()
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		if result.RowCount >= maxAsyncReconciliationFactRows {
			return result, errors.New("purchase entitlement fact count exceeds reconciliation safety limit")
		}
		fact := entity.IDOPurchaseEntitlement{}
		if err := tx.ScanRows(rows, &fact); err != nil {
			return result, err
		}
		_, payloadDigest, err := idoreconciliation.CanonicalJSON(fact)
		if err != nil {
			return result, err
		}
		result.RootDigest = idoreconciliation.FactRowDigest("purchase_entitlement", result.RowCount, payloadDigest, result.RootDigest)
		result.RowCount++
	}
	return result, rows.Err()
}

func loadOnchainDBSnapshotForTask(tx *gorm.DB, projectID uint64) (onchainDBSnapshot, error) {
	snapshot := onchainDBSnapshot{}
	for _, table := range []string{"fund_ledger", "ido_purchase_entitlement"} {
		var count int64
		if err := tx.Table(table).Where("project_id = ?", projectID).Count(&count).Error; err != nil {
			return snapshot, err
		}
		if count > maxAsyncReconciliationFactRows {
			return snapshot, errors.New("project financial fact count exceeds reconciliation safety limit")
		}
	}
	var err error
	snapshot.project, snapshot.accumulator, snapshot.entitlements, err = loadFundFactsTx(tx, projectID)
	if err != nil {
		return snapshot, err
	}
	if !snapshot.project.OnchainLocked {
		return snapshot, errors.New("V2 project has not reached an on-chain created state")
	}
	if err := tx.Where("project_id = ?", projectID).Order("`index` ASC").Find(&snapshot.tges).Error; err != nil {
		return snapshot, err
	}
	if err := tx.Where("id = 1").Take(&snapshot.sync).Error; err != nil {
		return snapshot, err
	}
	if err := validateOnchainOperationalStateTx(tx, snapshot.project, snapshot.sync, snapshot.sync.CursorBlock); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func GetIDOReconciliation(c *gin.Context) {
	runID, ok := reconciliationRunID(c)
	if !ok {
		return
	}
	if err := requireV2ReconciliationRun(db.Mysql().WithContext(c.Request.Context()), runID); err != nil {
		fundProjectReadError(c, err)
		return
	}
	summary, err := loadIDOReconciliationSummary(db.Mysql().WithContext(c.Request.Context()), runID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "code": "IDO_RECONCILIATION_NOT_FOUND", "message": "reconciliation run not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "IDO_RECONCILIATION_READ_FAILED", "message": "reconciliation status is unavailable"})
		return
	}
	detail := idoReconciliationDetail{Summary: summary, DatabaseChecks: []idoreconciliation.Check{}}
	if summary.Outcome != nil {
		manifest := entity.IDOReconciliationManifest{}
		if err := db.Mysql().WithContext(c.Request.Context()).Select("run_id,project_id,outcome,wallet_count,check_count,failed_check_count,payload,payload_digest,wallet_evidence_digest,check_evidence_digest,previous_manifest_digest,manifest_digest").Where("run_id = ?", runID).Take(&manifest).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "IDO_RECONCILIATION_MANIFEST_READ_FAILED", "message": "finalized reconciliation manifest is unavailable"})
			return
		}
		_, digest, err := idoreconciliation.CanonicalJSON(json.RawMessage(manifest.Payload))
		expectedManifestDigest := idoreconciliation.ManifestDigest(manifest.RunID, manifest.ProjectID, manifest.Outcome, manifest.WalletCount, manifest.CheckCount, manifest.FailedCheckCount, manifest.PayloadDigest, manifest.WalletEvidenceDigest, manifest.CheckEvidenceDigest, manifest.PreviousManifestDigest)
		if err != nil || digest != manifest.PayloadDigest || expectedManifestDigest != manifest.ManifestDigest || manifest.Outcome != *summary.Outcome {
			c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "IDO_RECONCILIATION_MANIFEST_INVALID", "message": "finalized reconciliation manifest digest is invalid"})
			return
		}
		payload := idoreconciliation.ManifestEnvelope{}
		if err := idoreconciliation.DecodeCanonicalJSON(manifest.Payload, &payload); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "IDO_RECONCILIATION_MANIFEST_INVALID", "message": "finalized reconciliation manifest encoding is invalid"})
			return
		}
		if payload.WalletEvidenceDigest != manifest.WalletEvidenceDigest || payload.CheckEvidenceDigest != manifest.CheckEvidenceDigest || verifyIDOReconciliationEvidence(db.Mysql().WithContext(c.Request.Context()), runID, manifest, payload) != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "IDO_RECONCILIATION_MANIFEST_INVALID", "message": "finalized reconciliation evidence chain is invalid"})
			return
		}
		if !summary.InvalidatedByReorg {
			databaseBalanced := payload.DatabaseBalanced
			detail.DatabaseBalanced = &databaseBalanced
		}
		detail.DatabaseChecks = payload.DatabaseChecks
		var onchainBalanced *bool
		if !summary.InvalidatedByReorg {
			value := payload.Onchain.Balanced
			onchainBalanced = &value
		}
		detail.Onchain = &idoReconciliationOnchainSummary{Scope: payload.Onchain.Scope, Balanced: onchainBalanced, BlockNumber: payload.Onchain.BlockNumber, BlockHash: payload.Onchain.BlockHash, LatestBlock: payload.Onchain.LatestBlock, Project: payload.Onchain.Project, TokenReserves: payload.Onchain.TokenReserves, Checks: payload.Onchain.Checks}
	}
	finalSummary, err := loadIDOReconciliationSummary(db.Mysql().WithContext(c.Request.Context()), runID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "IDO_RECONCILIATION_READ_FAILED", "message": "reconciliation current status is unavailable"})
		return
	}
	detail.Summary = finalSummary
	if finalSummary.InvalidatedByReorg {
		detail.DatabaseBalanced = nil
		if detail.Onchain != nil {
			detail.Onchain.Balanced = nil
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": detail})
}

func verifyIDOReconciliationEvidence(database *gorm.DB, runID uint64, manifest entity.IDOReconciliationManifest, payload idoreconciliation.ManifestEnvelope) error {
	wallets := make([]entity.IDOReconciliationWallet, 0, manifest.WalletCount)
	if err := database.Select("run_id,ordinal,wallet_address,expected_payment_raw,actual_payment_raw,expected_tokens_raw,actual_tokens_raw,expected_claimed_raw,actual_claimed_raw,expected_refunded,actual_refunded,previous_row_digest,identity_digest,row_digest").Where("run_id = ?", runID).Order("ordinal ASC").Find(&wallets).Error; err != nil {
		return err
	}
	walletRoot := idoreconciliation.ZeroDigest()
	walletIdentityRoot := idoreconciliation.ZeroDigest()
	walletFailedChecks := uint(0)
	for index, row := range wallets {
		expectedIdentity := idoreconciliation.WalletIdentityDigest(row.Ordinal, row.WalletAddress, row.ExpectedPaymentRaw, row.ExpectedTokensRaw, row.ExpectedClaimedRaw, row.ExpectedRefunded, walletIdentityRoot)
		expectedDigest := idoreconciliation.WalletDigest(row.RunID, row.Ordinal, row.WalletAddress, row.ExpectedPaymentRaw, row.ActualPaymentRaw, row.ExpectedTokensRaw, row.ActualTokensRaw, row.ExpectedClaimedRaw, row.ActualClaimedRaw, row.ExpectedRefunded, row.ActualRefunded, walletRoot)
		if row.Ordinal != uint(index) || row.PreviousRowDigest != walletRoot || row.IdentityDigest != expectedIdentity || row.RowDigest != expectedDigest {
			return errors.New("wallet evidence chain mismatch")
		}
		walletRoot, walletIdentityRoot = row.RowDigest, row.IdentityDigest
		if row.ExpectedPaymentRaw != row.ActualPaymentRaw {
			walletFailedChecks++
		}
		if row.ExpectedTokensRaw != row.ActualTokensRaw {
			walletFailedChecks++
		}
		if row.ExpectedClaimedRaw != row.ActualClaimedRaw {
			walletFailedChecks++
		}
		if row.ExpectedRefunded != row.ActualRefunded {
			walletFailedChecks++
		}
	}
	if uint(len(wallets)) != manifest.WalletCount || walletRoot != manifest.WalletEvidenceDigest {
		return errors.New("wallet evidence root mismatch")
	}
	checks := make([]entity.IDOReconciliationCheck, 0, manifest.CheckCount)
	if err := database.Select("run_id,ordinal,scope,code,ok,expected_exact,actual_exact,message,previous_row_digest,identity_digest,row_digest").Where("run_id = ?", runID).Order("ordinal ASC").Find(&checks).Error; err != nil {
		return err
	}
	checkRoot := idoreconciliation.ZeroDigest()
	checkIdentityRoot := idoreconciliation.ZeroDigest()
	failed := uint(0)
	for index, row := range checks {
		expectedIdentity := idoreconciliation.CheckIdentityDigest(row.Ordinal, row.Scope, row.Code, checkIdentityRoot)
		expectedDigest := idoreconciliation.CheckDigest(row.RunID, row.Ordinal, row.Scope, row.Code, row.OK, row.ExpectedExact, row.ActualExact, row.Message, checkRoot)
		if row.Ordinal != uint(index) || row.PreviousRowDigest != checkRoot || row.IdentityDigest != expectedIdentity || row.RowDigest != expectedDigest {
			return errors.New("check evidence chain mismatch")
		}
		if !row.OK {
			failed++
		}
		checkRoot, checkIdentityRoot = row.RowDigest, row.IdentityDigest
	}
	if uint(len(checks))+manifest.WalletCount*4 != manifest.CheckCount || failed+walletFailedChecks != manifest.FailedCheckCount || checkRoot != manifest.CheckEvidenceDigest {
		return errors.New("check evidence root mismatch")
	}
	visible := append(append([]idoreconciliation.Check(nil), payload.DatabaseChecks...), payload.Onchain.Checks...)
	if len(visible) == 0 || len(visible) != len(checks) {
		return errors.New("manifest visible checks are incomplete")
	}
	for index, check := range visible {
		row := checks[index]
		if row.Code != check.Code || row.OK != check.OK || row.ExpectedExact != reconciliationText(check.Expected, 500) || row.ActualExact != reconciliationText(check.Actual, 500) || row.Message != reconciliationText(check.Message, 500) {
			return errors.New("manifest check projection does not match evidence rows")
		}
	}
	return nil
}

func reconciliationText(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}

func ListProjectIDOReconciliations(c *gin.Context) {
	projectID, ok := adminProjectID(c)
	if !ok {
		return
	}
	if _, err := loadV2FundProject(db.Mysql().WithContext(c.Request.Context()), uint64(projectID)); err != nil {
		fundProjectReadError(c, err)
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	var total int64
	query := db.Mysql().WithContext(c.Request.Context()).Table("ido_reconciliation_run r").Joins("JOIN ido_reconciliation_job j ON j.run_id=r.id").Where("r.project_id = ?", projectID)
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "IDO_RECONCILIATION_LIST_FAILED", "message": "reconciliation history is unavailable"})
		return
	}
	ids := make([]uint64, 0, size)
	if err := query.Select("r.id").Order("r.id DESC").Offset((page-1)*size).Limit(size).Pluck("r.id", &ids).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "IDO_RECONCILIATION_LIST_FAILED", "message": "reconciliation history is unavailable"})
		return
	}
	items := make([]idoReconciliationSummary, 0, len(ids))
	for _, id := range ids {
		item, err := loadIDOReconciliationSummary(db.Mysql().WithContext(c.Request.Context()), id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "IDO_RECONCILIATION_LIST_FAILED", "message": "reconciliation history is unavailable"})
			return
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"items": items, "total": total, "page": page, "page_size": size}})
}

func ListIDOReconciliationWallets(c *gin.Context) {
	runID, ok := reconciliationRunID(c)
	if !ok {
		return
	}
	if err := requireV2ReconciliationRun(db.Mysql().WithContext(c.Request.Context()), runID); err != nil {
		fundProjectReadError(c, err)
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 50
	}
	query := db.Mysql().WithContext(c.Request.Context()).Model(&entity.IDOReconciliationWallet{}).Where("run_id = ?", runID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "IDO_RECONCILIATION_WALLET_READ_FAILED", "message": "wallet evidence is unavailable"})
		return
	}
	rows := make([]entity.IDOReconciliationWallet, 0, size)
	if err := query.Select("id,run_id,ordinal,wallet_address,expected_payment_raw,actual_payment_raw,expected_tokens_raw,actual_tokens_raw,expected_claimed_raw,actual_claimed_raw,expected_refunded,actual_refunded,row_digest,observed_at").Order("ordinal ASC").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "IDO_RECONCILIATION_WALLET_READ_FAILED", "message": "wallet evidence is unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"items": rows, "total": total, "page": page, "page_size": size}})
}

func CancelIDOReconciliation(c *gin.Context) {
	runID, ok := reconciliationRunID(c)
	if !ok {
		return
	}
	if err := requireV2ReconciliationRun(db.Mysql().WithContext(c.Request.Context()), runID); err != nil {
		fundProjectReadError(c, err)
		return
	}
	var request struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "IDO_RECONCILIATION_CANCEL_REASON_REQUIRED", "message": "cancel reason is required"})
		return
	}
	request.Reason = strings.TrimSpace(request.Reason)
	if utf8.RuneCountInString(request.Reason) < 3 || utf8.RuneCountInString(request.Reason) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "IDO_RECONCILIATION_CANCEL_REASON_INVALID", "message": "cancel reason must be 3-500 characters"})
		return
	}
	adminID := currentAdminID(c)
	if adminID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "code": "IDO_RECONCILIATION_ADMIN_REQUIRED", "message": "authenticated administrator is required"})
		return
	}
	state, err := cancelIDOReconciliation(runID, adminID, request.Reason)
	status, code := http.StatusInternalServerError, "IDO_RECONCILIATION_CANCEL_FAILED"
	if errors.Is(err, errAdminProjectExecuteRevoked) {
		status, code = http.StatusForbidden, "IDO_RECONCILIATION_ADMIN_REVOKED"
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		status, code = http.StatusNotFound, "IDO_RECONCILIATION_NOT_FOUND"
	} else if errors.Is(err, errIDOReconciliationNotCancellable) {
		status, code = http.StatusConflict, "IDO_RECONCILIATION_NOT_CANCELLABLE"
	}
	if err != nil {
		publicMessage := "reconciliation task could not be cancelled"
		if errors.Is(err, errAdminProjectExecuteRevoked) {
			publicMessage = "project execute permission is no longer available"
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			publicMessage = "reconciliation run was not found"
		} else if errors.Is(err, errIDOReconciliationNotCancellable) {
			publicMessage = errIDOReconciliationNotCancellable.Error()
		}
		c.JSON(status, gin.H{"status": false, "code": code, "message": publicMessage})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"run_id": runID, "state": state}})
}

func cancelIDOReconciliation(runID, adminID uint64, reason string) (string, error) {
	state := ""
	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := requireV2ReconciliationRun(tx, runID); err != nil {
			return err
		}
		if err := requireCurrentProjectExecuteAdmin(tx, adminID); err != nil {
			return err
		}
		job := entity.IDOReconciliationJob{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("run_id = ?", runID).Take(&job).Error; err != nil {
			return err
		}
		now := time.Now()
		updates := map[string]interface{}{"cancel_requested_by_admin_id": adminID, "cancel_reason": reason, "cancel_requested_at": now, "updated_at": now}
		eventType := "CANCEL_REQUESTED"
		if job.State == "queued" || job.State == "retrying" {
			state = "cancelled"
			eventType = "CANCELLED"
			updates["state"] = "cancelled"
			updates["active_slot"] = nil
			updates["completed_at"] = now
			updates["lease_token"] = ""
			updates["lease_expires_at"] = nil
		} else if job.State == "running" {
			state = "cancel_requested"
			updates["state"] = "cancel_requested"
		} else {
			return errIDOReconciliationNotCancellable
		}
		result := tx.Model(&entity.IDOReconciliationJob{}).Where("run_id = ? AND state = ?", runID, job.State).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errIDOReconciliationNotCancellable
		}
		return appendIDOReconciliationEventTx(tx, runID, eventType, "cancel", "", reason, &adminID, job.LeaseToken)
	})
	return state, err
}

func appendIDOReconciliationEventTx(tx *gorm.DB, runID uint64, eventType, stage, errorCode, message string, actorAdminID *uint64, leaseToken string) error {
	previous := idoreconciliation.ZeroDigest()
	sequence := uint(1)
	last := entity.IDOReconciliationEvent{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("sequence_no,event_hash").Where("run_id = ?", runID).Order("sequence_no DESC").Take(&last).Error
	if err == nil {
		sequence = last.SequenceNo + 1
		previous = last.EventHash
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
	row := entity.IDOReconciliationEvent{RunID: runID, SequenceNo: sequence, EventType: eventType, Stage: stage, ErrorCode: errorCode, Message: message, ActorAdminID: actorAdminID, LeaseTokenHash: leaseHash, PreviousEventHash: previous, EventHash: idoreconciliation.EventDigest(runID, sequence, eventType, stage, errorCode, message, actor, leaseHash, previous), CreatedAt: time.Now()}
	return tx.Create(&row).Error
}

func requireV2ReconciliationRun(database *gorm.DB, runID uint64) error {
	var identity struct {
		FundingMode     string `gorm:"column:funding_mode"`
		ContractVersion string `gorm:"column:contract_version"`
	}
	err := database.Table("ido_reconciliation_run r").Select("p.funding_mode,p.contract_version").Joins("JOIN projects p ON p.project_id = r.project_id").Where("r.id = ?", runID).Take(&identity).Error
	if err != nil {
		return err
	}
	if !constant.IsExactV2ProjectIdentity(identity.FundingMode, identity.ContractVersion) {
		return errors.New("fund ledger is available only for exact V2 stablecoin projects")
	}
	return nil
}

func loadIDOReconciliationSummary(database *gorm.DB, runID uint64) (idoReconciliationSummary, error) {
	result := idoReconciliationSummary{}
	err := database.Transaction(func(tx *gorm.DB) error {
		syncState := entity.ChainEventSyncState{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = 1").Take(&syncState).Error; err != nil {
			return err
		}
		var err error
		result, err = loadIDOReconciliationSummaryTx(tx, runID)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	return result, err
}

func loadIDOReconciliationSummaryTx(database *gorm.DB, runID uint64) (idoReconciliationSummary, error) {
	type row struct {
		entity.IDOReconciliationRun
		State             string
		Outcome           *string
		AttemptCount      uint
		NextWalletOrdinal uint
		LastErrorCode     string
		LastErrorMessage  string
		StartedAt         *time.Time
		CompletedAt       *time.Time
		ManifestDigest    string
	}
	value := row{}
	err := database.Table("ido_reconciliation_run r").Select("r.*,j.state,j.outcome,j.attempt_count,j.next_wallet_ordinal,j.last_error_code,j.last_error_message,j.started_at,j.completed_at,COALESCE(m.manifest_digest,'') manifest_digest").Joins("JOIN ido_reconciliation_job j ON j.run_id=r.id").Joins("LEFT JOIN ido_reconciliation_manifest m ON m.run_id=r.id").Where("r.id = ?", runID).Take(&value).Error
	if err != nil {
		return idoReconciliationSummary{}, err
	}
	var balanced *bool
	var activeReorgs int64
	if err := database.Model(&entity.ChainEventReorgIncident{}).Where("chain_id = ? AND LOWER(contract_address) = ? AND active_slot = 1 AND status = ?", value.ChainID, strings.ToLower(value.ContractAddress), "detected").Count(&activeReorgs).Error; err != nil {
		return idoReconciliationSummary{}, err
	}
	invalidatedByReorg := activeReorgs != 0
	balanced = currentReconciliationBalance(value.Outcome, invalidatedByReorg)
	return idoReconciliationSummary{RunID: value.ID, RunUUID: value.RunUUID, ProjectID: value.ProjectID, State: value.State, Outcome: value.Outcome, Balanced: balanced, EvidenceBlockNumber: value.EvidenceBlockNumber, EvidenceBlockHash: value.EvidenceBlockHash, ExpectedWalletCount: value.ExpectedWalletCount, ProcessedWalletCount: value.NextWalletOrdinal, AttemptCount: value.AttemptCount, LastErrorCode: value.LastErrorCode, LastErrorMessage: safeIDOReconciliationError(value.LastErrorCode), SourceDigest: value.SourceDigest, ManifestDigest: value.ManifestDigest, InvalidatedByReorg: invalidatedByReorg, RequestedByAdminID: value.RequestedByAdminID, Reason: value.Reason, RequestedAt: value.RequestedAt, StartedAt: value.StartedAt, CompletedAt: value.CompletedAt}, nil
}

func currentReconciliationBalance(outcome *string, invalidatedByReorg bool) *bool {
	if outcome == nil || invalidatedByReorg {
		return nil
	}
	value := *outcome == "BALANCED"
	return &value
}

func asyncReconciliationParticipantLimit() int {
	value := viper.GetInt("chain.project.reconciliation.maxParticipants")
	if value <= 0 {
		return defaultAsyncReconciliationParticipants
	}
	if value > hardAsyncReconciliationParticipants {
		return hardAsyncReconciliationParticipants
	}
	return value
}
func currentAdminRoles(c *gin.Context) string {
	if value, ok := c.Get(middleware.CtxAllRoles); ok {
		if roles, ok := value.(string); ok && strings.TrimSpace(roles) != "" {
			return strings.TrimSpace(roles)
		}
	}
	return "unknown"
}
func reconciliationRunID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("runId"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "IDO_RECONCILIATION_BAD_ID", "message": "invalid reconciliation run id"})
		return 0, false
	}
	return id, true
}
func asciiToken(value string) bool {
	for _, char := range value {
		if char < 0x21 || char > 0x7e {
			return false
		}
	}
	return true
}
func isDuplicateKey(err error) bool {
	var mysqlErr *driverMysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

func safeIDOReconciliationError(code string) string {
	switch code {
	case "RPC_UNAVAILABLE":
		return "configured chain RPC is unavailable"
	case "SNAPSHOT_INVALIDATED":
		return "the fixed evidence block or trusted contract identity is no longer valid"
	case "SOURCE_CHANGED":
		return "database source facts changed before finalization"
	case "RPC_TIMEOUT":
		return "fixed-block chain reads timed out"
	case "PARTICIPANT_LIMIT":
		return "project participant count exceeds the configured asynchronous reconciliation limit"
	case "INTERNAL_ERROR":
		return "reconciliation execution failed; inspect restricted job logs"
	default:
		return ""
	}
}

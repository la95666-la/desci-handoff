package controller

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	baseChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/fundledger"
	"joincare_server/pkg/common/idoreconciliation"
	"joincare_server/pkg/core/db"
	chainManager "joincare_server/pkg/core/third/chain"
)

const (
	onchainReconciliationTimeout = 20 * time.Second
	maxOnchainParticipants       = 200
)

type idoSnapshotReader = idoreconciliation.Reader
type onchainWalletSnapshot = idoreconciliation.WalletSnapshot
type onchainTokenReserveSnapshot = idoreconciliation.TokenReserveSnapshot
type onchainProjectSnapshot = idoreconciliation.ProjectSnapshot
type onchainReconciliationResult = idoreconciliation.Result

type onchainDBSnapshot struct {
	project      entity.Project
	accumulator  *fundReconciliationAccumulator
	entitlements []entity.IDOPurchaseEntitlement
	tges         []entity.ProjectTge
	sync         entity.ChainEventSyncState
	dbChecks     []fundReconciliationCheck
	dbBalanced   bool
}

// GetProjectOnchainReconciliation GET /admin/v1/projects/:id/onchain-reconciliation
func GetProjectOnchainReconciliation(c *gin.Context) {
	projectID, ok := adminProjectID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), onchainReconciliationTimeout)
	defer cancel()
	snapshot, err := loadOnchainDBSnapshot(ctx, projectID)
	if err != nil {
		onchainReconciliationError(c, err)
		return
	}
	reader := chainManager.GetClient(constant.ChainProjectV2)
	if reader == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "ONCHAIN_RECONCILIATION_RPC_UNAVAILABLE", "message": "IDO V2 RPC client is not configured"})
		return
	}
	result, err := reconcileProjectOnChain(ctx, reader, snapshot)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "ONCHAIN_RECONCILIATION_FAILED", "message": err.Error()})
		return
	}
	if err := recheckOnchainDBHealth(ctx, snapshot.project, snapshot.sync.CursorBlock); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "code": "ONCHAIN_RECONCILIATION_DB_CHANGED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"scope": "database_ledger_plus_onchain_snapshot", "balanced": snapshot.dbBalanced && result.Balanced,
		"database":         gin.H{"scope": "database_ledger_only", "balanced": snapshot.dbBalanced, "checks": snapshot.dbChecks},
		"onchain_snapshot": result,
	}})
}

func recheckOnchainDBHealth(ctx context.Context, project entity.Project, snapshotCursor uint64) error {
	return db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		state := entity.ChainEventSyncState{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = 1").Take(&state).Error; err != nil {
			return err
		}
		if state.CursorBlock < snapshotCursor {
			return errors.New("chain event cursor moved backwards during on-chain reconciliation")
		}
		if err := validateOnchainOperationalStateTx(tx, project, state, snapshotCursor); err != nil {
			return err
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
}

func loadOnchainDBSnapshot(ctx context.Context, projectID uint64) (onchainDBSnapshot, error) {
	snapshot := onchainDBSnapshot{}
	err := db.Mysql().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		snapshot.project, snapshot.accumulator, snapshot.entitlements, err = loadFundFactsTx(tx, projectID)
		if err != nil {
			return err
		}
		if !snapshot.project.OnchainLocked || snapshot.project.Status == constant.ProjectStatusSet || snapshot.project.Status == constant.ProjectStatusSetup {
			return errors.New("V2 project has not reached an on-chain created state")
		}
		if err := tx.Where("project_id = ?", projectID).Order("`index` ASC").Find(&snapshot.tges).Error; err != nil {
			return err
		}
		if err := tx.Where("id = 1").Take(&snapshot.sync).Error; err != nil {
			return fmt.Errorf("load chain event sync state: %w", err)
		}
		if err := validateOnchainOperationalStateTx(tx, snapshot.project, snapshot.sync, snapshot.sync.CursorBlock); err != nil {
			return err
		}
		_, snapshot.dbChecks, _ = snapshot.accumulator.Result()
		snapshot.dbBalanced = allReconciliationChecksOK(snapshot.dbChecks)
		if !snapshot.dbBalanced {
			return errors.New("database ledger must be balanced before on-chain reconciliation")
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return snapshot, err
}

func validateOnchainOperationalStateTx(tx *gorm.DB, project entity.Project, state entity.ChainEventSyncState, evidenceCursor uint64) error {
	if err := validateOnchainSyncState(project, state); err != nil {
		return err
	}
	var activeReorgs int64
	if err := tx.Model(&entity.ChainEventReorgIncident{}).Where("chain_id = ? AND LOWER(contract_address) = ? AND active_slot = 1 AND status = ?", project.ChainID, strings.ToLower(project.ContractAddress), "detected").Count(&activeReorgs).Error; err != nil {
		return err
	}
	if activeReorgs != 0 {
		return errors.New("active chain event reorg incident prevents on-chain reconciliation")
	}
	var unresolved int64
	if err := tx.Model(&entity.ChainEventFailure{}).Where("status IN ?", []string{constant.ChainEventFailureRetrying, constant.ChainEventFailureDeadLetter}).Count(&unresolved).Error; err != nil {
		return err
	}
	if unresolved != 0 {
		return fmt.Errorf("%d unresolved chain event failure(s) prevent on-chain reconciliation", unresolved)
	}
	var blockedSubmissions int64
	if err := tx.Model(&entity.ChainTxSubmission{}).Where(
		"chain_id = ? AND contract_address = ? AND project_id = ? AND state NOT IN ?",
		project.ChainID, common.HexToAddress(project.ContractAddress).Hex(), project.ProjectID,
		[]string{constant.ChainTxStateApplied, constant.ChainTxStateCancelled, constant.ChainTxStateAdminCancelled},
	).Count(&blockedSubmissions).Error; err != nil {
		return err
	}
	if blockedSubmissions != 0 {
		return fmt.Errorf("%d unapplied chain transaction(s) prevent on-chain reconciliation", blockedSubmissions)
	}
	var futureFacts int64
	if err := tx.Model(&entity.FundLedger{}).Where("project_id = ? AND block_number > ?", project.ProjectID, evidenceCursor).Count(&futureFacts).Error; err != nil {
		return err
	}
	var futureEntitlements int64
	if err := tx.Model(&entity.IDOPurchaseEntitlement{}).Where("project_id = ? AND block_number > ?", project.ProjectID, evidenceCursor).Count(&futureEntitlements).Error; err != nil {
		return err
	}
	if futureFacts != 0 || futureEntitlements != 0 {
		return errors.New("project contains financial facts beyond the reconciliation evidence cursor")
	}
	return nil
}

func validateOnchainSyncState(project entity.Project, state entity.ChainEventSyncState) error {
	maxLag := viper.GetUint64("chainEvent.maxHealthyLagBlocks")
	if maxLag == 0 {
		maxLag = 1000
	}
	if !state.Initialized || !state.V2Registered || state.LastSuccessAt == nil || state.LastError != "" || state.CursorBlock == 0 || state.CursorBlockHash == "" {
		return errors.New("chain event sync state is not healthy enough for on-chain reconciliation")
	}
	if state.UpdatedAt.IsZero() || time.Since(state.UpdatedAt) > 3*time.Minute || state.LagBlocks > maxLag || state.CursorBlock > state.SafeBlock {
		return errors.New("chain event sync state is stale or lagging")
	}
	if state.CursorBlock < state.V2StartBlock || !strings.EqualFold(state.V2ContractAddress, project.ContractAddress) {
		return errors.New("V2 event source identity does not cover the project snapshot")
	}
	configuredProxy := strings.TrimSpace(viper.GetString("chain.project.v2.contractAddress"))
	configuredChainID := viper.GetUint64("chain.project.chainID")
	configuredImplementation := strings.TrimSpace(viper.GetString("chain.project.v2.implementationAddress"))
	configuredCodeHash := strings.TrimSpace(viper.GetString("chain.project.v2.implementationCodeHash"))
	if configuredChainID == 0 || configuredChainID != project.ChainID || !strings.EqualFold(configuredProxy, state.V2ContractAddress) || !strings.EqualFold(configuredImplementation, state.V2ImplementationAddress) || !strings.EqualFold(configuredCodeHash, state.V2ImplementationCodeHash) {
		return errors.New("configured V2 proxy or implementation identity drift")
	}
	return nil
}

func reconcileProjectOnChain(ctx context.Context, reader idoSnapshotReader, dbSnapshot onchainDBSnapshot) (onchainReconciliationResult, error) {
	project := dbSnapshot.project
	confirmations, err := baseChain.ResolveConfirmations(project.ChainID, viper.GetString("chain.project.confirmations"))
	if err != nil {
		return onchainReconciliationResult{}, err
	}
	maxLag := viper.GetUint64("chainEvent.maxHealthyLagBlocks")
	if maxLag == 0 {
		maxLag = 1000
	}
	request := buildOnchainReconciliationRequest(dbSnapshot, confirmations, maxLag, maxOnchainParticipants)
	return idoreconciliation.Reconcile(ctx, reader, request)
}

func buildOnchainReconciliationRequest(snapshot onchainDBSnapshot, confirmations, maxLag uint64, maxParticipants int) idoreconciliation.Request {
	project := snapshot.project
	status, statusOK := projectStatusToV2(project.Status)
	vestingTimes, vestingPercents, vestingOK := storedVestingRaw(snapshot.tges)
	wallets := make([]idoreconciliation.ExpectedWallet, 0, len(snapshot.accumulator.purchaseWallets))
	for _, wallet := range entitlementWalletOrder(snapshot.entitlements) {
		key := strings.ToLower(wallet)
		wallets = append(wallets, idoreconciliation.ExpectedWallet{
			Wallet: strings.ToLower(common.HexToAddress(wallet).Hex()), PaymentAmountRaw: requiredMapUintString(snapshot.accumulator.purchaseByWallet, key),
			TokensBoughtRaw: requiredMapUintString(snapshot.accumulator.tokensByWallet, key), ClaimedTokensRaw: snapshot.accumulator.claimsByWalletValue(key).String(), Refunded: snapshot.accumulator.refundCountByWallet[key] == 1,
		})
	}
	paymentLiability, projectLiability := projectOutstandingLiabilities(snapshot.accumulator)
	return idoreconciliation.Request{
		ChainID: project.ChainID, BlockNumber: snapshot.sync.CursorBlock, BlockHash: snapshot.sync.CursorBlockHash,
		LatestConfirmationDepth: confirmations, MaxHealthyLagBlocks: maxLag, MaxParticipants: maxParticipants,
		ExpectedImplementation: snapshot.sync.V2ImplementationAddress, ExpectedImplementationHash: snapshot.sync.V2ImplementationCodeHash,
		Project: idoreconciliation.ExpectedProject{
			ProjectID: uint64(project.ProjectID), Status: status, StatusValid: statusOK, ContractAddress: project.ContractAddress,
			ProjectToken: project.TokenAddress, PaymentToken: project.PaymentTokenAddress, ProjectOwner: project.OwnerAddress, FeeReceiver: project.FeeReceiver,
			ProjectTokenDecimals: uint8(project.TokenDecimals), PaymentTokenDecimals: uint8(project.PaymentTokenDecimals), SuccessFeeRate: project.SuccessFeeRate,
			TokenPriceRaw: project.TokenPriceRaw, HardCapRaw: project.HardCapRaw, SoftCapRaw: project.SoftCapRaw, MaxPerUserRaw: project.MaxPerUserRaw,
			StartTimeRaw: fmt.Sprint(project.StartTime.Unix()), EndTimeRaw: fmt.Sprint(project.EndTime.Unix()), TotalRaisedRaw: project.TotalRaisedRaw,
			TotalTokensSoldRaw: snapshot.accumulator.tokensSold.String(), TotalTokensDepositedRaw: project.TotalDepositedRaw,
			FundsWithdrawn: project.FundWithdrawn != 0, PlatformFeeWithdrawn: project.PlatformFeeWithdrawn != 0, ProjectTokensWithdrawn: project.TokenWithdrawn != 0,
			VestingTimesRaw: idoreconciliation.BigIntStrings(vestingTimes), VestingPercentsRaw: idoreconciliation.BigIntStrings(vestingPercents), VestingValid: vestingOK,
		},
		Wallets: wallets,
		Reserves: []idoreconciliation.ExpectedReserve{
			{Role: "payment", Symbol: project.PaymentToken, TokenAddress: project.PaymentTokenAddress, TokenDecimals: uint8(project.PaymentTokenDecimals), ProjectLiabilityRaw: paymentLiability.String()},
			{Role: "project", Symbol: project.TokenSymbol, TokenAddress: project.TokenAddress, TokenDecimals: uint8(project.TokenDecimals), ProjectToken: true, ProjectLiabilityRaw: projectLiability.String()},
		},
	}
}

func projectOnchainChecks(project entity.Project, accumulator *fundReconciliationAccumulator, tges []entity.ProjectTge, chainProject baseChain.IDOV2ProjectSnapshot, expectedWallets []string, participants []common.Address) []fundReconciliationCheck {
	wantStatus, statusOK := projectStatusToV2(project.Status)
	wantTimes, wantPercents, vestingOK := storedVestingRaw(tges)
	wallets := make([]idoreconciliation.ExpectedWallet, len(expectedWallets))
	for i, wallet := range expectedWallets {
		wallets[i].Wallet = wallet
	}
	return idoreconciliation.ProjectChecks(idoreconciliation.ExpectedProject{
		ProjectID: uint64(project.ProjectID), Status: wantStatus, StatusValid: statusOK, ProjectToken: project.TokenAddress, PaymentToken: project.PaymentTokenAddress,
		ProjectOwner: project.OwnerAddress, FeeReceiver: project.FeeReceiver, ProjectTokenDecimals: uint8(project.TokenDecimals), PaymentTokenDecimals: uint8(project.PaymentTokenDecimals),
		SuccessFeeRate: project.SuccessFeeRate, TokenPriceRaw: project.TokenPriceRaw, HardCapRaw: project.HardCapRaw, SoftCapRaw: project.SoftCapRaw, MaxPerUserRaw: project.MaxPerUserRaw,
		StartTimeRaw: fmt.Sprint(project.StartTime.Unix()), EndTimeRaw: fmt.Sprint(project.EndTime.Unix()), TotalRaisedRaw: project.TotalRaisedRaw,
		TotalTokensSoldRaw: accumulator.tokensSold.String(), TotalTokensDepositedRaw: project.TotalDepositedRaw, FundsWithdrawn: project.FundWithdrawn != 0,
		PlatformFeeWithdrawn: project.PlatformFeeWithdrawn != 0, ProjectTokensWithdrawn: project.TokenWithdrawn != 0,
		VestingTimesRaw: idoreconciliation.BigIntStrings(wantTimes), VestingPercentsRaw: idoreconciliation.BigIntStrings(wantPercents), VestingValid: vestingOK,
	}, wallets, chainProject, participants)
}

func walletOnchainChecks(wallet string, accumulator *fundReconciliationAccumulator, purchase baseChain.IDOV2UserPurchaseSnapshot) []fundReconciliationCheck {
	key := strings.ToLower(wallet)
	return idoreconciliation.WalletChecks(idoreconciliation.ExpectedWallet{
		Wallet: wallet, PaymentAmountRaw: requiredMapUintString(accumulator.purchaseByWallet, key), TokensBoughtRaw: requiredMapUintString(accumulator.tokensByWallet, key),
		ClaimedTokensRaw: accumulator.claimsByWalletValue(key).String(), Refunded: accumulator.refundCountByWallet[key] == 1,
	}, idoreconciliation.WalletSnapshot{Wallet: common.HexToAddress(wallet).Hex(), PaymentAmountRaw: idoreconciliation.UintString(purchase.PaymentAmount), TokensBoughtRaw: idoreconciliation.UintString(purchase.TokensBought), ClaimedTokensRaw: idoreconciliation.UintString(purchase.ClaimedTokens), Refunded: purchase.Refunded})
}

func requiredMapUintString(values map[string]*big.Int, key string) string {
	value := values[key]
	if value == nil || value.Sign() < 0 {
		return "invalid"
	}
	return value.String()
}

func projectOutstandingLiabilities(accumulator *fundReconciliationAccumulator) (*big.Int, *big.Int) {
	paymentLiability := new(big.Int).Sub(new(big.Int).Sub(new(big.Int).Sub(accumulator.sum(fundledger.EntryPurchasePayment), accumulator.sum(fundledger.EntryPaymentRefunded)), accumulator.sum(fundledger.EntryProjectProceedsPaid)), accumulator.sum(fundledger.EntryPlatformFeePaid))
	projectLiability := new(big.Int).Sub(new(big.Int).Sub(accumulator.sum(fundledger.EntryProjectTokenDeposited), accumulator.sum(fundledger.EntryProjectTokenClaimed)), accumulator.sum(fundledger.EntryProjectTokensWithdrawn))
	return paymentLiability, projectLiability
}

func entitlementWalletOrder(entitlements []entity.IDOPurchaseEntitlement) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, fact := range entitlements {
		wallet := strings.ToLower(common.HexToAddress(fact.WalletAddress).Hex())
		if _, ok := seen[wallet]; ok {
			continue
		}
		seen[wallet] = struct{}{}
		result = append(result, wallet)
	}
	return result
}

func storedVestingRaw(tges []entity.ProjectTge) ([]*big.Int, []*big.Int, bool) {
	times := make([]*big.Int, 0, len(tges))
	percents := make([]*big.Int, 0, len(tges))
	total := new(big.Int)
	for i, tge := range tges {
		basis := tge.VestingPercent.Shift(2)
		if tge.Index != i+1 || !basis.IsInteger() || !basis.IsPositive() {
			return nil, nil, false
		}
		value := basis.BigInt()
		total.Add(total, value)
		times = append(times, big.NewInt(tge.VestingTime.Unix()))
		percents = append(percents, value)
	}
	return times, percents, len(tges) > 0 && total.Cmp(big.NewInt(10_000)) == 0
}

func projectStatusToV2(status string) (uint8, bool) {
	switch status {
	case constant.ProjectStatusPending:
		return 0, true
	case constant.ProjectStatusActive:
		return 1, true
	case constant.ProjectStatusSuccessful:
		return 2, true
	case constant.ProjectStatusFailed:
		return 3, true
	case constant.ProjectStatusCanceled:
		return 4, true
	default:
		return 0, false
	}
}

func (a *fundReconciliationAccumulator) claimsByWalletValue(wallet string) *big.Int {
	if value := a.claimsByWallet[wallet]; value != nil {
		return new(big.Int).Set(value)
	}
	return new(big.Int)
}

func allReconciliationChecksOK(checks []fundReconciliationCheck) bool {
	return idoreconciliation.AllChecksOK(checks)
}

func onchainReconciliationError(c *gin.Context, err error) {
	status, code := http.StatusServiceUnavailable, "ONCHAIN_RECONCILIATION_NOT_READY"
	if errors.Is(err, gorm.ErrRecordNotFound) {
		status, code = http.StatusNotFound, "ONCHAIN_RECONCILIATION_PROJECT_NOT_FOUND"
	} else if strings.Contains(err.Error(), "V2 stablecoin") || strings.Contains(err.Error(), "on-chain created") {
		status, code = http.StatusConflict, "ONCHAIN_RECONCILIATION_PROJECT_NOT_READY"
	}
	c.JSON(status, gin.H{"status": false, "code": code, "message": err.Error()})
}

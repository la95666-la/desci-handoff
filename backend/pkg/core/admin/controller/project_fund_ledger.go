package controller

import (
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/fundledger"
	"joincare_server/pkg/common/idoreconciliation"
	"joincare_server/pkg/common/money"
	"joincare_server/pkg/common/purchaseentitlement"
	"joincare_server/pkg/core/db"
)

const fundLedgerSafeSelect = "id,chain_id,contract_address,contract_version,project_id,wallet_address,counterparty_address,entry_type,asset_role,token_symbol,token_address,token_decimals,amount_raw,amount_display,tx_hash,log_index,block_number,block_hash,status,created_at,updated_at"
const purchaseEntitlementSafeSelect = "id,fund_ledger_id,chain_id,contract_address,contract_version,project_id,wallet_address,payment_token_symbol,payment_token_address,payment_token_decimals,payment_amount_raw,payment_amount_display,project_token_symbol,project_token_address,project_token_decimals,project_token_amount_raw,project_token_amount_display,tx_hash,log_index,block_number,block_hash,status,created_at"

var fundEntryTypes = []string{
	fundledger.EntryProjectTokenDeposited,
	fundledger.EntryPurchasePayment,
	fundledger.EntryPaymentRefunded,
	fundledger.EntryProjectTokenClaimed,
	fundledger.EntryProjectProceedsPaid,
	fundledger.EntryPlatformFeePaid,
	fundledger.EntryProjectTokensWithdrawn,
}

type fundLedgerAggregate struct {
	EntryType     string `json:"entry_type"`
	AssetRole     string `json:"asset_role"`
	TokenSymbol   string `json:"token_symbol"`
	TokenAddress  string `json:"token_address"`
	TokenDecimals uint8  `json:"token_decimals"`
	Count         uint64 `json:"count"`
	AmountRaw     string `json:"amount_raw"`
	AmountDisplay string `json:"amount_display"`
}

type fundReconciliationCheck = idoreconciliation.Check

type purchaseEntitlementSummary struct {
	Count                  uint64 `json:"count"`
	PaymentAmountRaw       string `json:"payment_amount_raw"`
	PaymentAmountDisplay   string `json:"payment_amount_display"`
	TotalTokensSoldRaw     string `json:"total_tokens_sold_raw"`
	TotalTokensSoldDisplay string `json:"total_tokens_sold_display"`
}

type socialIdentity struct {
	TwitterHandle    string `json:"twitter_handle"`
	TwitterName      string `json:"twitter_name"`
	TwitterAvatar    string `json:"twitter_avatar"`
	TelegramUsername string `json:"telegram_username"`
	TelegramName     string `json:"telegram_name"`
	TelegramAvatar   string `json:"telegram_avatar"`
}

type purchaseEntitlementItem struct {
	entity.IDOPurchaseEntitlement
	socialIdentity
}

type fundLedgerItem struct {
	entity.FundLedger
	socialIdentity
}

type walletPurchaseDetail struct {
	WalletAddress string `json:"wallet_address"`
	socialIdentity
	PaymentRaw          string               `json:"payment_raw"`
	PaymentDisplay      string               `json:"payment_display"`
	TokensBoughtRaw     string               `json:"tokens_bought_raw"`
	TokensBoughtDisplay string               `json:"tokens_bought_display"`
	EntitlementCount    int                  `json:"entitlement_count"`
	ProjectStatus       string               `json:"project_status"`
	RefundExpectedNow   bool                 `json:"refund_expected_now"`
	RefundedNow         bool                 `json:"refunded_now"`
	RefundedNowRaw      string               `json:"refunded_now_raw"`
	RefundedNowDisplay  string               `json:"refunded_now_display"`
	Ledger              []entity.FundLedger  `json:"ledger"`
	OnchainRunID        uint64               `json:"onchain_run_id,omitempty"`
	Onchain             *onchainObservedView `json:"onchain,omitempty"`
}

// onchainObservedView 审计快照的展示视图：附带除以小数位后的可读金额
type onchainObservedView struct {
	entity.IDOReconciliationWallet
	ExpectedPaymentDisplay string `json:"expected_payment_display"`
	ActualPaymentDisplay   string `json:"actual_payment_display"`
	ExpectedTokensDisplay  string `json:"expected_tokens_display"`
	ActualTokensDisplay    string `json:"actual_tokens_display"`
	ExpectedClaimedDisplay string `json:"expected_claimed_display"`
	ActualClaimedDisplay   string `json:"actual_claimed_display"`
}

func socialByIdentity(database *gorm.DB, addresses []string) map[string]entity.User {
	social := map[string]entity.User{}
	if len(addresses) == 0 {
		return social
	}
	users := make([]entity.User, 0, len(addresses))
	if err := database.Select("wallet_address,twitter_handle,twitter_display_name,twitter_avatar,telegram_username,telegram_first_name,telegram_last_name,telegram_avatar").
		Model(&entity.User{}).
		Where("LOWER(wallet_address) IN ?", addresses).
		Find(&users).Error; err == nil {
		for _, u := range users {
			social[strings.ToLower(u.WalletAddress)] = u
		}
	}
	return social
}

func socialFromUser(u entity.User) socialIdentity {
	tgName := strings.TrimSpace(strings.TrimSpace(u.TelegramFirstName) + " " + strings.TrimSpace(u.TelegramLastName))
	return socialIdentity{
		TwitterHandle:    u.TwitterHandle,
		TwitterName:      u.TwitterDisplayName,
		TwitterAvatar:    u.TwitterAvatar,
		TelegramUsername: u.TelegramUsername,
		TelegramName:     tgName,
		TelegramAvatar:   u.TelegramAvatar,
	}
}

type fundProjectSnapshot struct {
	ProjectID             int    `json:"project_id"`
	NameEn                string `json:"name_en"`
	Status                string `json:"status"`
	ChainID               uint64 `json:"chain_id"`
	ContractAddress       string `json:"contract_address"`
	ContractVersion       string `json:"contract_version"`
	PaymentToken          string `json:"payment_token"`
	PaymentTokenAddress   string `json:"payment_token_address"`
	PaymentTokenDecimals  int    `json:"payment_token_decimals"`
	ProjectToken          string `json:"project_token"`
	ProjectTokenAddress   string `json:"project_token_address"`
	ProjectTokenDecimals  int    `json:"project_token_decimals"`
	TokenPriceRaw         string `json:"token_price_raw"`
	TokenPriceDisplay     string `json:"token_price_display"`
	TotalRaisedRaw        string `json:"total_raised_raw"`
	TotalRaisedDisplay    string `json:"total_raised_display"`
	TotalDepositedRaw     string `json:"total_deposited_raw"`
	TotalDepositedDisplay string `json:"total_deposited_display"`
	ParticipantCount      uint32 `json:"participant_count"`
	FeeReceiver           string `json:"fee_receiver"`
	SuccessFeeRate        uint16 `json:"success_fee_rate"`
	FundWithdrawn         bool   `json:"fund_withdrawn"`
	PlatformFeeWithdrawn  bool   `json:"platform_fee_withdrawn"`
	TokenWithdrawn        bool   `json:"token_withdrawn"`
}

type fundReconciliationAccumulator struct {
	project             entity.Project
	sums                map[string]*big.Int
	counts              map[string]uint64
	purchaseWallets     map[string]struct{}
	purchaseByWallet    map[string]*big.Int
	refundByWallet      map[string]*big.Int
	refundCountByWallet map[string]uint64
	claimWallets        map[string]struct{}
	claimsByWallet      map[string]*big.Int
	purchasesByID       map[uint64]entity.FundLedger
	entitlementsByID    map[uint64]entity.IDOPurchaseEntitlement
	tokensByWallet      map[string]*big.Int
	entitlementPay      *big.Int
	tokensSold          *big.Int
	entitlementCount    uint64
	entitlementErrors   uint64
	identityErrors      uint64
	amountErrors        uint64
	semanticErrors      uint64
	noncanonicalRows    uint64
}

// ListProjectFundLedger GET /admin/v1/projects/:id/fund-ledger
func ListProjectFundLedger(c *gin.Context) {
	projectID, ok := adminProjectID(c)
	if !ok {
		return
	}
	database := db.Mysql().WithContext(c.Request.Context())
	if _, err := loadV2FundProject(database, projectID); err != nil {
		fundProjectReadError(c, err)
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	// Ordinary V2 history output excludes facts belonging to another contract
	// version. Reconciliation loaders below intentionally retain every version
	// so identity drift remains observable as corruption.
	query := database.Model(&entity.FundLedger{}).Where("project_id = ? AND contract_version = ?", projectID, constant.ContractVersionV2Stablecoin)
	if entryType := strings.TrimSpace(c.Query("entry_type")); entryType != "" {
		if !validFundEntryType(entryType) {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "FUND_LEDGER_BAD_ENTRY_TYPE", "message": "invalid fund ledger entry type"})
			return
		}
		query = query.Where("entry_type = ?", entryType)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		if status != fundledger.StatusCanonical && status != "reversed" && status != "reconciliation" {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "FUND_LEDGER_BAD_STATUS", "message": "invalid fund ledger status"})
			return
		}
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "FUND_LEDGER_LIST_FAILED", "message": err.Error()})
		return
	}
	rows := make([]entity.FundLedger, 0, pageSize)
	if err := query.Select(fundLedgerSafeSelect).Order("block_number DESC, log_index DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "FUND_LEDGER_LIST_FAILED", "message": err.Error()})
		return
	}
	addresses := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.WalletAddress != "" {
			addresses = append(addresses, strings.ToLower(r.WalletAddress))
		}
	}
	social := socialByIdentity(database, addresses)
	items := make([]fundLedgerItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, fundLedgerItem{
			FundLedger:     r,
			socialIdentity: socialFromUser(social[strings.ToLower(r.WalletAddress)]),
		})
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"items": items, "total": total, "page": page, "page_size": pageSize}})
}

// ListProjectPurchaseEntitlements GET /admin/v1/projects/:id/purchase-entitlements
func ListProjectPurchaseEntitlements(c *gin.Context) {
	projectID, ok := adminProjectID(c)
	if !ok {
		return
	}
	database := db.Mysql().WithContext(c.Request.Context())
	if _, err := loadV2FundProject(database, projectID); err != nil {
		fundProjectReadError(c, err)
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	query := database.Model(&entity.IDOPurchaseEntitlement{}).
		Where("project_id = ? AND contract_version = ?", projectID, constant.ContractVersionV2Stablecoin)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "PURCHASE_ENTITLEMENT_LIST_FAILED", "message": err.Error()})
		return
	}
	rows := make([]entity.IDOPurchaseEntitlement, 0, pageSize)
	if err := query.Select(purchaseEntitlementSafeSelect).Order("block_number DESC, log_index DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "PURCHASE_ENTITLEMENT_LIST_FAILED", "message": err.Error()})
		return
	}
	addresses := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.WalletAddress != "" {
			addresses = append(addresses, strings.ToLower(r.WalletAddress))
		}
	}
	social := socialByIdentity(database, addresses)
	items := make([]purchaseEntitlementItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, purchaseEntitlementItem{
			IDOPurchaseEntitlement: r,
			socialIdentity:         socialFromUser(social[strings.ToLower(r.WalletAddress)]),
		})
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"items": items, "total": total, "page": page, "page_size": pageSize}})
}

// GetWalletPurchaseDetail GET /admin/v1/projects/:id/wallet-purchase?wallet=0x...
func GetWalletPurchaseDetail(c *gin.Context) {
	projectID, ok := adminProjectID(c)
	if !ok {
		return
	}
	database := db.Mysql().WithContext(c.Request.Context())
	project, err := loadV2FundProject(database, projectID)
	if err != nil {
		fundProjectReadError(c, err)
		return
	}
	wallet := strings.TrimSpace(c.Query("wallet"))
	if wallet == "" || !common.IsHexAddress(wallet) {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "WALLET_PURCHASE_BAD_ADDRESS", "message": "invalid wallet address"})
		return
	}
	wallet = strings.ToLower(wallet)

	detail := walletPurchaseDetail{
		WalletAddress:  wallet,
		socialIdentity: socialFromUser(socialByIdentity(database, []string{wallet})[wallet]),
	}

	// 权益汇总只使用 canonical V2 权益；历史或异版本事实由 reconciliation
	// 路径读取并报告，不能进入普通钱包金额合计。
	entitlements := make([]entity.IDOPurchaseEntitlement, 0)
	if err := database.Model(&entity.IDOPurchaseEntitlement{}).
		Where("project_id = ? AND LOWER(wallet_address) = ? AND contract_version = ? AND status = ?", projectID, wallet, constant.ContractVersionV2Stablecoin, fundledger.StatusCanonical).
		Find(&entitlements).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "WALLET_PURCHASE_FAILED", "message": err.Error()})
		return
	}
	paymentRaw := new(big.Int)
	tokensRaw := new(big.Int)
	for _, e := range entitlements {
		if p, ok := new(big.Int).SetString(e.PaymentAmountRaw, 10); ok {
			paymentRaw.Add(paymentRaw, p)
		}
		if t, ok := new(big.Int).SetString(e.ProjectTokenAmountRaw, 10); ok {
			tokensRaw.Add(tokensRaw, t)
		}
	}
	detail.EntitlementCount = len(entitlements)
	detail.PaymentRaw = paymentRaw.String()
	detail.TokensBoughtRaw = tokensRaw.String()
	detail.PaymentDisplay = formatAggregateRaw(paymentRaw, uint8(project.PaymentTokenDecimals))
	detail.TokensBoughtDisplay = formatAggregateRaw(tokensRaw, uint8(project.TokenDecimals))

	// 该钱包的 V2 全状态流水历史（canonical/reversed/reconciliation）。
	// 保留全状态用于明确的历史展示，但排除其他 contract version。
	ledger := make([]entity.FundLedger, 0)
	if err := database.Model(&entity.FundLedger{}).Select(fundLedgerSafeSelect).
		Where("project_id = ? AND LOWER(wallet_address) = ? AND contract_version = ?", projectID, wallet, constant.ContractVersionV2Stablecoin).
		Order("block_number DESC, log_index DESC, id DESC").Find(&ledger).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "WALLET_PURCHASE_FAILED", "message": err.Error()})
		return
	}
	detail.Ledger = ledger

	// 实时退款状态：项目终态且未退款则预期应退；账本 payment_refunded 即链上 Refunded 事件落账
	detail.ProjectStatus = project.Status
	refundedNow := new(big.Int)
	for _, row := range ledger {
		if row.EntryType != fundledger.EntryPaymentRefunded || row.Status != fundledger.StatusCanonical {
			continue
		}
		if v, ok := new(big.Int).SetString(row.AmountRaw, 10); ok {
			refundedNow.Add(refundedNow, v)
		}
	}
	detail.RefundedNowRaw = refundedNow.String()
	detail.RefundedNowDisplay = formatAggregateRaw(refundedNow, uint8(project.PaymentTokenDecimals))
	detail.RefundedNow = refundedNow.Sign() > 0
	terminal := project.Status == constant.ProjectStatusFailed || project.Status == constant.ProjectStatusCanceled
	detail.RefundExpectedNow = terminal && paymentRaw.Sign() > 0

	// 最新审计任务的该钱包链上固定区块观测
	var run entity.IDOReconciliationRun
	if err := database.Model(&entity.IDOReconciliationRun{}).Where("project_id = ?", projectID).Order("id DESC").Limit(1).Find(&run).Error; err == nil && run.ID != 0 {
		detail.OnchainRunID = run.ID
		var obs entity.IDOReconciliationWallet
		if err := database.Model(&entity.IDOReconciliationWallet{}).Where("run_id = ? AND LOWER(wallet_address) = ?", run.ID, wallet).Limit(1).Find(&obs).Error; err == nil && obs.ID != 0 {
			view := onchainObservedView{IDOReconciliationWallet: obs}
			view.ExpectedPaymentDisplay = formatRawString(obs.ExpectedPaymentRaw, uint8(project.PaymentTokenDecimals))
			view.ActualPaymentDisplay = formatRawString(obs.ActualPaymentRaw, uint8(project.PaymentTokenDecimals))
			view.ExpectedTokensDisplay = formatRawString(obs.ExpectedTokensRaw, uint8(project.TokenDecimals))
			view.ActualTokensDisplay = formatRawString(obs.ActualTokensRaw, uint8(project.TokenDecimals))
			view.ExpectedClaimedDisplay = formatRawString(obs.ExpectedClaimedRaw, uint8(project.TokenDecimals))
			view.ActualClaimedDisplay = formatRawString(obs.ActualClaimedRaw, uint8(project.TokenDecimals))
			detail.Onchain = &view
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": true, "data": detail})
}

// GetProjectFundReconciliation GET /admin/v1/projects/:id/fund-reconciliation
func GetProjectFundReconciliation(c *gin.Context) {
	projectID, ok := adminProjectID(c)
	if !ok {
		return
	}
	database := db.Mysql().WithContext(c.Request.Context())
	project := entity.Project{}
	var aggregates []fundLedgerAggregate
	var checks []fundReconciliationCheck
	var entitlementSummary purchaseEntitlementSummary
	var activeReorgs int64
	err := database.Transaction(func(tx *gorm.DB) error {
		syncState := entity.ChainEventSyncState{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = 1").Take(&syncState).Error; err != nil {
			return err
		}
		var accumulator *fundReconciliationAccumulator
		var err error
		project, accumulator, _, err = loadFundFactsTx(tx, projectID)
		if err != nil {
			return err
		}
		aggregates, checks, entitlementSummary = accumulator.Result()
		if err := tx.Model(&entity.ChainEventReorgIncident{}).Where("chain_id = ? AND LOWER(contract_address) = ? AND active_slot = 1 AND status = ?", project.ChainID, strings.ToLower(project.ContractAddress), "detected").Count(&activeReorgs).Error; err != nil {
			return err
		}
		checks = append(checks, boolCheck("chain_reorg_clear", activeReorgs == 0, fmt.Sprintf("%d active chain reorg incident(s) require audited compensation", activeReorgs)))
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "V2 stablecoin") {
			fundProjectReadError(c, err)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"status": false, "code": "FUND_RECONCILIATION_FAILED", "message": err.Error()})
		}
		return
	}
	balanced := true
	for _, check := range checks {
		if !check.OK {
			balanced = false
			break
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{
		"scope": "database_ledger_only", "evidence": "project_aggregate+fund_ledger+purchase_entitlement", "balanced": balanced,
		"project": projectFundSnapshot(project), "aggregates": aggregates, "purchase_entitlements": entitlementSummary, "checks": checks,
	}})
}

// loadFundFactsTx is an audit/reconciliation boundary. It deliberately loads
// all child versions and statuses so mixed-version and noncanonical corruption
// is observed and reported instead of disappearing behind ordinary-read filters.
func loadFundFactsTx(tx *gorm.DB, projectID uint64) (entity.Project, *fundReconciliationAccumulator, []entity.IDOPurchaseEntitlement, error) {
	project, err := loadV2FundProject(tx, projectID)
	if err != nil {
		return project, nil, nil, err
	}
	accumulator := newFundReconciliationAccumulator(project)
	rows, err := tx.Model(&entity.FundLedger{}).Select(fundLedgerSafeSelect).Where("project_id = ?", projectID).Order("id ASC").Rows()
	if err != nil {
		return project, nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		fact := entity.FundLedger{}
		if err := tx.ScanRows(rows, &fact); err != nil {
			return project, nil, nil, err
		}
		accumulator.Add(fact)
	}
	if err := rows.Err(); err != nil {
		return project, nil, nil, err
	}
	entitlementRows, err := tx.Model(&entity.IDOPurchaseEntitlement{}).Select(purchaseEntitlementSafeSelect).Where("project_id = ?", projectID).Order("id ASC").Rows()
	if err != nil {
		return project, nil, nil, err
	}
	defer entitlementRows.Close()
	entitlements := make([]entity.IDOPurchaseEntitlement, 0)
	seenWallets := make(map[string]struct{})
	for entitlementRows.Next() {
		fact := entity.IDOPurchaseEntitlement{}
		if err := tx.ScanRows(entitlementRows, &fact); err != nil {
			return project, nil, nil, err
		}
		accumulator.AddEntitlement(fact)
		wallet := strings.ToLower(common.HexToAddress(fact.WalletAddress).Hex())
		if _, exists := seenWallets[wallet]; !exists {
			seenWallets[wallet] = struct{}{}
			entitlements = append(entitlements, fact)
		}
	}
	if err := entitlementRows.Err(); err != nil {
		return project, nil, nil, err
	}
	return project, accumulator, entitlements, nil
}

func adminProjectID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "code": "FUND_LEDGER_BAD_PROJECT_ID", "message": "invalid project id"})
		return 0, false
	}
	return id, true
}

func loadV2FundProject(database *gorm.DB, projectID uint64) (entity.Project, error) {
	project := entity.Project{}
	err := database.Select("project_id,name_en,status,owner_address,chain_id,contract_address,contract_version,funding_mode,onchain_locked,payment_token,payment_token_address,payment_token_decimals,token_symbol,token_address,token_decimals,token_price_raw,hard_cap_raw,soft_cap_raw,max_per_user_raw,start_time,end_time,total_raised,total_raised_raw,total_deposited,total_deposited_raw,participant_count,fee_receiver,success_fee_rate,fund_withdrawn,platform_fee_withdrawn,token_withdrawn").Where("project_id = ?", projectID).Take(&project).Error
	if err != nil {
		return project, err
	}
	if !constant.IsExactV2ProjectIdentity(project.FundingMode, project.ContractVersion) {
		return project, errors.New("fund ledger is available only for exact V2 stablecoin projects")
	}
	return project, nil
}

func fundProjectReadError(c *gin.Context, err error) {
	status, code := http.StatusConflict, "FUND_LEDGER_NOT_V2"
	if errors.Is(err, gorm.ErrRecordNotFound) {
		status, code = http.StatusNotFound, "FUND_LEDGER_PROJECT_NOT_FOUND"
	} else if !strings.Contains(err.Error(), "V2 stablecoin") {
		status, code = http.StatusInternalServerError, "FUND_LEDGER_PROJECT_FAILED"
	}
	c.JSON(status, gin.H{"status": false, "code": code, "message": err.Error()})
}

func validFundEntryType(value string) bool {
	for _, entryType := range fundEntryTypes {
		if value == entryType {
			return true
		}
	}
	return false
}

func newFundReconciliationAccumulator(project entity.Project) *fundReconciliationAccumulator {
	return &fundReconciliationAccumulator{
		project: project, sums: make(map[string]*big.Int, len(fundEntryTypes)), counts: make(map[string]uint64, len(fundEntryTypes)),
		purchaseWallets: make(map[string]struct{}), purchaseByWallet: make(map[string]*big.Int), refundByWallet: make(map[string]*big.Int), refundCountByWallet: make(map[string]uint64),
		claimWallets: make(map[string]struct{}), claimsByWallet: make(map[string]*big.Int), purchasesByID: make(map[uint64]entity.FundLedger),
		entitlementsByID: make(map[uint64]entity.IDOPurchaseEntitlement), tokensByWallet: make(map[string]*big.Int), entitlementPay: new(big.Int), tokensSold: new(big.Int),
	}
}

func (a *fundReconciliationAccumulator) Add(fact entity.FundLedger) {
	if fact.Status != fundledger.StatusCanonical {
		a.noncanonicalRows++
		return
	}
	if !validFundEntryType(fact.EntryType) {
		a.identityErrors++
		return
	}
	if !validLedgerEventIdentity(fact) {
		a.identityErrors++
		return
	}
	canonical, err := money.NormalizeRawUint256(fact.AmountRaw)
	if err != nil || canonical != fact.AmountRaw {
		a.amountErrors++
		return
	}
	display, err := money.FormatRaw(canonical, fact.TokenDecimals)
	if err != nil || display != fact.AmountDisplay {
		a.amountErrors++
	}
	identityValid := a.validIdentity(fact)
	if !identityValid {
		a.identityErrors++
	}
	amount, _ := new(big.Int).SetString(canonical, 10)
	if amount.Sign() == 0 && (fact.EntryType == fundledger.EntryProjectTokenDeposited || fact.EntryType == fundledger.EntryPurchasePayment || fact.EntryType == fundledger.EntryPaymentRefunded || fact.EntryType == fundledger.EntryProjectTokenClaimed) {
		a.semanticErrors++
	}
	if a.sums[fact.EntryType] == nil {
		a.sums[fact.EntryType] = new(big.Int)
	}
	a.sums[fact.EntryType].Add(a.sums[fact.EntryType], amount)
	a.counts[fact.EntryType]++
	if identityValid {
		wallet := strings.ToLower(fact.WalletAddress)
		switch fact.EntryType {
		case fundledger.EntryPurchasePayment:
			a.purchaseWallets[wallet] = struct{}{}
			addWalletAmount(a.purchaseByWallet, wallet, amount)
			if fact.ID == 0 {
				a.identityErrors++
			} else if _, exists := a.purchasesByID[fact.ID]; exists {
				a.identityErrors++
			} else {
				a.purchasesByID[fact.ID] = fact
			}
		case fundledger.EntryPaymentRefunded:
			addWalletAmount(a.refundByWallet, wallet, amount)
			a.refundCountByWallet[wallet]++
		case fundledger.EntryProjectTokenClaimed:
			a.claimWallets[wallet] = struct{}{}
			addWalletAmount(a.claimsByWallet, wallet, amount)
		}
	}
}

func (a *fundReconciliationAccumulator) AddEntitlement(fact entity.IDOPurchaseEntitlement) {
	a.entitlementCount++
	paymentRaw, paymentOK := canonicalPositiveRaw(fact.PaymentAmountRaw)
	projectRaw, projectOK := canonicalPositiveRaw(fact.ProjectTokenAmountRaw)
	identityOK := fact.ID > 0 && fact.FundLedgerID > 0 && fact.Status == fundledger.StatusCanonical &&
		fact.BlockNumber > 0 && validEVMHash(fact.TxHash) && validEVMHash(fact.BlockHash) &&
		fact.ProjectID == uint64(a.project.ProjectID) && fact.ChainID == a.project.ChainID &&
		fact.ContractVersion == constant.ContractVersionV2Stablecoin && strings.EqualFold(fact.ContractAddress, a.project.ContractAddress) &&
		common.IsHexAddress(fact.WalletAddress) && common.HexToAddress(fact.WalletAddress) != (common.Address{}) &&
		fact.PaymentTokenSymbol == a.project.PaymentToken && strings.EqualFold(fact.PaymentTokenAddress, a.project.PaymentTokenAddress) && int(fact.PaymentTokenDecimals) == a.project.PaymentTokenDecimals &&
		fact.ProjectTokenSymbol == a.project.TokenSymbol && strings.EqualFold(fact.ProjectTokenAddress, a.project.TokenAddress) && int(fact.ProjectTokenDecimals) == a.project.TokenDecimals
	paymentDisplay, paymentDisplayErr := money.FormatRaw(fact.PaymentAmountRaw, fact.PaymentTokenDecimals)
	projectDisplay, projectDisplayErr := money.FormatRaw(fact.ProjectTokenAmountRaw, fact.ProjectTokenDecimals)
	formulaOK := purchaseentitlement.MatchesPurchaseFormula(fact.PaymentAmountRaw, fact.ProjectTokenAmountRaw, a.project.TokenPriceRaw, uint8(a.project.TokenDecimals))
	if !identityOK || !paymentOK || !projectOK || !formulaOK || paymentDisplayErr != nil || projectDisplayErr != nil || paymentDisplay != fact.PaymentAmountDisplay || projectDisplay != fact.ProjectTokenDisplay {
		a.entitlementErrors++
		return
	}
	if _, exists := a.entitlementsByID[fact.FundLedgerID]; exists {
		a.entitlementErrors++
		return
	}
	parent, exists := a.purchasesByID[fact.FundLedgerID]
	if !exists || !purchaseEntitlementMatches(parent, fact) {
		a.entitlementErrors++
		return
	}
	a.entitlementsByID[fact.FundLedgerID] = fact
	a.entitlementPay.Add(a.entitlementPay, paymentRaw)
	a.tokensSold.Add(a.tokensSold, projectRaw)
	addWalletAmount(a.tokensByWallet, strings.ToLower(fact.WalletAddress), projectRaw)
}

func (a *fundReconciliationAccumulator) validIdentity(fact entity.FundLedger) bool {
	if fact.ProjectID != uint64(a.project.ProjectID) || fact.ChainID != a.project.ChainID || fact.ContractVersion != constant.ContractVersionV2Stablecoin || !strings.EqualFold(fact.ContractAddress, a.project.ContractAddress) {
		return false
	}
	expectedRole := fundledger.AssetRolePayment
	expectedSymbol, expectedAddress, expectedDecimals := a.project.PaymentToken, a.project.PaymentTokenAddress, a.project.PaymentTokenDecimals
	if fact.EntryType == fundledger.EntryProjectTokenDeposited || fact.EntryType == fundledger.EntryProjectTokenClaimed || fact.EntryType == fundledger.EntryProjectTokensWithdrawn {
		expectedRole = fundledger.AssetRoleProject
		expectedSymbol, expectedAddress, expectedDecimals = a.project.TokenSymbol, a.project.TokenAddress, a.project.TokenDecimals
	}
	return fact.AssetRole == expectedRole && fact.TokenSymbol == expectedSymbol && strings.EqualFold(fact.TokenAddress, expectedAddress) && int(fact.TokenDecimals) == expectedDecimals && a.validParticipants(fact)
}

func (a *fundReconciliationAccumulator) validParticipants(fact entity.FundLedger) bool {
	validWallet := func(value string) bool {
		return common.IsHexAddress(value) && common.HexToAddress(value) != (common.Address{})
	}
	switch fact.EntryType {
	case fundledger.EntryPurchasePayment, fundledger.EntryPaymentRefunded, fundledger.EntryProjectTokenClaimed:
		return validWallet(fact.WalletAddress) && strings.EqualFold(fact.CounterpartyAddress, a.project.ContractAddress)
	case fundledger.EntryProjectTokenDeposited, fundledger.EntryProjectProceedsPaid, fundledger.EntryProjectTokensWithdrawn:
		return strings.EqualFold(fact.WalletAddress, a.project.OwnerAddress) && strings.EqualFold(fact.CounterpartyAddress, a.project.ContractAddress)
	case fundledger.EntryPlatformFeePaid:
		return strings.EqualFold(fact.WalletAddress, a.project.FeeReceiver) && strings.EqualFold(fact.CounterpartyAddress, a.project.ContractAddress)
	default:
		return false
	}
}

func (a *fundReconciliationAccumulator) Result() ([]fundLedgerAggregate, []fundReconciliationCheck, purchaseEntitlementSummary) {
	aggregates := make([]fundLedgerAggregate, 0, len(fundEntryTypes))
	for _, entryType := range fundEntryTypes {
		raw := a.sum(entryType).String()
		role := fundledger.AssetRolePayment
		symbol, address, decimals := a.project.PaymentToken, a.project.PaymentTokenAddress, uint8(a.project.PaymentTokenDecimals)
		if entryType == fundledger.EntryProjectTokenDeposited || entryType == fundledger.EntryProjectTokenClaimed || entryType == fundledger.EntryProjectTokensWithdrawn {
			role = fundledger.AssetRoleProject
			symbol, address, decimals = a.project.TokenSymbol, a.project.TokenAddress, uint8(a.project.TokenDecimals)
		}
		display := formatAggregateRaw(a.sum(entryType), decimals)
		aggregates = append(aggregates, fundLedgerAggregate{EntryType: entryType, AssetRole: role, TokenSymbol: symbol, TokenAddress: address, TokenDecimals: decimals, Count: a.counts[entryType], AmountRaw: raw, AmountDisplay: display})
	}
	purchases := a.sum(fundledger.EntryPurchasePayment)
	refunds := a.sum(fundledger.EntryPaymentRefunded)
	proceeds := a.sum(fundledger.EntryProjectProceedsPaid)
	fees := a.sum(fundledger.EntryPlatformFeePaid)
	deposits := a.sum(fundledger.EntryProjectTokenDeposited)
	claims := a.sum(fundledger.EntryProjectTokenClaimed)
	tokenWithdrawals := a.sum(fundledger.EntryProjectTokensWithdrawn)
	withdrawalEventPresent := a.counts[fundledger.EntryProjectTokensWithdrawn] == 1
	entitlementComplete := a.entitlementErrors == 0 && a.entitlementCount == a.counts[fundledger.EntryPurchasePayment] && uint64(len(a.entitlementsByID)) == a.counts[fundledger.EntryPurchasePayment]
	soldWithinDeposit := a.tokensSold.Cmp(deposits) <= 0
	expectedFee, expectedProceeds := expectedSettlementRaw(a.project.TotalRaisedRaw, a.project.SuccessFeeRate)
	raisedDisplay, raisedDisplayOK := projectRawDisplayMatches(a.project.TotalRaisedRaw, uint8(a.project.PaymentTokenDecimals), a.project.TotalRaised.String())
	depositedDisplay, depositedDisplayOK := projectRawDisplayMatches(a.project.TotalDepositedRaw, uint8(a.project.TokenDecimals), a.project.TotalDeposited.String())
	checks := []fundReconciliationCheck{
		boolCheck("project_identity", validFundProjectIdentity(a.project), "项目链/代理/代币/精度/费率身份必须有效"),
		valueEqualityCheckWithOK("project_raised_display", raisedDisplay, a.project.TotalRaised.String(), raisedDisplayOK, "项目 total_raised 原始值/显示值必须匹配支付币精度"),
		valueEqualityCheckWithOK("project_deposited_display", depositedDisplay, a.project.TotalDeposited.String(), depositedDisplayOK, "项目 total_deposited 原始值/显示值必须匹配项目币精度"),
		rawEqualityCheck("purchase_total", a.project.TotalRaisedRaw, purchases.String(), "认购账本必须等于项目 total_raised_raw"),
		rawEqualityCheck("deposit_total", a.project.TotalDepositedRaw, deposits.String(), "存入账本必须等于项目 total_deposited_raw"),
		boolCheck("ledger_identity", a.identityErrors == 0, fmt.Sprintf("%d 条正式账本存在项目/代币身份不一致", a.identityErrors)),
		boolCheck("ledger_amount_encoding", a.amountErrors == 0, fmt.Sprintf("%d 条正式账本存在原始值/显示值编码无效", a.amountErrors)),
		boolCheck("ledger_amount_semantics", a.semanticErrors == 0, fmt.Sprintf("%d 条正式账本存在不可能的零金额", a.semanticErrors)),
		boolCheck("canonical_only", a.noncanonicalRows == 0, fmt.Sprintf("%d 条非正式/对账账本需要人工处理", a.noncanonicalRows)),
		boolCheck("participant_count", uint64(a.project.ParticipantCount) == uint64(len(a.purchaseWallets)), fmt.Sprintf("项目 participant_count=%d，去重认购钱包数=%d", a.project.ParticipantCount, len(a.purchaseWallets))),
		boolCheck("purchase_entitlement_integrity", entitlementComplete, fmt.Sprintf("认购事件=%d，权益记录=%d，匹配记录=%d，无效记录=%d", a.counts[fundledger.EntryPurchasePayment], a.entitlementCount, len(a.entitlementsByID), a.entitlementErrors)),
		rawEqualityCheck("purchase_entitlement_payment_total", purchases.String(), a.entitlementPay.String(), "权益支付总额必须等于认购支付账本总额"),
		boolCheck("tokens_sold_within_deposit", soldWithinDeposit, "衍生 totalTokensSold 不得超过项目币存入"),
		boolCheck("refunds_within_purchases", refunds.Cmp(purchases) <= 0, "支付退款不得超过认购"),
		boolCheck("wallet_refunds_match_purchases", a.walletRefundsMatchPurchases(), "每个被退款钱包必须有且仅有一条全额退款事件"),
		boolCheck("terminal_refund_completion", validTerminalRefundCompletion(a.project.Status, purchases, refunds), "失败/取消项目必须在账本平衡前退还全部认购款"),
		boolCheck("payment_outflows_within_purchases", new(big.Int).Add(new(big.Int).Add(new(big.Int).Set(refunds), proceeds), fees).Cmp(purchases) <= 0, "退款加结算流出不得超过认购"),
		boolCheck("project_token_outflows_within_deposit", new(big.Int).Add(new(big.Int).Set(claims), tokenWithdrawals).Cmp(deposits) <= 0, "Claim 加项目币退出不得超过存入"),
		boolCheck("project_proceeds_flag", (a.project.FundWithdrawn != 0) == (a.counts[fundledger.EntryProjectProceedsPaid] == 1) && a.counts[fundledger.EntryProjectProceedsPaid] <= 1, "项目结算标志必须匹配恰好一条账本事件"),
		flaggedRawCheck("project_proceeds", a.project.FundWithdrawn != 0, expectedProceeds, proceeds.String(), "项目结算标志与手续费快照金额必须匹配账本"),
		boolCheck("platform_fee_flag", (a.project.PlatformFeeWithdrawn != 0) == (a.counts[fundledger.EntryPlatformFeePaid] == 1) && a.counts[fundledger.EntryPlatformFeePaid] <= 1, "平台费标志必须匹配恰好一条账本事件"),
		flaggedRawCheck("platform_fee", a.project.PlatformFeeWithdrawn != 0, expectedFee, fees.String(), "平台费标志与手续费快照金额必须匹配账本"),
		boolCheck("project_token_withdrawal_flag", (a.project.TokenWithdrawn != 0) == (a.counts[fundledger.EntryProjectTokensWithdrawn] == 1) && a.counts[fundledger.EntryProjectTokensWithdrawn] <= 1, "项目币退出标志必须匹配恰好一条账本事件"),
		boolCheck("project_token_withdrawal_status", !withdrawalEventPresent || isTerminalProjectStatus(a.project.Status), "项目币退出要求 Successful、Failed 或 Canceled 状态"),
		boolCheck("failed_cancelled_withdrawal_amount", validFailedCancelledWithdrawal(a.project, deposits, tokenWithdrawals, withdrawalEventPresent), "失败/取消项目币退出必须等于全额存入金额"),
		boolCheck("successful_withdrawal_amount", validSuccessfulWithdrawal(a.project, deposits, a.tokensSold, tokenWithdrawals, withdrawalEventPresent, entitlementComplete), "成功项目币退出必须等于存入减去权益衍生 totalTokensSold"),
		boolCheck("terminal_payment_flow", validTerminalPaymentFlow(a.project, a.counts[fundledger.EntryPaymentRefunded], a.counts[fundledger.EntryProjectProceedsPaid], a.counts[fundledger.EntryPlatformFeePaid]), "退款与结算事件必须匹配项目终态"),
		boolCheck("claim_status", a.counts[fundledger.EntryProjectTokenClaimed] == 0 || a.project.Status == constant.ProjectStatusSuccessful, "项目币 Claim 要求 Successful 状态"),
		boolCheck("claim_wallet_ownership", a.claimWalletsHavePurchases(), "每个 Claim 钱包必须有正式认购记录"),
		boolCheck("claim_entitlement_evidence", entitlementComplete && a.claimsWithinEntitlements(), "每个钱包累计 Claim 不得超过权益衍生 tokensBought"),
	}
	summary := purchaseEntitlementSummary{
		Count: a.entitlementCount, PaymentAmountRaw: a.entitlementPay.String(),
		PaymentAmountDisplay: formatAggregateRaw(a.entitlementPay, uint8(a.project.PaymentTokenDecimals)),
		TotalTokensSoldRaw:   a.tokensSold.String(), TotalTokensSoldDisplay: formatAggregateRaw(a.tokensSold, uint8(a.project.TokenDecimals)),
	}
	return aggregates, checks, summary
}

func (a *fundReconciliationAccumulator) sum(entryType string) *big.Int {
	if value := a.sums[entryType]; value != nil {
		return new(big.Int).Set(value)
	}
	return new(big.Int)
}

func addWalletAmount(target map[string]*big.Int, wallet string, amount *big.Int) {
	if target[wallet] == nil {
		target[wallet] = new(big.Int)
	}
	target[wallet].Add(target[wallet], amount)
}

func (a *fundReconciliationAccumulator) walletRefundsMatchPurchases() bool {
	for wallet, refunded := range a.refundByWallet {
		purchased := a.purchaseByWallet[wallet]
		if purchased == nil || a.refundCountByWallet[wallet] != 1 || refunded.Cmp(purchased) != 0 {
			return false
		}
	}
	return true
}

func (a *fundReconciliationAccumulator) claimWalletsHavePurchases() bool {
	for wallet := range a.claimWallets {
		if _, ok := a.purchaseWallets[wallet]; !ok {
			return false
		}
	}
	return true
}

func (a *fundReconciliationAccumulator) claimsWithinEntitlements() bool {
	for wallet, claimed := range a.claimsByWallet {
		entitled := a.tokensByWallet[wallet]
		if entitled == nil || claimed.Cmp(entitled) > 0 {
			return false
		}
	}
	return true
}

func canonicalPositiveRaw(raw string) (*big.Int, bool) {
	canonical, err := money.NormalizeRawUint256(raw)
	if err != nil || canonical != raw {
		return nil, false
	}
	value, ok := new(big.Int).SetString(canonical, 10)
	return value, ok && value.Sign() > 0
}

func purchaseEntitlementMatches(parent entity.FundLedger, fact entity.IDOPurchaseEntitlement) bool {
	return parent.EntryType == fundledger.EntryPurchasePayment && parent.AssetRole == fundledger.AssetRolePayment && parent.Status == fundledger.StatusCanonical &&
		parent.ChainID == fact.ChainID && parent.ProjectID == fact.ProjectID &&
		strings.EqualFold(parent.ContractAddress, fact.ContractAddress) && parent.ContractVersion == fact.ContractVersion &&
		strings.EqualFold(parent.WalletAddress, fact.WalletAddress) && strings.EqualFold(parent.CounterpartyAddress, fact.ContractAddress) &&
		parent.TokenSymbol == fact.PaymentTokenSymbol && strings.EqualFold(parent.TokenAddress, fact.PaymentTokenAddress) && parent.TokenDecimals == fact.PaymentTokenDecimals &&
		parent.AmountRaw == fact.PaymentAmountRaw && parent.AmountDisplay == fact.PaymentAmountDisplay &&
		strings.EqualFold(parent.TxHash, fact.TxHash) && parent.LogIndex == fact.LogIndex && parent.BlockNumber == fact.BlockNumber && strings.EqualFold(parent.BlockHash, fact.BlockHash)
}

func expectedSettlementRaw(totalRaw string, feeRate uint16) (string, string) {
	total, ok := new(big.Int).SetString(totalRaw, 10)
	if !ok || total.Sign() < 0 {
		return "invalid", "invalid"
	}
	fee := new(big.Int).Mul(new(big.Int).Set(total), new(big.Int).SetUint64(uint64(feeRate)))
	fee.Div(fee, big.NewInt(10_000))
	return fee.String(), new(big.Int).Sub(total, fee).String()
}

func rawEqualityCheck(code, expected, actual, message string) fundReconciliationCheck {
	return fundReconciliationCheck{Code: code, OK: expected == actual, Expected: expected, Actual: actual, Message: message}
}

func valueEqualityCheckWithOK(code, expected, actual string, valid bool, message string) fundReconciliationCheck {
	return fundReconciliationCheck{Code: code, OK: valid, Expected: expected, Actual: actual, Message: message}
}

func boolCheck(code string, ok bool, message string) fundReconciliationCheck {
	return fundReconciliationCheck{Code: code, OK: ok, Message: message}
}

func flaggedRawCheck(code string, flagged bool, expected, actual, message string) fundReconciliationCheck {
	want := "0"
	if flagged {
		want = expected
	}
	return rawEqualityCheck(code, want, actual, message)
}

func validTerminalPaymentFlow(project entity.Project, refundEvents, proceedsEvents, feeEvents uint64) bool {
	switch project.Status {
	case constant.ProjectStatusSuccessful:
		return refundEvents == 0
	case constant.ProjectStatusFailed, constant.ProjectStatusCanceled:
		return proceedsEvents == 0 && feeEvents == 0 && project.FundWithdrawn == 0 && project.PlatformFeeWithdrawn == 0
	default:
		return refundEvents == 0 && proceedsEvents == 0 && feeEvents == 0 && project.FundWithdrawn == 0 && project.PlatformFeeWithdrawn == 0
	}
}

func validTerminalRefundCompletion(status string, purchases, refunds *big.Int) bool {
	if status != constant.ProjectStatusFailed && status != constant.ProjectStatusCanceled {
		return true
	}
	return refunds.Cmp(purchases) == 0
}

func isTerminalProjectStatus(status string) bool {
	return status == constant.ProjectStatusSuccessful || status == constant.ProjectStatusFailed || status == constant.ProjectStatusCanceled
}

func validFailedCancelledWithdrawal(project entity.Project, deposits, withdrawals *big.Int, eventPresent bool) bool {
	if project.Status != constant.ProjectStatusFailed && project.Status != constant.ProjectStatusCanceled {
		return true
	}
	if project.TokenWithdrawn == 0 {
		return !eventPresent
	}
	return eventPresent && withdrawals.Cmp(deposits) == 0
}

func validSuccessfulWithdrawal(project entity.Project, deposits, sold, withdrawals *big.Int, eventPresent, entitlementComplete bool) bool {
	if project.Status != constant.ProjectStatusSuccessful {
		return true
	}
	if project.TokenWithdrawn == 0 {
		return !eventPresent
	}
	if !eventPresent || !entitlementComplete || sold.Cmp(deposits) > 0 {
		return false
	}
	expected := new(big.Int).Sub(new(big.Int).Set(deposits), sold)
	return withdrawals.Cmp(expected) == 0
}

func projectFundSnapshot(project entity.Project) fundProjectSnapshot {
	tokenPrice := new(big.Int)
	if _, ok := tokenPrice.SetString(project.TokenPriceRaw, 10); !ok {
		tokenPrice = new(big.Int)
	}
	return fundProjectSnapshot{ProjectID: project.ProjectID, NameEn: project.NameEn, Status: project.Status, ChainID: project.ChainID, ContractAddress: project.ContractAddress, ContractVersion: project.ContractVersion, PaymentToken: project.PaymentToken, PaymentTokenAddress: project.PaymentTokenAddress, PaymentTokenDecimals: project.PaymentTokenDecimals, ProjectToken: project.TokenSymbol, ProjectTokenAddress: project.TokenAddress, ProjectTokenDecimals: project.TokenDecimals, TokenPriceRaw: project.TokenPriceRaw, TokenPriceDisplay: formatAggregateRaw(tokenPrice, uint8(project.PaymentTokenDecimals)), TotalRaisedRaw: project.TotalRaisedRaw, TotalRaisedDisplay: project.TotalRaised.String(), TotalDepositedRaw: project.TotalDepositedRaw, TotalDepositedDisplay: project.TotalDeposited.String(), ParticipantCount: project.ParticipantCount, FeeReceiver: project.FeeReceiver, SuccessFeeRate: project.SuccessFeeRate, FundWithdrawn: project.FundWithdrawn != 0, PlatformFeeWithdrawn: project.PlatformFeeWithdrawn != 0, TokenWithdrawn: project.TokenWithdrawn != 0}
}

func validFundProjectIdentity(project entity.Project) bool {
	return project.ProjectID > 0 && project.ChainID > 0 && constant.IsExactV2ProjectIdentity(project.FundingMode, project.ContractVersion) &&
		common.IsHexAddress(project.ContractAddress) && common.HexToAddress(project.ContractAddress) != (common.Address{}) &&
		common.IsHexAddress(project.PaymentTokenAddress) && common.HexToAddress(project.PaymentTokenAddress) != (common.Address{}) &&
		common.IsHexAddress(project.TokenAddress) && common.HexToAddress(project.TokenAddress) != (common.Address{}) &&
		common.IsHexAddress(project.OwnerAddress) && common.HexToAddress(project.OwnerAddress) != (common.Address{}) &&
		common.IsHexAddress(project.FeeReceiver) && common.HexToAddress(project.FeeReceiver) != (common.Address{}) &&
		!strings.EqualFold(project.PaymentTokenAddress, project.TokenAddress) &&
		(project.PaymentToken == constant.PaymentTokenUSDT || project.PaymentToken == constant.PaymentTokenUSDC) &&
		project.PaymentTokenDecimals >= 1 && project.PaymentTokenDecimals <= 18 && project.TokenDecimals >= 1 && project.TokenDecimals <= 18 &&
		project.SuccessFeeRate <= 1_000 && validPositiveRaw(project.TokenPriceRaw)
}

func validPositiveRaw(raw string) bool {
	_, ok := canonicalPositiveRaw(raw)
	return ok
}

func validLedgerEventIdentity(fact entity.FundLedger) bool {
	return fact.ID > 0 && fact.ChainID > 0 && fact.ProjectID > 0 && fact.BlockNumber > 0 && validEVMHash(fact.TxHash) && validEVMHash(fact.BlockHash)
}

func validEVMHash(value string) bool {
	if len(value) != 66 || !strings.HasPrefix(value, "0x") {
		return false
	}
	for _, char := range value[2:] {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}

func projectRawDisplayMatches(raw string, decimals uint8, display string) (string, bool) {
	formatted, err := money.FormatRaw(raw, decimals)
	if err != nil {
		return "invalid", false
	}
	normalizedDisplay, err := decimal.NewFromString(display)
	if err != nil {
		return formatted, false
	}
	expectedDisplay, err := decimal.NewFromString(formatted)
	return formatted, err == nil && normalizedDisplay.Equal(expectedDisplay)
}

func formatAggregateRaw(value *big.Int, decimals uint8) string {
	if value == nil {
		return "0"
	}
	return formatRawString(value.String(), decimals)
}

// formatRawString 将十进制 raw 字符串除以 10^decimals 后格式化为可读金额
func formatRawString(raw string, decimals uint8) string {
	if raw == "" {
		return "0"
	}
	value, ok := new(big.Int).SetString(raw, 10)
	if !ok || value.Sign() == 0 {
		return "0"
	}
	digits := value.String()
	if decimals == 0 {
		return digits
	}
	if len(digits) <= int(decimals) {
		digits = strings.Repeat("0", int(decimals)-len(digits)+1) + digits
	}
	cut := len(digits) - int(decimals)
	fraction := strings.TrimRight(digits[cut:], "0")
	if fraction == "" {
		return digits[:cut]
	}
	return digits[:cut] + "." + fraction
}

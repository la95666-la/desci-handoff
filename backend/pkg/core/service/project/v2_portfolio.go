package project

import (
	"errors"
	"math/big"
	"sort"
	"strings"
	"time"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/fundledger"
	"joincare_server/pkg/common/money"
	"joincare_server/pkg/core/db"
	"joincare_server/pkg/core/repository/user"
)

type V2PortfolioResponse struct {
	Positions []V2PortfolioPosition `json:"positions"`
	Purchases []V2PortfolioRecord   `json:"purchases"`
	Claims    []V2PortfolioRecord   `json:"claims"`
	Refunds   []V2PortfolioRecord   `json:"refunds"`
}

type V2PortfolioPosition struct {
	ProjectID            uint64 `json:"projectID,string"`
	Name                 string `json:"name"`
	Status               string `json:"status"`
	ChainID              uint64 `json:"chainId"`
	ContractAddress      string `json:"contractAddress"`
	ContractVersion      string `json:"contractVersion"`
	OnchainLocked        bool   `json:"onchainLocked,omitempty"`
	FeeReceiver          string `json:"feeReceiver,omitempty"`
	OwnerAddress         string `json:"ownerAddress,omitempty"`
	PaymentToken         string `json:"paymentToken"`
	PaymentTokenAddress  string `json:"paymentTokenAddress"`
	PaymentTokenDecimals uint8  `json:"paymentTokenDecimals"`
	PaymentAmountRaw     string `json:"paymentAmountRaw"`
	PaymentAmountDisplay string `json:"paymentAmountDisplay"`
	ProjectToken         string `json:"projectToken"`
	ProjectTokenAddress  string `json:"projectTokenAddress"`
	ProjectTokenDecimals uint8  `json:"projectTokenDecimals"`
	TokensBoughtRaw      string `json:"tokensBoughtRaw"`
	TokensBoughtDisplay  string `json:"tokensBoughtDisplay"`
	ClaimedRaw           string `json:"claimedRaw"`
	ClaimedDisplay       string `json:"claimedDisplay"`
	ClaimableRaw         string `json:"claimableRaw"`
	ClaimableDisplay     string `json:"claimableDisplay"`
	NextClaimTime        int64  `json:"nextClaimTime"`
	NextClaimRaw         string `json:"nextClaimRaw"`
	NextClaimDisplay     string `json:"nextClaimDisplay"`
	RefundedRaw          string `json:"refundedRaw"`
	RefundedDisplay      string `json:"refundedDisplay"`
	RefundableRaw        string `json:"refundableRaw"`
	RefundableDisplay    string `json:"refundableDisplay"`
}

type V2PortfolioRecord struct {
	ID                    uint64 `json:"id"`
	ProjectID             uint64 `json:"projectID,string"`
	Name                  string `json:"name"`
	Status                string `json:"status"`
	ChainID               uint64 `json:"chainId"`
	ContractAddress       string `json:"contractAddress"`
	ContractVersion       string `json:"contractVersion"`
	TokenSymbol           string `json:"tokenSymbol"`
	TokenAddress          string `json:"tokenAddress"`
	TokenDecimals         uint8  `json:"tokenDecimals"`
	AmountRaw             string `json:"amountRaw"`
	AmountDisplay         string `json:"amountDisplay"`
	ProjectTokenSymbol    string `json:"projectTokenSymbol,omitempty"`
	ProjectTokenAddress   string `json:"projectTokenAddress,omitempty"`
	ProjectTokenDecimals  uint8  `json:"projectTokenDecimals,omitempty"`
	ProjectTokenAmountRaw string `json:"projectTokenAmountRaw,omitempty"`
	ProjectTokenDisplay   string `json:"projectTokenAmountDisplay,omitempty"`
	TxHash                string `json:"txHash"`
	LogIndex              uint   `json:"logIndex"`
	BlockNumber           uint64 `json:"blockNumber"`
	CreatedAt             int64  `json:"createdAt"`
}

type portfolioAccumulator struct {
	position V2PortfolioPosition
	payment  *big.Int
	bought   *big.Int
	claimed  *big.Int
	refunded *big.Int
}

func V2Portfolio(uid int, lang string) (*V2PortfolioResponse, error) {
	wallet := user.WalletAddress(uid)
	if wallet == "" {
		return nil, errors.New("authenticated user has no primary wallet")
	}
	entitlements := make([]entity.IDOPurchaseEntitlement, 0, 20)
	var entitlementCount int64
	entitlementScope := db.Mysql().Model(&entity.IDOPurchaseEntitlement{}).
		Joins("JOIN projects p ON p.project_id = ido_purchase_entitlement.project_id").
		Where("LOWER(ido_purchase_entitlement.wallet_address) = ? AND ido_purchase_entitlement.contract_version = ? AND ido_purchase_entitlement.status = ? AND p.funding_mode = ? AND p.contract_version = ?", strings.ToLower(wallet), constant.ContractVersionV2Stablecoin, fundledger.StatusCanonical, constant.FundingModeCappedPresale, constant.ContractVersionV2Stablecoin)
	if err := entitlementScope.Count(&entitlementCount).Error; err != nil {
		return nil, err
	}
	if entitlementCount > 2000 {
		return nil, errors.New("V2 portfolio has more than 2000 purchase facts; paginated portfolio API is required")
	}
	if err := db.Mysql().Model(&entity.IDOPurchaseEntitlement{}).
		Joins("JOIN projects p ON p.project_id = ido_purchase_entitlement.project_id").
		Where("LOWER(ido_purchase_entitlement.wallet_address) = ? AND ido_purchase_entitlement.contract_version = ? AND ido_purchase_entitlement.status = ? AND p.funding_mode = ? AND p.contract_version = ?", strings.ToLower(wallet), constant.ContractVersionV2Stablecoin, fundledger.StatusCanonical, constant.FundingModeCappedPresale, constant.ContractVersionV2Stablecoin).
		Order("ido_purchase_entitlement.id DESC").Find(&entitlements).Error; err != nil {
		return nil, err
	}
	ledgers := make([]entity.FundLedger, 0, 20)
	var ledgerCount int64
	ledgerScope := db.Mysql().Model(&entity.FundLedger{}).
		Joins("JOIN projects p ON p.project_id = fund_ledger.project_id").
		Where("LOWER(fund_ledger.wallet_address) = ? AND fund_ledger.contract_version = ? AND fund_ledger.status = ? AND fund_ledger.entry_type IN ? AND p.funding_mode = ? AND p.contract_version = ?", strings.ToLower(wallet), constant.ContractVersionV2Stablecoin, fundledger.StatusCanonical, []string{fundledger.EntryProjectTokenClaimed, fundledger.EntryPaymentRefunded}, constant.FundingModeCappedPresale, constant.ContractVersionV2Stablecoin)
	if err := ledgerScope.Count(&ledgerCount).Error; err != nil {
		return nil, err
	}
	if ledgerCount > 2000 {
		return nil, errors.New("V2 portfolio has more than 2000 claim/refund facts; paginated portfolio API is required")
	}
	if err := db.Mysql().Model(&entity.FundLedger{}).
		Joins("JOIN projects p ON p.project_id = fund_ledger.project_id").
		Where("LOWER(fund_ledger.wallet_address) = ? AND fund_ledger.contract_version = ? AND fund_ledger.status = ? AND fund_ledger.entry_type IN ? AND p.funding_mode = ? AND p.contract_version = ?", strings.ToLower(wallet), constant.ContractVersionV2Stablecoin, fundledger.StatusCanonical, []string{fundledger.EntryProjectTokenClaimed, fundledger.EntryPaymentRefunded}, constant.FundingModeCappedPresale, constant.ContractVersionV2Stablecoin).
		Order("fund_ledger.id DESC").Find(&ledgers).Error; err != nil {
		return nil, err
	}
	projectIDs := make([]uint64, 0, len(entitlements)+len(ledgers))
	seen := map[uint64]struct{}{}
	for _, row := range entitlements {
		if _, ok := seen[row.ProjectID]; !ok {
			seen[row.ProjectID] = struct{}{}
			projectIDs = append(projectIDs, row.ProjectID)
		}
	}
	for _, row := range ledgers {
		if _, ok := seen[row.ProjectID]; !ok {
			seen[row.ProjectID] = struct{}{}
			projectIDs = append(projectIDs, row.ProjectID)
		}
	}
	projects := make([]entity.Project, 0, len(projectIDs))
	if len(projectIDs) != 0 {
		if err := db.Mysql().Where("project_id IN ? AND funding_mode = ? AND contract_version = ?", projectIDs, constant.FundingModeCappedPresale, constant.ContractVersionV2Stablecoin).Find(&projects).Error; err != nil {
			return nil, err
		}
	}
	projectByID := make(map[uint64]entity.Project, len(projects))
	for _, row := range projects {
		projectByID[uint64(row.ProjectID)] = row
	}
	if len(projectByID) != len(projectIDs) {
		return nil, errors.New("V2 portfolio facts reference a missing or non-V2 project; reconciliation required")
	}
	acc := make(map[uint64]*portfolioAccumulator, len(projects))
	result := &V2PortfolioResponse{Positions: []V2PortfolioPosition{}, Purchases: []V2PortfolioRecord{}, Claims: []V2PortfolioRecord{}, Refunds: []V2PortfolioRecord{}}
	for _, row := range entitlements {
		project, ok := projectByID[row.ProjectID]
		if !ok {
			continue
		}
		item := ensurePortfolioAccumulator(acc, row, project, lang)
		if err := addRaw(item.payment, row.PaymentAmountRaw); err != nil {
			return nil, err
		}
		if err := addRaw(item.bought, row.ProjectTokenAmountRaw); err != nil {
			return nil, err
		}
		result.Purchases = append(result.Purchases, V2PortfolioRecord{ID: row.ID, ProjectID: row.ProjectID, Name: portfolioProjectName(project, lang), Status: project.Status, ChainID: row.ChainID, ContractAddress: row.ContractAddress, ContractVersion: row.ContractVersion, TokenSymbol: row.PaymentTokenSymbol, TokenAddress: row.PaymentTokenAddress, TokenDecimals: row.PaymentTokenDecimals, AmountRaw: row.PaymentAmountRaw, AmountDisplay: row.PaymentAmountDisplay, ProjectTokenSymbol: row.ProjectTokenSymbol, ProjectTokenAddress: row.ProjectTokenAddress, ProjectTokenDecimals: row.ProjectTokenDecimals, ProjectTokenAmountRaw: row.ProjectTokenAmountRaw, ProjectTokenDisplay: row.ProjectTokenDisplay, TxHash: row.TxHash, LogIndex: row.LogIndex, BlockNumber: row.BlockNumber, CreatedAt: row.CreatedAt.Unix()})
	}
	for _, row := range ledgers {
		project, ok := projectByID[row.ProjectID]
		if !ok {
			continue
		}
		item := acc[row.ProjectID]
		if item == nil {
			return nil, errors.New("V2 claim/refund fact has no purchase entitlement; reconciliation required")
		}
		record := V2PortfolioRecord{ID: row.ID, ProjectID: row.ProjectID, Name: portfolioProjectName(project, lang), Status: project.Status, ChainID: row.ChainID, ContractAddress: row.ContractAddress, ContractVersion: row.ContractVersion, TokenSymbol: row.TokenSymbol, TokenAddress: row.TokenAddress, TokenDecimals: row.TokenDecimals, AmountRaw: row.AmountRaw, AmountDisplay: row.AmountDisplay, TxHash: row.TxHash, LogIndex: row.LogIndex, BlockNumber: row.BlockNumber, CreatedAt: row.CreatedAt.Unix()}
		switch row.EntryType {
		case fundledger.EntryProjectTokenClaimed:
			if err := addRaw(item.claimed, row.AmountRaw); err != nil {
				return nil, err
			}
			result.Claims = append(result.Claims, record)
		case fundledger.EntryPaymentRefunded:
			if err := addRaw(item.refunded, row.AmountRaw); err != nil {
				return nil, err
			}
			result.Refunds = append(result.Refunds, record)
		}
	}
	tges := make([]entity.ProjectTge, 0, len(projectIDs)*2)
	if len(projectIDs) != 0 {
		if err := db.Mysql().Where("project_id IN ?", projectIDs).Order("project_id, `index`").Find(&tges).Error; err != nil {
			return nil, err
		}
	}
	tgeByProject := map[uint64][]entity.ProjectTge{}
	for _, row := range tges {
		tgeByProject[uint64(row.ProjectID)] = append(tgeByProject[uint64(row.ProjectID)], row)
	}
	for projectID, item := range acc {
		if err := finalizePortfolioPosition(&item.position, item.payment, item.bought, item.claimed, item.refunded, tgeByProject[projectID], time.Now()); err != nil {
			return nil, err
		}
		result.Positions = append(result.Positions, item.position)
	}
	sort.Slice(result.Positions, func(i, j int) bool { return result.Positions[i].ProjectID > result.Positions[j].ProjectID })
	return result, nil
}

func ensurePortfolioAccumulator(values map[uint64]*portfolioAccumulator, row entity.IDOPurchaseEntitlement, project entity.Project, lang string) *portfolioAccumulator {
	if existing := values[row.ProjectID]; existing != nil {
		return existing
	}
	value := &portfolioAccumulator{payment: new(big.Int), bought: new(big.Int), claimed: new(big.Int), refunded: new(big.Int)}
	value.position = V2PortfolioPosition{ProjectID: row.ProjectID, Name: portfolioProjectName(project, lang), Status: project.Status, ChainID: row.ChainID, ContractAddress: row.ContractAddress, ContractVersion: row.ContractVersion, PaymentToken: row.PaymentTokenSymbol, PaymentTokenAddress: row.PaymentTokenAddress, PaymentTokenDecimals: row.PaymentTokenDecimals, ProjectToken: row.ProjectTokenSymbol, ProjectTokenAddress: row.ProjectTokenAddress, ProjectTokenDecimals: row.ProjectTokenDecimals}
	values[row.ProjectID] = value
	return value
}

func portfolioProjectName(project entity.Project, lang string) string {
	if useChineseProjectName(lang) && project.NameCn != "" {
		return project.NameCn
	}
	return project.NameEn
}

func addRaw(total *big.Int, raw string) error {
	canonical, err := money.NormalizeRawUint256(raw)
	if err != nil {
		return err
	}
	value, ok := new(big.Int).SetString(canonical, 10)
	if !ok {
		return errors.New("invalid raw uint256")
	}
	total.Add(total, value)
	return nil
}

func finalizePortfolioPosition(out *V2PortfolioPosition, payment, bought, claimed, refunded *big.Int, schedule []entity.ProjectTge, now time.Time) error {
	format := func(value *big.Int, decimals uint8) (string, string, error) {
		raw := value.String()
		display, err := money.FormatRaw(raw, decimals)
		return raw, display, err
	}
	var err error
	out.PaymentAmountRaw, out.PaymentAmountDisplay, err = format(payment, out.PaymentTokenDecimals)
	if err != nil {
		return err
	}
	out.TokensBoughtRaw, out.TokensBoughtDisplay, err = format(bought, out.ProjectTokenDecimals)
	if err != nil {
		return err
	}
	out.ClaimedRaw, out.ClaimedDisplay, err = format(claimed, out.ProjectTokenDecimals)
	if err != nil {
		return err
	}
	out.RefundedRaw, out.RefundedDisplay, err = format(refunded, out.PaymentTokenDecimals)
	if err != nil {
		return err
	}
	claimable, nextAmount := new(big.Int), new(big.Int)
	if out.Status == constant.ProjectStatusSuccessful {
		unlocked, nextBasis, nextTime := int64(0), int64(0), int64(0)
		for _, tge := range schedule {
			basis := tge.VestingPercent.Shift(2)
			if !basis.IsInteger() {
				return errors.New("non-canonical vesting basis points")
			}
			value := basis.IntPart()
			if !tge.VestingTime.After(now) {
				unlocked += value
			} else if nextTime == 0 {
				nextTime, nextBasis = tge.VestingTime.Unix(), value
			}
		}
		vested := new(big.Int).Mul(bought, big.NewInt(unlocked))
		vested.Quo(vested, big.NewInt(10_000))
		claimable.Sub(vested, claimed)
		if claimable.Sign() < 0 {
			claimable.SetInt64(0)
		}
		if nextTime != 0 {
			nextAmount.Mul(bought, big.NewInt(nextBasis))
			nextAmount.Quo(nextAmount, big.NewInt(10_000))
			out.NextClaimTime = nextTime
		}
	}
	out.ClaimableRaw, out.ClaimableDisplay, err = format(claimable, out.ProjectTokenDecimals)
	if err != nil {
		return err
	}
	out.NextClaimRaw, out.NextClaimDisplay, err = format(nextAmount, out.ProjectTokenDecimals)
	if err != nil {
		return err
	}
	refundable := new(big.Int)
	if out.Status == constant.ProjectStatusFailed || out.Status == constant.ProjectStatusCanceled {
		refundable.Sub(payment, refunded)
		if refundable.Sign() < 0 {
			refundable.SetInt64(0)
		}
	}
	out.RefundableRaw, out.RefundableDisplay, err = format(refundable, out.PaymentTokenDecimals)
	return err
}

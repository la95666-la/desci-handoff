package project

import (
	"database/sql"
	json "encoding/json/v2"
	"errors"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/money"
	"joincare_server/pkg/core/db"
)

type v3PortfolioProject struct {
	ProjectID            uint64 `gorm:"column:project_id"`
	NameEn               string `gorm:"column:name_en"`
	NameCn               string `gorm:"column:name_cn"`
	Status               string `gorm:"column:status"`
	State                string `gorm:"column:state"`
	ChainID              uint64 `gorm:"column:chain_id"`
	ProxyAddress         string `gorm:"column:proxy_address"`
	PaymentToken         string `gorm:"column:payment_token"`
	PaymentTokenAddress  string `gorm:"column:payment_token_address"`
	PaymentTokenDecimals uint8  `gorm:"column:payment_token_decimals"`
	ProjectToken         string `gorm:"column:token_symbol"`
	ProjectTokenAddress  string `gorm:"column:token_address"`
	ProjectTokenDecimals uint8  `gorm:"column:token_decimals"`
	Wallets              string `gorm:"column:wallets"`
	FeeReceiver          string `gorm:"column:fee_receiver"`
	OwnerAddress         string `gorm:"column:owner_address"`
}

type v3PortfolioState struct {
	project                   v3PortfolioProject
	position                  V2PortfolioPosition
	payment, tokens           *big.Int
	claimed, refunded         *big.Int
	effectPay, effectTok      *big.Int
	effectClaim, effectRefund *big.Int
}

type v3PortfolioEffect struct {
	JournalID     uint64    `gorm:"column:journal_id"`
	ProjectID     uint64    `gorm:"column:project_id"`
	WalletAddress string    `gorm:"column:wallet_address"`
	EntryKind     string    `gorm:"column:entry_kind"`
	AssetRole     string    `gorm:"column:asset_role"`
	TokenAddress  string    `gorm:"column:token_address"`
	AmountRaw     string    `gorm:"column:amount_raw"`
	TxHash        string    `gorm:"column:tx_hash"`
	LogIndex      uint      `gorm:"column:log_index"`
	BlockNumber   uint64    `gorm:"column:block_number"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

// IDOV3Portfolio returns only consistency-gated, canonical V3 financial facts.
// It deliberately does not reinterpret the V2 ledger or Warmup registrations.
func IDOV3Portfolio(wallet, lang string) (*V2PortfolioResponse, error) {
	if !common.IsHexAddress(wallet) || common.HexToAddress(wallet) == (common.Address{}) {
		return nil, errors.New("authenticated V3 wallet is invalid")
	}
	wallet = strings.ToLower(common.HexToAddress(wallet).Hex())
	var result *V2PortfolioResponse
	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = readV3Portfolio(tx, wallet, lang)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}

func v3PortfolioProjectionQuery(database *gorm.DB, projectID uint64) *gorm.DB {
	// Plain column equalities expose composite-key lookups to MySQL. Keep the
	// adjacent BINARY predicates as residual byte-exact identity checks: casts
	// alone prevent use of later key parts and scan the checkpoint history.
	query := `SELECT p.project_id,p.name_en,p.name_cn,p.status,x.state,x.chain_id,x.proxy_address,
		p.payment_token,p.payment_token_address,p.payment_token_decimals,p.token_symbol,p.token_address,p.token_decimals,x.wallets,p.fee_receiver,p.owner_address
		FROM ido_v3_project_projection x
		JOIN ido_v3_scan_cursor c ON c.chain_id=x.chain_id AND c.proxy_address=x.proxy_address AND BINARY c.proxy_address=BINARY x.proxy_address
			AND c.project_id=x.project_id AND c.contract_version=x.contract_version AND BINARY c.contract_version=BINARY x.contract_version
		JOIN ido_v3_block_checkpoint h ON h.chain_id=c.chain_id AND h.proxy_address=c.proxy_address AND BINARY h.proxy_address=BINARY c.proxy_address
			AND h.project_id=c.project_id AND h.contract_version=c.contract_version AND BINARY h.contract_version=BINARY c.contract_version
			AND h.block_number=c.cursor_block AND h.block_hash=c.cursor_hash AND BINARY h.block_hash=BINARY c.cursor_hash AND h.status='canonical'
		JOIN ido_v3_project_binding b ON b.chain_id=x.chain_id AND b.proxy_address=x.proxy_address AND BINARY b.proxy_address=BINARY x.proxy_address
			AND b.project_id=x.project_id AND b.contract_version=x.contract_version AND BINARY b.contract_version=BINARY x.contract_version
		JOIN projects p ON p.project_id=b.project_id AND p.chain_id=b.chain_id
			AND p.contract_address=b.proxy_address AND BINARY p.contract_address=BINARY b.proxy_address
			AND p.contract_version=b.contract_version AND BINARY p.contract_version=BINARY b.contract_version
		JOIN ido_v3_set_approval_identity i ON i.project_id=b.project_id AND i.chain_id=b.chain_id
			AND i.proxy_address=b.proxy_address AND BINARY i.proxy_address=BINARY b.proxy_address
			AND i.contract_version=b.contract_version AND BINARY i.contract_version=BINARY b.contract_version
		WHERE BINARY x.contract_version=BINARY 'ido_v3_stablecoin' AND c.state='running'
			AND x.revision=c.revision AND x.cursor_block=c.cursor_block AND x.cursor_hash=c.cursor_hash AND BINARY x.cursor_hash=BINARY c.cursor_hash
			AND BINARY p.funding_mode=BINARY 'capped_presale' AND BINARY p.contract_version=BINARY 'ido_v3_stablecoin'
			AND p.onchain_locked=1
			AND BINARY p.token_address=BINARY JSON_UNQUOTE(JSON_EXTRACT(i.identity_json,'$.project_token'))
			AND BINARY p.payment_token_address=BINARY JSON_UNQUOTE(JSON_EXTRACT(i.identity_json,'$.payment_token'))
			AND p.token_decimals=JSON_EXTRACT(i.identity_json,'$.token_decimals')
			AND p.payment_token_decimals=JSON_EXTRACT(i.identity_json,'$.payment_decimals')
			AND BINARY p.owner_address=BINARY JSON_UNQUOTE(JSON_EXTRACT(i.identity_json,'$.project_owner'))
			AND BINARY p.fee_receiver=BINARY JSON_UNQUOTE(JSON_EXTRACT(i.identity_json,'$.fee_receiver'))`
	if projectID != 0 {
		return database.Raw(query+" AND p.project_id=? ORDER BY p.project_id DESC LIMIT 2001", projectID)
	}
	return database.Raw(query + " ORDER BY p.project_id DESC LIMIT 2001")
}

func readV3Portfolio(database *gorm.DB, wallet, lang string) (*V2PortfolioResponse, error) {
	var rows []v3PortfolioProject
	projectionRead := v3PortfolioProjectionQuery(database, 0).Scan(&rows)
	if projectionRead.Error != nil {
		return nil, errors.New("V3 portfolio projection unavailable")
	}
	if len(rows) > 2000 {
		return nil, errors.New("V3 portfolio has more than 2000 positions; paginated portfolio API is required")
	}

	type walletProjection struct {
		Payment  string `json:"payment"`
		Tokens   string `json:"tokens"`
		Refunded string `json:"refunded"`
		Claimed  string `json:"claimed"`
	}
	positions := make(map[uint64]*v3PortfolioState)
	projectIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		if v3PortfolioBusinessStatus(row.State) != row.Status || row.ProjectID == 0 || row.ChainID == 0 ||
			!strings.EqualFold(row.ProxyAddress, strings.ToLower(row.ProxyAddress)) ||
			!common.IsHexAddress(row.ProxyAddress) || !common.IsHexAddress(row.PaymentTokenAddress) || !common.IsHexAddress(row.ProjectTokenAddress) ||
			!common.IsHexAddress(row.FeeReceiver) || !common.IsHexAddress(row.OwnerAddress) ||
			row.PaymentTokenDecimals == 0 || row.PaymentTokenDecimals > 18 || row.ProjectTokenDecimals == 0 || row.ProjectTokenDecimals > 18 {
			return nil, errors.New("V3 portfolio project identity unavailable")
		}
		all := map[string]walletProjection{}
		if json.Unmarshal([]byte(row.Wallets), &all) != nil {
			return nil, errors.New("V3 portfolio wallet projection invalid")
		}
		projected, ok := all[wallet]
		if !ok {
			continue
		}
		values := make([]*big.Int, 0, 4)
		for _, raw := range []string{projected.Payment, projected.Tokens, projected.Claimed, projected.Refunded} {
			canonical, err := money.NormalizeRawUint256(raw)
			value, valid := new(big.Int).SetString(canonical, 10)
			if err != nil || !valid || canonical != raw {
				return nil, errors.New("V3 portfolio wallet amount invalid")
			}
			values = append(values, value)
		}
		if values[0].Sign() == 0 || values[1].Sign() == 0 || values[2].Cmp(values[1]) > 0 || values[3].Cmp(values[0]) > 0 {
			return nil, errors.New("V3 portfolio wallet position inconsistent")
		}
		name := row.NameEn
		if useChineseProjectName(lang) && row.NameCn != "" {
			name = row.NameCn
		}
		state := &v3PortfolioState{project: row, payment: values[0], tokens: values[1], claimed: values[2], refunded: values[3],
			effectPay: new(big.Int), effectTok: new(big.Int), effectClaim: new(big.Int), effectRefund: new(big.Int)}
		state.position = V2PortfolioPosition{ProjectID: row.ProjectID, Name: name, Status: row.Status, ChainID: row.ChainID,
			OnchainLocked: true, FeeReceiver: row.FeeReceiver, OwnerAddress: row.OwnerAddress,
			ContractAddress: row.ProxyAddress, ContractVersion: constant.ContractVersionIDOV3Stablecoin,
			PaymentToken: row.PaymentToken, PaymentTokenAddress: row.PaymentTokenAddress, PaymentTokenDecimals: row.PaymentTokenDecimals,
			ProjectToken: row.ProjectToken, ProjectTokenAddress: row.ProjectTokenAddress, ProjectTokenDecimals: row.ProjectTokenDecimals}
		positions[row.ProjectID] = state
		projectIDs = append(projectIDs, row.ProjectID)
	}

	result := &V2PortfolioResponse{Positions: []V2PortfolioPosition{}, Purchases: []V2PortfolioRecord{}, Claims: []V2PortfolioRecord{}, Refunds: []V2PortfolioRecord{}}
	if len(projectIDs) == 0 {
		return result, nil
	}
	var effects []v3PortfolioEffect
	effectRead := v3PortfolioEffectsQuery(database, wallet, projectIDs).Scan(&effects)
	if effectRead.Error != nil || len(effects) > 4000 {
		return nil, errors.New("V3 portfolio canonical financial history unavailable")
	}
	return finishV3Portfolio(database, positions, projectIDs, effects, result)
}

func v3PortfolioEffectsQuery(database *gorm.DB, wallet string, projectIDs []uint64) *gorm.DB {
	query := `SELECT e.journal_id,e.project_id,e.wallet_address,e.entry_kind,e.asset_role,e.token_address,e.amount_raw,
		e.tx_hash,e.log_index,e.block_number,j.created_at
		FROM ido_v3_financial_effect e
		JOIN ido_v3_event_journal j ON j.id=e.journal_id AND j.chain_id=e.chain_id
			AND j.proxy_address=e.proxy_address AND BINARY j.proxy_address=BINARY e.proxy_address AND j.project_id=e.project_id
			AND j.contract_version=e.contract_version AND BINARY j.contract_version=BINARY e.contract_version AND j.block_number=e.block_number
			AND j.block_hash=e.block_hash AND BINARY j.block_hash=BINARY e.block_hash
			AND j.tx_hash=e.tx_hash AND BINARY j.tx_hash=BINARY e.tx_hash
			AND j.log_index=e.log_index AND j.event_kind=e.entry_kind AND BINARY j.event_kind=BINARY e.entry_kind AND j.status='canonical'
		JOIN ido_v3_project_projection x ON x.chain_id=e.chain_id AND x.proxy_address=e.proxy_address AND BINARY x.proxy_address=BINARY e.proxy_address
			AND x.project_id=e.project_id AND x.contract_version=e.contract_version AND BINARY x.contract_version=BINARY e.contract_version
		JOIN ido_v3_scan_cursor c ON c.chain_id=x.chain_id AND c.proxy_address=x.proxy_address AND BINARY c.proxy_address=BINARY x.proxy_address
			AND c.project_id=x.project_id AND c.contract_version=x.contract_version AND BINARY c.contract_version=BINARY x.contract_version
		JOIN ido_v3_block_checkpoint h ON h.chain_id=e.chain_id AND h.proxy_address=e.proxy_address AND BINARY h.proxy_address=BINARY e.proxy_address
			AND h.project_id=e.project_id AND h.contract_version=e.contract_version AND BINARY h.contract_version=BINARY e.contract_version
			AND h.block_number=e.block_number AND h.block_hash=e.block_hash AND BINARY h.block_hash=BINARY e.block_hash AND h.status='canonical'
		WHERE e.project_id IN ? AND BINARY e.contract_version=BINARY 'ido_v3_stablecoin'
			AND e.entry_kind IN ('purchase','Claimed','Refunded')
			AND e.status='canonical' AND c.state='running' AND x.revision=c.revision
			AND x.cursor_block=c.cursor_block AND x.cursor_hash=c.cursor_hash AND BINARY x.cursor_hash=BINARY c.cursor_hash AND e.block_number<=x.cursor_block
		`
	if wallet != "" {
		return database.Raw(query+" AND e.wallet_address=? ORDER BY e.block_number DESC,e.log_index DESC,e.asset_role ASC LIMIT 4001", projectIDs, wallet)
	}
	return database.Raw(query+" ORDER BY e.block_number DESC,e.log_index DESC,e.asset_role ASC LIMIT 4001", projectIDs)
}

func finishV3Portfolio(database *gorm.DB, positions map[uint64]*v3PortfolioState, projectIDs []uint64, effects []v3PortfolioEffect, result *V2PortfolioResponse) (*V2PortfolioResponse, error) {
	if err := applyV3PortfolioEffects(positions, effects, result); err != nil {
		return nil, err
	}
	return finalizeV3Portfolio(database, positions, projectIDs, result)
}

// Shared financial consistency validation for investor and owner views. This
// never reads V2 ledgers, and does not require a vesting schedule to list buys.
func applyV3PortfolioEffects(positions map[uint64]*v3PortfolioState, effects []v3PortfolioEffect, result *V2PortfolioResponse) error {
	type purchasePair struct{ payment, project *v3PortfolioEffect }
	purchases := map[uint64]*purchasePair{}
	for i := range effects {
		effect := &effects[i]
		state := positions[effect.ProjectID]
		if state == nil || effect.JournalID == 0 || effect.BlockNumber == 0 || effect.TxHash == "" {
			return errors.New("V3 portfolio financial identity unavailable")
		}
		canonical, err := money.NormalizeRawUint256(effect.AmountRaw)
		value, ok := new(big.Int).SetString(canonical, 10)
		if err != nil || !ok || canonical != effect.AmountRaw || value.Sign() == 0 {
			return errors.New("V3 portfolio financial amount invalid")
		}
		switch effect.EntryKind {
		case "purchase":
			pair := purchases[effect.JournalID]
			if pair == nil {
				pair = &purchasePair{}
				purchases[effect.JournalID] = pair
			}
			if effect.AssetRole == "payment" && strings.EqualFold(effect.TokenAddress, state.project.PaymentTokenAddress) && pair.payment == nil {
				pair.payment = effect
				state.effectPay.Add(state.effectPay, value)
			} else if effect.AssetRole == "project" && strings.EqualFold(effect.TokenAddress, state.project.ProjectTokenAddress) && pair.project == nil {
				pair.project = effect
				state.effectTok.Add(state.effectTok, value)
			} else {
				return errors.New("V3 portfolio purchase effect inconsistent")
			}
		case "Claimed":
			if effect.AssetRole != "project" || !strings.EqualFold(effect.TokenAddress, state.project.ProjectTokenAddress) {
				return errors.New("V3 portfolio claim effect inconsistent")
			}
			state.effectClaim.Add(state.effectClaim, value)
			display, formatErr := money.FormatRaw(effect.AmountRaw, state.project.ProjectTokenDecimals)
			if formatErr != nil {
				return formatErr
			}
			result.Claims = append(result.Claims, v3PortfolioRecord(state, effect, state.project.ProjectToken, state.project.ProjectTokenAddress, state.project.ProjectTokenDecimals, display))
		case "Refunded":
			if effect.AssetRole != "payment" || !strings.EqualFold(effect.TokenAddress, state.project.PaymentTokenAddress) {
				return errors.New("V3 portfolio refund effect inconsistent")
			}
			state.effectRefund.Add(state.effectRefund, value)
			display, formatErr := money.FormatRaw(effect.AmountRaw, state.project.PaymentTokenDecimals)
			if formatErr != nil {
				return formatErr
			}
			result.Refunds = append(result.Refunds, v3PortfolioRecord(state, effect, state.project.PaymentToken, state.project.PaymentTokenAddress, state.project.PaymentTokenDecimals, display))
		}
	}
	for _, pair := range purchases {
		if pair.payment == nil || pair.project == nil || pair.payment.ProjectID != pair.project.ProjectID || pair.payment.TxHash != pair.project.TxHash || pair.payment.LogIndex != pair.project.LogIndex || pair.payment.WalletAddress != pair.project.WalletAddress {
			return errors.New("V3 portfolio purchase pair unavailable")
		}
		state := positions[pair.payment.ProjectID]
		paymentDisplay, err := money.FormatRaw(pair.payment.AmountRaw, state.project.PaymentTokenDecimals)
		if err != nil {
			return err
		}
		tokenDisplay, err := money.FormatRaw(pair.project.AmountRaw, state.project.ProjectTokenDecimals)
		if err != nil {
			return err
		}
		record := v3PortfolioRecord(state, pair.payment, state.project.PaymentToken, state.project.PaymentTokenAddress, state.project.PaymentTokenDecimals, paymentDisplay)
		record.ProjectTokenSymbol, record.ProjectTokenAddress, record.ProjectTokenDecimals = state.project.ProjectToken, state.project.ProjectTokenAddress, state.project.ProjectTokenDecimals
		record.ProjectTokenAmountRaw, record.ProjectTokenDisplay = pair.project.AmountRaw, tokenDisplay
		result.Purchases = append(result.Purchases, record)
	}

	for _, state := range positions {
		if state.payment.Cmp(state.effectPay) != 0 || state.tokens.Cmp(state.effectTok) != 0 ||
			state.claimed.Cmp(state.effectClaim) != 0 || state.refunded.Cmp(state.effectRefund) != 0 {
			return errors.New("V3 portfolio projection and financial effects disagree")
		}
	}
	return nil
}

func finalizeV3Portfolio(database *gorm.DB, positions map[uint64]*v3PortfolioState, projectIDs []uint64, result *V2PortfolioResponse) (*V2PortfolioResponse, error) {
	var tges []entity.ProjectTge
	if err := database.Where("project_id IN ?", projectIDs).Order("project_id, `index`").Find(&tges).Error; err != nil {
		return nil, err
	}
	schedules := map[uint64][]entity.ProjectTge{}
	for _, tge := range tges {
		schedules[uint64(tge.ProjectID)] = append(schedules[uint64(tge.ProjectID)], tge)
	}
	for projectID, state := range positions {
		if err := finalizePortfolioPosition(&state.position, state.payment, state.tokens, state.claimed, state.refunded, schedules[projectID], time.Now()); err != nil {
			return nil, err
		}
		if state.position.Status == constant.ProjectStatusSuccessful && len(schedules[projectID]) == 0 {
			return nil, errors.New("V3 portfolio vesting schedule unavailable")
		}
		result.Positions = append(result.Positions, state.position)
	}
	sort.Slice(result.Positions, func(i, j int) bool { return result.Positions[i].ProjectID > result.Positions[j].ProjectID })
	sort.Slice(result.Purchases, func(i, j int) bool { return result.Purchases[i].BlockNumber > result.Purchases[j].BlockNumber })
	sort.Slice(result.Claims, func(i, j int) bool { return result.Claims[i].BlockNumber > result.Claims[j].BlockNumber })
	sort.Slice(result.Refunds, func(i, j int) bool { return result.Refunds[i].BlockNumber > result.Refunds[j].BlockNumber })
	return result, nil
}

func v3PortfolioBusinessStatus(state string) string {
	switch state {
	case "pending":
		return constant.ProjectStatusPending
	case "active":
		return constant.ProjectStatusActive
	case "successful":
		return constant.ProjectStatusSuccessful
	case "failed":
		return constant.ProjectStatusFailed
	case "cancelled":
		return constant.ProjectStatusCanceled
	default:
		return ""
	}
}

func v3PortfolioRecord(state *v3PortfolioState, effect *v3PortfolioEffect, symbol, token string, decimals uint8, display string) V2PortfolioRecord {
	return V2PortfolioRecord{ID: effect.JournalID, ProjectID: effect.ProjectID, Name: state.position.Name, Status: state.project.Status,
		ChainID: state.project.ChainID, ContractAddress: state.project.ProxyAddress, ContractVersion: constant.ContractVersionIDOV3Stablecoin,
		TokenSymbol: symbol, TokenAddress: token, TokenDecimals: decimals, AmountRaw: effect.AmountRaw, AmountDisplay: display,
		TxHash: effect.TxHash, LogIndex: effect.LogIndex, BlockNumber: effect.BlockNumber, CreatedAt: effect.CreatedAt.Unix()}
}

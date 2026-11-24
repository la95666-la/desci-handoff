package project

import (
	"database/sql"
	json "encoding/json/v2"
	"errors"
	"math/big"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/money"
	"joincare_server/pkg/core/db"
)

// IDOV3ProjectInvestors is owner-only, read-only and deliberately separate from
// the V2 ledger. Projection, canonical effects and totals share one DB snapshot.
func IDOV3ProjectInvestors(uid int, projectID uint64, owner string, page, pageSize int) (*ProjectInvestorsResponse, error) {
	if uid <= 0 || projectID == 0 || !common.IsHexAddress(owner) || common.HexToAddress(owner) == (common.Address{}) {
		return nil, errors.New("V3 investor owner identity invalid")
	}
	var result *ProjectInvestorsResponse
	err := db.Mysql().Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Table("user_wallet w").
			Joins("JOIN user_wallet_identity i ON i.wallet_address=w.wallet_address AND i.uid=w.uid AND i.chain=w.chain").
			Where("w.uid=? AND w.wallet_address=? AND w.chain='evm' AND w.verified_at IS NOT NULL", uid, strings.ToLower(owner)).
			Count(&count).Error; err != nil || count != 1 {
			return errors.New("V3 investor records require a verified owner wallet")
		}
		var err error
		result, err = readV3ProjectInvestors(tx, projectID, owner, page, pageSize)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}

func readV3ProjectInvestors(tx *gorm.DB, projectID uint64, owner string, page, pageSize int) (*ProjectInvestorsResponse, error) {
	var projects []v3PortfolioProject
	if err := v3PortfolioProjectionQuery(tx, projectID).Scan(&projects).Error; err != nil {
		return nil, errors.New("V3 investor projection unavailable")
	}
	// Missing/halted/unreconciled projection is unavailable, never an empty list.
	if len(projects) != 1 {
		return nil, errors.New("V3 investor projection not synchronized")
	}
	p := projects[0]
	if p.ProjectID != projectID || !strings.EqualFold(p.OwnerAddress, owner) ||
		v3PortfolioBusinessStatus(p.State) != p.Status || p.ChainID == 0 ||
		!common.IsHexAddress(p.ProxyAddress) || !common.IsHexAddress(p.PaymentTokenAddress) ||
		!common.IsHexAddress(p.ProjectTokenAddress) || !common.IsHexAddress(p.FeeReceiver) ||
		p.PaymentTokenDecimals == 0 || p.PaymentTokenDecimals > 18 || p.ProjectTokenDecimals == 0 || p.ProjectTokenDecimals > 18 {
		return nil, errors.New("V3 investor project identity unavailable")
	}
	var wallets map[string]struct {
		Payment  string `json:"payment"`
		Tokens   string `json:"tokens"`
		Claimed  string `json:"claimed"`
		Refunded string `json:"refunded"`
	}
	if json.Unmarshal([]byte(p.Wallets), &wallets) != nil || wallets == nil {
		return nil, errors.New("V3 investor wallet projection invalid")
	}
	states := map[string]*v3PortfolioState{}
	for wallet, w := range wallets {
		if !common.IsHexAddress(wallet) || wallet != strings.ToLower(wallet) || common.HexToAddress(wallet) == (common.Address{}) {
			return nil, errors.New("V3 investor wallet identity invalid")
		}
		values := []*big.Int{}
		for _, raw := range []string{w.Payment, w.Tokens, w.Claimed, w.Refunded} {
			canonical, err := money.NormalizeRawUint256(raw)
			if err != nil || canonical != raw {
				return nil, errors.New("V3 investor amount invalid")
			}
			v, _ := new(big.Int).SetString(raw, 10)
			values = append(values, v)
		}
		if values[0].Sign() <= 0 || values[1].Sign() <= 0 || values[2].Cmp(values[1]) > 0 || values[3].Cmp(values[0]) > 0 {
			return nil, errors.New("V3 investor position inconsistent")
		}
		states[wallet] = &v3PortfolioState{project: p, payment: values[0], tokens: values[1], claimed: values[2], refunded: values[3],
			effectPay: new(big.Int), effectTok: new(big.Int), effectClaim: new(big.Int), effectRefund: new(big.Int)}
	}
	var effects []v3PortfolioEffect
	if err := v3PortfolioEffectsQuery(tx, "", []uint64{projectID}).Scan(&effects).Error; err != nil || len(effects) > 4000 {
		return nil, errors.New("V3 investor canonical history unavailable or exceeds safety limit")
	}
	byWallet := map[string][]v3PortfolioEffect{}
	for _, e := range effects {
		if states[e.WalletAddress] == nil || e.ProjectID != projectID {
			return nil, errors.New("V3 investor effect identity inconsistent")
		}
		byWallet[e.WalletAddress] = append(byWallet[e.WalletAddress], e)
	}
	for wallet, state := range states {
		if err := applyV3PortfolioEffects(map[uint64]*v3PortfolioState{projectID: state}, byWallet[wallet], &V2PortfolioResponse{}); err != nil {
			return nil, err
		}
	}
	items := []ProjectInvestorItem{}
	for _, e := range effects {
		if e.EntryKind != "purchase" || e.AssetRole != "payment" {
			continue
		}
		display, err := money.FormatRaw(e.AmountRaw, p.PaymentTokenDecimals)
		if err != nil {
			return nil, err
		}
		items = append(items, ProjectInvestorItem{ID: e.JournalID, WalletAddress: e.WalletAddress, AmountDisplay: display,
			TokenSymbol: p.PaymentToken, TxHash: e.TxHash, BlockNumber: e.BlockNumber, Status: "canonical", CreatedAt: e.CreatedAt.Unix()})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].BlockNumber != items[j].BlockNumber {
			return items[i].BlockNumber > items[j].BlockNumber
		}
		return items[i].ID > items[j].ID
	})
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}
	total := len(items)
	start := total
	if page <= total/pageSize+1 {
		start = (page - 1) * pageSize
		if start > total {
			start = total
		}
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	items = items[start:end]
	addresses := []string{}
	for _, item := range items {
		addresses = append(addresses, item.WalletAddress)
	}
	if len(addresses) > 0 {
		var users []entity.User
		if err := tx.Select("wallet_address,twitter_handle,twitter_display_name,twitter_avatar,telegram_username,telegram_first_name,telegram_last_name,telegram_avatar").
			Where("LOWER(wallet_address) IN ?", addresses).Find(&users).Error; err != nil {
			return nil, err
		}
		for i := range items {
			for _, u := range users {
				if strings.EqualFold(u.WalletAddress, items[i].WalletAddress) {
					items[i].TwitterHandle = u.TwitterHandle
					items[i].TwitterName = u.TwitterDisplayName
					items[i].TwitterAvatar = u.TwitterAvatar
					items[i].TelegramUsername = u.TelegramUsername
					items[i].TelegramAvatar = u.TelegramAvatar
					items[i].TelegramName = strings.TrimSpace(u.TelegramFirstName + " " + u.TelegramLastName)
				}
			}
		}
	}
	return &ProjectInvestorsResponse{ProjectID: int(projectID), Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

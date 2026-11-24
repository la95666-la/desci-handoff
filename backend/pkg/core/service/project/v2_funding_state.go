package project

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/fundledger"
	"joincare_server/pkg/common/money"
	"joincare_server/pkg/core/db"
	"joincare_server/pkg/core/repository/user"
	chainManager "joincare_server/pkg/core/third/chain"
)

// FundingStateResponse is the wallet-scoped funding view for one project,
// resolved server-side (chain RPC + database) instead of through the browser
// wallet provider. This removes the browser-wallet dependency from read-only
// display data, so multi-extension provider conflicts no longer zero it out.
type FundingStateResponse struct {
	PaymentTokenSymbol   string `json:"paymentTokenSymbol"`
	PaymentTokenAddress  string `json:"paymentTokenAddress"`
	PaymentTokenDecimals uint8  `json:"paymentTokenDecimals"`

	BalanceRaw       string `json:"balanceRaw"`
	BalanceDisplay   string `json:"balanceDisplay"`
	AllowanceRaw     string `json:"allowanceRaw"`
	AllowanceDisplay string `json:"allowanceDisplay"`

	ContributionRaw     string `json:"contributionRaw"`
	ContributionDisplay string `json:"contributionDisplay"`
	TokensBoughtRaw     string `json:"tokensBoughtRaw"`
	TokensBoughtDisplay string `json:"tokensBoughtDisplay"`
	ClaimedRaw          string `json:"claimedRaw"`
	ClaimedDisplay      string `json:"claimedDisplay"`
	RefundedRaw         string `json:"refundedRaw"`
	RefundedDisplay     string `json:"refundedDisplay"`
}

func FundingState(uid int, projectID int) (*FundingStateResponse, error) {
	wallet := user.WalletAddress(uid)
	if wallet == "" {
		return nil, errors.New("authenticated user has no primary wallet")
	}

	var proj entity.Project
	if err := db.Mysql().Select("project_id,payment_token,payment_token_address,payment_token_decimals,token_symbol,token_decimals,contract_address,contract_version,funding_mode").
		Where("project_id = ?", projectID).
		Take(&proj).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}
	if err := constant.RequireExactV2ProjectIdentity(proj.FundingMode, proj.ContractVersion); err != nil {
		return nil, err
	}

	client := chainManager.GetClient(constant.ChainProjectV2)
	if client == nil {
		return nil, errors.New("IDO V2 chain client is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	token := common.HexToAddress(proj.PaymentTokenAddress)
	owner := common.HexToAddress(wallet)
	spender := common.HexToAddress(proj.ContractAddress)

	balance, err := client.ReadERC20BalanceAt(ctx, token, owner, nil)
	if err != nil {
		return nil, err
	}
	allowance, err := client.ReadERC20AllowanceAt(ctx, token, owner, spender, nil)
	if err != nil {
		return nil, err
	}

	// 贡献/获得代币优先链上直读（购买上链后立即反映，不等链事件同步）；
	// 链读失败时退回数据库聚合（链事件同步后的 canonical 值）。
	contribution := new(big.Int)
	bought := new(big.Int)
	if purchase, chainErr := client.ReadIDOV2UserPurchaseAt(ctx, new(big.Int).SetInt64(int64(projectID)), owner, nil); chainErr == nil {
		contribution.Set(purchase.PaymentAmount)
		bought.Set(purchase.TokensBought)
	} else {
		entitlements := make([]entity.IDOPurchaseEntitlement, 0, 16)
		if err := db.Mysql().Where("project_id = ? AND LOWER(wallet_address) = ? AND contract_version = ? AND status = ?", projectID, strings.ToLower(wallet), constant.ContractVersionV2Stablecoin, fundledger.StatusCanonical).
			Find(&entitlements).Error; err != nil {
			return nil, err
		}
		for _, row := range entitlements {
			if value, ok := new(big.Int).SetString(row.PaymentAmountRaw, 10); ok {
				contribution.Add(contribution, value)
			}
			if value, ok := new(big.Int).SetString(row.ProjectTokenAmountRaw, 10); ok {
				bought.Add(bought, value)
			}
		}
	}

	ledgers := make([]entity.FundLedger, 0, 16)
	if err := db.Mysql().Where("project_id = ? AND LOWER(wallet_address) = ? AND contract_version = ? AND status = ? AND entry_type IN ?", projectID, strings.ToLower(wallet), constant.ContractVersionV2Stablecoin, fundledger.StatusCanonical, []string{fundledger.EntryProjectTokenClaimed, fundledger.EntryPaymentRefunded}).
		Find(&ledgers).Error; err != nil {
		return nil, err
	}
	claimed := new(big.Int)
	refunded := new(big.Int)
	for _, row := range ledgers {
		switch row.EntryType {
		case fundledger.EntryProjectTokenClaimed:
			if value, ok := new(big.Int).SetString(row.AmountRaw, 10); ok {
				claimed.Add(claimed, value)
			}
		case fundledger.EntryPaymentRefunded:
			if value, ok := new(big.Int).SetString(row.AmountRaw, 10); ok {
				refunded.Add(refunded, value)
			}
		}
	}

	pd := uint8(proj.PaymentTokenDecimals)
	td := uint8(proj.TokenDecimals)
	resp := &FundingStateResponse{
		PaymentTokenSymbol:   proj.PaymentToken,
		PaymentTokenAddress:  proj.PaymentTokenAddress,
		PaymentTokenDecimals: pd,
		BalanceRaw:           balance.String(),
		AllowanceRaw:         allowance.String(),
		ContributionRaw:      contribution.String(),
		TokensBoughtRaw:      bought.String(),
		ClaimedRaw:           claimed.String(),
		RefundedRaw:          refunded.String(),
	}
	resp.BalanceDisplay, _ = money.FormatRaw(balance.String(), pd)
	resp.AllowanceDisplay, _ = money.FormatRaw(allowance.String(), pd)
	resp.ContributionDisplay, _ = money.FormatRaw(contribution.String(), pd)
	resp.RefundedDisplay, _ = money.FormatRaw(refunded.String(), pd)
	resp.TokensBoughtDisplay, _ = money.FormatRaw(bought.String(), td)
	resp.ClaimedDisplay, _ = money.FormatRaw(claimed.String(), td)

	return resp, nil
}

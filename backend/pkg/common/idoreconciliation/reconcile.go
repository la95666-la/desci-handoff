package idoreconciliation

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	baseChain "joincare_server/pkg/common/chain"
)

const ScopeOnchainSnapshot = "onchain_snapshot"

type Reader interface {
	IDOV2ContractAddress() common.Address
	RPCChainID(context.Context) (*big.Int, error)
	LatestBlockNumber(context.Context) (uint64, error)
	HeaderAt(context.Context, *big.Int) (*types.Header, error)
	VerifyIDOV2RuntimeAt(context.Context, *big.Int, common.Address, common.Hash) error
	ReadIDOV2ProjectAt(context.Context, *big.Int, *big.Int) (baseChain.IDOV2ProjectSnapshot, error)
	ReadIDOV2UserPurchaseAt(context.Context, *big.Int, common.Address, *big.Int) (baseChain.IDOV2UserPurchaseSnapshot, error)
	ReadIDOV2ParticipantsAt(context.Context, *big.Int, *big.Int) ([]common.Address, error)
	ReadIDOV2TokenConfigAt(context.Context, bool, common.Address, *big.Int) (baseChain.IDOV2TokenConfigSnapshot, error)
	ReadIDOV2ReservedBalanceAt(context.Context, common.Address, *big.Int) (*big.Int, error)
	ReadERC20BalanceAt(context.Context, common.Address, common.Address, *big.Int) (*big.Int, error)
	ReadERC20DecimalsAt(context.Context, common.Address, *big.Int) (uint8, error)
}

type Check struct {
	Code     string `json:"code"`
	OK       bool   `json:"ok"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Message  string `json:"message"`
}

type ExpectedWallet struct {
	Wallet           string `json:"wallet"`
	PaymentAmountRaw string `json:"payment_amount_raw"`
	TokensBoughtRaw  string `json:"tokens_bought_raw"`
	ClaimedTokensRaw string `json:"claimed_tokens_raw"`
	Refunded         bool   `json:"refunded"`
}

type ExpectedReserve struct {
	Role                string `json:"role"`
	Symbol              string `json:"symbol"`
	TokenAddress        string `json:"token_address"`
	TokenDecimals       uint8  `json:"token_decimals"`
	ProjectToken        bool   `json:"project_token"`
	ProjectLiabilityRaw string `json:"project_liability_raw"`
}

type ExpectedProject struct {
	ProjectID               uint64   `json:"project_id"`
	Status                  uint8    `json:"status"`
	StatusValid             bool     `json:"status_valid"`
	ContractAddress         string   `json:"contract_address"`
	ProjectToken            string   `json:"project_token"`
	PaymentToken            string   `json:"payment_token"`
	ProjectOwner            string   `json:"project_owner"`
	FeeReceiver             string   `json:"fee_receiver"`
	ProjectTokenDecimals    uint8    `json:"project_token_decimals"`
	PaymentTokenDecimals    uint8    `json:"payment_token_decimals"`
	SuccessFeeRate          uint16   `json:"success_fee_rate"`
	TokenPriceRaw           string   `json:"token_price_raw"`
	HardCapRaw              string   `json:"hard_cap_raw"`
	SoftCapRaw              string   `json:"soft_cap_raw"`
	MaxPerUserRaw           string   `json:"max_per_user_raw"`
	StartTimeRaw            string   `json:"start_time_raw"`
	EndTimeRaw              string   `json:"end_time_raw"`
	TotalRaisedRaw          string   `json:"total_raised_raw"`
	TotalTokensSoldRaw      string   `json:"total_tokens_sold_raw"`
	TotalTokensDepositedRaw string   `json:"total_tokens_deposited_raw"`
	FundsWithdrawn          bool     `json:"funds_withdrawn"`
	PlatformFeeWithdrawn    bool     `json:"platform_fee_withdrawn"`
	ProjectTokensWithdrawn  bool     `json:"project_tokens_withdrawn"`
	VestingTimesRaw         []string `json:"vesting_times_raw"`
	VestingPercentsRaw      []string `json:"vesting_percents_raw"`
	VestingValid            bool     `json:"vesting_valid"`
}

type Request struct {
	ChainID                    uint64            `json:"chain_id"`
	BlockNumber                uint64            `json:"block_number"`
	BlockHash                  string            `json:"block_hash"`
	LatestConfirmationDepth    uint64            `json:"confirmation_depth"`
	MaxHealthyLagBlocks        uint64            `json:"max_healthy_lag_blocks"`
	MaxParticipants            int               `json:"max_participants"`
	ExpectedImplementation     string            `json:"expected_implementation"`
	ExpectedImplementationHash string            `json:"expected_implementation_hash"`
	Project                    ExpectedProject   `json:"project"`
	Wallets                    []ExpectedWallet  `json:"wallets"`
	Reserves                   []ExpectedReserve `json:"reserves"`
}

type WalletSnapshot struct {
	Wallet           string `json:"wallet"`
	PaymentAmountRaw string `json:"payment_amount_raw"`
	TokensBoughtRaw  string `json:"tokens_bought_raw"`
	ClaimedTokensRaw string `json:"claimed_tokens_raw"`
	Refunded         bool   `json:"refunded"`
}

type TokenReserveSnapshot struct {
	Role                string `json:"role"`
	Symbol              string `json:"symbol"`
	TokenAddress        string `json:"token_address"`
	Allowed             bool   `json:"allowed"`
	Decimals            uint8  `json:"decimals"`
	ProjectDecimals     uint8  `json:"project_decimals"`
	GovernedDecimals    uint8  `json:"governed_decimals"`
	ERC20Decimals       uint8  `json:"erc20_decimals"`
	ProxyBalanceRaw     string `json:"proxy_balance_raw"`
	GlobalReservedRaw   string `json:"global_reserved_raw"`
	ProjectLiabilityRaw string `json:"project_liability_raw"`
}

type ProjectSnapshot struct {
	ProjectID               string   `json:"project_id"`
	Status                  uint8    `json:"status"`
	ProjectToken            string   `json:"project_token"`
	PaymentToken            string   `json:"payment_token"`
	ProjectOwner            string   `json:"project_owner"`
	FeeReceiver             string   `json:"fee_receiver"`
	ProjectTokenDecimals    uint8    `json:"project_token_decimals"`
	PaymentTokenDecimals    uint8    `json:"payment_token_decimals"`
	SuccessFeeRate          uint16   `json:"success_fee_rate"`
	TokenPriceRaw           string   `json:"token_price_raw"`
	HardCapRaw              string   `json:"hard_cap_raw"`
	SoftCapRaw              string   `json:"soft_cap_raw"`
	MaxPerUserRaw           string   `json:"max_per_user_raw"`
	StartTime               string   `json:"start_time"`
	EndTime                 string   `json:"end_time"`
	TotalRaisedRaw          string   `json:"total_raised_raw"`
	TotalTokensSoldRaw      string   `json:"total_tokens_sold_raw"`
	TotalTokensDepositedRaw string   `json:"total_tokens_deposited_raw"`
	FundsWithdrawn          bool     `json:"funds_withdrawn"`
	PlatformFeeWithdrawn    bool     `json:"platform_fee_withdrawn"`
	ProjectTokensWithdrawn  bool     `json:"project_tokens_withdrawn"`
	VestingTimes            []string `json:"vesting_times"`
	VestingPercents         []string `json:"vesting_percents"`
}

type Result struct {
	Scope         string                 `json:"scope"`
	Balanced      bool                   `json:"balanced"`
	BlockNumber   uint64                 `json:"block_number"`
	BlockHash     string                 `json:"block_hash"`
	LatestBlock   uint64                 `json:"latest_block"`
	Project       ProjectSnapshot        `json:"project"`
	Wallets       []WalletSnapshot       `json:"wallets"`
	TokenReserves []TokenReserveSnapshot `json:"token_reserves"`
	Checks        []Check                `json:"checks"`
}

type BaseSnapshot struct {
	LatestBlock uint64
	BlockHash   string
	Project     ProjectSnapshot
	Checks      []Check
}

func Reconcile(ctx context.Context, reader Reader, request Request) (Result, error) {
	base, err := CollectBase(ctx, reader, request)
	if err != nil {
		return Result{}, err
	}
	wallets, walletChecks, err := ObserveWalletRange(ctx, reader, request, 0, len(request.Wallets))
	if err != nil {
		return Result{}, err
	}
	return Complete(ctx, reader, request, base, wallets, walletChecks)
}

func CollectBase(ctx context.Context, reader Reader, request Request) (BaseSnapshot, error) {
	if reader == nil || request.ChainID == 0 || request.BlockNumber == 0 || request.Project.ProjectID == 0 {
		return BaseSnapshot{}, errors.New("invalid IDO reconciliation request")
	}
	proxy := common.HexToAddress(request.Project.ContractAddress)
	if !validAddress(request.Project.ContractAddress) || !validAddress(request.Project.ProjectToken) || !validAddress(request.Project.PaymentToken) ||
		!validAddress(request.Project.ProjectOwner) || !validAddress(request.Project.FeeReceiver) || proxy == (common.Address{}) || reader.IDOV2ContractAddress() != proxy {
		return BaseSnapshot{}, errors.New("V2 RPC client contract address does not match project identity")
	}
	if request.MaxParticipants <= 0 || len(request.Wallets) > request.MaxParticipants {
		return BaseSnapshot{}, fmt.Errorf("participant count %d exceeds reconciliation limit %d", len(request.Wallets), request.MaxParticipants)
	}
	if len(request.Reserves) != 2 {
		return BaseSnapshot{}, errors.New("reconciliation requires exactly two token reserve expectations")
	}
	block := new(big.Int).SetUint64(request.BlockNumber)
	chainID, err := reader.RPCChainID(ctx)
	if err != nil {
		return BaseSnapshot{}, fmt.Errorf("read RPC chain ID: %w", err)
	}
	if chainID == nil || !chainID.IsUint64() || chainID.Uint64() != request.ChainID {
		return BaseSnapshot{}, errors.New("RPC chain ID mismatch")
	}
	latest, err := reader.LatestBlockNumber(ctx)
	if err != nil || latest < request.BlockNumber {
		return BaseSnapshot{}, fmt.Errorf("RPC latest block does not cover DB cursor: latest=%d cursor=%d err=%v", latest, request.BlockNumber, err)
	}
	freshSafe := latest
	if request.LatestConfirmationDepth > 0 {
		if latest <= request.LatestConfirmationDepth {
			return BaseSnapshot{}, fmt.Errorf("RPC latest block %d has not reached confirmation depth %d", latest, request.LatestConfirmationDepth)
		}
		freshSafe = latest - request.LatestConfirmationDepth
	}
	if request.BlockNumber > freshSafe {
		return BaseSnapshot{}, fmt.Errorf("DB cursor %d exceeds fresh RPC safe block %d", request.BlockNumber, freshSafe)
	}
	if request.MaxHealthyLagBlocks == 0 || latest-request.BlockNumber > request.MaxHealthyLagBlocks {
		return BaseSnapshot{}, fmt.Errorf("RPC cursor lag %d exceeds healthy limit %d", latest-request.BlockNumber, request.MaxHealthyLagBlocks)
	}
	header, err := canonicalHeader(ctx, reader, block, request.BlockHash)
	if err != nil {
		return BaseSnapshot{}, err
	}
	implementation := common.HexToAddress(request.ExpectedImplementation)
	codeHash := common.HexToHash(request.ExpectedImplementationHash)
	if !validAddress(request.ExpectedImplementation) || !validHash(request.ExpectedImplementationHash) || implementation == (common.Address{}) || codeHash == (common.Hash{}) {
		return BaseSnapshot{}, errors.New("registered V2 implementation identity is invalid")
	}
	if err := reader.VerifyIDOV2RuntimeAt(ctx, block, implementation, codeHash); err != nil {
		return BaseSnapshot{}, err
	}
	projectID := new(big.Int).SetUint64(request.Project.ProjectID)
	onchainProject, err := reader.ReadIDOV2ProjectAt(ctx, projectID, block)
	if err != nil {
		return BaseSnapshot{}, err
	}
	participants, err := reader.ReadIDOV2ParticipantsAt(ctx, projectID, block)
	if err != nil {
		return BaseSnapshot{}, err
	}
	if len(participants) > request.MaxParticipants {
		return BaseSnapshot{}, fmt.Errorf("on-chain participant count %d exceeds reconciliation limit %d", len(participants), request.MaxParticipants)
	}
	checks := ProjectChecks(request.Project, request.Wallets, onchainProject, participants)
	return BaseSnapshot{LatestBlock: latest, BlockHash: header.Hash().Hex(), Project: projectJSON(onchainProject), Checks: checks}, nil
}

func ObserveWalletRange(ctx context.Context, reader Reader, request Request, start, end int) ([]WalletSnapshot, []Check, error) {
	if start < 0 || end < start || end > len(request.Wallets) {
		return nil, nil, errors.New("invalid reconciliation wallet range")
	}
	block := new(big.Int).SetUint64(request.BlockNumber)
	if _, err := canonicalHeader(ctx, reader, block, request.BlockHash); err != nil {
		return nil, nil, err
	}
	projectID := new(big.Int).SetUint64(request.Project.ProjectID)
	wallets := make([]WalletSnapshot, 0, end-start)
	checks := make([]Check, 0, (end-start)*4)
	for _, expected := range request.Wallets[start:end] {
		wallet := common.HexToAddress(expected.Wallet)
		if wallet == (common.Address{}) {
			return nil, nil, fmt.Errorf("invalid expected wallet %q", expected.Wallet)
		}
		purchase, err := reader.ReadIDOV2UserPurchaseAt(ctx, projectID, wallet, block)
		if err != nil {
			return nil, nil, err
		}
		actual := WalletSnapshot{Wallet: wallet.Hex(), PaymentAmountRaw: UintString(purchase.PaymentAmount), TokensBoughtRaw: UintString(purchase.TokensBought), ClaimedTokensRaw: UintString(purchase.ClaimedTokens), Refunded: purchase.Refunded}
		checks = append(checks, WalletChecks(expected, actual)...)
		wallets = append(wallets, actual)
	}
	if _, err := canonicalHeader(ctx, reader, block, request.BlockHash); err != nil {
		return nil, nil, err
	}
	return wallets, checks, nil
}

func Complete(ctx context.Context, reader Reader, request Request, base BaseSnapshot, wallets []WalletSnapshot, walletChecks []Check) (Result, error) {
	if len(wallets) != len(request.Wallets) || len(walletChecks) != len(request.Wallets)*4 {
		return Result{}, errors.New("wallet observations are incomplete")
	}
	for index := range wallets {
		if !strings.EqualFold(wallets[index].Wallet, request.Wallets[index].Wallet) {
			return Result{}, errors.New("wallet observation order does not match frozen source")
		}
	}
	block := new(big.Int).SetUint64(request.BlockNumber)
	proxy := common.HexToAddress(request.Project.ContractAddress)
	if proxy == (common.Address{}) {
		return Result{}, errors.New("invalid reconciliation proxy identity")
	}
	if _, err := canonicalHeader(ctx, reader, block, request.BlockHash); err != nil {
		return Result{}, err
	}
	checks := append(append([]Check(nil), base.Checks...), walletChecks...)
	reserves, reserveChecks, err := readReserves(ctx, reader, block, proxy, request.Reserves)
	if err != nil {
		return Result{}, err
	}
	checks = append(checks, reserveChecks...)
	finalHeader, err := canonicalHeader(ctx, reader, block, request.BlockHash)
	if err != nil {
		return Result{}, err
	}
	if !strings.EqualFold(finalHeader.Hash().Hex(), base.BlockHash) {
		return Result{}, errors.New("snapshot block changed during on-chain reconciliation")
	}
	return Result{Scope: ScopeOnchainSnapshot, Balanced: AllChecksOK(checks), BlockNumber: request.BlockNumber, BlockHash: finalHeader.Hash().Hex(), LatestBlock: base.LatestBlock, Project: base.Project, Wallets: wallets, TokenReserves: reserves, Checks: checks}, nil
}

func canonicalHeader(ctx context.Context, reader Reader, block *big.Int, expectedHash string) (*types.Header, error) {
	header, err := reader.HeaderAt(ctx, block)
	if err != nil {
		return nil, fmt.Errorf("read reconciliation snapshot header: %w", err)
	}
	if header == nil || header.Number == nil {
		return nil, errors.New("reconciliation snapshot header is unavailable")
	}
	if header.Number.Cmp(block) != 0 || !strings.EqualFold(header.Hash().Hex(), expectedHash) {
		return nil, errors.New("reconciliation snapshot block is no longer canonical on the configured RPC")
	}
	return header, nil
}

func ProjectChecks(expected ExpectedProject, wallets []ExpectedWallet, actual baseChain.IDOV2ProjectSnapshot, participants []common.Address) []Check {
	participantOK := len(participants) == len(wallets)
	if participantOK {
		for i := range participants {
			participantOK = strings.EqualFold(participants[i].Hex(), wallets[i].Wallet)
			if !participantOK {
				break
			}
		}
	}
	return []Check{
		rawCheck("chain_project_id", fmt.Sprint(expected.ProjectID), UintString(actual.ProjectID), "链上项目ID必须与数据库一致"),
		boolCheck("chain_project_identity", strings.EqualFold(actual.ProjectToken.Hex(), expected.ProjectToken) && strings.EqualFold(actual.PaymentToken.Hex(), expected.PaymentToken) && strings.EqualFold(actual.ProjectOwner.Hex(), expected.ProjectOwner) && strings.EqualFold(actual.FeeReceiver.Hex(), expected.FeeReceiver), "链上项目币/支付币/owner/费接收方必须与数据库一致"),
		boolCheck("chain_project_decimals_fee", actual.ProjectTokenDecimals == expected.ProjectTokenDecimals && actual.PaymentTokenDecimals == expected.PaymentTokenDecimals && actual.SuccessFeeRate == expected.SuccessFeeRate, "链上精度与费率快照必须与数据库一致"),
		boolCheck("chain_project_status", expected.StatusValid && actual.Status == expected.Status, "链上项目状态必须与数据库一致"),
		rawCheck("chain_token_price", expected.TokenPriceRaw, UintString(actual.TokenPrice), "链上代币价格必须与数据库一致"),
		rawCheck("chain_hard_cap", expected.HardCapRaw, UintString(actual.HardCap), "链上硬顶必须与数据库一致"),
		rawCheck("chain_soft_cap", expected.SoftCapRaw, UintString(actual.SoftCap), "链上软顶必须与数据库一致"),
		rawCheck("chain_max_per_user", expected.MaxPerUserRaw, UintString(actual.MaxPerUser), "链上每人上限必须与数据库一致"),
		rawCheck("chain_start_time", expected.StartTimeRaw, UintString(actual.StartTime), "链上开始时间必须与数据库一致"),
		rawCheck("chain_end_time", expected.EndTimeRaw, UintString(actual.EndTime), "链上结束时间必须与数据库一致"),
		rawCheck("chain_total_raised", expected.TotalRaisedRaw, UintString(actual.TotalRaised), "链上总融资额必须与数据库账本一致"),
		rawCheck("chain_total_tokens_sold", expected.TotalTokensSoldRaw, UintString(actual.TotalTokensSold), "链上总售出代币必须与认购权益一致"),
		rawCheck("chain_total_tokens_deposited", expected.TotalTokensDepositedRaw, UintString(actual.TotalTokensDeposited), "链上总存入代币必须与数据库账本一致"),
		boolCheck("chain_withdrawal_flags", actual.FundsWithdrawn == expected.FundsWithdrawn && actual.PlatformFeeWithdrawn == expected.PlatformFeeWithdrawn && actual.ProjectTokensWithdrawn == expected.ProjectTokensWithdrawn, "链上退出标志必须与数据库一致"),
		boolCheck("chain_vesting_schedule", expected.VestingValid && rawSlicesEqual(expected.VestingTimesRaw, actual.VestingTimes) && rawSlicesEqual(expected.VestingPercentsRaw, actual.VestingPercents), "链上解锁计划必须与存储的项目TGE一致"),
		boolCheck("chain_participants", participantOK, "链上参与人顺序与成员必须与认购权益一致"),
	}
}

func ProjectCheckCodes() []string {
	return []string{"chain_project_id", "chain_project_identity", "chain_project_decimals_fee", "chain_project_status", "chain_token_price", "chain_hard_cap", "chain_soft_cap", "chain_max_per_user", "chain_start_time", "chain_end_time", "chain_total_raised", "chain_total_tokens_sold", "chain_total_tokens_deposited", "chain_withdrawal_flags", "chain_vesting_schedule", "chain_participants"}
}

func WalletChecks(expected ExpectedWallet, actual WalletSnapshot) []Check {
	key := strings.ToLower(common.HexToAddress(expected.Wallet).Hex())
	return []Check{
		rawCheck("chain_wallet_payment_"+key, expected.PaymentAmountRaw, actual.PaymentAmountRaw, "钱包链上支付金额必须与认购账本一致"),
		rawCheck("chain_wallet_tokens_"+key, expected.TokensBoughtRaw, actual.TokensBoughtRaw, "钱包链上应得代币必须与权益一致"),
		rawCheck("chain_wallet_claimed_"+key, expected.ClaimedTokensRaw, actual.ClaimedTokensRaw, "钱包链上已Claim必须与领取代币账本一致"),
		boolCheck("chain_wallet_refunded_"+key, expected.Refunded == actual.Refunded, "钱包链上退款标志必须与退款账本一致"),
	}
}

func readReserves(ctx context.Context, reader Reader, block *big.Int, proxy common.Address, expected []ExpectedReserve) ([]TokenReserveSnapshot, []Check, error) {
	rows := make([]TokenReserveSnapshot, 0, len(expected))
	checks := make([]Check, 0, len(expected)*3)
	seen := make(map[string]struct{}, len(expected))
	for _, asset := range expected {
		if asset.Role != "payment" && asset.Role != "project" {
			return nil, nil, fmt.Errorf("invalid reserve role %q", asset.Role)
		}
		if _, exists := seen[asset.Role]; exists {
			return nil, nil, fmt.Errorf("duplicate reserve role %q", asset.Role)
		}
		seen[asset.Role] = struct{}{}
		token := common.HexToAddress(asset.TokenAddress)
		liability, ok := new(big.Int).SetString(asset.ProjectLiabilityRaw, 10)
		if token == (common.Address{}) || !ok || liability.BitLen() > 256 {
			return nil, nil, fmt.Errorf("invalid %s reserve expectation", asset.Role)
		}
		config, err := reader.ReadIDOV2TokenConfigAt(ctx, asset.ProjectToken, token, block)
		if err != nil {
			return nil, nil, err
		}
		reserved, err := reader.ReadIDOV2ReservedBalanceAt(ctx, token, block)
		if err != nil {
			return nil, nil, err
		}
		balance, err := reader.ReadERC20BalanceAt(ctx, token, proxy, block)
		if err != nil {
			return nil, nil, err
		}
		erc20Decimals, err := reader.ReadERC20DecimalsAt(ctx, token, block)
		if err != nil {
			return nil, nil, err
		}
		if reserved == nil || reserved.Sign() < 0 || balance == nil || balance.Sign() < 0 {
			return nil, nil, errors.New("invalid negative or nil token reserve observation")
		}
		checks = append(checks,
			boolCheck("chain_"+asset.Role+"_config_decimals", config.Decimals == asset.TokenDecimals && erc20Decimals == asset.TokenDecimals, "治理层与ERC20代币精度必须与项目快照一致；allowed仅是历史退出的信息性字段"),
			boolCheck("chain_"+asset.Role+"_reserve_solvency", balance.Cmp(reserved) >= 0, "代理合约代币余额必须覆盖全局预留余额"),
			boolCheck("chain_"+asset.Role+"_project_liability", liability.Sign() >= 0 && reserved.Cmp(liability) >= 0, "全局预留余额必须覆盖该项目未偿负债下限"),
		)
		rows = append(rows, TokenReserveSnapshot{Role: asset.Role, Symbol: asset.Symbol, TokenAddress: token.Hex(), Allowed: config.Allowed, Decimals: config.Decimals, ProjectDecimals: asset.TokenDecimals, GovernedDecimals: config.Decimals, ERC20Decimals: erc20Decimals, ProxyBalanceRaw: balance.String(), GlobalReservedRaw: reserved.String(), ProjectLiabilityRaw: liability.String()})
	}
	return rows, checks, nil
}

func projectJSON(value baseChain.IDOV2ProjectSnapshot) ProjectSnapshot {
	return ProjectSnapshot{ProjectID: UintString(value.ProjectID), Status: value.Status, ProjectToken: value.ProjectToken.Hex(), PaymentToken: value.PaymentToken.Hex(), ProjectOwner: value.ProjectOwner.Hex(), FeeReceiver: value.FeeReceiver.Hex(), ProjectTokenDecimals: value.ProjectTokenDecimals, PaymentTokenDecimals: value.PaymentTokenDecimals, SuccessFeeRate: value.SuccessFeeRate, TokenPriceRaw: UintString(value.TokenPrice), HardCapRaw: UintString(value.HardCap), SoftCapRaw: UintString(value.SoftCap), MaxPerUserRaw: UintString(value.MaxPerUser), StartTime: UintString(value.StartTime), EndTime: UintString(value.EndTime), TotalRaisedRaw: UintString(value.TotalRaised), TotalTokensSoldRaw: UintString(value.TotalTokensSold), TotalTokensDepositedRaw: UintString(value.TotalTokensDeposited), FundsWithdrawn: value.FundsWithdrawn, PlatformFeeWithdrawn: value.PlatformFeeWithdrawn, ProjectTokensWithdrawn: value.ProjectTokensWithdrawn, VestingTimes: BigIntStrings(value.VestingTimes), VestingPercents: BigIntStrings(value.VestingPercents)}
}

func rawCheck(code, expected, actual, message string) Check {
	_, expectedOK := canonicalUint(expected)
	_, actualOK := canonicalUint(actual)
	return Check{Code: code, OK: expectedOK && actualOK && expected == actual, Expected: expected, Actual: actual, Message: message}
}

func boolCheck(code string, ok bool, message string) Check {
	return Check{Code: code, OK: ok, Message: message}
}

func canonicalUint(value string) (*big.Int, bool) {
	if value == "0" {
		return new(big.Int), true
	}
	if value == "" || value[0] == '0' || len(value) > 78 {
		return nil, false
	}
	parsed, ok := new(big.Int).SetString(value, 10)
	if !ok || parsed.Sign() < 0 || parsed.BitLen() > 256 || parsed.String() != value {
		return nil, false
	}
	return parsed, true
}

func validAddress(value string) bool {
	return common.IsHexAddress(value) && common.HexToAddress(value) != (common.Address{})
}

func validHash(value string) bool {
	if len(value) != 66 || !strings.HasPrefix(value, "0x") {
		return false
	}
	for _, ch := range value[2:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", ch) {
			return false
		}
	}
	return true
}

func rawSlicesEqual(expected []string, actual []*big.Int) bool {
	if len(expected) != len(actual) {
		return false
	}
	for i := range expected {
		if expected[i] != UintString(actual[i]) {
			return false
		}
	}
	return true
}

func UintString(value *big.Int) string {
	if value == nil || value.Sign() < 0 || value.BitLen() > 256 {
		return "invalid"
	}
	return value.String()
}

func BigIntStrings(values []*big.Int) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = UintString(value)
	}
	return result
}

func AllChecksOK(checks []Check) bool {
	if len(checks) == 0 {
		return false
	}
	for _, check := range checks {
		if !check.OK {
			return false
		}
	}
	return true
}

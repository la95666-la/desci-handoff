package controller

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"

	baseChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/fundledger"
)

type fakeIDOSnapshotReader struct {
	contract      common.Address
	chainID       *big.Int
	latest        uint64
	header        *types.Header
	project       baseChain.IDOV2ProjectSnapshot
	participants  []common.Address
	purchases     map[common.Address]baseChain.IDOV2UserPurchaseSnapshot
	configs       map[common.Address]baseChain.IDOV2TokenConfigSnapshot
	reserves      map[common.Address]*big.Int
	balances      map[common.Address]*big.Int
	expectedBlock *big.Int
	runtimeErr    error
}

func (f *fakeIDOSnapshotReader) checkBlock(block *big.Int) {
	if block == nil || f.expectedBlock == nil || block.Cmp(f.expectedBlock) != 0 {
		panic("snapshot reader used a different block")
	}
}

func (f *fakeIDOSnapshotReader) IDOV2ContractAddress() common.Address { return f.contract }
func (f *fakeIDOSnapshotReader) RPCChainID(context.Context) (*big.Int, error) {
	return new(big.Int).Set(f.chainID), nil
}
func (f *fakeIDOSnapshotReader) LatestBlockNumber(context.Context) (uint64, error) {
	return f.latest, nil
}
func (f *fakeIDOSnapshotReader) HeaderAt(_ context.Context, block *big.Int) (*types.Header, error) {
	f.checkBlock(block)
	copy := *f.header
	return &copy, nil
}
func (f *fakeIDOSnapshotReader) VerifyIDOV2RuntimeAt(_ context.Context, block *big.Int, _ common.Address, _ common.Hash) error {
	f.checkBlock(block)
	return f.runtimeErr
}
func (f *fakeIDOSnapshotReader) ReadIDOV2ProjectAt(_ context.Context, _ *big.Int, block *big.Int) (baseChain.IDOV2ProjectSnapshot, error) {
	f.checkBlock(block)
	return f.project, nil
}
func (f *fakeIDOSnapshotReader) ReadIDOV2UserPurchaseAt(_ context.Context, _ *big.Int, user common.Address, block *big.Int) (baseChain.IDOV2UserPurchaseSnapshot, error) {
	f.checkBlock(block)
	return f.purchases[user], nil
}
func (f *fakeIDOSnapshotReader) ReadIDOV2ParticipantsAt(_ context.Context, _ *big.Int, block *big.Int) ([]common.Address, error) {
	f.checkBlock(block)
	return append([]common.Address(nil), f.participants...), nil
}
func (f *fakeIDOSnapshotReader) ReadIDOV2TokenConfigAt(_ context.Context, _ bool, token common.Address, block *big.Int) (baseChain.IDOV2TokenConfigSnapshot, error) {
	f.checkBlock(block)
	return f.configs[token], nil
}
func (f *fakeIDOSnapshotReader) ReadIDOV2ReservedBalanceAt(_ context.Context, token common.Address, block *big.Int) (*big.Int, error) {
	f.checkBlock(block)
	return new(big.Int).Set(f.reserves[token]), nil
}
func (f *fakeIDOSnapshotReader) ReadERC20BalanceAt(_ context.Context, token, _ common.Address, block *big.Int) (*big.Int, error) {
	f.checkBlock(block)
	return new(big.Int).Set(f.balances[token]), nil
}
func (f *fakeIDOSnapshotReader) ReadERC20DecimalsAt(_ context.Context, token common.Address, block *big.Int) (uint8, error) {
	f.checkBlock(block)
	return f.configs[token].Decimals, nil
}

func onchainFixture() (entity.Project, *fundReconciliationAccumulator, []entity.ProjectTge, baseChain.IDOV2ProjectSnapshot, []string, []common.Address) {
	project := fundTestProject()
	project.Status = constant.ProjectStatusSuccessful
	project.HardCapRaw, project.SoftCapRaw, project.MaxPerUserRaw = "1000000000", "500000000", "100000000"
	project.StartTime = time.Unix(1_800_000_000, 0)
	project.EndTime = time.Unix(1_800_086_400, 0)
	wallet := "0x4000000000000000000000000000000000000004"
	acc := newFundReconciliationAccumulator(project)
	acc.purchaseByWallet[strings.ToLower(wallet)] = big.NewInt(1_000_000_000)
	acc.tokensByWallet[strings.ToLower(wallet)] = mustControllerBig("1000000000000000000000")
	acc.claimsByWallet[strings.ToLower(wallet)] = mustControllerBig("250000000000000000000")
	acc.purchaseWallets[strings.ToLower(wallet)] = struct{}{}
	acc.tokensSold.Set(mustControllerBig("1000000000000000000000"))
	tges := []entity.ProjectTge{{Index: 1, VestingTime: time.Unix(1_800_100_000, 0), VestingPercent: decimal.NewFromInt(100)}}
	chainProject := baseChain.IDOV2ProjectSnapshot{
		ProjectID: big.NewInt(7), ProjectToken: common.HexToAddress(project.TokenAddress), PaymentToken: common.HexToAddress(project.PaymentTokenAddress), ProjectOwner: common.HexToAddress(project.OwnerAddress), FeeReceiver: common.HexToAddress(project.FeeReceiver),
		ProjectTokenDecimals: 18, PaymentTokenDecimals: 6, SuccessFeeRate: 200, Status: 2,
		TokenPrice: big.NewInt(1_000_000), HardCap: big.NewInt(1_000_000_000), SoftCap: big.NewInt(500_000_000), MaxPerUser: big.NewInt(100_000_000),
		StartTime: big.NewInt(1_800_000_000), EndTime: big.NewInt(1_800_086_400), TotalRaised: big.NewInt(1_000_000_000), TotalTokensSold: mustControllerBig("1000000000000000000000"), TotalTokensDeposited: mustControllerBig(project.TotalDepositedRaw),
		FundsWithdrawn: true, PlatformFeeWithdrawn: true, ProjectTokensWithdrawn: false,
		VestingTimes: []*big.Int{big.NewInt(1_800_100_000)}, VestingPercents: []*big.Int{big.NewInt(10_000)},
	}
	wallets := []string{strings.ToLower(wallet)}
	participants := []common.Address{common.HexToAddress(wallet)}
	return project, acc, tges, chainProject, wallets, participants
}

func TestProjectOnchainChecksExactSnapshot(t *testing.T) {
	project, acc, tges, chainProject, wallets, participants := onchainFixture()
	checks := projectOnchainChecks(project, acc, tges, chainProject, wallets, participants)
	if !allReconciliationChecksOK(checks) {
		t.Fatalf("exact on-chain snapshot rejected: %+v", checks)
	}
	chainProject.TotalTokensSold = new(big.Int).Add(chainProject.TotalTokensSold, big.NewInt(1))
	if allReconciliationChecksOK(projectOnchainChecks(project, acc, tges, chainProject, wallets, participants)) {
		t.Fatal("totalTokensSold drift accepted")
	}
}

func TestProjectOnchainChecksParticipantOrderFailsClosed(t *testing.T) {
	project, acc, tges, chainProject, wallets, _ := onchainFixture()
	wrong := []common.Address{common.HexToAddress("0x5000000000000000000000000000000000000005")}
	if allReconciliationChecksOK(projectOnchainChecks(project, acc, tges, chainProject, wallets, wrong)) {
		t.Fatal("participant drift accepted")
	}
}

func TestWalletOnchainChecks(t *testing.T) {
	_, acc, _, _, wallets, _ := onchainFixture()
	wallet := wallets[0]
	purchase := baseChain.IDOV2UserPurchaseSnapshot{PaymentAmount: big.NewInt(1_000_000_000), TokensBought: mustControllerBig("1000000000000000000000"), ClaimedTokens: mustControllerBig("250000000000000000000"), Refunded: false}
	if !allReconciliationChecksOK(walletOnchainChecks(wallet, acc, purchase)) {
		t.Fatal("matching wallet snapshot rejected")
	}
	purchase.ClaimedTokens.Add(purchase.ClaimedTokens, big.NewInt(1))
	if allReconciliationChecksOK(walletOnchainChecks(wallet, acc, purchase)) {
		t.Fatal("wallet claimedTokens drift accepted")
	}
	purchase.ClaimedTokens = nil
	if allReconciliationChecksOK(walletOnchainChecks(wallet, acc, purchase)) {
		t.Fatal("nil wallet amount accepted")
	}
}

func TestValidateOnchainSyncState(t *testing.T) {
	project, _, _, _, _, _ := onchainFixture()
	now := time.Now()
	viper.Set("chainEvent.maxHealthyLagBlocks", 100)
	viper.Set("chain.project.chainID", project.ChainID)
	viper.Set("chain.project.confirmations", "5")
	viper.Set("chain.project.v2.contractAddress", project.ContractAddress)
	viper.Set("chain.project.v2.implementationAddress", "0x5000000000000000000000000000000000000005")
	viper.Set("chain.project.v2.implementationCodeHash", "0x"+strings.Repeat("a", 64))
	state := entity.ChainEventSyncState{
		Initialized: true, V2Registered: true, CursorBlock: 200, CursorBlockHash: "0x" + strings.Repeat("b", 64), V2StartBlock: 100,
		V2ContractAddress: project.ContractAddress, V2ImplementationAddress: "0x5000000000000000000000000000000000000005", V2ImplementationCodeHash: "0x" + strings.Repeat("a", 64),
		SafeBlock: 201, LagBlocks: 1, LastSuccessAt: &now, UpdatedAt: now,
	}
	if err := validateOnchainSyncState(project, state); err != nil {
		t.Fatalf("healthy sync rejected: %v", err)
	}
	state.CursorBlockHash = ""
	if err := validateOnchainSyncState(project, state); err == nil {
		t.Fatal("missing cursor hash accepted")
	}
}

func TestProjectStatusToV2(t *testing.T) {
	wants := map[string]uint8{constant.ProjectStatusPending: 0, constant.ProjectStatusActive: 1, constant.ProjectStatusSuccessful: 2, constant.ProjectStatusFailed: 3, constant.ProjectStatusCanceled: 4}
	for status, want := range wants {
		got, ok := projectStatusToV2(status)
		if !ok || got != want {
			t.Fatalf("status %s got=%d ok=%t", status, got, ok)
		}
	}
	if _, ok := projectStatusToV2(constant.ProjectStatusSet); ok {
		t.Fatal("Set status accepted as on-chain project status")
	}
}

func TestProjectOutstandingLiabilities(t *testing.T) {
	acc := newFundReconciliationAccumulator(fundTestProject())
	acc.sums[fundledger.EntryPurchasePayment] = big.NewInt(1000)
	acc.sums[fundledger.EntryPaymentRefunded] = big.NewInt(100)
	acc.sums[fundledger.EntryProjectProceedsPaid] = big.NewInt(800)
	acc.sums[fundledger.EntryPlatformFeePaid] = big.NewInt(20)
	acc.sums[fundledger.EntryProjectTokenDeposited] = big.NewInt(5000)
	acc.sums[fundledger.EntryProjectTokenClaimed] = big.NewInt(1200)
	acc.sums[fundledger.EntryProjectTokensWithdrawn] = big.NewInt(300)
	payment, project := projectOutstandingLiabilities(acc)
	if payment.String() != "80" || project.String() != "3500" {
		t.Fatalf("unexpected liabilities payment=%s project=%s", payment, project)
	}
}

func TestReconcileProjectOnChainUsesOneBlockAndChecksReserves(t *testing.T) {
	project, acc, tges, chainProject, wallets, participants := onchainFixture()
	project.OnchainLocked = true
	now := time.Now()
	sync := entity.ChainEventSyncState{CursorBlock: 200, CursorBlockHash: common.BigToHash(big.NewInt(999)).Hex(), V2ImplementationAddress: "0x5000000000000000000000000000000000000005", V2ImplementationCodeHash: "0x" + strings.Repeat("a", 64), LastSuccessAt: &now}
	entitlement := entity.IDOPurchaseEntitlement{WalletAddress: wallets[0]}
	reader := &fakeIDOSnapshotReader{
		contract: common.HexToAddress(project.ContractAddress), chainID: new(big.Int).SetUint64(project.ChainID), latest: 205,
		header: &types.Header{Number: big.NewInt(200), Extra: []byte{1}}, project: chainProject, participants: participants,
		purchases:     map[common.Address]baseChain.IDOV2UserPurchaseSnapshot{participants[0]: {PaymentAmount: big.NewInt(1_000_000_000), TokensBought: mustControllerBig("1000000000000000000000"), ClaimedTokens: mustControllerBig("250000000000000000000"), Refunded: false}},
		configs:       map[common.Address]baseChain.IDOV2TokenConfigSnapshot{common.HexToAddress(project.PaymentTokenAddress): {Allowed: true, Decimals: 6}, common.HexToAddress(project.TokenAddress): {Allowed: true, Decimals: 18}},
		reserves:      map[common.Address]*big.Int{common.HexToAddress(project.PaymentTokenAddress): big.NewInt(1_000_000_000), common.HexToAddress(project.TokenAddress): mustControllerBig("10000000000000000000000")},
		balances:      map[common.Address]*big.Int{common.HexToAddress(project.PaymentTokenAddress): big.NewInt(1_000_000_000), common.HexToAddress(project.TokenAddress): mustControllerBig("10000000000000000000000")},
		expectedBlock: big.NewInt(200),
	}
	viper.Set("chainEvent.maxHealthyLagBlocks", 100)
	viper.Set("chain.project.confirmations", "5")
	sync.CursorBlockHash = reader.header.Hash().Hex()
	snapshot := onchainDBSnapshot{project: project, accumulator: acc, entitlements: []entity.IDOPurchaseEntitlement{entitlement}, tges: tges, sync: sync, dbBalanced: true}
	result, err := reconcileProjectOnChain(context.Background(), reader, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Balanced || result.BlockNumber != 200 || len(result.Wallets) != 1 || len(result.TokenReserves) != 2 {
		t.Fatalf("unexpected on-chain reconciliation: %+v", result)
	}
	reader.balances[common.HexToAddress(project.PaymentTokenAddress)] = big.NewInt(999_999_999)
	result, err = reconcileProjectOnChain(context.Background(), reader, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if result.Balanced {
		t.Fatal("globally insolvent payment reserve reported balanced")
	}
}

func TestReconcileProjectOnChainRequiresFreshSafeBlock(t *testing.T) {
	project, acc, tges, chainProject, wallets, participants := onchainFixture()
	project.OnchainLocked = true
	header := &types.Header{Number: big.NewInt(200), Extra: []byte{2}}
	reader := &fakeIDOSnapshotReader{
		contract: common.HexToAddress(project.ContractAddress), chainID: new(big.Int).SetUint64(project.ChainID), latest: 205, header: header,
		project: chainProject, participants: participants,
		purchases: map[common.Address]baseChain.IDOV2UserPurchaseSnapshot{participants[0]: {PaymentAmount: big.NewInt(1_000_000_000), TokensBought: mustControllerBig("1000000000000000000000"), ClaimedTokens: mustControllerBig("250000000000000000000")}},
		configs:   map[common.Address]baseChain.IDOV2TokenConfigSnapshot{}, reserves: map[common.Address]*big.Int{}, balances: map[common.Address]*big.Int{}, expectedBlock: big.NewInt(200),
	}
	now := time.Now()
	snapshot := onchainDBSnapshot{project: project, accumulator: acc, entitlements: []entity.IDOPurchaseEntitlement{{WalletAddress: wallets[0]}}, tges: tges, sync: entity.ChainEventSyncState{CursorBlock: 200, CursorBlockHash: header.Hash().Hex(), V2ImplementationAddress: "0x5000000000000000000000000000000000000005", V2ImplementationCodeHash: "0x" + strings.Repeat("a", 64), LastSuccessAt: &now}}
	viper.Set("chainEvent.maxHealthyLagBlocks", 100)
	viper.Set("chain.project.confirmations", "15")
	if _, err := reconcileProjectOnChain(context.Background(), reader, snapshot); err == nil || !strings.Contains(err.Error(), "fresh RPC safe block") {
		t.Fatalf("unsafe cursor accepted: %v", err)
	}
}

func mustControllerBig(value string) *big.Int {
	parsed, ok := new(big.Int).SetString(value, 10)
	if !ok {
		panic(value)
	}
	return parsed
}

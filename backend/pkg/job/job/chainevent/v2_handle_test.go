package chainevent

import (
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"

	commonChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/fundledger"
)

func TestLifecycleSubmissionIdentityValidation(t *testing.T) {
	valid := "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if !validLifecycleTxHash(valid) {
		t.Fatal("valid tx hash rejected")
	}
	for _, value := range []string{"", "0x1", "0xgggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg", "0x0000000000000000000000000000000000000000000000000000000000000000"} {
		if validLifecycleTxHash(value) {
			t.Fatalf("invalid tx hash accepted: %q", value)
		}
	}
}

func TestValidateV2FeeSnapshot(t *testing.T) {
	receiver := common.HexToAddress("0x8888888888888888888888888888888888888888")
	project := entity.Project{FeeReceiver: receiver.Hex(), SuccessFeeRate: 200}
	if err := validateV2FeeSnapshot(project, receiver, 200); err != nil {
		t.Fatalf("matching fee snapshot rejected: %v", err)
	}
	if err := validateV2FeeSnapshot(project, common.HexToAddress("0x7777777777777777777777777777777777777777"), 200); err == nil {
		t.Fatal("fee receiver drift accepted")
	}
	if err := validateV2FeeSnapshot(project, receiver, 201); err == nil {
		t.Fatal("fee rate drift accepted")
	}
}

func v2ProjectFixture() entity.Project {
	return entity.Project{
		ProjectID: 42, OwnerAddress: "0x5000000000000000000000000000000000000005",
		TokenAddress: "0x4000000000000000000000000000000000000004", TokenSymbol: "SCI", TokenDecimals: 18,
		PaymentToken: constant.PaymentTokenUSDT, PaymentTokenAddress: "0x3000000000000000000000000000000000000003", PaymentTokenDecimals: 6,
		ChainID: 97, ContractAddress: "0x1000000000000000000000000000000000000001", ContractVersion: constant.ContractVersionV2Stablecoin, FundingMode: constant.FundingModeCappedPresale,
		OnchainLocked: true, TokenPriceRaw: "400000", TotalRaisedRaw: "1000000000", TotalRaised: decimal.NewFromInt(1000), SuccessFeeRate: 200,
	}
}

func v2EventFixture() commonChain.V2Event {
	return commonChain.V2Event{
		Event: commonChain.V2Purchased, ContractAddress: common.HexToAddress("0x1000000000000000000000000000000000000001"),
		BlockNumber: 100, BlockHash: "0x" + strings.Repeat("b", 64), TxHash: "0x" + strings.Repeat("a", 64), LogIndex: 2, ProjectID: 42,
		PaymentToken: common.HexToAddress("0x3000000000000000000000000000000000000003"),
		ProjectToken: common.HexToAddress("0x4000000000000000000000000000000000000004"), User: common.HexToAddress("0x2000000000000000000000000000000000000002"),
		PaymentRaw: "1000000", ProjectTokenRaw: "2500000000000000000",
	}
}

func TestBuildV2FundFactUsesProjectTokenSnapshots(t *testing.T) {
	project := v2ProjectFixture()
	event := v2EventFixture()
	now := time.Unix(1_800_000_000, 0)
	fact, err := buildV2FundFact(&project, &event, fundledger.EntryPurchasePayment, fundledger.AssetRolePayment, event.PaymentRaw, event.User.Hex(), event.ContractAddress.Hex(), now)
	if err != nil {
		t.Fatal(err)
	}
	if fact.AmountRaw != "1000000" || fact.AmountDisplay != "1" || fact.TokenDecimals != 6 || fact.TokenAddress != project.PaymentTokenAddress || fact.ContractVersion != constant.ContractVersionV2Stablecoin {
		t.Fatalf("unexpected payment fact: %+v", fact)
	}
	claimFact, err := buildV2FundFact(&project, &event, fundledger.EntryProjectTokenClaimed, fundledger.AssetRoleProject, event.ProjectTokenRaw, event.User.Hex(), event.ContractAddress.Hex(), now)
	if err != nil {
		t.Fatal(err)
	}
	if claimFact.AmountDisplay != "2.5" || claimFact.TokenAddress != project.TokenAddress || claimFact.TokenDecimals != 18 {
		t.Fatalf("unexpected project fact: %+v", claimFact)
	}
}

func TestBuildV2PurchaseEntitlementPreservesBothAssets(t *testing.T) {
	project := v2ProjectFixture()
	event := v2EventFixture()
	now := time.Unix(1_800_000_000, 0)
	fact, err := buildV2PurchaseEntitlement(&project, &event, 99, now)
	if err != nil {
		t.Fatal(err)
	}
	if fact.FundLedgerID != 99 || fact.PaymentAmountRaw != "1000000" || fact.PaymentAmountDisplay != "1" || fact.ProjectTokenAmountRaw != "2500000000000000000" || fact.ProjectTokenDisplay != "2.5" {
		t.Fatalf("unexpected entitlement fact: %+v", fact)
	}
	if fact.PaymentTokenAddress != project.PaymentTokenAddress || fact.ProjectTokenAddress != project.TokenAddress || fact.WalletAddress != event.User.Hex() {
		t.Fatalf("entitlement identity drift: %+v", fact)
	}
}

func TestBuildV2PurchaseEntitlementRejectsFormulaDrift(t *testing.T) {
	project := v2ProjectFixture()
	event := v2EventFixture()
	event.ProjectTokenRaw = "2500000000000000001"
	if _, err := buildV2PurchaseEntitlement(&project, &event, 99, time.Unix(1_800_000_000, 0)); err == nil {
		t.Fatal("purchase entitlement formula drift accepted by event handler")
	}
}

func TestExpectedV2SettlementUsesRawFloor(t *testing.T) {
	project := v2ProjectFixture()
	fee, owner, err := expectedV2Settlement(&project)
	if err != nil {
		t.Fatal(err)
	}
	if fee != "20000000" || owner != "980000000" {
		t.Fatalf("fee=%s owner=%s", fee, owner)
	}
	project.TotalRaisedRaw = "1"
	project.SuccessFeeRate = 200
	fee, owner, err = expectedV2Settlement(&project)
	if err != nil || fee != "0" || owner != "1" {
		t.Fatalf("floor fee=%s owner=%s err=%v", fee, owner, err)
	}
}

func TestValidateV2ProjectSourceRejectsIdentityMismatch(t *testing.T) {
	event := v2EventFixture()
	tests := []struct {
		name   string
		mutate func(*entity.Project)
	}{
		{"contract address", func(project *entity.Project) { project.ContractAddress = "0x9000000000000000000000000000000000000009" }},
		{"fair mode with V2 version", func(project *entity.Project) { project.FundingMode = constant.FundingModeFairLaunch }},
		{"capped mode with Fair Launch version", func(project *entity.Project) { project.ContractVersion = constant.ContractVersionFairLaunchV1 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := v2ProjectFixture()
			test.mutate(&project)
			if err := validateV2ProjectSource(project, &event, 97, event.ContractAddress); err == nil {
				t.Fatal("identity mismatch accepted")
			}
		})
	}
}

func TestValidateV2ProjectSourceAcceptsExactIdentity(t *testing.T) {
	project := v2ProjectFixture()
	event := v2EventFixture()
	if err := validateV2ProjectSource(project, &event, 97, event.ContractAddress); err != nil {
		t.Fatalf("exact V2 identity rejected: %v", err)
	}
}

// 安全审计 MED-1：permissionless 入口事件（end/cancel/platform fee）必须容忍外部
// canonical 交易（无 durable submission 也落库）；operator-gated（create/activate）
// 维持严格匹配。
func TestV2LifecycleFinalizePolicy(t *testing.T) {
	cases := []struct {
		event    string
		action   string
		tolerant bool
	}{
		{commonChain.V2ProjectCreated, constant.TransactionTypeV2Create, false},
		{commonChain.V2ProjectActivated, constant.TransactionTypeV2Activate, false},
		{commonChain.V2ProjectEnded, constant.TransactionTypeProjectEnd, true},
		{commonChain.V2ProjectCancelled, constant.TransactionTypeProjectCancel, true},
		{commonChain.V2PlatformFeeWithdrawn, constant.TransactionTypePlatformFee, true},
	}
	for _, tc := range cases {
		action, known := v2LifecycleFinalizeAction(tc.event)
		if !known || action != tc.action {
			t.Fatalf("%s mapped to (%s,%v), want (%s,true)", tc.event, action, known, tc.action)
		}
		if got := v2LifecycleFinalizeTolerant(tc.event); got != tc.tolerant {
			t.Fatalf("%s tolerant=%v, want %v", tc.event, got, tc.tolerant)
		}
	}
	if action, known := v2LifecycleFinalizeAction(commonChain.V2Purchased); known || action != "" {
		t.Fatal("non-lifecycle event classified as lifecycle finalize")
	}
	if v2LifecycleFinalizeTolerant(commonChain.V2Purchased) {
		t.Fatal("non-lifecycle event classified tolerant")
	}
}

func TestValidateV2LifecycleSubmissionSenderUsesSeparatedCoreOperator(t *testing.T) {
	core := common.HexToAddress("0x1111111111111111111111111111111111111111")
	job := common.HexToAddress("0x2222222222222222222222222222222222222222")
	sender, err := validateV2LifecycleSubmissionSender(job, core)
	if err != nil {
		t.Fatalf("separated signer identities rejected: %v", err)
	}
	if sender != core || sender == job {
		t.Fatalf("lifecycle sender=%s want Core operator=%s, Job signer=%s", sender.Hex(), core.Hex(), job.Hex())
	}
	for _, tc := range []struct {
		name string
		job  common.Address
		core common.Address
	}{
		{"missing job", common.Address{}, core},
		{"missing core", job, common.Address{}},
		{"shared signer", core, core},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := validateV2LifecycleSubmissionSender(tc.job, tc.core); err == nil {
				t.Fatal("invalid signer separation accepted")
			}
		})
	}
}

// 安全审计 HIGH-1：Paused/Unpaused 治理 topic 必须映射到 global_pause/global_unpause。
func TestV2GlobalPauseActionForTopic(t *testing.T) {
	if action, event := v2GlobalPauseActionForTopic(commonChain.V2PausedTopic); action != constant.TransactionTypeGlobalPause || event != "Paused" {
		t.Fatalf("paused topic mapped to (%s,%s)", action, event)
	}
	if action, event := v2GlobalPauseActionForTopic(commonChain.V2UnpausedTopic); action != constant.TransactionTypeGlobalUnpause || event != "Unpaused" {
		t.Fatalf("unpaused topic mapped to (%s,%s)", action, event)
	}
	if action, _ := v2GlobalPauseActionForTopic(common.Hash{}); action != "" {
		t.Fatal("unknown topic accepted as governance pause action")
	}
	if commonChain.V2PausedTopic == commonChain.V2UnpausedTopic {
		t.Fatal("paused and unpaused topics collide")
	}
}

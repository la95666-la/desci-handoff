package controller

import (
	"encoding/json/v2"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/fundledger"
	"joincare_server/pkg/common/money"
	"joincare_server/pkg/common/purchaseentitlement"
)

func fundTestProject() entity.Project {
	return entity.Project{
		ProjectID: 7, NameEn: "Test", Status: constant.ProjectStatusSuccessful,
		OwnerAddress: "0x6000000000000000000000000000000000000006", FeeReceiver: "0x7000000000000000000000000000000000000007",
		ChainID: 97, ContractAddress: "0x1000000000000000000000000000000000000001", ContractVersion: constant.ContractVersionV2Stablecoin, FundingMode: constant.FundingModeCappedPresale,
		PaymentToken: constant.PaymentTokenUSDT, PaymentTokenAddress: "0x2000000000000000000000000000000000000002", PaymentTokenDecimals: 6,
		TokenSymbol: "TST", TokenAddress: "0x3000000000000000000000000000000000000003", TokenDecimals: 18,
		TokenPriceRaw:  "1000000",
		TotalRaisedRaw: "1000000000", TotalRaised: decimal.NewFromInt(1000), TotalDepositedRaw: "10000000000000000000000", TotalDeposited: decimal.NewFromInt(10000),
		ParticipantCount: 1, SuccessFeeRate: 200, FundWithdrawn: 1, PlatformFeeWithdrawn: 1,
	}
}

func fundTestFact(project entity.Project, entryType, raw, wallet string) entity.FundLedger {
	role, symbol, address, decimals := fundledger.AssetRolePayment, project.PaymentToken, project.PaymentTokenAddress, uint8(project.PaymentTokenDecimals)
	if entryType == fundledger.EntryProjectTokenDeposited || entryType == fundledger.EntryProjectTokenClaimed || entryType == fundledger.EntryProjectTokensWithdrawn {
		role, symbol, address, decimals = fundledger.AssetRoleProject, project.TokenSymbol, project.TokenAddress, uint8(project.TokenDecimals)
	}
	display, err := money.FormatRaw(raw, decimals)
	if err != nil {
		panic(err)
	}
	return entity.FundLedger{ProjectID: uint64(project.ProjectID), ChainID: project.ChainID, ContractAddress: project.ContractAddress, ContractVersion: project.ContractVersion, EntryType: entryType, AssetRole: role, TokenSymbol: symbol, TokenAddress: address, TokenDecimals: decimals, AmountRaw: raw, AmountDisplay: display, Status: fundledger.StatusCanonical, WalletAddress: wallet, CounterpartyAddress: project.ContractAddress}
}

func fundTestEntitlement(project entity.Project, parent entity.FundLedger) entity.IDOPurchaseEntitlement {
	payment, _ := new(big.Int).SetString(parent.AmountRaw, 10)
	price, _ := new(big.Int).SetString(project.TokenPriceRaw, 10)
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(project.TokenDecimals)), nil)
	projectRaw := new(big.Int).Quo(new(big.Int).Mul(payment, scale), price).String()
	projectDisplay, err := money.FormatRaw(projectRaw, uint8(project.TokenDecimals))
	if err != nil {
		panic(err)
	}
	return entity.IDOPurchaseEntitlement{
		ID: parent.ID, FundLedgerID: parent.ID, ChainID: project.ChainID, ContractAddress: project.ContractAddress, ContractVersion: project.ContractVersion,
		ProjectID: uint64(project.ProjectID), WalletAddress: parent.WalletAddress,
		PaymentTokenSymbol: project.PaymentToken, PaymentTokenAddress: project.PaymentTokenAddress, PaymentTokenDecimals: uint8(project.PaymentTokenDecimals), PaymentAmountRaw: parent.AmountRaw, PaymentAmountDisplay: parent.AmountDisplay,
		ProjectTokenSymbol: project.TokenSymbol, ProjectTokenAddress: project.TokenAddress, ProjectTokenDecimals: uint8(project.TokenDecimals), ProjectTokenAmountRaw: projectRaw, ProjectTokenDisplay: projectDisplay,
		TxHash: parent.TxHash, LogIndex: parent.LogIndex, BlockNumber: parent.BlockNumber, BlockHash: parent.BlockHash, Status: fundledger.StatusCanonical,
	}
}

func reconciliationResult(project entity.Project, facts ...entity.FundLedger) ([]fundLedgerAggregate, []fundReconciliationCheck, purchaseEntitlementSummary, bool) {
	accumulator := newFundReconciliationAccumulator(project)
	for index, fact := range facts {
		if fact.ID == 0 {
			fact.ID = uint64(index + 1)
		}
		if fact.TxHash == "" {
			fact.TxHash = fmt.Sprintf("0x%064x", index+1)
		}
		fact.LogIndex = uint(index)
		fact.BlockNumber = uint64(100 + index)
		if fact.BlockHash == "" {
			fact.BlockHash = fmt.Sprintf("0x%064x", 100+index)
		}
		accumulator.Add(fact)
		if fact.EntryType == fundledger.EntryPurchasePayment && fact.Status == fundledger.StatusCanonical {
			accumulator.AddEntitlement(fundTestEntitlement(project, fact))
		}
	}
	aggregates, checks, summary := accumulator.Result()
	balanced := true
	for _, check := range checks {
		if !check.OK {
			balanced = false
		}
	}
	return aggregates, checks, summary, balanced
}

func TestFundReconciliationBalancedSuccess(t *testing.T) {
	project := fundTestProject()
	wallet := "0x4000000000000000000000000000000000000004"
	aggregates, checks, summary, balanced := reconciliationResult(project,
		fundTestFact(project, fundledger.EntryProjectTokenDeposited, project.TotalDepositedRaw, project.OwnerAddress),
		fundTestFact(project, fundledger.EntryPurchasePayment, "400000000", wallet),
		fundTestFact(project, fundledger.EntryPurchasePayment, "600000000", wallet),
		fundTestFact(project, fundledger.EntryProjectProceedsPaid, "980000000", project.OwnerAddress),
		fundTestFact(project, fundledger.EntryPlatformFeePaid, "20000000", project.FeeReceiver),
	)
	if !balanced {
		t.Fatalf("balanced fixture failed: %+v", checks)
	}
	if got := aggregateByType(aggregates, fundledger.EntryPurchasePayment); got.AmountRaw != project.TotalRaisedRaw || got.AmountDisplay != "1000" || got.Count != 2 {
		t.Fatalf("purchase aggregate mismatch: %+v", got)
	}
	if summary.TotalTokensSoldRaw != "1000000000000000000000" || summary.TotalTokensSoldDisplay != "1000" || summary.Count != 2 {
		t.Fatalf("entitlement summary mismatch: %+v", summary)
	}
}

func TestFundReconciliationRejectsInvalidFacts(t *testing.T) {
	project := fundTestProject()
	project.FundWithdrawn, project.PlatformFeeWithdrawn = 0, 0
	base := []entity.FundLedger{
		fundTestFact(project, fundledger.EntryProjectTokenDeposited, project.TotalDepositedRaw, project.OwnerAddress),
		fundTestFact(project, fundledger.EntryPurchasePayment, project.TotalRaisedRaw, "0x4000000000000000000000000000000000000004"),
	}
	tests := []struct {
		name   string
		mutate func([]entity.FundLedger) []entity.FundLedger
	}{
		{name: "refund exceeds purchase", mutate: func(facts []entity.FundLedger) []entity.FundLedger {
			return append(facts, fundTestFact(project, fundledger.EntryPaymentRefunded, "1000000001", facts[1].WalletAddress))
		}},
		{name: "reconciliation status", mutate: func(facts []entity.FundLedger) []entity.FundLedger {
			facts[1].Status = "reconciliation"
			return facts
		}},
		{name: "token identity drift", mutate: func(facts []entity.FundLedger) []entity.FundLedger {
			facts[1].TokenDecimals = 18
			return facts
		}},
		{name: "wallet ownership drift", mutate: func(facts []entity.FundLedger) []entity.FundLedger {
			facts[1].WalletAddress = ""
			return facts
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			facts := append([]entity.FundLedger(nil), base...)
			_, checks, _, balanced := reconciliationResult(project, test.mutate(facts)...)
			if balanced {
				t.Fatalf("invalid fixture reported balanced: %+v", checks)
			}
		})
	}
}

func TestFundProjectIdentityRejectsImpossibleFeeRate(t *testing.T) {
	project := fundTestProject()
	project.SuccessFeeRate = 1001
	if validFundProjectIdentity(project) {
		t.Fatal("project fee rate above JoinCareIDOv2.MAX_FEE_RATE accepted")
	}
}

func TestPurchaseEntitlementFormulaUsesContractFloor(t *testing.T) {
	if !purchaseentitlement.MatchesPurchaseFormula("1000001", "1000001000000000000", "1000000", 18) {
		t.Fatal("exact contract purchase formula rejected")
	}
	if purchaseentitlement.MatchesPurchaseFormula("1000001", "1000001000000000001", "1000000", 18) {
		t.Fatal("project-token amount drift accepted")
	}
	if !purchaseentitlement.MatchesPurchaseFormula("1", "333333333333", "3000000", 18) {
		t.Fatal("Solidity floor formula rejected")
	}
	if !purchaseentitlement.MatchesPurchaseFormula("1000000000000000000", "500000", "2000000000000000000", 6) {
		t.Fatal("project token decimals below payment decimals rejected")
	}
}

func TestFundReconciliationBalancedFailedProject(t *testing.T) {
	project := fundTestProject()
	project.Status = constant.ProjectStatusFailed
	project.FundWithdrawn, project.PlatformFeeWithdrawn, project.TokenWithdrawn = 0, 0, 1
	wallet := "0x4000000000000000000000000000000000000004"
	_, checks, _, balanced := reconciliationResult(project,
		fundTestFact(project, fundledger.EntryProjectTokenDeposited, project.TotalDepositedRaw, project.OwnerAddress),
		fundTestFact(project, fundledger.EntryPurchasePayment, project.TotalRaisedRaw, wallet),
		fundTestFact(project, fundledger.EntryPaymentRefunded, project.TotalRaisedRaw, wallet),
		fundTestFact(project, fundledger.EntryProjectTokensWithdrawn, project.TotalDepositedRaw, project.OwnerAddress),
	)
	if !balanced {
		t.Fatalf("failed project fixture should balance: %+v", checks)
	}
}

func TestFundReconciliationFailedProjectRequiresCompleteRefunds(t *testing.T) {
	project := fundTestProject()
	project.Status = constant.ProjectStatusFailed
	project.FundWithdrawn, project.PlatformFeeWithdrawn = 0, 0
	wallet := "0x4000000000000000000000000000000000000004"
	_, checks, _, balanced := reconciliationResult(project,
		fundTestFact(project, fundledger.EntryProjectTokenDeposited, project.TotalDepositedRaw, project.OwnerAddress),
		fundTestFact(project, fundledger.EntryPurchasePayment, project.TotalRaisedRaw, wallet),
	)
	if balanced {
		t.Fatalf("failed project with outstanding refunds reported balanced: %+v", checks)
	}
}

func TestFundReconciliationWithdrawalFailsClosed(t *testing.T) {
	tests := []struct {
		name   string
		status string
		raw    string
	}{
		{name: "active withdrawal", status: constant.ProjectStatusActive, raw: "1"},
		{name: "failed partial withdrawal", status: constant.ProjectStatusFailed, raw: "1"},
		{name: "successful withdrawal needs sold-token evidence", status: constant.ProjectStatusSuccessful, raw: "1"},
		{name: "active zero withdrawal event", status: constant.ProjectStatusActive, raw: "0"},
		{name: "successful zero withdrawal still needs evidence", status: constant.ProjectStatusSuccessful, raw: "0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := fundTestProject()
			project.Status = test.status
			project.FundWithdrawn, project.PlatformFeeWithdrawn, project.TokenWithdrawn = 0, 0, 1
			_, checks, _, balanced := reconciliationResult(project,
				fundTestFact(project, fundledger.EntryProjectTokenDeposited, project.TotalDepositedRaw, project.OwnerAddress),
				fundTestFact(project, fundledger.EntryPurchasePayment, project.TotalRaisedRaw, "0x4000000000000000000000000000000000000004"),
				fundTestFact(project, fundledger.EntryProjectTokensWithdrawn, test.raw, project.OwnerAddress),
			)
			if balanced {
				t.Fatalf("withdrawal fixture must fail closed: %+v", checks)
			}
		})
	}
}

func TestFundReconciliationSuccessfulEntitlementsBalanceClaimsAndWithdrawal(t *testing.T) {
	project := fundTestProject()
	project.TokenWithdrawn = 1
	wallet := "0x4000000000000000000000000000000000000004"
	_, checks, summary, balanced := reconciliationResult(project,
		fundTestFact(project, fundledger.EntryProjectTokenDeposited, project.TotalDepositedRaw, project.OwnerAddress),
		fundTestFact(project, fundledger.EntryPurchasePayment, "400000000", wallet),
		fundTestFact(project, fundledger.EntryPurchasePayment, "600000000", wallet),
		fundTestFact(project, fundledger.EntryProjectTokenClaimed, "400000000000000000000", wallet),
		fundTestFact(project, fundledger.EntryProjectTokenClaimed, "100000000000000000000", wallet),
		fundTestFact(project, fundledger.EntryProjectProceedsPaid, "980000000", project.OwnerAddress),
		fundTestFact(project, fundledger.EntryPlatformFeePaid, "20000000", project.FeeReceiver),
		fundTestFact(project, fundledger.EntryProjectTokensWithdrawn, "9000000000000000000000", project.OwnerAddress),
	)
	if !balanced {
		t.Fatalf("successful entitlement fixture should balance: %+v", checks)
	}
	if summary.TotalTokensSoldRaw != "1000000000000000000000" {
		t.Fatalf("unexpected sold summary: %+v", summary)
	}
}

func TestFundReconciliationSuccessfulZeroWithdrawalUsesEventPresence(t *testing.T) {
	project := fundTestProject()
	project.TotalDepositedRaw = "1000000000000000000000"
	project.TotalDeposited = decimal.NewFromInt(1000)
	project.TokenWithdrawn = 1
	wallet := "0x4000000000000000000000000000000000000004"
	_, checks, _, balanced := reconciliationResult(project,
		fundTestFact(project, fundledger.EntryProjectTokenDeposited, project.TotalDepositedRaw, project.OwnerAddress),
		fundTestFact(project, fundledger.EntryPurchasePayment, project.TotalRaisedRaw, wallet),
		fundTestFact(project, fundledger.EntryProjectProceedsPaid, "980000000", project.OwnerAddress),
		fundTestFact(project, fundledger.EntryPlatformFeePaid, "20000000", project.FeeReceiver),
		fundTestFact(project, fundledger.EntryProjectTokensWithdrawn, "0", project.OwnerAddress),
	)
	if !balanced {
		t.Fatalf("zero unsold-token withdrawal event should balance: %+v", checks)
	}
}

func TestFundReconciliationEntitlementFailsClosed(t *testing.T) {
	project := fundTestProject()
	project.FundWithdrawn, project.PlatformFeeWithdrawn = 0, 0
	wallet := "0x4000000000000000000000000000000000000004"
	purchase := fundTestFact(project, fundledger.EntryPurchasePayment, project.TotalRaisedRaw, wallet)
	purchase.ID, purchase.TxHash, purchase.BlockHash, purchase.BlockNumber = 9, "0x"+strings.Repeat("a", 64), "0x"+strings.Repeat("b", 64), 100
	tests := []struct {
		name   string
		mutate func(*entity.IDOPurchaseEntitlement)
		add    bool
	}{
		{name: "missing entitlement", add: false},
		{name: "payment mismatch", add: true, mutate: func(f *entity.IDOPurchaseEntitlement) { f.PaymentAmountRaw = "999999999" }},
		{name: "wallet mismatch", add: true, mutate: func(f *entity.IDOPurchaseEntitlement) { f.WalletAddress = "0x5000000000000000000000000000000000000005" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			acc := newFundReconciliationAccumulator(project)
			acc.Add(fundTestFact(project, fundledger.EntryProjectTokenDeposited, project.TotalDepositedRaw, project.OwnerAddress))
			acc.Add(purchase)
			if test.add {
				entitlement := fundTestEntitlement(project, purchase)
				if test.mutate != nil {
					test.mutate(&entitlement)
				}
				acc.AddEntitlement(entitlement)
			}
			_, checks, _ := acc.Result()
			if allChecksOK(checks) {
				t.Fatalf("invalid entitlement fixture reported balanced: %+v", checks)
			}
		})
	}
}

func TestFundReconciliationRejectsMalformedEventIdentity(t *testing.T) {
	project := fundTestProject()
	project.FundWithdrawn, project.PlatformFeeWithdrawn = 0, 0
	wallet := "0x4000000000000000000000000000000000000004"
	for _, mutate := range []func(*entity.FundLedger){
		func(f *entity.FundLedger) { f.BlockNumber = 0 },
		func(f *entity.FundLedger) { f.TxHash = "" },
		func(f *entity.FundLedger) { f.BlockHash = "0x01" },
	} {
		acc := newFundReconciliationAccumulator(project)
		deposit := fundTestFact(project, fundledger.EntryProjectTokenDeposited, project.TotalDepositedRaw, project.OwnerAddress)
		deposit.ID, deposit.TxHash, deposit.BlockNumber, deposit.BlockHash = 1, "0x"+strings.Repeat("a", 64), 100, "0x"+strings.Repeat("b", 64)
		acc.Add(deposit)
		purchase := fundTestFact(project, fundledger.EntryPurchasePayment, project.TotalRaisedRaw, wallet)
		purchase.ID, purchase.TxHash, purchase.BlockNumber, purchase.BlockHash = 2, "0x"+strings.Repeat("c", 64), 101, "0x"+strings.Repeat("d", 64)
		mutate(&purchase)
		acc.Add(purchase)
		_, checks, _ := acc.Result()
		if allChecksOK(checks) {
			t.Fatalf("malformed event identity reported balanced: %+v", checks)
		}
	}
}

func TestFundReconciliationClaimCannotExceedWalletEntitlement(t *testing.T) {
	project := fundTestProject()
	project.FundWithdrawn, project.PlatformFeeWithdrawn = 0, 0
	wallet := "0x4000000000000000000000000000000000000004"
	_, checks, _, balanced := reconciliationResult(project,
		fundTestFact(project, fundledger.EntryProjectTokenDeposited, project.TotalDepositedRaw, project.OwnerAddress),
		fundTestFact(project, fundledger.EntryPurchasePayment, project.TotalRaisedRaw, wallet),
		fundTestFact(project, fundledger.EntryProjectTokenClaimed, "1000000000000000000001", wallet),
	)
	if balanced {
		t.Fatalf("claim above wallet entitlement reported balanced: %+v", checks)
	}
}

func TestFundReconciliationZeroAmountEventsRespectStatus(t *testing.T) {
	project := fundTestProject()
	project.Status = constant.ProjectStatusActive
	project.FundWithdrawn, project.PlatformFeeWithdrawn = 0, 1
	_, checks, _, balanced := reconciliationResult(project,
		fundTestFact(project, fundledger.EntryProjectTokenDeposited, project.TotalDepositedRaw, project.OwnerAddress),
		fundTestFact(project, fundledger.EntryPurchasePayment, project.TotalRaisedRaw, "0x4000000000000000000000000000000000000004"),
		fundTestFact(project, fundledger.EntryPlatformFeePaid, "0", project.FeeReceiver),
	)
	if balanced {
		t.Fatalf("active zero-amount settlement event must fail closed: %+v", checks)
	}
}

func TestFundReconciliationWalletOwnershipFailsClosed(t *testing.T) {
	project := fundTestProject()
	project.Status = constant.ProjectStatusFailed
	project.FundWithdrawn, project.PlatformFeeWithdrawn = 0, 0
	walletA := "0x4000000000000000000000000000000000000004"
	walletB := "0x5000000000000000000000000000000000000005"
	tests := []struct {
		name  string
		facts []entity.FundLedger
	}{
		{name: "refund wallet has no purchase", facts: []entity.FundLedger{
			fundTestFact(project, fundledger.EntryProjectTokenDeposited, project.TotalDepositedRaw, project.OwnerAddress),
			fundTestFact(project, fundledger.EntryPurchasePayment, project.TotalRaisedRaw, walletA),
			fundTestFact(project, fundledger.EntryPaymentRefunded, project.TotalRaisedRaw, walletB),
		}},
		{name: "wallet refund exceeds own purchase", facts: []entity.FundLedger{
			fundTestFact(project, fundledger.EntryProjectTokenDeposited, project.TotalDepositedRaw, project.OwnerAddress),
			fundTestFact(project, fundledger.EntryPurchasePayment, "400000000", walletA),
			fundTestFact(project, fundledger.EntryPurchasePayment, "600000000", walletB),
			fundTestFact(project, fundledger.EntryPaymentRefunded, "500000000", walletA),
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, checks, _, balanced := reconciliationResult(project, test.facts...)
			if balanced {
				t.Fatalf("wallet ownership fixture must fail: %+v", checks)
			}
		})
	}

	project.Status = constant.ProjectStatusSuccessful
	project.FundWithdrawn, project.PlatformFeeWithdrawn = 1, 1
	_, checks, _, balanced := reconciliationResult(project,
		fundTestFact(project, fundledger.EntryProjectTokenDeposited, project.TotalDepositedRaw, project.OwnerAddress),
		fundTestFact(project, fundledger.EntryPurchasePayment, project.TotalRaisedRaw, walletA),
		fundTestFact(project, fundledger.EntryProjectProceedsPaid, "980000000", project.OwnerAddress),
		fundTestFact(project, fundledger.EntryPlatformFeePaid, "20000000", project.FeeReceiver),
		fundTestFact(project, fundledger.EntryProjectTokenClaimed, "1", walletB),
	)
	if balanced {
		t.Fatalf("claim without wallet purchase/evidence must fail: %+v", checks)
	}
}

func TestFundReconciliationAggregateUsesArbitraryPrecision(t *testing.T) {
	project := fundTestProject()
	max := "115792089237316195423570985008687907853269984665640564039457584007913129639935"
	project.TotalRaisedRaw = newBigIntSum(max, max)
	project.TotalRaised = decimal.Zero
	project.TotalDepositedRaw = "0"
	project.TotalDeposited = decimal.Zero
	project.SuccessFeeRate = 0
	project.FundWithdrawn = 0
	acc := newFundReconciliationAccumulator(project)
	for index, wallet := range []string{"0x4000000000000000000000000000000000000004", "0x5000000000000000000000000000000000000005"} {
		fact := fundTestFact(project, fundledger.EntryPurchasePayment, max, wallet)
		fact.ID, fact.TxHash, fact.BlockNumber, fact.BlockHash = uint64(index+1), fmt.Sprintf("0x%064x", index+1), uint64(100+index), fmt.Sprintf("0x%064x", 100+index)
		acc.Add(fact)
	}
	aggregates, _, _ := acc.Result()
	got := aggregateByType(aggregates, fundledger.EntryPurchasePayment)
	if got.AmountRaw != project.TotalRaisedRaw || strings.Contains(got.AmountDisplay, "e") {
		t.Fatalf("arbitrary precision aggregate mismatch: %+v", got)
	}
}

func TestFundLedgerJSONNeverLeaksEventData(t *testing.T) {
	for name, value := range map[string]any{
		"fund ledger": entity.FundLedger{EventData: `{"secret":"raw-event-payload"}`},
		"entitlement": entity.IDOPurchaseEntitlement{EventData: `{"secret":"raw-event-payload"}`},
	} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		text := string(encoded)
		for _, forbidden := range []string{"event_data", "raw-event-payload"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("serialized %s leaked %q: %s", name, forbidden, text)
			}
		}
	}
}

func aggregateByType(items []fundLedgerAggregate, entryType string) fundLedgerAggregate {
	for _, item := range items {
		if item.EntryType == entryType {
			return item
		}
	}
	return fundLedgerAggregate{}
}

func allChecksOK(checks []fundReconciliationCheck) bool {
	for _, check := range checks {
		if !check.OK {
			return false
		}
	}
	return true
}

func newBigIntSum(left, right string) string {
	a, _ := new(big.Int).SetString(left, 10)
	b, _ := new(big.Int).SetString(right, 10)
	return new(big.Int).Add(a, b).String()
}

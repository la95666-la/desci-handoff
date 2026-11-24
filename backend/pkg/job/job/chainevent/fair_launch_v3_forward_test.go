package chainevent

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/entity"
)

func TestBuildFairLaunchV3ContributionPlan(t *testing.T) {
	binding, project := fairLaunchV3Fixture()
	event := fairLaunchV3Event("FairLaunchContribution", 102)
	event.Indexed["arg1"] = common.HexToAddress("0x1111111111111111111111111111111111111111")
	event.Indexed["arg2"] = common.HexToAddress(binding.PaymentTokenAddress)
	event.RawDecimal["arg3"] = "1000000"
	event.RawDecimal["arg4"] = "1000000"
	event.RawDecimal["arg5"] = "1000000"
	effect := FairLaunchMaterializationResult{EventName: event.Name, EffectDigest: testDigest("effect"), ProjectChanged: true, PositionChanged: true}
	plan, err := buildFairLaunchV3ForwardPlan(event, binding, project, effect, testDigest("payload"))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Ledger) != 1 || plan.Ledger[0].EntryType != "payment_contributed" || plan.Ledger[0].AmountDisplay != "1" || plan.Project.TotalRaisedRaw != "1000000" || plan.Position == nil {
		t.Fatalf("unexpected contribution plan: %#v", plan)
	}
}

func TestBuildFairLaunchV3RejectsCumulativeAmountDrift(t *testing.T) {
	binding, project := fairLaunchV3Fixture()
	deposit := fairLaunchV3Event("FairLaunchTokensDeposited", 102)
	deposit.Indexed["arg1"] = common.HexToAddress(binding.ProjectTokenAddress)
	deposit.Indexed["arg2"] = common.HexToAddress("0x1111111111111111111111111111111111111111")
	deposit.RawDecimal["arg3"] = "10"
	deposit.RawDecimal["arg4"] = "11"
	if _, err := buildFairLaunchV3ForwardPlan(deposit, binding, project, FairLaunchMaterializationResult{EventName: deposit.Name, EffectDigest: testDigest("effect")}, testDigest("payload")); err == nil {
		t.Fatal("deposit cumulative drift was accepted")
	}
	contribution := fairLaunchV3Event("FairLaunchContribution", 102)
	contribution.Indexed["arg1"] = common.HexToAddress("0x1111111111111111111111111111111111111111")
	contribution.Indexed["arg2"] = common.HexToAddress(binding.PaymentTokenAddress)
	contribution.RawDecimal["arg3"] = "10"
	contribution.RawDecimal["arg4"] = "10"
	contribution.RawDecimal["arg5"] = "11"
	if _, err := buildFairLaunchV3ForwardPlan(contribution, binding, project, FairLaunchMaterializationResult{EventName: contribution.Name, EffectDigest: testDigest("effect")}, testDigest("payload")); err == nil {
		t.Fatal("contribution total-raised drift was accepted")
	}
}

func TestBuildFairLaunchV3RejectsEndedPaymentTokenMismatch(t *testing.T) {
	binding, project := fairLaunchV3Fixture()
	event := fairLaunchV3Event("FairLaunchEnded", 102)
	event.RawDecimal["arg1"], event.RawDecimal["arg2"] = "4", "1"
	event.Indexed["arg3"] = common.HexToAddress("0x9999999999999999999999999999999999999999")
	event.RawDecimal["arg4"], event.RawDecimal["arg5"], event.RawDecimal["arg6"] = "0", "0", "0"
	event.RawDecimal["arg7"], event.RawDecimal["arg8"] = "50", "49"
	event.Values["arg9"] = common.HexToHash("0x49")
	if _, err := buildFairLaunchV3ForwardPlan(event, binding, project, FairLaunchMaterializationResult{EventName: event.Name, EffectDigest: testDigest("effect")}, testDigest("payload")); err == nil {
		t.Fatal("ended payment-token mismatch was accepted")
	}
	deposit := fairLaunchV3Event("FairLaunchTokensDeposited", 102)
	deposit.Indexed["arg1"] = common.HexToAddress(binding.PaymentTokenAddress)
	deposit.Indexed["arg2"] = common.HexToAddress("0x1111111111111111111111111111111111111111")
	deposit.RawDecimal["arg3"], deposit.RawDecimal["arg4"] = "100", "100"
	if _, err := buildFairLaunchV3ForwardPlan(deposit, binding, project, FairLaunchMaterializationResult{EventName: deposit.Name, EffectDigest: testDigest("effect")}, testDigest("payload")); err == nil {
		t.Fatal("deposit payment-token role was accepted")
	}
}

func TestBuildFairLaunchV3RejectsFinancialConservationMismatch(t *testing.T) {
	binding, project := fairLaunchV3Fixture()
	project.Status = "Pending"
	project.TotalTokensDepositedRaw = "999"
	project.SaleTokenAmountRaw = "1000"
	activated := fairLaunchV3Event("FairLaunchActivated", 102)
	if _, err := buildFairLaunchV3ForwardPlan(activated, binding, project, FairLaunchMaterializationResult{EventName: activated.Name, EffectDigest: testDigest("effect")}, testDigest("payload")); err == nil {
		t.Fatal("activation before full deposit was accepted")
	}
	project.Status = "Successful"
	finalRaised := "400"
	project.FinalTotalRaisedRaw = &finalRaised
	project.PlatformFeeReservedRaw = "10"
	project.ProjectProceedsWithdrawn = true
	project.ProjectProceedsPaidRaw = "390"
	proceeds := fairLaunchV3Event("FairLaunchProjectProceedsWithdrawn", 102)
	proceeds.Indexed["arg1"] = common.HexToAddress(binding.PaymentTokenAddress)
	proceeds.Indexed["arg2"] = common.HexToAddress("0x1111111111111111111111111111111111111111")
	proceeds.RawDecimal["arg3"], proceeds.RawDecimal["arg4"] = "389", "10"
	if _, err := buildFairLaunchV3ForwardPlan(proceeds, binding, project, FairLaunchMaterializationResult{EventName: proceeds.Name, EffectDigest: testDigest("effect")}, testDigest("payload")); err == nil {
		t.Fatal("project proceeds conservation mismatch was accepted")
	}
	fee := fairLaunchV3Event("FairLaunchPlatformFeeWithdrawn", 102)
	fee.Indexed["arg1"] = common.HexToAddress(binding.PaymentTokenAddress)
	fee.Indexed["arg2"] = common.HexToAddress("0x1111111111111111111111111111111111111111")
	fee.RawDecimal["arg3"] = "9"
	if _, err := buildFairLaunchV3ForwardPlan(fee, binding, project, FairLaunchMaterializationResult{EventName: fee.Name, EffectDigest: testDigest("effect")}, testDigest("payload")); err == nil {
		t.Fatal("platform fee reserve mismatch was accepted")
	}
	project.Status = "Cancelled"
	project.TotalTokensDepositedRaw = "1000"
	withdraw := fairLaunchV3Event("FairLaunchFailedProjectTokensWithdrawn", 102)
	withdraw.Indexed["arg1"] = common.HexToAddress(binding.ProjectTokenAddress)
	withdraw.Indexed["arg2"] = common.HexToAddress("0x1111111111111111111111111111111111111111")
	withdraw.RawDecimal["arg3"] = "999"
	if _, err := buildFairLaunchV3ForwardPlan(withdraw, binding, project, FairLaunchMaterializationResult{EventName: withdraw.Name, EffectDigest: testDigest("effect")}, testDigest("payload")); err == nil {
		t.Fatal("failed token withdrawal mismatch was accepted")
	}
}

func TestBuildFairLaunchV3AllowsEitherSuccessfulWithdrawalOrder(t *testing.T) {
	binding, project := fairLaunchV3Fixture()
	project.Status = "Successful"
	finalRaised := "400"
	project.FinalTotalRaisedRaw = &finalRaised
	fee := fairLaunchV3Event("FairLaunchPlatformFeeWithdrawn", 102)
	fee.Indexed["arg1"] = common.HexToAddress(binding.PaymentTokenAddress)
	fee.Indexed["arg2"] = common.HexToAddress("0x1111111111111111111111111111111111111111")
	fee.RawDecimal["arg3"] = "10"
	feePlan, err := buildFairLaunchV3ForwardPlan(fee, binding, project, FairLaunchMaterializationResult{EventName: fee.Name, EffectDigest: testDigest("effect")}, testDigest("payload"))
	if err != nil || !feePlan.Project.PlatformFeeWithdrawn || feePlan.Project.PlatformFeePaidRaw != "10" || feePlan.Project.PlatformFeeReservedRaw != "10" {
		t.Fatalf("fee-first withdrawal rejected: plan=%+v err=%v", feePlan.Project, err)
	}
	proceeds := fairLaunchV3Event("FairLaunchProjectProceedsWithdrawn", 102)
	proceeds.Indexed["arg1"] = common.HexToAddress(binding.PaymentTokenAddress)
	proceeds.Indexed["arg2"] = common.HexToAddress("0x2222222222222222222222222222222222222222")
	proceeds.RawDecimal["arg3"], proceeds.RawDecimal["arg4"] = "390", "10"
	proceedsPlan, err := buildFairLaunchV3ForwardPlan(proceeds, binding, feePlan.Project, FairLaunchMaterializationResult{EventName: proceeds.Name, EffectDigest: testDigest("effect")}, testDigest("payload"))
	if err != nil || !proceedsPlan.Project.ProjectProceedsWithdrawn || proceedsPlan.Project.ProjectProceedsPaidRaw != "390" {
		t.Fatalf("proceeds after fee rejected: plan=%+v err=%v", proceedsPlan.Project, err)
	}
}

func TestBuildFairLaunchV3RejectsCancelPreviousStatusMismatch(t *testing.T) {
	binding, project := fairLaunchV3Fixture()
	event := fairLaunchV3Event("FairLaunchCancelled", 102)
	event.RawDecimal["arg1"] = "0"
	event.Indexed["arg4"] = common.HexToAddress("0x1111111111111111111111111111111111111111")
	if _, err := buildFairLaunchV3ForwardPlan(event, binding, project, FairLaunchMaterializationResult{EventName: event.Name, EffectDigest: testDigest("effect")}, testDigest("payload")); err == nil {
		t.Fatal("cancel previous-status mismatch was accepted")
	}
}

func TestBuildFairLaunchV3EndedFailedPlan(t *testing.T) {
	binding, project := fairLaunchV3Fixture()
	project.TotalRaisedRaw = "50000000"
	event := fairLaunchV3Event("FairLaunchEnded", 101)
	event.RawDecimal["arg1"] = "4"
	event.RawDecimal["arg2"] = "1"
	event.Indexed["arg3"] = common.HexToAddress(binding.PaymentTokenAddress)
	event.RawDecimal["arg4"] = "50000000"
	event.RawDecimal["arg5"] = "0"
	event.RawDecimal["arg6"] = "0"
	event.RawDecimal["arg7"] = "50"
	event.RawDecimal["arg8"] = "49"
	event.Values["arg9"] = common.HexToHash("0x49")
	event.Log.BlockHash = common.HexToHash("0x50")
	plan, err := buildFairLaunchV3ForwardPlan(event, binding, project, FairLaunchMaterializationResult{EventName: event.Name, EffectDigest: testDigest("effect")}, testDigest("payload"))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Project.Status != "Failed" || plan.Project.TerminalReason != "soft_cap_not_met" || plan.Project.FinalTotalRaisedRaw == nil || *plan.Project.FinalTotalRaisedRaw != "50000000" || plan.Project.FinalPriceDenominatorRaw != nil {
		t.Fatalf("unexpected end plan: %#v", plan.Project)
	}
}

func TestBuildFairLaunchV3RejectsClaimBeforeRelease(t *testing.T) {
	binding, project := fairLaunchV3Fixture()
	event := fairLaunchV3Event("FairLaunchClaimed", 102)
	event.Indexed["arg1"] = common.HexToAddress("0x1111111111111111111111111111111111111111")
	event.Indexed["arg2"] = common.HexToAddress(binding.ProjectTokenAddress)
	event.RawDecimal["arg3"] = "1"
	event.RawDecimal["arg4"] = "1"
	if _, err := buildFairLaunchV3ForwardPlan(event, binding, project, FairLaunchMaterializationResult{EventName: event.Name, EffectDigest: testDigest("effect")}, testDigest("payload")); err == nil {
		t.Fatal("claim before successful release was accepted")
	}
}

func TestBuildFairLaunchV3SuccessfulReleasePlan(t *testing.T) {
	binding, project := fairLaunchV3Fixture()
	project.Status = "SuccessfulPendingRelease"
	blockNumber, anchorNumber := uint64(50), uint64(49)
	anchorHash := strings.ToLower(common.HexToHash("0x49").Hex())
	project.SettlementBlockNumber = &blockNumber
	project.SettlementAnchorBlockNumber = &anchorNumber
	project.SettlementAnchorBlockHash = &anchorHash
	releasePolicyDigest := strings.Repeat("5", 64)
	releasePolicyVersion := uint32(1)
	project.ReleasePolicyDigest = &releasePolicyDigest
	project.ReleasePolicyVersion = &releasePolicyVersion
	event := fairLaunchV3Event("FairLaunchSuccessfulReleased", 102)
	event.Indexed["arg1"] = new(big.Int).SetUint64(blockNumber)
	event.Indexed["arg2"] = common.HexToAddress("0x1111111111111111111111111111111111111111")
	event.RawDecimal["arg1"] = "50"
	event.RawDecimal["arg3"] = "49"
	event.RawDecimal["arg6"] = "0"
	event.RawDecimal["arg9"] = "1"
	event.Values["arg3"] = new(big.Int).SetUint64(anchorNumber)
	event.Values["arg4"] = common.HexToHash(anchorHash)
	event.Values["arg5"] = common.HexToHash("0x51")
	event.Values["arg6"] = uint32(0)
	event.Values["arg7"] = common.HexToHash("0x52")
	event.Values["arg8"] = common.HexToHash("0x" + releasePolicyDigest)
	event.Values["arg9"] = uint32(1)
	effect, err := MaterializeFairLaunchEvent(event, FairLaunchMaterializationContext{ChainID: 31337, SourceRevisionID: 1, EffectVersion: FairLaunchEffectManifestVersion, Canonical: true, Finalized: true})
	if err != nil || effect.PolicyChanged || !effect.ProjectChanged {
		t.Fatalf("unexpected release effect: %#v err=%v", effect, err)
	}
	plan, err := buildFairLaunchV3ForwardPlan(event, binding, project, effect, testDigest("payload"))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Project.Status != "Successful" || plan.Project.ReleasePolicyVersion == nil || *plan.Project.ReleasePolicyVersion != 1 {
		t.Fatalf("unexpected release plan: %#v", plan.Project)
	}
	event.RawDecimal["arg3"] = "48"
	if _, err := buildFairLaunchV3ForwardPlan(event, binding, project, effect, testDigest("payload")); err == nil {
		t.Fatal("mismatched release anchor was accepted")
	}
	event.RawDecimal["arg3"] = "49"
	event.Values["arg8"] = common.HexToHash("0x53")
	if _, err := buildFairLaunchV3ForwardPlan(event, binding, project, effect, testDigest("payload")); err == nil {
		t.Fatal("mismatched release policy was accepted")
	}
}

func TestFairLaunchLocalForwardSetExcludesGovernance(t *testing.T) {
	for _, event := range []string{"FairLaunchContributionsPaused", "FairLaunchOperatorUpdated", "FairLaunchPaymentTokenAdmissionUpdated", "FairLaunchExcessTokensRecovered"} {
		if fairLaunchLocalForwardEvents[event] {
			t.Fatalf("unsupported governance event enabled: %s", event)
		}
	}
}

func TestRawArgRejectsMissingAndMalformed(t *testing.T) {
	event := &commonchain.FairLaunchTypedEvent{RawDecimal: map[string]string{"bad": "01x"}}
	if rawArg(event, "missing") != "" || rawArg(event, "bad") != "" {
		t.Fatal("invalid raw argument accepted")
	}
}

func TestFairLaunchProjectionDigestValidation(t *testing.T) {
	_, project := fairLaunchV3Fixture()
	_, digest := fairLaunchV3Payload(fairLaunchProjectStatePayload(project))
	project.StateDigest = digest
	if !fairLaunchProjectStateDigestValid(project) {
		_, expected := fairLaunchV3Payload(fairLaunchProjectStatePayload(project))
		t.Fatalf("valid project digest was rejected state=%q expected=%q len=%d", project.StateDigest, expected, len(project.StateDigest))
	}
	project.TotalRaisedRaw = "1"
	if fairLaunchProjectStateDigestValid(project) {
		t.Fatal("tampered project projection was accepted")
	}
	position := entity.FairLaunchPositionV3{ProjectID: 102, WalletAddress: "0x1111111111111111111111111111111111111111", ContributionAmountRaw: "1000000", ClaimedTokensRaw: "0", Refunded: false, Status: "canonical", StateGeneration: 1}
	_, digest = fairLaunchV3Payload(fairLaunchPositionStatePayload(position))
	position.StateDigest = digest
	if !fairLaunchPositionStateDigestValid(position) {
		t.Fatal("valid position digest was rejected")
	}
	position.Refunded = true
	if fairLaunchPositionStateDigestValid(position) {
		t.Fatal("tampered position projection was accepted")
	}
}

func TestFairLaunchAttestationDigestValidation(t *testing.T) {
	lineage := entity.FairLaunchApplicationLineageV3{ID: 11, PayloadDigest: testDigest("payload"), EffectDigest: testDigest("effect"), LedgerDigest: testDigest("ledger"), MutationDigest: testDigest("mutation")}
	attestation := entity.FairLaunchEffectAttestationV3{EventIdentityID: 12, PlacementID: 13, SourceApplicationLineageID: lineage.ID, PayloadDigest: lineage.PayloadDigest, EffectDigest: lineage.EffectDigest, LedgerDigest: lineage.LedgerDigest, MutationDigest: lineage.MutationDigest, LedgerRowCount: 1, MutationRowCount: 2, ProjectMutationCount: 1, PositionMutationCount: 1}
	attestation.AttestationDigest = fairLaunchAttestationDigest(attestation.EventIdentityID, attestation.PlacementID, attestation.SourceApplicationLineageID, attestation.PayloadDigest, attestation.EffectDigest, attestation.LedgerDigest, attestation.MutationDigest, attestation.LedgerRowCount, attestation.MutationRowCount, attestation.ProjectMutationCount, attestation.PositionMutationCount)
	if !fairLaunchAttestationValid(attestation, lineage) {
		t.Fatalf("valid attestation digest was rejected state=%q len=%d", attestation.AttestationDigest, len(attestation.AttestationDigest))
	}
	attestation.LedgerRowCount++
	if fairLaunchAttestationValid(attestation, lineage) {
		t.Fatal("tampered attestation was accepted")
	}
}

func fairLaunchV3Fixture() (entity.FairLaunchProjectBindingV3, entity.FairLaunchProjectStateV3) {
	binding := entity.FairLaunchProjectBindingV3{ID: 1, RunRowID: 1, ChainID: 31337, SourceBindingID: 1, SourceID: 1, SourceRevisionID: 1, ContractAddress: "0x1000000000000000000000000000000000000001", ContractVersion: "fair_launch_v1", ProjectID: 102, ProjectTokenAddress: "0x2000000000000000000000000000000000000002", ProjectTokenDecimals: 18, PaymentTokenAddress: "0x3000000000000000000000000000000000000003", PaymentTokenDecimals: 6}
	project := entity.FairLaunchProjectStateV3{ID: 1, ProjectBindingID: 1, RunRowID: 1, ChainID: 31337, SourceID: 1, ContractAddress: binding.ContractAddress, ContractVersion: binding.ContractVersion, ProjectID: binding.ProjectID, ProjectTokenAddress: binding.ProjectTokenAddress, PaymentTokenAddress: binding.PaymentTokenAddress, Status: "Active", TerminalReason: "none", SaleTokenAmountRaw: "1000000000000000000000000", TotalRaisedRaw: "0", TotalTokensDepositedRaw: "1000000000000000000000000", TotalTokensClaimedRaw: "0", ParticipantCountRaw: "0", PlatformFeeReservedRaw: "0", ProjectProceedsPaidRaw: "0", PlatformFeePaidRaw: "0", StateGeneration: 1}
	return binding, project
}

func fairLaunchV3Event(name string, projectID uint64) *commonchain.FairLaunchTypedEvent {
	return &commonchain.FairLaunchTypedEvent{Name: name, ProjectID: new(big.Int).SetUint64(projectID), Indexed: map[string]any{"arg0": new(big.Int).SetUint64(projectID)}, Values: map[string]any{}, RawDecimal: map[string]string{"arg0": new(big.Int).SetUint64(projectID).String()}, Log: typesLogFixture()}
}

func typesLogFixture() types.Log {
	return types.Log{Address: common.HexToAddress("0x1000000000000000000000000000000000000001"), BlockNumber: 50, BlockHash: common.HexToHash("0x50"), TxHash: common.HexToHash("0x51"), Index: 0}
}

func testDigest(value string) string { return fairLaunchV3DomainDigest("test", value) }

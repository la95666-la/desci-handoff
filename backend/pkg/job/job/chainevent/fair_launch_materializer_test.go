package chainevent

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	commonchain "joincare_server/pkg/common/chain"
)

func TestFairLaunchMaterializerManifestCoversEveryFrozenEvent(t *testing.T) {
	if len(fairLaunchEffectManifestV1) != len(commonchain.FairLaunchEventSignatures) {
		t.Fatalf("manifest covers %d events, want %d", len(fairLaunchEffectManifestV1), len(commonchain.FairLaunchEventSignatures))
	}
	for name := range commonchain.FairLaunchEventSignatures {
		manifest, ok := FairLaunchEffectManifest(name)
		if !ok || manifest.EventName != name || manifest.Version != FairLaunchEffectManifestVersion || manifest.DigestDomain == "" {
			t.Fatalf("invalid manifest for %s: %+v", name, manifest)
		}
	}
}

func TestFairLaunchMaterializerFinancialManifestIsInternallyDerived(t *testing.T) {
	want := map[string]string{
		"FairLaunchTokensDeposited":              "project_token_deposited",
		"FairLaunchContribution":                 "payment_contributed",
		"FairLaunchClaimed":                      "project_token_claimed",
		"FairLaunchRefunded":                     "payment_refunded",
		"FairLaunchProjectProceedsWithdrawn":     "project_proceeds_paid",
		"FairLaunchPlatformFeeWithdrawn":         "platform_fee_paid",
		"FairLaunchFailedProjectTokensWithdrawn": "project_tokens_withdrawn",
		"FairLaunchExcessTokensRecovered":        "excess_tokens_recovered",
	}
	for eventName, entryType := range want {
		manifest, ok := FairLaunchEffectManifest(eventName)
		if !ok || len(manifest.Ledger) != 1 || manifest.Ledger[0].EntryType != entryType {
			t.Fatalf("%s ledger manifest=%+v", eventName, manifest.Ledger)
		}
		if eventName == "FairLaunchExcessTokensRecovered" && manifest.Ledger[0].Scope != FairLaunchLedgerScopeSource {
			t.Fatalf("source-level recovery was assigned to project ledger: %+v", manifest.Ledger[0])
		}
	}
	for eventName := range commonchain.FairLaunchEventSignatures {
		manifest, _ := FairLaunchEffectManifest(eventName)
		if _, financial := want[eventName]; !financial && len(manifest.Ledger) != 0 {
			t.Fatalf("%s unexpectedly has financial rows: %+v", eventName, manifest.Ledger)
		}
	}
}

func TestFairLaunchMaterializerRejectsUnversionedOrUnknownEffects(t *testing.T) {
	event := &commonchain.FairLaunchTypedEvent{Name: "FairLaunchActivated"}
	if _, err := MaterializeFairLaunchEvent(event, FairLaunchMaterializationContext{ChainID: 31337, SourceRevisionID: 1, EffectVersion: 2, Canonical: true, Finalized: true}); err == nil {
		t.Fatal("unknown effect manifest version accepted")
	}
	event.Name = "FairLaunchFutureEvent"
	if _, err := MaterializeFairLaunchEvent(event, FairLaunchMaterializationContext{ChainID: 31337, SourceRevisionID: 1, EffectVersion: 1, Canonical: true, Finalized: true}); err == nil {
		t.Fatal("unknown event accepted")
	}
}

func TestFairLaunchMaterializerUsesTypedCanonicalDigest(t *testing.T) {
	event := &commonchain.FairLaunchTypedEvent{
		Name:      "FairLaunchContribution",
		ProjectID: big.NewInt(7),
		Indexed: map[string]any{
			"arg0": big.NewInt(7),
			"arg1": common.HexToAddress("0x1000000000000000000000000000000000000001"),
			"arg2": common.HexToAddress("0x2000000000000000000000000000000000000002"),
		},
		Values:     map[string]any{"arg3": big.NewInt(1), "arg4": big.NewInt(23), "arg5": big.NewInt(23)},
		RawDecimal: map[string]string{"arg0": "7", "arg3": "1", "arg4": "23", "arg5": "23"},
		Log:        types.Log{Address: common.HexToAddress("0x3000000000000000000000000000000000000003"), TxHash: common.HexToHash("0x01")},
	}
	ctx := FairLaunchMaterializationContext{ChainID: 31337, TxLogOrdinal: 2, PlacementGeneration: 1, SourceRevisionID: 9, EffectVersion: 1, Canonical: true, Finalized: true}
	first, err := MaterializeFairLaunchEvent(event, ctx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MaterializeFairLaunchEvent(event, ctx)
	if err != nil || first.EffectDigest != second.EffectDigest || len(first.EffectDigest) != 64 {
		t.Fatalf("nondeterministic digest first=%+v second=%+v err=%v", first, second, err)
	}
	if first.Disposition != FairLaunchEffectExact || !first.ProjectChanged || !first.PositionChanged || first.GovernanceChanged || first.PolicyChanged || first.LedgerEntryTypes[0] != "payment_contributed" {
		t.Fatalf("unexpected contribution effect: %+v", first)
	}
	event.Values["arg3"] = big.NewInt(12)
	event.Values["arg4"] = big.NewInt(3)
	event.RawDecimal["arg3"], event.RawDecimal["arg4"] = "12", "3"
	other, err := MaterializeFairLaunchEvent(event, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if other.EffectDigest == first.EffectDigest {
		t.Fatal("effect digest collided for distinct typed values")
	}
}

func TestFairLaunchMaterializerSeparatesGovernanceAndPolicyMutations(t *testing.T) {
	governanceEvent := &commonchain.FairLaunchTypedEvent{Name: "FairLaunchOperatorUpdated", Indexed: map[string]any{"arg0": common.HexToAddress("0x1000000000000000000000000000000000000001"), "arg1": common.HexToAddress("0x2000000000000000000000000000000000000002")}, Values: map[string]any{}, RawDecimal: map[string]string{}, Log: types.Log{Address: common.HexToAddress("0x3000000000000000000000000000000000000003"), TxHash: common.HexToHash("0x01")}}
	ctx := FairLaunchMaterializationContext{ChainID: 31337, TxLogOrdinal: 1, PlacementGeneration: 1, SourceRevisionID: 1, EffectVersion: 1, Canonical: true, Finalized: true}
	governance, err := MaterializeFairLaunchEvent(governanceEvent, ctx)
	if err != nil || !governance.GovernanceChanged || governance.PolicyChanged {
		t.Fatalf("governance effect=%+v err=%v", governance, err)
	}
	policyEvent := &commonchain.FairLaunchTypedEvent{Name: "FairLaunchPaymentTokenEmergencyBlockUpdated", Indexed: map[string]any{"arg0": common.HexToAddress("0x1000000000000000000000000000000000000001")}, Values: map[string]any{"arg1": true}, RawDecimal: map[string]string{}, Log: types.Log{Address: governanceEvent.Log.Address, TxHash: common.HexToHash("0x02")}}
	policy, err := MaterializeFairLaunchEvent(policyEvent, ctx)
	if err != nil || policy.GovernanceChanged || !policy.PolicyChanged {
		t.Fatalf("policy effect=%+v err=%v", policy, err)
	}
}

func TestReverseFairLaunchMaterializationUsesCompensationTypes(t *testing.T) {
	forward := FairLaunchMaterializationResult{
		EventName:        "FairLaunchContribution",
		ManifestVersion:  1,
		Manifest:         fairLaunchEffectManifestV1["FairLaunchContribution"],
		Ledger:           append([]FairLaunchLedgerPlan(nil), fairLaunchEffectManifestV1["FairLaunchContribution"].Ledger...),
		LedgerEntryTypes: []string{"payment_contributed"},
	}
	reversed, err := ReverseFairLaunchMaterialization(forward, false)
	if err != nil {
		t.Fatal(err)
	}
	if reversed.Disposition != FairLaunchEffectReversed || reversed.LedgerEntryTypes[0] != "payment_contributed_reversal" || reversed.Ledger[0].FlowDirection != "compensation" {
		t.Fatal(reversed)
	}
	blocked, err := ReverseFairLaunchMaterialization(forward, true)
	if err != nil || blocked.Disposition != FairLaunchEffectBlocked || len(blocked.Ledger) != 0 {
		t.Fatalf("blocked reversal=%+v err=%v", blocked, err)
	}
}

package chain

import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestEveryFrozenEventHasTypedDecoderMetadata(t *testing.T) {
	if len(fairLaunchEventABI) != len(FairLaunchEventSignatures) {
		t.Fatalf("got %d typed events, want %d", len(fairLaunchEventABI), len(FairLaunchEventSignatures))
	}
	for topic, name := range fairLaunchFrozenTopics {
		event, ok := fairLaunchEventABI[topic]
		if !ok || event.Name != name {
			t.Fatalf("missing %s", name)
		}
	}
}

func TestFairLaunchIndexedMaskMatchesGeneratedContractManifest(t *testing.T) {
	type manifestInput struct {
		Ordinal uint `json:"ordinal"`
		Indexed bool `json:"indexed"`
	}
	type manifestEvent struct {
		Name   string          `json:"name"`
		Inputs []manifestInput `json:"inputs"`
	}
	type generatedManifest struct {
		EventCount int             `json:"eventCount"`
		Events     []manifestEvent `json:"events"`
	}
	_, file, _, _ := runtime.Caller(0)
	manifestPath := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "JoinCare_SmartContract-main", "scripts", "validation", "fair-launch-events.generated.json"))
	encoded, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Skipf("generated Contract event manifest not present beside Backend checkout: %v", err)
	}
	var generated generatedManifest
	if err := json.Unmarshal(encoded, &generated); err != nil {
		t.Fatal(err)
	}
	if generated.EventCount != len(FairLaunchEventSignatures) || len(generated.Events) != generated.EventCount {
		t.Fatalf("generated event count=%d/%d, want %d", generated.EventCount, len(generated.Events), len(FairLaunchEventSignatures))
	}
	for _, contractEvent := range generated.Events {
		signature, exists := FairLaunchEventSignatures[contractEvent.Name]
		if !exists {
			t.Fatalf("Contract manifest contains unknown event %s", contractEvent.Name)
		}
		event, err := abiEventFromSignature(contractEvent.Name, signature)
		if err != nil {
			t.Fatal(err)
		}
		if len(event.Inputs) != len(contractEvent.Inputs) {
			t.Fatalf("%s inputs=%d, Contract manifest=%d", contractEvent.Name, len(event.Inputs), len(contractEvent.Inputs))
		}
		for i, input := range contractEvent.Inputs {
			if input.Ordinal != uint(i) || event.Inputs[i].Indexed != input.Indexed {
				t.Fatalf("%s input %d indexed=%t, Contract manifest=%t", contractEvent.Name, i, event.Inputs[i].Indexed, input.Indexed)
			}
		}
	}
}

func TestFairLaunchIndexedMaskMatchesAuditedSolidityABI(t *testing.T) {
	want := map[string][]int{
		"FairLaunchProjectCreated":                       {0, 1, 2},
		"FairLaunchTokensDeposited":                      {0, 1, 2},
		"FairLaunchActivated":                            {0},
		"FairLaunchContribution":                         {0, 1, 2},
		"FairLaunchEnded":                                {0, 3},
		"FairLaunchSuccessfulReleased":                   {0, 1, 2},
		"FairLaunchPendingReleaseInvalidated":            {0, 5},
		"FairLaunchPaymentTokenInvalidated":              {0, 1, 2},
		"FairLaunchCancelled":                            {0, 4},
		"FairLaunchRefunded":                             {0, 1, 2},
		"FairLaunchClaimed":                              {0, 1, 2},
		"FairLaunchProjectProceedsWithdrawn":             {0, 1, 2},
		"FairLaunchPlatformFeeWithdrawn":                 {0, 1, 2},
		"FairLaunchFailedProjectTokensWithdrawn":         {0, 1, 2},
		"FairLaunchContributionsPaused":                  {0},
		"FairLaunchContributionsUnpaused":                {0},
		"FairLaunchPayoutsPaused":                        {0},
		"FairLaunchPayoutsUnpaused":                      {0},
		"FairLaunchClaimsPaused":                         {0},
		"FairLaunchClaimsUnpaused":                       {0},
		"FairLaunchOperatorUpdated":                      {0, 1},
		"FairLaunchPauseGuardianUpdated":                 {0, 1},
		"FairLaunchUnpauseTimelockUpdated":               {0, 1},
		"FairLaunchTreasuryUpdated":                      {0, 1},
		"FairLaunchPlatformFeeUpdated":                   {2, 3},
		"FairLaunchPaymentTokenPolicyRevisionRegistered": {0, 1},
		"FairLaunchPaymentTokenAdmissionUpdated":         {0, 1},
		"FairLaunchPaymentTokenEmergencyBlockUpdated":    {0},
		"FairLaunchProjectTokenConfigured":               {0},
		"FairLaunchBeneficiarySafeConfigured":            {0},
		"FairLaunchReleasePolicyConfigured":              {0},
		"FairLaunchExcessTokensRecovered":                {0, 1},
	}
	if len(want) != len(FairLaunchEventSignatures) {
		t.Fatalf("indexed-mask fixture covers %d events, want %d", len(want), len(FairLaunchEventSignatures))
	}
	for name, signature := range FairLaunchEventSignatures {
		event, err := abiEventFromSignature(name, signature)
		if err != nil {
			t.Fatal(err)
		}
		got := make([]int, 0)
		for i, input := range event.Inputs {
			if input.Indexed {
				got = append(got, i)
			}
		}
		if !equalInts(got, want[name]) {
			t.Fatalf("%s indexed positions=%v, want %v", name, got, want[name])
		}
	}
}

func TestTypedContributionPreservesUint256(t *testing.T) {
	event := fairLaunchEventABI[mustTopic(t, "FairLaunchContribution")]
	huge := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	data, err := event.Inputs.NonIndexed().Pack(huge, huge, huge)
	if err != nil {
		t.Fatal(err)
	}
	log := types.Log{Topics: []common.Hash{event.ID, common.BigToHash(big.NewInt(7)), addressTopic("0x1000000000000000000000000000000000000001"), addressTopic("0x2000000000000000000000000000000000000002")}, Data: data}
	got, err := DecodeFairLaunchTypedLog(log)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectID.Uint64() != 7 || got.RawDecimal["arg3"] != huge.String() {
		t.Fatalf("bad %+v", got)
	}
}

func TestTypedDecoderUsesSolidityTopicAndDataOrder(t *testing.T) {
	projectCreated := fairLaunchEventABI[mustTopic(t, "FairLaunchProjectCreated")]
	createdValues := []any{
		common.HexToAddress("0x3000000000000000000000000000000000000003"),
		common.HexToAddress("0x4000000000000000000000000000000000000004"),
		[32]byte{1}, uint32(2),
		common.HexToAddress("0x5000000000000000000000000000000000000005"),
		[32]byte{3}, uint32(4), uint16(250),
		big.NewInt(1000), big.NewInt(200), big.NewInt(5), big.NewInt(50),
		uint64(100), uint64(200), [32]byte{5}, uint32(6), [32]byte{7}, uint32(8), [32]byte{9}, uint32(10),
	}
	createdData, err := projectCreated.Inputs.NonIndexed().Pack(createdValues...)
	if err != nil {
		t.Fatal(err)
	}
	created, err := DecodeFairLaunchTypedLog(types.Log{Topics: []common.Hash{
		projectCreated.ID,
		common.BigToHash(big.NewInt(77)),
		addressTopic("0x1000000000000000000000000000000000000001"),
		addressTopic("0x2000000000000000000000000000000000000002"),
	}, Data: createdData})
	if err != nil {
		t.Fatal(err)
	}
	if created.ProjectID.Uint64() != 77 || created.Indexed["arg1"] != common.HexToAddress("0x1000000000000000000000000000000000000001") || created.Indexed["arg2"] != common.HexToAddress("0x2000000000000000000000000000000000000002") || created.Values["arg3"] != common.HexToAddress("0x3000000000000000000000000000000000000003") {
		t.Fatalf("project-created decode mismatch: %+v", created)
	}

	feeUpdated := fairLaunchEventABI[mustTopic(t, "FairLaunchPlatformFeeUpdated")]
	feeData, err := feeUpdated.Inputs.NonIndexed().Pack(uint16(100), uint16(250))
	if err != nil {
		t.Fatal(err)
	}
	fee, err := DecodeFairLaunchTypedLog(types.Log{Topics: []common.Hash{
		feeUpdated.ID,
		addressTopic("0x6000000000000000000000000000000000000006"),
		addressTopic("0x7000000000000000000000000000000000000007"),
	}, Data: feeData})
	if err != nil {
		t.Fatal(err)
	}
	if fee.RawDecimal["arg0"] != "100" || fee.RawDecimal["arg1"] != "250" || fee.Indexed["arg2"] != common.HexToAddress("0x6000000000000000000000000000000000000006") || fee.Indexed["arg3"] != common.HexToAddress("0x7000000000000000000000000000000000000007") {
		t.Fatalf("platform-fee decode mismatch: %+v", fee)
	}
}

func TestCanonicalTypedEventDigestIsTypedLengthSafeAndDeterministic(t *testing.T) {
	eventABI := fairLaunchEventABI[mustTopic(t, "FairLaunchPlatformFeeUpdated")]
	data, err := eventABI.Inputs.NonIndexed().Pack(uint16(1), uint16(23))
	if err != nil {
		t.Fatal(err)
	}
	event, err := DecodeFairLaunchTypedLog(types.Log{
		Address: common.HexToAddress("0x8000000000000000000000000000000000000008"),
		Topics:  []common.Hash{eventABI.ID, addressTopic("0x6000000000000000000000000000000000000006"), addressTopic("0x7000000000000000000000000000000000000007")},
		Data:    data,
	})
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := CanonicalFairLaunchTypedEvent(event)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"arguments":[{"indexed":false,"name":"arg0","type":"uint16","value":"1"},{"indexed":false,"name":"arg1","type":"uint16","value":"23"},{"indexed":true,"name":"arg2","type":"address","value":"0x6000000000000000000000000000000000000006"},{"indexed":true,"name":"arg3","type":"address","value":"0x7000000000000000000000000000000000000007"}],"emitting_address":"0x8000000000000000000000000000000000000008","event_name":"FairLaunchPlatformFeeUpdated"}`
	if string(canonical) != want {
		t.Fatalf("canonical payload=%s", canonical)
	}
	first, err := FairLaunchTypedEventDigest(event)
	if err != nil {
		t.Fatal(err)
	}
	second, err := FairLaunchTypedEventDigest(event)
	if err != nil || first != second || len(first) != 64 {
		t.Fatalf("digest mismatch first=%s second=%s err=%v", first, second, err)
	}

	otherData, err := eventABI.Inputs.NonIndexed().Pack(uint16(12), uint16(3))
	if err != nil {
		t.Fatal(err)
	}
	other, err := DecodeFairLaunchTypedLog(types.Log{Address: event.Log.Address, Topics: event.Log.Topics, Data: otherData})
	if err != nil {
		t.Fatal(err)
	}
	otherDigest, err := FairLaunchTypedEventDigest(other)
	if err != nil {
		t.Fatal(err)
	}
	if otherDigest == first {
		t.Fatal("typed digest collided for [1,23] and [12,3]")
	}
}

func TestTypedDecoderRejectsWrongIndexedTopicCount(t *testing.T) {
	event := fairLaunchEventABI[mustTopic(t, "FairLaunchEnded")]
	data, err := event.Inputs.NonIndexed().Pack(uint8(2), uint8(0), big.NewInt(10), big.NewInt(20), big.NewInt(30), big.NewInt(40), big.NewInt(39), [32]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	_, err = DecodeFairLaunchTypedLog(types.Log{Topics: []common.Hash{event.ID, common.BigToHash(big.NewInt(1))}, Data: data})
	if err == nil {
		t.Fatal("accepted FairLaunchEnded without indexed paymentToken topic")
	}
}

func addressTopic(value string) common.Hash {
	return common.BytesToHash(common.HexToAddress(value).Bytes())
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mustTopic(t *testing.T, name string) common.Hash {
	t.Helper()
	for h, n := range fairLaunchFrozenTopics {
		if n == name {
			return h
		}
	}
	t.Fatal("missing topic")
	return common.Hash{}
}

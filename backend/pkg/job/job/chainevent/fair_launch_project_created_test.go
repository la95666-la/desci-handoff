package chainevent

import (
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

func TestDecodeFairLaunchProjectCreatedFactsExactMapping(t *testing.T) {
	facts, err := decodeFairLaunchProjectCreatedFacts(fairLaunchProjectCreatedFixture())
	if err != nil {
		t.Fatal(err)
	}
	if facts.ProjectID != 41 || facts.ProjectToken != addressMainnet('1') || facts.PaymentToken != addressMainnet('2') || facts.ProjectOwner != addressMainnet('3') || facts.ProjectProceedsReceiver != addressMainnet('4') || facts.PlatformFeeReceiver != addressMainnet('5') {
		t.Fatalf("address mapping mismatch: %+v", facts)
	}
	if facts.ProjectProceedsReceiverAttestationVersion != 2 || facts.PlatformFeeReceiverAttestationVersion != 3 || facts.PlatformFeeBPS != 250 || facts.SaleTokenAmountRaw != "1000000000000000000000" || facts.SoftCapRaw != "1000000000000000000" || facts.MinContributionRaw != "1000000000000000000" || facts.MaxContributionRaw != "5000000000000000000" || facts.StartTime != 100 || facts.EndTime != 200 || facts.ProjectTokenPolicyVersion != 4 || facts.PaymentTokenPolicyVersion != 5 || facts.ReleasePolicyVersion != 6 {
		t.Fatalf("scalar mapping mismatch: %+v", facts)
	}
	if facts.ProjectProceedsReceiverAttestationDigest != strings.Repeat("11", 32) || facts.PlatformFeeReceiverAttestationDigest != strings.Repeat("22", 32) || facts.ProjectTokenPolicyDigest != strings.Repeat("33", 32) || facts.PaymentTokenPolicyDigest != strings.Repeat("44", 32) || facts.ReleasePolicyDigest != strings.Repeat("55", 32) {
		t.Fatalf("digest mapping mismatch: %+v", facts)
	}
}

func TestDecodeFairLaunchProjectCreatedFactsRejectsMalformedValues(t *testing.T) {
	tests := []func(*commonchain.FairLaunchTypedEvent){
		func(e *commonchain.FairLaunchTypedEvent) { e.Indexed["arg1"] = common.Address{} },
		func(e *commonchain.FairLaunchTypedEvent) { e.RawDecimal["arg11"] = "01" },
		func(e *commonchain.FairLaunchTypedEvent) { e.RawDecimal["arg13"] = "6"; e.RawDecimal["arg14"] = "5" },
		func(e *commonchain.FairLaunchTypedEvent) {
			e.RawDecimal["arg15"] = "200"
			e.RawDecimal["arg16"] = "200"
		},
		func(e *commonchain.FairLaunchTypedEvent) { e.RawDecimal["arg18"] = "0" },
	}
	for index, mutate := range tests {
		event := fairLaunchProjectCreatedFixture()
		mutate(event)
		if _, err := decodeFairLaunchProjectCreatedFacts(event); err == nil {
			t.Fatalf("malformed mutation %d accepted", index)
		}
	}
}

func TestValidateFairLaunchProjectCreatedAdmissionExactAndFailClosed(t *testing.T) {
	capability, admission, payment, project, facts := fairLaunchProjectAdmissionFixture(t)
	if err := validateFairLaunchProjectCreatedAdmission(capability, admission, payment, project, facts); err != nil {
		t.Fatalf("exact admission rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*entity.FairLaunchMainnetProjectAdmissionV1, *entity.FairLaunchMainnetPaymentTokenV1, *entity.Project, *fairLaunchProjectCreatedFacts)
	}{
		{"admission digest", func(a *entity.FairLaunchMainnetProjectAdmissionV1, _ *entity.FairLaunchMainnetPaymentTokenV1, _ *entity.Project, _ *fairLaunchProjectCreatedFacts) {
			a.AdmissionDigest = "invalid"
		}},
		{"payment binding", func(_ *entity.FairLaunchMainnetProjectAdmissionV1, p *entity.FairLaunchMainnetPaymentTokenV1, _ *entity.Project, _ *fairLaunchProjectCreatedFacts) {
			p.BindingDigest = "invalid"
		}},
		{"owner", func(_ *entity.FairLaunchMainnetProjectAdmissionV1, _ *entity.FairLaunchMainnetPaymentTokenV1, p *entity.Project, _ *fairLaunchProjectCreatedFacts) {
			p.OwnerAddress = addressMainnet('9')
		}},
		{"fee bps", func(_ *entity.FairLaunchMainnetProjectAdmissionV1, _ *entity.FairLaunchMainnetPaymentTokenV1, p *entity.Project, _ *fairLaunchProjectCreatedFacts) {
			p.SuccessFeeRate++
		}},
		{"content", func(_ *entity.FairLaunchMainnetProjectAdmissionV1, _ *entity.FairLaunchMainnetPaymentTokenV1, p *entity.Project, _ *fairLaunchProjectCreatedFacts) {
			p.Slug = ""
		}},
		{"metadata", func(_ *entity.FairLaunchMainnetProjectAdmissionV1, _ *entity.FairLaunchMainnetPaymentTokenV1, p *entity.Project, _ *fairLaunchProjectCreatedFacts) {
			p.OnchainJSON = `{}`
		}},
		{"cap", func(a *entity.FairLaunchMainnetProjectAdmissionV1, _ *entity.FairLaunchMainnetPaymentTokenV1, _ *entity.Project, f *fairLaunchProjectCreatedFacts) {
			f.MaxContributionRaw = addOneRaw(a.MaxWalletContributionRaw)
		}},
		{"unlimited cap", func(_ *entity.FairLaunchMainnetProjectAdmissionV1, _ *entity.FairLaunchMainnetPaymentTokenV1, _ *entity.Project, f *fairLaunchProjectCreatedFacts) {
			f.MaxContributionRaw = "0"
		}},
		{"aggregate cap differs", func(a *entity.FairLaunchMainnetProjectAdmissionV1, _ *entity.FairLaunchMainnetPaymentTokenV1, _ *entity.Project, _ *fairLaunchProjectCreatedFacts) {
			a.MaxTotalContributionRaw = addOneRaw(a.MaxTotalContributionRaw)
		}},
		{"activation cap", func(_ *entity.FairLaunchMainnetProjectAdmissionV1, p *entity.FairLaunchMainnetPaymentTokenV1, _ *entity.Project, _ *fairLaunchProjectCreatedFacts) {
			p.MaxActivationValueRaw = "1"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a, p, row, f := admission, payment, project, facts
			test.mutate(&a, &p, &row, &f)
			if err := validateFairLaunchProjectCreatedAdmission(capability, a, p, row, f); err == nil {
				t.Fatal("mismatch accepted")
			}
		})
	}
}

func fairLaunchProjectCreatedFixture() *commonchain.FairLaunchTypedEvent {
	event := &commonchain.FairLaunchTypedEvent{Name: "FairLaunchProjectCreated", ProjectID: big.NewInt(41), Indexed: map[string]any{"arg0": big.NewInt(41), "arg1": common.HexToAddress(addressMainnet('1')), "arg2": common.HexToAddress(addressMainnet('2'))}, Values: map[string]any{}, RawDecimal: map[string]string{"arg0": "41", "arg6": "2", "arg9": "3", "arg10": "250", "arg11": "1000000000000000000000", "arg12": "1000000000000000000", "arg13": "1000000000000000000", "arg14": "5000000000000000000", "arg15": "100", "arg16": "200", "arg18": "4", "arg20": "5", "arg22": "6"}}
	event.Values["arg3"] = common.HexToAddress(addressMainnet('3'))
	event.Values["arg4"] = common.HexToAddress(addressMainnet('4'))
	event.Values["arg5"] = bytes32Mainnet(0x11)
	event.Values["arg6"] = uint32(2)
	event.Values["arg7"] = common.HexToAddress(addressMainnet('5'))
	event.Values["arg8"] = bytes32Mainnet(0x22)
	event.Values["arg9"] = uint32(3)
	event.Values["arg10"] = uint16(250)
	event.Values["arg11"] = big.NewInt(1)
	event.Values["arg12"] = big.NewInt(1)
	event.Values["arg13"] = big.NewInt(1)
	event.Values["arg14"] = big.NewInt(1)
	event.Values["arg15"] = uint64(100)
	event.Values["arg16"] = uint64(200)
	event.Values["arg17"] = bytes32Mainnet(0x33)
	event.Values["arg18"] = uint32(4)
	event.Values["arg19"] = bytes32Mainnet(0x44)
	event.Values["arg20"] = uint32(5)
	event.Values["arg21"] = bytes32Mainnet(0x55)
	event.Values["arg22"] = uint32(6)
	return event
}

func fairLaunchProjectAdmissionFixture(t *testing.T) (FairLaunchMainnetCapability, entity.FairLaunchMainnetProjectAdmissionV1, entity.FairLaunchMainnetPaymentTokenV1, entity.Project, fairLaunchProjectCreatedFacts) {
	t.Helper()
	facts, err := decodeFairLaunchProjectCreatedFacts(fairLaunchProjectCreatedFixture())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(50, 0).UTC()
	capability := FairLaunchMainnetCapability{seal: &fairLaunchMainnetSeal{admission: FairLaunchMainnetAdmission{AuthorityRowID: 7, ChainID: 56, ProxyAddress: addressMainnet('a'), ValidUntil: now.Add(time.Hour)}}}
	revision := facts.PaymentTokenPolicyVersion
	project := entity.Project{ID: 8, ProjectID: int(facts.ProjectID), OwnerAddress: facts.ProjectOwner, NameEn: "Project", NameCn: "项目", Slug: "project", Tagline: "tagline", Description: "description", Tags: "[]", TokenAddress: facts.ProjectToken, TokenSymbol: "JCT", TokenDecimals: 18, PaymentToken: "USDT", PaymentTokenAddress: facts.PaymentToken, PaymentTokenDecimals: 18, PaymentTokenPolicyRevision: &revision, ProjectTokenPolicyDigest: facts.ProjectTokenPolicyDigest, ProjectTokenRuntimeCodeHash: strings.Repeat("6", 64), ChainID: 56, ContractAddress: addressMainnet('a'), ContractVersion: constant.ContractVersionFairLaunchV1, FundingMode: constant.FundingModeFairLaunch, FeeReceiver: facts.PlatformFeeReceiver, SuccessFeeRate: facts.PlatformFeeBPS, SaleTokenAmountRaw: facts.SaleTokenAmountRaw, SoftCapRaw: facts.SoftCapRaw, MinContributionRaw: facts.MinContributionRaw, MaxContributionRaw: facts.MaxContributionRaw, MaxPerUserRaw: "0", TotalRaisedRaw: "0", TotalDepositedRaw: "0", StartTime: time.Unix(int64(facts.StartTime), 0).UTC(), EndTime: time.Unix(int64(facts.EndTime), 0).UTC(), Status: constant.ProjectStatusSet}
	project.OnchainJSON = `{"fairLaunch":{"saleTokenAmountRaw":"` + facts.SaleTokenAmountRaw + `","softCapRaw":"` + facts.SoftCapRaw + `","minContributionRaw":"` + facts.MinContributionRaw + `","maxContributionRaw":"` + facts.MaxContributionRaw + `","projectProceedsReceiver":"` + facts.ProjectProceedsReceiver + `","projectProceedsReceiverAttestationDigest":"` + facts.ProjectProceedsReceiverAttestationDigest + `","projectProceedsReceiverAttestationVersion":2,"platformFeeReceiverAttestationDigest":"` + facts.PlatformFeeReceiverAttestationDigest + `","platformFeeReceiverAttestationVersion":3,"projectTokenPolicyDigest":"` + facts.ProjectTokenPolicyDigest + `","projectTokenPolicyVersion":4,"paymentTokenPolicyDigest":"` + facts.PaymentTokenPolicyDigest + `","paymentTokenPolicyVersion":5,"releasePolicyDigest":"` + facts.ReleasePolicyDigest + `","releasePolicyVersion":6}}`
	admission := entity.FairLaunchMainnetProjectAdmissionV1{ActivationRowID: 7, ChainID: 56, ProjectRowID: project.ID, ProjectID: facts.ProjectID, PaymentTokenAddress: facts.PaymentToken, PaymentTokenDecimals: 18, ProjectTokenAddress: facts.ProjectToken, ProjectTokenDecimals: 18, ProjectTokenRuntimeCodeHash: project.ProjectTokenRuntimeCodeHash, MaxTotalContributionRaw: facts.MaxContributionRaw, MaxWalletContributionRaw: facts.MaxContributionRaw}
	admission.AdmissionDigest = strings.Repeat("8", 64)
	payment := entity.FairLaunchMainnetPaymentTokenV1{ActivationRowID: 7, ChainID: 56, TokenSymbol: "USDT", TokenAddress: facts.PaymentToken, TokenDecimals: 18, RuntimeCodeHash: strings.Repeat("7", 64), PolicyDigest: facts.PaymentTokenPolicyDigest, PolicyVersion: facts.PaymentTokenPolicyVersion, MaxActivationValueRaw: facts.MaxContributionRaw}
	payment.BindingDigest = strings.Repeat("9", 64)
	return capability, admission, payment, project, facts
}

func addressMainnet(value byte) string { return "0x" + strings.Repeat(string(value), 40) }
func bytes32Mainnet(value byte) [32]byte {
	var out [32]byte
	for i := range out {
		out[i] = value
	}
	return out
}
func addOneRaw(value string) string {
	n, _ := new(big.Int).SetString(value, 10)
	return n.Add(n, big.NewInt(1)).String()
}

package project

import (
	"errors"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/projectruntime"
	"testing"
	"time"
)

type fakeFairInspector struct{}

func (fakeFairInspector) ProjectToken(a string) (FairLaunchProjectInspection, error) {
	return FairLaunchProjectInspection{Address: a, Symbol: "SALE", Decimals: 18, RuntimeCodeHash: "0x" + hex64('1'), PolicyDigest: "0x" + hex64('2'), PolicyVersion: 3}, nil
}
func (fakeFairInspector) BeneficiarySafe(a string) (FairLaunchBeneficiaryInspection, error) {
	return FairLaunchBeneficiaryInspection{Address: a, AttestationDigest: "0x" + hex64('3'), AttestationVersion: 4}, nil
}
func (fakeFairInspector) PlatformFeeReceiver(a string) (FairLaunchBeneficiaryInspection, error) {
	return FairLaunchBeneficiaryInspection{Address: a, AttestationDigest: "0x" + hex64('4'), AttestationVersion: 5}, nil
}
func TestFairLaunchSetPreviewIsUnavailableBeforeInspection(t *testing.T) {
	now := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	_, err := PrepareFairLaunchSetPreview(8, FairLaunchSetRequest{}, projectruntime.Runtime{Enabled: true}, FairLaunchRuntimeEvidence{}, nil, nil, now)
	if err == nil || err.Error() != constant.FairLaunchMaterializationUnavailable {
		t.Fatalf("set preview escaped materialization gate: %v", err)
	}
}

func TestFairLaunchSetPreviewDigest(t *testing.T) {
	if !constant.FairLaunchMaterializationEnabled {
		t.Skip("digest path remains covered for future materialization admission")
	}
	now := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	r := projectruntime.Runtime{Enabled: true, Version: constant.ContractVersionFairLaunchV1, FundingMode: constant.FundingModeFairLaunch, ChainID: 56, ContractAddress: "0x1000000000000000000000000000000000000001", ImplementationAddress: "0x2000000000000000000000000000000000000002", ImplementationCodeHash: "0x" + hex64('5'), PlatformFeeReceiver: "0x3000000000000000000000000000000000000003", PlatformFeeBps: 250, ReleasePolicyDigest: "0x" + hex64('6'), ReleasePolicyVersion: 2, PaymentTokens: map[string]projectruntime.PaymentToken{"USDT": {Symbol: "USDT", Address: "0x4000000000000000000000000000000000000004", Decimals: 18, PolicyDigest: "0x" + hex64('7'), PolicyVersion: 7}}}
	evidence := FairLaunchRuntimeEvidence{Runtime: FairLaunchPublicRuntimeIdentity{ChainID: 56, ProxyAddress: r.ContractAddress, ImplementationAddress: r.ImplementationAddress, ImplementationCodeHash: r.ImplementationCodeHash, SourceRegistryRevision: 4, SourceRegistryDigest: "0x" + hex64('8'), ObservedBlockNumber: "100", ObservedBlockHash: "0x" + hex64('9'), ObservedAt: now.Format(time.RFC3339), RuntimeVerified: true}, Generations: FairLaunchActionGenerations{Runtime: 1, Policy: 2, SourceRegistry: 3, Config: 4}}
	req := FairLaunchSetRequest{FundingMode: "fair_launch", ContractVersion: "fair_launch_v1", PaymentToken: "USDT", ProjectToken: "0x5000000000000000000000000000000000000005", ProjectOwner: "0x6000000000000000000000000000000000000006", ProjectProceedsReceiver: "0x7000000000000000000000000000000000000007", SaleTokenAmount: "100", SoftCap: "10", MinContribution: "1", MaxContribution: "0", StartTime: now.Add(time.Hour).Format(time.RFC3339), EndTime: now.Add(2 * time.Hour).Format(time.RFC3339), Vesting: []FairLaunchVestingRow{{BPS: 10000, UnlockTime: now.Add(3 * time.Hour).Format(time.RFC3339)}}}
	p, e := PrepareFairLaunchSetPreview(8, req, r, evidence, nil, fakeFairInspector{}, now)
	if e != nil {
		t.Fatal(e)
	}
	if p.CanonicalDigest == "" || p.RequiredDeposit.Raw != p.SaleTokenAmount.Raw {
		t.Fatal(p)
	}
	if !p.Runtime.RuntimeVerified || p.Runtime.SourceRegistryRevision != 4 || p.Runtime.ObservedBlockHash == "" {
		t.Fatal("preview did not bind trusted runtime evidence", p.Runtime)
	}
	req.CanonicalDigest = p.CanonicalDigest
	if _, e = VerifyFairLaunchSetDigest(8, req, r, evidence, nil, fakeFairInspector{}, now); e != nil {
		t.Fatal(e)
	}
	changed := evidence
	changed.Generations.Config++
	if _, e = VerifyFairLaunchSetDigest(8, req, r, changed, nil, fakeFairInspector{}, now); e == nil {
		t.Fatal("trusted evidence generation drift accepted")
	}
	req.SoftCap = "11"
	if _, e = VerifyFairLaunchSetDigest(8, req, r, evidence, nil, fakeFairInspector{}, now); e == nil {
		t.Fatal("tamper accepted")
	}
}
func TestCapabilitiesRequireEnabledAndVerifiedRuntimeEvidence(t *testing.T) {
	disabled := BuildFairLaunchCapabilities(8, projectruntime.Runtime{Version: "fair_launch_v1", FundingMode: "fair_launch"}, FairLaunchRuntimeEvidence{}, nil)
	if disabled.Available || disabled.ReasonCode != "runtime_disabled" || disabled.Runtime.RuntimeVerified || disabled.Actions["create"].Enabled || disabled.Actions["upgrade_proxy"].ExecutionDomain != "governance_timelock" {
		t.Fatal(disabled)
	}

	r := projectruntime.Runtime{Enabled: true, Version: "fair_launch_v1", FundingMode: "fair_launch", ChainID: 56, ContractAddress: "0x1000000000000000000000000000000000000001", ImplementationAddress: "0x2000000000000000000000000000000000000002", ImplementationCodeHash: "0x" + hex64('5')}
	unverified := BuildFairLaunchCapabilities(8, r, FairLaunchRuntimeEvidence{}, nil)
	if unverified.Available || unverified.ReasonCode != "runtime_unverified" || unverified.Runtime.RuntimeVerified || unverified.Actions["create"].Enabled {
		t.Fatal(unverified)
	}
	stale := BuildFairLaunchCapabilities(8, r, FairLaunchRuntimeEvidence{}, errors.New("runtime_stale: finalized coverage unavailable"))
	if stale.Available || stale.ReasonCode != "runtime_stale" || stale.Actions["create"].ReasonCode != constant.FairLaunchMaterializationUnavailable {
		t.Fatal(stale)
	}

	evidence := FairLaunchRuntimeEvidence{Runtime: FairLaunchPublicRuntimeIdentity{ChainID: 56, ProxyAddress: r.ContractAddress, ImplementationAddress: r.ImplementationAddress, ImplementationCodeHash: r.ImplementationCodeHash, SourceRegistryRevision: 4, SourceRegistryDigest: "0x" + hex64('6'), ObservedBlockNumber: "100", ObservedBlockHash: "0x" + hex64('7'), ObservedAt: time.Now().UTC().Format(time.RFC3339), RuntimeVerified: true}}
	verified := BuildFairLaunchCapabilities(8, r, evidence, nil)
	if !verified.Available || verified.ReasonCode != "" || !verified.Runtime.RuntimeVerified || verified.Runtime.SourceRegistryRevision != 4 {
		t.Fatal(verified)
	}
	for action, descriptor := range verified.Actions {
		if descriptor.Enabled || descriptor.ReasonCode != constant.FairLaunchMaterializationUnavailable {
			t.Fatalf("verified runtime enabled %s: %+v", action, descriptor)
		}
	}
	if verified.Actions["upgrade_proxy"].ExecutionDomain != "governance_timelock" {
		t.Fatal(verified)
	}
	mismatch := evidence
	mismatch.Runtime.ImplementationCodeHash = "0x" + hex64('8')
	stale = BuildFairLaunchCapabilities(8, r, mismatch, nil)
	if stale.Available || stale.ReasonCode != "runtime_stale" {
		t.Fatal(stale)
	}
}
func hex64(c byte) string {
	b := make([]byte, 64)
	for i := range b {
		b[i] = c
	}
	return string(b)
}

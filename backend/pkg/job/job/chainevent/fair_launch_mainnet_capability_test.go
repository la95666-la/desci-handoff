package chainevent

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"joincare_server/pkg/common/entity"
)

func TestFairLaunchMainnetCapabilityDefaultsFailClosed(t *testing.T) {
	if _, err := (FairLaunchMainnetCapability{}).Admission(); err == nil || err.Error() != FairLaunchMainnetActivationUnavailable {
		t.Fatalf("zero capability must be unavailable, got %v", err)
	}
	if capability := (FairLaunchMainnetCapability{}); capability.valid(time.Now()) {
		t.Fatal("zero capability must not be valid")
	}
	if _, err := LoadFairLaunchMainnetCapabilityTx(nil, FairLaunchMainnetActivationRequest{}, time.Now()); err == nil || err.Error() != FairLaunchMainnetActivationUnavailable {
		t.Fatalf("nil database must fail closed, got %v", err)
	}
}

func TestFairLaunchMainnetActivationRequestValidation(t *testing.T) {
	request, _, _, _, _, _, _, _, _, _ := fairLaunchMainnetCapabilityFixture(t)
	if !validFairLaunchMainnetActivationRequest(request) {
		t.Fatal("valid exact activation request rejected")
	}
	mutations := []func(*FairLaunchMainnetActivationRequest){
		func(value *FairLaunchMainnetActivationRequest) { value.ActivationID = "not-a-uuid" },
		func(value *FairLaunchMainnetActivationRequest) { value.Component = "release_tool" },
		func(value *FairLaunchMainnetActivationRequest) { value.GitCommit = value.GitCommit[:39] + "G" },
		func(value *FairLaunchMainnetActivationRequest) { value.ArtifactSHA256 = "0x" + value.ArtifactSHA256 },
		func(value *FairLaunchMainnetActivationRequest) { value.ComponentSetDigest = "" },
		func(value *FairLaunchMainnetActivationRequest) {
			value.ProxyAddress = "0x111111111111111111111111111111111111111A"
		},
		func(value *FairLaunchMainnetActivationRequest) { value.SourceID = 0 },
		func(value *FairLaunchMainnetActivationRequest) { value.LeaseToken = []byte("short") },
	}
	for index, mutate := range mutations {
		candidate := request
		candidate.LeaseToken = append([]byte(nil), request.LeaseToken...)
		mutate(&candidate)
		if validFairLaunchMainnetActivationRequest(candidate) {
			t.Fatalf("invalid activation request mutation %d accepted", index)
		}
	}
}

func TestValidateFairLaunchMainnetAdmissionExactIdentity(t *testing.T) {
	request, authority, activation, lease, history, component, sourceBinding, source, revision, leaseHash := fairLaunchMainnetCapabilityFixture(t)
	if err := validateFairLaunchMainnetAdmission(request, authority, activation, lease, history, component, sourceBinding, source, revision, leaseHash); err != nil {
		t.Fatalf("valid exact activation rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*FairLaunchMainnetActivationRequest, *entity.FairLaunchAuthorityV1, *entity.FairLaunchMainnetActivationV1, *entity.FairLaunchMainnetActivationLeaseV1, *entity.FairLaunchMainnetActivationHistoryV1, *entity.FairLaunchMainnetComponentBindingV1, *entity.FairLaunchSourceRevisionBindingV3, *[sha256.Size]byte)
	}{
		{name: "lease token", mutate: func(_ *FairLaunchMainnetActivationRequest, _ *entity.FairLaunchAuthorityV1, _ *entity.FairLaunchMainnetActivationV1, _ *entity.FairLaunchMainnetActivationLeaseV1, _ *entity.FairLaunchMainnetActivationHistoryV1, _ *entity.FairLaunchMainnetComponentBindingV1, _ *entity.FairLaunchSourceRevisionBindingV3, hash *[sha256.Size]byte) {
			hash[0] ^= 0xff
		}},
		{name: "authority kind", mutate: func(_ *FairLaunchMainnetActivationRequest, value *entity.FairLaunchAuthorityV1, _ *entity.FairLaunchMainnetActivationV1, _ *entity.FairLaunchMainnetActivationLeaseV1, _ *entity.FairLaunchMainnetActivationHistoryV1, _ *entity.FairLaunchMainnetComponentBindingV1, _ *entity.FairLaunchSourceRevisionBindingV3, _ *[sha256.Size]byte) {
			value.AuthorityKind = "local_e2e"
		}},
		{name: "chain", mutate: func(_ *FairLaunchMainnetActivationRequest, _ *entity.FairLaunchAuthorityV1, value *entity.FairLaunchMainnetActivationV1, _ *entity.FairLaunchMainnetActivationLeaseV1, _ *entity.FairLaunchMainnetActivationHistoryV1, _ *entity.FairLaunchMainnetComponentBindingV1, _ *entity.FairLaunchSourceRevisionBindingV3, _ *[sha256.Size]byte) {
			value.ChainID = 31337
		}},
		{name: "state", mutate: func(_ *FairLaunchMainnetActivationRequest, _ *entity.FairLaunchAuthorityV1, _ *entity.FairLaunchMainnetActivationV1, _ *entity.FairLaunchMainnetActivationLeaseV1, value *entity.FairLaunchMainnetActivationHistoryV1, _ *entity.FairLaunchMainnetComponentBindingV1, _ *entity.FairLaunchSourceRevisionBindingV3, _ *[sha256.Size]byte) {
			value.State = "admitted"
		}},
		{name: "component commit", mutate: func(_ *FairLaunchMainnetActivationRequest, _ *entity.FairLaunchAuthorityV1, _ *entity.FairLaunchMainnetActivationV1, _ *entity.FairLaunchMainnetActivationLeaseV1, _ *entity.FairLaunchMainnetActivationHistoryV1, value *entity.FairLaunchMainnetComponentBindingV1, _ *entity.FairLaunchSourceRevisionBindingV3, _ *[sha256.Size]byte) {
			value.GitCommit = repeatMainnetHex('f', 40)
		}},
		{name: "component set", mutate: func(_ *FairLaunchMainnetActivationRequest, _ *entity.FairLaunchAuthorityV1, _ *entity.FairLaunchMainnetActivationV1, value *entity.FairLaunchMainnetActivationLeaseV1, _ *entity.FairLaunchMainnetActivationHistoryV1, _ *entity.FairLaunchMainnetComponentBindingV1, _ *entity.FairLaunchSourceRevisionBindingV3, _ *[sha256.Size]byte) {
			value.ComponentSetDigest = repeatMainnetHex('f', 64)
		}},
		{name: "runtime code", mutate: func(_ *FairLaunchMainnetActivationRequest, _ *entity.FairLaunchAuthorityV1, value *entity.FairLaunchMainnetActivationV1, _ *entity.FairLaunchMainnetActivationLeaseV1, _ *entity.FairLaunchMainnetActivationHistoryV1, _ *entity.FairLaunchMainnetComponentBindingV1, _ *entity.FairLaunchSourceRevisionBindingV3, _ *[sha256.Size]byte) {
			value.ProxyRuntimeCodeHash = repeatMainnetHex('f', 64)
		}},
		{name: "source boundary", mutate: func(_ *FairLaunchMainnetActivationRequest, _ *entity.FairLaunchAuthorityV1, _ *entity.FairLaunchMainnetActivationV1, _ *entity.FairLaunchMainnetActivationLeaseV1, _ *entity.FairLaunchMainnetActivationHistoryV1, _ *entity.FairLaunchMainnetComponentBindingV1, value *entity.FairLaunchSourceRevisionBindingV3, _ *[sha256.Size]byte) {
			end := uint64(999)
			value.ToBlockNumber = &end
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidateRequest, candidateAuthority, candidateActivation, candidateLease, candidateHistory, candidateComponent, candidateSourceBinding, candidateSource, candidateRevision, candidateHash := fairLaunchMainnetCapabilityFixture(t)
			test.mutate(&candidateRequest, &candidateAuthority, &candidateActivation, &candidateLease, &candidateHistory, &candidateComponent, &candidateSourceBinding, &candidateHash)
			if err := validateFairLaunchMainnetAdmission(candidateRequest, candidateAuthority, candidateActivation, candidateLease, candidateHistory, candidateComponent, candidateSourceBinding, candidateSource, candidateRevision, candidateHash); err == nil || err.Error() != FairLaunchMainnetActivationUnavailable {
				t.Fatalf("mismatch must fail closed, got %v", err)
			}
		})
	}
}

func TestFairLaunchMainnetCapabilityUsesEarliestExpiry(t *testing.T) {
	now := time.Now().UTC()
	activationExpiry := now.Add(2 * time.Hour)
	leaseExpiry := now.Add(time.Hour)
	capability := FairLaunchMainnetCapability{seal: &fairLaunchMainnetSeal{admission: FairLaunchMainnetAdmission{ValidUntil: leaseExpiry}}}
	if !capability.valid(now) || capability.valid(leaseExpiry) || !activationExpiry.After(capability.seal.admission.ValidUntil) {
		t.Fatal("capability did not fail closed at the earliest authority expiry")
	}
}

func TestValidateFairLaunchMainnetAdmissionRequiresExactRPCAndSourceFinality(t *testing.T) {
	request, authority, activation, lease, history, component, sourceBinding, source, revision, leaseHash := fairLaunchMainnetCapabilityFixture(t)
	tests := []struct {
		name   string
		mutate func(*entity.FairLaunchMainnetActivationV1, *entity.ChainContractSource, *entity.ChainContractSourceRevision)
	}{
		{"rpc identity", func(value *entity.FairLaunchMainnetActivationV1, _ *entity.ChainContractSource, _ *entity.ChainContractSourceRevision) {
			value.RPCIdentityDigest = "bad"
		}},
		{"zero genesis", func(value *entity.FairLaunchMainnetActivationV1, _ *entity.ChainContractSource, _ *entity.ChainContractSourceRevision) {
			value.RPCGenesisHash = "0x" + repeatMainnetHex('0', 64)
		}},
		{"zero rpc identity", func(value *entity.FairLaunchMainnetActivationV1, _ *entity.ChainContractSource, _ *entity.ChainContractSourceRevision) {
			value.RPCIdentityDigest = repeatMainnetHex('0', 64)
		}},
		{"source registry digest", func(value *entity.FairLaunchMainnetActivationV1, _ *entity.ChainContractSource, _ *entity.ChainContractSourceRevision) {
			value.SourceRegistryDigest = "bad"
		}},
		{"zero source registry digest", func(value *entity.FairLaunchMainnetActivationV1, _ *entity.ChainContractSource, _ *entity.ChainContractSourceRevision) {
			value.SourceRegistryDigest = repeatMainnetHex('0', 64)
		}},
		{"source start hash", func(value *entity.FairLaunchMainnetActivationV1, _ *entity.ChainContractSource, _ *entity.ChainContractSourceRevision) {
			value.SourceStartBlockHash = "bad"
		}},
		{"source proxy", func(_ *entity.FairLaunchMainnetActivationV1, value *entity.ChainContractSource, _ *entity.ChainContractSourceRevision) {
			value.ContractAddress = "0xffffffffffffffffffffffffffffffffffffffff"
		}},
		{"confirmation policy", func(_ *entity.FairLaunchMainnetActivationV1, _ *entity.ChainContractSource, value *entity.ChainContractSourceRevision) {
			value.ConfirmationPolicy = "12_blocks"
		}},
		{"revision ABI", func(_ *entity.FairLaunchMainnetActivationV1, _ *entity.ChainContractSource, value *entity.ChainContractSourceRevision) {
			value.ABIDigest = repeatMainnetHex('f', 64)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidateActivation, candidateSource, candidateRevision := activation, source, revision
			test.mutate(&candidateActivation, &candidateSource, &candidateRevision)
			if err := validateFairLaunchMainnetAdmission(request, authority, candidateActivation, lease, history, component, sourceBinding, candidateSource, candidateRevision, leaseHash); err == nil || err.Error() != FairLaunchMainnetActivationUnavailable {
				t.Fatalf("mismatch must fail closed, got %v", err)
			}
		})
	}
}

func TestFairLaunchMainnetCapabilityExportsOnlyHashedLease(t *testing.T) {
	request, _, _, _, _, _, _, _, _, leaseHash := fairLaunchMainnetCapabilityFixture(t)
	now := time.Now().UTC()
	capability := FairLaunchMainnetCapability{seal: &fairLaunchMainnetSeal{admission: FairLaunchMainnetAdmission{AuthorityRowID: 7, ChainID: fairLaunchMainnetChainID, SourceBindingID: 9, SourceID: request.SourceID, SourceRevisionID: request.SourceRevisionID, FenceEpoch: 3, ValidUntil: now.Add(time.Hour)}, leaseHash: leaseHash}}
	authority, err := capability.v3Authority(now)
	if err != nil {
		t.Fatalf("valid capability did not export V3 authority: %v", err)
	}
	if authority.LeaseToken != hex.EncodeToString(leaseHash[:]) || authority.LeaseToken == string(request.LeaseToken) {
		t.Fatal("V3 authority did not use only the persisted lease hash")
	}
	if _, err := capability.v3Authority(capability.seal.admission.ValidUntil); err == nil || err.Error() != FairLaunchMainnetActivationUnavailable {
		t.Fatalf("expired capability exported authority: %v", err)
	}
}

func fairLaunchMainnetCapabilityFixture(t *testing.T) (FairLaunchMainnetActivationRequest, entity.FairLaunchAuthorityV1, entity.FairLaunchMainnetActivationV1, entity.FairLaunchMainnetActivationLeaseV1, entity.FairLaunchMainnetActivationHistoryV1, entity.FairLaunchMainnetComponentBindingV1, entity.FairLaunchSourceRevisionBindingV3, entity.ChainContractSource, entity.ChainContractSourceRevision, [sha256.Size]byte) {
	t.Helper()
	now := time.Now().UTC()
	leaseToken := []byte("0123456789abcdef0123456789abcdef")
	leaseHash := sha256.Sum256(leaseToken)
	request := FairLaunchMainnetActivationRequest{
		ActivationID: "12345678-1234-4abc-8def-1234567890ab", Component: "job", GitCommit: repeatMainnetHex('a', 40), ArtifactSHA256: repeatMainnetHex('b', 64), ComponentSetDigest: repeatMainnetHex('0', 64),
		ReleaseManifestDigest: repeatMainnetHex('c', 64), MigrationManifestDigest: repeatMainnetHex('d', 64), ActivationManifestDigest: repeatMainnetHex('e', 64),
		ProxyAddress: "0x1111111111111111111111111111111111111111", ProxyRuntimeCodeHash: repeatMainnetHex('1', 64), ImplementationAddress: "0x2222222222222222222222222222222222222222",
		ImplementationCodeHash: repeatMainnetHex('2', 64), ABIDigest: repeatMainnetHex('3', 64), SourceID: 11, SourceRevisionID: 12, LeaseToken: leaseToken,
	}
	authority := entity.FairLaunchAuthorityV1{ID: 7, AuthorityKind: fairLaunchMainnetAuthorityKind, AuthorityIdentity: request.ActivationID, Environment: fairLaunchMainnetEnvironment, ChainID: fairLaunchMainnetChainID, CreatedAt: now}
	activation := entity.FairLaunchMainnetActivationV1{AuthorityRowID: 7, ActivationID: request.ActivationID, Environment: fairLaunchMainnetEnvironment, ChainID: fairLaunchMainnetChainID, RPCGenesisHash: "0x" + repeatMainnetHex('8', 64), RPCIdentityDigest: repeatMainnetHex('9', 64), ProxyAddress: request.ProxyAddress, ProxyRuntimeCodeHash: request.ProxyRuntimeCodeHash, ImplementationAddress: request.ImplementationAddress, ImplementationCodeHash: request.ImplementationCodeHash, ABIDigest: request.ABIDigest, SourceID: request.SourceID, SourceRevisionID: request.SourceRevisionID, SourceStartBlock: 100, SourceStartBlockHash: "0x" + repeatMainnetHex('4', 64), SourceRegistryDigest: repeatMainnetHex('7', 64), ReleaseManifestDigest: request.ReleaseManifestDigest, MigrationManifestDigest: request.MigrationManifestDigest, ActivationManifestDigest: request.ActivationManifestDigest, ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(2 * time.Hour)}
	lease := entity.FairLaunchMainnetActivationLeaseV1{ID: 8, ActivationRowID: 7, ChainID: fairLaunchMainnetChainID, LeaseGeneration: 2, FenceEpoch: 3, LeaseTokenHash: hex.EncodeToString(leaseHash[:]), ComponentSetDigest: request.ComponentSetDigest, ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour), Status: "active"}
	history := entity.FairLaunchMainnetActivationHistoryV1{ActivationRowID: 7, ChainID: fairLaunchMainnetChainID, SequenceNo: 3, State: "source_activated"}
	component := entity.FairLaunchMainnetComponentBindingV1{ActivationRowID: 7, ChainID: fairLaunchMainnetChainID, Component: request.Component, GitCommit: request.GitCommit, ArtifactSHA256: request.ArtifactSHA256}
	sourceBinding := entity.FairLaunchSourceRevisionBindingV3{ID: 9, RunRowID: 7, ChainID: fairLaunchMainnetChainID, SourceID: request.SourceID, SourceRevisionID: request.SourceRevisionID, FromBlockNumber: activation.SourceStartBlock}
	source := entity.ChainContractSource{ID: request.SourceID, ChainID: fairLaunchMainnetChainID, ContractAddress: request.ProxyAddress, ContractVersion: fairLaunchLocalContractVersion, FundingMode: fairLaunchLocalFundingMode, StartBlock: activation.SourceStartBlock, Enabled: true}
	revision := entity.ChainContractSourceRevision{ID: request.SourceRevisionID, SourceID: request.SourceID, FromBlockNumber: activation.SourceStartBlock, ActivationBlockHash: activation.SourceStartBlockHash, ProxyKind: "uups", ImplementationAddress: request.ImplementationAddress, ImplementationCodeHash: request.ImplementationCodeHash, ABIDigest: request.ABIDigest, ConfirmationPolicy: "20_blocks"}
	return request, authority, activation, lease, history, component, sourceBinding, source, revision, leaseHash
}

func repeatMainnetHex(value byte, length int) string {
	result := make([]byte, length)
	for index := range result {
		result[index] = value
	}
	return string(result)
}

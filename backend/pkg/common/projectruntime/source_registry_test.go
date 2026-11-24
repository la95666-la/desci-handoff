package projectruntime

import (
	"strings"
	"testing"

	"joincare_server/pkg/common/constant"
)

func validSource() TrustedSource {
	return TrustedSource{
		Identity: SourceIdentity{
			ChainID: 56, ContractAddress: "0x1000000000000000000000000000000000000001",
			ContractVersion: constant.ContractVersionFairLaunchV1,
			FundingMode:     constant.FundingModeFairLaunch, Enabled: true,
		},
		Revision: SourceRevision{
			RevisionNo: 1, EffectiveFromBlock: 100,
			ActivationBlockHash:    "0x" + strings.Repeat("a", 64),
			ImplementationAddress:  "0x2000000000000000000000000000000000000002",
			ImplementationCodeHash: "0x" + strings.Repeat("b", 64),
			ABIVersion:             "fair-launch-v1", ABIDigest: "0x" + strings.Repeat("c", 64),
		},
	}
}

func TestSourceRegistryRoutesOnlyEnabledExplicitVersion(t *testing.T) {
	source := validSource()
	registry, err := NewSourceRegistry([]TrustedSource{source})
	if err != nil {
		t.Fatal(err)
	}
	got, err := registry.Source(constant.ContractVersionFairLaunchV1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Identity.ContractVersion != constant.ContractVersionFairLaunchV1 {
		t.Fatalf("wrong source: %+v", got)
	}
	if _, err := registry.Source("future_version"); err == nil {
		t.Fatal("unknown version fell back to an enabled ABI")
	}
}

func TestSourceRegistryDoesNotRouteDisabledSource(t *testing.T) {
	source := validSource()
	source.Identity.Enabled = false
	registry, err := NewSourceRegistry([]TrustedSource{source})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Source(source.Identity.ContractVersion); err == nil {
		t.Fatal("disabled source was routable")
	}
}

func TestValidateTrustedSourceRejectsPartialOrReversedBoundary(t *testing.T) {
	source := validSource()
	endBlock := uint64(100)
	source.Revision.EffectiveToBlock = &endBlock
	if err := ValidateTrustedSource(source); err == nil {
		t.Fatal("partial end boundary accepted")
	}
	endTx, endOrdinal := uint(0), uint(0)
	source.Revision.EffectiveToTxIndex = &endTx
	source.Revision.EffectiveToTxLogOrdinal = &endOrdinal
	if err := ValidateTrustedSource(source); err == nil {
		t.Fatal("non-increasing half-open range accepted")
	}
}

func TestValidateTrustedSourceRejectsMismatchedFundingFormula(t *testing.T) {
	source := validSource()
	source.Identity.FundingMode = constant.FundingModeCappedPresale
	if err := ValidateTrustedSource(source); err == nil {
		t.Fatal("mismatched funding mode/version accepted")
	}
}

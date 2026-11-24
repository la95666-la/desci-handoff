package projectruntime

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	"joincare_server/pkg/common/constant"
)

// SourceIdentity is the immutable address/version pair in the off-chain source
// registry. Signed envelope verification and persistence are deployment-gated;
// this type provides the fail-closed routing seam without enabling ingestion.
type SourceIdentity struct {
	ChainID         uint64 `json:"chainId"`
	ContractAddress string `json:"contractAddress"`
	ContractVersion string `json:"contractVersion"`
	FundingMode     string `json:"fundingMode"`
	Enabled         bool   `json:"enabled"`
}

// SourceRevision identifies one non-overlapping ABI/runtime range.
type SourceRevision struct {
	RevisionNo                uint32  `json:"revisionNo"`
	EffectiveFromBlock        uint64  `json:"effectiveFromBlock"`
	EffectiveFromTxIndex      uint    `json:"effectiveFromTxIndex"`
	EffectiveFromTxLogOrdinal uint    `json:"effectiveFromTxLogOrdinal"`
	EffectiveToBlock          *uint64 `json:"effectiveToBlock,omitempty"`
	EffectiveToTxIndex        *uint   `json:"effectiveToTxIndex,omitempty"`
	EffectiveToTxLogOrdinal   *uint   `json:"effectiveToTxLogOrdinal,omitempty"`
	ActivationBlockHash       string  `json:"activationBlockHash"`
	ImplementationAddress     string  `json:"implementationAddress"`
	ImplementationCodeHash    string  `json:"implementationCodeHash"`
	ABIVersion                string  `json:"abiVersion"`
	ABIDigest                 string  `json:"abiDigest"`
}

// TrustedSource is a validated in-memory materialization of a signed registry
// revision. Unknown versions or disabled sources are never routable.
type TrustedSource struct {
	Identity SourceIdentity `json:"identity"`
	Revision SourceRevision `json:"revision"`
}

// SourceRegistry validates and routes only explicitly registered sources.
type SourceRegistry struct {
	byVersion map[string]TrustedSource
}

func NewSourceRegistry(sources []TrustedSource) (*SourceRegistry, error) {
	registry := &SourceRegistry{byVersion: make(map[string]TrustedSource, len(sources))}
	seenAddresses := map[string]string{}
	for _, source := range sources {
		if err := ValidateTrustedSource(source); err != nil {
			return nil, err
		}
		if !source.Identity.Enabled {
			continue
		}
		version := source.Identity.ContractVersion
		if _, exists := registry.byVersion[version]; exists {
			return nil, fmt.Errorf("duplicate enabled source for contract version %q", version)
		}
		addressKey := strings.ToLower(source.Identity.ContractAddress)
		if previous, exists := seenAddresses[addressKey]; exists {
			return nil, fmt.Errorf("contract address is shared by enabled versions %q and %q", previous, version)
		}
		seenAddresses[addressKey] = version
		registry.byVersion[version] = canonicalSource(source)
	}
	return registry, nil
}

func (r *SourceRegistry) Source(contractVersion string) (TrustedSource, error) {
	if r == nil {
		return TrustedSource{}, errors.New("project contract source registry is unavailable")
	}
	source, ok := r.byVersion[contractVersion]
	if !ok {
		return TrustedSource{}, fmt.Errorf("project contract source %q is not enabled", contractVersion)
	}
	return source, nil
}

func ValidateTrustedSource(source TrustedSource) error {
	identity := source.Identity
	if identity.ChainID == 0 || !validAddress(identity.ContractAddress) {
		return errors.New("source chain and contract address are required")
	}
	if err := constantPair(identity.FundingMode, identity.ContractVersion); err != nil {
		return err
	}
	revision := source.Revision
	if revision.RevisionNo == 0 || revision.EffectiveFromBlock == 0 {
		return errors.New("source revision number and effective block must be positive")
	}
	if !validHash(revision.ActivationBlockHash) || !validAddress(revision.ImplementationAddress) || !validHash(revision.ImplementationCodeHash) || !validHash(revision.ABIDigest) || strings.TrimSpace(revision.ABIVersion) == "" {
		return errors.New("source revision runtime or ABI identity is invalid")
	}
	toSet := revision.EffectiveToBlock != nil || revision.EffectiveToTxIndex != nil || revision.EffectiveToTxLogOrdinal != nil
	if toSet && (revision.EffectiveToBlock == nil || revision.EffectiveToTxIndex == nil || revision.EffectiveToTxLogOrdinal == nil) {
		return errors.New("source revision end boundary must be entirely set or entirely absent")
	}
	if revision.EffectiveToBlock != nil && compareBoundary(
		*revision.EffectiveToBlock, *revision.EffectiveToTxIndex, *revision.EffectiveToTxLogOrdinal,
		revision.EffectiveFromBlock, revision.EffectiveFromTxIndex, revision.EffectiveFromTxLogOrdinal,
	) <= 0 {
		return errors.New("source revision end boundary must be after start boundary")
	}
	return nil
}

func constantPair(mode, version string) error {
	// Kept behind this small seam so source routing cannot silently infer an ABI.
	return constant.ValidateFundingModeContractVersion(mode, version)
}

func compareBoundary(aBlock uint64, aTx, aOrdinal uint, bBlock uint64, bTx, bOrdinal uint) int {
	if aBlock != bBlock {
		if aBlock < bBlock {
			return -1
		}
		return 1
	}
	if aTx != bTx {
		if aTx < bTx {
			return -1
		}
		return 1
	}
	if aOrdinal < bOrdinal {
		return -1
	}
	if aOrdinal > bOrdinal {
		return 1
	}
	return 0
}

func canonicalSource(source TrustedSource) TrustedSource {
	source.Identity.ContractAddress = common.HexToAddress(source.Identity.ContractAddress).Hex()
	source.Revision.ImplementationAddress = common.HexToAddress(source.Revision.ImplementationAddress).Hex()
	source.Revision.ActivationBlockHash = common.HexToHash(source.Revision.ActivationBlockHash).Hex()
	source.Revision.ImplementationCodeHash = common.HexToHash(source.Revision.ImplementationCodeHash).Hex()
	source.Revision.ABIDigest = common.HexToHash(source.Revision.ABIDigest).Hex()
	return source
}

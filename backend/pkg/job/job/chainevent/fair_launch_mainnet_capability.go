package chainevent

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"joincare_server/pkg/common/entity"
)

const (
	FairLaunchMainnetActivationUnavailable = "fair_launch_mainnet_activation_unavailable"
	fairLaunchMainnetEnvironment           = "test-server"
	fairLaunchMainnetAuthorityKind         = "test_server_bsc_mainnet"
	fairLaunchMainnetChainID               = uint64(56)
)

var fairLaunchMainnetComponents = map[string]bool{
	"backend":  true,
	"job":      true,
	"frontend": true,
	"admin":    true,
}

// FairLaunchMainnetActivationRequest contains only public immutable release
// identities plus an in-memory lease token. The token must come from a
// secret-bearing admission channel; callers must not put it in argv, ordinary
// environment variables, logs or persisted evidence.
type FairLaunchMainnetActivationRequest struct {
	ActivationID             string
	Component                string
	GitCommit                string
	ArtifactSHA256           string
	ComponentSetDigest       string
	ReleaseManifestDigest    string
	MigrationManifestDigest  string
	ActivationManifestDigest string
	ProxyAddress             string
	ProxyRuntimeCodeHash     string
	ImplementationAddress    string
	ImplementationCodeHash   string
	ABIDigest                string
	SourceID                 uint64
	SourceRevisionID         uint64
	LeaseToken               []byte
}

type FairLaunchMainnetAdmission struct {
	ActivationID             string
	AuthorityRowID           uint64
	ChainID                  uint64
	Component                string
	LeaseGeneration          uint64
	FenceEpoch               uint64
	RPCGenesisHash           string
	RPCIdentityDigest        string
	ProxyAddress             string
	ProxyRuntimeCodeHash     string
	ImplementationAddress    string
	ImplementationCodeHash   string
	ABIDigest                string
	SourceID                 uint64
	SourceRevisionID         uint64
	SourceBindingID          uint64
	SourceStartBlock         uint64
	SourceStartBlockHash     string
	SourceRegistryDigest     string
	ConfirmationPolicy       string
	ReleaseManifestDigest    string
	MigrationManifestDigest  string
	ActivationManifestDigest string
	ValidUntil               time.Time
	State                    string
	ProjectAdmissionCount    uint64
	PaymentTokenCount        uint64
}

// FairLaunchMainnetCapability is opaque outside this package. It is minted
// only from the persisted 0913 authority root and does not by itself start a
// scanner, enable a route, dispatch an action or submit a transaction.
type FairLaunchMainnetCapability struct {
	seal *fairLaunchMainnetSeal
}

type fairLaunchMainnetSeal struct {
	admission FairLaunchMainnetAdmission
	leaseHash [sha256.Size]byte
}

func (capability FairLaunchMainnetCapability) valid(now time.Time) bool {
	return capability.seal != nil && !now.IsZero() && now.Before(capability.seal.admission.ValidUntil)
}

func (capability FairLaunchMainnetCapability) Admission() (FairLaunchMainnetAdmission, error) {
	if capability.seal == nil {
		return FairLaunchMainnetAdmission{}, errors.New(FairLaunchMainnetActivationUnavailable)
	}
	return capability.seal.admission, nil
}

func (capability FairLaunchMainnetCapability) v3Authority(now time.Time) (fairLaunchV3Authority, error) {
	if !capability.valid(now) {
		return fairLaunchV3Authority{}, errors.New(FairLaunchMainnetActivationUnavailable)
	}
	admission := capability.seal.admission
	return fairLaunchV3Authority{
		RunRowID: admission.AuthorityRowID, ChainID: admission.ChainID, SourceBindingID: admission.SourceBindingID,
		SourceID: admission.SourceID, SourceRevisionID: admission.SourceRevisionID, LeaseToken: hex.EncodeToString(capability.seal.leaseHash[:]),
		FenceEpoch: admission.FenceEpoch, ContractVersion: fairLaunchLocalContractVersion,
		RejectedReason: FairLaunchMainnetActivationUnavailable, UnavailableReason: FairLaunchMainnetActivationUnavailable,
	}, nil
}

func LoadFairLaunchMainnetCapabilityTx(tx *gorm.DB, request FairLaunchMainnetActivationRequest, now time.Time) (FairLaunchMainnetCapability, error) {
	if tx == nil || now.IsZero() || !validFairLaunchMainnetActivationRequest(request) {
		return FairLaunchMainnetCapability{}, errors.New(FairLaunchMainnetActivationUnavailable)
	}
	var authority entity.FairLaunchAuthorityV1
	if err := tx.Where("authority_kind=? AND authority_identity=? AND environment=? AND chain_id=?", fairLaunchMainnetAuthorityKind, request.ActivationID, fairLaunchMainnetEnvironment, fairLaunchMainnetChainID).Take(&authority).Error; err != nil {
		return FairLaunchMainnetCapability{}, errors.New(FairLaunchMainnetActivationUnavailable)
	}
	var activation entity.FairLaunchMainnetActivationV1
	if err := tx.Where("authority_row_id=? AND activation_id=? AND environment=? AND chain_id=? AND valid_from<=? AND valid_until>?", authority.ID, request.ActivationID, fairLaunchMainnetEnvironment, fairLaunchMainnetChainID, now.UTC(), now.UTC()).Take(&activation).Error; err != nil {
		return FairLaunchMainnetCapability{}, errors.New(FairLaunchMainnetActivationUnavailable)
	}
	var lease entity.FairLaunchMainnetActivationLeaseV1
	if err := tx.Where("activation_row_id=? AND chain_id=? AND status='active' AND active_slot=1 AND valid_from<=? AND valid_until>?", authority.ID, fairLaunchMainnetChainID, now.UTC(), now.UTC()).Take(&lease).Error; err != nil {
		return FairLaunchMainnetCapability{}, errors.New(FairLaunchMainnetActivationUnavailable)
	}
	var history entity.FairLaunchMainnetActivationHistoryV1
	if err := tx.Where("activation_row_id=? AND chain_id=?", authority.ID, fairLaunchMainnetChainID).Order("sequence_no DESC").Take(&history).Error; err != nil {
		return FairLaunchMainnetCapability{}, errors.New(FairLaunchMainnetActivationUnavailable)
	}
	var component entity.FairLaunchMainnetComponentBindingV1
	if err := tx.Where("activation_row_id=? AND chain_id=? AND component=?", authority.ID, fairLaunchMainnetChainID, request.Component).Take(&component).Error; err != nil {
		return FairLaunchMainnetCapability{}, errors.New(FairLaunchMainnetActivationUnavailable)
	}
	var sourceBinding entity.FairLaunchSourceRevisionBindingV3
	if err := tx.Where("run_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=?", authority.ID, fairLaunchMainnetChainID, activation.SourceID, activation.SourceRevisionID).Take(&sourceBinding).Error; err != nil {
		return FairLaunchMainnetCapability{}, errors.New(FairLaunchMainnetActivationUnavailable)
	}
	var source entity.ChainContractSource
	if err := tx.Where("id=? AND chain_id=? AND contract_address=? AND contract_version=? AND funding_mode=? AND start_block=? AND enabled=1 AND retired_at IS NULL", activation.SourceID, fairLaunchMainnetChainID, activation.ProxyAddress, fairLaunchLocalContractVersion, fairLaunchLocalFundingMode, activation.SourceStartBlock).Take(&source).Error; err != nil {
		return FairLaunchMainnetCapability{}, errors.New(FairLaunchMainnetActivationUnavailable)
	}
	var revision entity.ChainContractSourceRevision
	if err := tx.Where("id=? AND source_id=? AND from_block_number=? AND from_transaction_index=0 AND from_tx_log_ordinal_inclusive=0 AND to_block_number IS NULL AND to_transaction_index IS NULL AND to_tx_log_ordinal_exclusive IS NULL", activation.SourceRevisionID, activation.SourceID, activation.SourceStartBlock).Take(&revision).Error; err != nil {
		return FairLaunchMainnetCapability{}, errors.New(FairLaunchMainnetActivationUnavailable)
	}
	var sourceWatermark int64
	if err := tx.Model(&entity.FairLaunchMainnetWatermarkV1{}).Where("activation_row_id=? AND chain_id=? AND watermark_kind='source_activated' AND project_id IS NULL", authority.ID, fairLaunchMainnetChainID).Count(&sourceWatermark).Error; err != nil || sourceWatermark != 1 {
		return FairLaunchMainnetCapability{}, errors.New(FairLaunchMainnetActivationUnavailable)
	}
	var projectCount, paymentTokenCount int64
	if err := tx.Model(&entity.FairLaunchMainnetProjectAdmissionV1{}).Where("activation_row_id=? AND chain_id=?", authority.ID, fairLaunchMainnetChainID).Count(&projectCount).Error; err != nil || projectCount <= 0 {
		return FairLaunchMainnetCapability{}, errors.New(FairLaunchMainnetActivationUnavailable)
	}
	if err := tx.Model(&entity.FairLaunchMainnetPaymentTokenV1{}).Where("activation_row_id=? AND chain_id=?", authority.ID, fairLaunchMainnetChainID).Count(&paymentTokenCount).Error; err != nil || paymentTokenCount <= 0 {
		return FairLaunchMainnetCapability{}, errors.New(FairLaunchMainnetActivationUnavailable)
	}
	leaseHash := sha256.Sum256(request.LeaseToken)
	if err := validateFairLaunchMainnetAdmission(request, authority, activation, lease, history, component, sourceBinding, source, revision, leaseHash); err != nil {
		return FairLaunchMainnetCapability{}, errors.New(FairLaunchMainnetActivationUnavailable)
	}
	validUntil := activation.ValidUntil
	if lease.ValidUntil.Before(validUntil) {
		validUntil = lease.ValidUntil
	}
	admission := FairLaunchMainnetAdmission{
		ActivationID: request.ActivationID, AuthorityRowID: authority.ID, ChainID: fairLaunchMainnetChainID, Component: request.Component,
		LeaseGeneration: lease.LeaseGeneration, FenceEpoch: lease.FenceEpoch, RPCGenesisHash: activation.RPCGenesisHash,
		RPCIdentityDigest: activation.RPCIdentityDigest, ProxyAddress: activation.ProxyAddress,
		ProxyRuntimeCodeHash: activation.ProxyRuntimeCodeHash, ImplementationAddress: activation.ImplementationAddress,
		ImplementationCodeHash: activation.ImplementationCodeHash, ABIDigest: activation.ABIDigest, SourceID: activation.SourceID,
		SourceRevisionID: activation.SourceRevisionID, SourceBindingID: sourceBinding.ID, SourceStartBlock: activation.SourceStartBlock,
		SourceStartBlockHash: activation.SourceStartBlockHash, SourceRegistryDigest: activation.SourceRegistryDigest, ConfirmationPolicy: revision.ConfirmationPolicy, ReleaseManifestDigest: activation.ReleaseManifestDigest,
		MigrationManifestDigest: activation.MigrationManifestDigest, ActivationManifestDigest: activation.ActivationManifestDigest,
		ValidUntil: validUntil, State: history.State, ProjectAdmissionCount: uint64(projectCount), PaymentTokenCount: uint64(paymentTokenCount),
	}
	return FairLaunchMainnetCapability{seal: &fairLaunchMainnetSeal{admission: admission, leaseHash: leaseHash}}, nil
}

func validFairLaunchMainnetActivationRequest(request FairLaunchMainnetActivationRequest) bool {
	return fairLaunchLocalRunPattern.MatchString(request.ActivationID) && fairLaunchMainnetComponents[request.Component] &&
		len(request.LeaseToken) >= 32 && len(request.LeaseToken) <= 256 && request.SourceID > 0 && request.SourceRevisionID > 0 &&
		fairLaunchLocalAddressPattern.MatchString(request.ProxyAddress) && fairLaunchLocalAddressPattern.MatchString(request.ImplementationAddress) &&
		fairLaunchLocalDigestPattern.MatchString(request.GitCommit) == false && len(request.GitCommit) == 40 && isLowerHex(request.GitCommit) &&
		validMainnetDigest(request.ArtifactSHA256) && validMainnetDigest(request.ComponentSetDigest) && validMainnetDigest(request.ReleaseManifestDigest) && validMainnetDigest(request.MigrationManifestDigest) &&
		validMainnetDigest(request.ActivationManifestDigest) && validMainnetDigest(request.ProxyRuntimeCodeHash) && validMainnetDigest(request.ImplementationCodeHash) && validMainnetDigest(request.ABIDigest)
}

func validateFairLaunchMainnetAdmission(request FairLaunchMainnetActivationRequest, authority entity.FairLaunchAuthorityV1, activation entity.FairLaunchMainnetActivationV1, lease entity.FairLaunchMainnetActivationLeaseV1, history entity.FairLaunchMainnetActivationHistoryV1, component entity.FairLaunchMainnetComponentBindingV1, sourceBinding entity.FairLaunchSourceRevisionBindingV3, source entity.ChainContractSource, revision entity.ChainContractSourceRevision, leaseHash [sha256.Size]byte) error {
	storedLeaseHash, err := hex.DecodeString(lease.LeaseTokenHash)
	if err != nil || len(storedLeaseHash) != sha256.Size || subtle.ConstantTimeCompare(storedLeaseHash, leaseHash[:]) != 1 {
		return errors.New(FairLaunchMainnetActivationUnavailable)
	}
	if authority.ID == 0 || activation.AuthorityRowID != authority.ID || lease.ActivationRowID != authority.ID || component.ActivationRowID != authority.ID || sourceBinding.RunRowID != authority.ID ||
		authority.AuthorityKind != fairLaunchMainnetAuthorityKind || authority.Environment != fairLaunchMainnetEnvironment || authority.ChainID != fairLaunchMainnetChainID ||
		activation.Environment != fairLaunchMainnetEnvironment || activation.ChainID != fairLaunchMainnetChainID || lease.ChainID != fairLaunchMainnetChainID || component.ChainID != fairLaunchMainnetChainID || sourceBinding.ChainID != fairLaunchMainnetChainID ||
		!validMainnetNonzeroHash(activation.RPCGenesisHash) || !validMainnetNonzeroDigest(activation.RPCIdentityDigest) || !validMainnetNonzeroDigest(activation.SourceRegistryDigest) || !fairLaunchLocalHashPattern.MatchString(activation.SourceStartBlockHash) ||
		lease.LeaseGeneration == 0 || lease.FenceEpoch == 0 || history.State != "source_activated" && history.State != "wallet_enabled" ||
		lease.ComponentSetDigest != request.ComponentSetDigest ||
		component.Component != request.Component || component.GitCommit != request.GitCommit || component.ArtifactSHA256 != request.ArtifactSHA256 ||
		activation.ProxyAddress != request.ProxyAddress || activation.ProxyRuntimeCodeHash != request.ProxyRuntimeCodeHash || activation.ImplementationAddress != request.ImplementationAddress ||
		activation.ImplementationCodeHash != request.ImplementationCodeHash || activation.ABIDigest != request.ABIDigest || activation.SourceID != request.SourceID || activation.SourceRevisionID != request.SourceRevisionID ||
		activation.ReleaseManifestDigest != request.ReleaseManifestDigest || activation.MigrationManifestDigest != request.MigrationManifestDigest || activation.ActivationManifestDigest != request.ActivationManifestDigest ||
		sourceBinding.SourceID != activation.SourceID || sourceBinding.SourceRevisionID != activation.SourceRevisionID || sourceBinding.FromBlockNumber != activation.SourceStartBlock || sourceBinding.ToBlockNumber != nil ||
		source.ID != activation.SourceID || source.ChainID != fairLaunchMainnetChainID || source.ContractAddress != activation.ProxyAddress || source.ContractVersion != fairLaunchLocalContractVersion || source.FundingMode != fairLaunchLocalFundingMode || source.StartBlock != activation.SourceStartBlock || !source.Enabled || source.RetiredAt != nil ||
		revision.ID != activation.SourceRevisionID || revision.SourceID != activation.SourceID || revision.FromBlockNumber != activation.SourceStartBlock || revision.FromTransactionIndex != 0 || revision.FromTxLogOrdinalInclusive != 0 || revision.ToBlockNumber != nil || revision.ToTransactionIndex != nil || revision.ToTxLogOrdinalExclusive != nil || revision.ActivationBlockHash != activation.SourceStartBlockHash || revision.ProxyKind != "uups" || revision.ImplementationAddress != activation.ImplementationAddress || revision.ImplementationCodeHash != activation.ImplementationCodeHash || revision.ABIDigest != activation.ABIDigest || revision.ConfirmationPolicy != "20_blocks" {
		return errors.New(FairLaunchMainnetActivationUnavailable)
	}
	return nil
}

func validMainnetDigest(value string) bool { return fairLaunchLocalDigestPattern.MatchString(value) }

func validMainnetNonzeroHash(value string) bool {
	return fairLaunchLocalHashPattern.MatchString(value) && value != "0x"+strings.Repeat("0", 64)
}

func validMainnetNonzeroDigest(value string) bool {
	return validMainnetDigest(value) && value != strings.Repeat("0", 64)
}

func isLowerHex(value string) bool {
	if value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

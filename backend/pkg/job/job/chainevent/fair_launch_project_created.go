package chainevent

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/money"
)

const FairLaunchMainnetProjectBootstrapUnavailable = "fair_launch_mainnet_project_bootstrap_unavailable"

type FairLaunchMainnetProjectBootstrapResult struct {
	ProjectRowID, ProjectBindingID, ProjectStateID   uint64
	EventIdentityID, BootstrapReceiptID, WatermarkID uint64
	Replayed                                         bool
}

type fairLaunchProjectCreatedFacts struct {
	ProjectID                                                              uint64
	ProjectIDRaw, ProjectToken, PaymentToken, ProjectOwner                 string
	ProjectProceedsReceiver, ProjectProceedsReceiverAttestationDigest      string
	ProjectProceedsReceiverAttestationVersion                              uint32
	PlatformFeeReceiver, PlatformFeeReceiverAttestationDigest              string
	PlatformFeeReceiverAttestationVersion                                  uint32
	PlatformFeeBPS                                                         uint16
	SaleTokenAmountRaw, SoftCapRaw, MinContributionRaw, MaxContributionRaw string
	StartTime, EndTime                                                     uint64
	ProjectTokenPolicyDigest                                               string
	ProjectTokenPolicyVersion                                              uint32
	PaymentTokenPolicyDigest                                               string
	PaymentTokenPolicyVersion                                              uint32
	ReleasePolicyDigest                                                    string
	ReleasePolicyVersion                                                   uint32
}

type fairLaunchPreparedMetadata struct {
	FairLaunch struct {
		SaleTokenAmountRaw, SoftCapRaw, MinContributionRaw, MaxContributionRaw string
		ProjectProceedsReceiver, ProjectProceedsReceiverAttestationDigest      string
		ProjectProceedsReceiverAttestationVersion                              uint32
		PlatformFeeReceiverAttestationDigest                                   string
		PlatformFeeReceiverAttestationVersion                                  uint32
		ProjectTokenPolicyDigest                                               string
		ProjectTokenPolicyVersion                                              uint32
		PaymentTokenPolicyDigest                                               string
		PaymentTokenPolicyVersion                                              uint32
		ReleasePolicyDigest                                                    string
		ReleasePolicyVersion                                                   uint32
	} `json:"fairLaunch"`
}

// BootstrapFairLaunchMainnetProjectCreatedTx binds a finalized canonical
// ProjectCreated receipt to the exact mainnet activation, project allowlist and
// pre-created content row. The caller owns the transaction boundary.
func BootstrapFairLaunchMainnetProjectCreatedTx(capability FairLaunchMainnetCapability, tx *gorm.DB, in FairLaunchCEWrite, archive fairLaunchV3ArchiveEvidence, now time.Time) (FairLaunchMainnetProjectBootstrapResult, error) {
	if tx == nil || in.Event == nil || in.Event.Name != "FairLaunchProjectCreated" || now.IsZero() || !capability.valid(now) {
		return FairLaunchMainnetProjectBootstrapResult{}, errors.New(FairLaunchMainnetProjectBootstrapUnavailable)
	}
	authority, err := capability.v3Authority(now)
	if err != nil || authority.ChainID != fairLaunchMainnetChainID || in.Context.ChainID != authority.ChainID || in.SourceID != authority.SourceID || in.Context.SourceRevisionID != authority.SourceRevisionID || in.Context.EffectVersion != FairLaunchEffectManifestVersion || !in.Context.Canonical || !in.Context.Finalized || in.LeaseToken != authority.LeaseToken || in.FenceEpoch != authority.FenceEpoch || archive.IdentityID == 0 || archive.PlacementID == 0 || archive.ObservationID == 0 || archive.PlacementGeneration == 0 || archive.SourceRevisionID != authority.SourceRevisionID || archive.BlockNumber != in.Event.Log.BlockNumber || archive.BlockHash != strings.ToLower(in.Event.Log.BlockHash.Hex()) || archive.TransactionIndex != in.TransactionIndex || archive.TxLogOrdinal != in.Context.TxLogOrdinal || archive.RPCLogIndex != uint32(in.Event.Log.Index) {
		return FairLaunchMainnetProjectBootstrapResult{}, errors.New(FairLaunchMainnetProjectBootstrapUnavailable)
	}
	admission, err := capability.Admission()
	if err != nil || strings.ToLower(in.Event.Log.Address.Hex()) != admission.ProxyAddress || archive.BlockNumber < admission.SourceStartBlock {
		return FairLaunchMainnetProjectBootstrapResult{}, errors.New(FairLaunchMainnetProjectBootstrapUnavailable)
	}
	facts, err := decodeFairLaunchProjectCreatedFacts(in.Event)
	if err != nil {
		return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError(err.Error())
	}
	return bootstrapFairLaunchMainnetProjectCreatedTx(capability, tx, in, archive, facts, now)
}

func bootstrapFairLaunchMainnetProjectCreatedTx(capability FairLaunchMainnetCapability, tx *gorm.DB, in FairLaunchCEWrite, archive fairLaunchV3ArchiveEvidence, facts fairLaunchProjectCreatedFacts, now time.Time) (FairLaunchMainnetProjectBootstrapResult, error) {
	authority, err := capability.v3Authority(now)
	if err != nil {
		return FairLaunchMainnetProjectBootstrapResult{}, err
	}
	// Serialize ProjectCreated bootstrap decisions for one activation. The
	// watermark table's historical unique key includes project_id, so locking
	// only matching watermark rows would allow two different projects to both
	// observe an empty first-project set and mint competing roots.
	var activation entity.FairLaunchMainnetActivationV1
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("authority_row_id=? AND chain_id=? AND source_id=? AND source_revision_id=?", authority.RunRowID, authority.ChainID, authority.SourceID, authority.SourceRevisionID).Take(&activation).Error; err != nil {
		return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError("activation authority unavailable")
	}
	var identity entity.ChainEventIdentityV2
	if tx.Where("id=? AND chain_id=? AND tx_hash=? AND tx_log_ordinal=?", archive.IdentityID, authority.ChainID, strings.ToLower(in.Event.Log.TxHash.Hex()), archive.TxLogOrdinal).Take(&identity).Error != nil {
		return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError("canonical identity unavailable")
	}
	var placement entity.ChainEventPlacementV2
	if tx.Where("id=? AND event_identity_id=? AND generation=? AND block_number=? AND block_hash=? AND transaction_index=? AND rpc_log_index=? AND source_revision_id=? AND canonical_slot=1 AND finalized_at IS NOT NULL AND noncanonical_at IS NULL", archive.PlacementID, identity.ID, archive.PlacementGeneration, archive.BlockNumber, archive.BlockHash, archive.TransactionIndex, archive.RPCLogIndex, archive.SourceRevisionID).Take(&placement).Error != nil {
		return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError("finalized placement unavailable")
	}
	var observation entity.ChainEventPlacementObservationV2
	if tx.Where("id=? AND placement_id=? AND event_identity_id=? AND provider_identity=? AND emitting_address=? AND topics_data_digest=? AND raw_log_digest=?", archive.ObservationID, placement.ID, identity.ID, in.ProviderIdentity, strings.ToLower(in.Event.Log.Address.Hex()), stripCE(in.TopicsDataDigest), stripCE(in.RawLogDigest)).Take(&observation).Error != nil {
		return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError("archived observation unavailable")
	}
	var conflicts int64
	if tx.Model(&entity.ChainEventPayloadConflictV2{}).Where("event_identity_id=? AND state='open'", identity.ID).Count(&conflicts).Error != nil || conflicts != 0 {
		return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError("payload conflict")
	}
	var admission entity.FairLaunchMainnetProjectAdmissionV1
	if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("activation_row_id=? AND chain_id=? AND project_id=?", authority.RunRowID, authority.ChainID, facts.ProjectID).Take(&admission).Error != nil {
		return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError("project is not admitted")
	}
	var payment entity.FairLaunchMainnetPaymentTokenV1
	if tx.Where("activation_row_id=? AND chain_id=? AND token_address=? AND token_decimals=?", authority.RunRowID, authority.ChainID, facts.PaymentToken, admission.PaymentTokenDecimals).Take(&payment).Error != nil {
		return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError("payment token is not admitted")
	}
	var project entity.Project
	if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND project_id=?", admission.ProjectRowID, admission.ProjectID).Take(&project).Error != nil {
		return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError("admitted project row unavailable")
	}
	var watermarks []entity.FairLaunchMainnetWatermarkV1
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("activation_row_id=? AND chain_id=? AND watermark_kind='first_project_created'", authority.RunRowID, authority.ChainID).Limit(2).Find(&watermarks).Error; err != nil || len(watermarks) > 1 {
		return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError("first project watermark unavailable")
	}
	firstProject := len(watermarks) == 0
	var watermark entity.FairLaunchMainnetWatermarkV1
	if !firstProject {
		watermark = watermarks[0]
		if watermark.ProjectID == nil || watermark.EventIdentityID == nil {
			return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError("first project watermark invalid")
		}
		if *watermark.ProjectID == facts.ProjectID && *watermark.EventIdentityID == identity.ID {
			return validateFairLaunchProjectCreatedReplayTx(tx, authority, project, admission, facts, identity.ID, placement.ID, observation.ID, in.Event, &watermark)
		}
		if *watermark.ProjectID == facts.ProjectID || *watermark.EventIdentityID == identity.ID {
			return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError("first project watermark conflict")
		}
		if err := validateFairLaunchMainnetFirstProjectWatermarkTx(tx, authority, watermark); err != nil {
			return FairLaunchMainnetProjectBootstrapResult{}, err
		}
		if project.OnchainLocked {
			return validateFairLaunchProjectCreatedReplayTx(tx, authority, project, admission, facts, identity.ID, placement.ID, observation.ID, in.Event, nil)
		}
	}
	if err := validateFairLaunchProjectCreatedAdmission(capability, admission, payment, project, facts); err != nil {
		return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError(err.Error())
	}
	if err := validateFairLaunchMainnetTokenBindingsTx(tx, authority, admission, payment, strings.ToLower(in.Event.Log.Address.Hex())); err != nil {
		return FairLaunchMainnetProjectBootstrapResult{}, err
	}
	payloadDigest, err := commonchain.FairLaunchTypedEventDigest(in.Event)
	if err != nil {
		return FairLaunchMainnetProjectBootstrapResult{}, err
	}
	bindingProjection := entity.FairLaunchProjectBindingV3{RunRowID: authority.RunRowID, ProjectRowID: project.ID, ChainID: authority.ChainID, ContractAddress: strings.ToLower(in.Event.Log.Address.Hex()), ContractVersion: authority.ContractVersion, ProjectID: facts.ProjectID, SourceBindingID: authority.SourceBindingID, SourceID: authority.SourceID, SourceRevisionID: authority.SourceRevisionID, ProjectTokenAddress: facts.ProjectToken, ProjectTokenDecimals: admission.ProjectTokenDecimals, PaymentTokenAddress: facts.PaymentToken, PaymentTokenDecimals: admission.PaymentTokenDecimals}
	bindingDigest := fairLaunchProjectBindingDigest(bindingProjection)
	state := initialFairLaunchProjectState(authority, facts, in.Event.Log.Address, identity.ID, placement.ID, now)
	watermarkDigest := fairLaunchV3DomainDigest("JC_FL_MAINNET_PROJECT_CREATED_WATERMARK_V1", map[string]any{"activationRowId": authority.RunRowID, "chainId": authority.ChainID, "sourceId": authority.SourceID, "sourceRevisionId": authority.SourceRevisionID, "projectId": facts.ProjectID, "eventIdentityId": identity.ID, "placementId": placement.ID, "observationId": observation.ID, "payloadDigest": payloadDigest, "bindingDigest": bindingDigest, "stateDigest": state.StateDigest})
	var bindingConflicts, stateConflicts int64
	if tx.Model(&entity.FairLaunchProjectBindingV3{}).Where("(chain_id=? AND contract_address=? AND contract_version=? AND project_id=?) OR (run_row_id=? AND project_row_id=?)", authority.ChainID, strings.ToLower(in.Event.Log.Address.Hex()), authority.ContractVersion, facts.ProjectID, authority.RunRowID, project.ID).Count(&bindingConflicts).Error != nil || bindingConflicts != 0 {
		return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError("project binding conflict")
	}
	if tx.Model(&entity.FairLaunchProjectStateV3{}).Where("chain_id=? AND contract_address=? AND contract_version=? AND project_id=?", authority.ChainID, strings.ToLower(in.Event.Log.Address.Hex()), authority.ContractVersion, facts.ProjectID).Count(&stateConflicts).Error != nil || stateConflicts != 0 {
		return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError("project state conflict")
	}
	binding := entity.FairLaunchProjectBindingV3{RunRowID: authority.RunRowID, ProjectRowID: project.ID, ChainID: authority.ChainID, ContractAddress: strings.ToLower(in.Event.Log.Address.Hex()), ContractVersion: authority.ContractVersion, ProjectID: facts.ProjectID, ProjectIDRaw: facts.ProjectIDRaw, SourceBindingID: authority.SourceBindingID, SourceID: authority.SourceID, SourceRevisionID: authority.SourceRevisionID, ProjectTokenAddress: facts.ProjectToken, ProjectTokenDecimals: admission.ProjectTokenDecimals, PaymentTokenAddress: facts.PaymentToken, PaymentTokenDecimals: admission.PaymentTokenDecimals, BindingDigest: bindingDigest, CreatedAt: now}
	if err := tx.Create(&binding).Error; err != nil {
		return FairLaunchMainnetProjectBootstrapResult{}, err
	}
	state.ProjectBindingID = binding.ID
	receipt := entity.FairLaunchMainnetProjectBootstrapReceiptV1{ActivationRowID: authority.RunRowID, ChainID: authority.ChainID, ProjectBindingID: binding.ID, ProjectID: facts.ProjectID, EventIdentityID: identity.ID, PlacementID: placement.ID, ObservationID: observation.ID, SourceID: authority.SourceID, SourceRevisionID: authority.SourceRevisionID, PlacementGeneration: placement.Generation, EffectManifestVersion: FairLaunchEffectManifestVersion, PayloadDigest: payloadDigest, BindingDigest: bindingDigest, InitialStateDigest: state.StateDigest, CreatedAt: now}
	receipt.ReceiptDigest = fairLaunchV3DomainDigest("JC_FL_MAINNET_PROJECT_BOOTSTRAP_RECEIPT_V1", map[string]any{"activationRowId": receipt.ActivationRowID, "chainId": receipt.ChainID, "projectBindingId": receipt.ProjectBindingID, "projectId": receipt.ProjectID, "eventIdentityId": receipt.EventIdentityID, "placementId": receipt.PlacementID, "observationId": receipt.ObservationID, "sourceId": receipt.SourceID, "sourceRevisionId": receipt.SourceRevisionID, "placementGeneration": receipt.PlacementGeneration, "effectManifestVersion": receipt.EffectManifestVersion, "payloadDigest": receipt.PayloadDigest, "bindingDigest": receipt.BindingDigest, "initialStateDigest": receipt.InitialStateDigest})
	if err := tx.Create(&receipt).Error; err != nil {
		return FairLaunchMainnetProjectBootstrapResult{}, err
	}
	state.BootstrapReceiptID = uint64Pointer(receipt.ID)
	if err := tx.Create(&state).Error; err != nil {
		return FairLaunchMainnetProjectBootstrapResult{}, err
	}
	if firstProject {
		watermark = entity.FairLaunchMainnetWatermarkV1{ActivationRowID: authority.RunRowID, ChainID: authority.ChainID, WatermarkKind: "first_project_created", ProjectID: uint64Pointer(facts.ProjectID), EventIdentityID: uint64Pointer(identity.ID), EvidenceDigest: watermarkDigest, CreatedAt: now}
		if err := tx.Create(&watermark).Error; err != nil {
			return FairLaunchMainnetProjectBootstrapResult{}, err
		}
	}
	if err := finalizeFairLaunchMainnetProjectRowTx(tx, project, identity.ID, placement.ID, payloadDigest, now); err != nil {
		return FairLaunchMainnetProjectBootstrapResult{}, err
	}
	watermarkID := uint64(0)
	if firstProject {
		watermarkID = watermark.ID
	}
	return FairLaunchMainnetProjectBootstrapResult{ProjectRowID: project.ID, ProjectBindingID: binding.ID, ProjectStateID: state.ID, EventIdentityID: identity.ID, BootstrapReceiptID: receipt.ID, WatermarkID: watermarkID}, nil
}

func validateFairLaunchMainnetFirstProjectWatermarkTx(tx *gorm.DB, authority fairLaunchV3Authority, watermark entity.FairLaunchMainnetWatermarkV1) error {
	if watermark.ActivationRowID != authority.RunRowID || watermark.ChainID != authority.ChainID || watermark.WatermarkKind != "first_project_created" || watermark.ProjectID == nil || watermark.EventIdentityID == nil {
		return bootstrapError("first project watermark authority mismatch")
	}
	var receipt entity.FairLaunchMainnetProjectBootstrapReceiptV1
	if err := tx.Where("activation_row_id=? AND chain_id=? AND project_id=? AND event_identity_id=? AND source_id=? AND source_revision_id=? AND effect_manifest_version=?", authority.RunRowID, authority.ChainID, *watermark.ProjectID, *watermark.EventIdentityID, authority.SourceID, authority.SourceRevisionID, FairLaunchEffectManifestVersion).Take(&receipt).Error; err != nil {
		return bootstrapError("first project bootstrap receipt unavailable")
	}
	var binding entity.FairLaunchProjectBindingV3
	if err := tx.Where("id=? AND run_row_id=? AND chain_id=? AND source_binding_id=? AND source_id=? AND source_revision_id=? AND project_id=?", receipt.ProjectBindingID, authority.RunRowID, authority.ChainID, authority.SourceBindingID, authority.SourceID, authority.SourceRevisionID, *watermark.ProjectID).Take(&binding).Error; err != nil || binding.BindingDigest != fairLaunchProjectBindingDigest(binding) || receipt.BindingDigest != binding.BindingDigest {
		return bootstrapError("first project binding unavailable")
	}
	expectedReceiptDigest := fairLaunchV3DomainDigest("JC_FL_MAINNET_PROJECT_BOOTSTRAP_RECEIPT_V1", map[string]any{"activationRowId": receipt.ActivationRowID, "chainId": receipt.ChainID, "projectBindingId": receipt.ProjectBindingID, "projectId": receipt.ProjectID, "eventIdentityId": receipt.EventIdentityID, "placementId": receipt.PlacementID, "observationId": receipt.ObservationID, "sourceId": receipt.SourceID, "sourceRevisionId": receipt.SourceRevisionID, "placementGeneration": receipt.PlacementGeneration, "effectManifestVersion": receipt.EffectManifestVersion, "payloadDigest": receipt.PayloadDigest, "bindingDigest": receipt.BindingDigest, "initialStateDigest": receipt.InitialStateDigest})
	expectedWatermarkDigest := fairLaunchV3DomainDigest("JC_FL_MAINNET_PROJECT_CREATED_WATERMARK_V1", map[string]any{"activationRowId": authority.RunRowID, "chainId": authority.ChainID, "sourceId": authority.SourceID, "sourceRevisionId": authority.SourceRevisionID, "projectId": receipt.ProjectID, "eventIdentityId": receipt.EventIdentityID, "placementId": receipt.PlacementID, "observationId": receipt.ObservationID, "payloadDigest": receipt.PayloadDigest, "bindingDigest": receipt.BindingDigest, "stateDigest": receipt.InitialStateDigest})
	if receipt.ReceiptDigest != expectedReceiptDigest || watermark.EvidenceDigest != expectedWatermarkDigest {
		return bootstrapError("first project watermark digest mismatch")
	}
	return nil
}

func fairLaunchProjectBindingDigest(binding entity.FairLaunchProjectBindingV3) string {
	return fairLaunchV3DomainDigest("JC_FL_PROJECT_BINDING_V1", map[string]any{"runRowId": binding.RunRowID, "projectRowId": binding.ProjectRowID, "chainId": binding.ChainID, "contractAddress": binding.ContractAddress, "contractVersion": binding.ContractVersion, "projectId": binding.ProjectID, "sourceBindingId": binding.SourceBindingID, "sourceId": binding.SourceID, "sourceRevisionId": binding.SourceRevisionID, "projectTokenAddress": binding.ProjectTokenAddress, "projectTokenDecimals": binding.ProjectTokenDecimals, "paymentTokenAddress": binding.PaymentTokenAddress, "paymentTokenDecimals": binding.PaymentTokenDecimals})
}

func decodeFairLaunchProjectCreatedFacts(event *commonchain.FairLaunchTypedEvent) (fairLaunchProjectCreatedFacts, error) {
	if event == nil || event.Name != "FairLaunchProjectCreated" || event.ProjectID == nil || event.ProjectID.Sign() <= 0 || !event.ProjectID.IsUint64() {
		return fairLaunchProjectCreatedFacts{}, errors.New("project-created event identity invalid")
	}
	address := func(index int) (string, error) {
		key := fmt.Sprintf("arg%d", index)
		for _, values := range []map[string]any{event.Indexed, event.Values} {
			if value, ok := values[key].(common.Address); ok && value != (common.Address{}) {
				return strings.ToLower(value.Hex()), nil
			}
		}
		return "", fmt.Errorf("%s address invalid", key)
	}
	digest := func(index int) (string, error) {
		value, ok := event.Values[fmt.Sprintf("arg%d", index)].([32]byte)
		if !ok {
			return "", fmt.Errorf("arg%d digest invalid", index)
		}
		return hex.EncodeToString(value[:]), nil
	}
	raw := func(index int, positive bool) (string, error) {
		value, ok := event.RawDecimal[fmt.Sprintf("arg%d", index)]
		if !ok {
			return "", fmt.Errorf("arg%d raw integer missing", index)
		}
		canonical, err := money.NormalizeRawUint256(value)
		if err != nil || canonical != value || positive && canonical == "0" {
			return "", fmt.Errorf("arg%d raw integer invalid", index)
		}
		return canonical, nil
	}
	u64 := func(index int) (uint64, error) {
		value, err := raw(index, false)
		if err != nil {
			return 0, err
		}
		parsed, ok := new(big.Int).SetString(value, 10)
		if !ok || !parsed.IsUint64() {
			return 0, fmt.Errorf("arg%d uint64 invalid", index)
		}
		return parsed.Uint64(), nil
	}
	u32 := func(index int) (uint32, error) {
		value, err := u64(index)
		if err != nil || value == 0 || value > uint64(^uint32(0)) {
			return 0, fmt.Errorf("arg%d uint32 invalid", index)
		}
		return uint32(value), nil
	}
	facts := fairLaunchProjectCreatedFacts{ProjectID: event.ProjectID.Uint64(), ProjectIDRaw: event.ProjectID.String()}
	var err error
	if facts.ProjectToken, err = address(1); err != nil {
		return facts, err
	}
	if facts.PaymentToken, err = address(2); err != nil {
		return facts, err
	}
	if facts.ProjectOwner, err = address(3); err != nil {
		return facts, err
	}
	if facts.ProjectProceedsReceiver, err = address(4); err != nil {
		return facts, err
	}
	if facts.ProjectProceedsReceiverAttestationDigest, err = digest(5); err != nil {
		return facts, err
	}
	if facts.ProjectProceedsReceiverAttestationVersion, err = u32(6); err != nil {
		return facts, err
	}
	if facts.PlatformFeeReceiver, err = address(7); err != nil {
		return facts, err
	}
	if facts.PlatformFeeReceiverAttestationDigest, err = digest(8); err != nil {
		return facts, err
	}
	if facts.PlatformFeeReceiverAttestationVersion, err = u32(9); err != nil {
		return facts, err
	}
	fee, err := u64(10)
	if err != nil || fee > 10000 {
		return facts, errors.New("arg10 uint16 invalid")
	}
	facts.PlatformFeeBPS = uint16(fee)
	if facts.SaleTokenAmountRaw, err = raw(11, true); err != nil {
		return facts, err
	}
	if facts.SoftCapRaw, err = raw(12, true); err != nil {
		return facts, err
	}
	if facts.MinContributionRaw, err = raw(13, true); err != nil {
		return facts, err
	}
	if facts.MaxContributionRaw, err = raw(14, true); err != nil {
		return facts, err
	}
	if facts.StartTime, err = u64(15); err != nil {
		return facts, err
	}
	if facts.EndTime, err = u64(16); err != nil {
		return facts, err
	}
	if facts.ProjectTokenPolicyDigest, err = digest(17); err != nil {
		return facts, err
	}
	if facts.ProjectTokenPolicyVersion, err = u32(18); err != nil {
		return facts, err
	}
	if facts.PaymentTokenPolicyDigest, err = digest(19); err != nil {
		return facts, err
	}
	if facts.PaymentTokenPolicyVersion, err = u32(20); err != nil {
		return facts, err
	}
	if facts.ReleasePolicyDigest, err = digest(21); err != nil {
		return facts, err
	}
	if facts.ReleasePolicyVersion, err = u32(22); err != nil {
		return facts, err
	}
	if facts.ProjectToken == facts.PaymentToken || facts.StartTime == 0 || facts.EndTime <= facts.StartTime || compareRawUint(facts.MinContributionRaw, facts.MaxContributionRaw) > 0 {
		return facts, errors.New("project-created values invalid")
	}
	return facts, nil
}

func validateFairLaunchProjectCreatedAdmission(capability FairLaunchMainnetCapability, admission entity.FairLaunchMainnetProjectAdmissionV1, payment entity.FairLaunchMainnetPaymentTokenV1, project entity.Project, facts fairLaunchProjectCreatedFacts) error {
	authority := capability.seal.admission
	if !fairLaunchLocalDigestPattern.MatchString(admission.AdmissionDigest) {
		return errors.New("project admission digest invalid")
	}
	if admission.ActivationRowID != authority.AuthorityRowID || admission.ChainID != authority.ChainID || admission.ProjectID != facts.ProjectID || admission.ProjectRowID != project.ID || admission.PaymentTokenAddress != facts.PaymentToken || admission.ProjectTokenAddress != facts.ProjectToken || admission.PaymentTokenDecimals != payment.TokenDecimals || payment.ActivationRowID != admission.ActivationRowID || payment.ChainID != admission.ChainID || payment.TokenAddress != facts.PaymentToken || payment.TokenDecimals != 18 || !fairLaunchLocalDigestPattern.MatchString(payment.BindingDigest) || payment.PolicyDigest != facts.PaymentTokenPolicyDigest || payment.PolicyVersion != facts.PaymentTokenPolicyVersion || admission.ProjectTokenRuntimeCodeHash != project.ProjectTokenRuntimeCodeHash || admission.PaymentTokenDecimals != 18 {
		return errors.New("project admission mismatch")
	}
	if project.ID != admission.ProjectRowID || project.ProjectID <= 0 || uint64(project.ProjectID) != facts.ProjectID || project.OnchainLocked || project.Status != constant.ProjectStatusSet || project.FundingMode != constant.FundingModeFairLaunch || project.ContractVersion != constant.ContractVersionFairLaunchV1 || project.ChainID != authority.ChainID || strings.ToLower(project.ContractAddress) != authority.ProxyAddress || strings.ToLower(project.TokenAddress) != facts.ProjectToken || strings.ToLower(project.PaymentTokenAddress) != facts.PaymentToken || strings.ToLower(project.OwnerAddress) != facts.ProjectOwner || strings.ToLower(project.FeeReceiver) != facts.PlatformFeeReceiver {
		return errors.New("prepared project identity mismatch")
	}
	if project.TokenDecimals != int(admission.ProjectTokenDecimals) || project.PaymentTokenDecimals != int(admission.PaymentTokenDecimals) || project.PaymentTokenPolicyRevision == nil || *project.PaymentTokenPolicyRevision != facts.PaymentTokenPolicyVersion || project.ProjectTokenPolicyDigest != facts.ProjectTokenPolicyDigest || project.SuccessFeeRate != facts.PlatformFeeBPS || project.SaleTokenAmountRaw != facts.SaleTokenAmountRaw || project.SoftCapRaw != facts.SoftCapRaw || project.MinContributionRaw != facts.MinContributionRaw || project.MaxContributionRaw != facts.MaxContributionRaw || project.MaxPerUserRaw != "0" || project.TotalRaisedRaw != "0" || project.TotalDepositedRaw != "0" || project.ParticipantCount != 0 || uint64(project.StartTime.UTC().Unix()) != facts.StartTime || uint64(project.EndTime.UTC().Unix()) != facts.EndTime {
		return errors.New("prepared project economics mismatch")
	}
	if facts.MaxContributionRaw == "0" || admission.MaxTotalContributionRaw != facts.MaxContributionRaw || admission.MaxWalletContributionRaw != facts.MaxContributionRaw || compareRawUint(facts.SoftCapRaw, admission.MaxTotalContributionRaw) > 0 || compareRawUint(admission.MaxTotalContributionRaw, payment.MaxActivationValueRaw) > 0 {
		return errors.New("project amount cap mismatch")
	}
	if strings.TrimSpace(project.NameEn) == "" || strings.TrimSpace(project.NameCn) == "" || strings.TrimSpace(project.Slug) == "" || strings.TrimSpace(project.Tagline) == "" || strings.TrimSpace(project.Description) == "" || strings.TrimSpace(project.Tags) == "" {
		return errors.New("project content row is incomplete")
	}
	var metadata fairLaunchPreparedMetadata
	if json.Unmarshal([]byte(project.OnchainJSON), &metadata) != nil {
		return errors.New("prepared project metadata invalid")
	}
	m := metadata.FairLaunch
	if m.SaleTokenAmountRaw != facts.SaleTokenAmountRaw || m.SoftCapRaw != facts.SoftCapRaw || m.MinContributionRaw != facts.MinContributionRaw || m.MaxContributionRaw != facts.MaxContributionRaw || strings.ToLower(m.ProjectProceedsReceiver) != facts.ProjectProceedsReceiver || normalizeDigest(m.ProjectProceedsReceiverAttestationDigest) != facts.ProjectProceedsReceiverAttestationDigest || m.ProjectProceedsReceiverAttestationVersion != facts.ProjectProceedsReceiverAttestationVersion || normalizeDigest(m.PlatformFeeReceiverAttestationDigest) != facts.PlatformFeeReceiverAttestationDigest || m.PlatformFeeReceiverAttestationVersion != facts.PlatformFeeReceiverAttestationVersion || normalizeDigest(m.ProjectTokenPolicyDigest) != facts.ProjectTokenPolicyDigest || m.ProjectTokenPolicyVersion != facts.ProjectTokenPolicyVersion || normalizeDigest(m.PaymentTokenPolicyDigest) != facts.PaymentTokenPolicyDigest || m.PaymentTokenPolicyVersion != facts.PaymentTokenPolicyVersion || normalizeDigest(m.ReleasePolicyDigest) != facts.ReleasePolicyDigest || m.ReleasePolicyVersion != facts.ReleasePolicyVersion {
		return errors.New("prepared project metadata mismatch")
	}
	return nil
}

func initialFairLaunchProjectState(authority fairLaunchV3Authority, facts fairLaunchProjectCreatedFacts, contract common.Address, identityID, placementID uint64, now time.Time) entity.FairLaunchProjectStateV3 {
	state := entity.FairLaunchProjectStateV3{RunRowID: authority.RunRowID, ChainID: authority.ChainID, SourceID: authority.SourceID, ContractAddress: strings.ToLower(contract.Hex()), ContractVersion: authority.ContractVersion, ProjectID: facts.ProjectID, ProjectTokenAddress: facts.ProjectToken, PaymentTokenAddress: facts.PaymentToken, Status: constant.ProjectStatusPending, TerminalReason: "none", SaleTokenAmountRaw: facts.SaleTokenAmountRaw, TotalRaisedRaw: "0", TotalTokensDepositedRaw: "0", TotalTokensClaimedRaw: "0", ParticipantCountRaw: "0", PlatformFeeReservedRaw: "0", ProjectProceedsPaidRaw: "0", PlatformFeePaidRaw: "0", ReleasePolicyDigest: stringPointer(facts.ReleasePolicyDigest), ReleasePolicyVersion: uint32Pointer(facts.ReleasePolicyVersion), StateGeneration: 1, LastEventIdentityID: uint64Pointer(identityID), LastPlacementID: uint64Pointer(placementID), LastSourceRevisionID: uint64Pointer(authority.SourceRevisionID), LastEffectManifestVersion: uint32Pointer(FairLaunchEffectManifestVersion), CreatedAt: now, UpdatedAt: now}
	_, state.StateDigest = fairLaunchV3Payload(fairLaunchProjectStatePayload(state))
	return state
}

func validateFairLaunchProjectCreatedReplayTx(tx *gorm.DB, authority fairLaunchV3Authority, project entity.Project, admission entity.FairLaunchMainnetProjectAdmissionV1, facts fairLaunchProjectCreatedFacts, identityID, placementID, observationID uint64, event *commonchain.FairLaunchTypedEvent, watermark *entity.FairLaunchMainnetWatermarkV1) (FairLaunchMainnetProjectBootstrapResult, error) {
	var binding entity.FairLaunchProjectBindingV3
	if tx.Where("run_row_id=? AND project_row_id=? AND chain_id=? AND source_binding_id=? AND source_id=? AND source_revision_id=? AND contract_address=? AND contract_version=? AND project_id=? AND project_token_address=? AND project_token_decimals=? AND payment_token_address=? AND payment_token_decimals=?", authority.RunRowID, project.ID, authority.ChainID, authority.SourceBindingID, authority.SourceID, authority.SourceRevisionID, strings.ToLower(project.ContractAddress), authority.ContractVersion, facts.ProjectID, facts.ProjectToken, admission.ProjectTokenDecimals, facts.PaymentToken, admission.PaymentTokenDecimals).Take(&binding).Error != nil {
		return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError("replay binding conflict")
	}
	var persisted entity.FairLaunchProjectStateV3
	if tx.Where("project_binding_id=? AND last_event_identity_id=? AND last_placement_id=? AND last_source_revision_id=? AND last_effect_manifest_version=?", binding.ID, identityID, placementID, authority.SourceRevisionID, FairLaunchEffectManifestVersion).Take(&persisted).Error != nil || !fairLaunchProjectStateDigestValid(persisted) || !project.OnchainLocked || project.Status != constant.ProjectStatusPending {
		return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError("replay state conflict")
	}
	if persisted.BootstrapReceiptID == nil {
		return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError("replay bootstrap receipt missing")
	}
	payloadDigest, err := commonchain.FairLaunchTypedEventDigest(event)
	if err != nil {
		return FairLaunchMainnetProjectBootstrapResult{}, err
	}
	watermarkID := uint64(0)
	if watermark != nil {
		watermarkDigest := fairLaunchV3DomainDigest("JC_FL_MAINNET_PROJECT_CREATED_WATERMARK_V1", map[string]any{"activationRowId": authority.RunRowID, "chainId": authority.ChainID, "sourceId": authority.SourceID, "sourceRevisionId": authority.SourceRevisionID, "projectId": facts.ProjectID, "eventIdentityId": identityID, "placementId": placementID, "observationId": observationID, "payloadDigest": payloadDigest, "bindingDigest": binding.BindingDigest, "stateDigest": persisted.StateDigest})
		if watermark.EventIdentityID == nil || *watermark.EventIdentityID != identityID || watermark.EvidenceDigest != watermarkDigest {
			return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError("replay watermark conflict")
		}
		watermarkID = watermark.ID
	}
	var receipt entity.FairLaunchMainnetProjectBootstrapReceiptV1
	if tx.Where("id=? AND activation_row_id=? AND chain_id=? AND project_binding_id=? AND project_id=? AND event_identity_id=? AND placement_id=? AND observation_id=? AND source_id=? AND source_revision_id=? AND effect_manifest_version=? AND payload_digest=? AND binding_digest=? AND initial_state_digest=?", *persisted.BootstrapReceiptID, authority.RunRowID, authority.ChainID, binding.ID, facts.ProjectID, identityID, placementID, observationID, authority.SourceID, authority.SourceRevisionID, FairLaunchEffectManifestVersion, payloadDigest, binding.BindingDigest, persisted.StateDigest).Take(&receipt).Error != nil {
		return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError("replay bootstrap receipt conflict")
	}
	expectedReceiptDigest := fairLaunchV3DomainDigest("JC_FL_MAINNET_PROJECT_BOOTSTRAP_RECEIPT_V1", map[string]any{"activationRowId": receipt.ActivationRowID, "chainId": receipt.ChainID, "projectBindingId": receipt.ProjectBindingID, "projectId": receipt.ProjectID, "eventIdentityId": receipt.EventIdentityID, "placementId": receipt.PlacementID, "observationId": receipt.ObservationID, "sourceId": receipt.SourceID, "sourceRevisionId": receipt.SourceRevisionID, "placementGeneration": receipt.PlacementGeneration, "effectManifestVersion": receipt.EffectManifestVersion, "payloadDigest": receipt.PayloadDigest, "bindingDigest": receipt.BindingDigest, "initialStateDigest": receipt.InitialStateDigest})
	if receipt.ReceiptDigest != expectedReceiptDigest {
		return FairLaunchMainnetProjectBootstrapResult{}, bootstrapError("replay bootstrap receipt digest conflict")
	}
	return FairLaunchMainnetProjectBootstrapResult{ProjectRowID: project.ID, ProjectBindingID: binding.ID, ProjectStateID: persisted.ID, EventIdentityID: identityID, BootstrapReceiptID: receipt.ID, WatermarkID: watermarkID, Replayed: true}, nil
}

func finalizeFairLaunchMainnetProjectRowTx(tx *gorm.DB, project entity.Project, identityID, placementID uint64, payloadDigest string, now time.Time) error {
	var metadata map[string]any
	if err := json.Unmarshal([]byte(project.OnchainJSON), &metadata); err != nil {
		return err
	}
	fairLaunch, ok := metadata["fairLaunch"].(map[string]any)
	if !ok {
		return bootstrapError("prepared project metadata missing")
	}
	fairLaunch["projectCreatedEventIdentityId"] = identityID
	fairLaunch["projectCreatedPlacementId"] = placementID
	fairLaunch["projectCreatedPayloadDigest"] = payloadDigest
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	result := tx.Model(&entity.Project{}).Where("id=? AND project_id=? AND status=? AND onchain_locked=0", project.ID, project.ProjectID, constant.ProjectStatusSet).Updates(map[string]any{"status": constant.ProjectStatusPending, "onchain_locked": true, "onchain_json": string(encoded), "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return bootstrapError("project row changed")
	}
	return nil
}

func validateFairLaunchMainnetTokenBindingsTx(tx *gorm.DB, authority fairLaunchV3Authority, admission entity.FairLaunchMainnetProjectAdmissionV1, payment entity.FairLaunchMainnetPaymentTokenV1, contractAddress string) error {
	for _, expected := range []struct {
		address  string
		decimals uint8
		codeHash string
	}{{payment.TokenAddress, payment.TokenDecimals, payment.RuntimeCodeHash}, {admission.ProjectTokenAddress, admission.ProjectTokenDecimals, admission.ProjectTokenRuntimeCodeHash}} {
		var row entity.FairLaunchSourceTokenBindingV3
		if tx.Where("run_row_id=? AND chain_id=? AND source_id=? AND contract_address=? AND token_address=? AND token_decimals=? AND runtime_code_hash=?", authority.RunRowID, authority.ChainID, authority.SourceID, contractAddress, expected.address, expected.decimals, expected.codeHash).Take(&row).Error != nil || row.MetadataVersion == 0 || row.AdmissionGeneration == 0 || !fairLaunchLocalDigestPattern.MatchString(row.MetadataDigest) {
			return bootstrapError("source token binding unavailable")
		}
	}
	return nil
}

func bootstrapError(reason string) error {
	return fmt.Errorf("%s: %s", FairLaunchMainnetProjectBootstrapUnavailable, reason)
}
func normalizeDigest(value string) string { return strings.TrimPrefix(strings.ToLower(value), "0x") }
func compareRawUint(left, right string) int {
	a, aok := new(big.Int).SetString(left, 10)
	b, bok := new(big.Int).SetString(right, 10)
	if !aok || !bok {
		return 1
	}
	return a.Cmp(b)
}
func uint64Pointer(value uint64) *uint64 { return &value }
func uint32Pointer(value uint32) *uint32 { return &value }
func stringPointer(value string) *string { return &value }

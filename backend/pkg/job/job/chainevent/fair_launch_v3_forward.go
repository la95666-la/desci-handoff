package chainevent

import (
	"crypto/sha256"
	"encoding/hex"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/money"
)

var fairLaunchV3ForwardEvents = map[string]bool{
	"FairLaunchTokensDeposited":              true,
	"FairLaunchActivated":                    true,
	"FairLaunchContribution":                 true,
	"FairLaunchEnded":                        true,
	"FairLaunchSuccessfulReleased":           true,
	"FairLaunchCancelled":                    true,
	"FairLaunchRefunded":                     true,
	"FairLaunchClaimed":                      true,
	"FairLaunchProjectProceedsWithdrawn":     true,
	"FairLaunchPlatformFeeWithdrawn":         true,
	"FairLaunchFailedProjectTokensWithdrawn": true,
}

// Compatibility alias for the still LOCAL-only decoder/transport. Remove it
// only when the mainnet scanner owns its independent typed-event admission.
var fairLaunchLocalForwardEvents = fairLaunchV3ForwardEvents

type fairLaunchV3Mutation struct {
	Kind          string
	AggregateKind string
	AggregateKey  string
	BeforeJCS     string
	BeforeDigest  string
	AfterJCS      string
	AfterDigest   string
}

type fairLaunchV3LedgerDraft struct {
	EntryType           string
	AssetRole           string
	FlowDirection       string
	WalletAddress       string
	CounterpartyAddress string
	TokenAddress        string
	TokenDecimals       uint8
	AmountRaw           string
	AmountDisplay       string
	RowDigest           string
}

type fairLaunchV3Plan struct {
	Effect         FairLaunchMaterializationResult
	PayloadDigest  string
	EffectDigest   string
	LedgerDigest   string
	MutationDigest string
	Project        entity.FairLaunchProjectStateV3
	Position       *entity.FairLaunchPositionV3
	Mutations      []fairLaunchV3Mutation
	Ledger         []fairLaunchV3LedgerDraft
}

// fairLaunchV3Authority contains only the already-verified provenance and
// cursor-fencing values needed by the financial materialization core. Local
// and mainnet callers must independently validate their archive/finality and
// live-chain evidence before constructing this value.
type fairLaunchV3Authority struct {
	RunRowID          uint64
	ChainID           uint64
	SourceBindingID   uint64
	SourceID          uint64
	SourceRevisionID  uint64
	LeaseToken        string
	FenceEpoch        uint64
	ContractVersion   string
	RejectedReason    string
	UnavailableReason string
}

type fairLaunchV3ArchiveEvidence struct {
	IdentityID          uint64
	PlacementID         uint64
	ObservationID       uint64
	PlacementGeneration uint32
	SourceRevisionID    uint64
	BlockNumber         uint64
	BlockHash           string
	TransactionIndex    uint32
	TxLogOrdinal        uint32
	RPCLogIndex         uint32
}

type fairLaunchLocalFinancialStageError struct {
	Stage string
	Err   error
}

func (staged *fairLaunchLocalFinancialStageError) Error() string { return staged.Stage }
func (staged *fairLaunchLocalFinancialStageError) Unwrap() error { return staged.Err }
func fairLaunchLocalFinancialStage(stage string, err error) error {
	if err == nil {
		return nil
	}
	return &fairLaunchLocalFinancialStageError{Stage: stage, Err: err}
}

func applyFairLaunchV3ForwardTx(capability FairLaunchLocalE2ECapability, tx *gorm.DB, in FairLaunchCEWrite, now time.Time) (FairLaunchCEWriteResult, error) {
	if in.Event == nil || in.LocalArchive == nil || !fairLaunchV3ForwardEvents[in.Event.Name] || in.Context.ChainID != fairLaunchLocalExpectedChainID || in.SourceID != capability.seal.admission.SourceID || in.Context.SourceRevisionID != capability.seal.admission.SourceRevisionID || in.Context.EffectVersion != FairLaunchEffectManifestVersion || !in.Context.Canonical || !in.Context.Finalized || in.LeaseToken != capability.seal.lease || in.FenceEpoch != capability.seal.fence || now.IsZero() {
		return FairLaunchCEWriteResult{}, errors.New(FairLaunchLocalE2EForwardUnavailable)
	}
	if !in.LocalArchive.valid(capability) {
		return FairLaunchCEWriteResult{}, fmt.Errorf("%s: archive handoff invalid", FairLaunchLocalE2EAdmissionRejected)
	}
	archive := *in.LocalArchive.seal
	if archive.SourceRevisionID != in.Context.SourceRevisionID || archive.BlockNumber != in.Event.Log.BlockNumber || archive.BlockHash != strings.ToLower(in.Event.Log.BlockHash.Hex()) || archive.TransactionIndex != in.TransactionIndex || archive.TxLogOrdinal != in.Context.TxLogOrdinal || archive.RPCLogIndex != uint32(in.Event.Log.Index) {
		return FairLaunchCEWriteResult{}, fmt.Errorf("%s: archive handoff mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	if err := verifyFairLaunchLocalLiveBlock(capability.seal.rpc, tx.Statement.Context, archive.BlockNumber, archive.BlockHash); err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	authority := fairLaunchV3Authority{RunRowID: capability.seal.admission.RunRowID, ChainID: fairLaunchLocalExpectedChainID, SourceBindingID: capability.seal.admission.SourceBindingID, SourceID: capability.seal.admission.SourceID, SourceRevisionID: capability.seal.admission.SourceRevisionID, LeaseToken: capability.seal.lease, FenceEpoch: capability.seal.fence, ContractVersion: fairLaunchLocalContractVersion, RejectedReason: FairLaunchLocalE2EAdmissionRejected, UnavailableReason: FairLaunchLocalE2EForwardUnavailable}
	evidence := fairLaunchV3ArchiveEvidence{IdentityID: archive.IdentityID, PlacementID: archive.PlacementID, ObservationID: archive.ObservationID, PlacementGeneration: archive.PlacementGeneration, SourceRevisionID: archive.SourceRevisionID, BlockNumber: archive.BlockNumber, BlockHash: archive.BlockHash, TransactionIndex: archive.TransactionIndex, TxLogOrdinal: archive.TxLogOrdinal, RPCLogIndex: archive.RPCLogIndex}
	return applyFairLaunchV3ForwardCoreTx(authority, tx, in, evidence, now)
}

func applyFairLaunchV3ForwardCoreTx(authority fairLaunchV3Authority, tx *gorm.DB, in FairLaunchCEWrite, archive fairLaunchV3ArchiveEvidence, now time.Time) (FairLaunchCEWriteResult, error) {
	if tx == nil || in.Event == nil || !fairLaunchV3ForwardEvents[in.Event.Name] || authority.RunRowID == 0 || authority.ChainID == 0 || authority.SourceBindingID == 0 || authority.SourceID == 0 || authority.SourceRevisionID == 0 || authority.LeaseToken == "" || authority.FenceEpoch == 0 || authority.ContractVersion == "" || authority.RejectedReason == "" || authority.UnavailableReason == "" || in.Context.ChainID != authority.ChainID || in.SourceID != authority.SourceID || in.Context.SourceRevisionID != authority.SourceRevisionID || in.Context.EffectVersion != FairLaunchEffectManifestVersion || !in.Context.Canonical || !in.Context.Finalized || in.LeaseToken != authority.LeaseToken || in.FenceEpoch != authority.FenceEpoch || archive.IdentityID == 0 || archive.PlacementID == 0 || archive.ObservationID == 0 || archive.PlacementGeneration == 0 || archive.SourceRevisionID != authority.SourceRevisionID || now.IsZero() {
		return FairLaunchCEWriteResult{}, errors.New(authority.UnavailableReason)
	}
	identityID := archive.IdentityID
	var placement entity.ChainEventPlacementV2
	if err := tx.Where("id=? AND event_identity_id=? AND generation=? AND block_number=? AND block_hash=? AND transaction_index=? AND rpc_log_index=? AND source_revision_id=? AND canonical_slot=1 AND finalized_at IS NOT NULL AND noncanonical_at IS NULL", archive.PlacementID, identityID, archive.PlacementGeneration, archive.BlockNumber, archive.BlockHash, archive.TransactionIndex, archive.RPCLogIndex, archive.SourceRevisionID).Take(&placement).Error; err != nil {
		return FairLaunchCEWriteResult{}, fmt.Errorf("%s: archived placement unavailable", authority.RejectedReason)
	}
	var observation entity.ChainEventPlacementObservationV2
	if err := tx.Where("id=? AND placement_id=? AND event_identity_id=? AND provider_identity=? AND emitting_address=? AND topics_data_digest=? AND raw_log_digest=?", archive.ObservationID, placement.ID, identityID, in.ProviderIdentity, strings.ToLower(in.Event.Log.Address.Hex()), stripCE(in.TopicsDataDigest), stripCE(in.RawLogDigest)).Take(&observation).Error; err != nil {
		return FairLaunchCEWriteResult{}, fmt.Errorf("%s: archived observation unavailable", authority.RejectedReason)
	}
	var cursor entity.SourceMaterializationCursor
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("source_id=? AND source_revision_id=? AND lease_token=? AND fence_epoch=? AND status='active' AND last_error=''", authority.SourceID, authority.SourceRevisionID, authority.LeaseToken, authority.FenceEpoch).Take(&cursor).Error; err != nil {
		return FairLaunchCEWriteResult{}, fairLaunchLocalFinancialStage("cursor_lock", err)
	}
	if archive.BlockNumber < cursor.MaterializedThroughBlock {
		return FairLaunchCEWriteResult{}, fmt.Errorf("%s: event precedes materialization cursor", authority.RejectedReason)
	}
	if archive.BlockNumber == cursor.MaterializedThroughBlock && compareFairLaunchPosition(archive.BlockNumber, archive.TransactionIndex, archive.TxLogOrdinal, cursor.MaterializedThroughBlock, cursor.MaterializedThroughTxIndex, cursor.MaterializedThroughTxLogOrdinal) <= 0 {
		return applyFairLaunchV3ReplayTx(authority, tx, in, identityID, placement, observation, now)
	}
	var projectBinding entity.FairLaunchProjectBindingV3
	if in.Event.ProjectID == nil || !in.Event.ProjectID.IsUint64() || in.Event.ProjectID.Sign() <= 0 {
		return FairLaunchCEWriteResult{}, fmt.Errorf("%s: project ID missing", authority.RejectedReason)
	}
	projectID := in.Event.ProjectID.Uint64()
	if err := tx.Where("run_row_id=? AND chain_id=? AND source_binding_id=? AND source_id=? AND source_revision_id=? AND contract_address=? AND contract_version=? AND project_id=?", authority.RunRowID, authority.ChainID, authority.SourceBindingID, authority.SourceID, authority.SourceRevisionID, strings.ToLower(in.Event.Log.Address.Hex()), authority.ContractVersion, projectID).Take(&projectBinding).Error; err != nil {
		return FairLaunchCEWriteResult{}, fairLaunchLocalFinancialStage("project_binding_load", err)
	}
	var project entity.FairLaunchProjectStateV3
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_binding_id=? AND run_row_id=? AND chain_id=? AND source_id=? AND project_id=? AND tombstoned=0", projectBinding.ID, authority.RunRowID, authority.ChainID, authority.SourceID, projectID).Take(&project).Error; err != nil {
		return FairLaunchCEWriteResult{}, fairLaunchLocalFinancialStage("project_state_lock", err)
	}
	if !fairLaunchProjectStateDigestValid(project) {
		return FairLaunchCEWriteResult{}, fairLaunchLocalFinancialStage("project_state_digest", errors.New(authority.RejectedReason))
	}
	var previousLineage *uint64
	if project.SourceApplicationLineageID != nil {
		previous := *project.SourceApplicationLineageID
		previousLineage = &previous
	}
	ctx := in.Context
	ctx.PlacementGeneration = placement.Generation
	effect, err := MaterializeFairLaunchEvent(in.Event, ctx)
	if err != nil || effect.Disposition != FairLaunchEffectExact || effect.GovernanceChanged || effect.PolicyChanged {
		return FairLaunchCEWriteResult{}, errors.New(authority.UnavailableReason)
	}
	payloadDigest, err := commonchain.FairLaunchTypedEventDigest(in.Event)
	if err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	plan, err := buildFairLaunchV3ForwardPlan(in.Event, projectBinding, project, effect, payloadDigest)
	if err != nil {
		return FairLaunchCEWriteResult{}, fairLaunchLocalFinancialStage("forward_plan", err)
	}
	if err := prepareFairLaunchV3PositionPlan(tx, &plan, in.Event, projectBinding, project); err != nil {
		return FairLaunchCEWriteResult{}, fairLaunchLocalFinancialStage("position_plan", err)
	}
	var attemptNo uint32
	if err := tx.Model(&entity.ChainEventApplicationAttemptV2{}).Where("event_identity_id=? AND placement_id=? AND effect_version=?", identityID, placement.ID, FairLaunchEffectManifestVersion).Select("COALESCE(MAX(attempt_no),0)").Scan(&attemptNo).Error; err != nil {
		return FairLaunchCEWriteResult{}, fairLaunchLocalFinancialStage("attempt_sequence", err)
	}
	attemptNo++
	application := entity.ChainEventApplicationAttemptV2{EventIdentityID: identityID, PlacementID: placement.ID, EffectVersion: FairLaunchEffectManifestVersion, ExpectedEffect: 1, AttemptNo: attemptNo, Disposition: "exact", EffectDigest: plan.EffectDigest, AppliedAt: localTimePointer(now)}
	if err := tx.Create(&application).Error; err != nil {
		return FairLaunchCEWriteResult{}, fairLaunchLocalFinancialStage("attempt_insert", err)
	}
	lineage := entity.FairLaunchApplicationLineageV3{CEApplicationAttemptID: application.ID, RunRowID: authority.RunRowID, ChainID: authority.ChainID, SourceBindingID: authority.SourceBindingID, SourceID: authority.SourceID, EventIdentityID: identityID, PlacementID: placement.ID, SourceRevisionID: authority.SourceRevisionID, EffectManifestVersion: FairLaunchEffectManifestVersion, PlacementGeneration: placement.Generation, Disposition: "forward", ExpectedEffect: 1, PayloadDigest: plan.PayloadDigest, EffectDigest: plan.EffectDigest, LedgerDigest: plan.LedgerDigest, MutationDigest: plan.MutationDigest, State: "applied", AppliedAt: localTimePointer(now), CreatedAt: now}
	if err := tx.Create(&lineage).Error; err != nil {
		return FairLaunchCEWriteResult{}, fairLaunchLocalFinancialStage("lineage_insert", err)
	}
	if previousLineage != nil {
		for _, mutation := range plan.Mutations {
			dependency := entity.FairLaunchEffectDependencyV3{PredecessorLineageID: *previousLineage, DependentLineageID: lineage.ID, RunRowID: lineage.RunRowID, ChainID: lineage.ChainID, SourceBindingID: lineage.SourceBindingID, SourceID: lineage.SourceID, SourceRevisionID: lineage.SourceRevisionID, EffectManifestVersion: lineage.EffectManifestVersion, AggregateKind: mutation.AggregateKind, AggregateKey: mutation.AggregateKey, DependencyDigest: fairLaunchV3DomainDigest("JC_FL_EFFECT_DEPENDENCY_V1", map[string]any{"predecessorLineageId": *previousLineage, "dependentLineageId": lineage.ID, "aggregateKind": mutation.AggregateKind, "aggregateKey": mutation.AggregateKey}), CreatedAt: now}
			if err := tx.Create(&dependency).Error; err != nil {
				return FairLaunchCEWriteResult{}, fairLaunchLocalFinancialStage("dependency_insert", err)
			}
		}
	}
	active := uint8(1)
	guard := entity.FairLaunchEffectGuardV3{RunRowID: lineage.RunRowID, ChainID: lineage.ChainID, SourceID: lineage.SourceID, EventIdentityID: identityID, EffectManifestVersion: FairLaunchEffectManifestVersion, PlacementGeneration: placement.Generation, SourceApplicationLineageID: lineage.ID, State: "applied", ActiveSlot: &active, PayloadDigest: plan.PayloadDigest, EffectDigest: plan.EffectDigest, CreatedAt: now}
	if err := tx.Create(&guard).Error; err != nil {
		return FairLaunchCEWriteResult{}, fairLaunchLocalFinancialStage("effect_guard_insert", err)
	}
	for index, mutation := range plan.Mutations {
		journal := entity.FairLaunchEffectJournalV3{RunRowID: lineage.RunRowID, ChainID: lineage.ChainID, SourceID: lineage.SourceID, EventIdentityID: identityID, PlacementID: placement.ID, SourceRevisionID: lineage.SourceRevisionID, SourceApplicationLineageID: lineage.ID, EffectManifestVersion: FairLaunchEffectManifestVersion, EffectKind: mutation.Kind, AggregateKind: mutation.AggregateKind, AggregateKey: mutation.AggregateKey, MutationOrdinal: uint32(index + 1), BeforePayloadJCS: mutation.BeforeJCS, BeforeDigest: mutation.BeforeDigest, AfterPayloadJCS: mutation.AfterJCS, AfterDigest: mutation.AfterDigest, CreatedAt: now}
		if err := tx.Create(&journal).Error; err != nil {
			return FairLaunchCEWriteResult{}, fairLaunchLocalFinancialStage("journal_insert", err)
		}
	}
	for index, draft := range plan.Ledger {
		entry := entity.FairLaunchLedgerEntryV3{RunRowID: lineage.RunRowID, ChainID: lineage.ChainID, SourceID: lineage.SourceID, ProjectBindingID: projectBinding.ID, ProjectID: projectBinding.ProjectID, ContractAddress: projectBinding.ContractAddress, ContractVersion: projectBinding.ContractVersion, WalletAddress: draft.WalletAddress, CounterpartyAddress: draft.CounterpartyAddress, EntryType: draft.EntryType, AssetRole: draft.AssetRole, FlowDirection: draft.FlowDirection, TokenAddress: draft.TokenAddress, TokenDecimals: draft.TokenDecimals, AmountRaw: draft.AmountRaw, AmountDisplay: draft.AmountDisplay, SourceEventIdentityID: identityID, SourcePlacementID: placement.ID, SourceApplicationLineageID: lineage.ID, SourceRevisionID: lineage.SourceRevisionID, EntryOrdinal: uint32(index + 1), EffectManifestVersion: FairLaunchEffectManifestVersion, RowDigest: draft.RowDigest, CreatedAt: now}
		if err := tx.Create(&entry).Error; err != nil {
			return FairLaunchCEWriteResult{}, fairLaunchLocalFinancialStage("ledger_insert", err)
		}
	}
	if err := persistFairLaunchV3Projection(tx, &plan, project, projectBinding, lineage, application, identityID, placement.ID, now); err != nil {
		return FairLaunchCEWriteResult{}, fairLaunchLocalFinancialStage("projection_persist", err)
	}
	projectCount, positionCount := uint64(1), uint64(0)
	if plan.Position != nil {
		positionCount = 1
	}
	attestationDigest := fairLaunchAttestationDigest(identityID, placement.ID, lineage.ID, plan.PayloadDigest, plan.EffectDigest, plan.LedgerDigest, plan.MutationDigest, uint64(len(plan.Ledger)), uint64(len(plan.Mutations)), projectCount, positionCount)
	attestation := entity.FairLaunchEffectAttestationV3{RunRowID: lineage.RunRowID, ChainID: lineage.ChainID, SourceBindingID: lineage.SourceBindingID, SourceID: lineage.SourceID, EventIdentityID: identityID, PlacementID: placement.ID, SourceRevisionID: lineage.SourceRevisionID, SourceApplicationLineageID: lineage.ID, EffectManifestVersion: FairLaunchEffectManifestVersion, PayloadDigest: plan.PayloadDigest, EffectDigest: plan.EffectDigest, LedgerDigest: plan.LedgerDigest, MutationDigest: plan.MutationDigest, LedgerRowCount: uint64(len(plan.Ledger)), MutationRowCount: uint64(len(plan.Mutations)), ProjectMutationCount: projectCount, PositionMutationCount: positionCount, GovernanceMutationCount: 0, PolicyMutationCount: 0, AttestationDigest: attestationDigest, CreatedAt: now}
	if err := tx.Create(&attestation).Error; err != nil {
		return FairLaunchCEWriteResult{}, fairLaunchLocalFinancialStage("attestation_insert", err)
	}
	cursor.SourceRevisionID = authority.SourceRevisionID
	cursor.MaterializedThroughBlock = archive.BlockNumber
	cursor.MaterializedThroughBlockHash = archive.BlockHash
	cursor.MaterializedThroughTxIndex = archive.TransactionIndex
	cursor.MaterializedThroughTxLogOrdinal = archive.TxLogOrdinal
	cursor.LastEventIdentityID = &identityID
	cursor.CursorGeneration++
	cursor.UpdatedAt = now
	if err := tx.Save(&cursor).Error; err != nil {
		return FairLaunchCEWriteResult{}, fairLaunchLocalFinancialStage("cursor_advance", err)
	}
	return FairLaunchCEWriteResult{IdentityID: identityID, PlacementID: placement.ID, ObservationID: observation.ID, ApplicationID: application.ID, Effect: effect}, nil
}

func applyFairLaunchV3ReplayTx(authority fairLaunchV3Authority, tx *gorm.DB, in FairLaunchCEWrite, identityID uint64, placement entity.ChainEventPlacementV2, observation entity.ChainEventPlacementObservationV2, now time.Time) (FairLaunchCEWriteResult, error) {
	var guard entity.FairLaunchEffectGuardV3
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_identity_id=? AND effect_manifest_version=? AND active_slot=1 AND state='applied'", identityID, FairLaunchEffectManifestVersion).Take(&guard).Error; err != nil {
		return FairLaunchCEWriteResult{}, fmt.Errorf("%s: replay active guard unavailable", authority.RejectedReason)
	}
	var lineage entity.FairLaunchApplicationLineageV3
	if err := tx.Where("id=? AND run_row_id=? AND source_id=? AND event_identity_id=? AND placement_id=? AND source_revision_id=? AND effect_manifest_version=? AND placement_generation=? AND disposition='forward' AND state='applied'", guard.SourceApplicationLineageID, authority.RunRowID, authority.SourceID, identityID, placement.ID, authority.SourceRevisionID, FairLaunchEffectManifestVersion, placement.Generation).Take(&lineage).Error; err != nil {
		return FairLaunchCEWriteResult{}, fmt.Errorf("%s: replay lineage unavailable", authority.RejectedReason)
	}
	var project entity.FairLaunchProjectStateV3
	var binding entity.FairLaunchProjectBindingV3
	if in.Event.ProjectID == nil || tx.Where("run_row_id=? AND source_id=? AND project_id=?", authority.RunRowID, authority.SourceID, in.Event.ProjectID.Uint64()).Take(&binding).Error != nil || tx.Where("project_binding_id=?", binding.ID).Take(&project).Error != nil {
		return FairLaunchCEWriteResult{}, fmt.Errorf("%s: replay projection unavailable", authority.RejectedReason)
	}
	ctx := in.Context
	ctx.PlacementGeneration = placement.Generation
	effect, err := MaterializeFairLaunchEvent(in.Event, ctx)
	if err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	payloadDigest, err := commonchain.FairLaunchTypedEventDigest(in.Event)
	if err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	if payloadDigest != lineage.PayloadDigest || effect.EffectDigest != lineage.EffectDigest || guard.PayloadDigest != lineage.PayloadDigest || guard.EffectDigest != lineage.EffectDigest {
		return FairLaunchCEWriteResult{}, fmt.Errorf("%s: replay digest mismatch", authority.RejectedReason)
	}
	var attestation entity.FairLaunchEffectAttestationV3
	if err := tx.Where("source_application_lineage_id=? AND payload_digest=? AND effect_digest=? AND ledger_digest=? AND mutation_digest=?", lineage.ID, lineage.PayloadDigest, lineage.EffectDigest, lineage.LedgerDigest, lineage.MutationDigest).Take(&attestation).Error; err != nil {
		return FairLaunchCEWriteResult{}, fmt.Errorf("%s: replay attestation mismatch", authority.RejectedReason)
	}
	if !fairLaunchAttestationValid(attestation, lineage) {
		return FairLaunchCEWriteResult{}, fmt.Errorf("%s: replay attestation digest mismatch", authority.RejectedReason)
	}
	var ledgerCount, mutationCount int64
	if tx.Model(&entity.FairLaunchLedgerEntryV3{}).Where("source_application_lineage_id=?", lineage.ID).Count(&ledgerCount).Error != nil || uint64(ledgerCount) != attestation.LedgerRowCount || tx.Model(&entity.FairLaunchEffectJournalV3{}).Where("source_application_lineage_id=?", lineage.ID).Count(&mutationCount).Error != nil || uint64(mutationCount) != attestation.MutationRowCount || project.SourceApplicationLineageID == nil || *project.SourceApplicationLineageID != lineage.ID || !fairLaunchProjectStateDigestValid(project) {
		return FairLaunchCEWriteResult{}, fmt.Errorf("%s: replay row/projection mismatch", authority.RejectedReason)
	}
	if attestation.ProjectMutationCount != 1 || attestation.PositionMutationCount > 1 || attestation.GovernanceMutationCount != 0 || attestation.PolicyMutationCount != 0 || attestation.MutationRowCount != attestation.ProjectMutationCount+attestation.PositionMutationCount {
		return FairLaunchCEWriteResult{}, fmt.Errorf("%s: replay mutation attestation mismatch", authority.RejectedReason)
	}
	if attestation.PositionMutationCount == 1 {
		var position entity.FairLaunchPositionV3
		if tx.Where("source_application_lineage_id=? AND last_event_identity_id=? AND last_placement_id=?", lineage.ID, identityID, placement.ID).Take(&position).Error != nil || !fairLaunchPositionStateDigestValid(position) {
			return FairLaunchCEWriteResult{}, fmt.Errorf("%s: replay position projection mismatch", authority.RejectedReason)
		}
	}
	var attemptNo uint32
	if err := tx.Model(&entity.ChainEventApplicationAttemptV2{}).Where("event_identity_id=? AND placement_id=? AND effect_version=?", identityID, placement.ID, FairLaunchEffectManifestVersion).Select("COALESCE(MAX(attempt_no),0)").Scan(&attemptNo).Error; err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	attemptNo++
	replay := entity.ChainEventApplicationAttemptV2{EventIdentityID: identityID, PlacementID: placement.ID, EffectVersion: FairLaunchEffectManifestVersion, ExpectedEffect: 0, AttemptNo: attemptNo, PredecessorApplicationID: &lineage.CEApplicationAttemptID, Disposition: "no_op", EffectDigest: lineage.EffectDigest, AppliedAt: &now}
	if err := tx.Create(&replay).Error; err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	replayLineage := entity.FairLaunchApplicationLineageV3{CEApplicationAttemptID: replay.ID, RunRowID: lineage.RunRowID, ChainID: lineage.ChainID, SourceBindingID: lineage.SourceBindingID, SourceID: lineage.SourceID, EventIdentityID: lineage.EventIdentityID, PlacementID: lineage.PlacementID, SourceRevisionID: lineage.SourceRevisionID, EffectManifestVersion: lineage.EffectManifestVersion, PlacementGeneration: lineage.PlacementGeneration, PredecessorLineageID: &lineage.ID, Disposition: "replay_no_op", ExpectedEffect: 0, PayloadDigest: lineage.PayloadDigest, EffectDigest: lineage.EffectDigest, LedgerDigest: lineage.LedgerDigest, MutationDigest: lineage.MutationDigest, State: "applied", AppliedAt: &now, CreatedAt: now}
	if err := tx.Create(&replayLineage).Error; err != nil {
		return FairLaunchCEWriteResult{}, err
	}
	effect.Disposition = FairLaunchEffectNoOp
	return FairLaunchCEWriteResult{IdentityID: identityID, PlacementID: placement.ID, ObservationID: observation.ID, ApplicationID: replay.ID, Effect: effect}, nil
}

func buildFairLaunchV3ForwardPlan(event *commonchain.FairLaunchTypedEvent, binding entity.FairLaunchProjectBindingV3, before entity.FairLaunchProjectStateV3, effect FairLaunchMaterializationResult, payloadDigest string) (fairLaunchV3Plan, error) {
	after := before
	positionMutation := event.Name == "FairLaunchContribution" || event.Name == "FairLaunchRefunded" || event.Name == "FairLaunchClaimed"
	var position *entity.FairLaunchPositionV3
	var amountRaw, tokenAddress, walletAddress, counterparty, entryType, assetRole, flow string
	var tokenDecimals uint8
	contract := strings.ToLower(event.Log.Address.Hex())
	switch event.Name {
	case "FairLaunchTokensDeposited":
		if before.Status != "Pending" && before.Status != "Active" {
			return fairLaunchV3Plan{}, errors.New("Fair Launch deposit state invalid")
		}
		amountRaw, tokenAddress, walletAddress, counterparty, entryType, assetRole, flow, tokenDecimals = rawArg(event, "arg3"), addressArg(event, "arg1"), addressArg(event, "arg2"), contract, "project_token_deposited", "project", "inflow", binding.ProjectTokenDecimals
		if rawArg(event, "arg4") != addRawCE(before.TotalTokensDepositedRaw, amountRaw) {
			return fairLaunchV3Plan{}, errors.New("Fair Launch deposit cumulative amount mismatch")
		}
		if compareRawUint(rawArg(event, "arg4"), before.SaleTokenAmountRaw) > 0 {
			return fairLaunchV3Plan{}, errors.New("Fair Launch deposit exceeds sale amount")
		}
		after.TotalTokensDepositedRaw = rawArg(event, "arg4")
	case "FairLaunchActivated":
		if before.Status != "Pending" || before.TotalTokensDepositedRaw != before.SaleTokenAmountRaw {
			return fairLaunchV3Plan{}, errors.New("Fair Launch activate state invalid")
		}
		after.Status, after.TerminalReason = "Active", "none"
	case "FairLaunchContribution":
		if before.Status != "Active" {
			return fairLaunchV3Plan{}, errors.New("Fair Launch contribution state invalid")
		}
		amountRaw, tokenAddress, walletAddress, counterparty, entryType, assetRole, flow, tokenDecimals = rawArg(event, "arg3"), addressArg(event, "arg2"), addressArg(event, "arg1"), contract, "payment_contributed", "payment", "inflow", binding.PaymentTokenDecimals
		if rawArg(event, "arg5") != addRawCE(before.TotalRaisedRaw, amountRaw) {
			return fairLaunchV3Plan{}, errors.New("Fair Launch contribution total raised mismatch")
		}
		after.TotalRaisedRaw = rawArg(event, "arg5")
	case "FairLaunchEnded":
		if before.Status != "Active" {
			return fairLaunchV3Plan{}, errors.New("Fair Launch end state invalid")
		}
		status, reason, err := fairLaunchTerminalState(rawArg(event, "arg1"), rawArg(event, "arg2"))
		if err != nil {
			return fairLaunchV3Plan{}, err
		}
		tokenAddress = addressArg(event, "arg3")
		if rawArg(event, "arg4") != before.TotalRaisedRaw || rawUint64(event, "arg7") != event.Log.BlockNumber || rawUint64(event, "arg8")+1 != rawUint64(event, "arg7") {
			return fairLaunchV3Plan{}, errors.New("Fair Launch end settlement mismatch")
		}
		if status == "SuccessfulPendingRelease" && (rawArg(event, "arg5") != before.TotalRaisedRaw || rawArg(event, "arg6") != before.SaleTokenAmountRaw) || status == "Failed" && (rawArg(event, "arg5") != "0" || rawArg(event, "arg6") != "0") {
			return fairLaunchV3Plan{}, errors.New("Fair Launch end price mismatch")
		}
		after.Status, after.TerminalReason = status, reason
		after.FinalTotalRaisedRaw = rawStringPointer(rawArg(event, "arg4"))
		after.FinalPriceNumeratorRaw = rawStringPointer(rawArg(event, "arg5"))
		if rawArg(event, "arg6") != "0" {
			after.FinalPriceDenominatorRaw = rawStringPointer(rawArg(event, "arg6"))
		} else {
			after.FinalPriceDenominatorRaw = nil
		}
		blockNumber := rawUint64(event, "arg7")
		anchorNumber := rawUint64(event, "arg8")
		after.SettlementBlockNumber, after.SettlementAnchorBlockNumber = &blockNumber, &anchorNumber
		anchorHash := hashArg(event, "arg9")
		after.SettlementBlockHash, after.SettlementAnchorBlockHash = rawStringPointer(strings.ToLower(event.Log.BlockHash.Hex())), &anchorHash
	case "FairLaunchSuccessfulReleased":
		if before.Status != "SuccessfulPendingRelease" {
			return fairLaunchV3Plan{}, errors.New("Fair Launch release state invalid")
		}
		if before.SettlementBlockNumber == nil || before.SettlementAnchorBlockNumber == nil || before.SettlementAnchorBlockHash == nil || rawUint64(event, "arg1") != *before.SettlementBlockNumber || rawUint64(event, "arg3") != *before.SettlementAnchorBlockNumber || hashArg(event, "arg4") != *before.SettlementAnchorBlockHash {
			return fairLaunchV3Plan{}, errors.New("Fair Launch release settlement anchor mismatch")
		}
		if before.ReleasePolicyDigest == nil || before.ReleasePolicyVersion == nil || strings.TrimPrefix(hashArg(event, "arg8"), "0x") != *before.ReleasePolicyDigest || uint32(rawUint64(event, "arg9")) != *before.ReleasePolicyVersion {
			return fairLaunchV3Plan{}, errors.New("Fair Launch release policy mismatch")
		}
		after.Status, after.TerminalReason = "Successful", "soft_cap_met"
		txHash := hashArg(event, "arg5")
		ordinal := uint32(rawUint64(event, "arg6"))
		manifest := strings.TrimPrefix(hashArg(event, "arg7"), "0x")
		policy := strings.TrimPrefix(hashArg(event, "arg8"), "0x")
		version := uint32(rawUint64(event, "arg9"))
		after.SettlementTxHash, after.SettlementTxLogOrdinal, after.ReleaseManifestDigest, after.ReleasePolicyDigest, after.ReleasePolicyVersion = &txHash, &ordinal, &manifest, &policy, &version
	case "FairLaunchCancelled":
		if before.Status != "Pending" && before.Status != "Active" {
			return fairLaunchV3Plan{}, errors.New("Fair Launch cancel state invalid")
		}
		previousStatus := map[string]string{"Pending": "0", "Active": "1", "Cancelled": "5"}[before.Status]
		if rawArg(event, "arg1") != previousStatus || addressArg(event, "arg4") == "" {
			return fairLaunchV3Plan{}, errors.New("Fair Launch cancel evidence mismatch")
		}
		after.Status, after.TerminalReason = "Cancelled", "cancelled"
	case "FairLaunchRefunded":
		if before.Status != "Failed" && before.Status != "Cancelled" {
			return fairLaunchV3Plan{}, errors.New("Fair Launch refund state invalid")
		}
		amountRaw, tokenAddress, walletAddress, counterparty, entryType, assetRole, flow, tokenDecimals = rawArg(event, "arg3"), addressArg(event, "arg2"), addressArg(event, "arg1"), contract, "payment_refunded", "payment", "outflow", binding.PaymentTokenDecimals
	case "FairLaunchClaimed":
		if before.Status != "Successful" {
			return fairLaunchV3Plan{}, errors.New("Fair Launch claim state invalid")
		}
		amountRaw, tokenAddress, walletAddress, counterparty, entryType, assetRole, flow, tokenDecimals = rawArg(event, "arg3"), addressArg(event, "arg2"), addressArg(event, "arg1"), contract, "project_token_claimed", "project", "outflow", binding.ProjectTokenDecimals
		after.TotalTokensClaimedRaw = addRawCE(before.TotalTokensClaimedRaw, amountRaw)
		if compareRawUint(after.TotalTokensClaimedRaw, before.SaleTokenAmountRaw) > 0 {
			return fairLaunchV3Plan{}, errors.New("Fair Launch claim exceeds sale amount")
		}
	case "FairLaunchProjectProceedsWithdrawn":
		if before.Status != "Successful" || before.ProjectProceedsWithdrawn {
			return fairLaunchV3Plan{}, errors.New("Fair Launch project proceeds state invalid")
		}
		amountRaw, tokenAddress, walletAddress, counterparty, entryType, assetRole, flow, tokenDecimals = rawArg(event, "arg3"), addressArg(event, "arg1"), addressArg(event, "arg2"), contract, "project_proceeds_paid", "payment", "outflow", binding.PaymentTokenDecimals
		feeRaw := rawArg(event, "arg4")
		if before.FinalTotalRaisedRaw == nil || addRawCE(amountRaw, feeRaw) != *before.FinalTotalRaisedRaw || before.PlatformFeeWithdrawn && (before.PlatformFeePaidRaw != feeRaw || before.PlatformFeeReservedRaw != feeRaw) {
			return fairLaunchV3Plan{}, errors.New("Fair Launch project proceeds conservation mismatch")
		}
		after.ProjectProceedsWithdrawn, after.ProjectProceedsPaidRaw, after.PlatformFeeReservedRaw = true, amountRaw, feeRaw
	case "FairLaunchPlatformFeeWithdrawn":
		if before.Status != "Successful" || before.PlatformFeeWithdrawn {
			return fairLaunchV3Plan{}, errors.New("Fair Launch platform fee state invalid")
		}
		amountRaw, tokenAddress, walletAddress, counterparty, entryType, assetRole, flow, tokenDecimals = rawArg(event, "arg3"), addressArg(event, "arg1"), addressArg(event, "arg2"), contract, "platform_fee_paid", "payment", "outflow", binding.PaymentTokenDecimals
		if before.FinalTotalRaisedRaw == nil || compareRawUint(amountRaw, *before.FinalTotalRaisedRaw) > 0 || before.ProjectProceedsWithdrawn && (amountRaw != before.PlatformFeeReservedRaw || addRawCE(before.ProjectProceedsPaidRaw, amountRaw) != *before.FinalTotalRaisedRaw) {
			return fairLaunchV3Plan{}, errors.New("Fair Launch platform fee reserve mismatch")
		}
		after.PlatformFeeWithdrawn, after.PlatformFeePaidRaw, after.PlatformFeeReservedRaw = true, amountRaw, amountRaw
	case "FairLaunchFailedProjectTokensWithdrawn":
		if (before.Status != "Failed" && before.Status != "Cancelled") || before.FailedProjectTokensWithdrawn {
			return fairLaunchV3Plan{}, errors.New("Fair Launch failed token withdrawal state invalid")
		}
		amountRaw, tokenAddress, walletAddress, counterparty, entryType, assetRole, flow, tokenDecimals = rawArg(event, "arg3"), addressArg(event, "arg1"), addressArg(event, "arg2"), contract, "project_tokens_withdrawn", "project", "outflow", binding.ProjectTokenDecimals
		if amountRaw != before.TotalTokensDepositedRaw {
			return fairLaunchV3Plan{}, errors.New("Fair Launch failed token amount mismatch")
		}
		after.FailedProjectTokensWithdrawn = true
	default:
		return fairLaunchV3Plan{}, errors.New(FairLaunchLocalE2EForwardUnavailable)
	}
	if binding.ProjectTokenAddress != after.ProjectTokenAddress || binding.PaymentTokenAddress != after.PaymentTokenAddress {
		return fairLaunchV3Plan{}, errors.New("Fair Launch token binding mismatch")
	}
	if tokenAddress != "" {
		expectedToken := binding.PaymentTokenAddress
		if assetRole == "project" {
			expectedToken = binding.ProjectTokenAddress
		}
		if tokenAddress != expectedToken {
			return fairLaunchV3Plan{}, errors.New("Fair Launch token role mismatch")
		}
	}
	beforePayload := fairLaunchProjectStatePayload(before)
	after.StateGeneration = before.StateGeneration + 1
	afterPayload := fairLaunchProjectStatePayload(after)
	beforeJCS, beforeDigest := fairLaunchV3Payload(beforePayload)
	afterJCS, afterDigest := fairLaunchV3Payload(afterPayload)
	after.StateDigest = afterDigest
	mutations := []fairLaunchV3Mutation{{Kind: fairLaunchEffectKind(event.Name), AggregateKind: "project", AggregateKey: strconv.FormatUint(binding.ProjectID, 10), BeforeJCS: beforeJCS, BeforeDigest: beforeDigest, AfterJCS: afterJCS, AfterDigest: afterDigest}}
	ledger := []fairLaunchV3LedgerDraft{}
	if entryType != "" {
		canonical, err := money.NormalizeRawUint256(amountRaw)
		if err != nil || canonical == "0" || !fairLaunchLocalAddressPattern.MatchString(tokenAddress) || !fairLaunchLocalAddressPattern.MatchString(walletAddress) || !fairLaunchLocalAddressPattern.MatchString(counterparty) {
			return fairLaunchV3Plan{}, errors.New("Fair Launch ledger identity invalid")
		}
		display, err := money.FormatRaw(canonical, tokenDecimals)
		if err != nil {
			return fairLaunchV3Plan{}, err
		}
		draft := fairLaunchV3LedgerDraft{EntryType: entryType, AssetRole: assetRole, FlowDirection: flow, WalletAddress: walletAddress, CounterpartyAddress: counterparty, TokenAddress: tokenAddress, TokenDecimals: tokenDecimals, AmountRaw: canonical, AmountDisplay: display}
		draft.RowDigest = fairLaunchV3DomainDigest("JC_FL_LEDGER_ROW_V1", map[string]any{"entryOrdinal": 1, "entryType": draft.EntryType, "assetRole": draft.AssetRole, "flowDirection": draft.FlowDirection, "walletAddress": draft.WalletAddress, "counterpartyAddress": draft.CounterpartyAddress, "tokenAddress": draft.TokenAddress, "tokenDecimals": draft.TokenDecimals, "amountRaw": draft.AmountRaw, "amountDisplay": draft.AmountDisplay, "projectId": binding.ProjectID})
		ledger = append(ledger, draft)
	}
	if positionMutation {
		position = &entity.FairLaunchPositionV3{ProjectBindingID: binding.ID, RunRowID: binding.RunRowID, ChainID: binding.ChainID, SourceID: binding.SourceID, ContractAddress: binding.ContractAddress, ContractVersion: binding.ContractVersion, ProjectID: binding.ProjectID, WalletAddress: walletAddress, PaymentTokenAddress: binding.PaymentTokenAddress, PaymentTokenDecimals: binding.PaymentTokenDecimals, ContributionAmountRaw: "0", ContributionAmountDisplay: "0", ProjectTokenAddress: binding.ProjectTokenAddress, ProjectTokenDecimals: binding.ProjectTokenDecimals, ClaimedTokensRaw: "0", ClaimedTokensDisplay: "0", StateGeneration: 1, Tombstoned: false, Status: "canonical"}
	}
	ledgerDigest := fairLaunchV3DomainDigest("JC_FL_LEDGER_SET_V1", ledger)
	mutationDigest := fairLaunchV3DomainDigest("JC_FL_MUTATION_SET_V1", mutations)
	return fairLaunchV3Plan{Effect: effect, PayloadDigest: payloadDigest, EffectDigest: effect.EffectDigest, LedgerDigest: ledgerDigest, MutationDigest: mutationDigest, Project: after, Position: position, Mutations: mutations, Ledger: ledger}, nil
}

func prepareFairLaunchV3PositionPlan(tx *gorm.DB, plan *fairLaunchV3Plan, event *commonchain.FairLaunchTypedEvent, binding entity.FairLaunchProjectBindingV3, beforeProject entity.FairLaunchProjectStateV3) error {
	if plan.Position == nil {
		return nil
	}
	wallet := plan.Position.WalletAddress
	var position entity.FairLaunchPositionV3
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_binding_id=? AND wallet_address=?", binding.ID, wallet).Take(&position).Error
	created := false
	if errors.Is(err, gorm.ErrRecordNotFound) {
		position = *plan.Position
		created = true
	} else if err != nil {
		return err
	}
	if position.Tombstoned || position.Status != "canonical" || position.PaymentTokenAddress != binding.PaymentTokenAddress || position.ProjectTokenAddress != binding.ProjectTokenAddress || position.PaymentTokenDecimals != binding.PaymentTokenDecimals || position.ProjectTokenDecimals != binding.ProjectTokenDecimals {
		return errors.New("Fair Launch position binding mismatch")
	}
	if !created && !fairLaunchPositionStateDigestValid(position) {
		return errors.New("Fair Launch position state digest mismatch")
	}
	beforePayload := fairLaunchPositionStatePayload(position)
	amount := plan.Ledger[0].AmountRaw
	switch event.Name {
	case "FairLaunchContribution":
		if rawArg(event, "arg4") != addRawCE(position.ContributionAmountRaw, amount) {
			return errors.New("Fair Launch contribution cumulative amount mismatch")
		}
		position.ContributionAmountRaw = rawArg(event, "arg4")
		if position.FirstContributionEventID == nil {
			plan.Project.ParticipantCountRaw = addRawCE(beforeProject.ParticipantCountRaw, "1")
		}
	case "FairLaunchRefunded":
		if created || position.Refunded || position.ContributionAmountRaw != amount {
			return errors.New("Fair Launch refund projection mismatch")
		}
		position.Refunded = true
	case "FairLaunchClaimed":
		if created || rawArg(event, "arg4") != addRawCE(position.ClaimedTokensRaw, amount) {
			return errors.New("Fair Launch claim cumulative amount mismatch")
		}
		position.ClaimedTokensRaw = rawArg(event, "arg4")
	}
	var formatErr error
	position.ContributionAmountDisplay, formatErr = money.FormatRaw(position.ContributionAmountRaw, position.PaymentTokenDecimals)
	if formatErr != nil {
		return formatErr
	}
	position.ClaimedTokensDisplay, formatErr = money.FormatRaw(position.ClaimedTokensRaw, position.ProjectTokenDecimals)
	if formatErr != nil {
		return formatErr
	}
	if !created {
		position.StateGeneration++
	}
	afterPayload := fairLaunchPositionStatePayload(position)
	beforeJCS, beforeDigest := fairLaunchV3Payload(beforePayload)
	afterJCS, afterDigest := fairLaunchV3Payload(afterPayload)
	position.StateDigest = afterDigest
	plan.Position = &position
	projectBeforeJCS, projectBeforeDigest := fairLaunchV3Payload(fairLaunchProjectStatePayload(beforeProject))
	projectAfterJCS, projectAfterDigest := fairLaunchV3Payload(fairLaunchProjectStatePayload(plan.Project))
	plan.Project.StateDigest = projectAfterDigest
	plan.Mutations[0] = fairLaunchV3Mutation{Kind: fairLaunchEffectKind(event.Name), AggregateKind: "project", AggregateKey: strconv.FormatUint(binding.ProjectID, 10), BeforeJCS: projectBeforeJCS, BeforeDigest: projectBeforeDigest, AfterJCS: projectAfterJCS, AfterDigest: projectAfterDigest}
	plan.Mutations = append(plan.Mutations, fairLaunchV3Mutation{Kind: fairLaunchEffectKind(event.Name), AggregateKind: "position", AggregateKey: strconv.FormatUint(binding.ProjectID, 10) + ":" + wallet, BeforeJCS: beforeJCS, BeforeDigest: beforeDigest, AfterJCS: afterJCS, AfterDigest: afterDigest})
	plan.MutationDigest = fairLaunchV3DomainDigest("JC_FL_MUTATION_SET_V1", plan.Mutations)
	return nil
}

func persistFairLaunchV3Projection(tx *gorm.DB, plan *fairLaunchV3Plan, _ entity.FairLaunchProjectStateV3, _ entity.FairLaunchProjectBindingV3, lineage entity.FairLaunchApplicationLineageV3, application entity.ChainEventApplicationAttemptV2, identityID, placementID uint64, now time.Time) error {
	after := plan.Project
	after.LastEventIdentityID = &identityID
	after.LastPlacementID = &placementID
	after.LastSourceRevisionID = &lineage.SourceRevisionID
	after.LastEffectManifestVersion = &lineage.EffectManifestVersion
	after.LastApplicationAttemptID = &application.ID
	after.SourceApplicationLineageID = &lineage.ID
	after.UpdatedAt = now
	if err := tx.Save(&after).Error; err != nil {
		return err
	}
	if plan.Position == nil {
		return nil
	}
	position := *plan.Position
	switch plan.Effect.EventName {
	case "FairLaunchContribution":
		if position.FirstContributionEventID == nil {
			position.FirstContributionEventID = &identityID
		}
		position.LastContributionEventID = &identityID
	case "FairLaunchRefunded":
		position.LastRefundEventID = &identityID
	case "FairLaunchClaimed":
		position.LastClaimEventID = &identityID
	}
	position.LastEventIdentityID, position.LastPlacementID, position.LastSourceRevisionID, position.LastEffectManifestVersion, position.LastApplicationAttemptID, position.SourceApplicationLineageID = &identityID, &placementID, &lineage.SourceRevisionID, &lineage.EffectManifestVersion, &application.ID, &lineage.ID
	position.UpdatedAt = now
	if position.ID == 0 {
		position.CreatedAt = now
		return tx.Create(&position).Error
	}
	return tx.Save(&position).Error
}

func fairLaunchProjectStatePayload(row entity.FairLaunchProjectStateV3) map[string]any {
	return map[string]any{"projectId": row.ProjectID, "status": row.Status, "terminalReason": row.TerminalReason, "saleTokenAmountRaw": row.SaleTokenAmountRaw, "totalRaisedRaw": row.TotalRaisedRaw, "totalTokensDepositedRaw": row.TotalTokensDepositedRaw, "totalTokensClaimedRaw": row.TotalTokensClaimedRaw, "participantCountRaw": row.ParticipantCountRaw, "finalTotalRaisedRaw": row.FinalTotalRaisedRaw, "finalPriceNumeratorRaw": row.FinalPriceNumeratorRaw, "finalPriceDenominatorRaw": row.FinalPriceDenominatorRaw, "platformFeeReservedRaw": row.PlatformFeeReservedRaw, "projectProceedsPaidRaw": row.ProjectProceedsPaidRaw, "platformFeePaidRaw": row.PlatformFeePaidRaw, "projectProceedsWithdrawn": row.ProjectProceedsWithdrawn, "platformFeeWithdrawn": row.PlatformFeeWithdrawn, "failedProjectTokensWithdrawn": row.FailedProjectTokensWithdrawn, "stateGeneration": row.StateGeneration}
}

func fairLaunchPositionStatePayload(row entity.FairLaunchPositionV3) map[string]any {
	return map[string]any{"projectId": row.ProjectID, "walletAddress": row.WalletAddress, "contributionAmountRaw": row.ContributionAmountRaw, "claimedTokensRaw": row.ClaimedTokensRaw, "refunded": row.Refunded, "status": row.Status, "stateGeneration": row.StateGeneration}
}

func fairLaunchProjectStateDigestValid(row entity.FairLaunchProjectStateV3) bool {
	if !fairLaunchLocalDigestPattern.MatchString(row.StateDigest) {
		return false
	}
	_, expected := fairLaunchV3Payload(fairLaunchProjectStatePayload(row))
	return row.StateDigest == expected
}

func fairLaunchPositionStateDigestValid(row entity.FairLaunchPositionV3) bool {
	if !fairLaunchLocalDigestPattern.MatchString(row.StateDigest) {
		return false
	}
	_, expected := fairLaunchV3Payload(fairLaunchPositionStatePayload(row))
	return row.StateDigest == expected
}

func fairLaunchAttestationValid(attestation entity.FairLaunchEffectAttestationV3, lineage entity.FairLaunchApplicationLineageV3) bool {
	if attestation.SourceApplicationLineageID != lineage.ID || attestation.PayloadDigest != lineage.PayloadDigest || attestation.EffectDigest != lineage.EffectDigest || attestation.LedgerDigest != lineage.LedgerDigest || attestation.MutationDigest != lineage.MutationDigest || !fairLaunchLocalDigestPattern.MatchString(attestation.AttestationDigest) {
		return false
	}
	expected := fairLaunchAttestationDigest(attestation.EventIdentityID, attestation.PlacementID, attestation.SourceApplicationLineageID, attestation.PayloadDigest, attestation.EffectDigest, attestation.LedgerDigest, attestation.MutationDigest, attestation.LedgerRowCount, attestation.MutationRowCount, attestation.ProjectMutationCount, attestation.PositionMutationCount)
	return attestation.AttestationDigest == expected
}

func fairLaunchAttestationDigest(eventIdentityID, placementID, lineageID uint64, payloadDigest, effectDigest, ledgerDigest, mutationDigest string, ledgerRowCount, mutationRowCount, projectMutationCount, positionMutationCount uint64) string {
	return fairLaunchV3DomainDigest("JC_FL_EFFECT_ATTESTATION_V1", map[string]any{"eventIdentityId": eventIdentityID, "placementId": placementID, "lineageId": lineageID, "payloadDigest": payloadDigest, "effectDigest": effectDigest, "ledgerDigest": ledgerDigest, "mutationDigest": mutationDigest, "ledgerRowCount": ledgerRowCount, "mutationRowCount": mutationRowCount, "projectMutationCount": projectMutationCount, "positionMutationCount": positionMutationCount})
}

func fairLaunchV3Payload(value any) (string, string) {
	raw, _ := jsonv2.Marshal(value, jsonv2.Deterministic(true))
	return string(raw), fairLaunchV3DomainDigest("JC_FL_AGGREGATE_STATE_V1", value)
}

func fairLaunchV3DomainDigest(domain string, value any) string {
	raw, _ := jsonv2.Marshal(value, jsonv2.Deterministic(true))
	digest := sha256.Sum256(append(append([]byte(domain), 0), raw...))
	return hex.EncodeToString(digest[:])
}

func fairLaunchEffectKind(eventName string) string {
	var out []rune
	for index, value := range eventName {
		if index > 0 && value >= 'A' && value <= 'Z' {
			out = append(out, '_')
		}
		if value >= 'A' && value <= 'Z' {
			value += 'a' - 'A'
		}
		out = append(out, value)
	}
	return strings.TrimPrefix(string(out), "fair_launch_")
}

func rawArg(event *commonchain.FairLaunchTypedEvent, key string) string {
	value, exists := event.RawDecimal[key]
	if !exists {
		return ""
	}
	raw, err := money.NormalizeRawUint256(value)
	if err != nil {
		return ""
	}
	return raw
}

func rawUint64(event *commonchain.FairLaunchTypedEvent, key string) uint64 {
	value, _ := strconv.ParseUint(rawArg(event, key), 10, 64)
	return value
}

func addressArg(event *commonchain.FairLaunchTypedEvent, key string) string {
	for _, values := range []map[string]any{event.Indexed, event.Values} {
		if address, ok := values[key].(common.Address); ok && address != (common.Address{}) {
			return strings.ToLower(address.Hex())
		}
	}
	return ""
}

func hashArg(event *commonchain.FairLaunchTypedEvent, key string) string {
	for _, values := range []map[string]any{event.Indexed, event.Values} {
		switch value := values[key].(type) {
		case common.Hash:
			return strings.ToLower(value.Hex())
		case [32]byte:
			return "0x" + hex.EncodeToString(value[:])
		}
	}
	return ""
}

func fairLaunchTerminalState(statusRaw, reasonRaw string) (string, string, error) {
	statuses := map[string]string{"2": "SuccessfulPendingRelease", "4": "Failed"}
	reasons := map[string]string{"0": "soft_cap_met", "1": "soft_cap_not_met", "3": "release_invalidated", "4": "release_invalidated", "5": "payment_token_invalidated"}
	status, statusOK := statuses[statusRaw]
	reason, reasonOK := reasons[reasonRaw]
	if !statusOK || !reasonOK || (status == "SuccessfulPendingRelease" && reasonRaw != "0") || (status == "Failed" && reasonRaw == "0") {
		return "", "", errors.New("Fair Launch terminal status/reason invalid")
	}
	return status, reason, nil
}

func compareFairLaunchPosition(block uint64, txIndex, ordinal uint32, oldBlock uint64, oldTx, oldOrdinal uint32) int {
	left := []*big.Int{new(big.Int).SetUint64(block), new(big.Int).SetUint64(uint64(txIndex)), new(big.Int).SetUint64(uint64(ordinal))}
	right := []*big.Int{new(big.Int).SetUint64(oldBlock), new(big.Int).SetUint64(uint64(oldTx)), new(big.Int).SetUint64(uint64(oldOrdinal))}
	for index := range left {
		if compared := left[index].Cmp(right[index]); compared != 0 {
			return compared
		}
	}
	return 0
}

func rawStringPointer(value string) *string { return &value }

func init() {
	keys := make([]string, 0, len(fairLaunchLocalForwardEvents))
	for key := range fairLaunchLocalForwardEvents {
		keys = append(keys, key)
	}
	sort.Strings(keys)
}

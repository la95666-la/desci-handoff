package chainevent

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"joincare_server/pkg/common/entity"
)

type FairLaunchLocalE2EAppliedEvidence struct {
	EventIdentityID      uint64 `json:"eventIdentityId"`
	PlacementID          uint64 `json:"placementId"`
	ObservationID        uint64 `json:"observationId"`
	ApplicationAttemptID uint64 `json:"applicationAttemptId"`
	LineageID            uint64 `json:"lineageId"`
	ProjectID            uint64 `json:"projectId"`
	ProjectStateDigest   string `json:"projectStateDigest"`
	PositionStateDigest  string `json:"positionStateDigest,omitzero"`
	LedgerRowCount       uint64 `json:"ledgerRowCount"`
	MutationRowCount     uint64 `json:"mutationRowCount"`
	CursorBlock          uint64 `json:"cursorBlock"`
	CursorGeneration     uint64 `json:"cursorGeneration"`
}

func VerifyFairLaunchLocalE2EApplied(capability FairLaunchLocalE2ECapability, database *gorm.DB, result FairLaunchCEWriteResult) (FairLaunchLocalE2EAppliedEvidence, error) {
	if capability.seal == nil || database == nil || result.IdentityID == 0 || result.PlacementID == 0 || result.ObservationID == 0 || result.ApplicationID == 0 {
		return FairLaunchLocalE2EAppliedEvidence{}, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	var application entity.ChainEventApplicationAttemptV2
	if err := database.Where("id=? AND event_identity_id=? AND placement_id=? AND effect_version=? AND disposition IN('exact','no_op')", result.ApplicationID, result.IdentityID, result.PlacementID, FairLaunchEffectManifestVersion).Take(&application).Error; err != nil {
		return FairLaunchLocalE2EAppliedEvidence{}, err
	}
	var lineage entity.FairLaunchApplicationLineageV3
	if err := database.Where("ce_application_attempt_id=? AND run_row_id=? AND source_id=? AND event_identity_id=? AND placement_id=? AND source_revision_id=? AND effect_manifest_version=? AND state='applied'", application.ID, capability.seal.admission.RunRowID, capability.seal.admission.SourceID, result.IdentityID, result.PlacementID, capability.seal.admission.SourceRevisionID, FairLaunchEffectManifestVersion).Take(&lineage).Error; err != nil {
		return FairLaunchLocalE2EAppliedEvidence{}, err
	}
	forwardLineageID := lineage.ID
	if lineage.Disposition == "replay_no_op" {
		if lineage.PredecessorLineageID == nil {
			return FairLaunchLocalE2EAppliedEvidence{}, errors.New(FairLaunchLocalE2EAdmissionRejected)
		}
		forwardLineageID = *lineage.PredecessorLineageID
	}
	var guard entity.FairLaunchEffectGuardV3
	if err := database.Where("event_identity_id=? AND effect_manifest_version=? AND source_application_lineage_id=? AND state='applied' AND active_slot=1", result.IdentityID, FairLaunchEffectManifestVersion, forwardLineageID).Take(&guard).Error; err != nil {
		return FairLaunchLocalE2EAppliedEvidence{}, err
	}
	var attestation entity.FairLaunchEffectAttestationV3
	if err := database.Where("source_application_lineage_id=?", forwardLineageID).Take(&attestation).Error; err != nil {
		return FairLaunchLocalE2EAppliedEvidence{}, err
	}
	var forwardLineage entity.FairLaunchApplicationLineageV3
	if err := database.Where("id=? AND disposition='forward' AND state='applied'", forwardLineageID).Take(&forwardLineage).Error; err != nil || !fairLaunchAttestationValid(attestation, forwardLineage) {
		return FairLaunchLocalE2EAppliedEvidence{}, fmt.Errorf("%s: applied attestation evidence mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	var ledgerCount, journalCount int64
	if err := database.Model(&entity.FairLaunchLedgerEntryV3{}).Where("source_application_lineage_id=?", forwardLineageID).Count(&ledgerCount).Error; err != nil || uint64(ledgerCount) != attestation.LedgerRowCount {
		return FairLaunchLocalE2EAppliedEvidence{}, fmt.Errorf("%s: applied ledger evidence mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	if err := database.Model(&entity.FairLaunchEffectJournalV3{}).Where("source_application_lineage_id=?", forwardLineageID).Count(&journalCount).Error; err != nil || uint64(journalCount) != attestation.MutationRowCount {
		return FairLaunchLocalE2EAppliedEvidence{}, fmt.Errorf("%s: applied mutation evidence mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	var project entity.FairLaunchProjectStateV3
	if err := database.Where("last_event_identity_id=? AND last_placement_id=? AND source_application_lineage_id=?", result.IdentityID, result.PlacementID, forwardLineageID).Take(&project).Error; err != nil {
		return FairLaunchLocalE2EAppliedEvidence{}, err
	}
	if !fairLaunchProjectStateDigestValid(project) {
		return FairLaunchLocalE2EAppliedEvidence{}, fmt.Errorf("%s: applied project projection digest mismatch", FairLaunchLocalE2EAdmissionRejected)
	}
	positionDigest := ""
	if attestation.PositionMutationCount == 1 {
		var position entity.FairLaunchPositionV3
		if err := database.Where("last_event_identity_id=? AND last_placement_id=? AND source_application_lineage_id=?", result.IdentityID, result.PlacementID, forwardLineageID).Take(&position).Error; err != nil {
			return FairLaunchLocalE2EAppliedEvidence{}, err
		}
		if !fairLaunchPositionStateDigestValid(position) {
			return FairLaunchLocalE2EAppliedEvidence{}, fmt.Errorf("%s: applied position projection digest mismatch", FairLaunchLocalE2EAdmissionRejected)
		}
		positionDigest = position.StateDigest
	}
	var cursor entity.SourceMaterializationCursor
	if err := database.Where("source_id=? AND source_revision_id=? AND last_event_identity_id=? AND status='active'", capability.seal.admission.SourceID, capability.seal.admission.SourceRevisionID, result.IdentityID).Take(&cursor).Error; err != nil {
		return FairLaunchLocalE2EAppliedEvidence{}, err
	}
	return FairLaunchLocalE2EAppliedEvidence{EventIdentityID: result.IdentityID, PlacementID: result.PlacementID, ObservationID: result.ObservationID, ApplicationAttemptID: result.ApplicationID, LineageID: lineage.ID, ProjectID: project.ProjectID, ProjectStateDigest: project.StateDigest, PositionStateDigest: positionDigest, LedgerRowCount: attestation.LedgerRowCount, MutationRowCount: attestation.MutationRowCount, CursorBlock: cursor.MaterializedThroughBlock, CursorGeneration: cursor.CursorGeneration}, nil
}

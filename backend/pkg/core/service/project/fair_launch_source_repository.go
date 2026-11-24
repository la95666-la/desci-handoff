package project

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/projectruntime"
)

// FairLaunchSourceIngestionTx is intentionally not constructible by ordinary
// repositories. A future separately reviewed ingestion-pool bootstrap may return
// one only after pinning a transaction and verifying CURRENT_USER().
type FairLaunchSourceIngestionTx struct{ tx *gorm.DB }

func newFairLaunchSourceIngestionTx(tx *gorm.DB) (*FairLaunchSourceIngestionTx, error) {
	if tx == nil {
		return nil, errors.New("source registry ingestion transaction unavailable")
	}
	if _, ok := tx.Statement.ConnPool.(gorm.TxCommitter); !ok {
		return nil, errors.New("source snapshot insertion requires a pinned transaction")
	}
	var currentUser string
	if err := tx.Raw("SELECT CURRENT_USER()").Scan(&currentUser).Error; err != nil || currentUser != "jc_fl_registry_ingest@%" {
		return nil, errors.New("source registry ingestion identity required")
	}
	return &FairLaunchSourceIngestionTx{tx: tx}, nil
}

func StoreFairLaunchSourceSnapshotTx(ingestion *FairLaunchSourceIngestionTx, row entity.FairLaunchSourceRegistrySnapshot, registry projectruntime.SourceRegistry, now time.Time) error {
	if ingestion == nil || ingestion.tx == nil || row.Revision == 0 || row.KeySetVersion == 0 || now.IsZero() {
		return errors.New("source_registry_ingestion_pool_unavailable")
	}
	tx := ingestion.tx
	trusted, err := registry.Source("fair_launch_v1")
	if err != nil {
		return err
	}
	if !fairLaunchSourceSnapshotIdentityMatches(row, trusted) {
		return errors.New("source snapshot exact binding is incomplete or unrelated")
	}
	binding := fairLaunchRegistryBinding{Environment: row.Environment, Revision: row.Revision, ChainID: row.ChainID, ProxyAddress: row.ProxyAddress, ImplementationAddress: row.ImplementationAddress, ImplementationCodeHash: row.ImplementationCodeHash, ABIDigest: row.ABIDigest, SourceID: row.SourceID, SourceRevisionID: row.SourceRevisionID, SourceRevisionNo: row.SourceRevisionNo, RuntimeGeneration: row.RuntimeGeneration, PolicyGeneration: row.PolicyGeneration, SourceRegistryGeneration: row.SourceRegistryGeneration, ConfigGeneration: row.ConfigGeneration}
	canonical, err := canonicalRegistryPayload(binding)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(canonical)
	computed := hex.EncodeToString(digest[:])
	if supplied := strings.TrimPrefix(strings.ToLower(row.PayloadDigest), "0x"); supplied != computed {
		return errors.New("source snapshot canonical payload digest mismatch")
	}
	row.CanonicalPayload = string(canonical)
	row.PayloadDigest = computed
	row.EnvelopeDigest = strings.TrimPrefix(strings.ToLower(row.EnvelopeDigest), "0x")
	row.ProxyAddress = strings.ToLower(prefixedHex(row.ProxyAddress))
	row.ImplementationAddress = strings.ToLower(prefixedHex(row.ImplementationAddress))
	row.ImplementationCodeHash = strings.ToLower(prefixedHex(row.ImplementationCodeHash))
	row.ABIDigest = strings.ToLower(prefixedHex(row.ABIDigest))
	row.AcceptedAt = now.UTC()
	// The reviewed stored procedure is the only path that may establish the
	// trigger's session evidence. Its EXIT HANDLER clears the variable and
	// RESIGNALs; direct INSERT remains trigger-rejected.
	call := `CALL jc_insert_fair_launch_source_registry_snapshot(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`
	if err := tx.Exec(call,
		row.Environment, row.Revision, row.CanonicalPayload, row.PayloadDigest, computed,
		row.EnvelopeDigest, row.ChainID, row.ProxyAddress, row.ImplementationAddress,
		row.ImplementationCodeHash, row.ABIDigest, row.SourceID, row.SourceRevisionID,
		row.SourceRevisionNo, row.RuntimeGeneration, row.PolicyGeneration,
		row.SourceRegistryGeneration, row.ConfigGeneration, row.KeySetVersion,
		row.ValidFrom, row.ValidUntil, row.AcceptedAt,
	).Error; err != nil {
		return err
	}
	var stored fairLaunchRegistryBinding
	if err := tx.Table("fair_launch_source_registry_snapshot").Where("environment=? AND revision=?", row.Environment, row.Revision).Take(&stored).Error; err != nil {
		return err
	}
	storedCanonical, err := validateCanonicalRegistryText(stored.CanonicalPayload, stored)
	if err != nil || string(storedCanonical) != row.CanonicalPayload || !strings.EqualFold(stored.PayloadDigest, computed) {
		return errors.New("source snapshot read-back JCS verification failed")
	}
	return nil
}

func fairLaunchSourceSnapshotIdentityMatches(row entity.FairLaunchSourceRegistrySnapshot, trusted projectruntime.TrustedSource) bool {
	return row.Environment != "" && row.ChainID == trusted.Identity.ChainID &&
		strings.EqualFold(prefixedHex(row.ProxyAddress), prefixedHex(trusted.Identity.ContractAddress)) &&
		strings.EqualFold(prefixedHex(row.ImplementationAddress), prefixedHex(trusted.Revision.ImplementationAddress)) &&
		strings.EqualFold(prefixedHex(row.ImplementationCodeHash), prefixedHex(trusted.Revision.ImplementationCodeHash)) &&
		strings.EqualFold(prefixedHex(row.ABIDigest), prefixedHex(trusted.Revision.ABIDigest)) &&
		row.SourceID > 0 && row.SourceRevisionID > 0 && row.SourceRevisionNo == trusted.Revision.RevisionNo &&
		row.RuntimeGeneration > 0 && row.PolicyGeneration > 0 && row.SourceRegistryGeneration > 0 && row.ConfigGeneration > 0
}
func AdvanceFairLaunchWatermarkTx(tx *gorm.DB, next entity.FairLaunchFeatureWatermark) error {
	if next.Environment == "" || next.Generation == 0 || next.SourceRegistryRevision == 0 || next.KeySetVersion == 0 {
		return errors.New("invalid feature watermark")
	}
	var current entity.FairLaunchFeatureWatermark
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("environment=?", next.Environment).Take(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(&next).Error
	}
	if err != nil {
		return err
	}
	if next.Generation <= current.Generation || next.SourceRegistryRevision < current.SourceRegistryRevision || next.SchemaGeneration < current.SchemaGeneration || next.EventModelGeneration < current.EventModelGeneration {
		return errors.New("feature watermark rollback rejected")
	}
	return tx.Save(&next).Error
}
func AcceptRestoreAttestationTx(tx *gorm.DB, row *entity.FairLaunchRestoreRecoveryAttestation, now time.Time) error {
	if row == nil || row.InitiatorID == 0 || row.ApproverID == 0 || row.ExecutorID == 0 || row.InitiatorID == row.ApproverID || row.InitiatorID == row.ExecutorID || row.ApproverID == row.ExecutorID {
		return errors.New("restore requires three distinct actors")
	}
	if row.ReconciliationState != "balanced" {
		return errors.New("restore reconciliation is not balanced")
	}
	var watermark entity.FairLaunchFeatureWatermark
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("environment=? AND generation=?", row.Environment, row.WatermarkGeneration).Take(&watermark).Error; err != nil {
		return err
	}
	row.AcceptedAt = &now
	return tx.Create(row).Error
}

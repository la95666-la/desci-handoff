package chainevent

import (
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"joincare_server/pkg/common/entity"
	"strings"
	"time"
)

type FairLaunchCursorFence struct {
	LeaseToken string
	FenceEpoch uint64
}

func AcquireSourceMaterializationCursorTx(tx *gorm.DB, sourceID, sourceRevisionID uint64, fence FairLaunchCursorFence, now time.Time) (entity.SourceMaterializationCursor, error) {
	if sourceID == 0 || sourceRevisionID == 0 || fence.LeaseToken == "" || fence.FenceEpoch == 0 {
		return entity.SourceMaterializationCursor{}, errors.New("invalid source cursor lease")
	}
	var row entity.SourceMaterializationCursor
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("source_id=?", sourceID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = entity.SourceMaterializationCursor{SourceID: sourceID, SourceRevisionID: sourceRevisionID, MaterializedThroughBlockHash: "0x" + strings.Repeat("0", 64), LeaseToken: fence.LeaseToken, FenceEpoch: fence.FenceEpoch, Status: "active", UpdatedAt: now, CursorGeneration: 1}
		err = tx.Create(&row).Error
		return row, err
	}
	if err != nil {
		return entity.SourceMaterializationCursor{}, err
	}
	if row.FenceEpoch >= fence.FenceEpoch && row.LeaseToken != fence.LeaseToken {
		return entity.SourceMaterializationCursor{}, errors.New("stale source cursor fence")
	}
	row.LeaseToken = fence.LeaseToken
	row.FenceEpoch = fence.FenceEpoch
	row.SourceRevisionID = sourceRevisionID
	row.Status = "active"
	row.UpdatedAt = now
	row.CursorGeneration++
	return row, tx.Save(&row).Error
}
func AppendSourceIngestionCoverageTx(tx *gorm.DB, row entity.SourceIngestionCoverage) error {
	if row.SourceID == 0 || row.FromBlock > row.ThroughBlock || len(row.CoverageDigest) != 64 {
		return errors.New("invalid source coverage")
	}
	return tx.Create(&row).Error
}

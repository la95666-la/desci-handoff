package chainreorg

import (
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/entity"
)

var ErrActiveIncident = errors.New("active chain event reorg incident blocks chain writes")

// LockNoActiveIncidentTx is the shared linearization boundary for chain writes.
// Callers must invoke it before locking project, outbox, or nonce rows.
func LockNoActiveIncidentTx(tx *gorm.DB, chainID uint64, contractAddress string) error {
	if tx == nil || chainID == 0 || strings.TrimSpace(contractAddress) == "" {
		return errors.New("chain reorg write gate identity is incomplete")
	}
	state := entity.ChainEventSyncState{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = 1").Take(&state).Error; err != nil {
		return err
	}
	var count int64
	if err := tx.Model(&entity.ChainEventReorgIncident{}).
		Where("chain_id = ? AND active_slot = 1 AND status = ?", chainID, "detected").
		Count(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return ErrActiveIncident
	}
	return nil
}

func RequireNoActiveIncident(database *gorm.DB, chainID uint64, contractAddress string) error {
	if database == nil {
		return errors.New("chain reorg write gate database is unavailable")
	}
	return database.Transaction(func(tx *gorm.DB) error {
		return LockNoActiveIncidentTx(tx, chainID, contractAddress)
	})
}

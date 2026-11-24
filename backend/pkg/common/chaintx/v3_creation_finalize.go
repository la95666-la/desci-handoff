package chaintx

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// The project is already locked by the creation receipt wrapper. Lock the
// exact submission before its shared nonce, then verify its DB-derived UTC
// signing instant. A stale preliminary read cannot establish this deadline.
func finalizeV3CreationReceiptLockedTx(tx *gorm.DB, row entity.ChainTxSubmission,
	binding idov3whitelist.ExpectedProjectBinding, operator common.Address, audit V3ReceiptAudit,
	setExpiresAt int64) (entity.ChainTxSubmission, error) {
	invalid := errors.New("V3 creation signed deadline unavailable")
	if tx == nil || row.Action != "ido_v3_create" {
		return entity.ChainTxSubmission{}, invalid
	}
	var current entity.ChainTxSubmission
	if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", row.ID).Take(&current).Error != nil ||
		!sameV3SignedAttempt(row, current) || !V3SignedBeforeSet(current, setExpiresAt) {
		return entity.ChainTxSubmission{}, invalid
	}
	return finalizeV3LifecycleReceiptLockedTx(tx, row, binding, operator, audit)
}

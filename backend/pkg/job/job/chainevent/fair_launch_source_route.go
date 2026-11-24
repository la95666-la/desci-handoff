package chainevent

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"gorm.io/gorm"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/projectruntime"
	"strings"
	"time"
)

func ResolveFairLaunchSourceRevisionTx(tx *gorm.DB, chainID uint64, emittingAddress string, block uint64, txIndex, ordinal uint, registry *projectruntime.SourceRegistry, now time.Time) (entity.ChainContractSourceRevision, error) {
	if registry == nil || chainID == 0 {
		return entity.ChainContractSourceRevision{}, errors.New("trusted source registry required")
	}
	trusted, err := registry.Source("fair_launch_v1")
	if err != nil {
		return entity.ChainContractSourceRevision{}, err
	}
	if trusted.Identity.ChainID != chainID || !strings.EqualFold(trusted.Identity.ContractAddress, emittingAddress) {
		return quarantineUnknownFairLaunchSource(tx, chainID, emittingAddress, now)
	}
	var source entity.ChainContractSource
	if err := tx.Where("chain_id=? AND contract_address=? AND contract_version=? AND enabled=1", chainID, strings.ToLower(emittingAddress), "fair_launch_v1").Take(&source).Error; err != nil {
		return entity.ChainContractSourceRevision{}, err
	}
	var revision entity.ChainContractSourceRevision
	if err := tx.Where(
		"source_id=? AND (from_block_number < ? OR (from_block_number = ? AND (from_transaction_index < ? OR (from_transaction_index = ? AND from_tx_log_ordinal_inclusive <= ?))))",
		source.ID, block, block, txIndex, txIndex, ordinal,
	).Order("from_block_number DESC, from_transaction_index DESC, from_tx_log_ordinal_inclusive DESC").Take(&revision).Error; err != nil {
		return entity.ChainContractSourceRevision{}, err
	}
	toBoundarySet := revision.ToBlockNumber != nil || revision.ToTransactionIndex != nil || revision.ToTxLogOrdinalExclusive != nil
	if toBoundarySet && (revision.ToBlockNumber == nil || revision.ToTransactionIndex == nil || revision.ToTxLogOrdinalExclusive == nil) {
		return entity.ChainContractSourceRevision{}, errors.New("database source revision end boundary is incomplete")
	}
	if revision.ToBlockNumber != nil && compareFairLaunchSourceBoundary(block, txIndex, ordinal, *revision.ToBlockNumber, uint(*revision.ToTransactionIndex), uint(*revision.ToTxLogOrdinalExclusive)) >= 0 {
		return entity.ChainContractSourceRevision{}, errors.New("database source revision does not cover event placement")
	}
	if revision.RevisionNo != trusted.Revision.RevisionNo || revision.FromBlockNumber != trusted.Revision.EffectiveFromBlock || uint(revision.FromTransactionIndex) != trusted.Revision.EffectiveFromTxIndex || uint(revision.FromTxLogOrdinalInclusive) != trusted.Revision.EffectiveFromTxLogOrdinal {
		return entity.ChainContractSourceRevision{}, errors.New("database source revision is not the trusted signed revision")
	}
	return revision, nil
}
func compareFairLaunchSourceBoundary(aBlock uint64, aTx, aOrdinal uint, bBlock uint64, bTx, bOrdinal uint) int {
	if aBlock < bBlock {
		return -1
	}
	if aBlock > bBlock {
		return 1
	}
	if aTx < bTx {
		return -1
	}
	if aTx > bTx {
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

func quarantineUnknownFairLaunchSource(tx *gorm.DB, chainID uint64, address string, now time.Time) (entity.ChainContractSourceRevision, error) {
	sum := sha256.Sum256([]byte(strings.ToLower(address)))
	row := entity.ChainEventIdentityQuarantineV2{ChainID: chainID, TxHash: "0x" + strings.Repeat("0", 64), ReasonCode: "unknown_fair_launch_source", EvidenceDigest: hex.EncodeToString(sum[:]), State: "open", CreatedAt: now}
	if tx != nil {
		_ = tx.Create(&row).Error
	}
	return entity.ChainContractSourceRevision{}, errors.New("unknown Fair Launch emitting proxy quarantined")
}

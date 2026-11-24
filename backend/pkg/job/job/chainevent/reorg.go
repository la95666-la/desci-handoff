package chainevent

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/job/chain"
	"joincare_server/pkg/job/db"
)

const (
	reorgStatusDetected = "detected"
	reorgDetectionCode  = "CURSOR_BLOCK_HASH_MISMATCH"
	reorgEventDetected  = "DETECTED"
	reorgZeroDigest     = "0000000000000000000000000000000000000000000000000000000000000000"
)

type cursorCanonicalityError struct {
	BlockNumber uint64
	Expected    string
	Observed    string
}

func (e *cursorCanonicalityError) Error() string {
	return fmt.Sprintf("cursor block hash mismatch at block %d: stored=%s canonical=%s; reconciliation required", e.BlockNumber, e.Expected, e.Observed)
}

func evidenceDigest(parts ...any) string {
	value := ""
	for i, part := range parts {
		if i > 0 {
			value += "\n"
		}
		value += fmt.Sprint(part)
	}
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func checkpointDigest(chainID, blockNumber uint64, blockHash, previous string) string {
	return evidenceDigest("joincare.chain-event.checkpoint.v1", chainID, blockNumber, strings.ToLower(blockHash), previous)
}

func reorgIncidentDigest(chainID uint64, contractAddress string, blockNumber uint64, expected, observed string) string {
	return evidenceDigest("joincare.chain-event.reorg-incident.v1", chainID, strings.ToLower(contractAddress), blockNumber, strings.ToLower(expected), strings.ToLower(observed))
}

func reorgEventDigest(incidentID, sequence uint64, eventType, message, previous string) string {
	return evidenceDigest("joincare.chain-event.reorg-event.v1", incidentID, sequence, eventType, message, previous)
}

func ensureCursorCheckpointTx(tx *gorm.DB, chainID, blockNumber uint64, blockHash string) (*entity.ChainEventBlockCheckpoint, error) {
	if chainID == 0 || blockNumber == 0 {
		return nil, errors.New("chain event checkpoint identity is invalid")
	}
	blockHash = strings.ToLower(blockHash)
	existing := entity.ChainEventBlockCheckpoint{}
	err := tx.Where("chain_id = ? AND block_number = ?", chainID, blockNumber).Take(&existing).Error
	if err == nil {
		if existing.BlockHash != blockHash {
			return nil, errors.New("stored chain event checkpoint block hash conflict")
		}
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	previous := entity.ChainEventBlockCheckpoint{}
	previousDigest := reorgZeroDigest
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Order("id DESC").Take(&previous).Error
	if err == nil {
		if previous.ChainID != chainID || blockNumber <= previous.BlockNumber {
			return nil, errors.New("chain event checkpoint cursor is not strictly monotonic")
		}
		previousDigest = previous.CheckpointHash
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	row := entity.ChainEventBlockCheckpoint{
		ChainID: chainID, BlockNumber: blockNumber, BlockHash: blockHash,
		PreviousCheckpointHash: previousDigest,
		CheckpointHash:         checkpointDigest(chainID, blockNumber, blockHash, previousDigest),
		CreatedAt:              time.Now(),
	}
	if err := tx.Create(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func persistCursorCheckpointTx(tx *gorm.DB, blockNumber uint64, blockHash string) error {
	if blockNumber == 0 {
		return nil
	}
	client := chain.GetClient()
	if client == nil || client.ChainID == nil || !client.ChainID.IsUint64() {
		return errors.New("chain event checkpoint chain identity is unavailable")
	}
	_, err := ensureCursorCheckpointTx(tx, client.ChainID.Uint64(), blockNumber, blockHash)
	return err
}

func persistDetectedReorgIncident(leaseToken string, state *entity.ChainEventSyncState, mismatch *cursorCanonicalityError) error {
	if state == nil || mismatch == nil || !state.V2Registered {
		return errors.New("V2 reorg incident source is incomplete")
	}
	client := chain.GetClient()
	if client == nil || client.ChainID == nil || !client.ChainID.IsUint64() || client.ChainID.Uint64() == 0 {
		return errors.New("V2 reorg incident chain identity is unavailable")
	}
	chainID := client.ChainID.Uint64()
	contract := strings.ToLower(state.V2ContractAddress)
	incidentKey := reorgIncidentDigest(chainID, contract, mismatch.BlockNumber, mismatch.Expected, mismatch.Observed)
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, leaseToken); err != nil {
			return err
		}
		checkpoint, err := ensureCursorCheckpointTx(tx, chainID, mismatch.BlockNumber, mismatch.Expected)
		if err != nil {
			return err
		}
		active := entity.ChainEventReorgIncident{}
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("chain_id = ? AND contract_address = ? AND active_slot = 1", chainID, contract).Take(&active).Error
		if err == nil {
			if active.IncidentKey != incidentKey {
				return errors.New("a different active chain event reorg incident already exists")
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		activeSlot := uint8(1)
		now := time.Now()
		incident := entity.ChainEventReorgIncident{
			IncidentKey: incidentKey, ChainID: chainID, ContractAddress: contract,
			ContractVersion: constant.ContractVersionV2Stablecoin,
			CursorBlock:     mismatch.BlockNumber, ExpectedBlockHash: strings.ToLower(mismatch.Expected),
			ObservedBlockHash: strings.ToLower(mismatch.Observed), CursorCheckpointID: checkpoint.ID,
			V2StartBlock: state.V2StartBlock, Status: reorgStatusDetected, ActiveSlot: &activeSlot,
			DetectionCode: reorgDetectionCode, DetectedAt: now, CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&incident).Error; err != nil {
			return err
		}
		message := fmt.Sprintf("cursor block %d changed from %s to %s", mismatch.BlockNumber, strings.ToLower(mismatch.Expected), strings.ToLower(mismatch.Observed))
		event := entity.ChainEventReorgEvent{
			IncidentID: incident.ID, SequenceNo: 1, EventType: reorgEventDetected, Message: message,
			PreviousEventHash: reorgZeroDigest,
			EventHash:         reorgEventDigest(incident.ID, 1, reorgEventDetected, message, reorgZeroDigest),
			CreatedAt:         now,
		}
		return tx.Create(&event).Error
	})
}

func hasActiveReorgIncidentTx(tx *gorm.DB, chainID uint64, contractAddress string) (bool, error) {
	var count int64
	err := tx.Model(&entity.ChainEventReorgIncident{}).
		Where("chain_id = ? AND contract_address = ? AND active_slot = 1 AND status = ?", chainID, strings.ToLower(contractAddress), reorgStatusDetected).
		Count(&count).Error
	return count != 0, err
}

func activeReorgIncidentError() error {
	client := chain.GetClient()
	if client == nil || client.ChainID == nil || !client.ChainID.IsUint64() || !chain.V2Enabled() {
		return nil
	}
	active, err := hasActiveReorgIncidentTx(db.Mysql(), client.ChainID.Uint64(), chain.V2ContractAddress().Hex())
	if err != nil {
		return fmt.Errorf("query active chain event reorg incident: %w", err)
	}
	if active {
		return errors.New("active chain event reorg incident requires audited compensation before sync can continue")
	}
	return nil
}

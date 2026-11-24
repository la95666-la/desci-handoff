package chainevent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/job/chain"
	"joincare_server/pkg/job/db"
)

var errDeadLetterBlocked = errors.New("chain event is in dead-letter state; admin replay required")

func chainEventMaxRetries() uint {
	maxRetries := viper.GetUint("chainEvent.maxRetries")
	if maxRetries == 0 {
		return 3
	}
	return maxRetries
}

func ensureEventCanRun(info types.Log) (bool, error) {
	failure := entity.ChainEventFailure{}
	err := db.Mysql().Select("status").
		Where("block_number = ? AND tx_hash = ? AND log_index = ?", info.BlockNumber, info.TxHash.Hex(), info.Index).
		Take(&failure).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query chain event failure state: %w", err)
	}
	if failure.Status == constant.ChainEventFailureDeadLetter {
		return true, errDeadLetterBlocked
	}
	return failure.Status == constant.ChainEventFailureRetrying, nil
}

func recordEventFailure(leaseToken string, info types.Log, event string, processErr error) error {
	rawLog, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal failed chain log: %w", err)
	}
	now := time.Now()
	maxRetries := chainEventMaxRetries()
	errorMessage := truncateFailureError(processErr.Error(), 8192)
	insert := entity.ChainEventFailure{
		Event:            event,
		BlockNumber:      info.BlockNumber,
		BlockHash:        info.BlockHash.Hex(),
		TxHash:           info.TxHash.Hex(),
		LogIndex:         info.Index,
		ContractAddress:  info.Address.Hex(),
		ContractVersion:  eventContractVersion(info.Address),
		RawLog:           string(rawLog),
		ErrorMessage:     errorMessage,
		Status:           constant.ChainEventFailureRetrying,
		RetryCount:       1,
		ConsecutiveCount: 1,
		FirstFailedAt:    now,
		LastFailedAt:     now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if leaseToken == "" {
			return errors.New("chain event failure write requires an active lease token")
		}
		if err := lockChainEventSyncFenceTx(tx, leaseToken); err != nil {
			return err
		}
		var committed int64
		if err := tx.Model(&entity.ChainEventRecord{}).Where("block_number = ? AND tx_hash = ? AND log_index = ?", info.BlockNumber, info.TxHash.Hex(), info.Index).Count(&committed).Error; err != nil {
			return err
		}
		if committed != 0 {
			return tx.Model(&entity.ChainEventFailure{}).
				Where("block_number = ? AND tx_hash = ? AND log_index = ?", info.BlockNumber, info.TxHash.Hex(), info.Index).
				Updates(map[string]interface{}{"status": constant.ChainEventFailureResolved, "consecutive_count": 0, "resolved_at": now, "updated_at": now}).Error
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "block_number"}, {Name: "tx_hash"}, {Name: "log_index"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"event":             event,
				"contract_address":  info.Address.Hex(),
				"contract_version":  eventContractVersion(info.Address),
				"block_hash":        info.BlockHash.Hex(),
				"raw_log":           string(rawLog),
				"error_message":     errorMessage,
				"retry_count":       gorm.Expr("retry_count + 1"),
				"consecutive_count": gorm.Expr("consecutive_count + 1"),
				"last_failed_at":    now,
				"resolved_at":       nil,
				"updated_at":        now,
			}),
		}).Create(&insert).Error; err != nil {
			return err
		}
		return tx.Model(&entity.ChainEventFailure{}).
			Where("block_number = ? AND tx_hash = ? AND log_index = ?", info.BlockNumber, info.TxHash.Hex(), info.Index).
			Update("status", gorm.Expr("CASE WHEN consecutive_count >= ? THEN ? ELSE ? END", maxRetries, constant.ChainEventFailureDeadLetter, constant.ChainEventFailureRetrying)).Error
	})
}

func recordReconciliationFailures(leaseToken string, logs []types.Log, processErr error) error {
	now := time.Now()
	errorMessage := truncateFailureError(processErr.Error(), 8192)
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if leaseToken == "" {
			return errors.New("reconciliation failure write requires an active lease token")
		}
		if err := lockChainEventSyncFenceTx(tx, leaseToken); err != nil {
			return err
		}
		for _, info := range logs {
			if info.Removed || info.BlockNumber == 0 {
				continue
			}
			rawLog, err := json.Marshal(info)
			if err != nil {
				return err
			}
			var committed int64
			if err := tx.Model(&entity.ChainEventRecord{}).Where("block_number = ? AND tx_hash = ? AND log_index = ?", info.BlockNumber, info.TxHash.Hex(), info.Index).Count(&committed).Error; err != nil {
				return err
			}
			if committed != 0 {
				if err := resolveEventFailure(tx, info.BlockNumber, info.TxHash.Hex(), info.Index); err != nil {
					return err
				}
				continue
			}
			insert := entity.ChainEventFailure{
				Event:            "reconciliation",
				BlockNumber:      info.BlockNumber,
				BlockHash:        info.BlockHash.Hex(),
				TxHash:           info.TxHash.Hex(),
				LogIndex:         info.Index,
				ContractAddress:  info.Address.Hex(),
				ContractVersion:  eventContractVersion(info.Address),
				RawLog:           string(rawLog),
				ErrorMessage:     errorMessage,
				Status:           constant.ChainEventFailureDeadLetter,
				RetryCount:       1,
				ConsecutiveCount: 1,
				FirstFailedAt:    now,
				LastFailedAt:     now,
				CreatedAt:        now,
				UpdatedAt:        now,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "block_number"}, {Name: "tx_hash"}, {Name: "log_index"}},
				DoUpdates: clause.Assignments(map[string]interface{}{
					"event":             "reconciliation",
					"contract_address":  info.Address.Hex(),
					"contract_version":  eventContractVersion(info.Address),
					"block_hash":        info.BlockHash.Hex(),
					"raw_log":           string(rawLog),
					"error_message":     errorMessage,
					"status":            constant.ChainEventFailureDeadLetter,
					"retry_count":       gorm.Expr("retry_count + 1"),
					"consecutive_count": gorm.Expr("consecutive_count + 1"),
					"last_failed_at":    now,
					"resolved_at":       nil,
					"updated_at":        now,
				}),
			}).Create(&insert).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func eventContractVersion(address common.Address) string {
	if chain.IsV2Log(address) {
		return constant.ContractVersionV2Stablecoin
	}
	return constant.ContractVersionV1Legacy
}

func truncateFailureError(message string, max int) string {
	if len(message) <= max {
		return message
	}
	for max > 0 && !utf8.ValidString(message[:max]) {
		max--
	}
	return message[:max]
}

func resolveEventFailure(tx *gorm.DB, dataBlock uint64, txHash string, logIndex uint) error {
	now := time.Now()
	return tx.Model(&entity.ChainEventFailure{}).
		Where("block_number = ? AND tx_hash = ? AND log_index = ? AND status <> ?", dataBlock, txHash, logIndex, constant.ChainEventFailureResolved).
		Updates(map[string]interface{}{
			"status":            constant.ChainEventFailureResolved,
			"consecutive_count": 0,
			"resolved_at":       now,
			"updated_at":        now,
		}).Error
}

func ensureNoUnresolvedFailures(fromBlock, toBlock uint64) error {
	return ensureNoUnresolvedFailuresTx(db.Mysql(), fromBlock, toBlock)
}

func ensureNoUnresolvedFailuresTx(tx *gorm.DB, fromBlock, toBlock uint64) error {
	if toBlock < fromBlock {
		return nil
	}
	now := time.Now()
	if err := tx.Exec(`
UPDATE chain_event_failure f
JOIN chain_event_record r
  ON r.block_number = f.block_number
 AND r.tx_hash = f.tx_hash
 AND r.log_index = f.log_index
SET f.status = ?, f.consecutive_count = 0, f.resolved_at = ?, f.updated_at = ?
WHERE f.block_number BETWEEN ? AND ? AND f.event <> 'reconciliation' AND f.status IN ?`,
		constant.ChainEventFailureResolved,
		now,
		now,
		fromBlock,
		toBlock,
		[]string{constant.ChainEventFailureRetrying, constant.ChainEventFailureDeadLetter},
	).Error; err != nil {
		return fmt.Errorf("resolve failures with committed event facts: %w", err)
	}
	var count int64
	if err := tx.Model(&entity.ChainEventFailure{}).
		Where("block_number BETWEEN ? AND ? AND status IN ?", fromBlock, toBlock, []string{
			constant.ChainEventFailureRetrying,
			constant.ChainEventFailureDeadLetter,
		}).Count(&count).Error; err != nil {
		return fmt.Errorf("query unresolved chain event failures: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%d unresolved chain event failure(s) remain in scanned range %d-%d", count, fromBlock, toBlock)
	}
	return nil
}

func ensureUnresolvedFailuresAppearInLogs(fromBlock, toBlock uint64, logs []types.Log) error {
	if toBlock < fromBlock {
		return nil
	}
	present := make(map[string]struct{}, len(logs))
	for _, info := range logs {
		if info.Removed {
			continue
		}
		present[eventIdentity(info.BlockNumber, info.TxHash.Hex(), info.Index)] = struct{}{}
	}
	failures := make([]entity.ChainEventFailure, 0)
	if err := db.Mysql().Select("block_number,tx_hash,log_index,status").
		Where("block_number BETWEEN ? AND ? AND status IN ?", fromBlock, toBlock, []string{
			constant.ChainEventFailureRetrying,
			constant.ChainEventFailureDeadLetter,
		}).Find(&failures).Error; err != nil {
		return fmt.Errorf("query unresolved failures before processing: %w", err)
	}
	for _, failure := range failures {
		if _, ok := present[eventIdentity(failure.BlockNumber, failure.TxHash, failure.LogIndex)]; !ok {
			return fmt.Errorf(
				"unresolved event block=%d tx=%s log_index=%d disappeared from canonical scan; reconciliation required",
				failure.BlockNumber, failure.TxHash, failure.LogIndex,
			)
		}
	}
	return nil
}

func eventIdentity(blockNumber uint64, txHash string, logIndex uint) string {
	return fmt.Sprintf("%d:%s:%d", blockNumber, strings.ToLower(txHash), logIndex)
}

func claimSyncLease(token string) (*entity.ChainEventSyncState, error) {
	result := db.Mysql().Model(&entity.ChainEventSyncState{}).Where("id = 1").Update("lease_token", token)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, errors.New("chain event sync state is missing; run migration before job")
	}
	state := entity.ChainEventSyncState{}
	if err := db.Mysql().Where("id = 1 AND lease_token = ?", token).Take(&state).Error; err != nil {
		return nil, err
	}
	return &state, nil
}

func determineInitialCursor(ctx context.Context, safeBlock uint64) (int, string, error) {
	v2Cursor := -1
	if chain.V2Enabled() {
		v2Cursor = initialCursorForStartBlock(chain.V2StartBlock())
		if uint64(v2Cursor) > safeBlock {
			return 0, "", fmt.Errorf("configured V2 startBlock=%d is beyond current safe block=%d", chain.V2StartBlock(), safeBlock)
		}
	}
	if chain.V1Enabled() && strings.TrimSpace(viper.GetString("chain.project.startBlock")) != "" {
		startBlock := viper.GetUint64("chain.project.startBlock")
		cursor := initialCursorForStartBlock(startBlock)
		if uint64(cursor) > safeBlock {
			return 0, "", fmt.Errorf("configured startBlock=%d is beyond current safe block=%d", startBlock, safeBlock)
		}
		if v2Cursor >= 0 && v2Cursor < cursor {
			return v2Cursor, fmt.Sprintf("configured V2 startBlock=%d", chain.V2StartBlock()), nil
		}
		return cursor, fmt.Sprintf("configured startBlock=%d", startBlock), nil
	}
	var maxProcessed sql.NullInt64
	if err := db.Mysql().Model(&entity.ChainEventRecord{}).Select("MAX(block_number)").Scan(&maxProcessed).Error; err != nil {
		return 0, "", fmt.Errorf("query processed event checkpoint: %w", err)
	}
	if maxProcessed.Valid && maxProcessed.Int64 >= 0 {
		cursor := int(maxProcessed.Int64)
		if v2Cursor >= 0 && v2Cursor < cursor {
			return v2Cursor, fmt.Sprintf("configured V2 startBlock=%d", chain.V2StartBlock()), nil
		}
		return cursor, "max processed event block", nil
	}
	cached, found, err := db.Redis().GetOptional(ctx, constant.KEY_BLOCK_NUMBER)
	if err != nil {
		return 0, "", fmt.Errorf("read legacy Redis cursor: %w", err)
	}
	if found {
		cursor, parseErr := strconv.Atoi(cached)
		if parseErr != nil || cursor < 0 {
			return 0, "", fmt.Errorf("invalid legacy Redis cursor %q", cached)
		}
		if v2Cursor >= 0 && v2Cursor < cursor {
			return v2Cursor, fmt.Sprintf("configured V2 startBlock=%d", chain.V2StartBlock()), nil
		}
		return cursor, "legacy Redis cursor", nil
	}
	if v2Cursor >= 0 {
		return v2Cursor, fmt.Sprintf("configured V2 startBlock=%d", chain.V2StartBlock()), nil
	}
	return int(safeBlock), "current safe block", nil
}

func registerConfiguredV2(ctx context.Context, token string, state *entity.ChainEventSyncState, safeBlock uint64) (*entity.ChainEventSyncState, error) {
	if state == nil {
		return nil, errors.New("chain event sync state is nil")
	}
	if !chain.V2Enabled() {
		if state.V2Registered {
			return nil, errors.New("V2 event source was registered but V2 configuration is now missing")
		}
		return state, nil
	}
	configuredAddress := chain.V2ContractAddress().Hex()
	configuredStart := chain.V2StartBlock()
	configuredImplementation := chain.V2ImplementationAddress().Hex()
	configuredCodeHash := chain.V2ImplementationCodeHash().Hex()
	targetCursor, shouldRewind, err := v2RegistrationTarget(state, configuredAddress, configuredStart, safeBlock)
	if err != nil {
		return nil, err
	}
	if state.V2Registered {
		if err := validateRegisteredV2Identity(state, configuredAddress, configuredStart, configuredImplementation, configuredCodeHash); err != nil {
			return nil, err
		}
		return state, nil
	}
	if err := requireInitialV2Governance(chain.V2PaymentTokenAllowed(constant.PaymentTokenUSDT), chain.V2PaymentTokenAllowed(constant.PaymentTokenUSDC)); err != nil {
		return nil, err
	}
	updates := map[string]interface{}{
		"v2_registered":               true,
		"v2_contract_address":         configuredAddress,
		"v2_start_block":              configuredStart,
		"v2_implementation_address":   configuredImplementation,
		"v2_implementation_code_hash": configuredCodeHash,
		"updated_at":                  time.Now(),
	}
	if shouldRewind {
		cursorHash, err := cursorBlockHash(ctx, targetCursor)
		if err != nil {
			return nil, err
		}
		updates["cursor_block"] = targetCursor
		updates["cursor_block_hash"] = cursorHash
		updates["lag_blocks"] = blockLag(targetCursor, safeBlock)
		updates["last_error"] = ""
	}
	if err := updateFencedSyncState(token, updates); err != nil {
		return nil, fmt.Errorf("register V2 event source: %w", err)
	}
	refreshed := entity.ChainEventSyncState{}
	if err := db.Mysql().Where("id = 1 AND lease_token = ?", token).Take(&refreshed).Error; err != nil {
		return nil, err
	}
	return &refreshed, nil
}

func requireInitialV2Governance(usdtAllowed, usdcAllowed bool) error {
	if !usdtAllowed || !usdcAllowed {
		return fmt.Errorf("initial V2 event source registration requires governed USDT and USDC: usdt_allowed=%t usdc_allowed=%t", usdtAllowed, usdcAllowed)
	}
	return nil
}

func validateRegisteredV2Identity(state *entity.ChainEventSyncState, address string, start uint64, implementation, codeHash string) error {
	if state == nil || !state.V2Registered {
		return errors.New("V2 event source is not registered")
	}
	if !strings.EqualFold(state.V2ContractAddress, address) || state.V2StartBlock != start || !strings.EqualFold(state.V2ImplementationAddress, implementation) || !strings.EqualFold(state.V2ImplementationCodeHash, codeHash) {
		return errors.New("V2 event source configuration or implementation drift")
	}
	return nil
}

func v2RegistrationTarget(state *entity.ChainEventSyncState, configuredAddress string, configuredStart, safeBlock uint64) (uint64, bool, error) {
	if state == nil {
		return 0, false, errors.New("chain event sync state is nil")
	}
	if !common.IsHexAddress(configuredAddress) || common.HexToAddress(configuredAddress) == (common.Address{}) {
		return 0, false, errors.New("configured V2 address is invalid")
	}
	if configuredStart == 0 {
		return 0, false, errors.New("configured V2 startBlock must be positive")
	}
	if state.V2Registered {
		if !strings.EqualFold(state.V2ContractAddress, common.HexToAddress(configuredAddress).Hex()) || state.V2StartBlock != configuredStart {
			return 0, false, errors.New("V2 event source configuration drift")
		}
		return state.CursorBlock, false, nil
	}
	target := uint64(initialCursorForStartBlock(configuredStart))
	if target > safeBlock {
		return 0, false, fmt.Errorf("V2 startBlock=%d is beyond current safe block=%d", configuredStart, safeBlock)
	}
	return target, state.Initialized && state.CursorBlock > target, nil
}

func initializeSyncState(token string, cursor, latest, safe uint64, cursorHash string) error {
	now := time.Now()
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, token); err != nil {
			return err
		}
		if err := persistCursorCheckpointTx(tx, cursor, cursorHash); err != nil {
			return err
		}
		return updateFencedSyncStateTx(tx, token, map[string]interface{}{
			"initialized":       true,
			"cursor_block":      cursor,
			"cursor_block_hash": cursorHash,
			"latest_block":      latest,
			"safe_block":        safe,
			"lag_blocks":        blockLag(cursor, safe),
			"last_started_at":   now,
			"last_error":        "",
			"updated_at":        now,
		})
	})
}

func updateSyncStarted(token string, cursor, latest, safe uint64, cursorHash string) error {
	now := time.Now()
	return updateFencedSyncState(token, map[string]interface{}{
		"cursor_block":      cursor,
		"cursor_block_hash": cursorHash,
		"latest_block":      latest,
		"safe_block":        safe,
		"lag_blocks":        blockLag(cursor, safe),
		"last_started_at":   now,
		"updated_at":        now,
	})
}

func updateSyncSuccess(token string, cursor, latest, safe uint64, cursorHash string) error {
	return db.Mysql().Transaction(func(tx *gorm.DB) error {
		if err := lockChainEventSyncFenceTx(tx, token); err != nil {
			return err
		}
		return updateSyncSuccessTx(tx, token, cursor, latest, safe, cursorHash)
	})
}

func updateSyncSuccessTx(tx *gorm.DB, token string, cursor, latest, safe uint64, cursorHash string) error {
	now := time.Now()
	if err := persistCursorCheckpointTx(tx, cursor, cursorHash); err != nil {
		return err
	}
	return updateFencedSyncStateTx(tx, token, map[string]interface{}{
		"cursor_block":      cursor,
		"cursor_block_hash": cursorHash,
		"latest_block":      latest,
		"safe_block":        safe,
		"lag_blocks":        blockLag(cursor, safe),
		"last_success_at":   now,
		"last_error":        "",
		"updated_at":        now,
	})
}

func updateSyncFailure(token string, cursor, latest, safe uint64, processErr error) error {
	now := time.Now()
	return updateFencedSyncState(token, map[string]interface{}{
		"cursor_block":    cursor,
		"latest_block":    latest,
		"safe_block":      safe,
		"lag_blocks":      blockLag(cursor, safe),
		"last_failure_at": now,
		"last_error":      processErr.Error(),
		"updated_at":      now,
	})
}

func updateFencedSyncState(token string, updates map[string]interface{}) error {
	return updateFencedSyncStateTx(db.Mysql(), token, updates)
}

func updateFencedSyncStateTx(tx *gorm.DB, token string, updates map[string]interface{}) error {
	result := tx.Model(&entity.ChainEventSyncState{}).
		Where("id = 1 AND lease_token = ?", token).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("chain event checkpoint fencing rejected stale lease")
	}
	return nil
}

func blockLag(cursor, safe uint64) uint64 {
	if safe <= cursor {
		return 0
	}
	return safe - cursor
}

func verifyCursorCanonicality(ctx context.Context, state *entity.ChainEventSyncState) error {
	if state == nil || !state.Initialized {
		return nil
	}
	cursor := state.CursorBlock
	if cursor == 0 {
		return nil
	}
	if state.CursorBlockHash == "" {
		return nil
	}
	header, err := chain.GetClient().Client.HeaderByNumber(ctx, new(big.Int).SetUint64(cursor))
	if err != nil {
		return fmt.Errorf("query cursor block header: %w", err)
	}
	if !strings.EqualFold(header.Hash().Hex(), state.CursorBlockHash) {
		return &cursorCanonicalityError{BlockNumber: cursor, Expected: strings.ToLower(state.CursorBlockHash), Observed: strings.ToLower(header.Hash().Hex())}
	}
	return nil
}

func cursorBlockHash(ctx context.Context, cursor uint64) (string, error) {
	if cursor == 0 {
		return "", nil
	}
	header, err := chain.GetClient().Client.HeaderByNumber(ctx, new(big.Int).SetUint64(cursor))
	if err != nil {
		return "", fmt.Errorf("query cursor block hash: %w", err)
	}
	return header.Hash().Hex(), nil
}

func revalidateScannedLogs(ctx context.Context, logs []types.Log, boundary uint64, boundaryHash string) error {
	expectedByBlock := make(map[uint64]string)
	for _, logEntry := range logs {
		if logEntry.Removed || logEntry.BlockNumber == 0 {
			continue
		}
		hash := logEntry.BlockHash.Hex()
		if existing, ok := expectedByBlock[logEntry.BlockNumber]; ok && existing != hash {
			return fmt.Errorf("provider returned multiple block hashes for block %d", logEntry.BlockNumber)
		}
		expectedByBlock[logEntry.BlockNumber] = hash
	}
	if boundary > 0 && boundaryHash != "" {
		expectedByBlock[boundary] = boundaryHash
	}
	for blockNumber, expectedHash := range expectedByBlock {
		header, err := chain.GetClient().Client.HeaderByNumber(ctx, new(big.Int).SetUint64(blockNumber))
		if err != nil {
			return fmt.Errorf("query canonical block %d: %w", blockNumber, err)
		}
		if header.Hash().Hex() != expectedHash {
			return fmt.Errorf(
				"scanned block hash mismatch at block %d: log=%s canonical=%s; reconciliation required",
				blockNumber, expectedHash, header.Hash().Hex(),
			)
		}
	}
	return nil
}

func revalidateCommittedCursorTx(ctx context.Context, tx *gorm.DB, expectedBlock uint64) error {
	state := entity.ChainEventSyncState{}
	if err := tx.Select("cursor_block,cursor_block_hash").Where("id = 1").Take(&state).Error; err != nil {
		return fmt.Errorf("read committed cursor identity: %w", err)
	}
	if state.CursorBlock != expectedBlock || state.CursorBlockHash == "" {
		if expectedBlock == 0 && state.CursorBlock == 0 && state.CursorBlockHash == "" {
			return nil
		}
		return fmt.Errorf("committed cursor changed during batch: got=%d expected=%d", state.CursorBlock, expectedBlock)
	}
	header, err := chain.GetClient().Client.HeaderByNumber(ctx, new(big.Int).SetUint64(expectedBlock))
	if err != nil {
		return fmt.Errorf("re-read committed cursor block %d: %w", expectedBlock, err)
	}
	if !strings.EqualFold(header.Hash().Hex(), state.CursorBlockHash) {
		return &cursorCanonicalityError{BlockNumber: expectedBlock, Expected: strings.ToLower(state.CursorBlockHash), Observed: strings.ToLower(header.Hash().Hex())}
	}
	return nil
}

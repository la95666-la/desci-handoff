package chaintx

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

var ErrSignerBusy = errors.New("relayer signer has an active transaction")
var ErrNonceDrift = errors.New("relayer nonce differs from the audited database cursor")

type Intent struct {
	ProjectID       uint64
	Action          string
	ChainID         uint64
	SenderAddress   common.Address
	ContractAddress common.Address
	CallData        []byte
	// Global marks a not-project-scoped governance action (global pause/unpause).
	// Such intents persist with project_id = 0; the schema check allows zero only
	// in combination with the whitelisted global actions.
	Global bool
}

func validateIntentShape(intent Intent) error {
	if (!intent.Global && intent.ProjectID == 0) || (intent.Global && intent.ProjectID != 0) || intent.Action == "" || intent.ChainID == 0 || intent.SenderAddress == (common.Address{}) || intent.ContractAddress == (common.Address{}) || len(intent.CallData) == 0 {
		return errors.New("invalid chain transaction intent")
	}
	return nil
}

func CreateIntentTx(tx *gorm.DB, intent Intent) (*entity.ChainTxSubmission, error) {
	if err := rejectFairLaunchTransactionAction(intent.Action); err != nil {
		return nil, err
	}
	return insertIntentTx(tx, intent)
}

// The typed V3 reservation is the only other caller. Generic outbox entry
// points still reject every V3 action before reaching this storage primitive.
func insertIntentTx(tx *gorm.DB, intent Intent) (*entity.ChainTxSubmission, error) {
	if err := validateIntentShape(intent); err != nil {
		return nil, err
	}
	// V3 binding and submission triggers compare the proxy byte-for-byte.
	// Persist its canonical lower-case form without changing historical V2 rows.
	contractAddress := intent.ContractAddress.Hex()
	if IsIDOV3TransactionAction(intent.Action) {
		if basechain.VerifyV3ActionCall(intent.Action, intent.ProjectID, intent.CallData) != nil {
			return nil, ErrIDOV3MaterializationUnavailable
		}
		contractAddress = strings.ToLower(contractAddress)
	}
	existing := entity.ChainTxSubmission{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"chain_id = ? AND contract_address = ? AND project_id = ? AND action = ? AND active_slot = 1",
		intent.ChainID, contractAddress, intent.ProjectID, intent.Action,
	).Take(&existing).Error
	if err == nil {
		if !strings.EqualFold(existing.SenderAddress, intent.SenderAddress.Hex()) || !bytes.Equal(existing.CallData, intent.CallData) {
			return nil, errors.New("active chain transaction intent does not match the requested call")
		}
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	var attempt uint32
	if err := tx.Model(&entity.ChainTxSubmission{}).Where(
		"chain_id = ? AND contract_address = ? AND project_id = ? AND action = ?",
		intent.ChainID, contractAddress, intent.ProjectID, intent.Action,
	).Select("COALESCE(MAX(attempt_no), 0)").Scan(&attempt).Error; err != nil {
		return nil, err
	}
	attempt++
	active := uint8(1)
	record := entity.ChainTxSubmission{
		ProjectID: intent.ProjectID, Action: intent.Action, AttemptNo: attempt,
		ChainID: intent.ChainID, SenderAddress: intent.SenderAddress.Hex(), ContractAddress: contractAddress,
		CallData: append([]byte(nil), intent.CallData...), ValueRaw: "0", State: constant.ChainTxStateIntent,
		GasPriceRaw: "0", ActiveSlot: &active, LastError: "", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := tx.Create(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func CancelIntentTx(tx *gorm.DB, id uint64) error {
	record := entity.ChainTxSubmission{}
	if err := tx.Select("id,action").Where("id = ?", id).Take(&record).Error; err != nil {
		return err
	}
	if err := rejectFairLaunchTransactionAction(record.Action); err != nil {
		return err
	}
	now := time.Now()
	result := tx.Model(&entity.ChainTxSubmission{}).Where(
		"id = ? AND state = ? AND nonce IS NULL AND tx_hash IS NULL AND raw_tx IS NULL",
		id, constant.ChainTxStateIntent,
	).Updates(map[string]interface{}{"state": constant.ChainTxStateCancelled, "active_slot": nil, "finalized_at": now, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("chain transaction intent is no longer cancellable")
	}
	return nil
}

// CancelUnsignedTx is the explicit operator cancellation path. Unlike the
// automatic preflight cleanup above, an audited administrator may also cancel
// a ready transaction, but never one that has allocated a nonce or signature.
func CancelUnsignedTx(tx *gorm.DB, id, adminID uint64, reason string) error {
	reason = strings.TrimSpace(reason)
	reasonLength := utf8.RuneCountInString(reason)
	if adminID == 0 || reasonLength < 3 || reasonLength > 500 {
		return errors.New("authorized administrator and 3-500 character cancellation reason are required")
	}
	record := entity.ChainTxSubmission{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&record).Error; err != nil {
		return err
	}
	if err := rejectFairLaunchTransactionAction(record.Action); err != nil {
		return err
	}
	if !AdminCancellable(record) {
		return errors.New("chain transaction is not an unsigned cancellable intent")
	}
	var signerReferences int64
	if err := tx.Model(&entity.ChainRelayerNonce{}).Where("active_submission_id = ?", id).Count(&signerReferences).Error; err != nil {
		return err
	}
	if signerReferences != 0 {
		return errors.New("unsigned intent is unexpectedly referenced by the relayer nonce fence")
	}
	now := time.Now()
	result := tx.Model(&entity.ChainTxSubmission{}).Where(
		"id = ? AND state IN ? AND active_slot = 1 AND nonce IS NULL AND gas_limit IS NULL AND raw_tx IS NULL AND tx_hash IS NULL",
		id, []string{constant.ChainTxStateIntent, constant.ChainTxStateReady},
	).Updates(map[string]interface{}{
		"state": constant.ChainTxStateAdminCancelled, "active_slot": nil,
		"last_error":            "cancelled by an authorized administrator",
		"cancelled_by_admin_id": adminID, "cancel_reason": reason, "cancelled_at": now,
		"finalized_at": now, "updated_at": now,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("chain transaction changed before administrator cancellation")
	}
	return nil
}

func AdminCancellable(record entity.ChainTxSubmission) bool {
	if record.State != constant.ChainTxStateIntent && record.State != constant.ChainTxStateReady {
		return false
	}
	return record.ActiveSlot != nil && *record.ActiveSlot == 1 && record.Nonce == nil && record.GasLimit == nil && len(record.RawTx) == 0 && record.TxHash == nil
}

func MarkReadyTx(tx *gorm.DB, id uint64) error {
	record := entity.ChainTxSubmission{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&record).Error; err != nil {
		return err
	}
	if err := rejectFairLaunchTransactionAction(record.Action); err != nil {
		return err
	}
	if record.State == constant.ChainTxStateReady || record.State == constant.ChainTxStateSigned || record.State == constant.ChainTxStateSubmitted || record.State == constant.ChainTxStateBroadcastUnknown || record.State == constant.ChainTxStateMinedSuccess || record.State == constant.ChainTxStateFinalSuccess || record.State == constant.ChainTxStateApplied {
		return nil
	}
	if record.State != constant.ChainTxStateIntent || record.Nonce != nil || record.TxHash != nil || len(record.RawTx) != 0 {
		return errors.New("chain transaction intent cannot transition to ready")
	}
	result := tx.Model(&entity.ChainTxSubmission{}).Where("id = ? AND state = ?", id, constant.ChainTxStateIntent).Updates(map[string]interface{}{"state": constant.ChainTxStateReady, "updated_at": time.Now()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("chain transaction intent changed before ready transition")
	}
	return nil
}

func Prepare(ctx context.Context, database *gorm.DB, client *basechain.Client, submissionID uint64) (*entity.ChainTxSubmission, error) {
	intent := entity.ChainTxSubmission{}
	if err := database.Where("id = ?", submissionID).Take(&intent).Error; err != nil {
		return nil, err
	}
	if err := rejectFairLaunchTransactionAction(intent.Action); err != nil {
		return nil, err
	}
	if intent.State != constant.ChainTxStateReady {
		return &intent, nil
	}
	if err := validateIntentClient(intent, client); err != nil {
		return nil, err
	}
	to := common.HexToAddress(intent.ContractAddress)
	gasLimit, gasPrice, err := client.EstimateLegacyTransaction(ctx, to, intent.CallData)
	if err != nil {
		return nil, err
	}
	latestNonce, err := client.Client.NonceAt(ctx, client.Address, nil)
	if err != nil {
		return nil, fmt.Errorf("query latest relayer nonce: %w", err)
	}
	pendingNonce, err := client.Client.PendingNonceAt(ctx, client.Address)
	if err != nil {
		return nil, fmt.Errorf("query pending relayer nonce: %w", err)
	}

	prepared := entity.ChainTxSubmission{}
	err = database.Transaction(func(tx *gorm.DB) error {
		if err := chainreorg.LockNoActiveIncidentTx(tx, intent.ChainID, intent.ContractAddress); err != nil {
			return err
		}
		row := entity.ChainTxSubmission{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", submissionID).Take(&row).Error; err != nil {
			return err
		}
		if row.State != constant.ChainTxStateReady {
			prepared = row
			return nil
		}
		if err := validateIntentClient(row, client); err != nil {
			return err
		}
		nonceState := entity.ChainRelayerNonce{}
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("chain_id = ? AND sender_address = ?", row.ChainID, client.Address.Hex()).Take(&nonceState).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if latestNonce != pendingNonce {
				return fmt.Errorf("%w: bootstrap latest=%d pending=%d", ErrNonceDrift, latestNonce, pendingNonce)
			}
			nonceState = entity.ChainRelayerNonce{ChainID: row.ChainID, SenderAddress: client.Address.Hex(), NextNonce: pendingNonce, CreatedAt: time.Now(), UpdatedAt: time.Now()}
			if err := tx.Create(&nonceState).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if nonceState.ActiveSubmissionID != nil && *nonceState.ActiveSubmissionID != row.ID {
			return ErrSignerBusy
		}
		if nonceState.NextNonce != pendingNonce || latestNonce > pendingNonce {
			return fmt.Errorf("%w: db=%d latest=%d pending=%d", ErrNonceDrift, nonceState.NextNonce, latestNonce, pendingNonce)
		}
		raw, hash, err := client.SignLegacyTransaction(nonceState.NextNonce, gasLimit, gasPrice, to, row.CallData)
		if err != nil {
			return err
		}
		if err := validateRaw(row, raw, hash, nonceState.NextNonce); err != nil {
			return err
		}
		nonce := nonceState.NextNonce
		now := time.Now()
		result := tx.Model(&entity.ChainTxSubmission{}).Where("id = ? AND state = ?", row.ID, constant.ChainTxStateReady).Updates(map[string]interface{}{
			"state": constant.ChainTxStateSigned, "nonce": nonce, "gas_limit": gasLimit, "gas_price_raw": gasPrice.String(),
			"raw_tx": raw, "tx_hash": common.HexToHash(hash).Hex(), "updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("chain transaction intent changed before signing")
		}
		result = tx.Model(&entity.ChainRelayerNonce{}).Where("chain_id = ? AND sender_address = ? AND active_submission_id IS NULL", row.ChainID, client.Address.Hex()).Updates(map[string]interface{}{
			"next_nonce": nonce + 1, "active_submission_id": row.ID, "updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrSignerBusy
		}
		return tx.Where("id = ?", row.ID).Take(&prepared).Error
	})
	if err != nil {
		return nil, err
	}
	return &prepared, nil
}

func Dispatch(ctx context.Context, database *gorm.DB, client *basechain.Client, submissionID uint64) (*entity.ChainTxSubmission, error) {
	record := entity.ChainTxSubmission{}
	if err := database.Where("id = ?", submissionID).Take(&record).Error; err != nil {
		return nil, err
	}
	if err := rejectFairLaunchTransactionAction(record.Action); err != nil {
		return nil, err
	}
	if err := validateIntentClient(record, client); err != nil {
		return nil, err
	}
	if record.State == constant.ChainTxStateIntent {
		return nil, errors.New("chain transaction intent has not passed external preflight")
	}
	if record.State == constant.ChainTxStateReady {
		prepared, err := Prepare(ctx, database, client, submissionID)
		if err != nil {
			return nil, err
		}
		record = *prepared
	}
	if record.State != constant.ChainTxStateSigned && record.State != constant.ChainTxStateSubmitted && record.State != constant.ChainTxStateBroadcastUnknown {
		return &record, nil
	}
	if record.TxHash == nil || record.Nonce == nil || len(record.RawTx) == 0 {
		return nil, errors.New("signed submission is missing hash or raw transaction")
	}
	if err := validateRaw(record, record.RawTx, *record.TxHash, *record.Nonce); err != nil {
		return nil, err
	}
	var broadcastErr error
	err := database.Transaction(func(tx *gorm.DB) error {
		if err := chainreorg.LockNoActiveIncidentTx(tx, record.ChainID, record.ContractAddress); err != nil {
			return err
		}
		current := entity.ChainTxSubmission{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", record.ID).Take(&current).Error; err != nil {
			return err
		}
		if current.State != constant.ChainTxStateSigned && current.State != constant.ChainTxStateSubmitted && current.State != constant.ChainTxStateBroadcastUnknown {
			record = current
			return nil
		}
		if current.TxHash == nil || current.Nonce == nil || len(current.RawTx) == 0 {
			return errors.New("signed submission is missing hash or raw transaction")
		}
		if err := validateRaw(current, current.RawTx, *current.TxHash, *current.Nonce); err != nil {
			return err
		}
		hash, sendErr := client.BroadcastRawTransaction(ctx, current.RawTx)
		if !strings.EqualFold(hash, *current.TxHash) {
			return errors.New("broadcast transaction hash differs from persisted hash")
		}
		broadcastErr = sendErr
		now := time.Now()
		state := constant.ChainTxStateSubmitted
		lastError := ""
		if sendErr != nil {
			state, lastError = constant.ChainTxStateBroadcastUnknown, truncateError(sendErr.Error())
		}
		result := tx.Model(&entity.ChainTxSubmission{}).Where("id = ? AND state IN ?", current.ID, []string{constant.ChainTxStateSigned, constant.ChainTxStateSubmitted, constant.ChainTxStateBroadcastUnknown}).Updates(map[string]interface{}{
			"state": state, "broadcast_count": gorm.Expr("broadcast_count + 1"), "last_broadcast_at": now, "last_error": lastError, "updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("submission changed while recording broadcast")
		}
		return tx.Where("id = ?", current.ID).Take(&record).Error
	})
	if err != nil {
		return nil, err
	}
	if err := database.Where("id = ?", record.ID).Take(&record).Error; err != nil {
		return nil, err
	}
	return &record, broadcastErr
}

func ReconcileReceipt(ctx context.Context, database *gorm.DB, client *basechain.Client, submissionID, confirmations uint64) (*entity.ChainTxSubmission, error) {
	record := entity.ChainTxSubmission{}
	if err := database.Where("id = ?", submissionID).Take(&record).Error; err != nil {
		return nil, err
	}
	if err := rejectFairLaunchTransactionAction(record.Action); err != nil {
		return nil, err
	}
	if err := validateIntentClient(record, client); err != nil {
		return nil, err
	}
	if record.TxHash == nil {
		return &record, nil
	}
	if record.State == constant.ChainTxStateApplied || record.State == constant.ChainTxStateFinalSuccess || record.State == constant.ChainTxStateFinalRevert || record.State == constant.ChainTxStateAmbiguous || record.State == constant.ChainTxStateCancelled || record.State == constant.ChainTxStateAdminCancelled {
		return &record, nil
	}
	receipt, err := client.Client.TransactionReceipt(ctx, common.HexToHash(*record.TxHash))
	if errors.Is(err, ethereum.NotFound) {
		if record.State == constant.ChainTxStateMinedSuccess || record.State == constant.ChainTxStateMinedRevert {
			if updateErr := updateObservedState(database, record.ID, constant.ChainTxStateBroadcastUnknown, nil, "receipt disappeared before finality"); updateErr != nil {
				return nil, updateErr
			}
		}
		if record.Nonce != nil {
			confirmedNonce, nonceErr := client.Client.NonceAt(ctx, client.Address, nil)
			if nonceErr != nil {
				return nil, fmt.Errorf("query confirmed relayer nonce: %w", nonceErr)
			}
			if confirmedNonce > *record.Nonce {
				if updateErr := updateObservedState(database, record.ID, constant.ChainTxStateAmbiguous, nil, fmt.Sprintf("confirmed relayer nonce %d consumed expected nonce %d without the persisted receipt", confirmedNonce, *record.Nonce)); updateErr != nil {
					return nil, updateErr
				}
				return load(database, record.ID)
			}
		}
		if record.BroadcastCount >= 5 && time.Since(record.CreatedAt) >= 3*time.Minute {
			if updateErr := updateObservedState(database, record.ID, constant.ChainTxStateAmbiguous, nil, "receipt remained unavailable after bounded exact-raw rebroadcasts"); updateErr != nil {
				return nil, updateErr
			}
			return load(database, record.ID)
		}
		return &record, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query transaction receipt: %w", err)
	}
	if receipt == nil || receipt.TxHash != common.HexToHash(*record.TxHash) {
		return nil, errors.New("RPC returned an invalid transaction receipt")
	}
	block := receipt.BlockNumber.Uint64()
	head, err := client.Client.BlockNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("query latest block for receipt: %w", err)
	}
	state := constant.ChainTxStateMinedSuccess
	if receipt.Status == types.ReceiptStatusFailed {
		state = constant.ChainTxStateMinedRevert
	}
	if err := updateReceipt(database, record.ID, state, receipt, nil); err != nil {
		return nil, err
	}
	if head < block || head-block < confirmations {
		return load(database, record.ID)
	}
	header, err := client.Client.HeaderByNumber(ctx, new(big.Int).SetUint64(block))
	if err != nil {
		return nil, fmt.Errorf("query canonical receipt block: %w", err)
	}
	if header.Hash() != receipt.BlockHash {
		if err := updateObservedState(database, record.ID, constant.ChainTxStateBroadcastUnknown, nil, "receipt block is no longer canonical"); err != nil {
			return nil, err
		}
		return load(database, record.ID)
	}
	confirmed, err := client.Client.TransactionReceipt(ctx, common.HexToHash(*record.TxHash))
	if err != nil || confirmed == nil || confirmed.BlockHash != receipt.BlockHash || confirmed.Status != receipt.Status {
		if err := updateObservedState(database, record.ID, constant.ChainTxStateBroadcastUnknown, nil, "receipt changed during canonical confirmation"); err != nil {
			return nil, err
		}
		return load(database, record.ID)
	}
	finalState := constant.ChainTxStateFinalSuccess
	if receipt.Status == types.ReceiptStatusFailed {
		finalState = constant.ChainTxStateFinalRevert
	}
	if err := finalizeReceipt(database, record, finalState, receipt); err != nil {
		return nil, err
	}
	return load(database, record.ID)
}

func RequireEventSubmissionTx(tx *gorm.DB, projectID uint64, action, txHash string, chainID uint64, contractAddress, senderAddress common.Address) (*entity.ChainTxSubmission, error) {
	if err := rejectFairLaunchTransactionAction(action); err != nil {
		return nil, err
	}
	if !validHash(txHash) {
		return nil, errors.New("lifecycle event transaction hash is invalid")
	}
	record := entity.ChainTxSubmission{}
	if chainID == 0 || contractAddress == (common.Address{}) || senderAddress == (common.Address{}) {
		return nil, errors.New("lifecycle event source identity is invalid")
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ? AND action = ? AND tx_hash = ? AND chain_id = ? AND contract_address = ? AND sender_address = ?", projectID, action, common.HexToHash(txHash).Hex(), chainID, contractAddress.Hex(), senderAddress.Hex()).Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("matching %s durable submission was not found", action)
		}
		return nil, err
	}
	allowed := map[string]bool{constant.ChainTxStateSigned: true, constant.ChainTxStateSubmitted: true, constant.ChainTxStateBroadcastUnknown: true, constant.ChainTxStateMinedSuccess: true, constant.ChainTxStateFinalSuccess: true, constant.ChainTxStateAmbiguous: true, constant.ChainTxStateApplied: true}
	if !allowed[record.State] {
		return nil, fmt.Errorf("matching %s submission state %s cannot be applied", action, record.State)
	}
	return &record, nil
}

// FindEventSubmissionTx returns an exact durable submission when one exists.
// It is used by legacy permissionless lifecycle events where external callers
// remain valid and absence of a relayer row must not block canonical state.
func FindEventSubmissionTx(tx *gorm.DB, projectID uint64, action, txHash string) (*entity.ChainTxSubmission, error) {
	if err := rejectFairLaunchTransactionAction(action); err != nil {
		return nil, err
	}
	if !validHash(txHash) {
		return nil, errors.New("lifecycle event transaction hash is invalid")
	}
	record := entity.ChainTxSubmission{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id = ? AND action = ? AND tx_hash = ?", projectID, action, common.HexToHash(txHash).Hex()).Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// FindGlobalEventSubmissionTx returns the durable submission of a not-project-scoped
// governance action (global pause / unpause) identified by its canonical transaction
// hash. Absence is not an error: the Paused/Unpaused event may equally originate from
// an owner-controlled direct on-chain call, in which case there is nothing to advance.
func FindGlobalEventSubmissionTx(tx *gorm.DB, action, txHash string, chainID uint64, contractAddress, senderAddress common.Address) (*entity.ChainTxSubmission, error) {
	if err := rejectFairLaunchTransactionAction(action); err != nil {
		return nil, err
	}
	if !validHash(txHash) {
		return nil, errors.New("governance event transaction hash is invalid")
	}
	if chainID == 0 || contractAddress == (common.Address{}) || senderAddress == (common.Address{}) {
		return nil, errors.New("governance event source identity is invalid")
	}
	record := entity.ChainTxSubmission{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"project_id = 0 AND action = ? AND tx_hash = ? AND chain_id = ? AND contract_address = ? AND sender_address = ?",
		action, common.HexToHash(txHash).Hex(), chainID, contractAddress.Hex(), senderAddress.Hex(),
	).Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func CancelUnsignedBusinessTx(tx *gorm.DB, projectID uint64, action string) error {
	if err := rejectFairLaunchTransactionAction(action); err != nil {
		return err
	}
	now := time.Now()
	result := tx.Model(&entity.ChainTxSubmission{}).Where(
		"project_id = ? AND action = ? AND active_slot = 1 AND state IN ? AND nonce IS NULL AND raw_tx IS NULL AND tx_hash IS NULL",
		projectID, action, []string{constant.ChainTxStateIntent, constant.ChainTxStateReady},
	).Updates(map[string]interface{}{"state": constant.ChainTxStateCancelled, "active_slot": nil, "last_error": "superseded by a canonical external lifecycle event", "finalized_at": now, "updated_at": now})
	return result.Error
}

func validHash(value string) bool {
	if len(value) != 66 || !strings.HasPrefix(value, "0x") || common.HexToHash(value) == (common.Hash{}) {
		return false
	}
	_, err := hex.DecodeString(value[2:])
	return err == nil
}

func MarkAppliedTx(tx *gorm.DB, submissionID uint64, eventRecordID uint64) error {
	now := time.Now()
	record := entity.ChainTxSubmission{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", submissionID).Take(&record).Error; err != nil {
		return err
	}
	if err := rejectFairLaunchTransactionAction(record.Action); err != nil {
		return err
	}
	if record.State == constant.ChainTxStateApplied {
		return nil
	}
	result := tx.Model(&entity.ChainTxSubmission{}).Where("id = ? AND state IN ?", submissionID, []string{constant.ChainTxStateSigned, constant.ChainTxStateSubmitted, constant.ChainTxStateBroadcastUnknown, constant.ChainTxStateMinedSuccess, constant.ChainTxStateFinalSuccess, constant.ChainTxStateAmbiguous}).Updates(map[string]interface{}{
		"state": constant.ChainTxStateApplied, "active_slot": nil, "final_event_record_id": eventRecordID, "finalized_at": now, "updated_at": now,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("submission cannot transition to applied")
	}
	return releaseSignerTx(tx, record, submissionID)
}

func validateIntentClient(row entity.ChainTxSubmission, client *basechain.Client) error {
	if client == nil || client.ChainID == nil || !client.ChainID.IsUint64() {
		return errors.New("chain client is invalid")
	}
	if row.ChainID != client.ChainID.Uint64() || !strings.EqualFold(row.SenderAddress, client.Address.Hex()) {
		return errors.New("submission chain or sender differs from client")
	}
	return nil
}

func validateRaw(row entity.ChainTxSubmission, raw []byte, hash string, nonce uint64) error {
	var tx types.Transaction
	if err := tx.UnmarshalBinary(raw); err != nil {
		return fmt.Errorf("decode signed transaction: %w", err)
	}
	if tx.Hash() != common.HexToHash(hash) || tx.Nonce() != nonce || tx.ChainId().Uint64() != row.ChainID || tx.To() == nil || !strings.EqualFold(tx.To().Hex(), row.ContractAddress) || tx.Value().Sign() != 0 || !bytes.Equal(tx.Data(), row.CallData) {
		return errors.New("persisted signed transaction does not match immutable intent")
	}
	sender, err := types.Sender(types.LatestSignerForChainID(new(big.Int).SetUint64(row.ChainID)), &tx)
	if err != nil || !strings.EqualFold(sender.Hex(), row.SenderAddress) {
		return errors.New("persisted signed transaction sender mismatch")
	}
	return nil
}

func updateReceipt(database *gorm.DB, id uint64, state string, receipt *types.Receipt, finalized *time.Time) error {
	status, block, gas := receipt.Status, receipt.BlockNumber.Uint64(), receipt.GasUsed
	values := map[string]interface{}{"state": state, "receipt_status": status, "receipt_block_number": block, "receipt_block_hash": receipt.BlockHash.Hex(), "receipt_gas_used": gas, "last_error": "", "updated_at": time.Now()}
	if finalized != nil {
		values["finalized_at"] = *finalized
	}
	if state == constant.ChainTxStateFinalRevert {
		values["active_slot"] = nil
	}
	return database.Model(&entity.ChainTxSubmission{}).Where("id = ? AND state NOT IN ?", id, []string{constant.ChainTxStateApplied, constant.ChainTxStateFinalSuccess, constant.ChainTxStateFinalRevert, constant.ChainTxStateAmbiguous, constant.ChainTxStateCancelled, constant.ChainTxStateAdminCancelled}).Updates(values).Error
}

func updateObservedState(database *gorm.DB, id uint64, state string, finalized *time.Time, message string) error {
	values := map[string]interface{}{"state": state, "receipt_status": nil, "receipt_block_number": nil, "receipt_block_hash": "", "receipt_gas_used": nil, "last_error": truncateError(message), "updated_at": time.Now()}
	if finalized != nil {
		values["finalized_at"] = *finalized
	}
	return database.Model(&entity.ChainTxSubmission{}).Where("id = ? AND state NOT IN ?", id, []string{constant.ChainTxStateApplied, constant.ChainTxStateFinalSuccess, constant.ChainTxStateFinalRevert, constant.ChainTxStateAmbiguous, constant.ChainTxStateCancelled, constant.ChainTxStateAdminCancelled}).Updates(values).Error
}

func finalizeReceipt(database *gorm.DB, record entity.ChainTxSubmission, state string, receipt *types.Receipt) error {
	return database.Transaction(func(tx *gorm.DB) error {
		current := entity.ChainTxSubmission{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", record.ID).Take(&current).Error; err != nil {
			return err
		}
		if current.State == constant.ChainTxStateApplied || current.State == constant.ChainTxStateFinalSuccess || current.State == constant.ChainTxStateFinalRevert || current.State == constant.ChainTxStateAmbiguous || current.State == constant.ChainTxStateCancelled || current.State == constant.ChainTxStateAdminCancelled {
			return nil
		}
		now := time.Now()
		if err := updateReceipt(tx, current.ID, state, receipt, &now); err != nil {
			return err
		}
		if state == constant.ChainTxStateFinalRevert {
			return releaseSignerTx(tx, current, current.ID)
		}
		return nil
	})
}

func releaseSignerTx(tx *gorm.DB, record entity.ChainTxSubmission, submissionID uint64) error {
	result := tx.Model(&entity.ChainRelayerNonce{}).Where("chain_id = ? AND sender_address = ? AND active_submission_id = ?", record.ChainID, record.SenderAddress, submissionID).Update("active_submission_id", nil)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("relayer active submission fence mismatch")
	}
	return nil
}

func load(database *gorm.DB, id uint64) (*entity.ChainTxSubmission, error) {
	record := entity.ChainTxSubmission{}
	if err := database.Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func truncateError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 8000 {
		return value
	}
	value = value[:8000]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

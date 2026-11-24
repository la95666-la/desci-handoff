package chaintx

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

const (
	ChainTxStateSigningReserved = "signing_reserved"
	ChainTxStateSigningRecovery = "signing_recovery"
)

// SigningReservation is the immutable request passed to a remote Sign Policy
// Gateway only after ReserveSigningTx commits successfully.
type SigningReservation struct {
	SubmissionID     uint64
	ChainID          uint64
	SenderAddress    string
	Nonce            uint64
	UnsignedTxDigest string
	SigningDigest    string
	ReservedAt       time.Time
}

// ReserveSigningTx atomically takes the sender-scoped fence without advancing
// next_nonce. No Gateway call may occur before this transaction commits.
func ReserveSigningTx(tx *gorm.DB, submissionID uint64, expectedSender common.Address, unsignedTxDigest, signingDigest string, now time.Time) (SigningReservation, error) {
	if submissionID == 0 || expectedSender == (common.Address{}) || !validDigest(unsignedTxDigest) || !validDigest(signingDigest) || now.IsZero() {
		return SigningReservation{}, errors.New("invalid signing reservation")
	}
	row := entity.ChainTxSubmission{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", submissionID).Take(&row).Error; err != nil {
		return SigningReservation{}, err
	}
	if err := rejectFairLaunchTransactionAction(row.Action); err != nil {
		return SigningReservation{}, err
	}
	if row.State != constant.ChainTxStateReady || !strings.EqualFold(row.SenderAddress, expectedSender.Hex()) || row.Nonce != nil || len(row.RawTx) != 0 || row.TxHash != nil {
		return SigningReservation{}, errors.New("submission is not eligible for signing reservation")
	}
	nonceState := entity.ChainRelayerNonce{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("chain_id = ? AND sender_address = ?", row.ChainID, expectedSender.Hex()).Take(&nonceState).Error; err != nil {
		return SigningReservation{}, fmt.Errorf("load sender nonce fence: %w", err)
	}
	if nonceState.ActiveSubmissionID != nil {
		return SigningReservation{}, ErrSignerBusy
	}
	nonce := nonceState.NextNonce
	result := tx.Model(&entity.ChainTxSubmission{}).Where("id = ? AND state = ?", row.ID, constant.ChainTxStateReady).Updates(map[string]interface{}{
		"state": ChainTxStateSigningReserved, "nonce": nonce,
		"unsigned_tx_digest": strings.ToLower(unsignedTxDigest), "signing_digest": strings.ToLower(signingDigest),
		"signing_reserved_at": now, "active_slot": 1, "updated_at": now,
	})
	if result.Error != nil {
		return SigningReservation{}, result.Error
	}
	if result.RowsAffected != 1 {
		return SigningReservation{}, errors.New("submission changed before signing reservation")
	}
	result = tx.Model(&entity.ChainRelayerNonce{}).Where("chain_id = ? AND sender_address = ? AND active_submission_id IS NULL", row.ChainID, expectedSender.Hex()).Updates(map[string]interface{}{"active_submission_id": row.ID, "updated_at": now})
	if result.Error != nil {
		return SigningReservation{}, result.Error
	}
	if result.RowsAffected != 1 {
		return SigningReservation{}, ErrSignerBusy
	}
	return SigningReservation{SubmissionID: row.ID, ChainID: row.ChainID, SenderAddress: expectedSender.Hex(), Nonce: nonce, UnsignedTxDigest: strings.ToLower(unsignedTxDigest), SigningDigest: strings.ToLower(signingDigest), ReservedAt: now}, nil
}

// MarkSigningRecoveryTx preserves the fence when Gateway journal state cannot
// prove that no raw transaction was committed.
func MarkSigningRecoveryTx(tx *gorm.DB, submissionID uint64, reason string, now time.Time) error {
	reason = strings.TrimSpace(reason)
	if submissionID == 0 || reason == "" || now.IsZero() {
		return errors.New("signing recovery reason is required")
	}
	// The action is not supplied by this legacy API, so one minimal read is
	// required before rejecting; no progression mutation occurs first.
	row := entity.ChainTxSubmission{}
	if err := tx.Select("id,action").Where("id = ?", submissionID).Take(&row).Error; err != nil {
		return err
	}
	if err := rejectFairLaunchTransactionAction(row.Action); err != nil {
		return err
	}
	result := tx.Model(&entity.ChainTxSubmission{}).Where("id = ? AND state = ? AND active_slot = 1", submissionID, ChainTxStateSigningReserved).Updates(map[string]interface{}{"state": ChainTxStateSigningRecovery, "signing_recovery_reason": reason, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("submission is not in signing_reserved")
	}
	return nil
}

// CommitGatewayResultTx durably records the exact Gateway generation and only
// then advances next_nonce. A timeout or identity mismatch must instead leave
// the sender fence in signing_recovery.
func CommitGatewayResultTx(tx *gorm.DB, reservation SigningReservation, result GatewayJournalEntry, now time.Time) error {
	if now.IsZero() {
		return errors.New("commit time is required")
	}
	req := GatewayCommitRequest{GatewayJournalKey: GatewayJournalKey{ChainID: reservation.ChainID, ExpectedSignerAddress: reservation.SenderAddress, Nonce: reservation.Nonce}, SubmissionID: reservation.SubmissionID, UnsignedTxDigest: reservation.UnsignedTxDigest, SigningDigest: reservation.SigningDigest}
	if err := ValidateGatewayCommit(result, req); err != nil {
		return err
	}
	row := entity.ChainTxSubmission{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", reservation.SubmissionID).Take(&row).Error; err != nil {
		return err
	}
	if err := rejectFairLaunchTransactionAction(row.Action); err != nil {
		return err
	}
	if row.State != ChainTxStateSigningReserved || row.Nonce == nil || *row.Nonce != reservation.Nonce || !strings.EqualFold(row.SenderAddress, reservation.SenderAddress) || !strings.EqualFold(row.UnsignedTxDigest, reservation.UnsignedTxDigest) || !strings.EqualFold(row.SigningDigest, reservation.SigningDigest) {
		return errors.New("signing reservation changed before gateway commit")
	}
	if len(row.RawTx) != 0 || row.TxHash != nil {
		return errors.New("signing reservation already has committed transaction material")
	}
	nonceState := entity.ChainRelayerNonce{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("chain_id = ? AND sender_address = ?", reservation.ChainID, reservation.SenderAddress).Take(&nonceState).Error; err != nil {
		return err
	}
	if nonceState.ActiveSubmissionID == nil || *nonceState.ActiveSubmissionID != reservation.SubmissionID || nonceState.NextNonce != reservation.Nonce {
		return errors.New("sender nonce fence does not match gateway commitment")
	}
	hash := strings.ToLower(result.TxHash)
	updates := map[string]interface{}{"state": constant.ChainTxStateSigned, "raw_tx": append([]byte(nil), result.RawTransaction...), "tx_hash": hash, "replacement_generation": result.ReplacementGeneration, "updated_at": now}
	res := tx.Model(&entity.ChainTxSubmission{}).Where("id = ? AND state = ? AND raw_tx IS NULL AND tx_hash IS NULL", reservation.SubmissionID, ChainTxStateSigningReserved).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errors.New("submission changed before gateway transaction commitment")
	}
	res = tx.Model(&entity.ChainRelayerNonce{}).Where("chain_id = ? AND sender_address = ? AND next_nonce = ? AND active_submission_id = ?", reservation.ChainID, reservation.SenderAddress, reservation.Nonce, reservation.SubmissionID).Updates(map[string]interface{}{"next_nonce": reservation.Nonce + 1, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errors.New("sender nonce changed before gateway transaction commitment")
	}
	return nil
}

// ReleaseAbsentGatewayReservationTx releases only a journal-proven absent or
// rejected generation. Unknown/committed responses must never clear the fence.
func ReleaseAbsentGatewayReservationTx(tx *gorm.DB, reservation SigningReservation, journal GatewayJournalEntry, now time.Time) error {
	if journal.ChainID != reservation.ChainID || !strings.EqualFold(journal.ExpectedSignerAddress, reservation.SenderAddress) || journal.Nonce != reservation.Nonce {
		return errors.New("gateway journal identity mismatch")
	}
	state, err := ReconcileGatewayJournal(journal)
	if err != nil {
		return err
	}
	if state != "safe_to_release_unsigned_reservation" || now.IsZero() {
		return errors.New("gateway journal does not prove an unsigned reservation")
	}
	row := entity.ChainTxSubmission{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", reservation.SubmissionID).Take(&row).Error; err != nil {
		return err
	}
	if err := rejectFairLaunchTransactionAction(row.Action); err != nil {
		return err
	}
	if row.State != ChainTxStateSigningReserved || row.Nonce == nil || *row.Nonce != reservation.Nonce || len(row.RawTx) != 0 || row.TxHash != nil {
		return errors.New("signing reservation is no longer unsigned")
	}
	res := tx.Model(&entity.ChainRelayerNonce{}).Where("chain_id = ? AND sender_address = ? AND next_nonce = ? AND active_submission_id = ?", reservation.ChainID, reservation.SenderAddress, reservation.Nonce, reservation.SubmissionID).Updates(map[string]interface{}{"active_submission_id": nil, "updated_at": now})
	if res.Error != nil || res.RowsAffected != 1 {
		if res.Error != nil {
			return res.Error
		}
		return errors.New("sender nonce fence changed before release")
	}
	res = tx.Model(&entity.ChainTxSubmission{}).Where("id = ? AND state = ? AND nonce = ?", reservation.SubmissionID, ChainTxStateSigningReserved, reservation.Nonce).Updates(map[string]interface{}{"state": constant.ChainTxStateReady, "nonce": nil, "unsigned_tx_digest": "", "signing_digest": "", "signing_reserved_at": nil, "active_slot": 1, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errors.New("submission changed before reservation release")
	}
	return nil
}

func validDigest(value string) bool {
	if len(value) != 66 || !strings.HasPrefix(value, "0x") {
		return false
	}
	for _, char := range value[2:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
			return false
		}
	}
	return common.HexToHash(value) != (common.Hash{})
}

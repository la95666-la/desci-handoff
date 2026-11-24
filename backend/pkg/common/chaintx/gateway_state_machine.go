package chaintx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"gorm.io/gorm"
	"joincare_server/pkg/common/entity"
	"time"
)

// RunGatewaySigning performs the reserved-generation-request-journal-commit
// state machine. Caller must supply an already built unsigned transaction; no
// private key or local signer is accepted anywhere in this API.
func RunGatewaySigning(ctx context.Context, db *gorm.DB, gateway SignPolicyGateway, reservation SigningReservation, unsignedTransaction []byte, generation uint32, now time.Time) (GatewayJournalEntry, error) {
	if db == nil {
		return GatewayJournalEntry{}, errors.New("gateway database is required")
	}
	if err := requireGatewaySubmissionAllowed(db, reservation.SubmissionID); err != nil {
		return GatewayJournalEntry{}, err
	}
	if gateway == nil || len(unsignedTransaction) == 0 {
		return GatewayJournalEntry{}, errors.New("remote gateway and unsigned transaction are required")
	}
	var signingGenerationID uint64
	if err := db.Transaction(func(tx *gorm.DB) error {
		row, err := CreateSigningGenerationTx(tx, reservation, generation, now)
		signingGenerationID = row.ID
		return err
	}); err != nil {
		return GatewayJournalEntry{}, err
	}
	request := GatewayCommitRequest{GatewayJournalKey: GatewayJournalKey{ChainID: reservation.ChainID, ExpectedSignerAddress: reservation.SenderAddress, Nonce: reservation.Nonce}, SubmissionID: reservation.SubmissionID, ReplacementGeneration: generation - 1, UnsignedTxDigest: reservation.UnsignedTxDigest, SigningDigest: reservation.SigningDigest, UnsignedTransaction: append([]byte(nil), unsignedTransaction...)}
	result, callErr := gateway.Commit(ctx, request)
	requestSum := sha256.Sum256(unsignedTransaction)
	requestDigest := "0x" + hex.EncodeToString(requestSum[:])
	if callErr != nil {
		journal, journalErr := gateway.Journal(ctx, request.GatewayJournalKey)
		if journalErr != nil {
			_ = db.Transaction(func(tx *gorm.DB) error {
				var row struct{ ID uint64 }
				if err := tx.Table("fair_launch_signing_generation").Select("id").Where("id=?", signingGenerationID).Take(&row).Error; err != nil {
					return err
				}
				_, err := AppendGatewayJournalTx(tx, entitySigningGeneration(signingGenerationID, reservation, generation), 1, "recovery", requestDigest, GatewayJournalEntry{}, time.Now().UTC())
				if err != nil {
					return err
				}
				return MarkSigningRecoveryTx(tx, reservation.SubmissionID, "gateway commit and journal unavailable", time.Now().UTC())
			})
			return GatewayJournalEntry{}, callErr
		}
		result = journal
	}
	state, _ := ReconcileGatewayJournal(result)
	return result, db.Transaction(func(tx *gorm.DB) error {
		gen := entitySigningGeneration(signingGenerationID, reservation, generation)
		journalState := "returned"
		if state == ChainTxStateSigningRecovery {
			journalState = "recovery"
		} else if state == "raw_tx_committed" {
			journalState = "raw_committed"
		} else if state == "safe_to_release_unsigned_reservation" {
			journalState = "rejected"
		}
		if _, err := AppendGatewayJournalTx(tx, gen, 1, journalState, requestDigest, result, time.Now().UTC()); err != nil {
			return err
		}
		switch state {
		case "raw_tx_committed":
			return CommitGatewayResultTx(tx, reservation, result, time.Now().UTC())
		case "safe_to_release_unsigned_reservation":
			return ReleaseAbsentGatewayReservationTx(tx, reservation, result, time.Now().UTC())
		default:
			return MarkSigningRecoveryTx(tx, reservation.SubmissionID, "gateway journal requires recovery", time.Now().UTC())
		}
	})
}

func RecoverGatewaySigning(ctx context.Context, db *gorm.DB, gateway SignPolicyGateway, reservation SigningReservation, generation uint32, now time.Time) (GatewayJournalEntry, error) {
	if db == nil {
		return GatewayJournalEntry{}, errors.New("gateway recovery dependencies are required")
	}
	if err := requireGatewaySubmissionAllowed(db, reservation.SubmissionID); err != nil {
		return GatewayJournalEntry{}, err
	}
	if gateway == nil {
		return GatewayJournalEntry{}, errors.New("gateway recovery dependencies are required")
	}
	entry, err := gateway.Journal(ctx, GatewayJournalKey{ChainID: reservation.ChainID, ExpectedSignerAddress: reservation.SenderAddress, Nonce: reservation.Nonce})
	if err != nil {
		_ = db.Transaction(func(tx *gorm.DB) error {
			return MarkSigningRecoveryTx(tx, reservation.SubmissionID, "gateway recovery journal unavailable", now)
		})
		return GatewayJournalEntry{}, err
	}
	state, _ := ReconcileGatewayJournal(entry)
	err = db.Transaction(func(tx *gorm.DB) error {
		switch state {
		case "raw_tx_committed":
			return CommitGatewayResultTx(tx, reservation, entry, now)
		case "safe_to_release_unsigned_reservation":
			return ReleaseAbsentGatewayReservationTx(tx, reservation, entry, now)
		default:
			return MarkSigningRecoveryTx(tx, reservation.SubmissionID, "gateway recovery remains ambiguous", now)
		}
	})
	return entry, err
}

// reservation carries no action, so this is the smallest safe identity read;
// it happens before generation persistence or any Gateway call.
func requireGatewaySubmissionAllowed(db *gorm.DB, submissionID uint64) error {
	var row struct {
		Action string `gorm:"column:action"`
	}
	if err := db.Model(&entity.ChainTxSubmission{}).Select("action").Where("id = ?", submissionID).Take(&row).Error; err != nil {
		return err
	}
	return rejectFairLaunchTransactionAction(row.Action)
}

func entitySigningGeneration(id uint64, r SigningReservation, generation uint32) entity.FairLaunchSigningGeneration {
	return entity.FairLaunchSigningGeneration{ID: id, SubmissionID: r.SubmissionID, Generation: generation, ChainID: r.ChainID, ExpectedSignerAddress: r.SenderAddress, Nonce: r.Nonce, SigningDigest: stripDigest(r.SigningDigest), UnsignedTxDigest: stripDigest(r.UnsignedTxDigest)}
}

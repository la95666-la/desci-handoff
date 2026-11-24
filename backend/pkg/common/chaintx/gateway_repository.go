package chaintx

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"gorm.io/gorm"
	"joincare_server/pkg/common/entity"
	"strings"
	"time"
)

func CreateSigningGenerationTx(tx *gorm.DB, r SigningReservation, generation uint32, now time.Time) (entity.FairLaunchSigningGeneration, error) {
	if err := requireGatewaySubmissionAllowed(tx, r.SubmissionID); err != nil {
		return entity.FairLaunchSigningGeneration{}, err
	}
	if generation == 0 || now.IsZero() {
		return entity.FairLaunchSigningGeneration{}, errors.New("invalid signing generation")
	}
	row := entity.FairLaunchSigningGeneration{SubmissionID: r.SubmissionID, Generation: generation, ChainID: r.ChainID, ExpectedSignerAddress: strings.ToLower(r.SenderAddress), Nonce: r.Nonce, SigningDigest: stripDigest(r.SigningDigest), UnsignedTxDigest: stripDigest(r.UnsignedTxDigest), State: "reserved", CreatedAt: now}
	if err := tx.Create(&row).Error; err != nil {
		return row, err
	}
	return row, nil
}
func AppendGatewayJournalTx(tx *gorm.DB, generation entity.FairLaunchSigningGeneration, journalGeneration uint32, state, requestDigest string, result GatewayJournalEntry, now time.Time) (entity.FairLaunchGatewaySigningJournal, error) {
	if err := requireGatewaySubmissionAllowed(tx, generation.SubmissionID); err != nil {
		return entity.FairLaunchGatewaySigningJournal{}, err
	}
	if journalGeneration == 0 || now.IsZero() {
		return entity.FairLaunchGatewaySigningJournal{}, errors.New("invalid journal generation")
	}
	rawDigest := ""
	if len(result.RawTransaction) > 0 {
		sum := sha256.Sum256(result.RawTransaction)
		rawDigest = hex.EncodeToString(sum[:])
	}
	row := entity.FairLaunchGatewaySigningJournal{SubmissionID: generation.SubmissionID, SigningGenerationID: generation.ID, JournalGeneration: journalGeneration, ChainID: generation.ChainID, ExpectedSignerAddress: generation.ExpectedSignerAddress, Nonce: generation.Nonce, SigningDigest: generation.SigningDigest, RequestDigest: stripDigest(requestDigest), RawTx: append([]byte(nil), result.RawTransaction...), RawTxDigest: rawDigest, TxHash: strings.ToLower(result.TxHash), State: state, CreatedAt: now, UpdatedAt: now}
	if err := tx.Create(&row).Error; err != nil {
		return row, err
	}
	return row, nil
}
func stripDigest(value string) string { return strings.TrimPrefix(strings.ToLower(value), "0x") }

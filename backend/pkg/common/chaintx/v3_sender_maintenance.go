package chaintx

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/v3diagnostic"
)

type SenderMaintenanceRPC interface{ RetiredNonceRPC }
type SenderMaintenanceResult struct {
	Refreshed        int
	Skipped          bool
	FailedSubmission string
	SkipReason       string
	ReadDiagnostic   *V3MaintenanceReadDiagnostic
}

// Only explicitly enrolled/non-revoked archived originals can be refreshed.
// No RPC under SQL locks; no key, send, nonce or outbox write capability.
// Idle senders only. Signed active attempts use the separately authorized
// active-original recovery path, never an automatic nonce release.
func RefreshIdleV3SenderEvidence(ctx context.Context, db *gorm.DB, chain uint64, sender common.Address, reader, independent SenderMaintenanceRPC) (result SenderMaintenanceResult, resultErr error) {
	stage := "sender_maintenance_selection"
	defer func() {
		if resultErr != nil && v3diagnostic.Reason(resultErr) == "UNAVAILABLE" {
			resultErr = v3diagnostic.New("SENDER_EVIDENCE_UNAVAILABLE", stage, nil)
		}
	}()
	if ctx == nil || ctx.Err() != nil || db == nil || chain == 0 || sender == (common.Address{}) || reader == nil || independent == nil {
		return result, errRetiredNonce
	}
	var fences []entity.ChainRelayerNonce
	if db.WithContext(ctx).Raw(`SELECT * FROM chain_relayer_nonce WHERE chain_id=? AND BINARY LOWER(sender_address)=BINARY ?`, chain, strings.ToLower(sender.Hex())).Scan(&fences).Error != nil || len(fences) != 1 {
		return result, errRetiredNonce
	}
	baseline := fences[0]
	if baseline.ActiveSubmissionID != nil {
		result.Skipped = true
		result.SkipReason = "active_submission"
		return result, nil
	}
	var rows []entity.ChainTxSubmission
	if db.WithContext(ctx).Raw(`SELECT * FROM chain_tx_submission WHERE chain_id=? AND BINARY LOWER(sender_address)=BINARY ? AND state='v3_reorg_suspended' ORDER BY id LIMIT 33`, chain, strings.ToLower(sender.Hex())).Scan(&rows).Error != nil || len(rows) > 32 {
		return result, errRetiredNonce
	}
	if len(rows) == 0 {
		result.Skipped = true
		result.SkipReason = "no_retired_records"
		return result, nil
	}
	// Reuse sufficient evidence; audits never slide timestamps on ordinary polls.
	readyErr := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if chainreorg.LockNoActiveIncidentTx(tx, chain, rows[0].ContractAddress) != nil {
			return errRetiredNonce
		}
		return requireV3SenderEvidenceTx(ctx, tx, chain, sender, SenderMaintenanceReserve)
	})
	if err := readyErr; err == nil {
		result.Skipped = true
		result.SkipReason = "sufficient_evidence"
		return result, nil
	} else if v3diagnostic.Reason(err) != "SENDER_EVIDENCE_PENDING" {
		return result, err
	}
	started := time.Now()
	primary := &v3MaintenanceReader{RetiredNonceRPC: reader, provider: "primary"}
	secondary := &v3MaintenanceReader{RetiredNonceRPC: independent, provider: "independent"}
	inputs := make([]activeRetirementInput, 0, len(rows))
	for _, row := range rows {
		result.FailedSubmission = strconv.FormatUint(row.ID, 10)
		stage = "sender_maintenance_enrollment"
		in, e := readActiveRetirementInput(db.WithContext(ctx), row, false)
		if e != nil {
			if ctx.Err() != nil {
				return result, v3diagnostic.New("SENDER_EVIDENCE_PENDING", "sender_maintenance_deadline", nil)
			}
			if v3diagnostic.Reason(e) != "UNAVAILABLE" {
				return result, e
			}
			return result, v3diagnostic.New("SENDER_RETIREMENT_REVIEW_REQUIRED", stage, nil)
		}
		first, e := auditRetiredNonce(ctx, primary, row, in.Binding, in.Payload)
		if e != nil {
			d := primary.diagnostic
			result.ReadDiagnostic = &d
			return result, e
		}
		second, e := auditRetiredNonce(ctx, secondary, row, in.Binding, in.Payload)
		if e != nil {
			d := secondary.diagnostic
			result.ReadDiagnostic = &d
			return result, e
		}
		stage = "sender_maintenance_provider_agreement"
		if !sameRetiredAudit(first, second) || first.ConsumedNonce != baseline.NextNonce || second.ConsumedNonce != baseline.NextNonce {
			return result, errRetiredNonce
		}
		in.Proof = first
		inputs = append(inputs, in)
	}
	stage = "sender_maintenance_final_nonce"
	for _, rpc := range []*v3MaintenanceReader{primary, secondary} {
		latest, e := rpc.NonceAt(ctx, sender, nil)
		if e != nil || latest != baseline.NextNonce {
			d := rpc.diagnostic
			result.ReadDiagnostic = &d
			return result, errRetiredNonce
		}
		pending, e := rpc.PendingNonceAt(ctx, sender)
		if e != nil || pending != latest {
			d := rpc.diagnostic
			result.ReadDiagnostic = &d
			return result, errRetiredNonce
		}
	}
	if time.Since(started) > 90*time.Second || ctx.Err() != nil {
		return result, errRetiredNonce
	}
	stage = "sender_maintenance_commit"
	if e := commitIdleV3SenderEvidence(ctx, db, chain, sender, baseline, inputs); e != nil {
		return result, e
	}
	result.Refreshed = len(inputs)
	result.FailedSubmission = ""
	return result, nil
}

func commitIdleV3SenderEvidence(ctx context.Context, db *gorm.DB, chain uint64, sender common.Address, baseline entity.ChainRelayerNonce, inputs []activeRetirementInput) error {
	invalid := v3diagnostic.New("SENDER_EVIDENCE_PENDING", "sender_maintenance_commit", nil)
	if len(inputs) == 0 || baseline.ActiveSubmissionID != nil {
		return invalid
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if chainreorg.LockNoActiveIncidentTx(tx, chain, inputs[0].Row.ContractAddress) != nil {
			return invalid
		}
		var rows []entity.ChainTxSubmission
		if tx.Raw(`SELECT * FROM chain_tx_submission WHERE chain_id=? AND BINARY LOWER(sender_address)=BINARY ? AND state='v3_reorg_suspended' ORDER BY id FOR UPDATE`, chain, strings.ToLower(sender.Hex())).Scan(&rows).Error != nil || len(rows) != len(inputs) {
			return invalid
		}
		for i, row := range rows {
			if !reflect.DeepEqual(row, inputs[i].Row) {
				return invalid
			}
			fresh, e := readActiveRetirementInput(tx, row, true)
			if e != nil || fresh.PreviousID != inputs[i].PreviousID || fresh.Binding != inputs[i].Binding || fresh.Payload != inputs[i].Payload {
				return invalid
			}
		}
		var fence entity.ChainRelayerNonce
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("chain_id=? AND sender_address=?", chain, baseline.SenderAddress).Take(&fence).Error != nil || fence.ActiveSubmissionID != nil || fence.NextNonce != baseline.NextNonce || !strings.EqualFold(fence.SenderAddress, sender.Hex()) {
			return invalid
		}
		for _, in := range inputs {
			p := in.Proof
			if p.SubmissionID != in.Row.ID || p.ConsumedNonce != fence.NextNonce || p.Nonce >= p.ConsumedNonce || p.RawSHA256 != retiredSHA(in.Row.RawTx) || len(p.EvidenceDigest) != 64 {
				return errors.New("retired proof mismatch")
			}
			r := tx.Exec(`INSERT INTO ido_v3_retired_nonce_evidence(submission_id,chain_id,project_id,sender_address,contract_address,action,nonce,tx_hash,raw_sha256,receipt_block,receipt_hash,journal_id,confirmed_block,confirmed_hash,consumed_nonce,evidence_digest,expires_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,UTC_TIMESTAMP(6)+INTERVAL 5 MINUTE)`, p.SubmissionID, p.ChainID, p.ProjectID, p.SenderAddress, p.ContractAddress, p.Action, p.Nonce, p.TxHash, p.RawSHA256, p.ReceiptBlock, p.ReceiptHash, p.JournalID, p.ConfirmedBlock, p.ConfirmedHash, p.ConsumedNonce, p.EvidenceDigest)
			if r.Error != nil || r.RowsAffected != 1 {
				return invalid
			}
			var accepted []struct{ ID uint64 }
			if tx.Raw(`SELECT id FROM ido_v3_retired_nonce_exemption WHERE id=?`, p.SubmissionID).Scan(&accepted).Error != nil || len(accepted) != 1 {
				return invalid
			}
		}
		return nil
	})
}

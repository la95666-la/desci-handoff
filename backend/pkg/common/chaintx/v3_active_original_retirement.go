package chaintx

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"joincare_server/pkg/common/chainreorg"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

type activeRetirementInput struct {
	Row        entity.ChainTxSubmission
	Binding    idov3whitelist.ExpectedProjectBinding
	Payload    string
	PreviousID uint64
	Proof      retiredNonceEvidence
}
type ActiveRetirementReport struct {
	ActiveSubmission uint64   `json:"active_submission"`
	Hash             string   `json:"original_hash"`
	Refreshed        []uint64 `json:"retired_submissions"`
	Execute          bool     `json:"execute"`
}

// Refreshes ONLY previously enrolled non-revoked retired originals. Two
// read-only providers audit exact signed bytes, canonical receipts and consumed
// nonces before SQL locks. No signing/send capability, outbox update, nonce
// release, enrollment, expiry extension or automatic retry is available.
func RefreshRetirementForActiveOriginal(ctx context.Context, db *gorm.DB, activeID uint64, hash common.Hash, expectedIDs []uint64, reader, independent interface {
	RetiredNonceRPC
	V3FirstBroadcastRPC
}, execute bool) (ActiveRetirementReport, error) {
	report := ActiveRetirementReport{ActiveSubmission: activeID, Hash: hash.Hex(), Execute: execute}
	invalid := errors.New("active original retired evidence refresh refused")
	if ctx == nil || ctx.Err() != nil || db == nil || activeID == 0 || hash == (common.Hash{}) || len(expectedIDs) == 0 || len(expectedIDs) > 32 || reader == nil || independent == nil {
		return report, invalid
	}
	var active entity.ChainTxSubmission
	if db.WithContext(ctx).Where("id=?", activeID).Take(&active).Error != nil || active.TxHash == nil || common.HexToHash(*active.TxHash) != hash {
		return report, invalid
	}
	var project struct {
		ChainID              uint64
		Proxy, Sale, Payment string
	}
	r := db.WithContext(ctx).Raw(`SELECT chain_id,contract_address AS proxy,token_address AS sale,payment_token_address AS payment FROM projects WHERE project_id=? AND contract_version='ido_v3_stablecoin' AND funding_mode='capped_presale' AND onchain_locked=1 AND status<>'archived'`, active.ProjectID).Scan(&project)
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: project.ChainID, ProjectID: active.ProjectID, ProxyAddress: common.HexToAddress(project.Proxy), ProjectToken: common.HexToAddress(project.Sale), PaymentToken: common.HexToAddress(project.Payment)}
	if r.Error != nil || r.RowsAffected != 1 || CheckV3OriginalFirstBroadcast(ctx, active, binding, reader, independent) != nil {
		return report, invalid
	}
	var retired []entity.ChainTxSubmission
	if db.WithContext(ctx).Raw(`SELECT * FROM chain_tx_submission WHERE chain_id=? AND BINARY LOWER(sender_address)=BINARY ? AND state='v3_reorg_suspended' ORDER BY id`, active.ChainID, strings.ToLower(active.SenderAddress)).Scan(&retired).Error != nil || len(retired) != len(expectedIDs) {
		return report, invalid
	}
	started := time.Now()
	inputs := make([]activeRetirementInput, 0, len(retired))
	for i, row := range retired {
		if row.ID != expectedIDs[i] || row.Nonce == nil || active.Nonce == nil || *row.Nonce >= *active.Nonce || !strings.EqualFold(row.ContractAddress, active.ContractAddress) {
			return report, invalid
		}
		input, e := readActiveRetirementInput(db.WithContext(ctx), row, false)
		if e != nil {
			return report, invalid
		}
		first, e := auditRetiredNonce(ctx, reader, row, input.Binding, input.Payload)
		if e != nil {
			return report, invalid
		}
		second, e := auditRetiredNonce(ctx, independent, row, input.Binding, input.Payload)
		if e != nil || !sameRetiredAudit(first, second) || first.ConsumedNonce != *active.Nonce || second.ConsumedNonce != *active.Nonce {
			return report, invalid
		}
		input.Proof = first
		inputs = append(inputs, input)
		report.Refreshed = append(report.Refreshed, row.ID)
	}
	// Do not stamp fresh TTL onto old RPC work; all audits and final original
	// checks must fit inside the bounded evidence preparation window.
	if time.Since(started) > 2*time.Minute || CheckV3OriginalFirstBroadcast(ctx, active, binding, reader, independent) != nil {
		return report, invalid
	}
	err := commitActiveRetirement(ctx, db, active, binding, inputs, execute)
	return report, err
}

func sameRetiredAudit(a, b retiredNonceEvidence) bool {
	a.ConfirmedBlock = 0
	a.ConfirmedHash = ""
	a.EvidenceDigest = ""
	b.ConfirmedBlock = 0
	b.ConfirmedHash = ""
	b.EvidenceDigest = ""
	return a == b
}

func readActiveRetirementInput(tx *gorm.DB, row entity.ChainTxSubmission, lock bool) (activeRetirementInput, error) {
	out := activeRetirementInput{Row: row}
	invalid := errors.New("retired enrollment/current facts unavailable")
	if row.TxHash == nil || row.Nonce == nil || row.ReceiptBlockNumber == nil || row.FinalEventRecordID == nil || retiredNonceSchemaReady(tx) != nil {
		return out, invalid
	}
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	var p struct {
		Status, Sale, Payment, Proxy, Version string
		ChainID                               uint64
		Locked                                bool
	}
	r := tx.Raw(`SELECT status,token_address AS sale,payment_token_address AS payment,contract_address AS proxy,contract_version AS version,chain_id,onchain_locked AS locked FROM projects WHERE project_id=?`+suffix, row.ProjectID).Scan(&p)
	if r.Error != nil || r.RowsAffected != 1 || p.Status != "archived" || !p.Locked || p.ChainID != row.ChainID || p.Version != "ido_v3_stablecoin" || !strings.EqualFold(p.Proxy, row.ContractAddress) {
		return out, invalid
	}
	out.Binding = idov3whitelist.ExpectedProjectBinding{ChainID: row.ChainID, ProjectID: row.ProjectID, ProxyAddress: common.HexToAddress(p.Proxy), ProjectToken: common.HexToAddress(p.Sale), PaymentToken: common.HexToAddress(p.Payment)}
	var enrollment []struct {
		ID    uint64
		State string
	}
	if tx.Raw(`SELECT id,state FROM ido_v3_retired_nonce_evidence WHERE submission_id=? ORDER BY id DESC LIMIT 1`+suffix, row.ID).Scan(&enrollment).Error != nil || len(enrollment) != 1 || enrollment[0].State != "valid" {
		return out, invalid
	}
	out.PreviousID = enrollment[0].ID
	if e := requireV3RetirementMarker(tx, row.ChainID, row.ContractAddress, row.ProjectID); e != nil {
		return out, e
	}
	var journal []struct{ Payload string }
	if tx.Raw(`SELECT payload FROM ido_v3_event_journal WHERE id=? AND chain_id=? AND project_id=? AND proxy_address=? AND contract_version='ido_v3_stablecoin' AND tx_hash=? AND block_number=? AND block_hash=? AND status='canonical'`, *row.FinalEventRecordID, row.ChainID, row.ProjectID, strings.ToLower(row.ContractAddress), strings.ToLower(*row.TxHash), *row.ReceiptBlockNumber, strings.ToLower(row.ReceiptBlockHash)).Scan(&journal).Error != nil || len(journal) != 1 {
		return out, invalid
	}
	out.Payload = journal[0].Payload
	return out, nil
}

var errActiveRetirementDry = errors.New("active retirement dry rollback")

func commitActiveRetirement(ctx context.Context, db *gorm.DB, active entity.ChainTxSubmission, b idov3whitelist.ExpectedProjectBinding, inputs []activeRetirementInput, execute bool) error {
	invalid := errors.New("active original retired evidence commit refused")
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if chainreorg.LockNoActiveIncidentTx(tx, active.ChainID, active.ContractAddress) != nil || LockV3RunningCursorTx(tx, b) != nil {
			return invalid
		}
		var current entity.ChainTxSubmission
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", active.ID).Take(&current).Error != nil || !sameActiveOriginal(active, current, b) {
			return invalid
		}
		var actual []entity.ChainTxSubmission
		if tx.Raw(`SELECT * FROM chain_tx_submission WHERE chain_id=? AND BINARY LOWER(sender_address)=BINARY ? AND state='v3_reorg_suspended' ORDER BY id FOR UPDATE`, active.ChainID, strings.ToLower(active.SenderAddress)).Scan(&actual).Error != nil || len(actual) != len(inputs) {
			return invalid
		}
		for i, row := range actual {
			if !reflect.DeepEqual(row, inputs[i].Row) {
				return invalid
			}
			fresh, e := readActiveRetirementInput(tx, row, true)
			if e != nil || fresh.PreviousID != inputs[i].PreviousID || fresh.Payload != inputs[i].Payload || fresh.Binding != inputs[i].Binding {
				return invalid
			}
			p := inputs[i].Proof
			// Bind each audited receipt to the persisted source facts; block
			// hashes themselves were independently read twice before SQL locks.
			if p.ReceiptHash != strings.ToLower(row.ReceiptBlockHash) || p.TxHash != strings.ToLower(*row.TxHash) || p.JournalID != *row.FinalEventRecordID || p.RawSHA256 != retiredSHA(row.RawTx) {
				return invalid
			}
		}
		var fence entity.ChainRelayerNonce
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("chain_id=? AND sender_address=?", active.ChainID, active.SenderAddress).Take(&fence).Error != nil || !activeRetirementFence(active, fence) {
			return invalid
		}
		if !execute {
			return errActiveRetirementDry
		}
		for _, in := range inputs {
			p := in.Proof
			if p.SubmissionID != in.Row.ID || p.ConsumedNonce != *active.Nonce || p.Nonce >= p.ConsumedNonce || len(p.EvidenceDigest) != 64 {
				return invalid
			}
			r := tx.Exec(`INSERT INTO ido_v3_retired_nonce_evidence(submission_id,chain_id,project_id,sender_address,contract_address,action,nonce,tx_hash,raw_sha256,receipt_block,receipt_hash,journal_id,confirmed_block,confirmed_hash,consumed_nonce,evidence_digest,expires_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,UTC_TIMESTAMP(6)+INTERVAL 5 MINUTE)`, p.SubmissionID, p.ChainID, p.ProjectID, p.SenderAddress, p.ContractAddress, p.Action, p.Nonce, p.TxHash, p.RawSHA256, p.ReceiptBlock, p.ReceiptHash, p.JournalID, p.ConfirmedBlock, p.ConfirmedHash, p.ConsumedNonce, p.EvidenceDigest)
			if r.Error != nil || r.RowsAffected != 1 {
				return invalid
			}
		}
		return nil
	})
	if !execute && errors.Is(err, errActiveRetirementDry) {
		return nil
	}
	return err
}
func sameActiveOriginal(a, c entity.ChainTxSubmission, b idov3whitelist.ExpectedProjectBinding) bool {
	return a.ID == c.ID && a.ProjectID == c.ProjectID && a.Action == c.Action && a.AttemptNo == c.AttemptNo && c.State == "signed" && c.BroadcastCount == 0 && c.LastBroadcastAt == nil && c.ActiveSlot != nil && *c.ActiveSlot == 1 && c.ReceiptStatus == nil && c.ReceiptBlockNumber == nil && c.FinalEventRecordID == nil && c.Nonce != nil && a.Nonce != nil && *c.Nonce == *a.Nonce && c.TxHash != nil && a.TxHash != nil && *a.TxHash == *c.TxHash && bytes.Equal(a.RawTx, c.RawTx) && VerifyV3PersistedAppliedAttempt(c, b) == nil
}
func activeRetirementFence(a entity.ChainTxSubmission, f entity.ChainRelayerNonce) bool {
	return a.Nonce != nil && *a.Nonce != ^uint64(0) && f.ChainID == a.ChainID && strings.EqualFold(f.SenderAddress, a.SenderAddress) && f.ActiveSubmissionID != nil && *f.ActiveSubmissionID == a.ID && f.NextNonce == *a.Nonce+1
}

package chaintx

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"reflect"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/v3diagnostic"
)

var errRetiredNonce = errors.New("retired V3 original-hash nonce reconciliation unavailable")

// No send/sign methods. A confirmed nonce alone never proves WHO consumed it.
type RetiredNonceRPC interface {
	V3ReceiptSource
	NonceAt(context.Context, common.Address, *big.Int) (uint64, error)
	PendingNonceAt(context.Context, common.Address) (uint64, error)
	TransactionByHash(context.Context, common.Hash) (*types.Transaction, bool, error)
}

type retiredNonceEvidence struct {
	SubmissionID, ChainID, ProjectID       uint64
	SenderAddress, ContractAddress, Action string
	Nonce                                  uint64
	TxHash, RawSHA256                      string
	ReceiptBlock                           uint64
	ReceiptHash                            string
	JournalID, ConfirmedBlock              uint64
	ConfirmedHash                          string
	ConsumedNonce                          uint64
	EvidenceDigest                         string
}

func retiredSHA(raw []byte) string { s := sha256.Sum256(raw); return hex.EncodeToString(s[:]) }

// Pure chain audit of an independently bound retired signed attempt. It neither
// mutates the supplied row nor releases its old nonce/project publication.
func auditRetiredNonce(ctx context.Context, rpc RetiredNonceRPC, row entity.ChainTxSubmission, b idov3whitelist.ExpectedProjectBinding, journalPayload string) (result retiredNonceEvidence, resultErr error) {
	stage := "retired_original_signature"
	defer func() {
		if resultErr != nil {
			resultErr = v3diagnostic.New("SENDER_EVIDENCE_PENDING", stage, nil)
		}
	}()
	var zero retiredNonceEvidence
	if ctx == nil || rpc == nil || ctx.Err() != nil || row.ID == 0 || row.State != ChainTxStateV3ReorgSuspended || row.ActiveSlot != nil || row.Nonce == nil || *row.Nonce == ^uint64(0) || row.TxHash == nil || row.FinalEventRecordID == nil || row.ReceiptStatus == nil || *row.ReceiptStatus != 1 || row.ReceiptBlockNumber == nil || row.ReceiptBlockHash == "" || VerifyV3PersistedAppliedAttempt(row, b) != nil {
		return zero, errRetiredNonce
	}
	stage = "retired_chain_identity"
	chain, e := rpc.ChainID(ctx)
	if e != nil || chain == nil || !chain.IsUint64() || chain.Uint64() != b.ChainID {
		return zero, errRetiredNonce
	}
	hash := common.HexToHash(*row.TxHash)
	stage = "retired_original_transaction"
	transaction, pending, e := rpc.TransactionByHash(ctx, hash)
	if e != nil || pending || transaction == nil || transaction.Hash() != hash {
		return zero, errRetiredNonce
	}
	raw, e := transaction.MarshalBinary()
	if e != nil || !bytes.Equal(raw, row.RawTx) {
		return zero, errRetiredNonce
	}
	stage = "retired_confirmed_anchor"
	head, e := rpc.BlockNumber(ctx)
	if e != nil || head < 14 {
		return zero, errRetiredNonce
	}
	confirmed := head - 14
	h, e := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(confirmed))
	if e != nil || h == nil || h.Number == nil || !h.Number.IsUint64() || h.Number.Uint64() != confirmed || h.Hash() == (common.Hash{}) {
		return zero, errRetiredNonce
	}
	sender := common.HexToAddress(row.SenderAddress)
	stage = "retired_consumed_nonce"
	consumed, e := rpc.NonceAt(ctx, sender, h.Number)
	if e != nil || consumed <= *row.Nonce {
		return zero, errRetiredNonce
	}
	var first []byte
	for pass := 0; pass < 2; pass++ {
		stage = "retired_original_receipt"
		receipt, e := rpc.TransactionReceipt(ctx, hash)
		if e != nil || receipt == nil || receipt.TxHash != hash || receipt.Status != 1 || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Uint64() != *row.ReceiptBlockNumber || !strings.EqualFold(receipt.BlockHash.Hex(), row.ReceiptBlockHash) || !idov3whitelist.HasConfirmations(head, *row.ReceiptBlockNumber, 15) {
			return zero, errRetiredNonce
		}
		stage = "retired_receipt_anchor"
		rh, e := rpc.HeaderByNumber(ctx, receipt.BlockNumber)
		if e != nil || rh == nil || rh.Number == nil || rh.Number.Cmp(receipt.BlockNumber) != 0 || rh.Hash() != receipt.BlockHash {
			return zero, errRetiredNonce
		}
		stage = "retired_journal_match"
		if matchRetiredJournal(row, b, journalPayload, receipt) != nil {
			return zero, errRetiredNonce
		}
		// Compare full RPC receipt JSON across independent reads, including logs.
		stage = "retired_repeat_receipt"
		encoded, e := json.Marshal(receipt)
		if e != nil {
			return zero, errRetiredNonce
		}
		if pass == 0 {
			first = encoded
		} else if !bytes.Equal(first, encoded) {
			return zero, errRetiredNonce
		}
	}
	stage = "retired_final_anchor"
	again, e := rpc.HeaderByNumber(ctx, h.Number)
	if e != nil || again == nil || again.Number == nil || again.Number.Cmp(h.Number) != 0 || again.Hash() != h.Hash() {
		return zero, errRetiredNonce
	}
	stage = "retired_final_nonce"
	latest, e := rpc.NonceAt(ctx, sender, nil)
	if e != nil || latest < consumed {
		return zero, errRetiredNonce
	}
	mempool, e := rpc.PendingNonceAt(ctx, sender)
	if e != nil || mempool != latest {
		return zero, errRetiredNonce
	}
	if ctx.Err() != nil {
		return zero, ctx.Err()
	}
	out := retiredNonceEvidence{SubmissionID: row.ID, ChainID: row.ChainID, ProjectID: row.ProjectID, SenderAddress: strings.ToLower(sender.Hex()), ContractAddress: strings.ToLower(b.ProxyAddress.Hex()), Action: row.Action, Nonce: *row.Nonce, TxHash: strings.ToLower(hash.Hex()), RawSHA256: retiredSHA(row.RawTx), ReceiptBlock: *row.ReceiptBlockNumber, ReceiptHash: strings.ToLower(row.ReceiptBlockHash), JournalID: *row.FinalEventRecordID, ConfirmedBlock: confirmed, ConfirmedHash: strings.ToLower(h.Hash().Hex()), ConsumedNonce: consumed}
	encoded, e := json.Marshal(out)
	if e != nil {
		return zero, e
	}
	out.EvidenceDigest = retiredSHA(encoded)
	return out, nil
}

func matchRetiredJournal(row entity.ChainTxSubmission, b idov3whitelist.ExpectedProjectBinding, payload string, r *types.Receipt) error {
	var stored basechain.V3ReplayEvent
	if json.Unmarshal([]byte(payload), &stored) != nil {
		return errRetiredNonce
	}
	matches := 0
	for _, l := range r.Logs {
		if l == nil || l.Address != b.ProxyAddress {
			continue
		}
		event, e := basechain.DecodeV3ReplayLog(*l, b)
		if e != nil {
			continue
		}
		action := ""
		if event.Created != nil {
			action = "ido_v3_create"
		}
		if event.Lifecycle != nil {
			switch event.Lifecycle.Kind {
			case "ProjectActivated":
				action = "ido_v3_activate"
			case "ProjectEnded":
				action = "ido_v3_end"
			case "PlatformFeeWithdrawn":
				action = "ido_v3_withdraw_platform_fee"
			}
		}
		if action != row.Action {
			continue
		}
		if !reflect.DeepEqual(event, stored) {
			return errRetiredNonce
		}
		var match error
		if event.Created != nil {
			match = basechain.MatchV3CreatedReceipt(event.Created, b, r)
		} else {
			match = basechain.MatchV3LifecycleReceipt(event.Lifecycle, b, r)
		}
		if match != nil {
			return errRetiredNonce
		}
		matches++
	}
	if matches != 1 {
		return errRetiredNonce
	}
	return nil
}

func retiredNonceSchemaReady(tx *gorm.DB) error {
	var n int64
	if tx.Raw(`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN ('ido_v3_retired_nonce_evidence','ido_v3_retired_nonce_exemption')`).Scan(&n).Error != nil || n != 2 {
		return errRetiredNonce
	}
	if tx.Raw(`SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME IN ('jc_v3_retired_nonce_insert','jc_v3_retired_nonce_update','jc_v3_retired_nonce_no_delete')`).Scan(&n).Error != nil || n != 3 {
		return errRetiredNonce
	}
	return nil
}

// enroll=false is the signing path: it may refresh ONLY an explicitly enrolled
// non-revoked original attempt. Merely archived rows do NOT get exemptions.
func reconcileRetiredNonceTx(ctx context.Context, tx *gorm.DB, rpc RetiredNonceRPC, id uint64, expectedHash common.Hash, enroll, execute bool) (retiredNonceEvidence, error) {
	var zero retiredNonceEvidence
	if retiredNonceSchemaReady(tx) != nil {
		return zero, errRetiredNonce
	}
	var row entity.ChainTxSubmission
	if tx.Raw(`SELECT * FROM chain_tx_submission WHERE id=? FOR UPDATE`, id).Scan(&row).Error != nil || row.ID != id || row.TxHash == nil || common.HexToHash(*row.TxHash) != expectedHash {
		return zero, errRetiredNonce
	}
	var project struct {
		Status, ProjectToken, PaymentToken, ContractVersion, ContractAddress string
		ChainID                                                              uint64
		OnchainLocked                                                        bool
	}
	r := tx.Raw(`SELECT status,token_address AS project_token,payment_token_address AS payment_token,contract_version,contract_address,chain_id,onchain_locked FROM projects WHERE project_id=? FOR UPDATE`, row.ProjectID).Scan(&project)
	if r.Error == nil && r.RowsAffected == 1 && project.Status != "archived" {
		return zero, v3diagnostic.New("SENDER_SUSPENDED", "sender_check", errRetiredNonce)
	}
	if r.Error != nil || r.RowsAffected != 1 || project.Status != "archived" || !project.OnchainLocked || project.ChainID != row.ChainID || project.ContractVersion != "ido_v3_stablecoin" || !strings.EqualFold(project.ContractAddress, row.ContractAddress) {
		return zero, errRetiredNonce
	}
	if e := requireV3RetirementMarker(tx, row.ChainID, row.ContractAddress, row.ProjectID); e != nil {
		return zero, e
	}
	if !enroll {
		var previous struct{ State string }
		if r := tx.Raw(`SELECT state FROM ido_v3_retired_nonce_evidence WHERE submission_id=? ORDER BY id DESC LIMIT 1 FOR UPDATE`, id).Scan(&previous); r.Error != nil || r.RowsAffected != 1 || previous.State != "valid" {
			return zero, errRetiredNonce
		}
	}
	if row.FinalEventRecordID == nil {
		return zero, errRetiredNonce
	}
	var journal struct{ Payload string }
	r = tx.Raw(`SELECT payload FROM ido_v3_event_journal WHERE id=? AND chain_id=? AND project_id=? AND proxy_address=? AND contract_version='ido_v3_stablecoin' AND tx_hash=? AND block_number=? AND block_hash=? AND status='canonical'`, *row.FinalEventRecordID, row.ChainID, row.ProjectID, strings.ToLower(row.ContractAddress), strings.ToLower(*row.TxHash), row.ReceiptBlockNumber, strings.ToLower(row.ReceiptBlockHash)).Scan(&journal)
	if r.Error != nil || r.RowsAffected != 1 {
		return zero, errRetiredNonce
	}
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: row.ChainID, ProjectID: row.ProjectID, ProxyAddress: common.HexToAddress(row.ContractAddress), ProjectToken: common.HexToAddress(project.ProjectToken), PaymentToken: common.HexToAddress(project.PaymentToken)}
	proof, e := auditRetiredNonce(ctx, rpc, row, binding, journal.Payload)
	if e != nil {
		return zero, e
	}
	var fence entity.ChainRelayerNonce
	if r = tx.Raw(`SELECT * FROM chain_relayer_nonce WHERE chain_id=? AND BINARY LOWER(sender_address)=BINARY ? FOR UPDATE`, row.ChainID, proof.SenderAddress).Scan(&fence); r.Error != nil || r.RowsAffected != 1 || fence.ActiveSubmissionID != nil {
		return zero, errRetiredNonce
	}
	latest, e := rpc.NonceAt(ctx, common.HexToAddress(row.SenderAddress), nil)
	if e != nil || fence.NextNonce != latest || latest <= proof.Nonce {
		return zero, errRetiredNonce
	}
	pending, e := rpc.PendingNonceAt(ctx, common.HexToAddress(row.SenderAddress))
	if e != nil || pending != latest {
		return zero, errRetiredNonce
	}
	if !execute {
		return proof, nil
	}
	var existing int64
	if tx.Raw(`SELECT COUNT(*) FROM ido_v3_retired_nonce_evidence WHERE evidence_digest=? AND state='valid' AND expires_at>UTC_TIMESTAMP(6)`, proof.EvidenceDigest).Scan(&existing).Error != nil {
		return zero, errRetiredNonce
	}
	if existing == 1 {
		return proof, nil
	}
	r = tx.Exec(`INSERT INTO ido_v3_retired_nonce_evidence(submission_id,chain_id,project_id,sender_address,contract_address,action,nonce,tx_hash,raw_sha256,receipt_block,receipt_hash,journal_id,confirmed_block,confirmed_hash,consumed_nonce,evidence_digest,expires_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,UTC_TIMESTAMP(6)+INTERVAL 5 MINUTE)`, proof.SubmissionID, proof.ChainID, proof.ProjectID, proof.SenderAddress, proof.ContractAddress, proof.Action, proof.Nonce, proof.TxHash, proof.RawSHA256, proof.ReceiptBlock, proof.ReceiptHash, proof.JournalID, proof.ConfirmedBlock, proof.ConfirmedHash, proof.ConsumedNonce, proof.EvidenceDigest)
	if r.Error != nil {
		return zero, errRetiredNonce
	}
	return proof, nil
}

// Explicit enrollment tool, default dry-run. Persists evidence only, not a new
// outbox state. Original signed bytes, applied status, nonce fence are untouched.
func ReconcileRetiredV3Nonce(ctx context.Context, db *gorm.DB, rpc RetiredNonceRPC, id uint64, expectedHash common.Hash, execute bool) (retiredNonceEvidence, error) {
	var proof retiredNonceEvidence
	if ctx == nil || db == nil || rpc == nil || id == 0 || expectedHash == (common.Hash{}) {
		return proof, errRetiredNonce
	}
	e := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var e error
		proof, e = reconcileRetiredNonceTx(ctx, tx, rpc, id, expectedHash, true, execute)
		return e
	})
	return proof, e
}

func requireV3SuspendedSenderTx(ctx context.Context, tx *gorm.DB, _ RetiredNonceRPC, chain uint64, sender common.Address) error {
	// Compatibility wrapper. No RPC and no evidence writes in signing locks.
	return requireV3SenderEvidenceTx(ctx, tx, chain, sender, SenderSigningReserve)
}

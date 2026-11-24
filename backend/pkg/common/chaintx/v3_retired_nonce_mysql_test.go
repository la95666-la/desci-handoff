package chaintx

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"joincare_server/pkg/common/entity"
)

func TestRetiredNonceDisposableMySQLGuardAndTriggerAgree(t *testing.T) {
	dsn := os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if dsn == "" {
		t.Skip("disposable MySQL only")
	}
	db, e := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal("fixture db unavailable")
	}
	var name, version string
	if db.Raw(`SELECT DATABASE()`).Scan(&name).Error != nil || !strings.HasPrefix(name, "jc_v3_audit_") || db.Raw(`SELECT VERSION()`).Scan(&version).Error != nil || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("refuse non-disposable DB")
	}
	old := viper.Get("idoV3RetiredNonce.enabled")
	viper.Set("idoV3RetiredNonce.enabled", true)
	defer viper.Set("idoV3RetiredNonce.enabled", old)
	ctx := context.Background()
	rollback := errors.New("retired nonce fixture rollback")
	e = db.Transaction(func(tx *gorm.DB) error {
		row, b, payload, rpc := retiredNonceFixture(t)
		proxy := strings.ToLower(b.ProxyAddress.Hex())
		exec := func(q string, args ...any) {
			t.Helper()
			if e := tx.Exec(q, args...).Error; e != nil {
				t.Fatal(e)
			}
		}
		exec(`INSERT INTO ido_v3_runtime(chain_id,proxy_address,implementation_address,proxy_code_hash,implementation_code_hash,governor_address,start_block) VALUES(31337,?,'0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',CONCAT('0x',REPEAT('a',64)),CONCAT('0x',REPEAT('b',64)),'0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',1)`, proxy)
		exec(`INSERT INTO projects(project_id,status,funding_mode,contract_version,chain_id,contract_address,token_address,payment_token_address,description,tags,team_info,roadmap,partners) VALUES(997,'Set','capped_presale','ido_v3_stablecoin',31337,?,?,?,'','','','','')`, proxy, strings.ToLower(b.ProjectToken.Hex()), strings.ToLower(b.PaymentToken.Hex()))
		exec(`INSERT INTO ido_v3_project_binding(project_id,chain_id,proxy_address) VALUES(997,31337,?)`, proxy)
		exec(`INSERT INTO ido_v3_set_create_commitment(project_id,chain_id,proxy_address,request_digest,manifest_digest,create_calldata_hash,operator_address) VALUES(997,31337,?,CONCAT('0x',REPEAT('1',64)),CONCAT('0x',REPEAT('2',64)),CONCAT('0x',REPEAT('3',64)),?)`, proxy, strings.ToLower(row.SenderAddress))
		exec(`UPDATE projects SET status='Pending',onchain_locked=1 WHERE project_id=997`)
		exec(`INSERT INTO chain_tx_submission(project_id,action,attempt_no,chain_id,sender_address,contract_address,call_data,state,active_slot,last_error) VALUES(997,'ido_v3_activate',1,31337,?,?,?,'ready',1,'')`, row.SenderAddress, proxy, row.CallData)
		exec(`UPDATE chain_tx_submission SET state='signed',nonce=7,gas_limit=100000,gas_price_raw='1',raw_tx=?,tx_hash=?,v3_signed_at=UTC_TIMESTAMP(6) WHERE project_id=997 AND state='ready'`, row.RawTx, *row.TxHash)
		exec(`INSERT INTO ido_v3_event_journal(chain_id,proxy_address,project_id,block_number,block_hash,tx_hash,log_index,event_kind,payload) VALUES(31337,?,997,90,?,?,1,'ProjectActivated',?)`, proxy, strings.ToLower(row.ReceiptBlockHash), strings.ToLower(*row.TxHash), payload)
		var jid uint64
		tx.Raw(`SELECT id FROM ido_v3_event_journal WHERE project_id=997`).Scan(&jid)
		exec(`UPDATE chain_tx_submission SET state='applied',active_slot=NULL,receipt_status=1,receipt_block_number=90,receipt_block_hash=?,final_event_record_id=? WHERE project_id=997 AND state='signed'`, row.ReceiptBlockHash, jid)
		exec(`UPDATE chain_tx_submission SET state='v3_reorg_suspended' WHERE project_id=997 AND state='applied'`)
		exec(`UPDATE projects SET status='archived' WHERE project_id=997`)
		exec(`INSERT INTO chain_relayer_nonce(chain_id,sender_address,next_nonce,active_submission_id) VALUES(31337,?,8,NULL)`, row.SenderAddress)
		exec(`INSERT INTO ido_v3_proxy_stream(chain_id,proxy_address,contract_version,mode,state,anchor_block,cursor_block,cursor_hash,revision) VALUES(31337,?,'ido_v3_stablecoin','stream','running',110,110,?,1)`, proxy, strings.ToLower(rpc.confirmed.Hash().Hex()))
		receipt, _ := json.Marshal(map[string]any{"RetiredTests": []uint64{997}})
		exec(`INSERT INTO ido_v3_proxy_coverage(chain_id,proxy_address,contract_version,epoch,block_number,block_hash,parent_hash,digest,payload) VALUES(31337,?,'ido_v3_stablecoin',1,110,?,?,REPEAT('a',64),?)`, proxy, strings.ToLower(rpc.confirmed.Hash().Hex()), strings.ToLower(common.HexToHash("0xa1").Hex()), string(receipt))
		var id uint64
		tx.Raw(`SELECT id FROM chain_tx_submission WHERE project_id=997`).Scan(&id)
		blocked := func() {
			t.Helper()
			if tx.Exec(`UPDATE chain_relayer_nonce SET active_submission_id=? WHERE chain_id=31337 AND sender_address=?`, id, row.SenderAddress).Error == nil {
				t.Fatal("SQL trigger admitted unresolved suspension")
			}
		}
		blocked()
		if requireV3SuspendedSenderTx(ctx, tx, rpc, 31337, common.HexToAddress(row.SenderAddress)) == nil {
			t.Fatal("unenrolled archived row bypassed Go guard")
		}
		before := []byte{}
		tx.Raw(`SELECT raw_tx FROM chain_tx_submission WHERE id=?`, id).Scan(&before)
		proof, e := reconcileRetiredNonceTx(ctx, tx, rpc, id, common.HexToHash(*row.TxHash), true, false)
		if e != nil {
			t.Fatal("dry-run", e)
		}
		var count int64
		tx.Raw(`SELECT COUNT(*) FROM ido_v3_retired_nonce_evidence WHERE submission_id=?`, id).Scan(&count)
		if count != 0 {
			t.Fatal("dry-run wrote evidence")
		}
		proof, e = reconcileRetiredNonceTx(ctx, tx, rpc, id, common.HexToHash(*row.TxHash), true, true)
		if e != nil {
			t.Fatal("enrollment", e)
		}
		viper.Set("idoV3RetiredNonce.enabled", false)
		if requireV3SuspendedSenderTx(ctx, tx, rpc, 31337, common.HexToAddress(row.SenderAddress)) == nil {
			t.Fatal("default-off signer accepted exemption")
		}
		viper.Set("idoV3RetiredNonce.enabled", true)
		if requireV3SuspendedSenderTx(ctx, tx, rpc, 31337, common.HexToAddress(row.SenderAddress)) != nil {
			t.Fatal("verified consumed attempt still fenced")
		}
		exec(`UPDATE chain_relayer_nonce SET active_submission_id=? WHERE chain_id=31337 AND sender_address=?`, id, row.SenderAddress)
		exec(`UPDATE chain_relayer_nonce SET active_submission_id=NULL WHERE chain_id=31337 AND sender_address=?`, row.SenderAddress)
		if tx.Exec(`UPDATE ido_v3_retired_nonce_evidence SET nonce=6 WHERE submission_id=?`, id).Error == nil {
			t.Fatal("evidence mutable")
		}
		if tx.Exec(`DELETE FROM ido_v3_retired_nonce_evidence WHERE submission_id=?`, id).Error == nil {
			t.Fatal("evidence deletable")
		}
		exec(`UPDATE ido_v3_retired_nonce_evidence SET state='revoked' WHERE submission_id=? AND state='valid'`, id)
		blocked()
		if requireV3SuspendedSenderTx(ctx, tx, rpc, 31337, common.HexToAddress(row.SenderAddress)) == nil {
			t.Fatal("revocation ignored")
		}
		// Explicit re-enrollment is required after revocation; normal signer cannot.
		rpc.confirmed.Number.SetUint64(111)
		if _, e = reconcileRetiredNonceTx(ctx, tx, rpc, id, common.HexToHash(*row.TxHash), true, true); e != nil {
			t.Fatal(e)
		}
		exec(`UPDATE projects SET status='V3ReorgHalted' WHERE project_id=997`)
		blocked()
		exec(`UPDATE projects SET status='archived' WHERE project_id=997`)
		rpc.reorg = true
		if _, e := auditRetiredNonce(ctx, rpc, row, b, payload); e == nil {
			t.Fatal("maintenance accepted replaced receipt")
		}
		rpc.reorg = false
		// Expiry alone closes SQL admission; only fresh independent audit may
		// renew a previously enrolled record, never extend stale evidence.
		exec(`UPDATE ido_v3_retired_nonce_evidence SET state='revoked' WHERE submission_id=? AND state='valid'`, id)
		exec(`INSERT INTO ido_v3_retired_nonce_evidence(submission_id,chain_id,project_id,sender_address,contract_address,action,nonce,tx_hash,raw_sha256,receipt_block,receipt_hash,journal_id,confirmed_block,confirmed_hash,consumed_nonce,evidence_digest,expires_at,created_at,state) SELECT submission_id,chain_id,project_id,sender_address,contract_address,action,nonce,tx_hash,raw_sha256,receipt_block,receipt_hash,journal_id,confirmed_block,confirmed_hash,consumed_nonce,REPEAT('d',64),UTC_TIMESTAMP(6)-INTERVAL 1 SECOND,UTC_TIMESTAMP(6)-INTERVAL 10 MINUTE,'valid' FROM ido_v3_retired_nonce_evidence WHERE submission_id=? ORDER BY id DESC LIMIT 1`, id)
		blocked()
		rpc.confirmed.Number.SetUint64(112)
		if requireV3SuspendedSenderTx(ctx, tx, nil, 31337, common.HexToAddress(row.SenderAddress)) == nil {
			t.Fatal("signing refreshed expired evidence")
		}
		var retired entity.ChainTxSubmission
		if tx.Where("id=?", id).Take(&retired).Error != nil {
			t.Fatal("retired")
		}
		in, e := readActiveRetirementInput(tx, retired, false)
		if e != nil {
			t.Fatal(e)
		}
		p, e := auditRetiredNonce(ctx, rpc, retired, b, in.Payload)
		if e != nil {
			t.Fatal(e)
		}
		in.Proof = p
		testIdleMaintenanceFences(t, tx, in, row.SenderAddress)
		var idle entity.ChainRelayerNonce
		tx.Where("chain_id=31337 AND sender_address=?", row.SenderAddress).Take(&idle)
		other := *rpc
		window, e := ReadV3SenderMaintenanceWindow(ctx, tx, 31337, common.HexToAddress(row.SenderAddress))
		if e != nil || window.RemainingSeconds == nil || *window.RemainingSeconds >= 0 || window.RetiredCount != 1 {
			t.Fatal("expired scheduling window", window, e)
		}
		out, e := RefreshIdleV3SenderEvidence(ctx, tx, 31337, common.HexToAddress(row.SenderAddress), rpc, &other)
		if e != nil || out.Refreshed != 1 || out.Skipped {
			t.Fatal("independent idle maintenance", out, e)
		}
		if e := requireV3SuspendedSenderTx(ctx, tx, nil, 31337, common.HexToAddress(row.SenderAddress)); e != nil {
			t.Fatal("fresh maintenance not ready", e)
		}
		window, e = ReadV3SenderMaintenanceWindow(ctx, tx, 31337, common.HexToAddress(row.SenderAddress))
		if e != nil || window.RemainingSeconds == nil || *window.RemainingSeconds < 280 {
			t.Fatal("fresh scheduling window", window, e)
		}
		exec(`UPDATE chain_relayer_nonce SET active_submission_id=? WHERE chain_id=31337 AND sender_address=?`, id, row.SenderAddress)
		exec(`UPDATE chain_relayer_nonce SET active_submission_id=NULL WHERE chain_id=31337 AND sender_address=?`, row.SenderAddress)
		var finalState string
		tx.Raw(`SELECT state FROM chain_tx_submission WHERE id=?`, id).Scan(&finalState)
		if finalState != ChainTxStateV3ReorgSuspended {
			t.Fatal("outbox status falsified")
		}
		if proof.Nonce != 7 || proof.ConsumedNonce != 8 {
			t.Fatal(proof)
		}
		testActiveOriginalRefreshMySQL(t, tx, id, b, proof)
		testRetirementMarkerIndexMySQL(t, tx, b.ChainID, b.ProjectID, proxy, id)
		return rollback
	})
	if !errors.Is(e, rollback) {
		t.Fatal(e)
	}
}

func testIdleMaintenanceFences(t *testing.T, tx *gorm.DB, in activeRetirementInput, sender string) {
	t.Helper()
	ctx := context.Background()
	var baseline entity.ChainRelayerNonce
	if tx.Where("chain_id=31337 AND sender_address=?", sender).Take(&baseline).Error != nil {
		t.Fatal("idle fence")
	}
	stop := errors.New("idle maintenance negative rollback")
	for _, kind := range []string{"revoked", "nonce-drift", "new-enrollment"} {
		e := tx.Transaction(func(w *gorm.DB) error {
			switch kind {
			case "revoked":
				if e := w.Exec(`UPDATE ido_v3_retired_nonce_evidence SET state='revoked' WHERE id=?`, in.PreviousID).Error; e != nil {
					return e
				}
			case "nonce-drift":
				if e := w.Exec(`UPDATE chain_relayer_nonce SET next_nonce=next_nonce+1 WHERE chain_id=31337 AND sender_address=?`, sender).Error; e != nil {
					return e
				}
			case "new-enrollment":
				in.PreviousID++
			}
			if commitIdleV3SenderEvidence(ctx, w, 31337, common.HexToAddress(sender), baseline, []activeRetirementInput{in}) == nil {
				t.Fatal("idle refresh ignored concurrent change", kind)
			}
			return stop
		})
		if !errors.Is(e, stop) {
			t.Fatal(e)
		}
	}
}

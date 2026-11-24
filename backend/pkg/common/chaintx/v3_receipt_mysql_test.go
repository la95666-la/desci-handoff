package chaintx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// Exercises real 1017-1021 constraints and ordered reorg/cursor/project/
// submission/sender locks. Only a loopback mock RPC is contacted; synthetic
// signatures and an outcome-unknown send never leave this isolated test.
func TestV3LifecycleReceiptDisposableMySQLFence(t *testing.T) {
	dsn := os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if dsn == "" {
		t.Skip("disposable V3 MySQL DSN is not set")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("disposable database unavailable")
	}
	var name, version string
	if db.Raw("SELECT DATABASE()").Scan(&name).Error != nil || !strings.HasPrefix(name, "jc_v3_audit_") ||
		db.Raw("SELECT VERSION()").Scan(&version).Error != nil || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("refuse non-disposable V3 database")
	}
	for _, tc := range []struct {
		name      string
		projectID uint64
		proxy     string
		key       string
		reverted  bool
	}{
		{"confirmed success holds sender", 91, "0x9191919191919191919191919191919191919191", strings.Repeat("5", 64), false},
		{"confirmed revert releases sender", 92, "0x9292929292929292929292929292929292929292", strings.Repeat("6", 64), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key, err := crypto.HexToECDSA(tc.key)
			if err != nil {
				t.Fatal(err)
			}
			operator := crypto.PubkeyToAddress(key.PublicKey)
			proxy := common.HexToAddress(tc.proxy)
			projectToken := common.HexToAddress("0x1111111111111111111111111111111111111111")
			paymentToken := common.HexToAddress("0x2222222222222222222222222222222222222222")
			binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: tc.projectID, ProxyAddress: proxy,
				ProjectToken: projectToken, PaymentToken: paymentToken}
			data, err := basechain.PackV3Lifecycle("activateProject", new(big.Int).SetUint64(tc.projectID))
			if err != nil {
				t.Fatal(err)
			}
			var sends atomic.Int32
			var pendingDrift atomic.Bool
			rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					ID     json.RawMessage   `json:"id"`
					Method string            `json:"method"`
					Params []json.RawMessage `json:"params"`
				}
				if json.NewDecoder(r.Body).Decode(&request) != nil {
					t.Errorf("invalid local RPC request")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				result := ""
				switch request.Method {
				case "eth_estimateGas":
					result = `"0x186a0"`
				case "eth_gasPrice":
					result = `"0x1"`
				case "eth_getTransactionCount":
					result = `"0x7"`
					if pendingDrift.Load() && len(request.Params) == 2 && string(request.Params[1]) == `"pending"` {
						result = `"0x8"`
					}
				case "eth_sendRawTransaction":
					if len(request.Params) != 1 {
						t.Errorf("unexpected broadcast parameters")
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					var encoded string
					var sent types.Transaction
					if json.Unmarshal(request.Params[0], &encoded) != nil || sent.UnmarshalBinary(common.FromHex(encoded)) != nil ||
						sent.To() == nil || *sent.To() != proxy {
						t.Errorf("wrong original signed transaction")
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					sends.Add(1)
				default:
					t.Errorf("unexpected local RPC method")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if result == "" {
					_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"error":{"code":-32000,"message":"outcome unknown"}}`, request.ID)
				} else {
					_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":%s}`, request.ID, result)
				}
			}))
			defer rpc.Close()
			client, err := basechain.NewIDOV3Client(rpc.URL, key, proxy, big.NewInt(31337))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Client.Close()
			createData, err := basechain.PackV3Create(basechain.V3CreateParams{ProjectID: new(big.Int).SetUint64(tc.projectID),
				ProjectToken: projectToken, PaymentToken: paymentToken, ProjectOwner: operator, FeeReceiver: operator,
				FeeRate: 200, Price: big.NewInt(10), HardCap: big.NewInt(1000), SoftCap: big.NewInt(500),
				MaxPerUser: big.NewInt(900), Start: big.NewInt(1000), End: big.NewInt(2000),
				VestingTimes: []*big.Int{big.NewInt(3000)}, VestingPercents: []*big.Int{big.NewInt(10000)}})
			if err != nil {
				t.Fatal(err)
			}
			createRaw, createHash, err := client.SignLegacyTransaction(6, 150000, big.NewInt(1), proxy, createData)
			if err != nil {
				t.Fatal(err)
			}
			seed := func(query string, args ...any) {
				t.Helper()
				if err := db.Exec(query, args...).Error; err != nil {
					t.Fatal("disposable V3 receipt fixture unavailable")
				}
			}
			seed(`INSERT INTO ido_v3_runtime(chain_id,proxy_address,implementation_address,proxy_code_hash,
				implementation_code_hash,governor_address,start_block) VALUES(31337,?,
				'0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',CONCAT('0x',REPEAT('a',64)),
				CONCAT('0x',REPEAT('b',64)),'0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',1)`, tc.proxy)
			seed(`INSERT INTO projects(project_id,status,funding_mode,contract_version,chain_id,contract_address,
				token_address,payment_token_address,description,tags,team_info,roadmap,partners)
				VALUES(?,'Set','capped_presale','ido_v3_stablecoin',31337,?,?,?,'','','','','')`, tc.projectID,
				tc.proxy, strings.ToLower(projectToken.Hex()), strings.ToLower(paymentToken.Hex()))
			seed(`INSERT INTO ido_v3_project_binding(project_id,chain_id,proxy_address) VALUES(?,31337,?)`, tc.projectID, tc.proxy)
			seed(`INSERT INTO ido_v3_set_create_commitment(project_id,chain_id,proxy_address,request_digest,
				manifest_digest,create_calldata_hash,operator_address) VALUES(?,31337,?,
				CONCAT('0x',REPEAT('1',64)),CONCAT('0x',REPEAT('2',64)),?,?)`,
				tc.projectID, tc.proxy, strings.ToLower(crypto.Keccak256Hash(createData).Hex()), strings.ToLower(operator.Hex()))
			seed(`INSERT INTO chain_tx_submission(project_id,action,attempt_no,chain_id,sender_address,contract_address,
				call_data,state,active_slot,last_error) VALUES(?,'ido_v3_create',1,31337,?, ?,?,'ready',1,'')`,
				tc.projectID, operator.Hex(), tc.proxy, createData)
			seed(`UPDATE chain_tx_submission SET state='signed',nonce=6,gas_limit=150000,gas_price_raw='1',
				raw_tx=?,tx_hash=?,v3_signed_at=UTC_TIMESTAMP(6) WHERE project_id=? AND action='ido_v3_create' AND state='ready'`,
				createRaw, createHash, tc.projectID)
			seed(`UPDATE projects SET status='Pending',onchain_locked=1 WHERE project_id=? AND status='Set'`, tc.projectID)
			seed(`INSERT INTO ido_v3_event_journal(chain_id,proxy_address,project_id,block_number,block_hash,
				tx_hash,log_index,event_kind,payload) VALUES(31337,?,?,89,?,?,0,'created','{}')`,
				tc.proxy, tc.projectID, "0x"+strings.Repeat("a", 64), strings.ToLower(createHash))
			var createEventID uint64
			if db.Raw(`SELECT id FROM ido_v3_event_journal WHERE chain_id=31337 AND proxy_address=? AND project_id=?
				AND event_kind='created'`, tc.proxy, tc.projectID).Scan(&createEventID).Error != nil || createEventID == 0 {
				t.Fatal("disposable canonical creation fixture unavailable")
			}
			seed(`UPDATE chain_tx_submission SET state='applied',active_slot=NULL,receipt_status=1,
				receipt_block_number=89,receipt_block_hash=?,final_event_record_id=?
				WHERE project_id=? AND action='ido_v3_create' AND state='signed'`, "0x"+strings.Repeat("a", 64), createEventID, tc.projectID)
			cursorHash := "0x" + strings.Repeat("c", 64)
			seed(`INSERT INTO ido_v3_scan_cursor(chain_id,proxy_address,project_id,cursor_block,cursor_hash,revision,state)
				VALUES(31337,?,?,90,?,1,'running')`, tc.proxy, tc.projectID, cursorHash)
			seed(`INSERT INTO ido_v3_block_checkpoint(chain_id,proxy_address,project_id,block_number,block_hash)
				VALUES(31337,?,?,90,?)`, tc.proxy, tc.projectID, cursorHash)
			plan := basechain.V3TransportPreview{Transport: basechain.V3RelayerTransport, To: proxy,
				From: operator, CallData: data}
			var intentID uint64
			if err := db.Transaction(func(tx *gorm.DB) error {
				if err := LockV3RunningCursorTx(tx, binding); err != nil {
					return err
				}
				var err error
				intentID, err = ReserveV3OperatorIntentTx(tx, binding, plan, "ido_v3_activate")
				if err != nil {
					return err
				}
				return ReadyV3IntentTx(tx, binding, operator, intentID)
			}); err != nil || intentID == 0 {
				t.Fatal("typed intent and ready transition unavailable")
			}
			seed(`UPDATE ido_v3_scan_cursor SET state='reorg_halted' WHERE chain_id=31337 AND proxy_address=? AND project_id=?`, tc.proxy, tc.projectID)
			if signed, err := PrepareV3Once(context.Background(), db, client, binding, operator, intentID, 0); err == nil || signed != nil {
				t.Fatal("halted cursor allocated a V3 nonce")
			}
			if err := db.Transaction(func(tx *gorm.DB) error {
				return CancelUnsignedV3IntentTx(tx, binding, operator, intentID)
			}); err != nil {
				t.Fatal("halted cursor stranded unsigned ready intent")
			}
			var cancelled entity.ChainTxSubmission
			if db.Where("id=?", intentID).Take(&cancelled).Error != nil ||
				cancelled.State != constant.ChainTxStateCancelled || cancelled.ActiveSlot != nil {
				t.Fatal("halted cursor cleanup changed signed or active slot")
			}
			seed(`UPDATE ido_v3_scan_cursor SET state='running' WHERE chain_id=31337 AND proxy_address=? AND project_id=?`, tc.proxy, tc.projectID)
			if err := db.Transaction(func(tx *gorm.DB) error {
				if err := LockV3RunningCursorTx(tx, binding); err != nil {
					return err
				}
				var err error
				intentID, err = ReserveV3OperatorIntentTx(tx, binding, plan, "ido_v3_activate")
				if err != nil {
					return err
				}
				return ReadyV3IntentTx(tx, binding, operator, intentID)
			}); err != nil || intentID == cancelled.ID {
				t.Fatal("halted cleanup prevented controlled reservation")
			}
			pendingDrift.Store(true)
			if signed, err := PrepareV3Once(context.Background(), db, client, binding, operator, intentID, 0); err == nil || signed != nil {
				t.Fatal("sender pending nonce drift allocated a V3 signature")
			}
			pendingDrift.Store(false)
			var unsigned entity.ChainTxSubmission
			if db.Where("id=?", intentID).Take(&unsigned).Error != nil || unsigned.State != constant.ChainTxStateReady ||
				unsigned.Nonce != nil || unsigned.TxHash != nil || unsigned.V3SignedAt != nil {
				t.Fatal("failed signing preflight mutated durable outbox")
			}
			// A crashed Core left this ready row older than its request
			// deadline. Job may cancel it only after the DB-local age fence.
			if err := db.Transaction(func(tx *gorm.DB) error {
				return CancelStaleUnsignedV3IntentTx(tx, binding, operator, intentID)
			}); err == nil {
				t.Fatal("fresh ready attempt passed crash cleanup")
			}
			seed(`UPDATE chain_tx_submission SET updated_at=CURRENT_TIMESTAMP(6)-INTERVAL 6 MINUTE
				WHERE id=? AND state='ready'`, intentID)
			if err := db.Transaction(func(tx *gorm.DB) error {
				return CancelStaleUnsignedV3IntentTx(tx, binding, operator, intentID)
			}); err != nil {
				t.Fatal("stale ready attempt could not be cancelled under cursor/project locks")
			}
			if db.Where("id=?", intentID).Take(&unsigned).Error != nil ||
				unsigned.State != constant.ChainTxStateCancelled || unsigned.ActiveSlot != nil {
				t.Fatal("unsigned cancellation left active business slot")
			}
			if err := db.Transaction(func(tx *gorm.DB) error {
				if err := LockV3RunningCursorTx(tx, binding); err != nil {
					return err
				}
				var err error
				intentID, err = ReserveV3OperatorIntentTx(tx, binding, plan, "ido_v3_activate")
				if err != nil {
					return err
				}
				return ReadyV3IntentTx(tx, binding, operator, intentID)
			}); err != nil || intentID == unsigned.ID {
				t.Fatalf("cancelled unsigned attempt prevented new controlled reservation: %v", err)
			}
			// The pinned Core admission is separate; exercise the actual
			// ready->signed nonce path against the local RPC and MySQL.
			row, err := PrepareV3Once(context.Background(), db, client, binding, operator, intentID, 0)
			if err != nil || row == nil || row.State != constant.ChainTxStateSigned || row.TxHash == nil {
				t.Fatal("durable V3 signed attempt unavailable")
			}
			if err := db.Transaction(func(tx *gorm.DB) error {
				return CancelUnsignedV3IntentTx(tx, binding, operator, row.ID)
			}); err == nil {
				t.Fatal("signed original attempt was cancelled by unsigned cleanup")
			}
			hash := *row.TxHash
			// The explicit dry-run exercises the SAME cursor/project/submission/
			// nonce locks, without a marker or network send. Permission revocation
			// and nonce drift must reject before the original can be dispatched.
			if got, e := PreviewV3PersistedFirstBroadcast(context.Background(), db, binding, operator, row.ID, func(*gorm.DB) error { return nil }); e != nil || got != hash || sends.Load() != 0 {
				t.Fatal("dry-run altered/sent", e)
			}
			if _, e := PreviewV3PersistedFirstBroadcast(context.Background(), db, binding, operator, row.ID, func(*gorm.DB) error { return errors.New("permission revoked") }); e == nil {
				t.Fatal("revoked original admitted")
			}
			var before entity.ChainTxSubmission
			if db.Where("id=?", row.ID).Take(&before).Error != nil || before.State != "signed" || before.BroadcastCount != 0 || before.ActiveSlot == nil {
				t.Fatal("preflight changed original")
			}
			var persisted entity.ChainTxSubmission
			if err := db.Where("project_id=? AND action='ido_v3_activate'", tc.projectID).Take(&persisted).Error; err != nil {
				t.Fatal("signed attempt not persisted")
			}
			if got, err := BroadcastV3Once(context.Background(), db, client, binding, operator, row.ID); err == nil || got != hash || sends.Load() != 1 {
				t.Fatalf("unknown broadcast result: error=%t originalHash=%t sends=%d", err != nil, got == hash, sends.Load())
			}
			if _, err := BroadcastV3Once(context.Background(), db, client, binding, operator, row.ID); err == nil || sends.Load() != 1 {
				t.Fatal("process retry resent already signed V3 bytes")
			}
			if db.Where("id=?", row.ID).Take(&row).Error != nil || row.State != constant.ChainTxStateBroadcastUnknown || row.BroadcastCount != 1 {
				t.Fatal("unknown send did not persist one-time marker")
			}
			header := &types.Header{Number: big.NewInt(91), Time: 1}
			status := uint64(types.ReceiptStatusSuccessful)
			if tc.reverted {
				status = types.ReceiptStatusFailed
			}
			receipt := &types.Receipt{TxHash: common.HexToHash(hash), BlockHash: header.Hash(),
				BlockNumber: big.NewInt(91), Status: status, GasUsed: 80000}
			if _, err := AuditV3PersistedReceipt(t.Context(), *row, binding, &v3ReceiptFake{block: 93}, 3); err == nil {
				t.Fatal("unknown original receipt advanced a signed attempt")
			}
			if _, err := AuditV3PersistedReceipt(t.Context(), *row, binding, &v3ReceiptFake{
				receipts: []*types.Receipt{receipt}, head: &types.Header{Number: big.NewInt(91), Time: 2}, block: 93}, 3); err == nil {
				t.Fatal("replaced receipt anchor was accepted")
			}
			audit, err := AuditV3PersistedReceipt(t.Context(), *row, binding, &v3ReceiptFake{
				receipts: []*types.Receipt{receipt}, head: header, block: 93}, 3)
			if err != nil || audit.Reverted != tc.reverted || audit.BlockHash != header.Hash() {
				t.Fatal("original-hash confirmed receipt audit unavailable")
			}
			// A halted cursor must fail before touching the exact attempt or
			// sender. The subsequently restored cursor is a scratch-only fixture.
			seed(`UPDATE ido_v3_scan_cursor SET state='reorg_halted' WHERE chain_id=31337 AND proxy_address=? AND project_id=?`, tc.proxy, tc.projectID)
			if err := db.Transaction(func(tx *gorm.DB) error {
				_, err := finalizeV3AdmittedLifecycleReceiptTx(tx, *row, binding, operator, audit)
				return err
			}); err == nil {
				t.Fatal("halted cursor advanced the original receipt")
			}
			var unchanged entity.ChainTxSubmission
			if db.Where("id=?", row.ID).Take(&unchanged).Error != nil || unchanged.State != constant.ChainTxStateBroadcastUnknown {
				t.Fatal("halted cursor mutated signed attempt")
			}
			seed(`UPDATE ido_v3_scan_cursor SET state='running' WHERE chain_id=31337 AND proxy_address=? AND project_id=?`, tc.proxy, tc.projectID)
			var finalized entity.ChainTxSubmission
			if err := db.Transaction(func(tx *gorm.DB) error {
				var err error
				finalized, err = finalizeV3AdmittedLifecycleReceiptTx(tx, *row, binding, operator, audit)
				return err
			}); err != nil {
				t.Fatal("confirmed original receipt finalization failed")
			}
			var fence entity.ChainRelayerNonce
			if db.Where("chain_id=31337 AND sender_address=?", operator.Hex()).Take(&fence).Error != nil {
				t.Fatal("shared sender fence unavailable")
			}
			if tc.reverted {
				if finalized.State != constant.ChainTxStateFinalRevert || finalized.ActiveSlot != nil ||
					finalized.ReceiptStatus == nil || *finalized.ReceiptStatus != 0 || fence.ActiveSubmissionID != nil {
					t.Fatal("confirmed revert did not release only the original sender")
				}
			} else if finalized.State != constant.ChainTxStateFinalSuccess || finalized.ActiveSlot == nil ||
				finalized.ReceiptStatus == nil || *finalized.ReceiptStatus != 1 ||
				fence.ActiveSubmissionID == nil || *fence.ActiveSubmissionID != row.ID {
				t.Fatal("confirmed success prematurely released sender before canonical event")
			}
			if fence.NextNonce != 8 || finalized.ReceiptBlockNumber == nil || *finalized.ReceiptBlockNumber != audit.Block ||
				!strings.EqualFold(finalized.ReceiptBlockHash, audit.BlockHash.Hex()) {
				t.Fatal("original nonce or receipt placement drifted")
			}
			if !tc.reverted {
				// This is not permission from a receipt: the authenticated Job
				// normally inserts this canonical event after full block replay.
				// Here exercise the exact outbox/journal/nonce SQL boundary.
				seed(`INSERT INTO ido_v3_event_journal(chain_id,proxy_address,project_id,block_number,block_hash,
					tx_hash,log_index,event_kind,payload) VALUES(31337,?,?,91,?,?,0,'ProjectActivated','{}')`,
					tc.proxy, tc.projectID, strings.ToLower(audit.BlockHash.Hex()), strings.ToLower(hash))
				var journalID uint64
				if db.Raw(`SELECT id FROM ido_v3_event_journal WHERE chain_id=31337 AND proxy_address=?
					AND project_id=? AND tx_hash=?`, tc.proxy, tc.projectID, strings.ToLower(hash)).Scan(&journalID).Error != nil || journalID == 0 {
					t.Fatal("canonical journal fixture unavailable")
				}
				if err := db.Transaction(func(tx *gorm.DB) error {
					return ApplyV3CanonicalEventTx(tx, binding, "ido_v3_activate", common.HexToHash(hash),
						audit.BlockHash, audit.Block, journalID, true)
				}); err != nil {
					t.Fatal("exact canonical event did not apply original attempt")
				}
				if db.Where("id=?", row.ID).Take(&finalized).Error != nil ||
					db.Where("chain_id=31337 AND sender_address=?", operator.Hex()).Take(&fence).Error != nil ||
					finalized.State != constant.ChainTxStateApplied || finalized.ActiveSlot != nil ||
					finalized.FinalEventRecordID == nil || *finalized.FinalEventRecordID != journalID || fence.ActiveSubmissionID != nil {
					t.Fatal("canonical event failed to release only exact sender and journal")
				}
			}
		})
	}
}

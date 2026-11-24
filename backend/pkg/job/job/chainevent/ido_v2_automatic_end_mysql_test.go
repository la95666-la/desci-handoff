package chainevent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	baseChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

// JC_IDO_END_TEST_DSN must point to a disposable, isolated MySQL 8 schema.
// No production or shared-test database is suitable: this test owns its schema.
func TestIDOV2EndRealMySQLIntentAndRecovery(t *testing.T) {
	dsn := os.Getenv("JC_IDO_END_TEST_DSN")
	if dsn == "" {
		t.Skip("set JC_IDO_END_TEST_DSN to a disposable isolated MySQL database")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var schemaName string
	if err := database.Raw("SELECT DATABASE()").Scan(&schemaName).Error; err != nil || schemaName != "jc_ido_end_it" {
		t.Fatalf("refusing non-disposable schema %q: %v", schemaName, err)
	}
	var existingTables int64
	if err := database.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE()").Scan(&existingTables).Error; err != nil || existingTables != 0 {
		t.Fatalf("refusing populated schema: tables=%d err=%v", existingTables, err)
	}
	for _, ddl := range []string{
		"CREATE TABLE chain_event_sync_state (id BIGINT UNSIGNED PRIMARY KEY) ENGINE=InnoDB",
		"CREATE TABLE chain_event_reorg_incident (id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT, chain_id BIGINT UNSIGNED NOT NULL, active_slot TINYINT UNSIGNED NULL, status VARCHAR(32) NOT NULL) ENGINE=InnoDB",
		"CREATE TABLE projects (project_id BIGINT UNSIGNED PRIMARY KEY, status VARCHAR(50) NOT NULL, onchain_locked BOOLEAN NOT NULL, funding_mode VARCHAR(32) NOT NULL, contract_version VARCHAR(32) NOT NULL, chain_id BIGINT UNSIGNED NOT NULL, contract_address VARCHAR(42) NOT NULL, end_time DATETIME(6) NOT NULL) ENGINE=InnoDB",
	} {
		if err := database.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := database.Exec(`CREATE TABLE chain_tx_submission (
		id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT, project_id BIGINT UNSIGNED NOT NULL,
		action VARCHAR(32) NOT NULL, attempt_no INT UNSIGNED NOT NULL, chain_id BIGINT UNSIGNED NOT NULL,
		sender_address VARCHAR(42) NOT NULL, contract_address VARCHAR(42) NOT NULL,
		call_data MEDIUMBLOB NOT NULL, value_raw VARCHAR(78) NOT NULL DEFAULT '0',
		state VARCHAR(32) NOT NULL, nonce BIGINT UNSIGNED NULL, gas_limit BIGINT UNSIGNED NULL,
		unsigned_tx_digest VARCHAR(66) NOT NULL DEFAULT '', signing_digest VARCHAR(66) NOT NULL DEFAULT '',
		replacement_generation INT UNSIGNED NOT NULL DEFAULT 0, signing_reserved_at DATETIME(6) NULL,
		signing_recovery_reason TEXT NOT NULL,
		gas_price_raw VARCHAR(78) NOT NULL DEFAULT '0', raw_tx MEDIUMBLOB NULL, tx_hash VARCHAR(66) NULL,
		active_slot TINYINT UNSIGNED NULL, broadcast_count INT UNSIGNED NOT NULL DEFAULT 0,
		last_broadcast_at DATETIME(6) NULL, last_error TEXT NOT NULL, receipt_status BIGINT UNSIGNED NULL,
		receipt_block_number BIGINT UNSIGNED NULL, receipt_block_hash VARCHAR(66) NOT NULL DEFAULT '',
		receipt_gas_used BIGINT UNSIGNED NULL, final_event_record_id BIGINT UNSIGNED NULL,
		finalized_at DATETIME(6) NULL, cancelled_by_admin_id BIGINT UNSIGNED NULL,
		cancel_reason VARCHAR(500) NOT NULL DEFAULT '', cancelled_at DATETIME(6) NULL,
		created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
		updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
		UNIQUE KEY uk_chain_tx_active_business (chain_id,contract_address,project_id,action,active_slot),
		UNIQUE KEY uk_chain_tx_business_attempt (chain_id,contract_address,project_id,action,attempt_no)
	) ENGINE=InnoDB`).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Exec(`CREATE TABLE chain_relayer_nonce (
		chain_id BIGINT UNSIGNED NOT NULL, sender_address VARCHAR(42) NOT NULL,
		next_nonce BIGINT UNSIGNED NOT NULL, active_submission_id BIGINT UNSIGNED NULL,
		created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
		updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
		PRIMARY KEY (chain_id,sender_address)
	) ENGINE=InnoDB`).Error; err != nil {
		t.Fatal(err)
	}
	var broadcasts []common.Hash
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode mock RPC: %v", err)
			return
		}
		var result any
		switch req.Method {
		case "eth_estimateGas":
			result = "0x186a0"
		case "eth_gasPrice":
			result = "0xb2d05e00"
		case "eth_getTransactionCount":
			result = "0x0"
		case "eth_getTransactionReceipt":
			result = nil
		case "eth_sendRawTransaction":
			if len(req.Params) != 1 {
				t.Errorf("invalid broadcast parameters")
				return
			}
			var raw string
			if err := json.Unmarshal(req.Params[0], &raw); err != nil {
				t.Errorf("decode raw tx: %v", err)
				return
			}
			var signed types.Transaction
			if err := signed.UnmarshalBinary(common.FromHex(raw)); err != nil {
				t.Errorf("invalid broadcast raw tx: %v", err)
				return
			}
			broadcasts = append(broadcasts, signed.Hash())
			result = signed.Hash().Hex()
		default:
			t.Errorf("unexpected mock RPC method %s", req.Method)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, mustJSONForTest(t, result))
	}))
	defer rpc.Close()
	contract := common.HexToAddress("0x1111111111111111111111111111111111111111")
	key, err := crypto.HexToECDSA(strings.Repeat("1", 64))
	if err != nil {
		t.Fatal(err)
	}
	client, err := baseChain.NewIDOV2Client(rpc.URL, key, contract, big.NewInt(56))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Client.Close()
	jobSigner := client.Address
	coreSigner := common.HexToAddress("0x3333333333333333333333333333333333333333")
	now := time.Now().UTC()
	if err := database.Exec("INSERT INTO chain_event_sync_state (id) VALUES (1)").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Exec("INSERT INTO projects (project_id,status,onchain_locked,funding_mode,contract_version,chain_id,contract_address,end_time) VALUES (9,?,?,?,?,?,?,?)", constant.ProjectStatusActive, true, constant.FundingModeCappedPresale, constant.ContractVersionV2Stablecoin, 56, contract.Hex(), now.Add(-time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	data := []byte{0xde, 0xad, 0xbe, 0xef}
	reserve := func() (*entity.ChainTxSubmission, error) {
		var row *entity.ChainTxSubmission
		err := database.Transaction(func(tx *gorm.DB) error {
			var reserveErr error
			row, reserveErr = reserveIDOV2EndTx(tx, 9, client, data, now)
			return reserveErr
		})
		if err != nil {
			return nil, err
		}
		return row, nil
	}
	if err := database.Exec("INSERT INTO chain_event_reorg_incident (chain_id,active_slot,status) VALUES (56,1,'detected')").Error; err != nil {
		t.Fatal(err)
	}
	if row, err := reserve(); err == nil || row != nil {
		t.Fatalf("active reorg admitted reservation: row=%+v err=%v", row, err)
	}
	var count int64
	if err := database.Model(&entity.ChainTxSubmission{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("reorg produced outbox write: count=%d err=%v", count, err)
	}
	if err := database.Exec("DELETE FROM chain_event_reorg_incident").Error; err != nil {
		t.Fatal(err)
	}
	// Core reserves the business action first; Job must not consume its slot.
	var core *entity.ChainTxSubmission
	if err := database.Transaction(func(tx *gorm.DB) error {
		var createErr error
		core, createErr = chaintx.CreateIntentTx(tx, chaintx.Intent{ProjectID: 9, Action: constant.TransactionTypeProjectEnd, ChainID: 56, SenderAddress: coreSigner, ContractAddress: contract, CallData: data})
		return createErr
	}); err != nil {
		t.Fatal(err)
	}
	if row, err := reserve(); err == nil || !strings.Contains(err.Error(), "active chain transaction intent does not match") || row != nil {
		t.Fatalf("Core-owned slot admitted Job signer: row=%+v err=%v", row, err)
	}
	if err := database.Transaction(func(tx *gorm.DB) error { return chaintx.CancelIntentTx(tx, core.ID) }); err != nil {
		t.Fatal(err)
	}
	first, err := reserve()
	if err != nil || first == nil || first.State != constant.ChainTxStateReady || first.ActiveSlot == nil || *first.ActiveSlot != 1 || first.SenderAddress != jobSigner.Hex() || !bytes.Equal(first.CallData, data) {
		t.Fatalf("first Job reservation invalid: row=%+v err=%v", first, err)
	}
	second, err := reserve()
	if err != nil || second == nil || first.ID != second.ID {
		t.Fatalf("retry did not reuse active Job slot: first=%+v second=%+v err=%v", first, second, err)
	}
	if err := database.Model(&entity.ChainTxSubmission{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("expected cancelled Core + ready Job: count=%d err=%v", count, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	submitted, err := chaintx.Dispatch(ctx, database, client, first.ID)
	if err != nil || submitted == nil || submitted.State != constant.ChainTxStateSubmitted || submitted.TxHash == nil || submitted.Nonce == nil || *submitted.Nonce != 0 || len(submitted.RawTx) == 0 {
		t.Fatalf("sign and broadcast did not persist exact nonce-zero transaction: row=%+v err=%v", submitted, err)
	}
	var nonce entity.ChainRelayerNonce
	if err := database.Where("chain_id=? AND sender_address=?", 56, jobSigner.Hex()).Take(&nonce).Error; err != nil || nonce.NextNonce != 1 || nonce.ActiveSubmissionID == nil || *nonce.ActiveSubmissionID != first.ID {
		t.Fatalf("Job signer nonce fence was not persisted: %+v err=%v", nonce, err)
	}
	reconciled, err := chaintx.ReconcileReceipt(ctx, database, client, first.ID, 0)
	if err != nil || reconciled == nil || reconciled.State != constant.ChainTxStateSubmitted {
		t.Fatalf("missing receipt must retain submitted state: row=%+v err=%v", reconciled, err)
	}
	rebroadcast, err := chaintx.Dispatch(ctx, database, client, first.ID)
	if err != nil || rebroadcast == nil || rebroadcast.State != constant.ChainTxStateSubmitted || len(broadcasts) != 2 || broadcasts[0] != broadcasts[1] || !strings.EqualFold(broadcasts[0].Hex(), *submitted.TxHash) || !bytes.Equal(submitted.RawTx, rebroadcast.RawTx) || rebroadcast.BroadcastCount != 2 {
		t.Fatalf("retry must use exact persisted raw transaction: row=%+v hashes=%v err=%v", rebroadcast, broadcasts, err)
	}
	if err := database.Where("chain_id=? AND sender_address=?", 56, jobSigner.Hex()).Take(&nonce).Error; err != nil || nonce.NextNonce != 1 {
		t.Fatalf("rebroadcast allocated another nonce: %+v err=%v", nonce, err)
	}
	// The V2 event consumer resolves the exact transaction hash, then finalizes
	// the outbox and releases the Job signer fence in the same DB transaction.
	if err := database.Transaction(func(tx *gorm.DB) error {
		matched, findErr := chaintx.FindEventSubmissionTx(tx, 9, constant.TransactionTypeProjectEnd, *submitted.TxHash)
		if findErr != nil {
			return findErr
		}
		if matched == nil || matched.ID != first.ID {
			return fmt.Errorf("event did not match Job submission: %+v", matched)
		}
		return chaintx.MarkAppliedTx(tx, matched.ID, 17)
	}); err != nil {
		t.Fatalf("event finalization failed: %v", err)
	}
	var applied entity.ChainTxSubmission
	if err := database.Where("id=?", first.ID).Take(&applied).Error; err != nil || applied.State != constant.ChainTxStateApplied || applied.ActiveSlot != nil || applied.FinalEventRecordID == nil || *applied.FinalEventRecordID != 17 {
		t.Fatalf("Job end intent did not finalize: %+v err=%v", applied, err)
	}
	if err := database.Where("chain_id=? AND sender_address=?", 56, jobSigner.Hex()).Take(&nonce).Error; err != nil || nonce.NextNonce != 1 || nonce.ActiveSubmissionID != nil {
		t.Fatalf("event finalization did not release Job signer: %+v err=%v", nonce, err)
	}
}

func mustJSONForTest(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

package chainevent

import (
	"context"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	baseChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

// Disposable fixture created by scripts/local/idoV2JobEndFixture.js on a
// fresh local Hardhat process. This test never reaches a public RPC.
func TestIDOV2EndRealHardhatAndMySQL(t *testing.T) {
	dsn, fixturePath := os.Getenv("JC_IDO_END_TEST_DSN"), os.Getenv("JC_IDO_END_FIXTURE")
	if dsn == "" || fixturePath == "" {
		t.Skip("requires disposable jc_ido_end_it MySQL and localhost Hardhat fixture")
	}
	var fixture struct {
		ChainID      uint64 `json:"chainId"`
		Contract     string `json:"contract"`
		ProjectID    int    `json:"projectId"`
		EndTime      int64  `json:"endTime"`
		PaymentToken string `json:"paymentToken"`
		JobKey       string `json:"jobKey"`
	}
	manifest, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(manifest, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.ChainID != 31337 || fixture.ProjectID != 9 || !common.IsHexAddress(fixture.Contract) || !common.IsHexAddress(fixture.PaymentToken) || fixture.EndTime <= 0 {
		t.Fatal("invalid disposable Hardhat fixture identity")
	}
	key, err := crypto.HexToECDSA(strings.TrimPrefix(fixture.JobKey, "0x"))
	if err != nil {
		t.Fatal(err)
	}
	contract := common.HexToAddress(fixture.Contract)
	client, err := baseChain.NewIDOV2Client("http://127.0.0.1:18546", key, contract, big.NewInt(31337))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	chainID, err := client.Client.ChainID(ctx)
	if err != nil || chainID.Uint64() != fixture.ChainID {
		t.Fatalf("wrong local Hardhat chain: %v %v", chainID, err)
	}
	code, err := client.Client.CodeAt(ctx, contract, nil)
	if err != nil || len(code) == 0 {
		t.Fatalf("disposable V2 contract missing: %v", err)
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var schema string
	if err := database.Raw("SELECT DATABASE()").Scan(&schema).Error; err != nil || schema != "jc_ido_end_it" {
		t.Fatalf("unsafe schema %q: %v", schema, err)
	}
	var tableCount int64
	if err := database.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE()").Scan(&tableCount).Error; err != nil || tableCount != 0 {
		t.Fatalf("schema is not empty: %d %v", tableCount, err)
	}
	for _, ddl := range []string{
		"CREATE TABLE chain_event_sync_state (id BIGINT UNSIGNED PRIMARY KEY) ENGINE=InnoDB",
		"CREATE TABLE chain_event_reorg_incident (id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT, chain_id BIGINT UNSIGNED NOT NULL, active_slot TINYINT UNSIGNED NULL, status VARCHAR(32) NOT NULL) ENGINE=InnoDB",
		"CREATE TABLE chain_event_record (id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT, event VARCHAR(50) NOT NULL, block_number BIGINT UNSIGNED NOT NULL, block_hash VARCHAR(66) NOT NULL, tx_hash VARCHAR(66) NOT NULL, log_index INT UNSIGNED NOT NULL, contract_address VARCHAR(42) NOT NULL, contract_version VARCHAR(32) NOT NULL, data TEXT NOT NULL, created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), UNIQUE KEY uk_chain_event_identity (block_number,tx_hash,log_index)) ENGINE=InnoDB",
		"CREATE TABLE chain_event_failure (id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT, block_number BIGINT UNSIGNED NOT NULL, tx_hash VARCHAR(66) NOT NULL, log_index INT UNSIGNED NOT NULL, status VARCHAR(20) NOT NULL, consecutive_count INT UNSIGNED NOT NULL DEFAULT 0, resolved_at DATETIME(6) NULL, updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)) ENGINE=InnoDB",
		"CREATE TABLE projects (project_id BIGINT UNSIGNED PRIMARY KEY, status VARCHAR(50) NOT NULL, onchain_locked BOOLEAN NOT NULL, funding_mode VARCHAR(32) NOT NULL, contract_version VARCHAR(32) NOT NULL, chain_id BIGINT UNSIGNED NOT NULL, contract_address VARCHAR(42) NOT NULL, payment_token_address VARCHAR(42) NOT NULL, total_raised_raw VARCHAR(78) NOT NULL, end_time DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)) ENGINE=InnoDB",
		`CREATE TABLE chain_tx_submission (id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT, project_id BIGINT UNSIGNED NOT NULL, action VARCHAR(32) NOT NULL, attempt_no INT UNSIGNED NOT NULL, chain_id BIGINT UNSIGNED NOT NULL, sender_address VARCHAR(42) NOT NULL, contract_address VARCHAR(42) NOT NULL, call_data MEDIUMBLOB NOT NULL, value_raw VARCHAR(78) NOT NULL DEFAULT '0', state VARCHAR(32) NOT NULL, nonce BIGINT UNSIGNED NULL, gas_limit BIGINT UNSIGNED NULL, unsigned_tx_digest VARCHAR(66) NOT NULL DEFAULT '', signing_digest VARCHAR(66) NOT NULL DEFAULT '', replacement_generation INT UNSIGNED NOT NULL DEFAULT 0, signing_reserved_at DATETIME(6) NULL, signing_recovery_reason TEXT NOT NULL, gas_price_raw VARCHAR(78) NOT NULL DEFAULT '0', raw_tx MEDIUMBLOB NULL, tx_hash VARCHAR(66) NULL, active_slot TINYINT UNSIGNED NULL, broadcast_count INT UNSIGNED NOT NULL DEFAULT 0, last_broadcast_at DATETIME(6) NULL, last_error TEXT NOT NULL, receipt_status BIGINT UNSIGNED NULL, receipt_block_number BIGINT UNSIGNED NULL, receipt_block_hash VARCHAR(66) NOT NULL DEFAULT '', receipt_gas_used BIGINT UNSIGNED NULL, final_event_record_id BIGINT UNSIGNED NULL, finalized_at DATETIME(6) NULL, cancelled_by_admin_id BIGINT UNSIGNED NULL, cancel_reason VARCHAR(500) NOT NULL DEFAULT '', cancelled_at DATETIME(6) NULL, created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6), UNIQUE KEY uk_chain_tx_active_business (chain_id,contract_address,project_id,action,active_slot), UNIQUE KEY uk_chain_tx_business_attempt (chain_id,contract_address,project_id,action,attempt_no)) ENGINE=InnoDB`,
		"CREATE TABLE chain_relayer_nonce (chain_id BIGINT UNSIGNED NOT NULL, sender_address VARCHAR(42) NOT NULL, next_nonce BIGINT UNSIGNED NOT NULL, active_submission_id BIGINT UNSIGNED NULL, created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6), PRIMARY KEY (chain_id,sender_address)) ENGINE=InnoDB",
	} {
		if err := database.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := database.Exec("INSERT INTO chain_event_sync_state(id) VALUES(1)").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Exec("INSERT INTO projects(project_id,status,onchain_locked,funding_mode,contract_version,chain_id,contract_address,payment_token_address,total_raised_raw,end_time) VALUES(?,?,?,?,?,?,?,?,?,?)", fixture.ProjectID, constant.ProjectStatusActive, true, constant.FundingModeCappedPresale, constant.ContractVersionV2Stablecoin, fixture.ChainID, contract.Hex(), fixture.PaymentToken, "0", time.Unix(fixture.EndTime, 0)).Error; err != nil {
		t.Fatal(err)
	}
	data, err := client.PackEndProject(big.NewInt(int64(fixture.ProjectID)))
	if err != nil {
		t.Fatal(err)
	}
	// Hardhat can advance block time without advancing the host wall clock.
	// The fixture's chain is already past endTime; use its observed timestamp.
	head, err := client.Client.HeaderByNumber(ctx, nil)
	if err != nil || head.Time <= uint64(fixture.EndTime) {
		t.Fatalf("fixture chain is not due: header=%+v err=%v", head, err)
	}
	var row *entity.ChainTxSubmission
	if err := database.Transaction(func(tx *gorm.DB) error {
		var reserveErr error
		row, reserveErr = reserveIDOV2EndTx(tx, fixture.ProjectID, client, data, time.Unix(int64(head.Time), 0).UTC())
		return reserveErr
	}); err != nil {
		t.Fatal(err)
	}
	if row == nil || row.State != constant.ChainTxStateReady {
		t.Fatalf("reservation failed: %+v", row)
	}
	row, err = chaintx.Dispatch(ctx, database, client, row.ID)
	if err != nil || row == nil || row.TxHash == nil || row.State != constant.ChainTxStateSubmitted {
		t.Fatalf("Hardhat broadcast failed: %+v %v", row, err)
	}
	receipt, err := client.Client.TransactionReceipt(ctx, common.HexToHash(*row.TxHash))
	if err != nil || receipt.Status != 1 || receipt.BlockNumber == nil {
		t.Fatalf("on-chain V2 end failed: %+v %v", receipt, err)
	}
	logs, err := client.Client.FilterLogs(ctx, ethereum.FilterQuery{FromBlock: receipt.BlockNumber, ToBlock: receipt.BlockNumber, Addresses: []common.Address{contract}})
	if err != nil {
		t.Fatal(err)
	}
	var ended *baseChain.V2Event
	for _, info := range logs {
		if info.TxHash != receipt.TxHash {
			continue
		}
		decoded, decodeErr := baseChain.DecodeV2Log(info)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if decoded != nil && decoded.Event == baseChain.V2ProjectEnded && decoded.ProjectID == uint64(fixture.ProjectID) && decoded.Status == 3 && decoded.TotalRaisedRaw == "0" && decoded.PaymentToken == common.HexToAddress(fixture.PaymentToken) {
			ended = decoded
		}
	}
	if ended == nil {
		t.Fatal("canonical V2 ProjectEnded event was not decoded")
	}
	row, err = chaintx.ReconcileReceipt(ctx, database, client, row.ID, 0)
	if err != nil || row == nil || row.State != constant.ChainTxStateFinalSuccess {
		t.Fatalf("Hardhat receipt finality failed: %+v %v", row, err)
	}
	consumer := &GetChainEventJob{}
	for _, tc := range []struct {
		name   string
		mutate func(*baseChain.V2Event)
	}{
		{"raised amount mismatch", func(event *baseChain.V2Event) { event.TotalRaisedRaw = "1" }},
		{"payment token mismatch", func(event *baseChain.V2Event) {
			event.PaymentToken = common.HexToAddress("0x2222222222222222222222222222222222222222")
		}},
		{"source chain mismatch", func(event *baseChain.V2Event) {}},
		{"source contract mismatch", func(event *baseChain.V2Event) {
			event.ContractAddress = common.HexToAddress("0x2222222222222222222222222222222222222222")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := *ended
			tc.mutate(&bad)
			expectedChain := fixture.ChainID
			if tc.name == "source chain mismatch" {
				expectedChain++
			}
			if err := database.Transaction(func(tx *gorm.DB) error { return consumer.processV2LogWithSourceTx(tx, &bad, expectedChain, contract) }); err == nil {
				t.Fatal("inconsistent canonical event was accepted")
			}
			var current entity.ChainTxSubmission
			var project entity.Project
			var eventCount int64
			var nonce entity.ChainRelayerNonce
			if err := database.Where("id=?", row.ID).Take(&current).Error; err != nil {
				t.Fatal(err)
			}
			if err := database.Where("project_id=?", fixture.ProjectID).Take(&project).Error; err != nil {
				t.Fatal(err)
			}
			if err := database.Model(&entity.ChainEventRecord{}).Count(&eventCount).Error; err != nil {
				t.Fatal(err)
			}
			if err := database.Where("chain_id=? AND sender_address=?", fixture.ChainID, client.Address.Hex()).Take(&nonce).Error; err != nil {
				t.Fatal(err)
			}
			if current.State != constant.ChainTxStateFinalSuccess || current.FinalEventRecordID != nil || current.ActiveSlot == nil || project.Status != constant.ProjectStatusActive || eventCount != 0 || nonce.ActiveSubmissionID == nil || *nonce.ActiveSubmissionID != row.ID {
				t.Fatalf("rejected event partially mutated state: outbox=%+v project=%s records=%d nonce=%+v", current, project.Status, eventCount, nonce)
			}
		})
	}
	applyEvent := func() error {
		return database.Transaction(func(tx *gorm.DB) error {
			return consumer.processV2LogWithSourceTx(tx, ended, fixture.ChainID, contract)
		})
	}
	// Fail after the status projection but before event/outbox finalization.
	// The transaction must roll the status back and retain the signer fence.
	if err := database.Exec(`CREATE TRIGGER jc_test_reject_end_event BEFORE INSERT ON chain_event_record
		FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='disposable event insert fault'`).Error; err != nil {
		t.Fatal(err)
	}
	if err := applyEvent(); err == nil || !strings.Contains(err.Error(), "disposable event insert fault") {
		t.Fatalf("event insert fault must abort the whole transaction: %v", err)
	}
	if err := database.Exec("DROP TRIGGER jc_test_reject_end_event").Error; err != nil {
		t.Fatal(err)
	}
	var afterFault entity.Project
	var afterFaultSubmission entity.ChainTxSubmission
	var afterFaultNonce entity.ChainRelayerNonce
	var afterFaultRecords int64
	if err := database.Where("project_id=?", fixture.ProjectID).Take(&afterFault).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Where("id=?", row.ID).Take(&afterFaultSubmission).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Where("chain_id=? AND sender_address=?", fixture.ChainID, client.Address.Hex()).Take(&afterFaultNonce).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&entity.ChainEventRecord{}).Count(&afterFaultRecords).Error; err != nil {
		t.Fatal(err)
	}
	if afterFault.Status != constant.ProjectStatusActive || afterFaultSubmission.State != constant.ChainTxStateFinalSuccess || afterFaultSubmission.ActiveSlot == nil || afterFaultSubmission.FinalEventRecordID != nil || afterFaultNonce.ActiveSubmissionID == nil || *afterFaultNonce.ActiveSubmissionID != row.ID || afterFaultRecords != 0 {
		t.Fatalf("event INSERT failure partially committed: project=%s submission=%+v nonce=%+v records=%d", afterFault.Status, afterFaultSubmission, afterFaultNonce, afterFaultRecords)
	}
	if err := applyEvent(); err != nil {
		t.Fatal(err)
	}
	if err := applyEvent(); err != nil {
		t.Fatalf("duplicate canonical event must be idempotent: %v", err)
	}
	var eventCount int64
	if err := database.Model(&entity.ChainEventRecord{}).Count(&eventCount).Error; err != nil || eventCount != 1 {
		t.Fatalf("event consumer did not persist exactly one record: count=%d err=%v", eventCount, err)
	}
	var eventRecord entity.ChainEventRecord
	if err := database.Where("tx_hash=?", receipt.TxHash.Hex()).Take(&eventRecord).Error; err != nil || eventRecord.Event != baseChain.V2ProjectEnded {
		t.Fatalf("canonical event record is incorrect: %+v err=%v", eventRecord, err)
	}
	if err := database.Where("id=?", row.ID).Take(&row).Error; err != nil || row.State != constant.ChainTxStateApplied || row.ActiveSlot != nil {
		t.Fatalf("event not applied: %+v %v", row, err)
	}
	if row.FinalEventRecordID == nil || *row.FinalEventRecordID != eventRecord.ID {
		t.Fatalf("outbox not linked to canonical event record: %+v", row)
	}
	var project entity.Project
	if err := database.Where("project_id=?", fixture.ProjectID).Take(&project).Error; err != nil || project.Status != constant.ProjectStatusFailed {
		t.Fatalf("V2 event did not transition project status: %+v err=%v", project, err)
	}
	var nonce entity.ChainRelayerNonce
	if err := database.Where("chain_id=? AND sender_address=?", fixture.ChainID, client.Address.Hex()).Take(&nonce).Error; err != nil || nonce.ActiveSubmissionID != nil || nonce.NextNonce != 1 {
		t.Fatalf("Job nonce fence not released: %+v %v", nonce, err)
	}
}

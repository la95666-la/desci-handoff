package chainevent

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
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	baseChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	jobredis "joincare_server/pkg/common/database/redis"
	"joincare_server/pkg/common/database/schema"
	"joincare_server/pkg/common/entity"
	commonlog "joincare_server/pkg/common/log"
	"joincare_server/pkg/job/chain"
	jobdb "joincare_server/pkg/job/db"
)

// The runner owns the MySQL schema, Redis namespace and Hardhat process. The
// fixture contains a disposable mnemonic-derived key; never log its contents.
func TestIDOV2FullSchemaHardhatEndAndScanner(t *testing.T) {
	dsn, redisAddress := os.Getenv("JC_IDO_FULL_SCHEMA_DSN"), os.Getenv("JC_IDO_FULL_SCHEMA_REDIS")
	fixturePath := os.Getenv("JC_IDO_FULL_SCHEMA_HARDHAT_FIXTURE")
	if dsn == "" || redisAddress != "127.0.0.1:16379" || fixturePath == "" {
		t.Skip("requires isolated full-schema MySQL, Redis and local Hardhat fixture")
	}
	var fixture struct {
		ChainID                uint64 `json:"chainId"`
		Contract               string `json:"contract"`
		ProjectID              int    `json:"projectId"`
		EndTime                int64  `json:"endTime"`
		PaymentToken           string `json:"paymentToken"`
		USDC                   string `json:"usdc"`
		Owner                  string `json:"owner"`
		Implementation         string `json:"implementation"`
		ImplementationCodeHash string `json:"implementationCodeHash"`
		StartBlock             uint64 `json:"startBlock"`
		JobKey                 string `json:"jobKey"`
	}
	raw, err := os.ReadFile(fixturePath)
	if err != nil || json.Unmarshal(raw, &fixture) != nil {
		t.Fatal("disposable fixture missing or malformed")
	}
	if fixture.ChainID != 31337 || fixture.ProjectID != 19 || fixture.StartBlock < 2 || fixture.EndTime <= 0 ||
		!common.IsHexAddress(fixture.Contract) || !common.IsHexAddress(fixture.Owner) ||
		!common.IsHexAddress(fixture.Implementation) || !common.IsHexAddress(fixture.PaymentToken) ||
		!common.IsHexAddress(fixture.USDC) || len(fixture.ImplementationCodeHash) != 66 || len(fixture.JobKey) != 66 {
		t.Fatal("invalid disposable fixture identity")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var schemaName string
	if err := database.Raw("SELECT DATABASE()").Scan(&schemaName).Error; err != nil || schemaName != "jc_ido_outbox_schema_it" {
		t.Fatalf("unsafe database %q: %v", schemaName, err)
	}
	if err := schema.RequireChainTxSigningMetadata(database); err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClusterClient(&redis.ClusterOptions{Addrs: []string{redisAddress}, MaxRedirects: 0})
	defer rdb.Close()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	// Earlier disposable failure-path tests intentionally leave sync state
	// unhealthy. This test alone owns the fresh success fixture: reset only the
	// scanner singleton inside the guarded one-use schema before first scan.
	if os.Getenv("JC_IDO_FULL_SCHEMA_HARDHAT_STAGE") != "successor" {
		if err := database.Exec(`UPDATE chain_event_sync_state SET initialized=0,cursor_block=0,cursor_block_hash='',v2_registered=0,v2_contract_address='',v2_start_block=0,v2_implementation_address='',v2_implementation_code_hash='',latest_block=0,safe_block=0,lag_blocks=0,last_started_at=NULL,last_success_at=NULL,last_failure_at=NULL,last_error='',lease_token='' WHERE id=1`).Error; err != nil {
			t.Fatal(err)
		}
	}
	previousDB := jobdb.Client
	jobdb.Client = &jobdb.Connection{Mysql: database, Redis: &jobredis.Connection{Client: rdb}}
	t.Cleanup(func() { jobdb.Client = previousDB; viper.Reset() })
	commonlog.Init(os.TempDir(), "ERROR", 1)
	viper.Set("chain.project.v1.enabled", "false")
	viper.Set("chain.project.url", "http://127.0.0.1:18546")
	viper.Set("chain.project.chainID", 31337)
	viper.Set("chain.project.jobPrivate", fixture.JobKey)
	viper.Set("chain.project.v2.contractAddress", fixture.Contract)
	viper.Set("chain.project.v2.startBlock", fmt.Sprint(fixture.StartBlock))
	viper.Set("chain.project.v2.implementationAddress", fixture.Implementation)
	viper.Set("chain.project.v2.implementationCodeHash", fixture.ImplementationCodeHash)
	viper.Set("chain.project.v2.coreOperatorAddress", fixture.Owner)
	viper.Set("chain.project.v2.paymentTokens.USDT.address", fixture.PaymentToken)
	viper.Set("chain.project.v2.paymentTokens.USDT.decimals", 6)
	viper.Set("chain.project.v2.paymentTokens.USDC.address", fixture.USDC)
	viper.Set("chain.project.v2.paymentTokens.USDC.decimals", 18)
	if err := chain.NewClient(); err != nil {
		t.Fatalf("local V2 runtime admission: %v", err)
	}
	client := chain.GetClient()
	defer client.Client.Close()
	if os.Getenv("JC_IDO_FULL_SCHEMA_HARDHAT_STAGE") == "successor" {
		var before entity.ChainEventSyncState
		if err := database.Where("id=1").Take(&before).Error; err != nil {
			t.Fatal(err)
		}
		if !before.Initialized || before.LastSuccessAt == nil || before.LastError != "" {
			t.Fatalf("predecessor scanner was not healthy: %+v", before)
		}
		(&GetChainEventJob{}).Run()
		var after entity.ChainEventSyncState
		if err := database.Where("id=1").Take(&after).Error; err != nil {
			t.Fatal(err)
		}
		if after.LeaseToken == before.LeaseToken || after.CursorBlock != before.CursorBlock || after.LastSuccessAt == nil || after.LastError != "" {
			t.Fatalf("independent successor lost canonical cursor: before=%+v after=%+v", before, after)
		}
		var records int64
		if err := database.Model(&entity.ChainEventRecord{}).Where("event = ? AND contract_address = ?", baseChain.V2ProjectEnded, fixture.Contract).Count(&records).Error; err != nil || records != 1 {
			t.Fatalf("independent successor duplicated event: count=%d err=%v", records, err)
		}
		var applied entity.ChainTxSubmission
		if err := database.Where("project_id=? AND action=?", fixture.ProjectID, constant.TransactionTypeProjectEnd).Take(&applied).Error; err != nil || applied.State != constant.ChainTxStateApplied || applied.FinalEventRecordID == nil {
			t.Fatalf("outbox changed in successor: state=%s err=%v", applied.State, err)
		}
		return
	}
	if err := database.Exec(`INSERT INTO projects (project_id,owner_address,name_en,name_cn,description,tags,team_info,roadmap,partners,status,funding_mode,contract_version,onchain_locked,chain_id,contract_address,payment_token_address,total_raised_raw,end_time)
VALUES (19,?,'Disposable End','Disposable End','Local test','[]','','','',?,?,?,?,?,?,?,'0',?)`, fixture.Owner, constant.ProjectStatusActive, constant.FundingModeCappedPresale, constant.ContractVersionV2Stablecoin, true, uint64(31337), fixture.Contract, fixture.PaymentToken, time.Unix(fixture.EndTime, 0).UTC()).Error; err != nil {
		t.Fatal(err)
	}
	// The real cron registration dispatches ProjectEndJob.Run, including the
	// Redis lease, chain-time check, durable reservation and signed broadcast.
	scheduler := cron.New(cron.WithSeconds())
	(&ProjectEndJob{}).Add(scheduler)
	if len(scheduler.Entries()) != 1 {
		t.Fatal("project-end cron missing")
	}
	scheduler.Start()
	defer func() { <-scheduler.Stop().Done() }()
	deadline := time.After(65 * time.Second)
	var submitted entity.ChainTxSubmission
	for submitted.TxHash == nil {
		select {
		case <-deadline:
			t.Fatal("real end cron never broadcast")
		case <-time.After(150 * time.Millisecond):
		}
		err := database.Where("project_id=? AND action=? AND state IN ?", fixture.ProjectID, constant.TransactionTypeProjectEnd, []string{constant.ChainTxStateSubmitted, constant.ChainTxStateFinalSuccess, constant.ChainTxStateApplied}).Take(&submitted).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatal(err)
		}
	}
	<-scheduler.Stop().Done()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	receipt, err := client.Client.TransactionReceipt(ctx, common.HexToHash(*submitted.TxHash))
	if err != nil || receipt.Status != 1 {
		t.Fatalf("Hardhat end receipt: %v status=%v", err, receipt)
	}
	if _, err := chaintx.ReconcileReceipt(ctx, database, client, submitted.ID, 0); err != nil {
		t.Fatal(err)
	}
	// Use the exact scanner cron registration on the successful chain path.
	scannerScheduler := cron.New(cron.WithSeconds())
	(&GetChainEventJob{}).Add(scannerScheduler)
	if len(scannerScheduler.Entries()) != 1 {
		t.Fatal("scanner cron missing")
	}
	scannerScheduler.Start()
	defer func() { <-scannerScheduler.Stop().Done() }()
	var state entity.ChainEventSyncState
	scannerDeadline := time.After(63 * time.Second)
	for {
		select {
		case <-scannerDeadline:
			t.Fatal("successful scanner cron did not advance before deadline")
		case <-time.After(150 * time.Millisecond):
		}
		if err := database.Where("id=1").Take(&state).Error; err != nil {
			t.Fatal(err)
		}
		if state.LastSuccessAt != nil && state.CursorBlock >= receipt.BlockNumber.Uint64() {
			break
		}
	}
	<-scannerScheduler.Stop().Done()
	if !state.Initialized || !state.V2Registered || state.CursorBlock < receipt.BlockNumber.Uint64() || state.LastSuccessAt == nil || state.LastError != "" {
		t.Fatalf("successful scanner did not commit cursor and V2 identity: %+v", state)
	}
	var applied entity.ChainTxSubmission
	if err := database.Where("id=?", submitted.ID).Take(&applied).Error; err != nil {
		t.Fatal(err)
	}
	var project entity.Project
	if err := database.Where("project_id=?", fixture.ProjectID).Take(&project).Error; err != nil {
		t.Fatal(err)
	}
	var records int64
	if err := database.Model(&entity.ChainEventRecord{}).Where("event = ? AND tx_hash = ?", baseChain.V2ProjectEnded, *submitted.TxHash).Count(&records).Error; err != nil {
		t.Fatal(err)
	}
	if applied.State != constant.ChainTxStateApplied || applied.FinalEventRecordID == nil || project.Status != constant.ProjectStatusFailed || records != 1 {
		t.Fatalf("end not materialized exactly once: state=%s project=%s records=%d", applied.State, project.Status, records)
	}
	firstLease := state.LeaseToken
	(&GetChainEventJob{}).Run()
	if err := database.Where("id=1").Take(&state).Error; err != nil {
		t.Fatal(err)
	}
	if state.LeaseToken == firstLease || state.LastError != "" || state.CursorBlock < receipt.BlockNumber.Uint64() {
		t.Fatalf("successor scanner failed: %+v", state)
	}
	if err := database.Model(&entity.ChainEventRecord{}).Where("event = ? AND tx_hash = ?", baseChain.V2ProjectEnded, *submitted.TxHash).Count(&records).Error; err != nil || records != 1 {
		t.Fatalf("replay duplicated event: %d %v", records, err)
	}
}

// The disposable test runner supplies a pinned, post-0907 MySQL schema plus
// the separately gated signing metadata migration. This test does not use RPC
// or a funded key; it does not claim to exercise the scanner or cron loop.
func TestIDOV2EndReservesOnFullMigratedMySQL(t *testing.T) {
	dsn := os.Getenv("JC_IDO_FULL_SCHEMA_DSN")
	if dsn == "" {
		t.Skip("requires owned disposable MySQL schema; see scripts/tests/ido_outbox_schema_disposable_test.sh")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var name string
	if err := database.Raw("SELECT DATABASE()").Scan(&name).Error; err != nil || name != "jc_ido_outbox_schema_it" {
		t.Fatalf("refusing unexpected schema %q: %v", name, err)
	}
	var count int64
	if err := database.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE()").Scan(&count).Error; err != nil || count < 103 {
		t.Fatalf("requires pinned complete schema: tables=%d err=%v", count, err)
	}
	if err := schema.RequireChainTxSigningMetadata(database); err != nil {
		t.Fatal(err)
	}
	var existing int64
	if err := database.Model(&entity.ChainTxSubmission{}).Where("project_id = ?", 9).Count(&existing).Error; err != nil || existing != 0 {
		t.Fatalf("disposable end slot not empty: count=%d err=%v", existing, err)
	}
	key, err := crypto.HexToECDSA(strings.Repeat("1", 64)) // local test identity only
	if err != nil {
		t.Fatal(err)
	}
	contract := common.HexToAddress("0x2222222222222222222222222222222222222222")
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("invalid local RPC request: %v", err)
			return
		}
		var result any
		switch request.Method {
		case "eth_estimateGas":
			result = "0x186a0"
		case "eth_gasPrice":
			result = "0xb2d05e00"
		case "eth_getTransactionCount":
			result = "0x0"
		case "eth_getTransactionReceipt":
			result = nil
		case "eth_sendRawTransaction":
			if len(request.Params) != 1 {
				t.Errorf("expected exactly one raw transaction")
				return
			}
			var raw string
			if err := json.Unmarshal(request.Params[0], &raw); err != nil {
				t.Errorf("invalid raw transaction parameter: %v", err)
				return
			}
			var signed types.Transaction
			if err := signed.UnmarshalBinary(common.FromHex(raw)); err != nil {
				t.Errorf("invalid signed transaction: %v", err)
				return
			}
			result = signed.Hash().Hex()
		default:
			t.Errorf("unexpected local RPC method %s", request.Method)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		payload, err := json.Marshal(result)
		if err != nil {
			t.Error(err)
			return
		}
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":%s}`, request.ID, payload)
	}))
	defer rpc.Close()
	client, err := baseChain.NewIDOV2Client(rpc.URL, key, contract, big.NewInt(31337))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Client.Close()
	if err := database.Exec(`INSERT INTO projects (project_id,owner_address,name_en,name_cn,description,tags,team_info,roadmap,partners,status,funding_mode,contract_version,onchain_locked,chain_id,contract_address,payment_token_address,total_raised_raw,end_time)
VALUES (9,?,'Disposable IDO','Disposable IDO','Disposable schema test','[]','','','',?,?,?,?,?,?,?,'0',?)`, client.Address.Hex(), constant.ProjectStatusActive, constant.FundingModeCappedPresale, constant.ContractVersionV2Stablecoin, true, uint64(31337), contract.Hex(), "0x3333333333333333333333333333333333333333", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("full-schema project seed rejected: %v", err)
	}
	data, err := client.PackEndProject(big.NewInt(9))
	if err != nil {
		t.Fatal(err)
	}
	coreSigner := common.HexToAddress("0x4444444444444444444444444444444444444444")
	var core *entity.ChainTxSubmission
	if err := database.Transaction(func(tx *gorm.DB) error {
		var txErr error
		core, txErr = chaintx.CreateIntentTx(tx, chaintx.Intent{ProjectID: 9, Action: constant.TransactionTypeProjectEnd, ChainID: 31337, SenderAddress: coreSigner, ContractAddress: contract, CallData: data})
		return txErr
	}); err != nil {
		t.Fatalf("full-schema Core business slot failed: %v", err)
	}
	reserve := func() (*entity.ChainTxSubmission, error) {
		var result *entity.ChainTxSubmission
		err := database.Transaction(func(tx *gorm.DB) error {
			var txErr error
			result, txErr = reserveIDOV2EndTx(tx, 9, client, data, time.Now())
			return txErr
		})
		return result, err
	}
	if reserved, err := reserve(); err == nil || reserved != nil || !strings.Contains(err.Error(), "active chain transaction intent does not match") {
		t.Fatalf("Core-owned slot admitted Job signer on full schema: row=%+v err=%v", reserved, err)
	}
	if err := database.Transaction(func(tx *gorm.DB) error { return chaintx.CancelIntentTx(tx, core.ID) }); err != nil {
		t.Fatalf("full-schema Core intent cancellation failed: %v", err)
	}
	first, err := reserve()
	if err != nil || first == nil || first.State != constant.ChainTxStateReady || first.ActiveSlot == nil || *first.ActiveSlot != 1 {
		t.Fatalf("full-schema reservation failed: row=%+v err=%v", first, err)
	}
	second, err := reserve()
	if err != nil || second == nil || second.ID != first.ID || second.State != constant.ChainTxStateReady {
		t.Fatalf("retry did not reuse full-schema business slot: row=%+v err=%v", second, err)
	}
	if err := database.Model(&entity.ChainTxSubmission{}).Where("project_id = ?", 9).Count(&existing).Error; err != nil || existing != 2 {
		t.Fatalf("duplicate outbox intents: count=%d err=%v", existing, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	submitted, err := chaintx.Dispatch(ctx, database, client, first.ID)
	if err != nil || submitted == nil || submitted.State != constant.ChainTxStateSubmitted || submitted.Nonce == nil || submitted.TxHash == nil || len(submitted.RawTx) == 0 {
		t.Fatalf("full-schema signing/broadcast failed: submission=%+v err=%v", submitted, err)
	}
	replayed, err := chaintx.Dispatch(ctx, database, client, first.ID)
	if err != nil || replayed == nil || replayed.TxHash == nil || *replayed.TxHash != *submitted.TxHash || replayed.BroadcastCount != 2 {
		t.Fatalf("full-schema exact-transaction replay failed: submission=%+v err=%v", replayed, err)
	}
	var fence entity.ChainRelayerNonce
	if err := database.Where("chain_id=? AND sender_address=?", 31337, client.Address.Hex()).Take(&fence).Error; err != nil || fence.NextNonce != 1 || fence.ActiveSubmissionID == nil || *fence.ActiveSubmissionID != first.ID {
		t.Fatalf("full-schema Job nonce fence invalid: %+v err=%v", fence, err)
	}
}

// Mirror the database schema gates invoked by cmd/job.InitAllConnection on
// the pinned baseline plus the explicitly approved candidate migration.
// A function-level outbox test alone cannot prove that the Job can start.
func TestIDOV2JobStartupSchemaGatesOnFullMigratedMySQL(t *testing.T) {
	dsn := os.Getenv("JC_IDO_FULL_SCHEMA_DSN")
	if dsn == "" {
		t.Skip("requires owned disposable MySQL schema")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var name string
	if err := database.Raw("SELECT DATABASE()").Scan(&name).Error; err != nil || name != "jc_ido_outbox_schema_it" {
		t.Fatalf("refusing unexpected schema %q: %v", name, err)
	}
	for _, gate := range []struct {
		name  string
		check func(*gorm.DB) error
	}{
		{"collation", schema.RequireChainEventCollationCompatibility},
		{"signing metadata", schema.RequireChainTxSigningMetadata},
		{"purchase entitlement", schema.RequireIDOPurchaseEntitlement},
		{"reconciliation evidence", schema.RequireIDOReconciliationEvidence},
		{"reorg evidence", schema.RequireChainEventReorgEvidence},
		{"reorg recovery scan", schema.RequireChainEventReorgRecoveryScan},
		{"reorg materialization", schema.RequireChainEventReorgMaterialization},
		{"reorg assessment", schema.RequireChainEventReorgAssessment},
		{"reorg prerequisite", schema.RequireChainEventReorgPrerequisite},
		{"reorg generation", schema.RequireChainEventReorgGeneration},
		{"V2-only policy", schema.RequireV2OnlySourcePolicy},
		{"security control plane", schema.RequireSecurityControlPlane},
	} {
		t.Run(gate.name, func(t *testing.T) {
			if err := gate.check(database); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Runs the actual GetChainEventJob.Run() Redis and DB lease acquisition path.
// RPC is intentionally broken inside the network-none container: the result
// must durably mark the previously healthy scanner state failed, not look ready.
func TestIDOV2ScannerRunPersistsRPCHeadFailureOnFullSchema(t *testing.T) {
	dsn := os.Getenv("JC_IDO_FULL_SCHEMA_DSN")
	redisAddress := os.Getenv("JC_IDO_FULL_SCHEMA_REDIS")
	if dsn == "" || redisAddress == "" {
		t.Skip("requires owned disposable MySQL and Redis cluster endpoint")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var name string
	if err := database.Raw("SELECT DATABASE()").Scan(&name).Error; err != nil || name != "jc_ido_outbox_schema_it" {
		t.Fatalf("refusing unexpected schema %q: %v", name, err)
	}
	rdb := redis.NewClusterClient(&redis.ClusterOptions{Addrs: []string{redisAddress}, MaxRedirects: 0})
	defer rdb.Close()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("disposable Redis cluster unavailable: %v", err)
	}
	oldDB := jobdb.Client
	jobdb.Client = &jobdb.Connection{Mysql: database, Redis: &jobredis.Connection{Client: rdb}}
	t.Cleanup(func() { jobdb.Client = oldDB })
	commonlog.Init(os.TempDir(), "ERROR", 1)
	baseline := time.Now()
	if err := database.Model(&entity.ChainEventSyncState{}).Where("id=1").Updates(map[string]interface{}{
		"last_started_at": baseline, "last_success_at": baseline, "last_failure_at": nil, "last_error": "", "updated_at": baseline,
	}).Error; err != nil {
		t.Fatal(err)
	}
	// Another Redis lock holder must prevent the run from mutating the DB.
	const foreignToken = "disposable-foreign-scanner"
	if acquired, err := jobdb.Redis().TryTokenLock(context.Background(), constant.KEY_CHAIN_EVENT_JOB_LOCK, foreignToken, time.Minute); err != nil || !acquired {
		t.Fatalf("could not establish isolated competing scanner lock: acquired=%t err=%v", acquired, err)
	}
	probeCount := 0
	(&GetChainEventJob{}).runWithHead(func(context.Context) (uint64, uint64, error) {
		probeCount++
		return 0, 0, errors.New("blocked scanner unexpectedly reached RPC")
	})
	if probeCount != 0 {
		t.Fatal("blocked scanner reached RPC despite Redis lock")
	}
	lockedState := entity.ChainEventSyncState{}
	if err := database.Where("id=1").Take(&lockedState).Error; err != nil || lockedState.LastFailureAt != nil {
		t.Fatalf("blocked scanner mutated durable status: %+v err=%v", lockedState, err)
	}
	if err := jobdb.Redis().UnlockTokenLock(context.Background(), constant.KEY_CHAIN_EVENT_JOB_LOCK, foreignToken); err != nil {
		t.Fatal(err)
	}
	(&GetChainEventJob{}).runWithHead(func(context.Context) (uint64, uint64, error) {
		probeCount++
		return 0, 0, errors.New("disposable RPC head unavailable")
	})
	state := entity.ChainEventSyncState{}
	if err := database.Where("id=1").Take(&state).Error; err != nil {
		t.Fatal(err)
	}
	if probeCount != 1 || state.LastFailureAt == nil || !strings.Contains(state.LastError, "disposable RPC head unavailable") || state.LastSuccessAt == nil {
		t.Fatalf("RPC head failure not durably observed: last_failure=%v error=%q", state.LastFailureAt, state.LastError)
	}
	router := gin.New()
	RegisterHealthRoutes(router)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/chain-events", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("failed scanner still advertised healthy: %d %s", response.Code, response.Body.String())
	}
	// A new Job instance must be able to reclaim both the distributed lock and
	// the DB lease after the failed run, without making the scanner look ready.
	firstLease := state.LeaseToken
	firstFailure := *state.LastFailureAt
	(&GetChainEventJob{}).runWithHead(func(context.Context) (uint64, uint64, error) {
		return 0, 0, errors.New("disposable RPC still unavailable after restart")
	})
	if err := database.Where("id=1").Take(&state).Error; err != nil {
		t.Fatal(err)
	}
	if state.LeaseToken == firstLease || state.LastFailureAt == nil || state.LastFailureAt.Before(firstFailure) || !strings.Contains(state.LastError, "still unavailable after restart") {
		t.Fatalf("successor scanner did not own its failure: lease=%q old=%q last_failure=%v err=%q", state.LeaseToken, firstLease, state.LastFailureAt, state.LastError)
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/chain-events", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("second failed run advertised healthy: %d %s", response.Code, response.Body.String())
	}
	// Cancellation while the admitted RPC read is in flight must return,
	// persist unhealthy state and release the token lock for a successor.
	cancelCtx, cancelRun := context.WithCancel(context.Background())
	readEntered := make(chan struct{})
	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		(&GetChainEventJob{}).runWithHeadContext(cancelCtx, func(ctx context.Context) (uint64, uint64, error) {
			close(readEntered)
			<-ctx.Done()
			return 0, 0, ctx.Err()
		})
	}()
	select {
	case <-readEntered:
	case <-time.After(3 * time.Second):
		cancelRun()
		t.Fatal("scanner did not enter cancellable RPC read")
	}
	cancelRun()
	select {
	case <-runDone:
	case <-time.After(3 * time.Second):
		t.Fatal("scanner failed to stop after context cancellation")
	}
	if err := database.Where("id=1").Take(&state).Error; err != nil || !strings.Contains(state.LastError, context.Canceled.Error()) {
		t.Fatalf("shutdown cancellation not durably observed: error=%q err=%v", state.LastError, err)
	}
	const successor = "disposable-after-stop"
	if acquired, err := jobdb.Redis().TryTokenLock(context.Background(), constant.KEY_CHAIN_EVENT_JOB_LOCK, successor, time.Second); err != nil || !acquired {
		t.Fatalf("stopped scanner retained distributed lock: acquired=%t err=%v", acquired, err)
	}
	if err := jobdb.Redis().UnlockTokenLock(context.Background(), constant.KEY_CHAIN_EVENT_JOB_LOCK, successor); err != nil {
		t.Fatal(err)
	}
}

// The runner executes this test twice as separate OS processes against the
// same disposable MySQL and Redis. No real RPC or shared environment is used.
func TestIDOV2ScannerSuccessorProcessRetainsFailureOnFullSchema(t *testing.T) {
	stage := os.Getenv("JC_IDO_SCANNER_RESTART_STAGE")
	if stage == "" {
		t.Skip("requires first/successor stages from disposable schema runner")
	}
	if stage != "first" && stage != "successor" {
		t.Fatal("unknown disposable scanner restart stage")
	}
	dsn, redisAddress := os.Getenv("JC_IDO_FULL_SCHEMA_DSN"), os.Getenv("JC_IDO_FULL_SCHEMA_REDIS")
	if dsn == "" || redisAddress != "127.0.0.1:16379" {
		t.Fatal("requires private disposable schema and local Redis endpoint")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var name string
	if err := database.Raw("SELECT DATABASE()").Scan(&name).Error; err != nil || name != "jc_ido_outbox_schema_it" {
		t.Fatalf("refusing unexpected schema %q: %v", name, err)
	}
	rdb := redis.NewClusterClient(&redis.ClusterOptions{Addrs: []string{redisAddress}, MaxRedirects: 0})
	defer rdb.Close()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	previous := jobdb.Client
	jobdb.Client = &jobdb.Connection{Mysql: database, Redis: &jobredis.Connection{Client: rdb}}
	t.Cleanup(func() { jobdb.Client = previous })
	commonlog.Init(os.TempDir(), "ERROR", 1)
	before := entity.ChainEventSyncState{}
	if err := database.Where("id=1").Take(&before).Error; err != nil {
		t.Fatal(err)
	}
	if stage == "successor" && (before.LastFailureAt == nil || !strings.Contains(before.LastError, "first process RPC failed") || before.LeaseToken == "") {
		t.Fatalf("first process failure not durable across restart: %+v", before)
	}
	(&GetChainEventJob{}).runWithHead(func(context.Context) (uint64, uint64, error) {
		return 0, 0, fmt.Errorf("%s process RPC failed", stage)
	})
	after := entity.ChainEventSyncState{}
	if err := database.Where("id=1").Take(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.LeaseToken == before.LeaseToken || after.LastFailureAt == nil || !strings.Contains(after.LastError, stage+" process RPC failed") {
		t.Fatalf("new process did not reclaim lease and retain failure: before=%q after=%q failure=%v err=%q", before.LeaseToken, after.LeaseToken, after.LastFailureAt, after.LastError)
	}
	router := gin.New()
	RegisterHealthRoutes(router)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/chain-events", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("failed %s process advertised ready: %d %s", stage, response.Code, response.Body.String())
	}
}

// Exercises the production scanner's Add -> robfig/cron -> Run registration,
// not just a direct Run call. The injected head error keeps the test offline.
func TestIDOV2ScannerCronDispatchFailsClosedOnFullSchema(t *testing.T) {
	dsn, redisAddress := os.Getenv("JC_IDO_FULL_SCHEMA_DSN"), os.Getenv("JC_IDO_FULL_SCHEMA_REDIS")
	if dsn == "" || redisAddress != "127.0.0.1:16379" {
		t.Skip("requires owned disposable MySQL and Redis endpoint")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var name string
	if err := database.Raw("SELECT DATABASE()").Scan(&name).Error; err != nil || name != "jc_ido_outbox_schema_it" {
		t.Fatalf("refusing unexpected schema %q: %v", name, err)
	}
	rdb := redis.NewClusterClient(&redis.ClusterOptions{Addrs: []string{redisAddress}, MaxRedirects: 0})
	defer rdb.Close()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	previous := jobdb.Client
	jobdb.Client = &jobdb.Connection{Mysql: database, Redis: &jobredis.Connection{Client: rdb}}
	t.Cleanup(func() { jobdb.Client = previous })
	commonlog.Init(os.TempDir(), "ERROR", 1)
	before := entity.ChainEventSyncState{}
	if err := database.Where("id=1").Take(&before).Error; err != nil {
		t.Fatal(err)
	}
	// WithSeconds is the real starter configuration. Advance no clock and
	// retain the actual 0 * * * * * schedule; bound this test to 62 seconds.
	scheduler := cron.New(cron.WithSeconds())
	job := &GetChainEventJob{readHead: func(context.Context) (uint64, uint64, error) {
		return 0, 0, errors.New("disposable cron RPC unavailable")
	}}
	job.Add(scheduler)
	if len(scheduler.Entries()) != 1 {
		t.Fatal("scanner cron entry not registered")
	}
	scheduler.Start()
	defer func() { <-scheduler.Stop().Done() }()
	deadline := time.After(62 * time.Second)
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("scheduled scanner did not persist its RPC failure within one minute")
		case <-ticker.C:
			after := entity.ChainEventSyncState{}
			if err := database.Where("id=1").Take(&after).Error; err != nil {
				t.Fatal(err)
			}
			if after.LeaseToken != before.LeaseToken && strings.Contains(after.LastError, "disposable cron RPC unavailable") {
				router := gin.New()
				RegisterHealthRoutes(router)
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/chain-events", nil))
				if response.Code != http.StatusServiceUnavailable {
					t.Fatalf("scheduled scanner failure advertised ready: %d %s", response.Code, response.Body.String())
				}
				return
			}
		}
	}
}

func TestScannerRunContextStopsBeforeAcquiringRedisLock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	invoked := false
	job := &GetChainEventJob{readHead: func(context.Context) (uint64, uint64, error) {
		invoked = true
		return 0, 0, nil
	}}
	job.SetRunContext(ctx)
	job.Run()
	if invoked {
		t.Fatal("stopped scanner read RPC head")
	}
}

// Exercises the scanner's actual durable lease/failure helpers and HTTP
// health projection. It does not assert that Redis or RPC scanning ran.
func TestIDOV2SyncLeaseAndAlertOnFullMigratedMySQL(t *testing.T) {
	dsn := os.Getenv("JC_IDO_FULL_SCHEMA_DSN")
	if dsn == "" {
		t.Skip("requires owned disposable MySQL schema")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var name string
	if err := database.Raw("SELECT DATABASE()").Scan(&name).Error; err != nil || name != "jc_ido_outbox_schema_it" {
		t.Fatalf("refusing unexpected schema %q: %v", name, err)
	}
	previous := jobdb.Client
	jobdb.Client = &jobdb.Connection{Mysql: database}
	t.Cleanup(func() { jobdb.Client = previous })
	const owned = "disposable-scanner-lease-owner"
	state, err := claimSyncLease(owned)
	if err != nil || state == nil || state.ID != 1 || state.LeaseToken != owned {
		t.Fatalf("actual scanner lease claim failed: %+v err=%v", state, err)
	}
	now := time.Now()
	if err := updateFencedSyncState(owned, map[string]interface{}{"last_started_at": now, "last_success_at": now, "last_error": "", "updated_at": now}); err != nil {
		t.Fatal(err)
	}
	healthStatus := func() int {
		t.Helper()
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/health/chain-events", nil)
		router := gin.New()
		RegisterHealthRoutes(router)
		router.ServeHTTP(response, request)
		return response.Code
	}
	if status := healthStatus(); status != http.StatusOK {
		t.Fatalf("healthy isolated scanner DB state reported %d", status)
	}
	// A failed canonical log must remain visible across retries and move to a
	// dead-letter state after the configured bounded attempts.
	info := types.Log{
		Address:     common.HexToAddress("0x2222222222222222222222222222222222222222"),
		BlockNumber: 1,
		BlockHash:   common.HexToHash("0x" + strings.Repeat("a", 64)),
		TxHash:      common.HexToHash("0x" + strings.Repeat("b", 64)),
		Index:       1,
	}
	for attempt := 1; attempt <= 3; attempt++ {
		if err := recordEventFailure(owned, info, "disposable_decode", errors.New("disposable event decode failed")); err != nil {
			t.Fatalf("persist failed event attempt %d: %v", attempt, err)
		}
		failure := entity.ChainEventFailure{}
		if err := database.Where("block_number=? AND tx_hash=? AND log_index=?", info.BlockNumber, info.TxHash.Hex(), info.Index).Take(&failure).Error; err != nil {
			t.Fatal(err)
		}
		expected := constant.ChainEventFailureRetrying
		if attempt == 3 {
			expected = constant.ChainEventFailureDeadLetter
		}
		if failure.Status != expected || failure.ConsecutiveCount != uint(attempt) {
			t.Fatalf("unexpected durable retry state after attempt %d: %s count=%d", attempt, failure.Status, failure.ConsecutiveCount)
		}
	}
	if _, err := ensureEventCanRun(info); !errors.Is(err, errDeadLetterBlocked) {
		t.Fatalf("dead-letter event was not blocked: %v", err)
	}
	if status := healthStatus(); status != http.StatusServiceUnavailable {
		t.Fatalf("dead letter not reflected in health: %d", status)
	}
	if err := database.Transaction(func(tx *gorm.DB) error {
		return resolveEventFailure(tx, info.BlockNumber, info.TxHash.Hex(), info.Index)
	}); err != nil {
		t.Fatal(err)
	}
	if status := healthStatus(); status != http.StatusOK {
		t.Fatalf("resolved local dead letter did not restore health: %d", status)
	}
	if err := updateSyncFailure(owned, 0, 2, 2, errors.New("disposable scanner RPC stopped")); err != nil {
		t.Fatal(err)
	}
	if status := healthStatus(); status != http.StatusServiceUnavailable {
		t.Fatalf("scanner failure not reflected in health: %d", status)
	}
	if err := updateFencedSyncState("stale-disposable-lease", map[string]interface{}{"last_error": ""}); err == nil {
		t.Fatal("stale scanner lease cleared durable failure")
	}
	if status := healthStatus(); status != http.StatusServiceUnavailable {
		t.Fatalf("stale lease cleared scanner health alert: %d", status)
	}
	if err := updateFencedSyncState(owned, map[string]interface{}{"last_success_at": time.Now(), "last_error": "", "updated_at": time.Now()}); err != nil {
		t.Fatal(err)
	}
	if status := healthStatus(); status != http.StatusOK {
		t.Fatalf("owned recovery did not restore isolated scanner health: %d", status)
	}
}

// Governance/configuration events without material effects still need their
// retry resolution committed atomically with the scanner's fenced checkpoint.
// A second db.Mysql().Transaction while holding the outer fence can deadlock.
func TestV2NonMaterialRetrySharesScannerBatchTransactionOnFullSchema(t *testing.T) {
	dsn := os.Getenv("JC_IDO_FULL_SCHEMA_DSN")
	if dsn == "" {
		t.Skip("requires owned disposable MySQL schema")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var name string
	if err := database.Raw("SELECT DATABASE()").Scan(&name).Error; err != nil || name != "jc_ido_outbox_schema_it" {
		t.Fatalf("refusing unexpected schema %q: %v", name, err)
	}
	previous := jobdb.Client
	jobdb.Client = &jobdb.Connection{Mysql: database}
	t.Cleanup(func() { jobdb.Client = previous })
	const owner = "disposable-nonmaterial-retry"
	if _, err := claimSyncLease(owner); err != nil {
		t.Fatal(err)
	}
	info := types.Log{
		Address:     common.HexToAddress("0x2222222222222222222222222222222222222222"),
		BlockNumber: 19, BlockHash: common.HexToHash("0x" + strings.Repeat("c", 64)),
		TxHash: common.HexToHash("0x" + strings.Repeat("d", 64)), Index: 2,
		Topics: []common.Hash{crypto.Keccak256Hash([]byte("PaymentTokenConfigured(address,bool,uint8)"))},
	}
	if !baseChain.IsApprovedV2NonMaterialTopic(info.Topics[0]) {
		t.Fatal("fixture must be an approved non-material V2 topic")
	}
	if err := recordEventFailure(owner, info, "disposable_governance", errors.New("original transient fault")); err != nil {
		t.Fatal(err)
	}
	initial := entity.ChainEventSyncState{}
	if err := database.Where("id=1").Take(&initial).Error; err != nil {
		t.Fatal(err)
	}
	// Rollback must leave both the failure and the checkpoint untouched.
	rollback := errors.New("injected cursor persistence failure")
	apply := func(abort bool) error {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		return database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := lockChainEventSyncFenceTx(tx, owner); err != nil {
				return err
			}
			hasFailure, err := ensureEventCanRun(info)
			if err != nil || !hasFailure {
				return fmt.Errorf("retry not available: found=%t err=%v", hasFailure, err)
			}
			if err := resolveRetriedV2NonMaterialTx(tx, info, hasFailure); err != nil {
				return err
			}
			if abort {
				return rollback
			}
			return updateFencedSyncStateTx(tx, owner, map[string]interface{}{
				"cursor_block": uint64(19), "updated_at": time.Now(),
			})
		})
	}
	if err := apply(true); !errors.Is(err, rollback) {
		t.Fatalf("rollback failed or nested transaction blocked: %v", err)
	}
	var failure entity.ChainEventFailure
	var checkpoint entity.ChainEventSyncState
	if err := database.Where("block_number=? AND tx_hash=? AND log_index=?", info.BlockNumber, info.TxHash.Hex(), info.Index).Take(&failure).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Where("id=1").Take(&checkpoint).Error; err != nil || failure.Status != constant.ChainEventFailureRetrying || checkpoint.CursorBlock != initial.CursorBlock {
		t.Fatalf("non-material retry escaped rollback: failure=%s cursor=%d err=%v", failure.Status, checkpoint.CursorBlock, err)
	}
	if err := apply(false); err != nil {
		t.Fatalf("fenced non-material replay failed: %v", err)
	}
	if err := database.Where("block_number=? AND tx_hash=? AND log_index=?", info.BlockNumber, info.TxHash.Hex(), info.Index).Take(&failure).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Where("id=1").Take(&checkpoint).Error; err != nil || failure.Status != constant.ChainEventFailureResolved || checkpoint.CursorBlock != 19 {
		t.Fatalf("retry and cursor did not commit together: failure=%s cursor=%d err=%v", failure.Status, checkpoint.CursorBlock, err)
	}
}

package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// This test only runs in the network-none pinned MySQL 8.4 disposable runner.
// It verifies actual pending -> executed application and subsequent immutable
// reader, not a simulated RPC, signer, scanner, or production authorization.
func TestApprovedV3SetDisposableMySQLTransaction(t *testing.T) {
	dsn := os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if dsn == "" {
		t.Skip("disposable V3 MySQL DSN is not set")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("disposable database unavailable")
	}
	var name, version string
	if database.Raw("SELECT DATABASE()").Scan(&name).Error != nil || !strings.HasPrefix(name, "jc_v3_audit_") ||
		database.Raw("SELECT VERSION()").Scan(&version).Error != nil || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("refuse non-disposable MySQL")
	}
	if err := idov3whitelist.ApprovedV3SchemaReady(database); err != nil {
		t.Fatalf("approved V3 schema readiness rejected disposable migration: %v", err)
	}
	// The production projects columns are DATETIME(0), not DATETIME(6).
	// Reproduce both sides of the MySQL half-second rounding boundary on the
	// same disposable MySQL 8.4 connection used by the Set transaction.
	if err := database.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TEMPORARY TABLE v3_approved_time_precision (
			start_time DATETIME NOT NULL,end_time DATETIME NOT NULL,tge_time DATETIME NOT NULL)`).Error; err != nil {
			return err
		}
		for _, nanos := range []int{420000000, 586000000} {
			value := time.Date(2030, 1, 1, 7, 0, 53, nanos, time.UTC)
			end, tge := value.Add(time.Hour), value.Add(time.Hour+10*time.Minute)
			if err := tx.Exec(`INSERT INTO v3_approved_time_precision(start_time,end_time,tge_time) VALUES (?,?,?)`, value, end, tge).Error; err != nil {
				return err
			}
			var stored []struct {
				Start time.Time `gorm:"column:start_time"`
				End   time.Time `gorm:"column:end_time"`
				TGE   time.Time `gorm:"column:tge_time"`
			}
			if err := tx.Raw(`SELECT start_time,end_time,tge_time FROM v3_approved_time_precision`).Scan(&stored).Error; err != nil || len(stored) != 1 || !stored[0].Start.Equal(value.Round(time.Second)) || !stored[0].End.Equal(end.Round(time.Second)) || !stored[0].TGE.Equal(tge.Round(time.Second)) {
				return fmt.Errorf("MySQL DATETIME(0) rounding differs from reviewed seconds: %v %v", stored, err)
			}
			request := OnchainParamsRequest{StartTime: value.Format(time.RFC3339Nano), EndTime: end.Format(time.RFC3339Nano), TGEAfterEndMin: 10, TGE: []OnchainTGEEntry{{Percent: decimal.NewFromInt(100)}}}
			raw, e := json.Marshal(request)
			if e != nil {
				return e
			}
			digest := sha256.Sum256(raw)
			record := idov3whitelist.ApprovedSetRecord{Request: raw, Identity: idov3whitelist.ApprovedSetIdentity{CreateDeadline: value.Unix(), RequestDigest: "0x" + hex.EncodeToString(digest[:])}}
			project := entity.Project{StartTime: stored[0].Start, EndTime: stored[0].End, TgeTime: stored[0].TGE}
			startSecond, endSecond, times, e := approvedV3CreateTimes(project, []entity.ProjectTge{{Index: 1, VestingTime: stored[0].TGE}}, record)
			if e != nil || startSecond != value.Unix() || endSecond != end.Unix() || len(times) != 1 || times[0].Int64() != tge.Unix() {
				return fmt.Errorf("reviewed time reconstruction failed: %d %d %v %v", startSecond, endSecond, times, e)
			}
			if err := tx.Exec(`DELETE FROM v3_approved_time_precision`).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("disposable MySQL time precision: %v", err)
	}
	const projectID int64 = 90092
	start := time.Now().UTC().Add(4 * time.Hour).Truncate(time.Second)
	req := OnchainParamsRequest{ContractVersion: constant.ContractVersionIDOV3Stablecoin,
		PaymentToken: "USDT", TokenAddress: v3TokenTESTCAT, OwnerAddress: v3DiagnosticOperator,
		TokenPrice: "1", HardCap: "10", SoftCap: "5", MaxPerUser: "2",
		StartTime: start.Format(time.RFC3339), EndTime: start.Add(time.Hour).Format(time.RFC3339),
		TGE: []OnchainTGEEntry{{Percent: decimal.NewFromInt(100)}}, TGEAfterEndMin: 60}
	payload, err := json.Marshal(struct {
		ProjectID int64                `json:"project_id"`
		Params    OnchainParamsRequest `json:"params"`
	}{projectID, req})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Exec(`INSERT INTO projects(project_id,status,funding_mode,contract_version,chain_id,
		contract_address,description,tags,team_info,roadmap,partners)
		VALUES (?,'Setup','capped_presale','unassigned',0,'','','','','','')`, projectID).Error; err != nil {
		t.Fatal("project fixture unavailable")
	}
	if err := database.Exec(`INSERT INTO admin_approval(operation,target_kind,target_id,payload,initiator_id,status,
		expires_at) VALUES ('project.set_onchain_params','project',?,?,7,'pending',?)`, "90092", string(payload),
		time.Now().UTC().Add(time.Hour)).Error; err != nil {
		t.Fatal("approval fixture unavailable")
	}
	var approvalID uint64
	if database.Raw(`SELECT id FROM admin_approval WHERE target_kind='project' AND target_id='90092'
		AND operation='project.set_onchain_params' LIMIT 1`).Scan(&approvalID).Error != nil || approvalID == 0 {
		t.Fatal("approval identity unavailable")
	}
	res := &resolvedOnchainParams{Req: req, Trusted: trustedPaymentToken{Symbol: "USDT", Address: v3TokenUSDT,
		ChainID: 56, Contract: v3TokenProxy, Decimals: 18, FeeReceiver: v3DiagnosticGovernor, SuccessFeeRate: 100},
		TokenDecimals: 18, TokenSymbolValue: "TESTCAT", PriceRaw: "1000000000000000000",
		HardRaw: "10000000000000000000", SoftRaw: "5000000000000000000", MaxRaw: "2000000000000000000",
		Start: start, End: start.Add(time.Hour), TGERows: []entity.ProjectTge{{ProjectID: int(projectID), Index: 1,
			TgePercent: decimal.NewFromInt(100), VestingPercent: decimal.NewFromInt(100),
			VestingTime: start.Add(2 * time.Hour)}}}
	if err := database.Transaction(func(tx *gorm.DB) error {
		if _, err := readPendingApprovedV3SetTx(tx, approvalID, 7, projectID, req); err != nil {
			return err
		}
		if _, err := persistApprovedV3SetTx(tx, approvalID, 7, 8, projectID, req, res); err != nil {
			return err
		}
		return tx.Exec(`UPDATE admin_approval SET status='executed',approver_id=8,reviewed_at=NOW(),
			executed_at=NOW() WHERE id=? AND status='pending'`, approvalID).Error
	}); err != nil {
		t.Fatalf("approved Set transaction failed: %v", err)
	}
	record, err := idov3whitelist.ReadApprovedSetRecord(context.Background(), database, uint64(projectID))
	if err != nil || record.Identity.ApprovalID != approvalID || record.Identity.InitiatorID != 7 ||
		record.Identity.ApproverID != 8 || record.Identity.CreateDeadline != start.Unix() {
		t.Fatalf("persisted approval evidence mismatch: %v", err)
	}
	if database.Exec(`UPDATE projects SET hard_cap_raw='1' WHERE project_id=?`, projectID).Error == nil {
		t.Fatal("approved economic terms changed")
	}
	if database.Exec(`UPDATE admin_approval SET approver_id=7 WHERE id=?`, approvalID).Error == nil {
		t.Fatal("executed four-eyes identity changed")
	}
	ctx := context.Background()
	if idov3whitelist.RequireFreshApprovedV3Scan(ctx, database) == nil {
		t.Fatal("proof admitted before the first complete scan")
	}
	generation, err := idov3whitelist.BeginApprovedV3Scan(ctx, database)
	if err != nil || generation == 0 {
		t.Fatalf("scan start unavailable: %v", err)
	}
	if idov3whitelist.RequireFreshApprovedV3Scan(ctx, database) == nil {
		t.Fatal("incomplete scan admitted")
	}
	if idov3whitelist.FinishApprovedV3Scan(ctx, database, generation+1) == nil {
		t.Fatal("foreign scan generation completed")
	}
	if err := database.Exec(`UPDATE ido_v3_approved_scan_health SET binding_count=binding_count+1 WHERE chain_id=56`).Error; err != nil {
		t.Fatal("scan count mismatch fixture unavailable")
	}
	if idov3whitelist.FinishApprovedV3Scan(ctx, database, generation) == nil {
		t.Fatal("scan completed after the binding set changed")
	}
	if err := database.Exec(`UPDATE ido_v3_approved_scan_health SET binding_count=binding_count-1 WHERE chain_id=56`).Error; err != nil {
		t.Fatal("scan count fixture reset unavailable")
	}
	if err := idov3whitelist.FinishApprovedV3Scan(ctx, database, generation); err != nil {
		t.Fatalf("complete scan not published: %v", err)
	}
	if err := idov3whitelist.RequireFreshApprovedV3Scan(ctx, database); err != nil {
		t.Fatalf("healthy complete scan unavailable: %v", err)
	}
	if err := database.Exec(`UPDATE ido_v3_approved_scan_health SET
		started_at=UTC_TIMESTAMP(6)-INTERVAL 120 SECOND,
		last_success_at=UTC_TIMESTAMP(6)-INTERVAL 120 SECOND WHERE chain_id=56`).Error; err != nil {
		t.Fatal("stale scan fixture unavailable")
	}
	if idov3whitelist.RequireFreshApprovedV3Scan(ctx, database) == nil {
		t.Fatal("stale completed scan admitted")
	}
	if err := database.Exec(`UPDATE ido_v3_approved_scan_health SET
		started_at=UTC_TIMESTAMP(6),last_success_at=UTC_TIMESTAMP(6) WHERE chain_id=56`).Error; err != nil {
		t.Fatal("stale scan fixture reset unavailable")
	}
	if err := database.Exec(`UPDATE ido_v3_approved_scan_health SET binding_count=binding_count+1 WHERE chain_id=56`).Error; err != nil {
		t.Fatal("freshness mismatch fixture unavailable")
	}
	if idov3whitelist.RequireFreshApprovedV3Scan(ctx, database) == nil {
		t.Fatal("healthy scan admitted a different binding set")
	}
	if err := database.Exec(`UPDATE ido_v3_approved_scan_health SET binding_count=binding_count-1 WHERE chain_id=56`).Error; err != nil {
		t.Fatal("freshness fixture reset unavailable")
	}
	// Start enumeration before a new binding is approved; its completion must
	// not certify the earlier snapshot as a full scan of both projects.
	partialGeneration, err := idov3whitelist.BeginApprovedV3Scan(ctx, database)
	if err != nil {
		t.Fatalf("partial scan start unavailable: %v", err)
	}
	// A second approved project arriving after a complete tick invalidates
	// the global proof fence until every binding has been enumerated again.
	const secondID int64 = 90095
	if err := database.Exec(`INSERT INTO projects(project_id,status,funding_mode,contract_version,chain_id,
		contract_address,description,tags,team_info,roadmap,partners)
		VALUES (?,'Setup','capped_presale','unassigned',0,'','','','','','')`, secondID).Error; err != nil {
		t.Fatal("second project fixture unavailable")
	}
	secondPayload, err := json.Marshal(struct {
		ProjectID int64                `json:"project_id"`
		Params    OnchainParamsRequest `json:"params"`
	}{secondID, req})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Exec(`INSERT INTO admin_approval(operation,target_kind,target_id,payload,initiator_id,status,
		expires_at) VALUES ('project.set_onchain_params','project','90095',?,7,'pending',?)`,
		string(secondPayload), time.Now().UTC().Add(time.Hour)).Error; err != nil {
		t.Fatal("second approval fixture unavailable")
	}
	var secondApproval uint64
	if database.Raw(`SELECT id FROM admin_approval WHERE target_kind='project' AND target_id='90095'
		AND operation='project.set_onchain_params' LIMIT 1`).Scan(&secondApproval).Error != nil || secondApproval == 0 {
		t.Fatal("second approval identity unavailable")
	}
	if err := database.Transaction(func(tx *gorm.DB) error {
		if _, err := readPendingApprovedV3SetTx(tx, secondApproval, 7, secondID, req); err != nil {
			return err
		}
		secondRes := *res
		secondRes.TGERows = append([]entity.ProjectTge(nil), res.TGERows...)
		secondRes.TGERows[0].ID = 0
		secondRes.TGERows[0].ProjectID = int(secondID)
		if _, err := persistApprovedV3SetTx(tx, secondApproval, 7, 8, secondID, req, &secondRes); err != nil {
			return err
		}
		return tx.Exec(`UPDATE admin_approval SET status='executed',approver_id=8,reviewed_at=NOW(),
			executed_at=NOW() WHERE id=? AND status='pending'`, secondApproval).Error
	}); err != nil {
		t.Fatalf("second approved Set failed: %v", err)
	}
	if idov3whitelist.RequireFreshApprovedV3Scan(ctx, database) == nil {
		t.Fatal("prior full tick authorized a newly admitted project")
	}
	ids, err := idov3whitelist.ListApprovedV3ProjectIDs(ctx, database, 0, 32)
	if err != nil || len(ids) != 2 || ids[0] != uint64(projectID) || ids[1] != uint64(secondID) {
		t.Fatalf("new project omitted from full-tick enumeration: %v %v", ids, err)
	}
	if idov3whitelist.FinishApprovedV3Scan(ctx, database, partialGeneration) == nil {
		t.Fatal("scan omitted a binding added during enumeration")
	}
	if _, err := idov3whitelist.BeginApprovedV3Scan(ctx, database); err != nil {
		t.Fatalf("next scan not started: %v", err)
	}
	if idov3whitelist.RequireFreshApprovedV3Scan(ctx, database) == nil {
		t.Fatal("prior healthy scan remained proof authority during a new tick")
	}
	// The approval service's handler/savepoint and final approval status share
	// one transaction. Simulate failure after all Set inserts and verify that
	// no bound project or immutable evidence escapes the rollback.
	const rolledBackID int64 = 90094
	if err := database.Exec(`INSERT INTO projects(project_id,status,funding_mode,contract_version,chain_id,
		contract_address,description,tags,team_info,roadmap,partners)
		VALUES (?,'Setup','capped_presale','unassigned',0,'','','','','','')`, rolledBackID).Error; err != nil {
		t.Fatal("rollback project fixture unavailable")
	}
	rollbackPayload, err := json.Marshal(struct {
		ProjectID int64                `json:"project_id"`
		Params    OnchainParamsRequest `json:"params"`
	}{rolledBackID, req})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Exec(`INSERT INTO admin_approval(operation,target_kind,target_id,payload,initiator_id,status,
		expires_at) VALUES ('project.set_onchain_params','project','90094',?,7,'pending',?)`,
		string(rollbackPayload), time.Now().UTC().Add(time.Hour)).Error; err != nil {
		t.Fatal("rollback approval fixture unavailable")
	}
	var rollbackApproval uint64
	if database.Raw(`SELECT id FROM admin_approval WHERE target_kind='project' AND target_id='90094'
		AND operation='project.set_onchain_params' LIMIT 1`).Scan(&rollbackApproval).Error != nil || rollbackApproval == 0 {
		t.Fatal("rollback approval identity unavailable")
	}
	rollbackErr := database.Transaction(func(tx *gorm.DB) error {
		if _, err := readPendingApprovedV3SetTx(tx, rollbackApproval, 7, rolledBackID, req); err != nil {
			return err
		}
		if _, err := persistApprovedV3SetTx(tx, rollbackApproval, 7, 8, rolledBackID, req, res); err != nil {
			return err
		}
		return errors.New("intentional local rollback")
	})
	if rollbackErr == nil {
		t.Fatal("intentional rollback ignored")
	}
	var rows []struct {
		Status  string `gorm:"column:status"`
		Version string `gorm:"column:contract_version"`
	}
	if database.Raw(`SELECT status,contract_version FROM projects WHERE project_id=?`, rolledBackID).
		Scan(&rows).Error != nil || len(rows) != 1 || rows[0].Status != constant.ProjectStatusSetup ||
		rows[0].Version != constant.ContractVersionUnassigned {
		t.Fatal("rolled-back Set modified project")
	}
	for _, table := range []string{"ido_v3_project_binding", "ido_v3_set_create_commitment", "ido_v3_set_approval_identity"} {
		var n int64
		if database.Raw(`SELECT COUNT(*) FROM `+table+` WHERE project_id=?`, rolledBackID).Scan(&n).Error != nil || n != 0 {
			t.Fatal("rolled-back Set left durable evidence")
		}
	}
	var pending int64
	if database.Raw(`SELECT COUNT(*) FROM admin_approval WHERE id=? AND status='pending' AND approver_id IS NULL`, rollbackApproval).
		Scan(&pending).Error != nil || pending != 1 {
		t.Fatal("rolled-back approval was executed")
	}
}

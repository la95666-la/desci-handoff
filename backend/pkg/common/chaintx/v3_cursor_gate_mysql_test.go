package chaintx

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"joincare_server/pkg/common/idov3whitelist"
)

// Two physical connections, real InnoDB RR snapshots, no sleeps or public RPC.
// The reader's snapshot MUST predate the writer's commit. This would fail with
// the former ordinary checkpoint SELECT, even though the checkpoint is real.
func TestV3CursorCurrentCheckpointDisposableMySQL(t *testing.T) {
	dsn := os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if dsn == "" {
		t.Skip("network-none disposable MySQL only")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("disposable MySQL unavailable")
	}
	var name, version string
	if db.Raw("SELECT DATABASE()").Scan(&name).Error != nil || !strings.HasPrefix(name, "jc_v3_audit_") || db.Raw("SELECT VERSION()").Scan(&version).Error != nil || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("refuse non-disposable DB")
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	pool.SetMaxOpenConns(4)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db = db.WithContext(ctx)
	const id uint64 = 99801
	const proxy = "0x8989898989898989898989898989898989898989"
	oldHash, newHash := "0x"+strings.Repeat("a", 64), "0x"+strings.Repeat("b", 64)
	seed := func(q string, args ...any) {
		t.Helper()
		if e := db.Exec(q, args...).Error; e != nil {
			t.Fatal(e)
		}
	}
	seed(`INSERT INTO ido_v3_runtime(chain_id,proxy_address,implementation_address,proxy_code_hash,implementation_code_hash,governor_address,start_block) VALUES(31337,?,'0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',CONCAT('0x',REPEAT('a',64)),CONCAT('0x',REPEAT('b',64)),'0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',1)`, proxy)
	seed(`INSERT INTO projects(project_id,status,funding_mode,contract_version,chain_id,contract_address,description,tags,team_info,roadmap,partners) VALUES(?,'Set','capped_presale','ido_v3_stablecoin',31337,?,'','','','','')`, id, proxy)
	seed(`INSERT INTO ido_v3_project_binding(project_id,chain_id,proxy_address) VALUES(?,31337,?)`, id, proxy)
	seed(`INSERT INTO ido_v3_scan_cursor(chain_id,proxy_address,project_id,cursor_block,cursor_hash,revision,state) VALUES(31337,?,?,90,?,1,'running')`, proxy, id, oldHash)
	seed(`INSERT INTO ido_v3_block_checkpoint(chain_id,proxy_address,project_id,block_number,block_hash) VALUES(31337,?,?,90,?)`, proxy, id, oldHash)
	reader := db.Begin(&sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if reader.Error != nil {
		t.Fatal(reader.Error)
	}
	defer reader.Rollback()
	var snapshot, readerConnection uint64
	if reader.Raw(`SELECT CONNECTION_ID()`).Scan(&readerConnection).Error != nil {
		t.Fatal("reader connection")
	}
	if reader.Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE project_id=?`, id).Scan(&snapshot).Error != nil || snapshot != 90 {
		t.Fatal("old snapshot not established")
	}
	// A separate transaction stands in for the scanner's atomic publication.
	if err = db.Transaction(func(writer *gorm.DB) error {
		var connection uint64
		if e := writer.Raw(`SELECT CONNECTION_ID()`).Scan(&connection).Error; e != nil {
			return e
		}
		if connection == readerConnection {
			t.Fatal("fixture did not use separate connections")
		}
		if e := writer.Exec(`INSERT INTO ido_v3_block_checkpoint(chain_id,proxy_address,project_id,block_number,block_hash) VALUES(31337,?,?,91,?)`, proxy, id, newHash).Error; e != nil {
			return e
		}
		return writer.Exec(`UPDATE ido_v3_scan_cursor SET cursor_block=91,cursor_hash=?,revision=2 WHERE project_id=?`, newHash, id).Error
	}); err != nil {
		t.Fatal(err)
	}
	var current uint64
	if reader.Raw(`SELECT cursor_block FROM ido_v3_scan_cursor WHERE project_id=? FOR UPDATE`, id).Scan(&current).Error != nil || current != 91 {
		t.Fatal("locking cursor did not see committed scanner advance")
	}
	var staleCount int64
	if reader.Raw(`SELECT COUNT(*) FROM ido_v3_block_checkpoint WHERE project_id=? AND block_number=91 AND block_hash=? AND status='canonical'`, id, newHash).Scan(&staleCount).Error != nil || staleCount != 0 {
		t.Fatal("fixture did not reproduce invisible checkpoint in original snapshot")
	}
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: id, ProxyAddress: common.HexToAddress(proxy)}
	if err = LockV3RunningCursorTx(reader, binding); err != nil {
		t.Fatalf("canonical Job advance falsely rejected under old RR snapshot: %v", err)
	}
	if reader.Commit().Error != nil {
		t.Fatal("reader commit")
	}
	// No weakening: fresh missing/orphaned/hash-mismatched checkpoints and
	// halted cursors still fail. Restore fixture state between each case.
	for _, kind := range []string{"missing", "orphaned", "wrong-hash", "halted"} {
		t.Run(kind, func(t *testing.T) {
			seed(`UPDATE ido_v3_scan_cursor SET state='running',cursor_block=91,cursor_hash=? WHERE project_id=?`, newHash, id)
			seed(`UPDATE ido_v3_block_checkpoint SET status='canonical' WHERE project_id=? AND block_number=91`, id)
			switch kind {
			case "missing":
				seed(`UPDATE ido_v3_scan_cursor SET cursor_block=92 WHERE project_id=?`, id)
			case "orphaned":
				seed(`UPDATE ido_v3_block_checkpoint SET status='orphaned' WHERE project_id=? AND block_number=91`, id)
			case "wrong-hash":
				seed(`UPDATE ido_v3_scan_cursor SET cursor_hash=? WHERE project_id=?`, oldHash, id)
			case "halted":
				seed(`UPDATE ido_v3_scan_cursor SET state='reorg_halted' WHERE project_id=?`, id)
			}
			if e := db.Transaction(func(tx *gorm.DB) error { return LockV3RunningCursorTx(tx, binding) }); e == nil {
				t.Fatal("invalid canonical cursor admitted")
			}
		})
	}
	t.Log("RR snapshot reproduced; current cursor/checkpoint admitted on 2 connections; invalid evidence remained fail-closed")
}

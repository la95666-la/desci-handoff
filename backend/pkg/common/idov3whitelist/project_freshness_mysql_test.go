package idov3whitelist

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Exact production gate SQL against isolated TEMPORARY tables on one physical
// MySQL8.4 connection. No runtime identity/approval is invented by this test:
// that separate canonical candidate reader has independent fixture coverage.
func TestProjectFreshnessDisposableMySQLIsolationAndExpiry(t *testing.T) {
	dsn := os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if dsn == "" {
		t.Skip("disposable MySQL only")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	var name, version string
	db.Raw("SELECT DATABASE()").Scan(&name)
	db.Raw("SELECT VERSION()").Scan(&version)
	if !strings.HasPrefix(name, "jc_v3_audit_") || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("refuse non-disposable database")
	}
	err = db.Connection(func(conn *gorm.DB) error {
		exec := func(sql string, args ...any) {
			t.Helper()
			if err := conn.Exec(sql, args...).Error; err != nil {
				t.Fatal(err)
			}
		}
		ddl := []string{
			`CREATE TEMPORARY TABLE ido_v3_project_freshness(chain_id BIGINT,proxy_address VARCHAR(42),contract_version VARCHAR(32),project_id BIGINT,epoch BIGINT,process_token CHAR(64),cursor_block BIGINT,cursor_hash VARCHAR(66),cursor_revision BIGINT,target_block BIGINT,target_hash VARCHAR(66),evidence_digest CHAR(64),verified_at DATETIME(6))`,
			`CREATE TEMPORARY TABLE ido_v3_source_freshness(chain_id BIGINT,proxy_address VARCHAR(42),contract_version VARCHAR(32),epoch BIGINT,process_token CHAR(64),state VARCHAR(8),target_block BIGINT,observed_head BIGINT,verified_at DATETIME(6))`,
			`CREATE TEMPORARY TABLE ido_v3_proxy_stream(chain_id BIGINT,proxy_address VARCHAR(42),contract_version VARCHAR(32),epoch BIGINT,mode VARCHAR(8),state VARCHAR(8),cursor_block BIGINT,observed_head BIGINT,last_success_at DATETIME(6))`,
			`CREATE TEMPORARY TABLE ido_v3_scan_cursor(chain_id BIGINT,proxy_address VARCHAR(42),contract_version VARCHAR(32),project_id BIGINT,cursor_block BIGINT,cursor_hash VARCHAR(66),revision BIGINT,state VARCHAR(8))`,
			`CREATE TEMPORARY TABLE ido_v3_project_projection(chain_id BIGINT,proxy_address VARCHAR(42),contract_version VARCHAR(32),project_id BIGINT,cursor_block BIGINT,cursor_hash VARCHAR(66),revision BIGINT)`,
			`CREATE TEMPORARY TABLE ido_v3_block_checkpoint(chain_id BIGINT,proxy_address VARCHAR(42),contract_version VARCHAR(32),project_id BIGINT,block_number BIGINT,block_hash VARCHAR(66),status VARCHAR(12))`,
			`CREATE TEMPORARY TABLE ido_v3_proxy_coverage(chain_id BIGINT,proxy_address VARCHAR(42),contract_version VARCHAR(32),epoch BIGINT,block_number BIGINT,block_hash VARCHAR(66))`,
			`CREATE TEMPORARY TABLE chain_event_reorg_incident(chain_id BIGINT,contract_address VARCHAR(42),active_slot INT,status VARCHAR(16))`,
		}
		for _, sql := range ddl {
			exec(sql)
		}
		defer func() {
			for _, table := range []string{"ido_v3_project_freshness", "ido_v3_source_freshness", "ido_v3_proxy_stream", "ido_v3_scan_cursor", "ido_v3_project_projection", "ido_v3_block_checkpoint", "ido_v3_proxy_coverage", "chain_event_reorg_incident"} {
				conn.Exec("DROP TEMPORARY TABLE " + table)
			}
		}()
		hash := common.HexToHash("0x11").Hex()
		digest := strings.Repeat("a", 64)
		token := strings.Repeat("b", 64)
		seed := func() {
			for _, table := range []string{"ido_v3_project_freshness", "ido_v3_source_freshness", "ido_v3_proxy_stream", "ido_v3_scan_cursor", "ido_v3_project_projection", "ido_v3_block_checkpoint", "ido_v3_proxy_coverage", "chain_event_reorg_incident"} {
				exec("DELETE FROM " + table)
			}
			exec(`INSERT INTO ido_v3_project_freshness VALUES(56,?,'ido_v3_stablecoin',19,2,?,100,?,7,100,?,?,UTC_TIMESTAMP(6))`, approvedScanProxy, token, hash, hash, digest)
			exec(`INSERT INTO ido_v3_source_freshness VALUES(56,?,'ido_v3_stablecoin',2,?,'healthy',100,114,UTC_TIMESTAMP(6))`, approvedScanProxy, token)
			exec(`INSERT INTO ido_v3_proxy_stream VALUES(56,?,'ido_v3_stablecoin',2,'stream','running',105,119,UTC_TIMESTAMP(6))`, approvedScanProxy)
			exec(`INSERT INTO ido_v3_scan_cursor VALUES(56,?,'ido_v3_stablecoin',19,100,?,7,'running')`, approvedScanProxy, hash)
			exec(`INSERT INTO ido_v3_project_projection VALUES(56,?,'ido_v3_stablecoin',19,100,?,7)`, approvedScanProxy, hash)
			exec(`INSERT INTO ido_v3_block_checkpoint VALUES(56,?,'ido_v3_stablecoin',19,100,?,'canonical')`, approvedScanProxy, hash)
			exec(`INSERT INTO ido_v3_proxy_coverage VALUES(56,?,'ido_v3_stablecoin',2,100,?)`, approvedScanProxy, hash)
		}
		candidate := ApprovedLockedCandidate{CursorBlock: 100, CursorHash: common.HexToHash(hash), CursorRevision: 7}
		for _, tc := range []struct {
			name, change string
			valid        bool
		}{
			{"valid", "", true}, {"unrelated project stale", `INSERT INTO ido_v3_scan_cursor VALUES(56,'other','ido_v3_stablecoin',20,1,'bad',1,'halted')`, true},
			{"old project TTL no longer expires", `UPDATE ido_v3_project_freshness SET verified_at=UTC_TIMESTAMP(6)-INTERVAL 91 SECOND`, true},
			{"project within five minutes", `UPDATE ido_v3_project_freshness SET verified_at=UTC_TIMESTAMP(6)-INTERVAL 299 SECOND`, true},
			{"project expired", `UPDATE ido_v3_project_freshness SET verified_at=UTC_TIMESTAMP(6)-INTERVAL 301 SECOND`, false},
			{"future stamp", `UPDATE ido_v3_project_freshness SET verified_at=UTC_TIMESTAMP(6)+INTERVAL 1 SECOND`, false},
			{"source checking", `UPDATE ido_v3_source_freshness SET state='checking'`, false},
			{"source expired", `UPDATE ido_v3_source_freshness SET verified_at=UTC_TIMESTAMP(6)-INTERVAL 91 SECOND`, false},
			{"producer expired", `UPDATE ido_v3_proxy_stream SET last_success_at=UTC_TIMESTAMP(6)-INTERVAL 91 SECOND`, false},
			{"epoch changed", `UPDATE ido_v3_proxy_stream SET epoch=3`, false},
			{"restart", `UPDATE ido_v3_source_freshness SET process_token=REPEAT('c',64)`, false},
			{"cursor moved", `UPDATE ido_v3_scan_cursor SET cursor_block=101,revision=8`, false},
			{"cursor halted", `UPDATE ido_v3_scan_cursor SET state='halted'`, false},
			{"projection revision mismatch", `UPDATE ido_v3_project_projection SET revision=8`, false},
			{"approval digest changed", `UPDATE ido_v3_project_freshness SET evidence_digest=REPEAT('d',64)`, false},
			{"old 120 boundary now admitted", `UPDATE ido_v3_proxy_stream SET cursor_block=221,observed_head=235`, true},
			{"five minute boundary admitted", `UPDATE ido_v3_proxy_stream SET cursor_block=760,observed_head=774`, true},
			{"five minute boundary exceeded", `UPDATE ido_v3_proxy_stream SET cursor_block=761,observed_head=775`, false},
			{"source 135 boundary admitted", `UPDATE ido_v3_proxy_stream SET observed_head=240`, true},
			{"source 135 boundary exceeded", `UPDATE ido_v3_proxy_stream SET observed_head=241`, false},
			{"checkpoint orphaned", `UPDATE ido_v3_block_checkpoint SET status='orphaned'`, false},
			{"coverage removed", `DELETE FROM ido_v3_proxy_coverage`, false},
			{"global reorg", `INSERT INTO chain_event_reorg_incident VALUES(56,'0x0d4040875742975269c55e16c8884d3ee9510b8e',1,'detected')`, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				seed()
				if tc.change != "" {
					exec(tc.change)
				}
				err := requireProjectFreshnessSnapshot(context.Background(), conn, 19, candidate, digest)
				if (err == nil) != tc.valid {
					t.Fatal(tc.name, err)
				}
			})
		}
		// Checking an existing certificate never changes its established TTL;
		// failed replacement leaves its original committed timestamp intact.
		seed()
		exec(`UPDATE ido_v3_project_freshness SET verified_at=UTC_TIMESTAMP(6)-INTERVAL 200 SECOND`)
		var oldStamp, newStamp string
		conn.Raw(`SELECT CAST(verified_at AS CHAR) FROM ido_v3_project_freshness`).Scan(&oldStamp)
		if err := requireProjectFreshnessSnapshot(context.Background(), conn, 19, candidate, digest); err != nil {
			t.Fatal("valid old certificate unavailable during rescan", err)
		}
		conn.Raw(`SELECT CAST(verified_at AS CHAR) FROM ido_v3_project_freshness`).Scan(&newStamp)
		if oldStamp != newStamp {
			t.Fatal("gate renewed certificate")
		}
		rollback := errors.New("publication rollback")
		err := conn.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(`UPDATE ido_v3_project_freshness SET verified_at=UTC_TIMESTAMP(6)`).Error; err != nil {
				return err
			}
			return rollback
		})
		if !errors.Is(err, rollback) {
			t.Fatal(err)
		}
		conn.Raw(`SELECT CAST(verified_at AS CHAR) FROM ido_v3_project_freshness`).Scan(&newStamp)
		if oldStamp != newStamp {
			t.Fatal("rollback lost original certificate")
		}
		if err := conn.Transaction(func(tx *gorm.DB) error {
			return tx.Exec(`UPDATE ido_v3_project_freshness SET verified_at=UTC_TIMESTAMP(6)`).Error
		}); err != nil {
			t.Fatal(err)
		}
		conn.Raw(`SELECT CAST(verified_at AS CHAR) FROM ido_v3_project_freshness`).Scan(&newStamp)
		if oldStamp == newStamp {
			t.Fatal("success did not publish new lifetime")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

package idov3whitelist

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	drivermysql "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Read the EXACT production query, not a hand-maintained test surrogate.
func candidateIndexSQL(t *testing.T) (string, string) {
	t.Helper()
	path := os.Getenv("JC_CANDIDATE_QUERY_SOURCE")
	if path == "" {
		path = "approved_locked_candidate.go"
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	match := regexp.MustCompile("const query = `([\\s\\S]*?)`").FindStringSubmatch(string(raw))
	if len(match) != 2 {
		t.Fatal("candidate query missing")
	}
	sql := match[1]
	legacy := sql
	for _, pair := range [][2]string{{"ck", "s"}, {"k", "j"}} {
		a, b := pair[0], pair[1]
		plain := "\n\t\t AND " + a + ".proxy_address=" + b + ".proxy_address"
		if strings.Count(sql, plain) != 1 || !strings.Contains(sql, "BINARY "+a+".proxy_address=BINARY "+b+".proxy_address") {
			t.Fatal("index locator or exact guard missing", a)
		}
		legacy = strings.Replace(legacy, plain, "", 1)
	}
	return sql, legacy
}

func TestCandidateIndexRetainsEveryOriginalPredicate(t *testing.T) {
	sql, old := candidateIndexSQL(t)
	if strings.Count(sql, "?") != 4 || strings.Count(old, "?") != 4 {
		t.Fatal("argument order changed")
	}
	for _, guard := range []string{"BINARY ck.block_hash=BINARY s.cursor_hash", "BINARY k.block_hash=BINARY j.block_hash", "BINARY ck.status=BINARY 'canonical'", "BINARY k.status=BINARY 'canonical'", "t.final_event_record_id=j.id", "t.active_slot IS NULL", "t.receipt_status=1", "BINARY t.receipt_block_hash=BINARY j.block_hash", "LIMIT 2"} {
		if !strings.Contains(sql, guard) {
			t.Fatal("guard removed", guard)
		}
	}
}

// Network-none MySQL8.4 only. Isolated schema mirrors the relevant composite
// primary keys, with30k quiet checkpoints, multiple proxy/project scopes and
// exact byte/canonical corruption cases. No production DSN may run this test.
func TestCandidateIndexDisposableMySQL(t *testing.T) {
	dsn := os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if dsn == "" {
		t.Skip("disposable MySQL only")
	}
	parent, e := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal("fixture database")
	}
	var name, version string
	if parent.Raw("SELECT DATABASE()").Scan(&name).Error != nil || !strings.HasPrefix(name, "jc_v3_audit_") || parent.Raw("SELECT VERSION()").Scan(&version).Error != nil || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("refuse non-disposable database")
	}
	isolated := fmt.Sprintf("jc_v3_audit_candidate_%d", time.Now().UnixNano())
	if e := parent.Exec("CREATE DATABASE " + isolated).Error; e != nil {
		t.Fatal(e)
	}
	defer parent.Exec("DROP DATABASE " + isolated)
	cfg, e := drivermysql.ParseDSN(dsn)
	if e != nil {
		t.Fatal("DSN")
	}
	cfg.DBName = isolated
	db, e := gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal("isolated database")
	}
	pool, _ := db.DB()
	defer pool.Close()
	exec := func(s string, a ...any) {
		t.Helper()
		if e := db.Exec(s, a...).Error; e != nil {
			t.Fatal(e)
		}
	}
	// Case-insensitive column collation proves the retained BINARY predicates
	// reject aliases that an ordinary index equality deliberately only locates.
	for _, ddl := range []string{
		`CREATE TABLE projects(project_id BIGINT UNSIGNED PRIMARY KEY,chain_id BIGINT UNSIGNED,contract_address VARCHAR(42),contract_version VARCHAR(32),funding_mode VARCHAR(32),onchain_locked TINYINT,token_address VARCHAR(42),payment_token_address VARCHAR(42),owner_address VARCHAR(42),fee_receiver VARCHAR(42),success_fee_rate INT,token_decimals INT,payment_token_decimals INT)`,
		`CREATE TABLE ido_v3_project_binding(project_id BIGINT UNSIGNED PRIMARY KEY,chain_id BIGINT UNSIGNED,proxy_address VARCHAR(42),contract_version VARCHAR(32))`,
		`CREATE TABLE ido_v3_set_create_commitment(project_id BIGINT UNSIGNED PRIMARY KEY,chain_id BIGINT UNSIGNED,proxy_address VARCHAR(42),contract_version VARCHAR(32),create_calldata_hash VARCHAR(66))`,
		`CREATE TABLE ido_v3_scan_cursor(chain_id BIGINT UNSIGNED,proxy_address VARCHAR(42) CHARACTER SET ascii,project_id BIGINT UNSIGNED,contract_version VARCHAR(32),cursor_block BIGINT UNSIGNED,cursor_hash VARCHAR(66),revision BIGINT UNSIGNED,state VARCHAR(16),PRIMARY KEY(chain_id,proxy_address,project_id))`,
		`CREATE TABLE ido_v3_block_checkpoint(chain_id BIGINT UNSIGNED,proxy_address VARCHAR(42) CHARACTER SET ascii,project_id BIGINT UNSIGNED,block_number BIGINT UNSIGNED,block_hash VARCHAR(66) CHARACTER SET ascii,status VARCHAR(16),contract_version VARCHAR(32),PRIMARY KEY(chain_id,proxy_address,project_id,block_number,block_hash),KEY checkpoint_status(chain_id,proxy_address,project_id,status,block_number))`,
		`CREATE TABLE ido_v3_event_journal(id BIGINT UNSIGNED PRIMARY KEY,chain_id BIGINT UNSIGNED,proxy_address VARCHAR(42) CHARACTER SET ascii,project_id BIGINT UNSIGNED,contract_version VARCHAR(32),block_number BIGINT UNSIGNED,block_hash VARCHAR(66),tx_hash VARCHAR(66),log_index INT,payload JSON,status VARCHAR(16),event_kind VARCHAR(32))`,
		`CREATE TABLE chain_tx_submission(id BIGINT UNSIGNED PRIMARY KEY,chain_id BIGINT UNSIGNED,project_id BIGINT UNSIGNED,contract_address VARCHAR(42),action VARCHAR(48),tx_hash VARCHAR(66),state VARCHAR(32),final_event_record_id BIGINT UNSIGNED,active_slot TINYINT NULL,receipt_status INT,receipt_block_number BIGINT UNSIGNED,receipt_block_hash VARCHAR(66),sender_address VARCHAR(42))`,
	} {
		exec(ddl + " DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci")
	}
	proxy := "0x0d4040875742975269c55e16c8884d3ee9510b8e"
	hash := "0x" + strings.Repeat("a", 64)
	txhash := "0x" + strings.Repeat("b", 64)
	exec(`INSERT INTO projects VALUES(19,56,?,'ido_v3_stablecoin','capped_presale',1,?,?,?, ?,100,18,18)`, proxy, proxy, proxy, proxy, proxy)
	exec(`INSERT INTO ido_v3_project_binding VALUES(19,56,?,'ido_v3_stablecoin')`, proxy)
	exec(`INSERT INTO ido_v3_set_create_commitment VALUES(19,56,?,'ido_v3_stablecoin',?)`, proxy, hash)
	exec(`INSERT INTO ido_v3_scan_cursor VALUES(56,?,19,'ido_v3_stablecoin',128805710,?,11,'running')`, proxy, hash)
	exec(`INSERT INTO ido_v3_event_journal VALUES(7,56,?,19,'ido_v3_stablecoin',128805700,?,?,0,'{"Created":{}}','canonical','created')`, proxy, hash, txhash)
	exec(`INSERT INTO chain_tx_submission VALUES(99,56,19,?,'ido_v3_create',?,'applied',7,NULL,1,128805700,?,?)`, proxy, txhash, hash, proxy)
	exec(`INSERT INTO ido_v3_block_checkpoint VALUES(56,?,19,128805700,?,'canonical','ido_v3_stablecoin'),(56,?,19,128805710,?,'canonical','ido_v3_stablecoin')`, proxy, hash, proxy, hash)
	exec(`CREATE TABLE digits(n INT PRIMARY KEY)`)
	exec(`INSERT INTO digits VALUES(0),(1),(2),(3),(4),(5),(6),(7),(8),(9)`)
	exec(`INSERT INTO ido_v3_block_checkpoint SELECT 56,IF(a.n<5,?,CONCAT('0x',REPEAT('f',40))),20+MOD(b.n,3),1+a.n+10*b.n+100*c.n+1000*d.n+10000*e.n,?,'canonical','ido_v3_stablecoin' FROM digits a CROSS JOIN digits b CROSS JOIN digits c CROSS JOIN digits d CROSS JOIN digits e WHERE e.n<3`, proxy, hash)
	optimized, legacy := candidateIndexSQL(t)
	exec("ANALYZE TABLE projects,ido_v3_project_binding,ido_v3_set_create_commitment,ido_v3_scan_cursor,ido_v3_block_checkpoint,ido_v3_event_journal,chain_tx_submission")
	args := []any{"applied", uint64(19), proxy, "running"}
	query := func(s string) []map[string]any {
		t.Helper()
		var r []map[string]any
		if e := db.Raw(s, args...).Scan(&r).Error; e != nil {
			t.Fatal(e)
		}
		return r
	}
	start := time.Now()
	old := query(legacy)
	oldTime := time.Since(start)
	start = time.Now()
	fast := query(optimized)
	fastTime := time.Since(start)
	if len(fast) != 1 || !reflect.DeepEqual(old, fast) {
		t.Fatal("canonical result parity failed")
	}
	var plan []struct {
		Table  string
		Key    string
		KeyLen string `gorm:"column:key_len"`
		Rows   uint64
	}
	if e := db.Raw("EXPLAIN "+optimized, args...).Scan(&plan).Error; e != nil {
		t.Fatal(e)
	}
	found := 0
	for _, p := range plan {
		if p.Table == "ck" || p.Table == "k" {
			found++
			if p.Key != "PRIMARY" || p.KeyLen == "8" || p.Rows > 4 {
				t.Fatal("checkpoint range scan returned", p)
			}
		}
	}
	if found != 2 {
		t.Fatal(plan)
	}
	for _, tc := range []struct {
		name, change string
		valid        bool
	}{
		{"cursor hash case", `UPDATE ido_v3_block_checkpoint SET block_hash=UPPER(block_hash) WHERE project_id=19 AND block_number=128805710`, false},
		{"proxy case", `UPDATE ido_v3_block_checkpoint SET proxy_address=UPPER(proxy_address) WHERE project_id=19`, false},
		{"version case", `UPDATE ido_v3_block_checkpoint SET contract_version='IDO_V3_STABLECOIN' WHERE project_id=19`, false},
		{"orphaned", `UPDATE ido_v3_block_checkpoint SET status='orphaned' WHERE project_id=19`, false},
		{"canonical case", `UPDATE ido_v3_block_checkpoint SET status='CANONICAL' WHERE project_id=19`, false},
		{"foreign project", `UPDATE ido_v3_block_checkpoint SET project_id=123 WHERE project_id=19`, false},
		{"missing creation checkpoint", `DELETE FROM ido_v3_block_checkpoint WHERE project_id=19 AND block_number=128805700`, false},
		{"halted cursor", `UPDATE ido_v3_scan_cursor SET state='reorg_halted'`, false},
		{"receipt mismatch", `UPDATE chain_tx_submission SET receipt_block_hash=CONCAT('0x',REPEAT('c',64))`, false},
		{"wrong action", `UPDATE chain_tx_submission SET action='ido_v3_end'`, false},
		{"active slot", `UPDATE chain_tx_submission SET active_slot=1`, false},
		{"ambiguous creation", `INSERT INTO ido_v3_event_journal SELECT 8,chain_id,proxy_address,project_id,contract_version,block_number,block_hash,tx_hash,1,payload,status,event_kind FROM ido_v3_event_journal WHERE id=7`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rollback := errors.New("fixture rollback")
			if e := db.Transaction(func(w *gorm.DB) error {
				if e := w.Exec(tc.change).Error; e != nil {
					return e
				}
				var a, b []map[string]any
				if e := w.Raw(legacy, args...).Scan(&a).Error; e != nil {
					return e
				}
				if e := w.Raw(optimized, args...).Scan(&b).Error; e != nil {
					return e
				}
				if !reflect.DeepEqual(a, b) || (len(b) > 0) != tc.valid {
					t.Fatal("residual predicate/parity changed", len(a), len(b))
				}
				return rollback
			}); !errors.Is(e, rollback) {
				t.Fatal("fixture query/rollback failed", e)
			}
		})
	}
	// Halted recovery uses the same optimized query with its original state pair.
	exec(`UPDATE ido_v3_scan_cursor SET state='reorg_halted'`)
	exec(`UPDATE chain_tx_submission SET state='v3_reorg_suspended'`)
	args[0], args[3] = "v3_reorg_suspended", "reorg_halted"
	if a, b := query(legacy), query(optimized); len(b) != 1 || !reflect.DeepEqual(a, b) {
		t.Fatal("halted recovery parity")
	}
	t.Logf("30k decoy checkpoints: legacy=%s indexed=%s;ck/k existing PK point lookup;running/halted/case/orphan/receipt parity", oldTime, fastTime)
}

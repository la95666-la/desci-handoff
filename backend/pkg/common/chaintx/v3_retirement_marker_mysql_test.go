package chaintx

import (
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"strings"
	"testing"
	"time"
)

// Called inside rollback-only network-none MySQL8.4 retirement fixture. Large
// quiet history must not change Go/SQL admission or JSON-row scan cost.
func testRetirementMarkerIndexMySQL(t *testing.T, tx *gorm.DB, chain, project uint64, proxy string, submission uint64) {
	t.Helper()
	exec := func(q string, args ...any) {
		t.Helper()
		if e := tx.Exec(q, args...).Error; e != nil {
			t.Fatal(e)
		}
	}
	// The exact primary key keeps ordinary coverage rows distinct. 20k x1KiB
	// payloads are enough to expose the old scan plan without a huge test dump.
	exec(`INSERT INTO ido_v3_proxy_coverage(chain_id,proxy_address,contract_version,epoch,block_number,block_hash,parent_hash,digest,payload)
 SELECT ?,?,'ido_v3_stablecoin',1,1000+a.n+10*b.n+100*c.n+1000*d.n+10000*z.n,CONCAT('0x',REPEAT('a',64)),CONCAT('0x',REPEAT('b',64)),REPEAT('c',64),JSON_OBJECT('Quiet',REPEAT('x',1024))
 FROM (SELECT 0 n UNION ALL SELECT 1 UNION ALL SELECT 2 UNION ALL SELECT 3 UNION ALL SELECT 4 UNION ALL SELECT 5 UNION ALL SELECT 6 UNION ALL SELECT 7 UNION ALL SELECT 8 UNION ALL SELECT 9) a
 CROSS JOIN (SELECT 0 n UNION ALL SELECT 1 UNION ALL SELECT 2 UNION ALL SELECT 3 UNION ALL SELECT 4 UNION ALL SELECT 5 UNION ALL SELECT 6 UNION ALL SELECT 7 UNION ALL SELECT 8 UNION ALL SELECT 9) b
 CROSS JOIN (SELECT 0 n UNION ALL SELECT 1 UNION ALL SELECT 2 UNION ALL SELECT 3 UNION ALL SELECT 4 UNION ALL SELECT 5 UNION ALL SELECT 6 UNION ALL SELECT 7 UNION ALL SELECT 8 UNION ALL SELECT 9) c
 CROSS JOIN (SELECT 0 n UNION ALL SELECT 1 UNION ALL SELECT 2 UNION ALL SELECT 3 UNION ALL SELECT 4 UNION ALL SELECT 5 UNION ALL SELECT 6 UNION ALL SELECT 7 UNION ALL SELECT 8 UNION ALL SELECT 9) d
 CROSS JOIN (SELECT 0 n UNION ALL SELECT 1) z`, chain, proxy)
	var plan []struct {
		Key  string
		Rows uint64
		Type string
	}
	if tx.Raw(`EXPLAIN SELECT COUNT(*) FROM ido_v3_proxy_coverage FORCE INDEX(ix_v3_coverage_retirement_marker) WHERE chain_id=? AND proxy_address=? AND contract_version='ido_v3_stablecoin' AND retirement_marker=1 AND JSON_CONTAINS(payload,CAST(? AS JSON),'$.RetiredTests')`, chain, proxy, project).Scan(&plan).Error != nil || len(plan) != 1 || plan[0].Key != "ix_v3_coverage_retirement_marker" || plan[0].Rows > 4 {
		t.Fatal("marker index not selective", plan)
	}
	start := time.Now()
	if e := requireV3RetirementMarker(tx, chain, proxy, project); e != nil {
		t.Fatal(e)
	}
	var original, optimized uint64
	if tx.Raw(`SELECT COUNT(*) FROM ido_v3_proxy_coverage WHERE chain_id=? AND proxy_address=? AND JSON_CONTAINS(payload,CAST(? AS JSON),'$.RetiredTests')`, chain, proxy, project).Scan(&original).Error != nil {
		t.Fatal("reference")
	}
	if tx.Raw(`SELECT COUNT(*) FROM ido_v3_proxy_coverage FORCE INDEX(ix_v3_coverage_retirement_marker) WHERE chain_id=? AND proxy_address=? AND contract_version='ido_v3_stablecoin' AND retirement_marker=1 AND JSON_CONTAINS(payload,CAST(? AS JSON),'$.RetiredTests')`, chain, proxy, project).Scan(&optimized).Error != nil || optimized != original || optimized != 1 {
		t.Fatal("membership changed", original, optimized)
	}
	// SQL view uses same index. It must preserve exact id admission/expiry /
	// journal/root facts while indexing only marker envelopes.
	var admitted []uint64
	if tx.Raw(`SELECT id FROM ido_v3_retired_nonce_exemption WHERE id=?`, submission).Scan(&admitted).Error != nil || len(admitted) != 1 {
		t.Fatal("SQL exemption parity", admitted)
	}
	var viewPlan []struct {
		Table, Key string
		Rows       uint64
	}
	if tx.Raw(`EXPLAIN SELECT id FROM ido_v3_retired_nonce_exemption WHERE id=?`, submission).Scan(&viewPlan).Error != nil {
		t.Fatal("view plan unavailable")
	}
	found := false
	for _, step := range viewPlan {
		if step.Table == "c" {
			found = true
			if step.Key != "ix_v3_coverage_retirement_marker" || step.Rows > 4 {
				t.Fatal("SQL view still scans coverage", viewPlan)
			}
		}
	}
	if !found {
		t.Fatal("view coverage plan not verified", viewPlan)
	}
	if requireV3RetirementMarker(tx, chain, proxy, project+1) == nil {
		t.Fatal("foreign project admitted")
	}
	var payload string
	tx.Raw(`SELECT payload FROM ido_v3_proxy_coverage WHERE chain_id=? AND proxy_address=? AND contract_version='ido_v3_stablecoin' AND epoch=1 AND block_number=110`, chain, proxy).Scan(&payload)
	// Duplicate marker must still fail the Go exact-one guard.
	if !strings.Contains(payload, "RetiredTests") {
		t.Fatal("marker fixture missing")
	}
	exec(`INSERT INTO ido_v3_proxy_coverage(chain_id,proxy_address,contract_version,epoch,block_number,block_hash,parent_hash,digest,payload) VALUES(?,?,'ido_v3_stablecoin',1,900,?,?,REPEAT('d',64),?)`, chain, proxy, common.HexToHash("0x111").Hex(), common.HexToHash("0x222").Hex(), payload)
	if requireV3RetirementMarker(tx, chain, proxy, project) == nil {
		t.Fatal("duplicate marker accepted")
	}
	raw, _ := json.Marshal(plan)
	t.Logf("20k quiet rows: indexed marker estimate<=4, exact membership/view parity retained, duration=%s plan=%s", time.Since(start), raw)
}

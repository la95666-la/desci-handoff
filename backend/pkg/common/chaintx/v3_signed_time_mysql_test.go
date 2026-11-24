package chaintx

import (
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/entity"
)

// Runs only in the network-none disposable MySQL 8.4 container. It verifies
// the raw atomic signing update, read-only shared entity field, UTC deadline
// and 1021 trigger together rather than a mock query.
func TestV3SignedTimeDisposableMySQLReadyTransition(t *testing.T) {
	dsn := os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if dsn == "" {
		t.Skip("disposable V3 MySQL DSN is not set")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal("disposable database unavailable")
	}
	var name, version string
	if db.Raw("SELECT DATABASE()").Scan(&name).Error != nil || !strings.HasPrefix(name, "jc_v3_audit_") ||
		db.Raw("SELECT VERSION()").Scan(&version).Error != nil || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("refuse non-disposable V3 database")
	}
	const proxy = "0x8888888888888888888888888888888888888888"
	const operator = "0x1111111111111111111111111111111111111111"
	if err := db.Exec(`INSERT INTO ido_v3_runtime(chain_id,proxy_address,implementation_address,proxy_code_hash,
		implementation_code_hash,governor_address,start_block) VALUES(31337,?,
		'0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',CONCAT('0x',REPEAT('a',64)),
		CONCAT('0x',REPEAT('b',64)),'0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',1)`, proxy).Error; err != nil {
		t.Fatal("disposable runtime seed failed")
	}
	if err := db.Exec(`INSERT INTO projects(project_id,status,funding_mode,contract_version,chain_id,contract_address,
		description,tags,team_info,roadmap,partners) VALUES(90,'Set','capped_presale','ido_v3_stablecoin',31337,?,'','','','','')`, proxy).Error; err != nil {
		t.Fatal("disposable project seed failed")
	}
	if err := db.Exec(`INSERT INTO ido_v3_project_binding(project_id,chain_id,proxy_address) VALUES(90,31337,?)`, proxy).Error; err != nil {
		t.Fatal("disposable binding seed failed")
	}
	if err := db.Exec(`INSERT INTO ido_v3_set_create_commitment(project_id,chain_id,proxy_address,request_digest,
		manifest_digest,create_calldata_hash,operator_address) VALUES(90,31337,?,CONCAT('0x',REPEAT('1',64)),
		CONCAT('0x',REPEAT('2',64)),CONCAT('0x',REPEAT('3',64)),?)`, proxy, operator).Error; err != nil {
		t.Fatal("disposable Set commitment seed failed")
	}
	if err := db.Exec(`INSERT INTO chain_tx_submission(project_id,action,attempt_no,chain_id,sender_address,
		contract_address,call_data,state,active_slot,last_error) VALUES(90,'ido_v3_create',1,31337,?, ?,X'01020304','ready',1,'')`, operator, proxy).Error; err != nil {
		t.Fatal("disposable ready attempt seed failed")
	}
	deadline := time.Now().Add(time.Minute).UTC()
	var readyID uint64
	if db.Raw(`SELECT id FROM chain_tx_submission WHERE project_id=90 AND action='ido_v3_create'`).Scan(&readyID).Error != nil || readyID == 0 {
		t.Fatal("ready id unavailable")
	}
	// A future Set deadline with only 17 seconds remaining MUST NOT persist
	// a signature. Reproduce #99 without any signing/send capability.
	cutoff := time.Now().Add(17*time.Second - V3CreateSigningReserve).UTC()
	near := db.Exec(v3PersistSignedSQL, uint64(7), uint64(100000), "1", []byte{1}, "0x"+strings.Repeat("a", 64), time.Now(), readyID, cutoff.Format("2006-01-02 15:04:05"))
	if near.Error != nil || near.RowsAffected != 0 {
		t.Fatal("near deadline persisted signed bytes")
	}
	// Crossing the Set deadline cannot persist bytes, hash or a signing
	// timestamp, even when the request was admitted before this SQL step.
	expired := time.Now().Add(-time.Minute).UTC().Format("2006-01-02 15:04:05")
	denied := db.Exec(`UPDATE chain_tx_submission SET state='signed',nonce=?,gas_limit=?,gas_price_raw=?,
		raw_tx=?,tx_hash=?,v3_signed_at=UTC_TIMESTAMP(6),updated_at=UTC_TIMESTAMP(6)
		WHERE project_id=90 AND action='ido_v3_create' AND state='ready' AND active_slot=1
		AND nonce IS NULL AND raw_tx IS NULL AND tx_hash IS NULL AND UTC_TIMESTAMP(6) < ?`,
		uint64(7), uint64(100000), "1", []byte{1}, "0x"+strings.Repeat("a", 64), expired)
	if denied.Error != nil || denied.RowsAffected != 0 {
		t.Fatal("expired Set persisted a signed attempt")
	}
	var untouched struct {
		State  string     `gorm:"column:state"`
		Hash   *string    `gorm:"column:tx_hash"`
		Signed *time.Time `gorm:"column:v3_signed_at"`
	}
	if db.Raw(`SELECT state,tx_hash,v3_signed_at FROM chain_tx_submission WHERE project_id=90 AND action='ido_v3_create'`).Scan(&untouched).Error != nil ||
		untouched.State != "ready" || untouched.Hash != nil || untouched.Signed != nil {
		t.Fatal("expired signing attempt changed durable outbox")
	}
	// Exercise the same atomic SQL shape as PrepareV3Once, including the
	// UTC deadline and signed bytes (never exposed outside this scratch DB).
	changed := db.Exec(v3PersistSignedSQL,
		uint64(7), uint64(100000), "1", []byte{1}, "0x"+strings.Repeat("a", 64), time.Now(), readyID, deadline.Format("2006-01-02 15:04:05"))
	if changed.Error != nil || changed.RowsAffected != 1 {
		t.Fatal("V3 ready-to-signed timestamp update unavailable")
	}
	var row entity.ChainTxSubmission
	if db.Where("project_id=90 AND action='ido_v3_create'").Take(&row).Error != nil {
		t.Fatal("signed attempt read unavailable")
	}
	if row.V3SignedAt == nil || row.V3SignedAt.IsZero() ||
		!V3SignedBeforeSet(row, deadline.Unix()) || V3SignedBeforeSet(row, time.Now().Add(-time.Second).Unix()) {
		var direct struct {
			Signed *time.Time `gorm:"column:v3_signed_at"`
		}
		_ = db.Raw("SELECT v3_signed_at FROM chain_tx_submission WHERE project_id=90 AND action='ido_v3_create'").Scan(&direct).Error
		t.Fatalf("immutable UTC signing time was not persisted or interpreted: modelPresent=%t directPresent=%t beforeDeadline=%t beforePast=%t",
			row.V3SignedAt != nil, direct.Signed != nil, V3SignedBeforeSet(row, deadline.Unix()), V3SignedBeforeSet(row, time.Now().Add(-time.Second).Unix()))
	}
	if db.Exec(`UPDATE chain_tx_submission SET v3_signed_at=UTC_TIMESTAMP(6)+INTERVAL 1 SECOND
		WHERE project_id=90 AND action='ido_v3_create'`).Error == nil {
		t.Fatal("V3 signing time rewrite bypassed 1021 trigger")
	}
}

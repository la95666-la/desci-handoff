package chaintx

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"os"
	"strings"
	"testing"
)

func TestOriginal99DispositionDisposableMySQLGuards(t *testing.T) {
	dsn := os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if dsn == "" {
		t.Skip("disposable only")
	}
	db, e := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if e != nil {
		t.Fatal("fixture")
	}
	var name, version string
	db.Raw("SELECT DATABASE()").Scan(&name)
	db.Raw("SELECT VERSION()").Scan(&version)
	if !strings.HasPrefix(name, "jc_v3_audit_") || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("refuse non-disposable")
	}
	// Unknown attempts may not be manufactured as disposed. Trigger protects
	// INSERT, while existing V3 guards continue rejecting unsigned/altered rows.
	r := db.Exec(`INSERT INTO chain_tx_submission(project_id,action,attempt_no,chain_id,sender_address,contract_address,call_data,value_raw,state,nonce,gas_limit,gas_price_raw,raw_tx,tx_hash,active_slot,last_error)
 VALUES(90,'ido_v3_create',77,31337,'0x1111111111111111111111111111111111111111','0x8888888888888888888888888888888888888888',X'01020304','0','v3_nonce_disposed',7,21000,'1',X'01',CONCAT('0x',REPEAT('c',64)),NULL,'')`)
	if r.Error == nil {
		t.Fatal("fabricated disposed state accepted")
	}
	// An unrelated signed original cannot gain the new terminal state without
	// its canonical linked disposition receipt. No direct slot clearing.
	r = db.Exec(`UPDATE chain_tx_submission SET state='v3_nonce_disposed',active_slot=NULL WHERE project_id=90 AND action='ido_v3_create'`)
	if r.Error == nil {
		t.Fatal("unlinked original disposal accepted")
	}
	// SQL authority records must exist before any disposition is inserted.
	r = db.Exec(`INSERT INTO ido_v3_original99_disposition(original_id,original_hash,original_raw_sha256,chain_id,sender_address,nonce,target_address,gas_limit,gas_price_raw,value_raw,raw_tx,tx_hash,state,authorization_ref)
 VALUES(99,?,REPEAT('a',64),56,?,81,?,21000,'100000000','0',X'01',CONCAT('0x',REPEAT('b',64)),'signed',?)`, Original99Hash, original99Sender, original99Sender, original99Authorization)
	if r.Error == nil {
		t.Fatal("missing original fence accepted")
	}
	var triggers int64
	if db.Raw(`SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() AND TRIGGER_NAME IN ('jc_original99_insert','jc_original99_update','jc_original99_no_delete','jc_original99_submission','jc_original99_no_fabricated_insert','jc_original99_nonce')`).Scan(&triggers).Error != nil || triggers != 6 {
		t.Fatal("guards missing", triggers)
	}
	// Exercise the full *SQL* state machine using disposable fake signed bytes.
	// No private key, RPC or production data is used or copied into this DB.
	tx := db.Begin()
	defer tx.Rollback()
	exec := func(sql string, args ...any) {
		t.Helper()
		if e := tx.Exec(sql, args...).Error; e != nil {
			t.Fatal(e)
		}
	}
	proxy := "0x0d4040875742975269c55e16c8884d3ee9510b8e"
	var runtime int64
	if tx.Raw(`SELECT COUNT(*) FROM ido_v3_runtime WHERE chain_id=56 AND proxy_address=?`, proxy).Scan(&runtime).Error != nil || runtime != 1 {
		t.Fatal("approved fixture runtime missing")
	}
	exec(`INSERT INTO projects(project_id,status,funding_mode,contract_version,chain_id,contract_address,description,tags,team_info,roadmap,partners) VALUES(?,'Set','capped_presale','ido_v3_stablecoin',56,?,'','','','','')`, Original99Project, proxy)
	exec(`INSERT INTO ido_v3_project_binding(project_id,chain_id,proxy_address) VALUES(?,56,?)`, Original99Project, proxy)
	exec(`INSERT INTO ido_v3_set_create_commitment(project_id,chain_id,proxy_address,request_digest,manifest_digest,create_calldata_hash,operator_address) VALUES(?,56,?,CONCAT('0x',REPEAT('1',64)),CONCAT('0x',REPEAT('2',64)),CONCAT('0x',REPEAT('3',64)),?)`, Original99Project, proxy, original99Sender)
	exec(`INSERT INTO chain_tx_submission(id,project_id,action,attempt_no,chain_id,sender_address,contract_address,call_data,state,active_slot,last_error) VALUES(99,?,'ido_v3_create',1,56,?,?,X'01020304','ready',1,'')`, Original99Project, original99Sender, proxy)
	exec(`UPDATE chain_tx_submission SET state='signed',nonce=81,gas_limit=822863,gas_price_raw='50000000',raw_tx=X'0102',tx_hash=?,v3_signed_at=UTC_TIMESTAMP(6) WHERE id=99`, Original99Hash)
	exec(`INSERT INTO chain_relayer_nonce(chain_id,sender_address,next_nonce,active_submission_id) VALUES(56,?,82,99)`, original99Sender)
	exec(`INSERT INTO ido_v3_original99_disposition(original_id,original_hash,original_raw_sha256,chain_id,sender_address,nonce,target_address,gas_limit,gas_price_raw,value_raw,raw_tx,tx_hash,state,authorization_ref)
 VALUES(99,?,LOWER(SHA2(X'0102',256)),56,?,81,?,21000,'100000000','0',X'0304',CONCAT('0x',REPEAT('b',64)),'signed',?)`, Original99Hash, original99Sender, original99Sender, original99Authorization)
	if tx.Exec(`UPDATE ido_v3_original99_disposition SET gas_price_raw='200000000' WHERE original_id=99`).Error == nil {
		t.Fatal("signed fee changed")
	}
	exec(`UPDATE ido_v3_original99_disposition SET state='broadcast_unknown',broadcast_count=1,broadcast_marked_at=UTC_TIMESTAMP(6) WHERE original_id=99`)
	if tx.Exec(`UPDATE ido_v3_original99_disposition SET state='broadcast_unknown',broadcast_count=1 WHERE original_id=99`).Error == nil {
		t.Fatal("second send marker accepted")
	}
	if tx.Exec(`UPDATE ido_v3_original99_disposition SET state='confirmed' WHERE original_id=99`).Error == nil {
		t.Fatal("NULL receipt accepted")
	}
	exec(`INSERT INTO ido_v3_proxy_stream(chain_id,proxy_address,contract_version,mode,state,anchor_block,cursor_block,cursor_hash,revision,epoch) VALUES(56,?,'ido_v3_stablecoin','stream','running',1,100,CONCAT('0x',REPEAT('a',64)),1,1)`, proxy)
	exec(`INSERT INTO ido_v3_proxy_coverage(chain_id,proxy_address,contract_version,epoch,block_number,block_hash,parent_hash,digest,payload) VALUES(56,?,'ido_v3_stablecoin',1,100,CONCAT('0x',REPEAT('a',64)),CONCAT('0x',REPEAT('b',64)),REPEAT('c',64),'{}')`, proxy)
	exec(`UPDATE ido_v3_original99_disposition SET state='confirmed',receipt_status=1,receipt_block=100,receipt_hash=CONCAT('0x',REPEAT('a',64)),confirmed_block=114,confirmed_hash=CONCAT('0x',REPEAT('c',64)),stream_epoch=1 WHERE original_id=99`)
	exec(`UPDATE chain_tx_submission SET state='v3_nonce_disposed',active_slot=NULL WHERE id=99`)
	exec(`UPDATE chain_relayer_nonce SET active_submission_id=NULL WHERE chain_id=56 AND sender_address=?`, original99Sender)
	var original struct {
		State   string
		Nonce   uint64
		Hash    string
		Receipt *uint64
	}
	if tx.Raw(`SELECT state,nonce,tx_hash AS hash,receipt_status AS receipt FROM chain_tx_submission WHERE id=99`).Scan(&original).Error != nil || original.State != "v3_nonce_disposed" || original.Nonce != 81 || original.Hash != Original99Hash || original.Receipt != nil {
		t.Fatal("original history corrupted", original)
	}
	exec(`UPDATE ido_v3_proxy_stream SET epoch=2 WHERE chain_id=56 AND proxy_address=?`, proxy)
	if tx.Exec(`UPDATE chain_relayer_nonce SET active_submission_id=100,next_nonce=83 WHERE chain_id=56 AND sender_address=?`, original99Sender).Error == nil {
		t.Fatal("reorg did not fence new signing")
	}
	t.Log("original99 migration twice; INSERT/update disposal, absent original and immutable source guards remain closed")
}

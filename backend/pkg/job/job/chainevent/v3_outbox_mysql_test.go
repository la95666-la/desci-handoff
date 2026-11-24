package chainevent

import (
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
	"joincare_server/pkg/common/idov3whitelist"
)

// Own an isolated, synthetic original signed event. Prove Job suspension and
// the shared V2/V3 sender trigger without depending on another test process
// or inferring canonicality from this fixture alone.
func TestV3OutboxDisposableMySQLReorgSenderFence(t *testing.T) {
	dsn := os.Getenv("JOINCARE_IDO_V3_MYSQL_WRITER_DSN")
	if dsn == "" {
		t.Skip("disposable V3 MySQL DSN is not set")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("disposable database unavailable")
	}
	var name, version string
	if db.Raw("SELECT DATABASE()").Scan(&name).Error != nil || !strings.HasPrefix(name, "jc_v3_audit_") ||
		db.Raw("SELECT VERSION()").Scan(&version).Error != nil || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("refuse non-disposable V3 database")
	}
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 94,
		ProxyAddress: common.HexToAddress("0x9494949494949494949494949494949494949494")}
	proxy := strings.ToLower(binding.ProxyAddress.Hex())
	key, err := crypto.HexToECDSA(strings.Repeat("8", 64))
	if err != nil {
		t.Fatal(err)
	}
	operator := crypto.PubkeyToAddress(key.PublicKey)
	data, err := commonchain.PackV3Lifecycle("activateProject", big.NewInt(94))
	if err != nil {
		t.Fatal(err)
	}
	signed, err := types.SignTx(types.NewTx(&types.LegacyTx{Nonce: 7, To: &binding.ProxyAddress,
		Gas: 100000, GasPrice: big.NewInt(1), Data: data}), types.LatestSignerForChainID(big.NewInt(31337)), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	seed := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal("disposable reorg fixture unavailable")
		}
	}
	seed(`INSERT INTO ido_v3_runtime(chain_id,proxy_address,implementation_address,proxy_code_hash,
		implementation_code_hash,governor_address,start_block) VALUES(31337,?,
		'0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',CONCAT('0x',REPEAT('a',64)),
		CONCAT('0x',REPEAT('b',64)),'0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',1)`, proxy)
	seed(`INSERT INTO projects(project_id,status,funding_mode,contract_version,chain_id,contract_address,
		token_address,payment_token_address,description,tags,team_info,roadmap,partners)
		VALUES(94,'Set','capped_presale','ido_v3_stablecoin',31337,?,
		'0x1111111111111111111111111111111111111111','0x2222222222222222222222222222222222222222',
		'','','','','')`, proxy)
	seed(`INSERT INTO ido_v3_project_binding(project_id,chain_id,proxy_address) VALUES(94,31337,?)`, proxy)
	seed(`INSERT INTO ido_v3_set_create_commitment(project_id,chain_id,proxy_address,request_digest,
		manifest_digest,create_calldata_hash,operator_address) VALUES(94,31337,?,
		CONCAT('0x',REPEAT('1',64)),CONCAT('0x',REPEAT('2',64)),CONCAT('0x',REPEAT('3',64)),?)`,
		proxy, strings.ToLower(operator.Hex()))
	seed(`UPDATE projects SET status='Pending',onchain_locked=1 WHERE project_id=94 AND status='Set'`)
	seed(`INSERT INTO chain_tx_submission(project_id,action,attempt_no,chain_id,sender_address,
		contract_address,call_data,state,active_slot,last_error)
		VALUES(94,'ido_v3_activate',1,31337,?,?,?,'ready',1,'')`, operator.Hex(), proxy, data)
	seed(`UPDATE chain_tx_submission SET state='signed',nonce=7,gas_limit=100000,gas_price_raw='1',
		raw_tx=?,tx_hash=?,v3_signed_at=UTC_TIMESTAMP(6) WHERE project_id=94 AND action='ido_v3_activate' AND state='ready'`,
		raw, signed.Hash().Hex())
	blockHash := "0x" + strings.Repeat("d", 64)
	seed(`INSERT INTO ido_v3_event_journal(chain_id,proxy_address,project_id,block_number,block_hash,
		tx_hash,log_index,event_kind,payload) VALUES(31337,?,94,91,?,?,0,'ProjectActivated','{}')`,
		proxy, blockHash, strings.ToLower(signed.Hash().Hex()))
	var journalID uint64
	if db.Raw(`SELECT id FROM ido_v3_event_journal WHERE chain_id=31337 AND proxy_address=?
		AND project_id=94 AND tx_hash=?`, proxy, strings.ToLower(signed.Hash().Hex())).Scan(&journalID).Error != nil || journalID == 0 {
		t.Fatal("disposable reorg journal fixture unavailable")
	}
	seed(`UPDATE chain_tx_submission SET state='applied',active_slot=NULL,receipt_status=1,
		receipt_block_number=91,receipt_block_hash=?,final_event_record_id=?
		WHERE project_id=94 AND action='ido_v3_activate' AND state='signed'`, blockHash, journalID)
	var original entity.ChainTxSubmission
	read := db.Where("project_id=94 AND action='ido_v3_activate'").Take(&original)
	if read.Error != nil || original.State != constant.ChainTxStateApplied ||
		original.FinalEventRecordID == nil || original.TxHash == nil {
		t.Fatalf("applied original-hash fixture missing: query_failed=%t state=%q journal_present=%t hash_present=%t",
			read.Error != nil, original.State, original.FinalEventRecordID != nil, original.TxHash != nil)
	}
	seed(`INSERT INTO chain_relayer_nonce(chain_id,sender_address,next_nonce,active_submission_id)
		VALUES(31337,?,8,NULL)`, operator.Hex())
	if err := db.Transaction(func(tx *gorm.DB) error { return suspendV3AppliedOutboxTx(tx, binding, nil) }); err != nil {
		t.Fatal("reorg suspension unavailable")
	}
	var row entity.ChainTxSubmission
	if db.Where("id=?", original.ID).Take(&row).Error != nil ||
		row.State != chaintx.ChainTxStateV3ReorgSuspended || row.ActiveSlot != nil ||
		row.FinalEventRecordID == nil || *row.FinalEventRecordID != *original.FinalEventRecordID ||
		row.TxHash == nil || *row.TxHash != *original.TxHash {
		t.Fatal("suspension changed immutable signed attempt")
	}
	// Simulate a V2 allocator taking the shared nonce row. The trigger must
	// reject it even though the V3 submission no longer has active_slot=1.
	if db.Exec(`UPDATE chain_relayer_nonce SET active_submission_id=? WHERE chain_id=31337 AND sender_address=?
		AND active_submission_id IS NULL`, uint64(999999), original.SenderAddress).Error == nil {
		t.Fatal("suspended V3 signature allowed shared sender allocation")
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return resumeV3AppliedOutboxTx(tx, binding, 90) }); err != nil {
		t.Fatal("orphaned prefix inspection unavailable")
	}
	if db.Where("id=?", original.ID).Take(&row).Error != nil || row.State != chaintx.ChainTxStateV3ReorgSuspended {
		t.Fatal("orphaned signature incorrectly resumed")
	}
	// After independent prefix authentication, only the exact original
	// canonical journal ID/hash may restore a surviving block.
	if err := db.Transaction(func(tx *gorm.DB) error { return resumeV3AppliedOutboxTx(tx, binding, 91) }); err != nil {
		t.Fatal("surviving original-hash prefix unavailable")
	}
	if db.Where("id=?", original.ID).Take(&row).Error != nil || row.State != constant.ChainTxStateApplied ||
		row.FinalEventRecordID == nil || *row.FinalEventRecordID != *original.FinalEventRecordID {
		t.Fatal("surviving prefix changed original canonical attempt")
	}
}

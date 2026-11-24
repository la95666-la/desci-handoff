package chainevent

import (
	"context"
	"encoding/json"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestV3JournalApplyAtomicCursorAndProjection(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	pay := common.HexToAddress("0x2222222222222222222222222222222222222222")
	sale := common.HexToAddress("0x3333333333333333333333333333333333333333")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7, PaymentToken: pay, ProjectToken: sale}
	hash := common.HexToHash("0xa1")
	parent := common.HexToHash("0xa0")
	log := types.Log{Address: proxy, BlockNumber: 11, BlockHash: hash, TxHash: common.HexToHash("0xb1"), Index: 1,
		Topics: []common.Hash{crypto.Keccak256Hash([]byte("ProjectActivated(uint256)")), common.BigToHash(big.NewInt(7))}}
	for _, tc := range []struct {
		name        string
		logs        []types.Log
		cursorHash  string
		cursorBlock uint64
		revision    uint64
		fenceLost   bool
		expected    bool
	}{
		{name: "empty block", expected: true},
		{name: "valid event but no creation rolls back", logs: []types.Log{log}},
		{name: "wrong proxy rolls back", logs: []types.Log{func() types.Log { v := log; v.Address = pay; return v }()}},
		{name: "duplicate index rolls back", logs: []types.Log{log, log}},
		{name: "wrong parent rejects before inserts", cursorHash: common.Hash{}.Hex()},
		{name: "missing parent checkpoint rejects before inserts"},
		{name: "revision overflow rejects before inserts", revision: math.MaxUint64},
		{name: "height overflow rejects before inserts", cursorBlock: math.MaxUint64},
		{name: "lost revision fence rolls back inserts", fenceLost: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectBegin()
			if tc.cursorHash == "" {
				tc.cursorHash = v3LowerHash(parent)
			}
			if tc.cursorBlock == 0 {
				tc.cursorBlock = 10
			}
			if tc.revision == 0 {
				tc.revision = 2
			}
			mock.ExpectQuery(regexp.QuoteMeta("SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).
				WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "revision", "state"}).AddRow(strconv.FormatUint(tc.cursorBlock, 10), tc.cursorHash, strconv.FormatUint(tc.revision, 10), "running"))
			if tc.cursorHash == v3LowerHash(parent) && tc.cursorBlock == 10 && tc.revision != math.MaxUint64 {
				rows := sqlmock.NewRows([]string{"block_hash"})
				if tc.name != "missing parent checkpoint rejects before inserts" {
					rows.AddRow(tc.cursorHash)
				}
				mock.ExpectQuery(regexp.QuoteMeta("SELECT block_hash FROM ido_v3_block_checkpoint")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(10)).WillReturnRows(rows)
			}
			if tc.expected || tc.fenceLost {
				mock.ExpectExec(regexp.QuoteMeta("INSERT INTO ido_v3_block_checkpoint")).WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectQuery(regexp.QuoteMeta("SELECT block_number,block_hash,tx_hash,log_index,event_kind,payload FROM ido_v3_event_journal")).
					WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"block_number", "block_hash", "tx_hash", "log_index", "event_kind", "payload"}))
				mock.ExpectExec(regexp.QuoteMeta("DELETE FROM ido_v3_project_projection")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnResult(sqlmock.NewResult(0, 0))
				updated := int64(1)
				if tc.fenceLost {
					updated = 0
				}
				mock.ExpectExec(regexp.QuoteMeta("UPDATE ido_v3_scan_cursor SET cursor_block=")).WithArgs(uint64(11), v3LowerHash(hash), uint64(31337), v3Lower(proxy), uint64(7), uint64(10), v3LowerHash(parent), uint64(2)).WillReturnResult(sqlmock.NewResult(0, updated))
				if tc.fenceLost {
					mock.ExpectRollback()
				} else {
					mock.ExpectCommit()
				}
			} else {
				mock.ExpectRollback()
			}
			err = v3Journal{db: database}.apply(context.Background(), binding, 11, parent, hash, tc.logs)
			if (err == nil) != tc.expected {
				t.Fatalf("wrong transaction result: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func v3LowerHash(h common.Hash) string { return strings.ToLower(h.Hex()) }

func expectV3FinancialSuspension(mock sqlmock.Sqlmock, proxy common.Address) {
	mock.ExpectExec(regexp.QuoteMeta("UPDATE ido_v3_financial_effect SET status='suspended'")).
		WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnResult(sqlmock.NewResult(0, 0))
	expectV3OutboxSuspension(mock, proxy)
}

func expectV3OutboxSuspension(mock sqlmock.Sqlmock, proxy common.Address) {
	mock.ExpectExec(regexp.QuoteMeta("UPDATE chain_tx_submission s JOIN ido_v3_event_journal j")).
		WithArgs("v3_reorg_suspended", uint64(31337), uint64(7), v3Lower(proxy)).WillReturnResult(sqlmock.NewResult(0, 0))
}

func expectV3BusinessHalt(mock sqlmock.Sqlmock, proxy common.Address, status string) {
	expectV3FinancialSuspension(mock, proxy)
	expectV3BusinessHaltOnly(mock, proxy, status)
}

func expectV3BusinessHaltOnly(mock sqlmock.Sqlmock, proxy common.Address, status string) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT p.status FROM projects p JOIN ido_v3_project_binding b")).
		WithArgs(uint64(7), uint64(31337), v3Lower(proxy), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(status))
	if status != v3BusinessHalted {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE projects SET status='V3ReorgHalted'")).
			WithArgs(uint64(7), uint64(31337), v3Lower(proxy)).WillReturnResult(sqlmock.NewResult(0, 1))
	}
}

func TestV3JournalRollbackRevokesProjectionAndHalts(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7}
	hash := common.HexToHash("0xa1")
	for _, tc := range []struct {
		name       string
		checkpoint string
		revision   uint64
		state      string
		success    bool
	}{
		{"halted ancestor matches", v3LowerHash(hash), 2, "reorg_halted", true},
		{"unrelated ancestor refuses orphan write", common.Hash{}.Hex(), 2, "reorg_halted", false},
		{"stale revision refuses orphan write", v3LowerHash(hash), 1, "reorg_halted", false},
		{"running cursor cannot orphan", v3LowerHash(hash), 2, "running", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta("SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "revision", "state"}).AddRow(12, v3LowerHash(common.HexToHash("0xb2")), 2, tc.state))
			if tc.revision == 2 && tc.state == "reorg_halted" {
				mock.ExpectQuery(regexp.QuoteMeta("SELECT block_hash FROM ido_v3_block_checkpoint")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(10)).WillReturnRows(sqlmock.NewRows([]string{"block_hash"}).AddRow(tc.checkpoint))
			}
			if tc.success {
				mock.ExpectExec(regexp.QuoteMeta("UPDATE ido_v3_financial_effect SET status='orphaned'")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(10)).WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec(regexp.QuoteMeta("UPDATE chain_tx_submission s JOIN ido_v3_event_journal j")).WithArgs("v3_reorg_suspended", uint64(31337), uint64(7), v3Lower(proxy), uint64(10)).WillReturnResult(sqlmock.NewResult(0, 0))
				for _, table := range []string{"ido_v3_event_journal", "ido_v3_block_checkpoint"} {
					mock.ExpectExec(regexp.QuoteMeta("UPDATE "+table+" SET status='orphaned'")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(10)).WillReturnResult(sqlmock.NewResult(0, 2))
				}
				mock.ExpectExec(regexp.QuoteMeta("DELETE FROM ido_v3_project_projection")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
				expectV3BusinessHaltOnly(mock, proxy, "Active")
				mock.ExpectExec(regexp.QuoteMeta("UPDATE ido_v3_scan_cursor SET cursor_block=")).WithArgs(uint64(10), v3LowerHash(hash), uint64(31337), v3Lower(proxy), uint64(7), uint64(12), v3LowerHash(common.HexToHash("0xb2")), uint64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			err = v3Journal{db: database}.rollback(context.Background(), binding, 10, hash, tc.revision)
			if (err == nil) != tc.success {
				t.Fatalf("rollback result: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestV3JournalRecoveryRefusesRevisionOverflowBeforeWrites(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	hash := common.HexToHash("0xa1")
	binding := idov3whitelist.ExpectedProjectBinding{
		ChainID: 31337, ProxyAddress: proxy, ProjectID: 7,
		PaymentToken: common.HexToAddress("0x2222222222222222222222222222222222222222"),
		ProjectToken: common.HexToAddress("0x3333333333333333333333333333333333333333"),
	}
	for _, operation := range []string{"rollback", "resume"} {
		t.Run(operation, func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectBegin()
			block := uint64(12)
			if operation == "resume" {
				block = 10
			}
			mock.ExpectQuery(regexp.QuoteMeta("SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).
				WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "revision", "state"}).AddRow(block, v3LowerHash(hash), strconv.FormatUint(math.MaxUint64, 10), "reorg_halted"))
			mock.ExpectRollback()
			journal := v3Journal{db: database}
			if operation == "rollback" {
				err = journal.rollback(context.Background(), binding, 10, hash, math.MaxUint64)
			} else {
				err = journal.resumeAtAncestor(context.Background(), binding, 10, hash, math.MaxUint64)
			}
			if err == nil {
				t.Fatal("overflow revision accepted")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestV3JournalRebuildPersistsBoundWalletProjection(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	pay := common.HexToAddress("0x2222222222222222222222222222222222222222")
	sale := common.HexToAddress("0x3333333333333333333333333333333333333333")
	owner := common.HexToAddress("0x4444444444444444444444444444444444444444")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7, PaymentToken: pay, ProjectToken: sale}
	hash := common.HexToHash("0xa1")
	txhash := common.HexToHash("0xb1")
	created := &commonchain.V3CreatedEvent{ChainID: 31337, Proxy: proxy, ProjectID: 7, ProjectToken: sale, PaymentToken: pay,
		ProjectOwner: owner, FeeReceiver: owner, ProjectDecimals: 1, PaymentDecimals: 1, TokenPriceRaw: "20", MaxPerUserRaw: "900", HardCapRaw: "1000", SoftCapRaw: "500", BlockNumber: 11, BlockHash: hash, TxHash: txhash, LogIndex: 1}
	encoded, err := json.Marshal(commonchain.V3ReplayEvent{Created: created})
	if err != nil {
		t.Fatal(err)
	}
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT block_number,block_hash,tx_hash,log_index,event_kind,payload FROM ido_v3_event_journal")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"block_number", "block_hash", "tx_hash", "log_index", "event_kind", "payload"}).AddRow(11, v3LowerHash(hash), v3LowerHash(txhash), 1, "created", string(encoded)))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT block_hash FROM ido_v3_block_checkpoint")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(11)).WillReturnRows(sqlmock.NewRows([]string{"block_hash"}).AddRow(v3LowerHash(hash)))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO ido_v3_project_projection")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(1), uint64(11), v3LowerHash(hash), "pending", "0", "0", "0", "0", "0", "{}", "0", "0", "0").WillReturnResult(sqlmock.NewResult(1, 1))
	if err := rebuildV3ProjectTx(database, binding, 11, hash, 1); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestV3JournalRebuildRejectsUncheckpointedOrFutureFact(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	pay := common.HexToAddress("0x2222222222222222222222222222222222222222")
	sale := common.HexToAddress("0x3333333333333333333333333333333333333333")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7, PaymentToken: pay, ProjectToken: sale}
	hash := common.HexToHash("0xa1")
	for _, tc := range []struct {
		name              string
		rowHeight         uint64
		checkpoint        string
		includeCheckpoint bool
	}{
		{"missing checkpoint", 11, "", false},
		{"wrong checkpoint hash", 11, v3LowerHash(common.HexToHash("0xb2")), true},
		{"future fact", 12, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectQuery(regexp.QuoteMeta("SELECT block_number,block_hash,tx_hash,log_index,event_kind,payload FROM ido_v3_event_journal")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"block_number", "block_hash", "tx_hash", "log_index", "event_kind", "payload"}).AddRow(tc.rowHeight, v3LowerHash(hash), v3LowerHash(common.HexToHash("0xc1")), 1, "created", "{}"))
			if tc.rowHeight <= 11 {
				rows := sqlmock.NewRows([]string{"block_hash"})
				if tc.includeCheckpoint {
					rows.AddRow(tc.checkpoint)
				}
				mock.ExpectQuery(regexp.QuoteMeta("SELECT block_hash FROM ido_v3_block_checkpoint")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(11)).WillReturnRows(rows)
			}
			if err := rebuildV3ProjectTx(db, binding, 11, hash, 1); err == nil {
				t.Fatal("invalid journal fact published")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestV3JournalRebuildRejectsPersistedKindMismatch(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	payment := common.HexToAddress("0x2222222222222222222222222222222222222222")
	sale := common.HexToAddress("0x3333333333333333333333333333333333333333")
	hash := common.HexToHash("0xa1")
	owner := common.HexToAddress("0x4444444444444444444444444444444444444444")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7, PaymentToken: payment, ProjectToken: sale}
	created := commonchain.V3ReplayEvent{Created: &commonchain.V3CreatedEvent{ChainID: 31337, ProjectID: 7, Proxy: proxy, PaymentToken: payment, ProjectToken: sale, ProjectOwner: owner, FeeReceiver: owner, ProjectDecimals: 1, PaymentDecimals: 1, FeeRate: 200, TokenPriceRaw: "20", HardCapRaw: "1000", SoftCapRaw: "500", MaxPerUserRaw: "900", BlockNumber: 11, BlockHash: hash, TxHash: common.HexToHash("0xb1"), LogIndex: 1}}
	encoded, err := json.Marshal(created)
	if err != nil {
		t.Fatal(err)
	}
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT block_number,block_hash,tx_hash,log_index,event_kind,payload FROM ido_v3_event_journal")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"block_number", "block_hash", "tx_hash", "log_index", "event_kind", "payload"}).AddRow(11, v3LowerHash(hash), v3LowerHash(created.Created.TxHash), 1, "purchase", string(encoded)))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT block_hash FROM ido_v3_block_checkpoint")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(11)).WillReturnRows(sqlmock.NewRows([]string{"block_hash"}).AddRow(v3LowerHash(hash)))
	if err := rebuildV3ProjectTx(db, binding, 11, hash, 1); err == nil {
		t.Fatal("kind-tampered event published")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestV3JournalResumeRequiresHaltedAncestorAndRebuild(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	pay := common.HexToAddress("0x2222222222222222222222222222222222222222")
	sale := common.HexToAddress("0x3333333333333333333333333333333333333333")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7, PaymentToken: pay, ProjectToken: sale}
	hash := common.HexToHash("0xa1")
	for _, tc := range []struct {
		name, state string
		revision    uint64
		success     bool
	}{{"halted rebuild", "reorg_halted", 2, true}, {"running cannot resume", "running", 2, false}, {"stale preflight revision", "reorg_halted", 1, false}} {
		t.Run(tc.name, func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta("SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "revision", "state"}).AddRow(10, v3LowerHash(hash), 2, tc.state))
			if tc.success {
				mock.ExpectQuery(regexp.QuoteMeta("SELECT block_hash FROM ido_v3_block_checkpoint")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(10)).WillReturnRows(sqlmock.NewRows([]string{"block_hash"}).AddRow(v3LowerHash(hash)))
				mock.ExpectQuery(regexp.QuoteMeta("SELECT block_number,block_hash,tx_hash,log_index,event_kind,payload FROM ido_v3_event_journal")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"block_number", "block_hash", "tx_hash", "log_index", "event_kind", "payload"}))
				mock.ExpectExec(regexp.QuoteMeta("DELETE FROM ido_v3_project_projection")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectQuery(regexp.QuoteMeta("SELECT state FROM ido_v3_project_projection")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(10), v3LowerHash(hash), uint64(3)).WillReturnRows(sqlmock.NewRows([]string{"state"}).AddRow("pending"))
				mock.ExpectQuery(regexp.QuoteMeta("SELECT p.status FROM projects p JOIN ido_v3_project_binding b")).WithArgs(uint64(7), uint64(31337), v3Lower(proxy), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(v3BusinessHalted))
				mock.ExpectExec(regexp.QuoteMeta("UPDATE projects SET status=?")).WithArgs("Pending", uint64(7), uint64(31337), v3Lower(proxy)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(regexp.QuoteMeta("UPDATE ido_v3_financial_effect SET status='canonical'")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(10)).WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM chain_tx_submission WHERE chain_id=?")).WithArgs(uint64(31337), uint64(7), v3Lower(proxy)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
				mock.ExpectExec(regexp.QuoteMeta("UPDATE ido_v3_scan_cursor SET revision=revision+1")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(10), v3LowerHash(hash), uint64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			err = v3Journal{db: database}.resumeAtAncestor(context.Background(), binding, 10, hash, tc.revision)
			if (err == nil) != tc.success {
				t.Fatalf("resume result: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestV3JournalHaltMismatchAtomicallyRevokesProjection(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7}
	hash := common.HexToHash("0xa1")
	for _, tc := range []struct {
		name       string
		cursorHash string
		revision   uint64
		fenceLost  bool
		success    bool
	}{
		{name: "matching fence", cursorHash: v3LowerHash(hash), revision: 2, success: true},
		{name: "changed hash fence", cursorHash: common.Hash{}.Hex(), revision: 2},
		{name: "revision overflow before deletion", cursorHash: v3LowerHash(hash), revision: math.MaxUint64},
		{name: "revision changed at conditional update", cursorHash: v3LowerHash(hash), revision: 2, fenceLost: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta("SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "revision", "state"}).AddRow(10, tc.cursorHash, strconv.FormatUint(tc.revision, 10), "running"))
			if tc.success || tc.fenceLost {
				mock.ExpectExec(regexp.QuoteMeta("DELETE FROM ido_v3_project_projection")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
				expectV3BusinessHalt(mock, proxy, "Active")
				updated := int64(1)
				if tc.fenceLost {
					updated = 0
				}
				mock.ExpectExec(regexp.QuoteMeta("UPDATE ido_v3_scan_cursor SET state='reorg_halted'")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(10), v3LowerHash(hash), uint64(2)).WillReturnResult(sqlmock.NewResult(0, updated))
			}
			if tc.success {
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			err = v3Journal{db: database}.haltMismatch(context.Background(), binding, 10, hash)
			if (err == nil) != tc.success {
				t.Fatalf("halt result: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestV3BootstrapUnknownHaltFencesCursorAndRevision(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7}
	hash := common.HexToHash("0xa1")
	for _, tc := range []struct {
		name, state string
		revision    uint64
		block       uint64
		want        bool
	}{
		{"first committed revision", "running", 1, 90, true},
		{"stale bootstrap against advanced scanner", "running", 2, 90, false},
		{"stale bootstrap against moved cursor", "running", 1, 91, false},
		{"already halted by canonical mismatch", "reorg_halted", 2, 90, true},
		{"later halted revision is not bootstrap revocation", "reorg_halted", 3, 90, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta("SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "revision", "state"}).AddRow(tc.block, v3LowerHash(hash), tc.revision, tc.state))
			if tc.state == "running" && tc.want {
				mock.ExpectExec(regexp.QuoteMeta("DELETE FROM ido_v3_project_projection")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
				expectV3BusinessHalt(mock, proxy, "Pending")
				mock.ExpectExec(regexp.QuoteMeta("UPDATE ido_v3_scan_cursor SET state='reorg_halted'")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(90), v3LowerHash(hash)).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			if tc.want {
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			err = (v3Journal{db: db}).haltBootstrapUnknown(context.Background(), binding, 90, hash)
			if (err == nil) != tc.want {
				t.Fatalf("bootstrap unknown halt: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestV3RangeUnknownHaltFencesPublishedRevision(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7}
	for _, tc := range []struct {
		name, state     string
		block, revision uint64
		want            bool
	}{
		{"exact published cursor", "running", 94, 3, true},
		{"advanced cursor", "running", 95, 4, false},
		{"same height newer revision", "running", 94, 4, false},
		{"already revoked", "reorg_halted", 94, 4, true},
		{"stale revoked revision", "reorg_halted", 94, 5, false},
		{"revision overflow refuses halt", "running", 94, math.MaxUint64, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta("SELECT cursor_block,revision,state FROM ido_v3_scan_cursor")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "revision", "state"}).AddRow(tc.block, strconv.FormatUint(tc.revision, 10), tc.state))
			if tc.want && tc.state == "running" {
				mock.ExpectExec(regexp.QuoteMeta("DELETE FROM ido_v3_project_projection")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
				expectV3BusinessHalt(mock, proxy, "Active")
				mock.ExpectExec(regexp.QuoteMeta("UPDATE ido_v3_scan_cursor SET state='reorg_halted'")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(94), uint64(3)).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			if tc.want {
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			err = (v3Journal{db: db}).haltRangeUnknown(context.Background(), binding, 94, 3)
			if (err == nil) != tc.want {
				t.Fatalf("range unknown halt: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestV3PublishedProjectionRequiresRunningMatchingCursorCheckpoint(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7, PaymentToken: common.HexToAddress("0x2222222222222222222222222222222222222222"), ProjectToken: common.HexToAddress("0x3333333333333333333333333333333333333333")}
	for _, tc := range []struct {
		name    string
		payload string
		success bool
	}{{"valid internal snapshot", "{}", true}, {"missing joined row", "", false}, {"malformed JSON", "not-json", false}} {
		t.Run(tc.name, func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			rows := sqlmock.NewRows([]string{"wallets", "state", "status"})
			if tc.payload != "" {
				rows.AddRow(tc.payload, "pending", "Pending")
			}
			mock.ExpectQuery(regexp.QuoteMeta("SELECT p.wallets,p.state,project.status FROM ido_v3_project_projection p")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7), v3Lower(binding.ProjectToken), v3Lower(binding.PaymentToken)).WillReturnRows(rows)
			result, err := readV3PublishedProjection(context.Background(), db, binding)
			if (err == nil) != tc.success || (tc.success && result != "{}") {
				t.Fatalf("incorrect internal projection result: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

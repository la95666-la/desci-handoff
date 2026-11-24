package chainevent

import (
	"context"
	"errors"
	"math/big"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"joincare_server/pkg/common/idov3whitelist"
)

// Replace the creation header only during the last confirmation-head read.
// No SQL should be executed from a bootstrap based on the older hash.
type v3BootstrapSwitchOnFinalHead struct {
	*v3PinnedScanStub
	switched bool
	heads    int
}

func (s *v3BootstrapSwitchOnFinalHead) BlockNumber(ctx context.Context) (uint64, error) {
	s.heads++
	if s.heads == 3 {
		s.switched = true
		changed := *s.head
		changed.Extra = []byte("creation replaced at final head")
		s.head = &changed
	}
	return s.v3PinnedScanStub.BlockNumber(ctx)
}

// Synthetic provider only: no source admission or deployment implication.
func TestV3BootstrapCreationFailsClosedBeforeAnyDatabaseWrite(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	impl := common.HexToAddress("0x4444444444444444444444444444444444444444")
	payment := common.HexToAddress("0x2222222222222222222222222222222222222222")
	sale := common.HexToAddress("0x3333333333333333333333333333333333333333")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7, PaymentToken: payment, ProjectToken: sale}
	runtime := idov3whitelist.RuntimeIdentity{Implementation: impl, ProxyCodeHash: crypto.Keccak256Hash([]byte{0x60, 0x80}), CodeHash: crypto.Keccak256Hash([]byte{0x60, 0x01})}
	parentHead := &types.Header{Number: big.NewInt(89)}
	head := &types.Header{Number: big.NewInt(90), ParentHash: parentHead.Hash()}
	addr, _ := abi.NewType("address", "", nil)
	u8, _ := abi.NewType("uint8", "", nil)
	u16, _ := abi.NewType("uint16", "", nil)
	u256, _ := abi.NewType("uint256", "", nil)
	fields := abi.Arguments{{Type: addr}, {Type: addr}, {Type: u8}, {Type: u8}, {Type: u16}}
	for i := 0; i < 6; i++ {
		fields = append(fields, abi.Argument{Type: u256})
	}
	data, err := fields.Pack(sale, payment, uint8(18), uint8(18), uint16(200), big.NewInt(20), big.NewInt(1000), big.NewInt(500), big.NewInt(900), big.NewInt(100), big.NewInt(200))
	if err != nil {
		t.Fatal(err)
	}
	created := types.Log{Address: proxy, BlockNumber: 90, BlockHash: head.Hash(), TxHash: common.HexToHash("0xb1"), Index: 3,
		Topics: []common.Hash{crypto.Keccak256Hash([]byte("ProjectCreated(uint256,address,address,address,address,uint8,uint8,uint16,uint256,uint256,uint256,uint256,uint256,uint256)")), common.BigToHash(big.NewInt(7)), common.BytesToHash(sale.Bytes()), common.BytesToHash(payment.Bytes())}, Data: data}
	newStub := func() *v3PinnedScanStub {
		return &v3PinnedScanStub{chain: 31337, head: head, parentHead: parentHead, proxy: proxy, impl: impl, payment: payment, sale: sale, project: 7, tuple: v3PendingTuple(7, sale, payment), logs: []types.Log{created}, receipt: &types.Receipt{Status: types.ReceiptStatusSuccessful, BlockNumber: big.NewInt(90), BlockHash: head.Hash(), TxHash: created.TxHash, Logs: []*types.Log{&created}}}
	}
	for _, tc := range []struct {
		name   string
		mutate func(*v3PinnedScanStub)
	}{
		{"missing creation", func(s *v3PinnedScanStub) { s.logs = nil }},
		{"missing parent header", func(s *v3PinnedScanStub) { s.parentHead = nil }},
		{"incorrect parent header", func(s *v3PinnedScanStub) { s.parentHead = &types.Header{Number: big.NewInt(89), Extra: []byte{1}} }},
		{"ambiguous creation", func(s *v3PinnedScanStub) {
			second := created
			second.Index = 4
			s.logs = append(s.logs, second)
			s.receipt.Logs = append(s.receipt.Logs, &second)
		}},
		{"immutable economics drift", func(s *v3PinnedScanStub) { s.tuple[32+9*32+31]++ }},
		{"receipt changed after project read", func(s *v3PinnedScanStub) { s.changeThirdReceipt = true }},
		{"filter changed after project read", func(s *v3PinnedScanStub) { s.changeThirdFilter = true }},
		{"same-length filter changed after project read", func(s *v3PinnedScanStub) {
			s.changeThirdFilter = true
			changed := s.logs[0]
			changed.Data = append([]byte(nil), changed.Data...)
			changed.Data[len(changed.Data)-1] ^= 1
			s.thirdFilterLogs = []types.Log{changed}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := newStub()
			tc.mutate(stub)
			if err := v3BootstrapCreation(context.Background(), &gorm.DB{}, stub, binding, runtime, 90, 3); err == nil {
				t.Fatal("invalid bootstrap reached database")
			}
		})
	}
	if err := v3BootstrapCreation(context.Background(), &gorm.DB{}, newStub(), binding, runtime, 1, 3); err == nil {
		t.Fatal("genesis parent accepted")
	}
	late := &v3BootstrapSwitchOnFinalHead{v3PinnedScanStub: newStub()}
	// The scanner reads the head twice before the bootstrap's final head.
	if err := v3BootstrapCreation(context.Background(), &gorm.DB{}, late, binding, runtime, 90, 3); err == nil || !strings.Contains(err.Error(), "final anchor changed") || !late.switched {
		t.Fatalf("late creation replacement reached database: switched=%v heads=%d err=%v", late.switched, late.heads, err)
	}
	for _, failAt := range []string{"cursor", "checkpoint"} {
		t.Run("atomic "+failAt+" failure", func(t *testing.T) {
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
			cursor := mock.ExpectExec(regexp.QuoteMeta("INSERT INTO ido_v3_scan_cursor"))
			if failAt == "cursor" {
				cursor.WillReturnError(errors.New("synthetic DB failure"))
			} else {
				cursor.WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(regexp.QuoteMeta("INSERT INTO ido_v3_block_checkpoint")).WillReturnError(errors.New("synthetic DB failure"))
			}
			mock.ExpectRollback()
			if err := v3BootstrapCreation(context.Background(), db, newStub(), binding, runtime, 90, 3); err == nil {
				t.Fatal("failed bootstrap insert published creation")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
	// Creation and deposit can share one transaction. Its receipt may be
	// replaced only after the final fixed-block project read: the bootstrap
	// must recheck BOTH event facts, rather than only the creation log.
	deposit := types.Log{Address: proxy, BlockNumber: 90, BlockHash: head.Hash(), TxHash: created.TxHash, Index: 4,
		Topics: []common.Hash{crypto.Keccak256Hash([]byte("ProjectTokensDeposited(uint256,address,uint256)")), common.BigToHash(big.NewInt(7)), common.BytesToHash(sale.Bytes())},
		Data:   common.LeftPadBytes(big.NewInt(400).Bytes(), 32)}
	multi := newStub()
	multi.logs = append(multi.logs, deposit)
	multi.receipt.Logs = append(multi.receipt.Logs, &deposit)
	multi.changeNthReceipt = 6 // two initial reads per log, then two post-project reads
	if err := v3BootstrapCreation(context.Background(), &gorm.DB{}, multi, binding, runtime, 90, 3); err == nil || !strings.Contains(err.Error(), "receipt changed") || multi.receiptReads != 6 {
		t.Fatalf("post-read deposit receipt not rejected before DB: reads=%d err=%v", multi.receiptReads, err)
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
	multi = newStub()
	multi.logs = append(multi.logs, deposit)
	multi.receipt.Logs = append(multi.receipt.Logs, &deposit)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO ido_v3_scan_cursor")).WillReturnError(errors.New("synthetic DB failure"))
	mock.ExpectRollback()
	if err := v3BootstrapCreation(context.Background(), database, multi, binding, runtime, 90, 3); err == nil || multi.receiptReads != 6 {
		t.Fatalf("valid multi-event receipts failed before atomic insert: reads=%d err=%v", multi.receiptReads, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

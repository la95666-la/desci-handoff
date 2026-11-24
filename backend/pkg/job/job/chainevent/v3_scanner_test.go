package chainevent

import (
	"context"
	"math/big"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"joincare_server/pkg/common/idov3whitelist"
)

// This stub never grants runtime approval. Every test is rejected before an
// authenticated project reader can be reached.
type v3ScannerEarlyRejectRPC struct {
	chain, latest uint64
	header        *types.Header
}

func (r v3ScannerEarlyRejectRPC) ChainID(context.Context) (*big.Int, error) {
	return new(big.Int).SetUint64(r.chain), nil
}
func (r v3ScannerEarlyRejectRPC) BlockNumber(context.Context) (uint64, error) { return r.latest, nil }
func (r v3ScannerEarlyRejectRPC) HeaderByNumber(context.Context, *big.Int) (*types.Header, error) {
	return r.header, nil
}
func (r v3ScannerEarlyRejectRPC) CodeAt(context.Context, common.Address, *big.Int) ([]byte, error) {
	panic("unauthorized code read")
}
func (r v3ScannerEarlyRejectRPC) StorageAt(context.Context, common.Address, common.Hash, *big.Int) ([]byte, error) {
	panic("unauthorized storage read")
}
func (r v3ScannerEarlyRejectRPC) CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
	panic("unauthorized project read")
}
func (r v3ScannerEarlyRejectRPC) TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error) {
	panic("unauthorized receipt read")
}
func (r v3ScannerEarlyRejectRPC) FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error) {
	panic("unauthorized filter")
}

func TestV3ScannerRejectsWrongChainAndUnconfirmedBlockBeforeReading(t *testing.T) {
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 7, ProxyAddress: common.HexToAddress("0x1111111111111111111111111111111111111111"),
		PaymentToken: common.HexToAddress("0x2222222222222222222222222222222222222222"), ProjectToken: common.HexToAddress("0x3333333333333333333333333333333333333333")}
	for _, tc := range []struct {
		name  string
		rpc   v3ScannerEarlyRejectRPC
		depth uint64
	}{
		{"wrong chain", v3ScannerEarlyRejectRPC{chain: 56, latest: 100}, 2},
		{"insufficient confirmations", v3ScannerEarlyRejectRPC{chain: 31337, latest: 90}, 2},
		{"header unavailable", v3ScannerEarlyRejectRPC{chain: 31337, latest: 100}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := v3ScanBlock(context.Background(), tc.rpc, binding, idov3whitelist.RuntimeIdentity{}, 90, tc.depth)
			if err == nil {
				t.Fatal("untrusted scanner source admitted")
			}
		})
	}
}

func TestV3CursorCanonicalityChecksStoredHash(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7}
	header := &types.Header{Number: big.NewInt(90)}
	for _, tc := range []struct {
		name, stored, state string
		want                bool
	}{
		{"canonical", v3LowerHash(header.Hash()), "running", true},
		{"reorg", common.HexToHash("0xff").Hex(), "running", false},
		{"missing checkpoint", v3LowerHash(header.Hash()), "running", false},
		{"checkpoint mismatch", v3LowerHash(header.Hash()), "running", false},
		{"halted", v3LowerHash(header.Hash()), "reorg_halted", false},
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
			mock.ExpectQuery(regexp.QuoteMeta("SELECT cursor_block,cursor_hash,state FROM ido_v3_scan_cursor")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "state"}).AddRow(90, tc.stored, tc.state))
			if tc.state == "running" {
				rows := sqlmock.NewRows([]string{"block_hash"})
				if tc.name != "missing checkpoint" {
					value := tc.stored
					if tc.name == "checkpoint mismatch" {
						value = common.HexToHash("0xcc").Hex()
					}
					rows.AddRow(value)
				}
				mock.ExpectQuery(regexp.QuoteMeta("SELECT block_hash FROM ido_v3_block_checkpoint")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(90)).WillReturnRows(rows)
				if tc.name == "missing checkpoint" || tc.name == "checkpoint mismatch" {
					mock.ExpectBegin()
					mock.ExpectQuery(regexp.QuoteMeta("SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "revision", "state"}).AddRow(90, tc.stored, 2, "running"))
					mock.ExpectExec(regexp.QuoteMeta("DELETE FROM ido_v3_project_projection")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
					expectV3BusinessHalt(mock, proxy, "Active")
					mock.ExpectExec(regexp.QuoteMeta("UPDATE ido_v3_scan_cursor SET state='reorg_halted'")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(90), tc.stored, uint64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
					mock.ExpectCommit()
				}
			}
			if tc.name == "reorg" {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta("SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "revision", "state"}).AddRow(90, tc.stored, 2, "running"))
				mock.ExpectExec(regexp.QuoteMeta("DELETE FROM ido_v3_project_projection")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
				expectV3BusinessHalt(mock, proxy, "Active")
				mock.ExpectExec(regexp.QuoteMeta("UPDATE ido_v3_scan_cursor SET state='reorg_halted'")).WithArgs(uint64(31337), v3Lower(proxy), uint64(7), uint64(90), tc.stored, uint64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			rpc := v3ScannerEarlyRejectRPC{chain: 31337, header: header}
			err = checkV3CursorCanonical(context.Background(), db, rpc, binding)
			if (err == nil) != tc.want {
				t.Fatalf("cursor check result: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

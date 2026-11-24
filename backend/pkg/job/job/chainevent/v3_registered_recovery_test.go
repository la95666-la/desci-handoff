package chainevent

import (
	"context"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
)

// All failures below must precede header/filter/receipt reads and SQL writes.
func TestV3RegisteredRecoveryRejectsUntrustedOrUnboundedCursor(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	base := idov3whitelist.AdmittedProject{
		Binding: idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 7, ProxyAddress: proxy,
			ProjectToken: common.HexToAddress("0x2222222222222222222222222222222222222222"),
			PaymentToken: common.HexToAddress("0x3333333333333333333333333333333333333333")},
		Runtime:       idov3whitelist.RuntimeIdentity{Implementation: common.HexToAddress("0x4444444444444444444444444444444444444444"), CodeHash: common.HexToHash("0xaa")},
		CreationBlock: 90, Confirmations: 3,
	}
	for _, tc := range []struct {
		name            string
		block, revision uint64
		state           string
	}{
		{"active cursor", 90, 2, "running"},
		{"before signed creation", 89, 2, "reorg_halted"},
		{"unbounded historical prefix", 602, 2, "reorg_halted"},
		{"revision overflow", 90, math.MaxUint64, "reorg_halted"},
		{"missing revision", 90, 0, "reorg_halted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectQuery(regexp.QuoteMeta("SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor")).
				WithArgs(uint64(31337), strings.ToLower(proxy.Hex()), uint64(7)).
				WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "cursor_hash", "revision", "state"}).
					AddRow(tc.block, "0x"+strings.Repeat("a", 64), strconv.FormatUint(tc.revision, 10), tc.state))
			if err := recoverV3HaltedPrefix(context.Background(), database, v3ScannerEarlyRejectRPC{chain: 31337, latest: 700}, base); err == nil {
				t.Fatal("unbounded or unauthenticated recovery proceeded")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
	// Invalid admitted runtime is rejected before opening a transaction.
	base.Runtime.CodeHash = common.Hash{}
	if err := recoverV3HaltedPrefix(context.Background(), nil, v3ScannerEarlyRejectRPC{chain: 31337}, base); err == nil {
		t.Fatal("missing runtime admitted")
	}
}

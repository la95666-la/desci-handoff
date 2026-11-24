package chainevent

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
)

func TestApprovedUnsignedCleanupDoesNotTouchSignedOrMissingAttempt(t *testing.T) {
	for _, rows := range []int{0, 2} {
		conn, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
		if err != nil {
			t.Fatal(err)
		}
		result := sqlmock.NewRows([]string{"id"})
		for i := 0; i < rows; i++ {
			result.AddRow(i + 1)
		}
		mock.ExpectQuery("SELECT id FROM chain_tx_submission WHERE chain_id=").
			WithArgs(uint64(56), uint64(19), "0x0d4040875742975269c55e16c8884d3ee9510b8e",
				"0xfd712061ee08e070a930ecd45e935ae5d7f5445c").WillReturnRows(result)
		var rpc v3ScannerRPC = struct{ v3ScannerRPC }{}
		record := idov3whitelist.ApprovedSetRecord{Identity: idov3whitelist.ApprovedSetIdentity{
			ProjectID: 19, ChainID: 56, Proxy: "0x0d4040875742975269c55e16c8884d3ee9510b8e",
			Operator: "0xfd712061ee08e070a930ecd45e935ae5d7f5445c",
		}}
		err = cancelApprovedV3StaleUnsigned(context.Background(), database, rpc, record,
			common.HexToHash("0x01"), common.HexToHash("0x02"))
		if (err != nil) != (rows == 2) {
			t.Fatalf("rows=%d err=%v", rows, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		conn.Close()
	}
}

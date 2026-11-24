package chainevent

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestApprovedV3BridgeRejectsUnexecutedSetBeforeChainRead(t *testing.T) {
	if err := bridgeApprovedV3Creation(nil, nil, nil, 19, common.Hash{}, common.Hash{}); err == nil {
		t.Fatal("nil scope accepted")
	}
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
	// No method on this embedded interface may run before approved Set join.
	var rpc v3ScannerRPC = struct{ v3ScannerRPC }{}
	if err := bridgeApprovedV3Creation(context.Background(), database, rpc, 19,
		common.HexToHash("0x01"), common.HexToHash("0x02")); err == nil {
		t.Fatal("unexecuted Set admitted to creation bridge")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

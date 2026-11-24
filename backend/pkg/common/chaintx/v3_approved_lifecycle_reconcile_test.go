package chaintx

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestApprovedLifecycleRecoveryRequiresAppliedOriginalCreation(t *testing.T) {
	if _, err := ReconcileApprovedV3LifecycleReceipt(nil, nil, nil, 19, 3, common.Hash{}, common.Hash{}); err == nil {
		t.Fatal("missing admission accepted")
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
	// No RPC call or outbox state change is permitted without an executed Set
	// and an already applied, canonical original creation receipt.
	var rpc V3LifecycleRecoveryRPC = struct{ V3LifecycleRecoveryRPC }{}
	if _, err := ReconcileApprovedV3LifecycleReceipt(context.Background(), database, rpc,
		19, 3, common.HexToHash("0x01"), common.HexToHash("0x02")); err == nil {
		t.Fatal("missing creation admission accepted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

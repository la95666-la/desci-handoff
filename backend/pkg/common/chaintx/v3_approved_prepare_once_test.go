package chaintx

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestApprovedV3SignerRequiresExecutedProjectDecisionBeforeNonceAllocation(t *testing.T) {
	if _, err := PrepareApprovedV3Once(nil, nil, nil, idov3whitelist.ExpectedProjectBinding{}, 1); err == nil {
		t.Fatal("missing approval and client accepted")
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
	// The signer has no RPC/client nonce call or DB write until an executed
	// four-eyes decision joins the immutable project commitment.
	client := &basechain.Client{}
	if _, err := PrepareApprovedV3Once(context.Background(), database, client,
		idov3whitelist.ExpectedProjectBinding{ProjectID: 19,
			ProxyAddress: common.HexToAddress("0x0d4040875742975269c55e16c8884d3ee9510b8e")}, 1); err == nil {
		t.Fatal("candidate without executed Set reached signing")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

package project

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	basechain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
)

func TestApprovedV3CreateCannotUseCandidateWithoutExecutedSet(t *testing.T) {
	operator := common.HexToAddress(v3DiagnosticOperator)
	for _, tc := range []struct {
		name    string
		project bool
	}{
		{name: "missing project"},
		{name: "Set row without executed approval", project: true},
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
			mock.ExpectBegin()
			rows := sqlmock.NewRows([]string{"project_id", "status", "onchain_locked", "funding_mode", "contract_version"})
			if tc.project {
				rows.AddRow(19, constant.ProjectStatusSet, 0, constant.FundingModeCappedPresale,
					constant.ContractVersionIDOV3Stablecoin)
			}
			mock.ExpectQuery("SELECT \\* FROM `projects` WHERE project_id = \\? LIMIT \\? FOR UPDATE").
				WithArgs(int64(19), 1).WillReturnRows(rows)
			if tc.project {
				mock.ExpectQuery("SELECT p.chain_id,p.contract_address AS proxy_address").
					WillReturnRows(sqlmock.NewRows([]string{"chain_id"}))
			}
			mock.ExpectRollback()
			tx := database.Begin().WithContext(context.Background())
			if _, err := prepareApprovedV3CreateTx(tx, 19, basechain.V3Operator, operator, time.Now(), common.HexToAddress(v3TokenOwnerSafe)); err == nil {
				t.Fatal("candidate without executed approval admitted")
			}
			_ = tx.Rollback()
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
	if _, err := prepareApprovedV3CreateTx(nil, 19, basechain.V3Operator, operator, time.Now(), common.HexToAddress(v3TokenOwnerSafe)); err == nil {
		t.Fatal("nil transaction admitted")
	}
	if _, err := previewApprovedV3CoreCreate(nil, nil, 19, operator, basechain.V3Operator); err == nil {
		t.Fatal("nil context and DB admitted")
	}
}

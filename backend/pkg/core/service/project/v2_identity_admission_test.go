package project

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/core/db"
)

func v2AdmissionMock(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return database, mock
}

func TestV2ProductActionsRejectIdentityMismatchBeforeRelayerWork(t *testing.T) {
	old := db.Client.Mysql
	t.Cleanup(func() { db.Client.Mysql = old })

	calls := []struct {
		name string
		call func() error
	}{
		{"create", func() error { _, err := CreateWithGuard(7, nil); return err }},
		{"activate", func() error { _, err := ActiveWithGuard(7, nil); return err }},
		{"end", func() error { _, err := EndWithGuard(7, nil); return err }},
		{"cancel", func() error { _, err := CancelWithGuard(7, nil); return err }},
		{"platform fee", func() error { _, err := WithdrawPlatformFeeWithGuard(7, nil); return err }},
	}
	for _, pair := range [][2]string{
		{constant.FundingModeFairLaunch, constant.ContractVersionV2Stablecoin},
		{constant.FundingModeCappedPresale, constant.ContractVersionFairLaunchV1},
		{constant.FundingModeCappedPresale, constant.ContractVersionIDOV3Stablecoin},
	} {
		for _, call := range calls {
			t.Run(call.name+"/"+pair[0]+"/"+pair[1], func(t *testing.T) {
				database, mock := v2AdmissionMock(t)
				db.Client.Mysql = database
				columns := []string{"project_id", "status", "contract_version", "funding_mode", "platform_fee_withdrawn"}
				rows := sqlmock.NewRows(columns).AddRow(7, constant.ProjectStatusSet, pair[1], pair[0], 0)
				mock.ExpectQuery("SELECT .* FROM `projects` WHERE project_id = \\? .*LIMIT \\?").WithArgs(int64(7), 1).WillReturnRows(rows)
				if err := call.call(); err == nil {
					t.Fatal("identity mismatch admitted")
				}
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestFundingStateRejectsIdentityMismatchBeforeRPC(t *testing.T) {
	old := db.Client.Mysql
	t.Cleanup(func() { db.Client.Mysql = old })
	database, mock := v2AdmissionMock(t)
	db.Client.Mysql = database
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `wallet_address` FROM `user` WHERE id = ?")).WithArgs(9).WillReturnRows(sqlmock.NewRows([]string{"wallet_address"}).AddRow("0x1111111111111111111111111111111111111111"))
	rows := sqlmock.NewRows([]string{"project_id", "contract_version", "funding_mode"}).AddRow(7, constant.ContractVersionV2Stablecoin, constant.FundingModeFairLaunch)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT project_id,payment_token,payment_token_address,payment_token_decimals,token_symbol,token_decimals,contract_address,contract_version,funding_mode FROM `projects` WHERE project_id = ? LIMIT ?")).WithArgs(7, 1).WillReturnRows(rows)
	if _, err := FundingState(9, 7); err == nil {
		t.Fatal("funding state identity mismatch admitted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

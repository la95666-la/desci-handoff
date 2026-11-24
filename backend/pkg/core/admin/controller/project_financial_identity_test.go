package controller

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
)

func financialIdentityMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db, mock
}

func TestLoadV2FundProjectRequiresExactModeVersionPair(t *testing.T) {
	const query = "SELECT project_id,name_en,status,owner_address,chain_id,contract_address,contract_version,funding_mode,onchain_locked,payment_token,payment_token_address,payment_token_decimals,token_symbol,token_address,token_decimals,token_price_raw,hard_cap_raw,soft_cap_raw,max_per_user_raw,start_time,end_time,total_raised,total_raised_raw,total_deposited,total_deposited_raw,participant_count,fee_receiver,success_fee_rate,fund_withdrawn,platform_fee_withdrawn,token_withdrawn FROM `projects` WHERE project_id = ? LIMIT ?"
	for _, tc := range []struct {
		name, mode, version string
		allowed             bool
	}{
		{"exact pair", constant.FundingModeCappedPresale, constant.ContractVersionV2Stablecoin, true},
		{"fair launch exact identity", constant.FundingModeFairLaunch, constant.ContractVersionFairLaunchV1, false},
		{"fair mode v2 version", constant.FundingModeFairLaunch, constant.ContractVersionV2Stablecoin, false},
		{"capped mode fair version", constant.FundingModeCappedPresale, constant.ContractVersionFairLaunchV1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := financialIdentityMockDB(t)
			rows := sqlmock.NewRows([]string{"project_id", "contract_version", "funding_mode"}).AddRow(9, tc.version, tc.mode)
			mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(uint64(9), 1).WillReturnRows(rows)
			_, err := loadV2FundProject(db, 9)
			if tc.allowed && err != nil {
				t.Fatalf("exact pair rejected: %v", err)
			}
			if !tc.allowed && (err == nil || err.Error() != "fund ledger is available only for exact V2 stablecoin projects") {
				t.Fatalf("mismatch accepted/incorrect error: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReconciliationRunRequiresExactProjectIdentityWithoutMutation(t *testing.T) {
	const query = "SELECT p.funding_mode,p.contract_version FROM ido_reconciliation_run r JOIN projects p ON p.project_id = r.project_id WHERE r.id = ? LIMIT ?"
	for _, tc := range []struct {
		name, mode, version string
		allowed             bool
	}{
		{"exact pair", constant.FundingModeCappedPresale, constant.ContractVersionV2Stablecoin, true},
		{"fair launch", constant.FundingModeFairLaunch, constant.ContractVersionFairLaunchV1, false},
		{"mixed pair", constant.FundingModeCappedPresale, constant.ContractVersionFairLaunchV1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := financialIdentityMockDB(t)
			rows := sqlmock.NewRows([]string{"funding_mode", "contract_version"}).AddRow(tc.mode, tc.version)
			mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(uint64(3), 1).WillReturnRows(rows)
			err := requireV2ReconciliationRun(db, 3)
			if tc.allowed && err != nil {
				t.Fatalf("exact pair rejected: %v", err)
			}
			if !tc.allowed && (err == nil || err.Error() != "fund ledger is available only for exact V2 stablecoin projects") {
				t.Fatalf("mismatch accepted/incorrect error: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

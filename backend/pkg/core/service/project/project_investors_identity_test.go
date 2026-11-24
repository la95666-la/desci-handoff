package project

import (
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/core/db"
)

func TestProjectInvestorsRequiresExactV2IdentityBeforeLedgerQuery(t *testing.T) {
	old := db.Client.Mysql
	t.Cleanup(func() { db.Client.Mysql = old })

	for _, tc := range []struct {
		name, mode, version string
		allowed             bool
	}{
		{"exact capped V2", constant.FundingModeCappedPresale, constant.ContractVersionV2Stablecoin, true},
		{"mixed fair V3 cannot read V2 investor ledger", constant.FundingModeFairLaunch, constant.ContractVersionIDOV3Stablecoin, false},
		{"fair mode V2 version", constant.FundingModeFairLaunch, constant.ContractVersionV2Stablecoin, false},
		{"capped mode Fair Launch version", constant.FundingModeCappedPresale, constant.ContractVersionFairLaunchV1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			database, mock := v2AdmissionMock(t)
			db.Client.Mysql = database
			wallet := "0x1111111111111111111111111111111111111111"
			mock.ExpectQuery(regexp.QuoteMeta("SELECT `wallet_address` FROM `user` WHERE id = ?")).
				WithArgs(9).
				WillReturnRows(sqlmock.NewRows([]string{"wallet_address"}).AddRow(wallet))
			mock.ExpectQuery("SELECT id,project_id,owner_address,payment_token,payment_token_decimals,funding_mode,contract_version FROM `projects` WHERE project_id = \\?").
				WithArgs(7).
				WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "owner_address", "payment_token", "payment_token_decimals", "funding_mode", "contract_version"}).
					AddRow(1, 7, wallet, "USDT", 6, tc.mode, tc.version))

			if tc.allowed {
				// The contract-version predicate excludes mixed-version child facts.
				mock.ExpectQuery("SELECT count\\(\\*\\) FROM `fund_ledger` WHERE project_id = \\? AND contract_version = \\? AND entry_type = \\? AND status = \\?").
					WithArgs(7, constant.ContractVersionV2Stablecoin, "purchase_payment", "canonical").
					WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(0))
				mock.ExpectQuery("SELECT id,wallet_address,amount_display,token_symbol,tx_hash,block_number,status,created_at FROM `fund_ledger` WHERE project_id = \\? AND contract_version = \\? AND entry_type = \\? AND status = \\? ORDER BY id DESC LIMIT \\?").
					WithArgs(7, constant.ContractVersionV2Stablecoin, "purchase_payment", "canonical", 10).
					WillReturnRows(sqlmock.NewRows([]string{"id", "wallet_address", "amount_display", "token_symbol", "tx_hash", "block_number", "status", "created_at"}))
			}

			result, err := ProjectInvestors(9, 7, 1, 10)
			if tc.allowed {
				if err != nil {
					t.Fatalf("exact V2 identity rejected: %v", err)
				}
				if result.Total != 0 || len(result.Items) != 0 {
					t.Fatalf("unexpected investor result: %+v", result)
				}
			} else if err == nil || !strings.Contains(err.Error(), "project funding identity must be") {
				t.Fatalf("identity mismatch admitted or returned unstable error: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

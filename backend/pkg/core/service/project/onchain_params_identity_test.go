package project

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/core/db"
)

func TestGetOnchainParamsRecognizesExactV3WithoutReinterpretingV2(t *testing.T) {
	previous := db.Client.Mysql
	t.Cleanup(func() { db.Client.Mysql = previous })
	for _, tc := range []struct {
		name, mode, version string
		configured          bool
	}{
		{"v3", constant.FundingModeCappedPresale, constant.ContractVersionIDOV3Stablecoin, true},
		{"v2", constant.FundingModeCappedPresale, constant.ContractVersionV2Stablecoin, true},
		{"v3-mismatched-mode", constant.FundingModeFairLaunch, constant.ContractVersionIDOV3Stablecoin, false},
		{"v2-mismatched-mode", constant.FundingModeFairLaunch, constant.ContractVersionV2Stablecoin, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			database, mock := v2AdmissionMock(t)
			db.Client.Mysql = database
			mock.ExpectQuery("SELECT .* FROM `projects` WHERE project_id = \\? .*LIMIT \\?").
				WithArgs(int64(7), 1).
				WillReturnRows(sqlmock.NewRows([]string{"project_id", "funding_mode", "contract_version", "chain_id", "contract_address", "token_price"}).
					AddRow(7, tc.mode, tc.version, 97, "0x2222222222222222222222222222222222222222", "1"))
			if tc.configured {
				mock.ExpectQuery("SELECT \\* FROM `project_tge` WHERE project_id = \\?").WithArgs(int64(7)).
					WillReturnRows(sqlmock.NewRows([]string{"project_id", "index", "tge_percent"}))
			}
			got, err := GetOnchainParams(7)
			if err != nil || got.Configured != tc.configured {
				t.Fatalf("GetOnchainParams configured=%v error=%v", got, err)
			}
			if got.FundingMode != tc.mode || got.Version != tc.version || got.ChainID != 97 || got.Contract != "0x2222222222222222222222222222222222222222" {
				t.Fatalf("GetOnchainParams changed persisted identity: %+v", got)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

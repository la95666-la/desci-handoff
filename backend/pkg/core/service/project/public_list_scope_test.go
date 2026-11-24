package project

import (
	"database/sql/driver"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"joincare_server/pkg/core/db"
)

func TestPublicListRequiresListingVisibility(t *testing.T) {
	old := db.Client.Mysql
	t.Cleanup(func() { db.Client.Mysql = old })
	database, mock := v2AdmissionMock(t)
	db.Client.Mysql = database

	args := []driver.Value{1}
	for _, status := range publicProjectStatuses() {
		args = append(args, status)
	}
	args = append(args, 20)
	queryPattern := regexp.QuoteMeta("FROM `projects` WHERE is_listed = ? AND status in (") + ".*" + regexp.QuoteMeta(") AND ((status <> 'V3ReorgHalted' OR (funding_mode='capped_presale' AND contract_version='ido_v3_stablecoin' AND onchain_locked=1))) ORDER BY id desc LIMIT ?")
	mock.ExpectQuery(queryPattern).
		WithArgs(args...).
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "status"}))

	response, err := List(0, "", "en-US")
	if err != nil {
		t.Fatal(err)
	}
	if response.Cursor != -1 || len(response.List) != 0 {
		t.Fatalf("unexpected empty public list response: %+v", response)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPublishedHaltedV3RetainsReadOnlyListIdentity(t *testing.T) {
	old := db.Client.Mysql
	t.Cleanup(func() { db.Client.Mysql = old })
	database, mock := v2AdmissionMock(t)
	db.Client.Mysql = database
	mock.ExpectQuery("(?s)FROM `projects` WHERE is_listed = .*V3ReorgHalted.*funding_mode='capped_presale'.*contract_version='ido_v3_stablecoin'.*onchain_locked=1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "name_en", "status", "funding_mode", "contract_version", "onchain_locked", "chain_id", "contract_address"}).
			AddRow(94, uint64(43879281503240193), "mRNACAT", "V3ReorgHalted", "capped_presale", "ido_v3_stablecoin", true, 56, "0x0d4040875742975269c55e16c8884d3ee9510b8e"))
	mock.ExpectQuery("SELECT .* FROM `project_files`").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
	mock.ExpectQuery("SELECT .* FROM `project_tge`").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
	result, err := List(0, "", "en-US")
	if err != nil || len(result.List) != 1 {
		t.Fatalf("halted public read disappeared: %+v %v", result, err)
	}
	if result.List[0].Status != "V3ReorgHalted" || result.List[0].ContractVersion != "ido_v3_stablecoin" {
		t.Fatal("maintenance must not pretend lifecycle is Active", result.List[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

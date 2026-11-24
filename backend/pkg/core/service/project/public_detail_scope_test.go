package project

import (
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"

	"joincare_server/pkg/core/db"
)

func TestDetailQueryRestrictsRowsToPublicStatuses(t *testing.T) {
	old := db.Client.Mysql
	t.Cleanup(func() { db.Client.Mysql = old })
	database, mock := v2AdmissionMock(t)
	db.Client.Mysql = database

	args := []driver.Value{4}
	for _, status := range publicProjectDetailStatuses() {
		args = append(args, status)
	}
	args = append(args, 1)
	queryPattern := regexp.QuoteMeta("FROM `projects` WHERE (project_id = ? AND status IN (") + ".*" + regexp.QuoteMeta(")) AND ((status <> 'V3ReorgHalted' OR (funding_mode='capped_presale' AND contract_version='ido_v3_stablecoin' AND onchain_locked=1))) LIMIT ?")
	mock.ExpectQuery(queryPattern).
		WithArgs(args...).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "status"}))

	if _, err := Detail(0, 4, "en-US"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("archived or unknown project detail must be hidden as not found, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDetailQueryDoesNotRequirePublicListingVisibility(t *testing.T) {
	old := db.Client.Mysql
	t.Cleanup(func() { db.Client.Mysql = old })
	database, mock := v2AdmissionMock(t)
	db.Client.Mysql = database

	args := []driver.Value{4}
	for _, status := range publicProjectDetailStatuses() {
		args = append(args, status)
	}
	args = append(args, 1)
	queryPattern := regexp.QuoteMeta("FROM `projects` WHERE (project_id = ? AND status IN (") + ".*" + regexp.QuoteMeta(")) AND ((status <> 'V3ReorgHalted' OR (funding_mode='capped_presale' AND contract_version='ido_v3_stablecoin' AND onchain_locked=1))) LIMIT ?")
	mock.ExpectQuery(queryPattern).
		WithArgs(args...).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "status", "is_listed"}))

	if _, err := Detail(0, 4, "en-US"); err == nil {
		t.Fatal("empty detail row must still return not found")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

package idov3whitelist

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestApprovedV3SchemaFailsClosedOnPartialInstall(t *testing.T) {
	if ApprovedV3SchemaReady(nil) == nil {
		t.Fatal("nil DB accepted")
	}
	for _, tc := range []struct {
		name                                  string
		tables, triggers, constraints, legacy int64
		fail                                  bool
		want                                  bool
	}{
		{"complete", 21, 9, 6, 0, false, true}, {"missing scan health", 20, 9, 6, 0, false, false},
		{"missing trigger", 21, 8, 6, 0, false, false}, {"missing approval FK", 21, 9, 5, 0, false, false},
		{"legacy binding", 21, 9, 6, 1, false, false}, {"SQL unavailable", 21, 9, 6, 0, true, false},
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
			query := mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.TABLES")
			if tc.fail {
				query.WillReturnError(errors.New("private DB diagnostics"))
			} else {
				query.WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(tc.tables))
				if tc.tables == 21 {
					mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.TRIGGERS").
						WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(tc.triggers))
					if tc.triggers == 9 {
						mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.TABLE_CONSTRAINTS").
							WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(tc.constraints))
						if tc.constraints == 6 {
							mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.COLUMNS").
								WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(5))
							mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.TABLE_CONSTRAINTS").
								WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(3))
							mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM ido_v3_project_binding b").
								WithArgs("0x0d4040875742975269c55e16c8884d3ee9510b8e").
								WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(tc.legacy))
						}
					}
				}
			}
			err = ApprovedV3SchemaReady(database)
			if (err == nil) != tc.want || (err != nil && err.Error() == "private DB diagnostics") {
				t.Fatalf("schema result: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

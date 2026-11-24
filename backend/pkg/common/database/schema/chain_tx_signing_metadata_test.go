package schema

import (
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestChainTxSigningMetadataGateFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ledger  int64
		columns int64
		pass    bool
	}{
		{"missing migration", 0, 0, false},
		{"wrong schema", 1, 4, false},
		{"exact", 1, 5, true},
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
			mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM schema_migrations")).WithArgs(chainTxSigningMetadataMigration, chainTxSigningMetadataChecksum).WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(tc.ledger))
			if tc.ledger == 1 {
				mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM information_schema.COLUMNS")).WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(tc.columns))
			}
			err = RequireChainTxSigningMetadata(database)
			if (err == nil) != tc.pass {
				t.Fatalf("schema gate: pass=%t error=%v", tc.pass, err)
			}
			if err != nil && !strings.Contains(err.Error(), "chain tx signing metadata") {
				t.Fatalf("unexplained schema rejection: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

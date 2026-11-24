package chainevent

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
)

func TestParseV3ScanAdmissionRejectsMalformedManifestAndSignature(t *testing.T) {
	key := make(ed25519.PublicKey, ed25519.PublicKeySize)
	sig := base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
	manifest := idov3whitelist.AdmissionManifest{ChainID: 31337}
	encoded, _ := json.Marshal(manifest)
	for _, tc := range []struct {
		name, content, signature string
		key                      ed25519.PublicKey
		valid                    bool
	}{
		{"valid shape only", string(encoded), sig, key, true},
		{"missing pinned key", string(encoded), sig, nil, false},
		{"unknown field", `{"chain_id":31337,"invented":true}`, sig, key, false},
		{"two objects", string(encoded) + string(encoded), sig, key, false},
		{"short signature", string(encoded), base64.StdEncoding.EncodeToString([]byte{1}), key, false},
		{"whitespace signature", string(encoded), sig + " ", key, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseV3ScanAdmission(tc.key, tc.content, tc.signature)
			if (err == nil) != tc.valid {
				t.Fatalf("parse accepted=%t err=%v", err == nil, err)
			}
		})
	}
}

func TestV3ScannerFailureRevokesOnlyExactRunningCursor(t *testing.T) {
	proxy := "0x" + strings.Repeat("1", 40)
	for _, tc := range []struct {
		name, state  string
		revision     uint64
		expectUpdate bool
	}{
		{"running", "running", 4, true},
		{"halted", "reorg_halted", 4, false},
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
			job := &V3ScannerJob{Database: database, Admission: V3ScanAdmission{Manifest: idov3whitelist.AdmissionManifest{
				ChainID: 31337, ProjectID: 7, ProxyAddress: proxy,
				ProjectToken: "0x2222222222222222222222222222222222222222",
				PaymentToken: "0x3333333333333333333333333333333333333333",
			}}}
			mock.ExpectQuery(regexp.QuoteMeta("SELECT cursor_block,revision,state FROM ido_v3_scan_cursor")).
				WithArgs(uint64(31337), proxy, uint64(7)).
				WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "revision", "state"}).AddRow(90, tc.revision, tc.state))
			if tc.expectUpdate {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta("SELECT cursor_block,revision,state FROM ido_v3_scan_cursor")).
					WithArgs(uint64(31337), proxy, uint64(7)).
					WillReturnRows(sqlmock.NewRows([]string{"cursor_block", "revision", "state"}).AddRow(90, tc.revision, tc.state))
				mock.ExpectExec(regexp.QuoteMeta("DELETE FROM ido_v3_project_projection")).WithArgs(uint64(31337), proxy, uint64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
				expectV3BusinessHalt(mock, common.HexToAddress(proxy), "Active")
				mock.ExpectExec(regexp.QuoteMeta("UPDATE ido_v3_scan_cursor SET state='reorg_halted'")).
					WithArgs(uint64(31337), proxy, uint64(7), uint64(90), tc.revision).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			if err := job.haltPublishedProjection(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

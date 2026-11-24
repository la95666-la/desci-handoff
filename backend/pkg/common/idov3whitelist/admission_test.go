package idov3whitelist

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestSignedProjectAdmissionRequiresIndependentSignatureAndExactDBJoin(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	address := func(c string) string { return "0x" + strings.Repeat(c, 40) }
	hash := func(c string) string { return "0x" + strings.Repeat(c, 64) }
	now := time.Unix(1_760_000_000, 0)
	base := AdmissionManifest{ChainID: 31337, ProjectID: 19, ProxyAddress: address("1"), ProjectToken: address("2"), PaymentToken: address("3"), Implementation: address("4"), Governor: address("5"), ProxyCodeHash: hash("a"), ImplementationCodeHash: hash("b"), StartBlock: 89, CreationBlock: 90, Confirmations: 3, ExpiresAt: now.Add(time.Hour).Unix()}
	for _, tc := range []struct {
		name          string
		mutate        func(*AdmissionManifest)
		badSignature  bool
		badDB         bool
		missingDB     bool
		duplicateDB   bool
		readFailure   bool
		commitFailure bool
		want          bool
	}{
		{name: "exact", want: true},
		{name: "wrong signature", badSignature: true},
		{name: "expired", mutate: func(m *AdmissionManifest) { m.ExpiresAt = now.Unix() }},
		{name: "unreviewed proxy", mutate: func(m *AdmissionManifest) { m.ProxyAddress = address("6") }, badSignature: true},
		{name: "wrong candidate token", badDB: true},
		{name: "missing binding", missingDB: true},
		{name: "ambiguous binding", duplicateDB: true},
		{name: "signed but wrong candidate token", mutate: func(m *AdmissionManifest) { m.ProjectToken = address("7") }},
		{name: "database unavailable", readFailure: true},
		{name: "commit unavailable", commitFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := base
			if tc.mutate != nil {
				tc.mutate(&manifest)
			}
			payload, _ := json.Marshal(manifest)
			signature := ed25519.Sign(private, append([]byte("joincare:ido-v3:admission:v1\x00"), payload...))
			if tc.badSignature {
				signature[0] ^= 1
			}
			conn, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if tc.want || tc.badDB || tc.missingDB || tc.duplicateDB || tc.readFailure || tc.commitFailure || tc.name == "signed but wrong candidate token" {
				mock.ExpectBegin()
				q := mock.ExpectQuery(regexp.QuoteMeta("FROM projects p")).WithArgs(manifest.ProjectID, manifest.ChainID, manifest.ProxyAddress)
				if tc.readFailure {
					q.WillReturnError(errors.New("sensitive db diagnostics"))
				} else {
					projectToken := base.ProjectToken
					if tc.badDB {
						projectToken = address("7")
					}
					rows := sqlmock.NewRows([]string{"project_token", "payment_token", "implementation_address", "governor_address", "start_block", "proxy_code_hash", "implementation_code_hash"})
					if !tc.missingDB {
						rows.AddRow(projectToken, base.PaymentToken, base.Implementation, base.Governor, base.StartBlock, base.ProxyCodeHash, base.ImplementationCodeHash)
					}
					if tc.duplicateDB {
						rows.AddRow(projectToken, base.PaymentToken, base.Implementation, base.Governor, base.StartBlock, base.ProxyCodeHash, base.ImplementationCodeHash)
					}
					q.WillReturnRows(rows)
				}
				if tc.want || tc.commitFailure {
					commit := mock.ExpectCommit()
					if tc.commitFailure {
						commit.WillReturnError(errors.New("sensitive commit diagnostics"))
					}
				} else {
					mock.ExpectRollback()
				}
			}
			admitted, err := AdmitSignedProject(context.Background(), database, public, manifest, signature, now)
			if (err == nil) != tc.want || (tc.want && (admitted.Binding.ProjectID != base.ProjectID || admitted.Runtime.Implementation.Hex() == "")) {
				t.Fatalf("admission result: %+v, %v", admitted, err)
			}
			if err != nil && strings.Contains(err.Error(), "sensitive") {
				t.Fatal("raw DB error leaked")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

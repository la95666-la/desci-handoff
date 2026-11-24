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
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/constant"
)

func TestSignedV3SetAdmissionRejectsForeignIdentityAndChangedPayload(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	addr := func(c string) string { return "0x" + strings.Repeat(c, 40) }
	hash := func(c string) string { return "0x" + strings.Repeat(c, 64) }
	now := time.Unix(1_800_000_000, 0)
	m := SetManifest{ChainID: 31337, ProjectID: 19, FundingMode: constant.FundingModeCappedPresale, ContractVersion: constant.ContractVersionIDOV3Stablecoin,
		ProxyAddress: addr("1"), ProjectToken: addr("8"), ProjectOwner: addr("9"), Implementation: addr("2"),
		OwnerSafe: addr("3"), Operator: addr("a"), Governor: addr("4"), PaymentToken: addr("5"), FeeReceiver: addr("6"),
		ProxyCodeHash: hash("a"), ImplementationCodeHash: hash("b"), OwnerCodeHash: hash("c"),
		GovernorCodeHash: hash("d"), StartBlock: 99, Confirmations: 3, PaymentSymbol: "USDT",
		PaymentDecimals: 18, FeeRate: 200, RequestDigest: hash("e"), ExpiresAt: now.Add(time.Hour).Unix()}
	for _, tc := range []struct {
		name                                    string
		mutate                                  func(*SetManifest)
		badSig, wrongDB, missing, dbError, want bool
	}{
		{name: "exact", want: true},
		{name: "different digest", mutate: func(m *SetManifest) { m.RequestDigest = hash("f") }},
		{name: "V2 identity", mutate: func(m *SetManifest) { m.ContractVersion = constant.ContractVersionV2Stablecoin }},
		{name: "wrong funding mode", mutate: func(m *SetManifest) { m.FundingMode = constant.FundingModeFairLaunch }},
		{name: "project token equals payment", mutate: func(m *SetManifest) { m.ProjectToken = m.PaymentToken }},
		{name: "project owner absent", mutate: func(m *SetManifest) { m.ProjectOwner = "" }},
		{name: "operator absent", mutate: func(m *SetManifest) { m.Operator = "" }},
		{name: "operator owner collision", mutate: func(m *SetManifest) { m.Operator = m.OwnerSafe }},
		{name: "project token address absent", mutate: func(m *SetManifest) { m.ProjectToken = "" }},
		{name: "expired", mutate: func(m *SetManifest) { m.ExpiresAt = now.Unix() }},
		{name: "foreign proxy", mutate: func(m *SetManifest) { m.ProxyAddress = addr("7") }},
		{name: "invalid signature", badSig: true},
		{name: "foreign runtime", wrongDB: true},
		{name: "missing project binding", missing: true},
		{name: "DB error masked", dbError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := m
			if tc.mutate != nil {
				tc.mutate(&manifest)
			}
			payload, _ := json.Marshal(manifest)
			sig := ed25519.Sign(priv, append([]byte("joincare:ido-v3:set:v1\x00"), payload...))
			if tc.badSig {
				sig[0] ^= 1
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
			if tc.want || tc.wrongDB || tc.missing || tc.dbError {
				mock.ExpectBegin()
				q := mock.ExpectQuery(regexp.QuoteMeta("FROM projects p JOIN ido_v3_runtime r")).WithArgs(manifest.ChainID, manifest.ProxyAddress, manifest.ProjectID, constant.ProjectStatusSetup)
				if tc.dbError {
					q.WillReturnError(errors.New("secret database diagnostics"))
				} else {
					rows := sqlmock.NewRows([]string{"implementation_address", "governor_address", "start_block", "proxy_code_hash", "implementation_code_hash"})
					if !tc.missing {
						implementation := m.Implementation
						if tc.wrongDB {
							implementation = addr("7")
						}
						rows.AddRow(implementation, m.Governor, m.StartBlock, m.ProxyCodeHash, m.ImplementationCodeHash)
					}
					q.WillReturnRows(rows)
				}
				if tc.want {
					mock.ExpectCommit()
				} else {
					mock.ExpectRollback()
				}
			}
			err = AdmitSignedSet(context.Background(), database, pub, manifest, sig, common.HexToHash(m.RequestDigest), now)
			if (err == nil) != tc.want || (err != nil && strings.Contains(err.Error(), "secret")) {
				t.Fatalf("Set admission: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSignedSetManifestOnlyVerifiesReviewedBytesNotPersistedBinding(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	address := func(c string) string { return "0x" + strings.Repeat(c, 40) }
	hash := func(c string) string { return "0x" + strings.Repeat(c, 64) }
	now := time.Unix(1_800_000_000, 0)
	m := SetManifest{ChainID: 31337, ProjectID: 19, FundingMode: constant.FundingModeCappedPresale,
		ContractVersion: constant.ContractVersionIDOV3Stablecoin, ProxyAddress: address("1"),
		ProjectToken: address("2"), ProjectOwner: address("3"), Implementation: address("4"),
		OwnerSafe: address("5"), Operator: address("6"), Governor: address("7"),
		PaymentToken: address("8"), FeeReceiver: address("9"), ProxyCodeHash: hash("a"),
		ImplementationCodeHash: hash("b"), OwnerCodeHash: hash("e"), GovernorCodeHash: hash("f"),
		StartBlock: 90, Confirmations: 3, PaymentSymbol: "USDT", PaymentDecimals: 18,
		FeeRate: 200, RequestDigest: hash("1"), ExpiresAt: now.Add(time.Hour).Unix()}
	payload, _ := json.Marshal(m)
	sig := ed25519.Sign(priv, append([]byte("joincare:ido-v3:set:v1\x00"), payload...))
	if err := VerifySignedSetManifest(pub, m, sig, common.HexToHash(m.RequestDigest), now); err != nil {
		t.Fatal(err)
	}
	if err := VerifySignedSetManifest(pub, m, sig, common.HexToHash(hash("2")), now); err == nil {
		t.Fatal("unreviewed request digest accepted")
	}
	if err := VerifySignedSetManifest(pub, m, sig, common.HexToHash(m.RequestDigest), now.Add(time.Hour)); err == nil {
		t.Fatal("expired review accepted")
	}
	m.FeeReceiver = address("a")
	if err := VerifySignedSetManifest(pub, m, sig, common.HexToHash(m.RequestDigest), now); err == nil {
		t.Fatal("altered fee receiver accepted")
	}
}

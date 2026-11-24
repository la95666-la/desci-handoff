package login

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"joincare_server/pkg/common/entity"
)

func TestSignedPrimaryWalletBindingOwnershipAndAtomicFailure(t *testing.T) {
	wallet := "0x81020dba311f8440c658fbd76c31196d834ab0de"
	now := time.Now()
	for _, kind := range []string{"new", "existing-verified", "legacy-unverified", "missing-identity", "foreign-identity", "foreign-binding", "duplicate-binding", "write-failure", "readback-failure", "disabled", "wrong-signer"} {
		t.Run(kind, func(t *testing.T) {
			db, m := inviteCapTestDB(t)
			u := entity.User{ID: 36934, WalletAddress: wallet, IsEnable: 1, SessionVersion: 1}
			signer := wallet
			if kind == "disabled" {
				u.IsEnable = 0
			}
			if kind == "wrong-signer" {
				signer = "0x1111111111111111111111111111111111111111"
			}
			m.ExpectBegin()
			early := kind == "disabled" || kind == "wrong-signer"
			if !early {
				identity := sqlmock.NewRows([]string{"wallet_address", "uid", "chain", "claim_method"})
				uid := 36934
				if kind == "foreign-identity" {
					uid = 7
				}
				if kind != "missing-identity" {
					identity.AddRow(wallet, uid, "evm", "primary_registration")
				}
				m.ExpectQuery("SELECT wallet_address,uid,chain,claim_method,claimed_at FROM user_wallet_identity").WithArgs(wallet).WillReturnRows(identity)
				if kind != "missing-identity" && kind != "foreign-identity" {
					rows := sqlmock.NewRows([]string{"id", "uid", "wallet_address", "chain", "verified_at", "verification_method"})
					switch kind {
					case "existing-verified":
						rows.AddRow(12, 36934, common.HexToAddress(wallet).Hex(), "evm", now, "personal_sign")
					case "legacy-unverified":
						rows.AddRow(12, 36934, wallet, "evm", nil, "")
					case "foreign-binding":
						rows.AddRow(12, 7, wallet, "evm", now, "primary_registration")
					case "duplicate-binding":
						rows.AddRow(12, 36934, wallet, "evm", now, "primary_registration").AddRow(13, 36934, wallet, "evm", now, "primary_registration")
					}
					m.ExpectQuery("SELECT \\* FROM user_wallet WHERE LOWER").WithArgs(wallet).WillReturnRows(rows)
					if kind != "foreign-binding" && kind != "duplicate-binding" {
						m.ExpectExec("UPDATE `user_wallet` SET").WillReturnResult(sqlmock.NewResult(0, 1))
						if kind == "existing-verified" {
							m.ExpectExec("UPDATE `user_wallet` SET `is_primary`=").WillReturnResult(sqlmock.NewResult(0, 1))
						} else if kind == "legacy-unverified" {
							m.ExpectExec("UPDATE `user_wallet` SET").WillReturnResult(sqlmock.NewResult(0, 1))
						} else {
							q := m.ExpectExec("INSERT INTO `user_wallet`")
							if kind == "write-failure" {
								q.WillReturnError(errors.New("unique conflict"))
							} else {
								q.WillReturnResult(sqlmock.NewResult(12, 1))
							}
						}
						if kind != "write-failure" {
							n := 1
							if kind == "readback-failure" {
								n = 0
							}
							m.ExpectQuery("SELECT COUNT.*FROM user_wallet w JOIN user_wallet_identity").WithArgs(int64(36934), wallet).WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(n))
						}
					}
				}
			}
			pass := kind == "new" || kind == "existing-verified" || kind == "legacy-unverified"
			if pass {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			err := db.Transaction(func(tx *gorm.DB) error { return ensureSignedPrimaryWalletTx(tx, u, signer, now) })
			if pass != (err == nil) {
				t.Fatal(err)
			}
			if e := m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestSignedWalletLoginLocksUserAndRollsBackMissingOwnership(t *testing.T) {
	db, m := inviteCapTestDB(t)
	w := "0x81020dba311f8440c658fbd76c31196d834ab0de"
	m.ExpectBegin()
	m.ExpectQuery("SELECT .* FROM `user` WHERE wallet_address = .* FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "wallet_address", "is_enable", "session_version"}).AddRow(36934, w, 1, 1))
	m.ExpectQuery("SELECT wallet_address,uid,chain,claim_method,claimed_at FROM user_wallet_identity").WillReturnRows(sqlmock.NewRows([]string{"uid"}))
	m.ExpectRollback()
	if _, e := signedWalletLogin(db, w, ""); e == nil {
		t.Fatal("missing identity accepted")
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestOnlySignedLoginAndRegistrationCanRestoreBinding(t *testing.T) {
	raw, e := os.ReadFile("service.go")
	if e != nil {
		t.Fatal(e)
	}
	s := string(raw)
	login := s[strings.Index(s, "func Login("):strings.Index(s, "func RegistrationStatus(")]
	if strings.Index(login, "verifyAuthSignature(") < 0 || strings.Index(login, "signedWalletLogin(") < strings.Index(login, "verifyAuthSignature(") {
		t.Fatal("binding before signature verification")
	}
	register := s[strings.Index(s, "func Register("):strings.Index(s, "func Login(")]
	if !strings.Contains(register, "ensureSignedPrimaryWalletTx(") || !strings.Contains(register, "completeSignedLoginTx(") {
		t.Fatal("registration missing primary binding")
	}
	auto := s[strings.Index(s, "func AutoLogin("):strings.Index(s, "func Profile(")]
	if strings.Contains(auto, "ensureSignedPrimaryWalletTx") || strings.Contains(auto, "signedWalletLogin") {
		t.Fatal("token-only renewal granted signature authority")
	}
}

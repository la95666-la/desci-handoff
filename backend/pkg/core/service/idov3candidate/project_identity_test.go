package idov3candidate

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestAuditInertProjectIdentityFailsClosed(t *testing.T) {
	scope := NewScope(31337, 41376991395446785, common.HexToAddress("0x4444444444444444444444444444444444444444"))
	const sale = "0x1111111111111111111111111111111111111111"
	const pay = "0x2222222222222222222222222222222222222222"
	const impl = "0x3333333333333333333333333333333333333333"
	const governor = "0x5555555555555555555555555555555555555555"
	for _, tc := range []struct {
		name                         string
		rows                         int
		sale, pay, impl, governor    string
		proxyHash, implHash          string
		startBlock                   uint64
		failure, commitFailure, want bool
	}{
		{name: "exact inert identity", rows: 1, want: true},
		{name: "missing binding or wrong version"},
		{name: "duplicate identity", rows: 2},
		{name: "zero project token", rows: 1, sale: "0x" + strings.Repeat("0", 40)},
		{name: "case-variant token", rows: 1, sale: "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		{name: "reused token", rows: 1, sale: pay},
		{name: "zero hash", rows: 1, proxyHash: "0x" + strings.Repeat("0", 64)},
		{name: "zero governor", rows: 1, governor: "0x" + strings.Repeat("0", 40)},
		{name: "governor equals implementation", rows: 1, governor: impl},
		{name: "zero start", rows: 1, startBlock: 0},
		{name: "read error", failure: true},
		{name: "commit error", rows: 1, commitFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pick := func(value, fallback string) string {
				if value == "" {
					return fallback
				}
				return value
			}
			start := tc.startBlock
			if tc.name != "zero start" {
				start = 89
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
			mock.ExpectBegin()
			query := mock.ExpectQuery(regexp.QuoteMeta("FROM projects p")).WithArgs(scope.ProjectID.Uint64(), scope.ChainID.Uint64(), strings.ToLower(scope.ProxyAddress.Hex()))
			if tc.failure {
				query.WillReturnError(errors.New("sensitive database diagnostics"))
			} else {
				rows := sqlmock.NewRows([]string{"project_token", "payment_token", "implementation_address", "governor_address", "start_block", "proxy_code_hash", "implementation_code_hash"})
				for i := 0; i < tc.rows; i++ {
					rows.AddRow(pick(tc.sale, sale), pick(tc.pay, pay), pick(tc.impl, impl), pick(tc.governor, governor), start,
						pick(tc.proxyHash, "0x"+strings.Repeat("a", 64)), pick(tc.implHash, "0x"+strings.Repeat("b", 64)))
				}
				query.WillReturnRows(rows)
			}
			if tc.want || tc.commitFailure {
				commit := mock.ExpectCommit()
				if tc.commitFailure {
					commit.WillReturnError(errors.New("sensitive commit failure"))
				}
			} else {
				mock.ExpectRollback()
			}
			binding, err := AuditInertProjectIdentity(context.Background(), database, scope)
			if (err == nil) != tc.want {
				t.Fatalf("unexpected result binding=%+v err=%v", binding, err)
			}
			if err != nil && strings.Contains(err.Error(), "sensitive") {
				t.Fatal("raw database error leaked")
			}
			if tc.want && (binding.ProjectID != scope.ProjectID.Uint64() || binding.ProjectToken != common.HexToAddress(sale) || binding.Implementation != common.HexToAddress(impl) || binding.Governor != common.HexToAddress(governor) || binding.StartBlock != 89) {
				t.Fatal("wrong audit identity")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

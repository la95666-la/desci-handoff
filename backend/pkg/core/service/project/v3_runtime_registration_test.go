package project

import (
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func reviewedV3RuntimeFixture() v3ReviewedRuntime {
	return v3ReviewedRuntime{
		ChainID: 56, Proxy: strings.ToLower(v3TokenProxy),
		Implementation: strings.ToLower(v3TokenImplementation),
		ProxyCodeHash:  v3TokenProxyHash, ImplementationHash: v3TokenImplementationHash,
		Governor: strings.ToLower(v3DiagnosticGovernor), StartBlock: 128805696,
	}
}

func TestReviewedV3RuntimeRegistrationRequiresExactDeployment(t *testing.T) {
	database, mock := v2AdmissionMock(t)
	r := reviewedV3RuntimeFixture()
	for _, mutate := range []func(*v3ReviewedRuntime){
		func(v *v3ReviewedRuntime) { v.ChainID = 97 },
		func(v *v3ReviewedRuntime) { v.Proxy = strings.ToLower(v3TokenImplementation) },
		func(v *v3ReviewedRuntime) { v.Governor = strings.ToLower(v3TokenOwnerSafe) },
		func(v *v3ReviewedRuntime) { v.StartBlock = 1 },
	} {
		candidate := r
		mutate(&candidate)
		if err := ensureReviewedV3RuntimeTx(database, candidate); err == nil {
			t.Fatal("foreign runtime accepted")
		}
	}
	if err := ensureReviewedV3RuntimeTx(nil, r); err == nil {
		t.Fatal("runtime registered without transaction")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReviewedV3RuntimeRegistrationIsConditionalAndImmutable(t *testing.T) {
	r := reviewedV3RuntimeFixture()
	for _, tc := range []struct {
		name                              string
		existing, mismatch, insertFailure bool
	}{
		{"first registration", false, false, false},
		{"already registered", true, false, false},
		{"foreign existing runtime", true, true, false},
		{"insert failure", false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := v2AdmissionMock(t)
			mock.ExpectBegin()
			rows := sqlmock.NewRows([]string{"implementation_address", "proxy_code_hash", "implementation_code_hash", "governor_address", "start_block", "state"})
			if tc.existing {
				governor := r.Governor
				if tc.mismatch {
					governor = strings.ToLower(v3TokenOwnerSafe)
				}
				rows.AddRow(r.Implementation, r.ProxyCodeHash, r.ImplementationHash, governor, r.StartBlock, "unverified")
			}
			mock.ExpectQuery("SELECT implementation_address,proxy_code_hash,implementation_code_hash").WithArgs(r.ChainID, r.Proxy).WillReturnRows(rows)
			if !tc.existing {
				insert := mock.ExpectExec("INSERT INTO ido_v3_runtime").WithArgs(r.ChainID, r.Proxy, r.Implementation,
					r.ProxyCodeHash, r.ImplementationHash, r.Governor, r.StartBlock)
				if tc.insertFailure {
					insert.WillReturnError(errors.New("sensitive database detail"))
				} else {
					insert.WillReturnResult(sqlmock.NewResult(0, 1))
				}
			}
			mock.ExpectRollback() // Caller owns the enclosing approval transaction.
			tx := db.Begin()
			err := ensureReviewedV3RuntimeTx(tx, r)
			if (err == nil) != (!tc.mismatch && !tc.insertFailure) || (err != nil && strings.Contains(err.Error(), "sensitive")) {
				t.Fatalf("unexpected result: %v", err)
			}
			_ = tx.Rollback()
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

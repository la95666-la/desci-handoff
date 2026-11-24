package chaintx

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"

	"joincare_server/pkg/common/v3diagnostic"
)

func TestSigningSenderEvidenceIsDBOnlyAndFailClosed(t *testing.T) {
	old := viper.Get("idoV3RetiredNonce.enabled")
	defer viper.Set("idoV3RetiredNonce.enabled", old)
	viper.Set("idoV3RetiredNonce.enabled", true)
	for _, kind := range []string{"fresh", "expired", "revoked", "unenrolled", "schema"} {
		t.Run(kind, func(t *testing.T) {
			d, m := recoveryFlowDB(t)
			m.ExpectQuery("SELECT id FROM chain_tx_submission.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(41))
			m.ExpectQuery("SELECT COUNT.*information_schema.TABLES").WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(2))
			n := 3
			if kind == "schema" {
				n = 0
			}
			m.ExpectQuery("SELECT COUNT.*information_schema.TRIGGERS").WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(n))
			if kind != "schema" {
				r := sqlmock.NewRows([]string{"id", "state"})
				if kind != "unenrolled" {
					state := "valid"
					if kind == "revoked" {
						state = "revoked"
					}
					r.AddRow(1, state)
				}
				m.ExpectQuery("SELECT id,state FROM ido_v3_retired_nonce_evidence.*FOR UPDATE").WillReturnRows(r)
				if kind == "fresh" || kind == "expired" {
					rows := sqlmock.NewRows([]string{"id"})
					if kind == "fresh" {
						rows.AddRow(1)
					}
					m.ExpectQuery("SELECT e.id FROM ido_v3_retired_nonce_evidence.*FOR SHARE").WithArgs(uint64(1), int64(30)).WillReturnRows(rows)
				}
			}
			// nil RPC panics if the old inline chain audit is accidentally reintroduced.
			e := requireV3SuspendedSenderTx(context.Background(), d, nil, 56, common.HexToAddress("0x123"))
			if (e == nil) != (kind == "fresh") {
				t.Fatal(kind, e)
			}
			if kind == "expired" && v3diagnostic.Reason(e) != "SENDER_EVIDENCE_PENDING" {
				t.Fatal(e)
			}
			if e := m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestPreIntentGateReservesRunwayAndDetectsActiveSlot(t *testing.T) {
	for _, active := range []bool{false, true} {
		d, m := recoveryFlowDB(t)
		m.ExpectBegin()
		m.ExpectQuery("SELECT .*chain_event_sync_state").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		m.ExpectQuery("SELECT count.*chain_event_reorg_incident").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		m.ExpectQuery("SELECT COUNT.*information_schema.TRIGGERS").WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(2))
		var slot any
		if active {
			slot = uint64(74)
		}
		m.ExpectQuery("SELECT .*chain_relayer_nonce.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"active_submission_id", "next_nonce"}).AddRow(slot, 85))
		if active {
			m.ExpectRollback()
		} else {
			m.ExpectQuery("SELECT id,nonce FROM chain_tx_submission").WillReturnRows(sqlmock.NewRows([]string{"id", "nonce"}))
			m.ExpectCommit()
		}
		e := RequireV3SenderReady(context.Background(), d, 56, common.HexToAddress("0x123"))
		if (e == nil) == active {
			t.Fatal(e)
		}
		if e := m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
	if SenderPreflightReserve < 150*time.Second || SenderSigningReserve <= 0 {
		t.Fatal("runway")
	}
	if SenderMaintenanceReserve <= SenderPreflightReserve+60*time.Second {
		t.Fatal("maintenance must refresh before request admission closes")
	}
}

func TestCreateGatePrecedesIntentAndHistoricalRPCNotInSigning(t *testing.T) {
	for _, path := range []string{"../../core/service/project/v3_approved_create_execute.go", "../../core/service/project/v3_approved_lifecycle_execute.go"} {
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		s := string(b)
		if strings.Index(s, "RequireV3SenderReady(") < 0 || strings.Index(s, "RequireV3SenderReady(") > strings.Index(s, "phase = \"reserve_intent\"") {
			t.Fatal("gate after reservation", path)
		}
	}
	b, e := os.ReadFile("v3_retired_nonce.go")
	if e != nil {
		t.Fatal(e)
	}
	s := string(b)
	i := strings.Index(s, "func requireV3SuspendedSenderTx")
	if i < 0 || strings.Contains(s[i:], "reconcileRetiredNonceTx(") {
		t.Fatal("RPC maintenance returned to signing")
	}
}

func TestActiveOriginalTakesPrecedenceOverExpiredSenderEvidence(t *testing.T) {
	old := viper.Get("idoV3RetiredNonce.enabled")
	defer viper.Set("idoV3RetiredNonce.enabled", old)
	viper.Set("idoV3RetiredNonce.enabled", true)
	d, m := recoveryFlowDB(t)
	m.ExpectBegin()
	m.ExpectQuery("SELECT .*chain_event_sync_state").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	m.ExpectQuery("SELECT count.*chain_event_reorg_incident").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	m.ExpectQuery("SELECT COUNT.*information_schema.TRIGGERS").WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(2))
	m.ExpectQuery("SELECT .*chain_relayer_nonce.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"active_submission_id"}).AddRow(99))
	m.ExpectRollback()
	err := RequireV3SenderReady(context.Background(), d, 56, common.HexToAddress("0x123"))
	if v3diagnostic.Reason(err) != "NONCE_UNRESOLVED" {
		t.Fatal(err)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

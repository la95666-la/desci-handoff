package chaintx

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
)

func TestApprovedLockedAdmissionRejectsMissingFourEyesBeforeRPC(t *testing.T) {
	if _, err := AdmitApprovedLockedProject(nil, nil, nil, 19, common.Hash{}, common.Hash{}); err == nil {
		t.Fatal("nil scope admitted")
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
	mock.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
	// An RPC read before the exact executed-approval join would panic.
	var rpc V3LifecycleRecoveryRPC = struct{ V3LifecycleRecoveryRPC }{}
	if _, err := AdmitApprovedLockedProject(context.Background(), database, rpc, 19,
		common.HexToHash("0x01"), common.HexToHash("0x02")); err == nil {
		t.Fatal("candidate without executed Set admitted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestScannerFullAuditSnapshotNeverBypassesOriginalExecutedSet(t *testing.T) {
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
	var rpc V3LifecycleRecoveryRPC = struct{ V3LifecycleRecoveryRPC }{}
	admitted, snapshot, err := AuditApprovedLockedProjectForScanner(context.Background(), database, rpc, 19, common.HexToHash("0x01"), common.HexToHash("0x02"))
	if err == nil || admitted.Binding.ProjectID != 0 || snapshot.SubmissionID != 0 {
		t.Fatal("missing Set granted full scanner baseline", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestActivationCreationAdmissionDiagnosticIsFinite(t *testing.T) {
	for _, check := range []activationAdmissionCheck{"candidate", "original_attempt", "chain", "confirmations",
		"creation_header", "creation_receipt", "creation_logs", "creation_event", "creation_project",
		"creation_runtime", "cursor_header", "confirmed_runtime", "confirmed_project", "repeat_receipt",
		"final_confirmations", "final_headers", "candidate_recheck"} {
		if got := ActivationAdmissionFailureCheck(fmt.Errorf("wrapped: %w", check)); got != string(check) {
			t.Fatalf("check %s mapped to %s", check, got)
		}
	}
	if got := ActivationAdmissionFailureCheck(errors.New("commercial provider URL")); got != "unavailable" {
		t.Fatal("RPC error leaked")
	}
	if got := ActivationAdmissionFailureCheck(activationAdmissionCheck("unexpected_secret")); got != "unavailable" {
		t.Fatal("unrecognized check leaked")
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
	mock.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
	var rpc V3LifecycleRecoveryRPC = struct{ V3LifecycleRecoveryRPC }{}
	if _, _, err := AdmitApprovedLockedProjectForActivation(context.Background(), database, rpc, 19,
		common.HexToHash("0x01"), common.HexToHash("0x02")); ActivationAdmissionFailureCheck(err) != "candidate" {
		t.Fatalf("missing executed Set check=%s", ActivationAdmissionFailureCheck(err))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApprovedPreviewCursorProgressOnlyAcceptsContiguousForwardScan(t *testing.T) {
	before := idov3whitelist.ApprovedLockedCandidate{CursorBlock: 100, CursorRevision: 7,
		CursorHash: common.HexToHash("0x01")}
	forward := before
	forward.CursorBlock, forward.CursorRevision, forward.CursorHash = 132, 39, common.HexToHash("0x02")
	if !approvedV3PreviewCursorProgress(before, before, true) ||
		!approvedV3PreviewCursorProgress(before, forward, true) ||
		!approvedV3PreviewCursorProgress(before, forward, false) {
		t.Fatal("all readers must admit unchanged or canonical contiguous forward progress")
	}
	for _, changed := range []idov3whitelist.ApprovedLockedCandidate{
		{CursorBlock: 99, CursorRevision: 6, CursorHash: common.HexToHash("0x03")},
		{CursorBlock: 100, CursorRevision: 8, CursorHash: before.CursorHash},
		{CursorBlock: 100, CursorRevision: 7, CursorHash: common.HexToHash("0x03")},
		{CursorBlock: 132, CursorRevision: 40, CursorHash: forward.CursorHash},
		{CursorBlock: 132, CursorRevision: 39},
	} {
		if approvedV3PreviewCursorProgress(before, changed, true) || approvedV3PreviewCursorProgress(before, changed, false) {
			t.Fatalf("rewound or rewritten cursor admitted: %+v", changed)
		}
	}
}

func TestPolicyObservationCandidateAllowsOnlyCanonicalForwardProgress(t *testing.T) {
	before := idov3whitelist.ApprovedLockedCandidate{
		Record:       idov3whitelist.ApprovedSetRecord{Request: []byte(`{"review":5}`)},
		SubmissionID: 42, CreationBlock: 129857102,
		CreationTx: common.HexToHash("0x01"), CursorBlock: 129858217,
		CursorRevision: 1116, CursorHash: common.HexToHash("0x02"),
	}
	forward := before
	forward.CursorBlock += 16
	forward.CursorRevision += 16
	forward.CursorHash = common.HexToHash("0x03")
	if err := RecheckPolicyObservationCandidate(before, before); err != nil {
		t.Fatal("unchanged canonical candidate rejected", err)
	}
	if err := RecheckPolicyObservationCandidate(before, forward); err != nil {
		t.Fatal("contiguous forward canonical candidate rejected", err)
	}
	for _, change := range []func(*idov3whitelist.ApprovedLockedCandidate){
		func(v *idov3whitelist.ApprovedLockedCandidate) { v.CursorRevision++ },
		func(v *idov3whitelist.ApprovedLockedCandidate) { v.CursorBlock-- },
		func(v *idov3whitelist.ApprovedLockedCandidate) { v.CursorHash = common.HexToHash("0x04") },
		func(v *idov3whitelist.ApprovedLockedCandidate) { v.SubmissionID++ },
		func(v *idov3whitelist.ApprovedLockedCandidate) { v.Record.Request = []byte(`{"review":6}`) },
	} {
		changed := before
		change(&changed)
		if err := RecheckPolicyObservationCandidate(before, changed); err == nil {
			t.Fatal("changed policy observation creation evidence admitted")
		}
	}
}

func TestPolicyObservationFinalFenceRejectsMissingSetBeforeRPC(t *testing.T) {
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	before := idov3whitelist.ApprovedLockedCandidate{SubmissionID: 42,
		CreationTx: common.HexToHash("0x01"), CreationHash: common.HexToHash("0x02")}
	mock.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
	// Any premature chain read would panic. A missing/changed immutable Set or
	// original create must fail before checking a current cursor or receipt.
	var rpc V3LifecycleRecoveryRPC = struct{ V3LifecycleRecoveryRPC }{}
	if err := RecheckApprovedPolicyObservation(context.Background(), database, rpc, 19, before); err == nil {
		t.Fatal("missing approved Set admitted at the final observation fence")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPolicyObservationFinalFenceRejectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := RecheckApprovedPolicyObservation(ctx, &gorm.DB{}, nil, 19,
		idov3whitelist.ApprovedLockedCandidate{SubmissionID: 42}); err == nil {
		t.Fatal("cancelled request admitted at final observation fence")
	}
}

func TestApprovedPreviewEvidenceCannotChangeAcrossCursorAdvance(t *testing.T) {
	before := idov3whitelist.ApprovedLockedCandidate{
		Record:       idov3whitelist.ApprovedSetRecord{Request: []byte(`{"review":1}`)},
		SubmissionID: 41, CreationBlock: 100, CreationTx: common.HexToHash("0x01"),
	}
	after := before
	if !sameApprovedV3PreviewEvidence(before, after) {
		t.Fatal("unchanged admission evidence refused")
	}
	after.Record.Request = []byte(`{"review":2}`)
	if sameApprovedV3PreviewEvidence(before, after) {
		t.Fatal("changed four-eyes request admitted")
	}
	after = before
	after.SubmissionID++
	if sameApprovedV3PreviewEvidence(before, after) {
		t.Fatal("changed original outbox admitted")
	}
}

func TestDisplayHaltedCursorMustRemainExactlyUnchanged(t *testing.T) {
	before := idov3whitelist.ApprovedLockedCandidate{CursorBlock: 100, CursorRevision: 7, CursorHash: common.HexToHash("0x01")}
	if !approvedV3DisplayCursorProgress(before, before, true) {
		t.Fatal("unchanged halted observation rejected")
	}
	forward := before
	forward.CursorBlock++
	forward.CursorRevision++
	forward.CursorHash = common.HexToHash("0x02")
	if approvedV3DisplayCursorProgress(before, forward, true) || !approvedV3DisplayCursorProgress(before, forward, false) {
		t.Fatal("display halted scope confused with running forward progress")
	}
	for _, change := range []func(*idov3whitelist.ApprovedLockedCandidate){func(c *idov3whitelist.ApprovedLockedCandidate) { c.CursorRevision++ }, func(c *idov3whitelist.ApprovedLockedCandidate) { c.CursorHash = common.HexToHash("0x03") }, func(c *idov3whitelist.ApprovedLockedCandidate) { c.CursorBlock-- }} {
		after := before
		change(&after)
		if approvedV3DisplayCursorProgress(before, after, true) {
			t.Fatal("halt/resume or rewrite accepted")
		}
	}
}

func TestDisplayAdmissionStillRequiresExecutedSetBeforeRPC(t *testing.T) {
	for _, halted := range []bool{false, true} {
		conn, mock, e := sqlmock.New()
		if e != nil {
			t.Fatal(e)
		}
		database, e := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
		if e != nil {
			t.Fatal(e)
		}
		mock.ExpectQuery("FROM projects p").WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
		var rpc V3LifecycleRecoveryRPC = struct{ V3LifecycleRecoveryRPC }{}
		if _, _, e := ReadApprovedV3DisplayAdmission(context.Background(), database, rpc, 19, common.HexToHash("0x01"), common.HexToHash("0x02"), halted); e == nil {
			t.Fatal("missing approved source admitted")
		}
		if e := mock.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
		conn.Close()
	}
}

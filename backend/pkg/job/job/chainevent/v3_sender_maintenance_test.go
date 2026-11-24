package chainevent

import (
	"testing"
	"time"

	"github.com/spf13/viper"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/v3diagnostic"
)

func TestSenderMaintenanceDefaultOffAndBoundedBackoff(t *testing.T) {
	old := viper.Get("idoV3RetiredNonce.maintenance.enabled")
	defer viper.Set("idoV3RetiredNonce.maintenance.enabled", old)
	viper.Set("idoV3RetiredNonce.maintenance.enabled", false)
	j := &ApprovedV3ScannerJob{}
	j.runSenderMaintenance() // nil DB/source cannot start work
	s := v3SenderMaintenanceState{}
	for i := 0; i < 10; i++ {
		before := time.Now()
		s.record(false)
		if s.next.Sub(before) > 12*time.Second || s.failures > 4 {
			t.Fatal("unbounded retry", s.failures, s.next)
		}
	}
	s.record(true)
	if s.failures != 0 || time.Until(s.next) > 31*time.Second {
		t.Fatal("success cadence")
	}
}

func TestMaintenanceSemanticMismatchDoesNotUseUrgentTransportRetry(t *testing.T) {
	err := v3diagnostic.New("SENDER_EVIDENCE_PENDING", "retired_chain_identity", nil)
	for _, kind := range []string{"verification", "timeout", "rate_limited"} {
		out := chaintx.SenderMaintenanceResult{ReadDiagnostic: &chaintx.V3MaintenanceReadDiagnostic{Kind: kind}}
		scheduled := senderMaintenanceRetryError(err, out)
		if (v3diagnostic.Reason(scheduled) == "SENDER_RETIREMENT_REVIEW_REQUIRED") != (kind == "verification") {
			t.Fatal(kind, scheduled)
		}
	}
}

func TestMaintenanceRetryNeverWaitsPastAdmissionWindow(t *testing.T) {
	now := time.Now()
	s := &v3SenderMaintenanceState{}
	e := v3diagnostic.New("SENDER_EVIDENCE_PENDING", "retired_chain_identity", nil)
	for _, remaining := range []time.Duration{300 * time.Second, 280 * time.Second, 220 * time.Second, 122 * time.Second, -time.Second} {
		expiry := now.Add(remaining)
		s.recordWindow(now, e, &expiry)
		if s.next.Sub(now) > 25*time.Second {
			t.Fatal("retry sleeps through runway", remaining, s.next.Sub(now))
		}
	}
	e = v3diagnostic.New("SENDER_RETIREMENT_REVIEW_REQUIRED", "sender_maintenance_enrollment", nil)
	s.recordWindow(now, e, nil)
	if s.next.Sub(now) < time.Minute {
		t.Fatal("revoked/enrollment failure hot-looped")
	}
}

package chainevent

import (
	"context"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"

	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/chaintx"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/common/log"
	"joincare_server/pkg/common/v3diagnostic"
)

type v3SenderMaintenanceState struct {
	mu       sync.Mutex
	next     time.Time
	failures uint
	attempt  uint64
}

// Independent low-frequency lane. Never runs in scanner/project row locks and
// never signs or sends. Explicit default-off rollout prevents deployment alone
// from introducing production evidence writes. Fixed approved sender/scope.
func (j *ApprovedV3ScannerJob) runSenderMaintenance() {
	if j == nil || j.Database == nil || !viper.GetBool("idoV3RetiredNonce.maintenance.enabled") || !viper.GetBool("idoV3RetiredNonce.enabled") || viper.GetString("application.environment") != "prod" || j.Stream.Mode != "stream" {
		return
	}
	s := &j.senderMaintenance
	if idov3whitelist.ValidateApprovedV3RPCEndpoint(j.RPCURL) != nil {
		return
	}
	if !s.mu.TryLock() {
		return
	}
	defer s.mu.Unlock()
	if time.Now().Before(s.next) {
		return
	}
	s.attempt++
	parent := j.Context
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 100*time.Second)
	defer cancel()
	var identity struct{ UUID, DB string }
	if j.Database.WithContext(ctx).Raw(`SELECT @@server_uuid AS uuid,DATABASE() AS db`).Scan(&identity).Error != nil || identity.UUID != "78dbed8b-77a9-11f1-ac1c-06eab0296b63" || identity.DB != "joincare" {
		s.record(false)
		return
	}
	reader, err := chainrpc.DialScanner(ctx, j.RPCURL)
	if err != nil {
		s.record(false)
		return
	}
	defer reader.Close()
	other, err := chainrpc.DialScanner(ctx, "https://bsc-dataseed1.bnbchain.org")
	if err != nil {
		s.record(false)
		return
	}
	defer other.Close()
	// Scheduling snapshot only. A refreshed/revoked/changed record still must
	// pass the unchanged live gate, two-provider audit and atomic SQL view.
	window, _ := chaintx.ReadV3SenderMaintenanceWindow(ctx, j.Database, 56, common.HexToAddress("0xfd712061ee08e070a930ecd45e935ae5d7f5445c"))
	started := time.Now()
	out, err := chaintx.RefreshIdleV3SenderEvidence(ctx, j.Database, 56, common.HexToAddress("0xfd712061ee08e070a930ecd45e935ae5d7f5445c"), reader, other)
	var deadline *time.Time
	deadline = window.EvidenceExpiresAt
	if out.Refreshed > 0 {
		fresh, _ := chaintx.ReadV3SenderMaintenanceWindow(ctx, j.Database, 56, common.HexToAddress("0xfd712061ee08e070a930ecd45e935ae5d7f5445c"))
		if fresh.EvidenceExpiresAt != nil {
			deadline = fresh.EvidenceExpiresAt
		}
	}
	s.recordWindow(time.Now(), senderMaintenanceRetryError(err, out), deadline)
	if l := log.GetLogger(); l != nil {
		reason, stage := "OK", "complete"
		if err != nil {
			reason, stage = v3diagnostic.Reason(err), v3diagnostic.Stage(err)
		}
		fields := map[string]any{"success": err == nil, "refreshed": out.Refreshed, "skipped": out.Skipped, "skip_reason": out.SkipReason, "failed_submission_id": out.FailedSubmission, "reason_code": reason, "stage": stage, "elapsed_ms": time.Since(started).Milliseconds(), "next_attempt_at": s.next.UTC().Format(time.RFC3339Nano)}
		if deadline != nil {
			fields["evidence_remaining_s"] = int64(time.Until(*deadline).Seconds())
		}
		if d := out.ReadDiagnostic; d != nil {
			fields["provider"] = d.Provider
			fields["rpc_method"] = d.Method
			fields["rpc_error_kind"] = d.Kind
			fields["rpc_elapsed_ms"] = d.Millis
			fields["rpc_attempts"] = d.Attempts
		}
		l.WithFields(fields).Info("V3 sender retirement maintenance")
	}
}

// Retry cadence is not an authority decision. A successful RPC returning
// mismatched chain/receipt/nonce evidence is NOT a transient transport failure.
// Keep its original diagnostic, but avoid an aggressive background retry loop.
func senderMaintenanceRetryError(err error, out chaintx.SenderMaintenanceResult) error {
	if err != nil && ((out.ReadDiagnostic != nil && out.ReadDiagnostic.Kind == "verification") || v3diagnostic.Stage(err) == "sender_maintenance_provider_agreement") {
		return v3diagnostic.New("SENDER_RETIREMENT_REVIEW_REQUIRED", v3diagnostic.Stage(err), nil)
	}
	return err
}
func (s *v3SenderMaintenanceState) record(ok bool) {
	var e error
	if !ok {
		e = v3diagnostic.New("SENDER_EVIDENCE_UNAVAILABLE", "sender_maintenance_selection", nil)
	}
	s.recordWindow(time.Now(), e, nil)
}

func (s *v3SenderMaintenanceState) recordWindow(now time.Time, err error, expires *time.Time) {
	if err == nil {
		s.failures = 0
		s.next = now.Add(30 * time.Second)
		return
	}
	if s.failures < 4 {
		s.failures++
	}
	delay := time.Duration(15*(1<<s.failures)) * time.Second
	// Explicit authority/revocation failures are never hot-looped. Finite
	// background retries do not enroll/revive them or relax any condition.
	if v3diagnostic.Reason(err) != "SENDER_RETIREMENT_REVIEW_REQUIRED" {
		// Do not schedule beyond the request's remaining180s runway after
		// allowing the independent audit90s plus the5s scheduler granularity.
		latest := time.Duration(0)
		if expires != nil {
			latest = expires.Sub(now) - chaintx.SenderPreflightReserve - 95*time.Second
		}
		if latest < 10*time.Second {
			latest = 10 * time.Second
		}
		if delay > latest {
			delay = latest
		}
	}
	// Deterministic <=2s jitter decorrelates service retries, never grants
	// evidence or changes its expiry. First urgency retry stays <=12s.
	delay += time.Duration(s.attempt%3) * time.Second
	s.next = now.Add(delay)
}

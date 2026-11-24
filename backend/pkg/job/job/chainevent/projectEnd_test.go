package chainevent

import (
	"testing"
	"time"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

func TestSubmissionHashValue(t *testing.T) {
	if got := submissionHashValue(nil); got != "" {
		t.Fatalf("nil submission hash=%q", got)
	}
	hash := "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if got := submissionHashValue(&entity.ChainTxSubmission{TxHash: &hash}); got != hash {
		t.Fatalf("submission hash=%q want=%q", got, hash)
	}
}

func TestEligibleForExactRebroadcast(t *testing.T) {
	now := time.Now()
	old := now.Add(-time.Minute)
	recent := now.Add(-10 * time.Second)
	tests := []struct {
		name string
		row  entity.ChainTxSubmission
		want bool
	}{
		{name: "intent waits for external preflight", row: entity.ChainTxSubmission{State: constant.ChainTxStateIntent}, want: false},
		{name: "ready can prepare", row: entity.ChainTxSubmission{State: constant.ChainTxStateReady}, want: true},
		{name: "signed can broadcast", row: entity.ChainTxSubmission{State: constant.ChainTxStateSigned}, want: true},
		{name: "recent submitted waits", row: entity.ChainTxSubmission{State: constant.ChainTxStateSubmitted, LastBroadcastAt: &recent}, want: false},
		{name: "old submitted replays exact raw", row: entity.ChainTxSubmission{State: constant.ChainTxStateSubmitted, LastBroadcastAt: &old}, want: true},
		{name: "unknown without timestamp replays", row: entity.ChainTxSubmission{State: constant.ChainTxStateBroadcastUnknown}, want: true},
		{name: "mined never rebroadcasts", row: entity.ChainTxSubmission{State: constant.ChainTxStateMinedSuccess}, want: false},
		{name: "ambiguous never rebroadcasts", row: entity.ChainTxSubmission{State: constant.ChainTxStateAmbiguous}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := eligibleForExactRebroadcast(test.row, now); got != test.want {
				t.Fatalf("eligible=%t want=%t", got, test.want)
			}
		})
	}
}

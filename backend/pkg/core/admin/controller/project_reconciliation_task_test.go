package controller

import (
	"testing"

	"github.com/spf13/viper"
)

func TestAsyncReconciliationParticipantLimit(t *testing.T) {
	viper.Set("chain.project.reconciliation.maxParticipants", 0)
	if got := asyncReconciliationParticipantLimit(); got != defaultAsyncReconciliationParticipants {
		t.Fatalf("default participant limit=%d", got)
	}
	viper.Set("chain.project.reconciliation.maxParticipants", 5000)
	if got := asyncReconciliationParticipantLimit(); got != 5000 {
		t.Fatalf("configured participant limit=%d", got)
	}
	viper.Set("chain.project.reconciliation.maxParticipants", 50000)
	if got := asyncReconciliationParticipantLimit(); got != hardAsyncReconciliationParticipants {
		t.Fatalf("hard participant limit=%d", got)
	}
}

func TestCurrentReconciliationBalanceFailsClosedAfterReorg(t *testing.T) {
	balanced, unbalanced := "BALANCED", "UNBALANCED"
	if currentReconciliationBalance(nil, false) != nil || currentReconciliationBalance(&balanced, true) != nil {
		t.Fatal("unfinished or reorg-invalidated evidence must not project a balance conclusion")
	}
	if value := currentReconciliationBalance(&balanced, false); value == nil || !*value {
		t.Fatal("current BALANCED evidence was not projected")
	}
	if value := currentReconciliationBalance(&unbalanced, false); value == nil || *value {
		t.Fatal("current UNBALANCED evidence was not projected")
	}
}

func TestAsyncReconciliationSourceLimitFitsMediumBlob(t *testing.T) {
	const mediumBlobMaxBytes = 16*1024*1024 - 1
	if maxAsyncReconciliationSourceBytes <= 0 || maxAsyncReconciliationSourceBytes >= mediumBlobMaxBytes {
		t.Fatalf("unsafe reconciliation source limit %d", maxAsyncReconciliationSourceBytes)
	}
	if maxAsyncReconciliationFactRows < hardAsyncReconciliationParticipants {
		t.Fatalf("fact row limit %d cannot cover wallet limit %d", maxAsyncReconciliationFactRows, hardAsyncReconciliationParticipants)
	}
}

func TestIDOReconciliationPublicErrorsAreAllowlisted(t *testing.T) {
	for code, want := range map[string]string{
		"RPC_UNAVAILABLE":      "configured chain RPC is unavailable",
		"SNAPSHOT_INVALIDATED": "the fixed evidence block or trusted contract identity is no longer valid",
		"SOURCE_CHANGED":       "database source facts changed before finalization",
		"RPC_TIMEOUT":          "fixed-block chain reads timed out",
		"PARTICIPANT_LIMIT":    "project participant count exceeds the configured asynchronous reconciliation limit",
		"INTERNAL_ERROR":       "reconciliation execution failed; inspect restricted job logs",
	} {
		if got := safeIDOReconciliationError(code); got != want {
			t.Fatalf("code=%s got=%q", code, got)
		}
	}
	if leaked := safeIDOReconciliationError("mysql password=secret rpc=https://private"); leaked != "" {
		t.Fatalf("unknown internal error leaked: %q", leaked)
	}
}

func TestASCIIIdempotencyToken(t *testing.T) {
	if !asciiToken("request-0123456789") || asciiToken("contains space") || asciiToken("审计请求") {
		t.Fatal("idempotency ASCII token validation drift")
	}
}

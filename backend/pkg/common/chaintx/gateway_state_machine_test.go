package chaintx

import (
	"context"
	"testing"
	"time"
)

type neverGateway struct{}

func (neverGateway) Commit(context.Context, GatewayCommitRequest) (GatewayJournalEntry, error) {
	panic("must not call")
}
func (neverGateway) Journal(context.Context, GatewayJournalKey) (GatewayJournalEntry, error) {
	panic("must not call")
}
func TestRunGatewaySigningRejectsMissingUnsignedTxBeforeGateway(t *testing.T) {
	_, err := RunGatewaySigning(context.Background(), nil, neverGateway{}, SigningReservation{}, nil, 1, time.Now())
	if err == nil {
		t.Fatal("expected fail closed")
	}
}

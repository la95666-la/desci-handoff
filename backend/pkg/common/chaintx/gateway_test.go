package chaintx

import "testing"

func TestGatewayJournalFailsClosed(t *testing.T) {
	req := GatewayCommitRequest{GatewayJournalKey: GatewayJournalKey{ChainID: 56, ExpectedSignerAddress: "0x1000000000000000000000000000000000000001", Nonce: 9}, SubmissionID: 2, ReplacementGeneration: 0, UnsignedTxDigest: "0x" + repeatHex('1'), SigningDigest: "0x" + repeatHex('2')}
	ok := GatewayJournalEntry{GatewayJournalKey: req.GatewayJournalKey, SubmissionID: 2, ReplacementGeneration: 0, State: GatewayStateCommitted, UnsignedTxDigest: req.UnsignedTxDigest, SigningDigest: req.SigningDigest, RawTransaction: []byte{1}, TxHash: "0x" + repeatHex('3')}
	if err := ValidateGatewayCommit(ok, req); err != nil {
		t.Fatal(err)
	}
	bad := ok
	bad.ExpectedSignerAddress = "0x2000000000000000000000000000000000000002"
	if ValidateGatewayCommit(bad, req) == nil {
		t.Fatal("expected identity mismatch")
	}
	unknown := GatewayJournalEntry{State: GatewayStateUnknown}
	state, err := ReconcileGatewayJournal(unknown)
	if err != nil || state != ChainTxStateSigningRecovery {
		t.Fatalf("%s %v", state, err)
	}
}
func repeatHex(c byte) string {
	b := make([]byte, 64)
	for i := range b {
		b[i] = c
	}
	return string(b)
}

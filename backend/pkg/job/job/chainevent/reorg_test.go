package chainevent

import (
	"strings"
	"testing"
)

func TestReorgEvidenceDigestsAreDeterministicAndDomainSeparated(t *testing.T) {
	blockHash := "0x" + strings.Repeat("a", 64)
	otherHash := "0x" + strings.Repeat("b", 64)
	checkpoint := checkpointDigest(97, 500, blockHash, reorgZeroDigest)
	if len(checkpoint) != 64 || checkpoint != checkpointDigest(97, 500, blockHash, reorgZeroDigest) {
		t.Fatalf("checkpoint digest is not deterministic: %q", checkpoint)
	}
	incident := reorgIncidentDigest(97, "0x1000000000000000000000000000000000000001", 500, blockHash, otherHash)
	event := reorgEventDigest(1, 1, reorgEventDetected, "cursor changed", reorgZeroDigest)
	if incident == checkpoint || event == checkpoint || event == incident {
		t.Fatal("reorg evidence digest domains collide")
	}
}

func TestCursorCanonicalityErrorRetainsStructuredEvidence(t *testing.T) {
	err := &cursorCanonicalityError{BlockNumber: 500, Expected: "0xold", Observed: "0xnew"}
	if err.BlockNumber != 500 || err.Expected != "0xold" || err.Observed != "0xnew" || !strings.Contains(err.Error(), "reconciliation required") {
		t.Fatalf("unexpected cursor canonicality error: %+v", err)
	}
}

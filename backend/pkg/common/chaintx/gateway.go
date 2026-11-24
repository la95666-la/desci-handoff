package chaintx

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// SignPolicyGateway is the only Fair Launch signing seam. Implementations must
// use a remote policy-enforcing signer; there is deliberately no private-key API.
type SignPolicyGateway interface {
	Commit(context.Context, GatewayCommitRequest) (GatewayJournalEntry, error)
	Journal(context.Context, GatewayJournalKey) (GatewayJournalEntry, error)
}

type GatewayJournalKey struct {
	ChainID               uint64
	ExpectedSignerAddress string
	Nonce                 uint64
}
type GatewayCommitRequest struct {
	GatewayJournalKey
	SubmissionID          uint64
	ReplacementGeneration uint32
	ContractAddress       string
	UnsignedTxDigest      string
	SigningDigest         string
	UnsignedTransaction   []byte
}
type GatewayJournalEntry struct {
	GatewayJournalKey
	SubmissionID          uint64
	ReplacementGeneration uint32
	State                 string
	UnsignedTxDigest      string
	SigningDigest         string
	RawTransaction        []byte
	TxHash                string
}

const (
	GatewayStateAbsent    = "absent"
	GatewayStateCommitted = "committed"
	GatewayStateRejected  = "rejected"
	GatewayStateUnknown   = "unknown"
)

func ValidateGatewayCommit(res GatewayJournalEntry, req GatewayCommitRequest) error {
	if res.ChainID != req.ChainID || !strings.EqualFold(res.ExpectedSignerAddress, req.ExpectedSignerAddress) || res.Nonce != req.Nonce || res.SubmissionID != req.SubmissionID || res.ReplacementGeneration != req.ReplacementGeneration || !strings.EqualFold(res.UnsignedTxDigest, req.UnsignedTxDigest) || !strings.EqualFold(res.SigningDigest, req.SigningDigest) {
		return errors.New("gateway journal identity mismatch")
	}
	if res.State != GatewayStateCommitted || len(res.RawTransaction) == 0 || !validTxHash(res.TxHash) {
		return fmt.Errorf("gateway did not durably commit signed transaction: %s", res.State)
	}
	return nil
}

func ReconcileGatewayJournal(entry GatewayJournalEntry) (string, error) {
	switch entry.State {
	case GatewayStateAbsent:
		return "safe_to_release_unsigned_reservation", nil
	case GatewayStateCommitted:
		if len(entry.RawTransaction) == 0 || !validTxHash(entry.TxHash) {
			return ChainTxStateSigningRecovery, errors.New("committed gateway journal is incomplete")
		}
		return "raw_tx_committed", nil
	case GatewayStateRejected:
		return "safe_to_release_unsigned_reservation", nil
	case GatewayStateUnknown, "":
		return ChainTxStateSigningRecovery, nil
	default:
		return ChainTxStateSigningRecovery, fmt.Errorf("unsupported gateway journal state %q", entry.State)
	}
}

func validTxHash(s string) bool {
	if len(s) != 66 || !strings.HasPrefix(s, "0x") {
		return false
	}
	for _, c := range s[2:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

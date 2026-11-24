package idov3whitelist

import (
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// CandidateMemberRow describes one unapproved, append-only foundation row.
// This check is suitable for offline DB audits and cannot approve, execute,
// finalize, or provide membership proofs.
type CandidateMemberRow struct {
	Index  uint32
	Wallet string
	Leaf   common.Hash
}

// CheckCandidateRows checks that the entire supplied row set is contiguous,
// canonical and consistent with the supplied candidate digest. It does not
// authenticate who selected these rows or whether they came from durable DB.
func CheckCandidateRows(scope Scope, candidate SnapshotCommitment, rows []CandidateMemberRow) error {
	if err := validateScope(scope); err != nil {
		return err
	}
	if candidate.Version == 0 || candidate.MemberCount == 0 || candidate.MemberCount > MaxWallets || len(rows) != int(candidate.MemberCount) {
		return errors.New("V3 candidate member count mismatch")
	}
	wallets := make([]string, len(rows))
	for i, row := range rows {
		if row.Index != uint32(i) || row.Leaf == (common.Hash{}) || !strings.HasPrefix(row.Wallet, "0x") || row.Wallet != strings.ToLower(row.Wallet) {
			return errors.New("V3 candidate row is not canonical")
		}
		wallet, err := parseWallet(row.Wallet)
		if err != nil {
			return errors.New("V3 candidate wallet is invalid")
		}
		computed, err := Leaf(scope, wallet)
		if err != nil || computed != row.Leaf {
			return errors.New("V3 candidate leaf does not bind its wallet")
		}
		wallets[i] = row.Wallet
	}
	tree, err := Build(scope, wallets)
	if err != nil {
		return err
	}
	for i, member := range tree.Members() {
		if member.Leaf != rows[i].Leaf {
			return errors.New("V3 candidate rows are not in canonical leaf order")
		}
	}
	commitment, err := tree.CommitSnapshot(scope, candidate.Version)
	if err != nil || commitment != candidate {
		return errors.New("V3 candidate rows do not match complete snapshot commitment")
	}
	return nil
}

package idov3whitelist

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestCheckCandidateRowsRequiresCompleteOrderedCanonicalLeaves(t *testing.T) {
	scope := Scope{ChainID: big.NewInt(31337), ProxyAddress: common.HexToAddress("0x1234567890123456789012345678901234567890"), ProjectID: big.NewInt(41376991395446785)}
	tree, err := Build(scope, []string{"0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222", "0x3333333333333333333333333333333333333333"})
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := tree.CommitSnapshot(scope, 7)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]CandidateMemberRow, 0, 3)
	for i, member := range tree.Members() {
		rows = append(rows, CandidateMemberRow{Index: uint32(i), Wallet: strings.ToLower(member.Wallet.Hex()), Leaf: member.Leaf})
	}
	check := func(scope Scope, candidate SnapshotCommitment, rows []CandidateMemberRow) error {
		return CheckCandidateRows(scope, candidate, rows)
	}
	if err := check(scope, candidate, rows); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Scope, *SnapshotCommitment, *[]CandidateMemberRow){
		"incomplete":        func(_ *Scope, _ *SnapshotCommitment, r *[]CandidateMemberRow) { *r = (*r)[:2] },
		"different chain":   func(s *Scope, _ *SnapshotCommitment, _ *[]CandidateMemberRow) { s.ChainID = big.NewInt(56) },
		"different project": func(s *Scope, _ *SnapshotCommitment, _ *[]CandidateMemberRow) { s.ProjectID = big.NewInt(1) },
		"different proxy": func(s *Scope, _ *SnapshotCommitment, _ *[]CandidateMemberRow) {
			s.ProxyAddress = common.HexToAddress("0x2234567890123456789012345678901234567890")
		},
		"wrong version": func(_ *Scope, c *SnapshotCommitment, _ *[]CandidateMemberRow) { c.Version++ },
		"wrong digest":  func(_ *Scope, c *SnapshotCommitment, _ *[]CandidateMemberRow) { c.Digest = common.HexToHash("0x12") },
		"missing index": func(_ *Scope, _ *SnapshotCommitment, r *[]CandidateMemberRow) { (*r)[1].Index = 9 },
		"wrong leaf": func(_ *Scope, _ *SnapshotCommitment, r *[]CandidateMemberRow) {
			(*r)[0].Leaf = common.HexToHash("0x33")
		},
		"reordered": func(_ *Scope, _ *SnapshotCommitment, r *[]CandidateMemberRow) {
			(*r)[0], (*r)[1] = (*r)[1], (*r)[0]
			(*r)[0].Index = 0
			(*r)[1].Index = 1
		},
		"duplicate": func(_ *Scope, _ *SnapshotCommitment, r *[]CandidateMemberRow) {
			(*r)[1].Wallet = (*r)[0].Wallet
			(*r)[1].Leaf = (*r)[0].Leaf
		},
		"mixed case": func(_ *Scope, _ *SnapshotCommitment, r *[]CandidateMemberRow) {
			(*r)[0].Wallet = strings.ToUpper((*r)[0].Wallet)
		},
	} {
		t.Run(name, func(t *testing.T) {
			s := scope
			c := candidate
			r := append([]CandidateMemberRow(nil), rows...)
			change(&s, &c, &r)
			if err := check(s, c, r); err == nil {
				t.Fatal("invalid candidate rows accepted")
			}
		})
	}
}

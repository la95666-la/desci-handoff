package idov3whitelist

import (
	"bytes"
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestSnapshotCommitmentBindsFullMemberSetAndScope(t *testing.T) {
	vectors := loadVectors(t)
	fixtureBytes, err := os.ReadFile("testdata/ido-v3-snapshot-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var committed struct {
		Schema string `json:"snapshotSchema"`
		Cases  []struct {
			Root, MembersDigest, Digest, TargetVersion string
			MemberCount                                uint32
		} `json:"cases"`
	}
	if err := json.Unmarshal(fixtureBytes, &committed); err != nil {
		t.Fatal(err)
	}
	if committed.Schema != snapshotSchema.Hex() || len(committed.Cases) != len(vectors.Cases) {
		t.Fatal("snapshot schema or case count drift")
	}
	for i, item := range vectors.Cases {
		scope := Scope{
			ChainID: parseDecimal(t, item.Scope.ChainID), ProjectID: parseDecimal(t, item.Scope.ProjectID),
			ProxyAddress: common.HexToAddress(item.Scope.ProxyAddress),
		}
		tree, err := Build(scope, item.Wallets)
		if err != nil {
			t.Fatal(err)
		}
		candidate, err := tree.CommitSnapshot(scope, 1)
		if err != nil {
			t.Fatal(err)
		}
		want := committed.Cases[i]
		if candidate.Root.Hex() != want.Root || candidate.MembersDigest.Hex() != want.MembersDigest ||
			candidate.Digest.Hex() != want.Digest || candidate.MemberCount != want.MemberCount || want.TargetVersion != "1" {
			t.Fatalf("Go/JS snapshot drift case %d: got=%+v want=%+v", i, candidate, want)
		}
	}
	base := vectors.Cases[2]
	scope := Scope{
		ChainID: parseDecimal(t, base.Scope.ChainID), ProjectID: parseDecimal(t, base.Scope.ProjectID),
		ProxyAddress: common.HexToAddress(base.Scope.ProxyAddress),
	}
	tree, err := Build(scope, base.Wallets)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := tree.CommitSnapshot(scope, 1)
	if err != nil || actual.Root != tree.Root || actual.Version != 1 || actual.MemberCount != 3 {
		t.Fatalf("invalid member snapshot: %+v err=%v", actual, err)
	}
	leaves := make([]byte, 0, 96)
	for _, entry := range base.Leaves {
		leaves = append(leaves, common.HexToHash(entry.Hash).Bytes()...)
	}
	if actual.MembersDigest != crypto.Keccak256Hash(leaves) {
		t.Fatal("member digest did not bind sorted leaves")
	}
	bytes32, err := abi.NewType("bytes32", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	uint256, err := abi.NewType("uint256", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	address, err := abi.NewType("address", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	packed, err := (abi.Arguments{
		{Type: bytes32}, {Type: uint256}, {Type: address}, {Type: uint256},
		{Type: uint256}, {Type: bytes32}, {Type: bytes32}, {Type: uint256},
	}).Pack(snapshotSchema, scope.ChainID, scope.ProxyAddress, scope.ProjectID, big.NewInt(1),
		tree.Root, actual.MembersDigest, big.NewInt(3))
	if err != nil {
		t.Fatal(err)
	}
	if actual.Digest != crypto.Keccak256Hash(packed) {
		t.Fatal("snapshot digest differs from independent ABI encoder")
	}
	reordered := append([]string(nil), base.Wallets...)
	reordered[0], reordered[2] = reordered[2], reordered[0]
	sameTree, err := Build(scope, reordered)
	if err != nil {
		t.Fatal(err)
	}
	same, err := sameTree.CommitSnapshot(scope, 1)
	if err != nil || same.Digest != actual.Digest {
		t.Fatalf("input order changed canonical commitment: %v", err)
	}
	updatedVersion, err := tree.CommitSnapshot(scope, 2)
	if err != nil || updatedVersion.Digest == actual.Digest {
		t.Fatalf("target version unbound: %v", err)
	}
	otherScope := scope
	otherScope.ProjectID = new(big.Int).Add(scope.ProjectID, big.NewInt(1))
	if _, err := tree.CommitSnapshot(otherScope, 1); err == nil {
		t.Fatal("scope mismatch accepted")
	}
	if _, err := tree.CommitSnapshot(scope, 0); err == nil {
		t.Fatal("zero target version accepted")
	}
	if _, err := (*Tree)(nil).CommitSnapshot(scope, 1); err == nil {
		t.Fatal("nil tree accepted")
	}
	// An independently produced different member set cannot pass as the
	// original approved snapshot, even when its version is the same.
	changed, err := Build(scope, base.Wallets[1:])
	if err != nil {
		t.Fatal(err)
	}
	other, err := changed.CommitSnapshot(scope, 1)
	if err != nil || bytes.Equal(actual.Digest[:], other.Digest[:]) {
		t.Fatalf("member deletion not bound: %v", err)
	}
	tree.Root = other.Root
	if _, err := tree.CommitSnapshot(scope, 1); err == nil {
		t.Fatal("mutable exported root produced a snapshot")
	}
	if _, _, err := tree.ProofForPolicy(scope, Policy{Enabled: true, Root: other.Root, Version: 1}, base.Wallets[0]); err == nil {
		t.Fatal("mutable exported root produced a policy proof")
	}
}

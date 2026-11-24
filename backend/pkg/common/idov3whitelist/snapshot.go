package idov3whitelist

import (
	"bytes"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var snapshotSchema = crypto.Keccak256Hash([]byte("JOINCARE_IDO_V3_WHITELIST_SNAPSHOT_V1"))

// SnapshotCommitment commits to a complete, canonically ordered member set,
// not just the Merkle root. This digest is a payload for subsequent independent
// approval and immutable storage: it is NOT an approval, signature, receipt,
// finality proof, or authorization to serve a user proof.
type SnapshotCommitment struct {
	Root          common.Hash
	MembersDigest common.Hash
	Digest        common.Hash
	Version       uint64
	MemberCount   uint32
}

// CommitSnapshot binds the new effective version and all canonical member
// leaves to the chain, V3 proxy and project. Callers must persist the complete
// member list, independently bind the resulting digest to two distinct
// approvers, and verify chain execution/finality before publishing a proof.
func (tree *Tree) CommitSnapshot(scope Scope, targetVersion uint64) (SnapshotCommitment, error) {
	if tree == nil || len(tree.levels) == 0 || len(tree.members) == 0 {
		return SnapshotCommitment{}, errors.New("invalid whitelist tree")
	}
	if tree.Root != tree.levels[len(tree.levels)-1][0] {
		return SnapshotCommitment{}, errors.New("whitelist tree root changed")
	}
	if err := validateScope(scope); err != nil {
		return SnapshotCommitment{}, err
	}
	if scope.ChainID.Cmp(tree.scope.ChainID) != 0 || scope.ProjectID.Cmp(tree.scope.ProjectID) != 0 || scope.ProxyAddress != tree.scope.ProxyAddress {
		return SnapshotCommitment{}, errors.New("whitelist project scope mismatch")
	}
	if targetVersion == 0 {
		return SnapshotCommitment{}, errors.New("invalid whitelist target version")
	}
	leaves := make([]byte, 0, 32*len(tree.members))
	for i, member := range tree.members {
		if i > 0 && bytes.Compare(tree.members[i-1].Leaf[:], member.Leaf[:]) >= 0 {
			return SnapshotCommitment{}, errors.New("invalid canonical member order")
		}
		leaves = append(leaves, member.Leaf[:]...)
	}
	membersDigest := crypto.Keccak256Hash(leaves)
	// Fixed-width ABI slots: schema, chainId, proxy, projectId, version,
	// root, membersDigest, memberCount. No ambiguity from string joining or
	// packed dynamic array encoding.
	var encoded [32 * 8]byte
	copy(encoded[:32], snapshotSchema[:])
	scope.ChainID.FillBytes(encoded[32:64])
	copy(encoded[64+12:96], scope.ProxyAddress[:])
	scope.ProjectID.FillBytes(encoded[96:128])
	new(big.Int).SetUint64(targetVersion).FillBytes(encoded[128:160])
	copy(encoded[160:192], tree.Root[:])
	copy(encoded[192:224], membersDigest[:])
	new(big.Int).SetUint64(uint64(len(tree.members))).FillBytes(encoded[224:256])
	return SnapshotCommitment{
		Root: tree.Root, MembersDigest: membersDigest,
		Digest: crypto.Keccak256Hash(encoded[:]), Version: targetVersion,
		MemberCount: uint32(len(tree.members)),
	}, nil
}

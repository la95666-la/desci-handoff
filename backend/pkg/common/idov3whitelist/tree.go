// Package idov3whitelist implements the exact IDO V3 Merkle tree wire format.
// It does not read a database, RPC endpoint, or signing key. A root or proof
// produced here is not an authorization until the governed on-chain policy is
// finalized and matches the full, immutable member snapshot.
package idov3whitelist

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const MaxWallets = 4096

var leafSchema = crypto.Keccak256Hash([]byte("JOINCARE_IDO_V3_WHITELIST_LEAF_V1"))

type Scope struct {
	ChainID      *big.Int
	ProxyAddress common.Address
	ProjectID    *big.Int
}

type Member struct {
	Wallet common.Address
	Leaf   common.Hash
}

// Tree is a complete, normalized membership snapshot, not just a root. Keep
// the member list immutable after approving a version so proofs remain valid.
type Tree struct {
	Root    common.Hash
	members []Member
	levels  [][]common.Hash
	scope   Scope
}

// Members returns a copy in canonical leaf order. Mutation by the caller
// cannot change subsequent membership/proof lookups on the original tree.
func (tree *Tree) Members() []Member {
	if tree == nil {
		return nil
	}
	return append([]Member(nil), tree.members...)
}

func positiveUint256(value *big.Int, field string) error {
	if value == nil || value.Sign() <= 0 || value.BitLen() > 256 {
		return fmt.Errorf("invalid %s", field)
	}
	return nil
}

func validateScope(scope Scope) error {
	if err := positiveUint256(scope.ChainID, "chainId"); err != nil {
		return err
	}
	if scope.ProxyAddress == (common.Address{}) {
		return errors.New("invalid proxyAddress")
	}
	return positiveUint256(scope.ProjectID, "projectId")
}

func parseWallet(raw string) (common.Address, error) {
	// Mirror ethers.getAddress: require 0x, exactly 20 bytes, and validate
	// EIP-55 when the input contains both upper- and lowercase hex digits.
	if len(raw) != 42 || !strings.HasPrefix(raw, "0x") || !common.IsHexAddress(raw) {
		return common.Address{}, errors.New("invalid wallet")
	}
	wallet := common.HexToAddress(raw)
	if wallet == (common.Address{}) {
		return common.Address{}, errors.New("invalid wallet")
	}
	hex := raw[2:]
	if hex != strings.ToLower(hex) && hex != strings.ToUpper(hex) && raw != wallet.Hex() {
		return common.Address{}, errors.New("invalid wallet checksum")
	}
	return wallet, nil
}

// Leaf reproduces keccak256(abi.encode(schema,chainId,proxy,projectId,buyer)).
// All five fields occupy full 32-byte ABI slots; never use packed encoding.
func Leaf(scope Scope, buyer common.Address) (common.Hash, error) {
	if err := validateScope(scope); err != nil {
		return common.Hash{}, err
	}
	if buyer == (common.Address{}) {
		return common.Hash{}, errors.New("invalid wallet")
	}
	var data [32 * 5]byte
	copy(data[:32], leafSchema[:])
	scope.ChainID.FillBytes(data[32:64])
	copy(data[64+12:96], scope.ProxyAddress[:])
	scope.ProjectID.FillBytes(data[96:128])
	copy(data[128+12:160], buyer[:])
	return crypto.Keccak256Hash(data[:]), nil
}

func pairHash(left, right common.Hash) common.Hash {
	var data [64]byte
	if bytes.Compare(left[:], right[:]) > 0 {
		left, right = right, left
	}
	copy(data[:32], left[:])
	copy(data[32:], right[:])
	return crypto.Keccak256Hash(data[:])
}

// Build rejects empty or duplicate wallet sets; members are sorted by leaf
// bytes and unpaired odd nodes are promoted unchanged. This matches the
// Solidity MerkleProof.verifyCalldata commutative pair hashing.
func Build(scope Scope, wallets []string) (*Tree, error) {
	if err := validateScope(scope); err != nil {
		return nil, err
	}
	if len(wallets) == 0 || len(wallets) > MaxWallets {
		return nil, errors.New("invalid whitelist size")
	}
	seen := make(map[common.Address]struct{}, len(wallets))
	members := make([]Member, 0, len(wallets))
	for _, raw := range wallets {
		wallet, err := parseWallet(raw)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[wallet]; exists {
			return nil, errors.New("duplicate wallet")
		}
		seen[wallet] = struct{}{}
		leaf, err := Leaf(scope, wallet)
		if err != nil {
			return nil, err
		}
		members = append(members, Member{Wallet: wallet, Leaf: leaf})
	}
	sort.Slice(members, func(i, j int) bool {
		return bytes.Compare(members[i].Leaf[:], members[j].Leaf[:]) < 0
	})
	first := make([]common.Hash, len(members))
	for i := range members {
		first[i] = members[i].Leaf
	}
	levels := [][]common.Hash{first}
	for len(levels[len(levels)-1]) > 1 {
		current := levels[len(levels)-1]
		next := make([]common.Hash, 0, (len(current)+1)/2)
		for i := 0; i < len(current); i += 2 {
			if i+1 < len(current) {
				next = append(next, pairHash(current[i], current[i+1]))
			} else {
				next = append(next, current[i])
			}
		}
		levels = append(levels, next)
	}
	return &Tree{
		Root: levels[len(levels)-1][0], members: members, levels: levels,
		scope: Scope{ChainID: new(big.Int).Set(scope.ChainID), ProjectID: new(big.Int).Set(scope.ProjectID), ProxyAddress: scope.ProxyAddress},
	}, nil
}

// Policy is an independently obtained and finalized V3 on-chain observation.
// The caller is responsible for project identity, approval, wallet binding,
// trusted RPC, finality and reorg detection. Never use an unverified DB policy
// or a Warmup registration record as this observation.
type Policy struct {
	Enabled bool
	Root    common.Hash
	Version uint64
}

// ProofForPolicy refuses to serve stale or cross-project proof material. The
// returned version must be included in the purchaseWithProof transaction; the
// contract rechecks it at execution. The caller must not treat a matching root
// alone as proof of admin approval or chain finality.
func (tree *Tree) ProofForPolicy(scope Scope, policy Policy, rawWallet string) (uint64, []common.Hash, error) {
	if tree == nil || len(tree.levels) == 0 {
		return 0, nil, errors.New("invalid whitelist tree")
	}
	if tree.Root != tree.levels[len(tree.levels)-1][0] {
		return 0, nil, errors.New("whitelist tree root changed")
	}
	if err := validateScope(scope); err != nil {
		return 0, nil, err
	}
	if scope.ChainID.Cmp(tree.scope.ChainID) != 0 || scope.ProjectID.Cmp(tree.scope.ProjectID) != 0 || scope.ProxyAddress != tree.scope.ProxyAddress {
		return 0, nil, errors.New("whitelist project scope mismatch")
	}
	if !policy.Enabled || policy.Version == 0 || policy.Root == (common.Hash{}) || policy.Root != tree.Root {
		return 0, nil, errors.New("whitelist policy not effective for this snapshot")
	}
	proof, err := tree.Proof(rawWallet)
	if err != nil {
		return 0, nil, err
	}
	return policy.Version, proof, nil
}

// Proof returns an independent slice of siblings from leaf to root. Nonmember
// queries fail closed; the caller must still bind the result to a chain/root
// version read from the correct, finalized V3 proxy.
func (tree *Tree) Proof(rawWallet string) ([]common.Hash, error) {
	if tree == nil || len(tree.levels) == 0 {
		return nil, errors.New("invalid whitelist tree")
	}
	wallet, err := parseWallet(rawWallet)
	if err != nil {
		return nil, err
	}
	index := -1
	for i := range tree.members {
		if tree.members[i].Wallet == wallet {
			index = i
			break
		}
	}
	if index < 0 {
		return nil, errors.New("wallet is not in the whitelist")
	}
	proof := make([]common.Hash, 0, len(tree.levels)-1)
	for _, level := range tree.levels[:len(tree.levels)-1] {
		sibling := index ^ 1
		if sibling < len(level) {
			proof = append(proof, level[sibling])
		}
		index /= 2
	}
	return proof, nil
}

package idov3whitelist

import (
	"bytes"
	"encoding/json"
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type vectorFile struct {
	Schema string `json:"schema"`
	Cases  []struct {
		Scope struct {
			ChainID      string `json:"chainId"`
			ProxyAddress string `json:"proxyAddress"`
			ProjectID    string `json:"projectId"`
		} `json:"scope"`
		Wallets []string `json:"wallets"`
		Root    string   `json:"root"`
		Leaves  []struct {
			Wallet string `json:"wallet"`
			Hash   string `json:"hash"`
		} `json:"leaves"`
		Proofs []struct {
			Wallet string   `json:"wallet"`
			Hash   string   `json:"hash"`
			Proof  []string `json:"proof"`
		} `json:"proofs"`
	} `json:"cases"`
}

func loadVectors(t *testing.T) vectorFile {
	t.Helper()
	raw, err := os.ReadFile("testdata/ido-v3-whitelist-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors vectorFile
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	return vectors
}

func parseDecimal(t *testing.T, value string) *big.Int {
	t.Helper()
	parsed, ok := new(big.Int).SetString(value, 10)
	if !ok {
		t.Fatalf("invalid vector decimal %q", value)
	}
	return parsed
}

func TestContractVectorAgreement(t *testing.T) {
	vectors := loadVectors(t)
	if vectors.Schema != leafSchema.Hex() {
		t.Fatalf("schema drift: got=%s want=%s", leafSchema.Hex(), vectors.Schema)
	}
	if len(vectors.Cases) != 4 {
		t.Fatalf("unexpected case count: %d", len(vectors.Cases))
	}
	for _, item := range vectors.Cases {
		scope := Scope{
			ChainID: parseDecimal(t, item.Scope.ChainID), ProjectID: parseDecimal(t, item.Scope.ProjectID),
			ProxyAddress: common.HexToAddress(item.Scope.ProxyAddress),
		}
		tree, err := Build(scope, item.Wallets)
		if err != nil {
			t.Fatal(err)
		}
		if tree.Root.Hex() != item.Root {
			t.Fatalf("root drift for size=%d got=%s want=%s", len(item.Wallets), tree.Root.Hex(), item.Root)
		}
		reversed := append([]string(nil), item.Wallets...)
		for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
			reversed[left], reversed[right] = reversed[right], reversed[left]
		}
		other, err := Build(scope, reversed)
		if err != nil || other.Root != tree.Root {
			t.Fatalf("wallet order changed root size=%d err=%v", len(item.Wallets), err)
		}
		for i, member := range tree.Members() {
			want := item.Leaves[i]
			if member.Wallet.Hex() != want.Wallet || member.Leaf.Hex() != want.Hash {
				t.Fatalf("leaf drift size=%d index=%d: %+v vs %+v", len(item.Wallets), i, member, want)
			}
		}
		for _, entry := range item.Proofs {
			wallet := common.HexToAddress(entry.Wallet)
			gotLeaf, err := Leaf(scope, wallet)
			if err != nil || gotLeaf.Hex() != entry.Hash {
				t.Fatalf("leaf mismatch size=%d wallet=%s err=%v", len(item.Wallets), entry.Wallet, err)
			}
			proof, err := tree.Proof(entry.Wallet)
			if err != nil || len(proof) != len(entry.Proof) {
				t.Fatalf("proof length mismatch size=%d wallet=%s err=%v", len(item.Wallets), entry.Wallet, err)
			}
			current := gotLeaf
			for i, sibling := range proof {
				if sibling.Hex() != entry.Proof[i] {
					t.Fatalf("proof drift size=%d index=%d", len(item.Wallets), i)
				}
				current = pairHash(current, sibling)
			}
			if current != tree.Root {
				t.Fatalf("proof did not produce root size=%d wallet=%s", len(item.Wallets), entry.Wallet)
			}
		}
	}
}

func TestScopeInputAndMembershipGuard(t *testing.T) {
	const buyer = "0x1111111111111111111111111111111111111111"
	scope := Scope{ChainID: big.NewInt(56), ProxyAddress: common.HexToAddress("0x1234567890123456789012345678901234567890"), ProjectID: big.NewInt(9001)}
	for _, wallets := range [][]string{nil, {}, {buyer, strings.ToLower(buyer)}, {"not-a-wallet"}, {common.Address{}.Hex()}} {
		if _, err := Build(scope, wallets); err == nil {
			t.Fatalf("unsafe wallet list accepted: %v", wallets)
		}
	}
	badScopes := []Scope{
		{ChainID: big.NewInt(0), ProxyAddress: scope.ProxyAddress, ProjectID: scope.ProjectID},
		{ChainID: scope.ChainID, ProjectID: scope.ProjectID},
		{ChainID: scope.ChainID, ProxyAddress: scope.ProxyAddress, ProjectID: big.NewInt(0)},
		{ChainID: scope.ChainID, ProxyAddress: scope.ProxyAddress, ProjectID: new(big.Int).Lsh(big.NewInt(1), 256)},
	}
	for _, candidate := range badScopes {
		if _, err := Build(candidate, []string{buyer}); err == nil {
			t.Fatalf("invalid scope accepted: %+v", candidate)
		}
	}
	tree, err := Build(scope, []string{buyer})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tree.Proof("0x2222222222222222222222222222222222222222"); err == nil {
		t.Fatal("non-member proof accepted")
	}
	if proof, err := tree.Proof(buyer); err != nil || len(proof) != 0 {
		t.Fatalf("one member must have empty proof: %v err=%v", proof, err)
	}
	if _, err := (*Tree)(nil).Proof(buyer); err == nil {
		t.Fatal("nil tree accepted")
	}
	copyOfMembers := tree.Members()
	copyOfMembers[0].Wallet = common.Address{}
	if _, err := tree.Proof(buyer); err != nil {
		t.Fatalf("mutation of returned member snapshot affected tree: %v", err)
	}
	oversized := make([]string, MaxWallets+1)
	if _, err := Build(scope, oversized); err == nil {
		t.Fatal("oversized whitelist accepted")
	}
	checksummed := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678").Hex()
	if _, err := Build(scope, []string{checksummed}); err != nil {
		t.Fatalf("valid EIP-55 wallet rejected: %v", err)
	}
	var broken []byte = []byte(checksummed)
	for i := 2; i < len(broken); i++ {
		if broken[i] >= 'a' && broken[i] <= 'f' {
			broken[i] -= 'a' - 'A'
			break
		}
		if broken[i] >= 'A' && broken[i] <= 'F' {
			broken[i] += 'a' - 'A'
			break
		}
	}
	if _, err := Build(scope, []string{string(broken)}); err == nil {
		t.Fatal("invalid mixed-case EIP-55 wallet accepted")
	}
}

func TestLeafUsesABIPaddingAndProjectProxyDomain(t *testing.T) {
	scope := Scope{ChainID: big.NewInt(56), ProxyAddress: common.HexToAddress("0x1234567890123456789012345678901234567890"), ProjectID: big.NewInt(41376991395446785)}
	wallet := common.HexToAddress("0x1111111111111111111111111111111111111111")
	got, err := Leaf(scope, wallet)
	if err != nil {
		t.Fatal(err)
	}
	packed := append(append(append(append([]byte{}, leafSchema[:]...), scope.ChainID.Bytes()...), scope.ProxyAddress[:]...), scope.ProjectID.Bytes()...)
	packed = append(packed, wallet[:]...)
	if got == crypto.Keccak256Hash(packed) {
		t.Fatal("ABI leaf unexpectedly equals packed leaf")
	}
	other := scope
	other.ProxyAddress = common.HexToAddress("0x2234567890123456789012345678901234567890")
	otherLeaf, err := Leaf(other, wallet)
	if err != nil || reflect.DeepEqual(got, otherLeaf) || bytes.Equal(got[:], otherLeaf[:]) {
		t.Fatalf("proxy scope not bound: %s %s err=%v", got.Hex(), otherLeaf.Hex(), err)
	}
}

func TestPolicyProofFailsClosedOnUnmatchedEffectiveIdentity(t *testing.T) {
	vectors := loadVectors(t)
	item := vectors.Cases[2] // odd promotion with 3 members
	scope := Scope{
		ChainID: parseDecimal(t, item.Scope.ChainID), ProjectID: parseDecimal(t, item.Scope.ProjectID),
		ProxyAddress: common.HexToAddress(item.Scope.ProxyAddress),
	}
	tree, err := Build(scope, item.Wallets)
	if err != nil {
		t.Fatal(err)
	}
	policy := Policy{Enabled: true, Root: tree.Root, Version: 7}
	version, proof, err := tree.ProofForPolicy(scope, policy, item.Proofs[1].Wallet)
	if err != nil || version != 7 || len(proof) != len(item.Proofs[1].Proof) {
		t.Fatalf("effective policy proof mismatch: version=%d proof=%v err=%v", version, proof, err)
	}
	// The policy's own version is returned, not inferred from the root. The
	// execution-time Solidity check independently rejects a stale version.
	policy.Version = 8
	version, _, err = tree.ProofForPolicy(scope, policy, item.Proofs[1].Wallet)
	if err != nil || version != 8 {
		t.Fatalf("policy version did not follow on-chain observation: version=%d err=%v", version, err)
	}
	invalidPolicies := []Policy{
		{Enabled: false, Root: tree.Root, Version: 7},
		{Enabled: true, Root: common.Hash{}, Version: 7},
		{Enabled: true, Root: tree.Root, Version: 0},
		{Enabled: true, Root: common.HexToHash("0x01"), Version: 7},
	}
	for _, candidate := range invalidPolicies {
		if _, _, err := tree.ProofForPolicy(scope, candidate, item.Wallets[0]); err == nil {
			t.Fatalf("invalid effective policy accepted: %+v", candidate)
		}
	}
	other := scope
	other.ProjectID = new(big.Int).Add(scope.ProjectID, big.NewInt(1))
	if _, _, err := tree.ProofForPolicy(other, policy, item.Wallets[0]); err == nil {
		t.Fatal("cross-project policy accepted")
	}
	other = scope
	other.ChainID = new(big.Int).Add(scope.ChainID, big.NewInt(1))
	if _, _, err := tree.ProofForPolicy(other, policy, item.Wallets[0]); err == nil {
		t.Fatal("cross-chain policy accepted")
	}
	other = scope
	other.ProxyAddress = common.HexToAddress("0x2234567890123456789012345678901234567890")
	if _, _, err := tree.ProofForPolicy(other, policy, item.Wallets[0]); err == nil {
		t.Fatal("cross-proxy policy accepted")
	}
	if _, _, err := tree.ProofForPolicy(scope, policy, "0x0000000000000000000000000000000000000001"); err == nil {
		t.Fatal("nonmember policy proof accepted")
	}
	if _, _, err := (*Tree)(nil).ProofForPolicy(scope, policy, item.Wallets[0]); err == nil {
		t.Fatal("nil tree policy proof accepted")
	}
	// BigInt caller mutation cannot change the stored normalized tree scope.
	scope.ChainID.Add(scope.ChainID, big.NewInt(1))
	if _, _, err := tree.ProofForPolicy(scope, policy, item.Wallets[0]); err == nil {
		t.Fatal("caller-mutated chain scope accepted")
	}
}

package idov3whitelist

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestV3PolicyCallBindsProjectVersionAndSafeOnlyPayload(t *testing.T) {
	scope := Scope{ChainID: big.NewInt(31337), ProjectID: big.NewInt(93),
		ProxyAddress: common.HexToAddress("0x9393939393939393939393939393939393939393")}
	tree, err := Build(scope, []string{"0x1111111111111111111111111111111111111111"})
	if err != nil {
		t.Fatal(err)
	}
	change, err := PreviewPolicyChange(scope, ProjectPending, Policy{}, tree)
	if err != nil {
		t.Fatal(err)
	}
	call, err := PackPolicyChangeCall(scope, change)
	if err != nil || len(call) != 132 ||
		!bytes.Equal(call[:4], crypto.Keccak256([]byte("setWhitelistPolicy(uint256,uint64,bool,bytes32)"))[:4]) ||
		new(big.Int).SetBytes(call[4:36]).Cmp(scope.ProjectID) != 0 ||
		new(big.Int).SetBytes(call[36:68]).Uint64() != 0 || call[99] != 1 ||
		!bytes.Equal(call[100:132], change.Root[:]) {
		t.Fatal("V3 policy Safe simulation calldata differed from exact scope and version")
	}
	wrong := change
	wrong.TargetVersion++
	if _, err := PackPolicyChangeCall(scope, wrong); err == nil {
		t.Fatal("stale version accepted")
	}
	wrong = change
	wrong.Root = common.HexToHash("0x01")
	if _, err := PackPolicyChangeCall(scope, wrong); err == nil {
		t.Fatal("unreviewed root accepted")
	}
	if _, err := PackPolicyChangeCall(Scope{ProjectID: big.NewInt(93)}, change); err == nil {
		t.Fatal("versionless V3 scope accepted")
	}
}

package chain

import (
	"bytes"
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestV3LifecycleCalldataUsesIndependentABI(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	pay := common.HexToAddress("0x2222222222222222222222222222222222222222")
	sale := common.HexToAddress("0x3333333333333333333333333333333333333333")
	params := V3CreateParams{ProjectID: big.NewInt(7), ProjectToken: sale, PaymentToken: pay, ProjectOwner: proxy, FeeReceiver: proxy, FeeRate: 200,
		Price: big.NewInt(20), HardCap: big.NewInt(1000), SoftCap: big.NewInt(500), MaxPerUser: big.NewInt(900), Start: big.NewInt(100), End: big.NewInt(200),
		VestingTimes: []*big.Int{big.NewInt(300)}, VestingPercents: []*big.Int{big.NewInt(10000)}}
	got, err := PackV3Create(params)
	if err != nil || len(got) < 4 {
		t.Fatalf("valid V3 create rejected: %v", err)
	}
	independent, err := idoV3LifecycleABI.Pack("createProject", params.ProjectID, sale, pay, proxy, proxy, params.FeeRate, params.Price, params.HardCap, params.SoftCap, params.MaxPerUser, params.Start, params.End, params.VestingTimes, params.VestingPercents)
	if err != nil || !bytes.Equal(got, independent) {
		t.Fatal("V3 create ABI drift")
	}
	for _, method := range []string{"activateProject", "cancelProject", "endProject", "withdrawRaisedFunds", "withdrawPlatformFee", "withdrawProjectTokens"} {
		packed, err := PackV3Lifecycle(method, big.NewInt(7))
		if err != nil {
			t.Fatalf("V3 %s rejected: %v", method, err)
		}
		want, err := idoV3LifecycleABI.Pack(method, big.NewInt(7))
		if err != nil || !bytes.Equal(packed, want) {
			t.Fatalf("V3 %s selector drift", method)
		}
	}
	if _, err := PackV3Lifecycle("setWhitelistPolicy", big.NewInt(7)); err == nil {
		t.Fatal("Safe governance misrouted through relayer lifecycle")
	}
	if _, err := PackV3Lifecycle("activateProject", big.NewInt(0)); err == nil {
		t.Fatal("zero project accepted")
	}
	deposit, err := PackV3ProjectDeposit(big.NewInt(7), big.NewInt(900))
	if err != nil || len(deposit) != 68 || !bytes.Equal(deposit[:4], crypto.Keccak256([]byte("depositProjectTokens(uint256,uint256)"))[:4]) || deposit[35] != 7 || deposit[67] != 0x84 {
		t.Fatal("V3 owner deposit ABI drift")
	}
	for _, bad := range [][2]*big.Int{{nil, big.NewInt(1)}, {big.NewInt(7), nil}, {big.NewInt(0), big.NewInt(1)}, {big.NewInt(7), big.NewInt(-1)}, {big.NewInt(7), new(big.Int).Lsh(big.NewInt(1), 256)}} {
		if _, err := PackV3ProjectDeposit(bad[0], bad[1]); err == nil {
			t.Fatal("invalid project-owner deposit packed")
		}
	}
	if _, err := PackV3Lifecycle("depositProjectTokens", big.NewInt(7)); err == nil {
		t.Fatal("project-owner deposit routed to relayer lifecycle")
	}
	invalid := params
	invalid.VestingPercents = []*big.Int{big.NewInt(9999)}
	if _, err := PackV3Create(invalid); err == nil {
		t.Fatal("incomplete vesting accepted")
	}
	invalid = params
	invalid.PaymentToken = sale
	if _, err := PackV3Create(invalid); err == nil {
		t.Fatal("same payment and sale token accepted")
	}
}

func TestV3LifecycleCalldataMatchesSuppliedCompiledArtifact(t *testing.T) {
	path := os.Getenv("JOINCARE_IDO_V3_ARTIFACT")
	if path == "" {
		t.Skip("compiled V3 artifact not supplied")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("compiled V3 artifact unavailable")
	}
	var artifact struct {
		ContractName string          `json:"contractName"`
		ABI          json.RawMessage `json:"abi"`
	}
	if json.Unmarshal(raw, &artifact) != nil || artifact.ContractName != "JoinCareIDOv3" || len(artifact.ABI) == 0 {
		t.Fatal("wrong V3 artifact")
	}
	var entries []struct {
		Type, Name string
		Inputs     []struct{ Type string }
	}
	if json.Unmarshal(artifact.ABI, &entries) != nil {
		t.Fatal("malformed V3 artifact ABI")
	}
	for _, method := range []string{"createProject", "activateProject", "depositProjectTokens", "cancelProject", "endProject", "withdrawRaisedFunds", "withdrawPlatformFee", "withdrawProjectTokens"} {
		pack, ok := idoV3LifecycleABI.Methods[method]
		if !ok {
			t.Fatalf("missing local V3 method %s", method)
		}
		found := 0
		for _, entry := range entries {
			if entry.Type != "function" || entry.Name != method {
				continue
			}
			found++
			if len(entry.Inputs) != len(pack.Inputs) {
				t.Fatalf("V3 %s ABI arity drift", method)
			}
			for i := range entry.Inputs {
				if entry.Inputs[i].Type != pack.Inputs[i].Type.String() {
					t.Fatalf("V3 %s argument %d ABI drift", method, i)
				}
			}
		}
		if found != 1 {
			t.Fatalf("V3 %s compiled ABI missing or ambiguous", method)
		}
	}
}

package idov3whitelist

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type projectRPCStub struct {
	*stubPolicyRPC
	projectTuple []byte
	reads        int
	changed      bool
	failure      bool
}

// Use an independent ABI encoder rather than the hand-assembled words in the
// negative tests. The tuple signature mirrors JoinCareIDOv3.getProject in the
// contract artifact, including the outer dynamic tuple offset.
func TestReadProjectIdentityAtAcceptsABIEncodedV3Project(t *testing.T) {
	const signature = `[{"type":"function","name":"getProject","inputs":[{"name":"projectId","type":"uint256"}],"outputs":[{"type":"tuple","components":[{"name":"projectId","type":"uint256"},{"name":"projectToken","type":"address"},{"name":"paymentToken","type":"address"},{"name":"projectOwner","type":"address"},{"name":"feeReceiver","type":"address"},{"name":"projectTokenDecimals","type":"uint8"},{"name":"paymentTokenDecimals","type":"uint8"},{"name":"successFeeRate","type":"uint16"},{"name":"status","type":"uint8"},{"name":"tokenPrice","type":"uint256"},{"name":"hardCap","type":"uint256"},{"name":"softCap","type":"uint256"},{"name":"maxPerUser","type":"uint256"},{"name":"startTime","type":"uint256"},{"name":"endTime","type":"uint256"},{"name":"totalRaised","type":"uint256"},{"name":"totalTokensSold","type":"uint256"},{"name":"totalTokensDeposited","type":"uint256"},{"name":"fundsWithdrawn","type":"bool"},{"name":"platformFeeWithdrawn","type":"bool"},{"name":"projectTokensWithdrawn","type":"bool"},{"name":"vestingTimes","type":"uint256[]"},{"name":"vestingPercents","type":"uint256[]"}]}]}]`
	parsed, err := abi.JSON(strings.NewReader(signature))
	if err != nil {
		t.Fatal(err)
	}
	scope := Scope{ChainID: big.NewInt(31337), ProjectID: big.NewInt(41376991395446785), ProxyAddress: common.HexToAddress("0x1234567890123456789012345678901234567890")}
	impl := common.HexToAddress("0x2234567890123456789012345678901234567890")
	sale := common.HexToAddress("0x3234567890123456789012345678901234567890")
	payment := common.HexToAddress("0x4234567890123456789012345678901234567890")
	project := struct {
		ProjectId              *big.Int
		ProjectToken           common.Address
		PaymentToken           common.Address
		ProjectOwner           common.Address
		FeeReceiver            common.Address
		ProjectTokenDecimals   uint8
		PaymentTokenDecimals   uint8
		SuccessFeeRate         uint16
		Status                 uint8
		TokenPrice             *big.Int
		HardCap                *big.Int
		SoftCap                *big.Int
		MaxPerUser             *big.Int
		StartTime              *big.Int
		EndTime                *big.Int
		TotalRaised            *big.Int
		TotalTokensSold        *big.Int
		TotalTokensDeposited   *big.Int
		FundsWithdrawn         bool
		PlatformFeeWithdrawn   bool
		ProjectTokensWithdrawn bool
		VestingTimes           []*big.Int
		VestingPercents        []*big.Int
	}{scope.ProjectID, sale, payment, sale, payment, 18, 6, 200, uint8(ProjectActive),
		big.NewInt(2), big.NewInt(1000), big.NewInt(500), big.NewInt(600), big.NewInt(100), big.NewInt(200),
		big.NewInt(100), big.NewInt(10), big.NewInt(1000), false, false, false,
		[]*big.Int{big.NewInt(300), big.NewInt(400)}, []*big.Int{big.NewInt(2500), big.NewInt(7500)}}
	encoded, err := parsed.Methods["getProject"].Outputs.Pack(project)
	if err != nil {
		t.Fatal(err)
	}
	proxyCode, implCode := []byte{0x60, 0x80}, []byte{0x60, 0x01}
	runtime := RuntimeIdentity{Implementation: impl, ProxyCodeHash: crypto.Keccak256Hash(proxyCode), CodeHash: crypto.Keccak256Hash(implCode)}
	base := &stubPolicyRPC{chainID: scope.ChainID, proxy: scope.ProxyAddress, project: scope.ProjectID,
		implementation: impl, implementationCode: implCode, slot: common.LeftPadBytes(impl.Bytes(), 32),
		header: &types.Header{Number: big.NewInt(90)}, code: proxyCode, version: v3ContractVersion[:], policy: make([]byte, 96)}
	stub := &projectRPCStub{stubPolicyRPC: base, projectTuple: encoded}
	got, err := ReadProjectIdentityAt(context.Background(), stub, scope, runtime, 90, base.header.Hash())
	if err != nil || got.ProjectID != scope.ProjectID.Uint64() || got.ProjectToken != sale || got.PaymentToken != payment || got.State != ProjectActive ||
		got.ProjectOwner != sale || got.FeeReceiver != payment || got.ProjectDecimals != 18 || got.PaymentDecimals != 6 || got.FeeRate != 200 ||
		got.TokenPriceRaw != "2" || got.HardCapRaw != "1000" || got.SoftCapRaw != "500" || got.MaxPerUserRaw != "600" || got.StartTimeRaw != "100" || got.EndTimeRaw != "200" {
		t.Fatalf("independently ABI-encoded V3 project rejected: %+v %v", got, err)
	}
}

func (s *projectRPCStub) CallContract(ctx context.Context, call ethereum.CallMsg, block *big.Int) ([]byte, error) {
	if bytes.Equal(call.Data[:4], v3ProjectSelector) {
		if s.failure {
			return nil, errors.New("sensitive RPC project response")
		}
		if call.To == nil || *call.To != s.proxy || block.Cmp(s.header.Number) != 0 || len(call.Data) != 36 ||
			new(big.Int).SetBytes(call.Data[4:]).Cmp(s.project) != 0 {
			return nil, errors.New("wrong project selector")
		}
		return s.projectTuple, nil
	}
	return s.stubPolicyRPC.CallContract(ctx, call, block)
}

func (s *projectRPCStub) HeaderByNumber(ctx context.Context, block *big.Int) (*types.Header, error) {
	s.reads++
	if s.changed && s.reads == 3 {
		return &types.Header{Number: block, Extra: []byte("replacement")}, nil
	}
	return s.stubPolicyRPC.HeaderByNumber(ctx, block)
}

func TestReadProjectIdentityAtRequiresExactTupleAndPinnedRuntime(t *testing.T) {
	scope := Scope{ChainID: big.NewInt(31337), ProjectID: big.NewInt(41376991395446785), ProxyAddress: common.HexToAddress("0x1234567890123456789012345678901234567890")}
	impl := common.HexToAddress("0x2234567890123456789012345678901234567890")
	sale := common.HexToAddress("0x3234567890123456789012345678901234567890")
	payment := common.HexToAddress("0x4234567890123456789012345678901234567890")
	proxyCode, implCode := []byte{0x60, 0x80}, []byte{0x60, 0x01}
	runtime := RuntimeIdentity{Implementation: impl, ProxyCodeHash: crypto.Keccak256Hash(proxyCode), CodeHash: crypto.Keccak256Hash(implCode)}
	word := func(v uint64) []byte { return common.LeftPadBytes(new(big.Int).SetUint64(v).Bytes(), 32) }
	tuple := make([]byte, 32+v3ProjectWords*32+2*64)
	copy(tuple[:32], word(32))
	field := func(i int) []byte { return tuple[32+i*32 : 32+(i+1)*32] }
	copy(field(0), word(scope.ProjectID.Uint64()))
	for index, address := range map[int]common.Address{1: sale, 2: payment, 3: sale, 4: payment} {
		copy(field(index), common.LeftPadBytes(address.Bytes(), 32))
	}
	copy(field(5), word(18))
	copy(field(6), word(6))
	copy(field(7), word(200))
	copy(field(8), word(uint64(ProjectActive)))
	for index, value := range map[int]uint64{9: 2, 10: 1000, 11: 500, 12: 600, 13: 100, 14: 200, 15: 100, 16: 10, 17: 1000} {
		copy(field(index), word(value))
	}
	copy(field(21), word(v3ProjectWords*32))
	copy(field(22), word(v3ProjectWords*32+64))
	copy(tuple[32+v3ProjectWords*32:], word(1))
	copy(tuple[32+v3ProjectWords*32+32:], word(300))
	copy(tuple[32+v3ProjectWords*32+64:], word(1))
	copy(tuple[32+v3ProjectWords*32+96:], word(10000))
	base := &stubPolicyRPC{chainID: scope.ChainID, proxy: scope.ProxyAddress, project: scope.ProjectID,
		implementation: impl, implementationCode: implCode, slot: common.LeftPadBytes(impl.Bytes(), 32),
		header: &types.Header{Number: big.NewInt(90)}, code: proxyCode, version: v3ContractVersion[:], policy: make([]byte, 96)}
	stub := &projectRPCStub{stubPolicyRPC: base, projectTuple: tuple}
	hash := base.header.Hash()
	wantState := ProjectActive
	read := func() error {
		stub.reads, base.reads = 0, 0
		got, err := ReadProjectIdentityAt(context.Background(), stub, scope, runtime, 90, hash)
		if err == nil && (got.ChainID != 31337 || got.ProxyAddress != scope.ProxyAddress || got.BlockHash != hash || got.ProjectID != scope.ProjectID.Uint64() || got.State != wantState || got.ProjectToken != sale || got.PaymentToken != payment) {
			t.Fatalf("wrong V3 project observation: %+v", got)
		}
		return err
	}
	if err := read(); err != nil {
		t.Fatalf("valid project read rejected: %v", err)
	}
	for _, state := range []ProjectState{ProjectPending, ProjectSuccessful, ProjectFailed, ProjectCancelled} {
		stub.projectTuple = append([]byte(nil), tuple...)
		stub.projectTuple[32+8*32+31] = uint8(state)
		wantState = state
		switch state {
		case ProjectPending:
			clear(stub.projectTuple[32+15*32 : 32+17*32])
		case ProjectSuccessful:
			copy(stub.projectTuple[32+15*32:32+16*32], word(500))
		}
		if err := read(); err != nil {
			t.Fatalf("valid state %d rejected: %v", state, err)
		}
	}
	wantState = ProjectActive
	// Every strict prefix of a valid dynamic tuple must fail as a normal
	// decoding error, never panic even if it retains a plausible array length.
	for length := 0; length < len(tuple); length++ {
		stub.projectTuple = tuple[:length]
		if err := read(); err == nil {
			t.Fatalf("truncated V3 project tuple accepted at %d bytes", length)
		}
	}
	for name, change := range map[string]func([]byte){
		"wrong project":      func(b []byte) { b[63]++ },
		"zero token":         func(b []byte) { clear(b[32+32 : 32+64]) },
		"dirty address":      func(b []byte) { b[32+32] = 1 },
		"invalid status":     func(b []byte) { b[32+8*32+31] = 5 },
		"dirty status":       func(b []byte) { b[32+8*32] = 1 },
		"dirty boolean":      func(b []byte) { b[32+18*32+31] = 2 },
		"zero price":         func(b []byte) { clear(b[32+9*32 : 32+10*32]) },
		"soft above hard":    func(b []byte) { b[32+11*32+30] = 4 },
		"raised above cap":   func(b []byte) { b[32+15*32+30] = 4 },
		"sold over deposit":  func(b []byte) { b[32+16*32+30] = 4 },
		"pending raised":     func(b []byte) { b[32+8*32+31] = uint8(ProjectPending) },
		"success below soft": func(b []byte) { b[32+8*32+31] = uint8(ProjectSuccessful) },
		"failed above soft":  func(b []byte) { b[32+8*32+31] = uint8(ProjectFailed); b[32+15*32+30] = 2 },
		"vesting too early":  func(b []byte) { b[32+v3ProjectWords*32+32+30], b[32+v3ProjectWords*32+32+31] = 0, 100 },
		"wrong percent sum":  func(b []byte) { b[32+v3ProjectWords*32+96+31] = 1 },
		"bad array offset":   func(b []byte) { b[32+21*32+31]++ },
		"bad array length":   func(b []byte) { b[32+v3ProjectWords*32+31] = 11 },
		"truncated first array": func(b []byte) {
			b[32+v3ProjectWords*32+31] = 10
			stub.projectTuple = b
		},
		"truncated second array": func(b []byte) {
			stub.projectTuple = b[:len(b)-32]
		},
		"unequal schedules":  func(b []byte) { b[32+v3ProjectWords*32+64+31] = 2 },
		"dirty tuple prefix": func(b []byte) { b[0] = 1 },
		"extra bytes":        func(b []byte) { b = append(b, 0); stub.projectTuple = b },
	} {
		t.Run(name, func(t *testing.T) {
			stub.projectTuple = append([]byte(nil), tuple...)
			change(stub.projectTuple)
			if err := read(); err == nil {
				t.Fatal("malformed project admitted")
			}
		})
	}
	stub.projectTuple = tuple
	stub.changed = true
	if err := read(); err == nil {
		t.Fatal("mid-read reorg accepted")
	}
	stub.changed = false
	stub.failure = true
	if err := read(); err == nil || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("raw RPC error leaked or admitted: %v", err)
	}
}

package chain

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestPreviewV3LifecycleCallStatesRolesAndBinding(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7,
		PaymentToken: common.HexToAddress("0x2222222222222222222222222222222222222222"),
		ProjectToken: common.HexToAddress("0x3333333333333333333333333333333333333333")}
	base := idov3whitelist.ProjectIdentityObservation{ChainID: binding.ChainID, ProjectID: binding.ProjectID,
		ProxyAddress: proxy, PaymentToken: binding.PaymentToken, ProjectToken: binding.ProjectToken,
		ProjectOwner: common.HexToAddress("0x4444444444444444444444444444444444444444"),
		FeeReceiver:  common.HexToAddress("0x5555555555555555555555555555555555555555"),
		BlockNumber:  90, BlockHash: common.HexToHash("0xbeef")}
	callerIdentity := V3CallerIdentity{Operator: common.HexToAddress("0x6666666666666666666666666666666666666666"), OwnerSafe: common.HexToAddress("0x7777777777777777777777777777777777777777")}
	for _, test := range []struct {
		name   string
		state  idov3whitelist.ProjectState
		method string
		role   V3LifecycleRole
		accept bool
	}{
		{"operator activate", idov3whitelist.ProjectPending, "activateProject", V3Operator, true},
		{"owner safe activate", idov3whitelist.ProjectPending, "activateProject", V3OwnerSafe, true},
		{"project owner activate", idov3whitelist.ProjectPending, "activateProject", V3ProjectOwner, false},
		{"active activate", idov3whitelist.ProjectActive, "activateProject", V3Operator, false},
		{"pending project owner cancel", idov3whitelist.ProjectPending, "cancelProject", V3ProjectOwner, true},
		{"active project owner cancel", idov3whitelist.ProjectActive, "cancelProject", V3ProjectOwner, false},
		{"active safe cancel", idov3whitelist.ProjectActive, "cancelProject", V3OwnerSafe, true},
		{"public end", idov3whitelist.ProjectActive, "endProject", V3PublicCaller, true},
		{"owner safe end", idov3whitelist.ProjectActive, "endProject", V3OwnerSafe, true},
		{"unknown role end", idov3whitelist.ProjectActive, "endProject", V3LifecycleRole(255), false},
		{"premature end", idov3whitelist.ProjectPending, "endProject", V3PublicCaller, false},
		{"successful project owner proceeds", idov3whitelist.ProjectSuccessful, "withdrawRaisedFunds", V3ProjectOwner, true},
		{"safe proceeds", idov3whitelist.ProjectSuccessful, "withdrawRaisedFunds", V3OwnerSafe, false},
		{"failed proceeds", idov3whitelist.ProjectFailed, "withdrawRaisedFunds", V3ProjectOwner, false},
		{"public platform fee", idov3whitelist.ProjectSuccessful, "withdrawPlatformFee", V3PublicCaller, true},
		{"operator platform fee", idov3whitelist.ProjectSuccessful, "withdrawPlatformFee", V3Operator, true},
		{"unknown role platform fee", idov3whitelist.ProjectSuccessful, "withdrawPlatformFee", V3LifecycleRole(255), false},
		{"pending platform fee", idov3whitelist.ProjectPending, "withdrawPlatformFee", V3PublicCaller, false},
		{"failed token return", idov3whitelist.ProjectFailed, "withdrawProjectTokens", V3ProjectOwner, true},
		{"cancelled token return", idov3whitelist.ProjectCancelled, "withdrawProjectTokens", V3ProjectOwner, true},
		{"successful token return", idov3whitelist.ProjectSuccessful, "withdrawProjectTokens", V3ProjectOwner, true},
		{"active token return", idov3whitelist.ProjectActive, "withdrawProjectTokens", V3ProjectOwner, false},
		{"unknown method", idov3whitelist.ProjectActive, "purchase", V3PublicCaller, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			observed := base
			observed.State = test.state
			caller := callerIdentity.Operator
			if test.role == V3ProjectOwner {
				caller = observed.ProjectOwner
			}
			if test.role == V3OwnerSafe {
				caller = callerIdentity.OwnerSafe
			}
			data, err := PreviewV3LifecycleCall(binding, observed, test.method, caller, test.role, callerIdentity)
			if (err == nil) != test.accept {
				t.Fatalf("accept=%v err=%v", test.accept, err)
			}
			if err == nil && (!bytes.Equal(data[:4], crypto.Keccak256([]byte(test.method + "(uint256)"))[:4]) || data[35] != 7) {
				t.Fatal("V3 selector or bound project ID changed")
			}
		})
	}
	for _, mutation := range []func(*idov3whitelist.ProjectIdentityObservation){
		func(v *idov3whitelist.ProjectIdentityObservation) {
			v.ProxyAddress = common.HexToAddress("0x6666666666666666666666666666666666666666")
		},
		func(v *idov3whitelist.ProjectIdentityObservation) { v.PaymentToken = v.ProjectToken },
		func(v *idov3whitelist.ProjectIdentityObservation) { v.ProjectID++ },
		func(v *idov3whitelist.ProjectIdentityObservation) { v.BlockHash = common.Hash{} },
		func(v *idov3whitelist.ProjectIdentityObservation) { v.ProjectOwner = common.Address{} },
	} {
		observed := base
		mutation(&observed)
		if _, err := PreviewV3LifecycleCall(binding, observed, "activateProject", callerIdentity.Operator, V3Operator, callerIdentity); err == nil {
			t.Fatal("mismatched project observation accepted")
		}
	}
	for _, method := range []string{"cancelProject", "withdrawRaisedFunds", "withdrawProjectTokens"} {
		observed := base
		observed.State = idov3whitelist.ProjectSuccessful
		if method == "cancelProject" {
			observed.State = idov3whitelist.ProjectPending
		}
		for _, caller := range []common.Address{common.Address{}, binding.ProxyAddress, base.FeeReceiver} {
			if _, err := PreviewV3LifecycleCall(binding, observed, method, caller, V3ProjectOwner, callerIdentity); err == nil {
				t.Fatalf("spoofed project-owner role allowed %s", method)
			}
		}
	}
	for _, role := range []V3LifecycleRole{V3Operator, V3OwnerSafe} {
		if _, err := PreviewV3LifecycleCall(binding, base, "activateProject", base.ProjectOwner, role, callerIdentity); err == nil {
			t.Fatal("role label spoofed privileged caller")
		}
	}
	if _, err := PreviewV3LifecycleCall(binding, base, "activateProject", callerIdentity.OwnerSafe, V3OwnerSafe, V3CallerIdentity{}); err == nil {
		t.Fatal("missing independently expected safe identity accepted")
	}
	if _, err := PreviewV3LifecycleCall(binding, base, "activateProject", callerIdentity.Operator, V3Operator,
		V3CallerIdentity{Operator: callerIdentity.Operator, OwnerSafe: callerIdentity.Operator}); err == nil {
		t.Fatal("overlapping operator and owner safe accepted")
	}
	if _, err := PreviewV3LifecycleCall(binding, base, "activateProject", callerIdentity.Operator, V3Operator,
		V3CallerIdentity{Operator: callerIdentity.Operator}); err == nil {
		t.Fatal("operator-only expectation accepted without owner safe identity")
	}
}

func TestPreviewV3CreateCallRejectsVersionlessOrForeignIdentity(t *testing.T) {
	addr := func(hex string) common.Address { return common.HexToAddress(hex) }
	proxy := addr("0x1111111111111111111111111111111111111111")
	sale := addr("0x3333333333333333333333333333333333333333")
	payment := addr("0x2222222222222222222222222222222222222222")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7, PaymentToken: payment, ProjectToken: sale}
	identity := V3CallerIdentity{Operator: addr("0x6666666666666666666666666666666666"), OwnerSafe: addr("0x7777777777777777777777777777777777777777")}
	params := V3CreateParams{ProjectID: big.NewInt(7), ProjectToken: sale, PaymentToken: payment,
		ProjectOwner: addr("0x4444444444444444444444444444444444444444"), FeeReceiver: addr("0x5555555555555555555555555555555555555555"),
		FeeRate: 200, Price: big.NewInt(20), HardCap: big.NewInt(1000), SoftCap: big.NewInt(500), MaxPerUser: big.NewInt(900),
		Start: big.NewInt(100), End: big.NewInt(200), VestingTimes: []*big.Int{big.NewInt(300)}, VestingPercents: []*big.Int{big.NewInt(10000)}}
	for _, role := range []V3LifecycleRole{V3Operator, V3OwnerSafe} {
		caller := identity.Operator
		if role == V3OwnerSafe {
			caller = identity.OwnerSafe
		}
		data, err := PreviewV3CreateCall(binding, params, caller, role, identity)
		if err != nil || !bytes.Equal(data[:4], crypto.Keccak256([]byte("createProject(uint256,address,address,address,address,uint16,uint256,uint256,uint256,uint256,uint256,uint256,uint256[],uint256[])"))[:4]) {
			t.Fatalf("valid V3 create preview rejected: %v", err)
		}
	}
	if _, err := PreviewV3CreateCall(binding, params, params.ProjectOwner, V3ProjectOwner, identity); err == nil {
		t.Fatal("project owner not authorized to create")
	}
	if _, err := PreviewV3CreateCall(binding, params, identity.Operator, V3PublicCaller, identity); err == nil {
		t.Fatal("public caller not authorized to create")
	}
	bad := binding
	bad.ProjectID++
	if _, err := PreviewV3CreateCall(bad, params, identity.Operator, V3Operator, identity); err == nil {
		t.Fatal("foreign project ID accepted")
	}
	bad = binding
	bad.ProxyAddress = sale
	if _, err := PreviewV3CreateCall(bad, params, identity.OwnerSafe, V3OwnerSafe, identity); err == nil {
		t.Fatal("project token proxy collision accepted")
	}
	bad = binding
	bad.ChainID = 0
	if _, err := PreviewV3CreateCall(bad, params, identity.OwnerSafe, V3OwnerSafe, identity); err == nil {
		t.Fatal("missing chain identity accepted")
	}
	bad = binding
	bad.PaymentToken = addr("0x6666666666666666666666666666666666666666")
	if _, err := PreviewV3CreateCall(bad, params, identity.OwnerSafe, V3OwnerSafe, identity); err == nil {
		t.Fatal("unbound payment token accepted")
	}
	params.ProjectID = new(big.Int).Lsh(big.NewInt(1), 65)
	if _, err := PreviewV3CreateCall(binding, params, identity.Operator, V3Operator, identity); err == nil {
		t.Fatal("out-of-range project ID accepted")
	}
	params.ProjectID = big.NewInt(7)
	if _, err := PreviewV3CreateCall(binding, params, identity.OwnerSafe, V3Operator, identity); err == nil {
		t.Fatal("Safe caller accepted by relayer role label")
	}
	if _, err := PreviewV3CreateCall(binding, params, identity.Operator, V3Operator, V3CallerIdentity{}); err == nil {
		t.Fatal("missing independently expected operator accepted")
	}
}

func TestPreviewV3ProjectDepositOnlyPendingOwnerWallet(t *testing.T) {
	addr := func(last byte) common.Address { return common.BytesToAddress([]byte{last}) }
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 7, ProxyAddress: addr(1), PaymentToken: addr(2), ProjectToken: addr(3)}
	observed := idov3whitelist.ProjectIdentityObservation{ChainID: binding.ChainID, ProjectID: binding.ProjectID,
		ProxyAddress: binding.ProxyAddress, PaymentToken: binding.PaymentToken, ProjectToken: binding.ProjectToken,
		ProjectOwner: addr(4), State: idov3whitelist.ProjectPending, BlockNumber: 100, BlockHash: common.HexToHash("0xdead")}
	call, err := PreviewV3ProjectDepositCall(binding, observed, observed.ProjectOwner, big.NewInt(500), V3ProjectOwner)
	if err != nil || len(call) != 68 {
		t.Fatal("valid project-owner deposit preview rejected")
	}
	for _, role := range []V3LifecycleRole{V3Operator, V3OwnerSafe, V3PublicCaller} {
		if _, err := PreviewV3ProjectDepositCall(binding, observed, observed.ProjectOwner, big.NewInt(500), role); err == nil {
			t.Fatal("non-owner deposit role accepted")
		}
	}
	for _, caller := range []common.Address{common.Address{}, binding.ProxyAddress, addr(9)} {
		if _, err := PreviewV3ProjectDepositCall(binding, observed, caller, big.NewInt(500), V3ProjectOwner); err == nil {
			t.Fatal("non-owner caller accepted with project-owner role label")
		}
	}
	for _, change := range []func(*idov3whitelist.ProjectIdentityObservation){
		func(v *idov3whitelist.ProjectIdentityObservation) { v.State = idov3whitelist.ProjectActive },
		func(v *idov3whitelist.ProjectIdentityObservation) { v.ProjectOwner = common.Address{} },
		func(v *idov3whitelist.ProjectIdentityObservation) { v.ProjectOwner = binding.ProjectToken },
		func(v *idov3whitelist.ProjectIdentityObservation) { v.ProxyAddress = addr(8) },
		func(v *idov3whitelist.ProjectIdentityObservation) { v.BlockHash = common.Hash{} },
	} {
		bad := observed
		change(&bad)
		if _, err := PreviewV3ProjectDepositCall(binding, bad, observed.ProjectOwner, big.NewInt(500), V3ProjectOwner); err == nil {
			t.Fatal("invalid project-owner deposit observation accepted")
		}
	}
	if _, err := PreviewV3ProjectDepositCall(binding, observed, observed.ProjectOwner, big.NewInt(0), V3ProjectOwner); err == nil {
		t.Fatal("zero deposit accepted")
	}
}

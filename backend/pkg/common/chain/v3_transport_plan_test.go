package chain

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestV3TransportPreviewNeverRoutesSafeOrWalletThroughRelayer(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	pay := common.HexToAddress("0x2222222222222222222222222222222222222222")
	sale := common.HexToAddress("0x3333333333333333333333333333333333333333")
	owner := common.HexToAddress("0x4444444444444444444444444444444444444444")
	operator := common.HexToAddress("0x5555555555555555555555555555555555555555")
	safe := common.HexToAddress("0x6666666666666666666666666666666666666666")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 7, ProxyAddress: proxy, PaymentToken: pay, ProjectToken: sale}
	identities := V3CallerIdentity{Operator: operator, OwnerSafe: safe}
	observed := idov3whitelist.ProjectIdentityObservation{ChainID: binding.ChainID, ProjectID: binding.ProjectID,
		ProxyAddress: proxy, PaymentToken: pay, ProjectToken: sale, ProjectOwner: owner, FeeReceiver: safe,
		State: idov3whitelist.ProjectPending, BlockNumber: 90, BlockHash: common.HexToHash("0xbeef")}
	for _, tc := range []struct {
		name   string
		role   V3LifecycleRole
		caller common.Address
		method string
		want   V3Transport
	}{
		{"operator activation", V3Operator, operator, "activateProject", V3RelayerTransport},
		{"safe activation", V3OwnerSafe, safe, "activateProject", V3SafeTransport},
		{"owner cancellation", V3ProjectOwner, owner, "cancelProject", V3WalletTransport},
		{"safe cancellation", V3OwnerSafe, safe, "cancelProject", V3SafeTransport},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := PreviewV3LifecycleTransport(binding, observed, tc.method, tc.caller, tc.role, identities)
			if err != nil || plan.Transport != tc.want || plan.From != tc.caller || plan.To != proxy || len(plan.CallData) != 36 {
				t.Fatalf("misrouted V3 call: %+v %v", plan, err)
			}
		})
	}
	params := V3CreateParams{ProjectID: big.NewInt(7), ProjectToken: sale, PaymentToken: pay, ProjectOwner: owner, FeeReceiver: safe,
		FeeRate: 200, Price: big.NewInt(20), HardCap: big.NewInt(1000), SoftCap: big.NewInt(500), MaxPerUser: big.NewInt(900),
		Start: big.NewInt(100), End: big.NewInt(200), VestingTimes: []*big.Int{big.NewInt(300)}, VestingPercents: []*big.Int{big.NewInt(10000)}}
	for _, tc := range []struct {
		role   V3LifecycleRole
		caller common.Address
		want   V3Transport
	}{{V3Operator, operator, V3RelayerTransport}, {V3OwnerSafe, safe, V3SafeTransport}} {
		plan, err := PreviewV3CreateTransport(binding, params, tc.caller, tc.role, identities)
		if err != nil || plan.Transport != tc.want || plan.To != proxy || plan.From != tc.caller || len(plan.CallData) < 4 {
			t.Fatalf("create transport mismatch: %+v %v", plan, err)
		}
	}
	if _, err := PreviewV3CreateTransport(binding, params, owner, V3ProjectOwner, identities); err == nil {
		t.Fatal("owner created via relayer")
	}
	if _, err := PreviewV3LifecycleTransport(binding, observed, "activateProject", owner, V3Operator, identities); err == nil {
		t.Fatal("role-only operator admitted")
	}
	deposit, err := PreviewV3DepositTransport(binding, observed, owner, big.NewInt(10))
	if err != nil || deposit.Transport != V3WalletTransport || deposit.From != owner || deposit.To != proxy {
		t.Fatalf("wallet deposit misrouted: %+v %v", deposit, err)
	}
	if _, err := PreviewV3DepositTransport(binding, observed, safe, big.NewInt(10)); err == nil {
		t.Fatal("safe spoofed owner deposit")
	}
	observed.State = idov3whitelist.ProjectSuccessful
	if _, err := PreviewV3LifecycleTransport(binding, observed, "withdrawRaisedFunds", safe, V3OwnerSafe, identities); err == nil {
		t.Fatal("safe withdrew project-owner proceeds")
	}
	observed.State = idov3whitelist.ProjectActive
	if _, err := PreviewV3LifecycleTransport(binding, observed, "cancelProject", owner, V3ProjectOwner, identities); err == nil {
		t.Fatal("owner cancelled active project")
	}
	for _, method := range []string{"endProject", "withdrawPlatformFee"} {
		if method == "withdrawPlatformFee" {
			observed.State = idov3whitelist.ProjectSuccessful
		}
		plan, err := PreviewV3LifecycleTransport(binding, observed, method, pay, V3PublicCaller, identities)
		if err != nil || plan.Transport != V3WalletTransport || plan.To != proxy || plan.From != pay {
			t.Fatalf("public V3 lifecycle call misrouted: %s %+v %v", method, plan, err)
		}
	}
	plan, err := PreviewV3LifecycleTransport(binding, observed, "withdrawRaisedFunds", owner, V3ProjectOwner, identities)
	if err != nil || plan.Transport != V3WalletTransport || plan.To != proxy || plan.From != owner {
		t.Fatalf("owner withdrawal misrouted: %+v %v", plan, err)
	}
}

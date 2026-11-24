package chain

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"joincare_server/pkg/common/idov3whitelist"
)

func TestMatchV3ReplayProjectEndOfBlockFailsClosed(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	payment := common.HexToAddress("0x2222222222222222222222222222222222222222")
	sale := common.HexToAddress("0x3333333333333333333333333333333333333333")
	hash := common.HexToHash("0xa1")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProjectID: 7, ProxyAddress: proxy, PaymentToken: payment, ProjectToken: sale}
	totals := V3ReplayTotals{State: idov3whitelist.ProjectActive, Raised: big.NewInt(600), Sold: big.NewInt(300)}
	observed := idov3whitelist.ProjectIdentityObservation{ChainID: 31337, ProjectID: 7, ProxyAddress: proxy, PaymentToken: payment, ProjectToken: sale, BlockNumber: 90, BlockHash: hash, State: idov3whitelist.ProjectActive, RaisedAtBlockRaw: "600", TokensSoldAtBlockRaw: "300"}
	check := func(o idov3whitelist.ProjectIdentityObservation, h uint64, x common.Hash) error {
		return MatchV3ReplayProjectEndOfBlock(binding, totals, o, h, x)
	}
	if err := check(observed, 90, hash); err != nil {
		t.Fatalf("matching end-of-block rejected: %v", err)
	}
	for _, tc := range []struct {
		name  string
		alter func(*idov3whitelist.ProjectIdentityObservation)
	}{
		{"wrong proxy", func(o *idov3whitelist.ProjectIdentityObservation) { o.ProxyAddress = payment }},
		{"wrong chain", func(o *idov3whitelist.ProjectIdentityObservation) { o.ChainID = 56 }},
		{"wrong project", func(o *idov3whitelist.ProjectIdentityObservation) { o.ProjectID = 8 }},
		{"wrong token", func(o *idov3whitelist.ProjectIdentityObservation) { o.ProjectToken = payment }},
		{"wrong state", func(o *idov3whitelist.ProjectIdentityObservation) { o.State = idov3whitelist.ProjectSuccessful }},
		{"wrong block", func(o *idov3whitelist.ProjectIdentityObservation) { o.BlockNumber = 89 }},
		{"wrong hash", func(o *idov3whitelist.ProjectIdentityObservation) { o.BlockHash = common.HexToHash("0xb1") }},
		{"missing amount", func(o *idov3whitelist.ProjectIdentityObservation) { o.RaisedAtBlockRaw = "" }},
		{"leading zero", func(o *idov3whitelist.ProjectIdentityObservation) { o.RaisedAtBlockRaw = "0600" }},
		{"wrong raised", func(o *idov3whitelist.ProjectIdentityObservation) { o.RaisedAtBlockRaw = "599" }},
		{"wrong sold", func(o *idov3whitelist.ProjectIdentityObservation) { o.TokensSoldAtBlockRaw = "299" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := observed
			tc.alter(&o)
			if check(o, 90, hash) == nil {
				t.Fatal("mismatch accepted")
			}
		})
	}
	if check(observed, 91, hash) == nil || check(observed, 90, common.HexToHash("0xb1")) == nil {
		t.Fatal("caller anchor mismatch accepted")
	}
}

func TestMatchV3CreationProjectAtBlockChecksImmutableEconomics(t *testing.T) {
	proxy := common.HexToAddress("0x1111111111111111111111111111111111111111")
	payment := common.HexToAddress("0x2222222222222222222222222222222222222222")
	sale := common.HexToAddress("0x3333333333333333333333333333333333333333")
	owner := common.HexToAddress("0x4444444444444444444444444444444444444444")
	receiver := common.HexToAddress("0x5555555555555555555555555555555555555555")
	hash := common.HexToHash("0xa1")
	binding := idov3whitelist.ExpectedProjectBinding{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7, PaymentToken: payment, ProjectToken: sale}
	created := &V3CreatedEvent{ChainID: 31337, Proxy: proxy, ProjectID: 7, PaymentToken: payment, ProjectToken: sale, ProjectOwner: owner, FeeReceiver: receiver, ProjectDecimals: 18, PaymentDecimals: 6, FeeRate: 200, TokenPriceRaw: "20", HardCapRaw: "1000", SoftCapRaw: "500", MaxPerUserRaw: "900", StartTimeRaw: "100", EndTimeRaw: "200", BlockNumber: 90, BlockHash: hash}
	observed := idov3whitelist.ProjectIdentityObservation{ChainID: 31337, ProxyAddress: proxy, ProjectID: 7, PaymentToken: payment, ProjectToken: sale, ProjectOwner: owner, FeeReceiver: receiver, ProjectDecimals: 18, PaymentDecimals: 6, FeeRate: 200, TokenPriceRaw: "20", HardCapRaw: "1000", SoftCapRaw: "500", MaxPerUserRaw: "900", StartTimeRaw: "100", EndTimeRaw: "200", BlockNumber: 90, BlockHash: hash, State: idov3whitelist.ProjectPending}
	if err := MatchV3CreationProjectAtBlock(created, binding, observed); err != nil {
		t.Fatal(err)
	}
	later := observed
	later.BlockNumber = 91
	later.BlockHash = common.HexToHash("0xb1")
	later.State = idov3whitelist.ProjectActive
	if err := MatchV3CreationProjectImmutable(created, binding, later); err != nil {
		t.Fatalf("immutable identity drifted across blocks: %v", err)
	}
	later.TokenPriceRaw = "21"
	if err := MatchV3CreationProjectImmutable(created, binding, later); err == nil {
		t.Fatal("later immutable price drift accepted")
	}
	for name, mutate := range map[string]func(*idov3whitelist.ProjectIdentityObservation){
		"owner":        func(o *idov3whitelist.ProjectIdentityObservation) { o.ProjectOwner = payment },
		"fee receiver": func(o *idov3whitelist.ProjectIdentityObservation) { o.FeeReceiver = payment },
		"decimals":     func(o *idov3whitelist.ProjectIdentityObservation) { o.PaymentDecimals = 18 },
		"fee":          func(o *idov3whitelist.ProjectIdentityObservation) { o.FeeRate = 201 },
		"price":        func(o *idov3whitelist.ProjectIdentityObservation) { o.TokenPriceRaw = "21" },
		"hard":         func(o *idov3whitelist.ProjectIdentityObservation) { o.HardCapRaw = "1001" },
		"soft":         func(o *idov3whitelist.ProjectIdentityObservation) { o.SoftCapRaw = "501" },
		"max":          func(o *idov3whitelist.ProjectIdentityObservation) { o.MaxPerUserRaw = "901" },
		"start":        func(o *idov3whitelist.ProjectIdentityObservation) { o.StartTimeRaw = "101" },
		"end":          func(o *idov3whitelist.ProjectIdentityObservation) { o.EndTimeRaw = "201" },
		"block":        func(o *idov3whitelist.ProjectIdentityObservation) { o.BlockNumber = 91 },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := observed
			mutate(&candidate)
			if MatchV3CreationProjectAtBlock(created, binding, candidate) == nil {
				t.Fatal("immutable creation drift accepted")
			}
		})
	}
}

package idov3whitelist

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestMatchProjectIdentityObservationFailsClosedOnEveryBindingField(t *testing.T) {
	expected := ExpectedProjectBinding{ChainID: 31337,
		ProxyAddress: common.HexToAddress("0x1111111111111111111111111111111111111111"), ProjectID: 41376991395446785,
		PaymentToken: common.HexToAddress("0x2222222222222222222222222222222222222222"),
		ProjectToken: common.HexToAddress("0x3333333333333333333333333333333333333333")}
	observed := ProjectIdentityObservation{ChainID: expected.ChainID, ProxyAddress: expected.ProxyAddress,
		ProjectID: expected.ProjectID, PaymentToken: expected.PaymentToken, ProjectToken: expected.ProjectToken,
		State: ProjectActive, BlockNumber: 90, BlockHash: common.HexToHash("0x01")}
	if err := MatchProjectIdentityObservation(expected, observed); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*ProjectIdentityObservation){
		"chain": func(v *ProjectIdentityObservation) { v.ChainID = 56 },
		"proxy": func(v *ProjectIdentityObservation) {
			v.ProxyAddress = common.HexToAddress("0x4444444444444444444444444444444444444444")
		},
		"project": func(v *ProjectIdentityObservation) { v.ProjectID++ },
		"payment": func(v *ProjectIdentityObservation) { v.PaymentToken = v.ProjectToken },
		"sale":    func(v *ProjectIdentityObservation) { v.ProjectToken = v.PaymentToken },
		"block":   func(v *ProjectIdentityObservation) { v.BlockNumber = 0 },
		"hash":    func(v *ProjectIdentityObservation) { v.BlockHash = common.Hash{} },
		"state":   func(v *ProjectIdentityObservation) { v.State = ProjectState(255) },
	} {
		t.Run(name, func(t *testing.T) {
			got := observed
			change(&got)
			if MatchProjectIdentityObservation(expected, got) == nil {
				t.Fatal("mismatched identity admitted")
			}
		})
	}
	bad := expected
	bad.PaymentToken = bad.ProjectToken
	if MatchProjectIdentityObservation(bad, observed) == nil {
		t.Fatal("invalid persisted token identity accepted")
	}
	bad = expected
	bad.ProxyAddress = bad.PaymentToken
	if MatchProjectIdentityObservation(bad, observed) == nil {
		t.Fatal("proxy as token admitted")
	}
}

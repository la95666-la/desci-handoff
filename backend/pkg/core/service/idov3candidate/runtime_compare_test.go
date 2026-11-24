package idov3candidate

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestMatchInertRuntimeExpectationNeverIgnoresIdentityFields(t *testing.T) {
	addr := func(byteValue byte) common.Address { return common.BytesToAddress([]byte{byteValue}) }
	hash := func(byteValue byte) common.Hash { return common.BytesToHash([]byte{byteValue}) }
	audit := InertProjectIdentityAudit{ChainID: 31337, ProjectID: 89, ProxyAddress: addr(1), ProjectToken: addr(2), PaymentToken: addr(3),
		Implementation: addr(4), Governor: addr(5), StartBlock: 90, ProxyCodeHash: hash(6), ImplementationCodeHash: hash(7)}
	expected := InertRuntimeExpectation{ChainID: audit.ChainID, ProxyAddress: audit.ProxyAddress, Implementation: audit.Implementation,
		Governor: audit.Governor, StartBlock: audit.StartBlock, ProxyCodeHash: audit.ProxyCodeHash, ImplementationCodeHash: audit.ImplementationCodeHash}
	if err := MatchInertRuntimeExpectation(audit, expected); err != nil {
		t.Fatal(err)
	}
	for name, alter := range map[string]func(*InertRuntimeExpectation){
		"chain":               func(v *InertRuntimeExpectation) { v.ChainID = 56 },
		"proxy":               func(v *InertRuntimeExpectation) { v.ProxyAddress = addr(8) },
		"implementation":      func(v *InertRuntimeExpectation) { v.Implementation = addr(8) },
		"governor":            func(v *InertRuntimeExpectation) { v.Governor = addr(8) },
		"start block":         func(v *InertRuntimeExpectation) { v.StartBlock = 91 },
		"proxy hash":          func(v *InertRuntimeExpectation) { v.ProxyCodeHash = hash(8) },
		"implementation hash": func(v *InertRuntimeExpectation) { v.ImplementationCodeHash = hash(8) },
	} {
		t.Run(name, func(t *testing.T) {
			changed := expected
			alter(&changed)
			if MatchInertRuntimeExpectation(audit, changed) == nil {
				t.Fatal("identity drift accepted")
			}
		})
	}
	for name, alter := range map[string]func(*InertProjectIdentityAudit){
		"missing project": func(v *InertProjectIdentityAudit) { v.ProjectID = 0 },
		"missing token":   func(v *InertProjectIdentityAudit) { v.ProjectToken = common.Address{} },
		"token alias":     func(v *InertProjectIdentityAudit) { v.ProjectToken = v.PaymentToken },
		"zero governor":   func(v *InertProjectIdentityAudit) { v.Governor = common.Address{} },
		"zero hash":       func(v *InertProjectIdentityAudit) { v.ProxyCodeHash = common.Hash{} },
	} {
		t.Run(name, func(t *testing.T) {
			changed := audit
			alter(&changed)
			if MatchInertRuntimeExpectation(changed, expected) == nil {
				t.Fatal("malformed inert identity accepted")
			}
		})
	}
}

func TestMatchInertProjectExpectationDoesNotConstructAuthority(t *testing.T) {
	addr := func(last byte) common.Address { return common.BytesToAddress([]byte{last}) }
	audit := InertProjectIdentityAudit{ChainID: 31337, ProjectID: 89, ProxyAddress: addr(1), ProjectToken: addr(2), PaymentToken: addr(3)}
	expected := InertProjectExpectation{ChainID: audit.ChainID, ProjectID: audit.ProjectID, ProxyAddress: audit.ProxyAddress, ProjectToken: audit.ProjectToken, PaymentToken: audit.PaymentToken}
	if err := MatchInertProjectExpectation(audit, expected); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*InertProjectExpectation){
		"chain":         func(v *InertProjectExpectation) { v.ChainID = 56 },
		"project":       func(v *InertProjectExpectation) { v.ProjectID++ },
		"proxy":         func(v *InertProjectExpectation) { v.ProxyAddress = addr(4) },
		"sale token":    func(v *InertProjectExpectation) { v.ProjectToken = addr(4) },
		"payment token": func(v *InertProjectExpectation) { v.PaymentToken = addr(4) },
	} {
		t.Run(name, func(t *testing.T) {
			changed := expected
			mutate(&changed)
			if MatchInertProjectExpectation(audit, changed) == nil {
				t.Fatal("inert project drift accepted")
			}
		})
	}
	bad := audit
	bad.ProjectToken = audit.PaymentToken
	if MatchInertProjectExpectation(bad, expected) == nil {
		t.Fatal("aliased tokens accepted")
	}
}

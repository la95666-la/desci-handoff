package idov3candidate

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
)

// InertRuntimeExpectation is an independently supplied identity for a
// diagnostic comparison. It carries no approval, provenance or signer rights.
// Matching an unverified database row to it must never construct a trusted
// RuntimeIdentity, scanner source, project binding or proof.
type InertRuntimeExpectation struct {
	ChainID                uint64
	ProxyAddress           common.Address
	Implementation         common.Address
	Governor               common.Address
	StartBlock             uint64
	ProxyCodeHash          common.Hash
	ImplementationCodeHash common.Hash
}

// InertProjectExpectation must be assembled independently from the joined
// candidate tables. It is an audit target, NOT an admitted project binding.
type InertProjectExpectation struct {
	ChainID      uint64
	ProjectID    uint64
	ProxyAddress common.Address
	ProjectToken common.Address
	PaymentToken common.Address
}

// MatchInertProjectExpectation never turns a matching project row into an
// ExpectedProjectBinding. Matching these fields does not authenticate the
// source of the expectation or establish on-chain creation/finality.
func MatchInertProjectExpectation(audit InertProjectIdentityAudit, expected InertProjectExpectation) error {
	if audit.ChainID == 0 || audit.ProjectID == 0 || audit.ProxyAddress == (common.Address{}) ||
		audit.ProjectToken == (common.Address{}) || audit.PaymentToken == (common.Address{}) ||
		audit.ProjectToken == audit.PaymentToken || audit.ProxyAddress == audit.ProjectToken || audit.ProxyAddress == audit.PaymentToken ||
		audit.ChainID != expected.ChainID || audit.ProjectID != expected.ProjectID || audit.ProxyAddress != expected.ProxyAddress ||
		audit.ProjectToken != expected.ProjectToken || audit.PaymentToken != expected.PaymentToken {
		return errors.New("V3 inert project expectation unavailable or mismatched")
	}
	return nil
}

// MatchInertRuntimeExpectation compares the COMPLETE inert runtime identity.
// It deliberately returns only an error, not an authority-bearing object.
func MatchInertRuntimeExpectation(audit InertProjectIdentityAudit, expected InertRuntimeExpectation) error {
	if audit.ChainID == 0 || audit.ProjectID == 0 || audit.ProxyAddress == (common.Address{}) ||
		audit.ProjectToken == (common.Address{}) || audit.PaymentToken == (common.Address{}) ||
		audit.ProjectToken == audit.PaymentToken || audit.ProjectToken == audit.ProxyAddress || audit.PaymentToken == audit.ProxyAddress ||
		audit.ChainID != expected.ChainID || audit.ProxyAddress != expected.ProxyAddress ||
		audit.Implementation == (common.Address{}) || audit.Implementation == audit.ProxyAddress ||
		audit.Governor == (common.Address{}) || audit.Governor == audit.ProxyAddress || audit.Governor == audit.Implementation ||
		audit.Implementation != expected.Implementation || audit.Governor != expected.Governor ||
		audit.StartBlock == 0 || audit.StartBlock != expected.StartBlock ||
		audit.ProxyCodeHash == (common.Hash{}) || audit.ProxyCodeHash != expected.ProxyCodeHash ||
		audit.ImplementationCodeHash == (common.Hash{}) || audit.ImplementationCodeHash != expected.ImplementationCodeHash {
		return errors.New("V3 inert runtime expectation unavailable or mismatched")
	}
	return nil
}

package chain

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"joincare_server/pkg/common/idov3whitelist"
)

// V3Transport is an inert transport designation, not an authorization. Safe
// transactions must never be represented as a relayer outbox intent; project
// owner and public calls must be signed by their own wallet. Only an approved
// operator may eventually use a separately gated durable relayer path.
type V3Transport uint8

const (
	V3RelayerTransport V3Transport = iota + 1
	V3SafeTransport
	V3WalletTransport
)

type V3TransportPreview struct {
	Transport V3Transport
	To        common.Address
	From      common.Address
	CallData  []byte
}

func v3TransportForRole(role V3LifecycleRole) (V3Transport, error) {
	switch role {
	case V3Operator:
		return V3RelayerTransport, nil
	case V3OwnerSafe:
		return V3SafeTransport, nil
	case V3ProjectOwner, V3PublicCaller:
		return V3WalletTransport, nil
	default:
		return 0, errors.New("V3 caller transport unavailable")
	}
}

// PreviewV3CreateTransport binds the ABI and caller to a single channel.
// It does not persist an action, authenticate the inputs or grant a signer.
func PreviewV3CreateTransport(binding idov3whitelist.ExpectedProjectBinding, params V3CreateParams,
	caller common.Address, role V3LifecycleRole, expected V3CallerIdentity) (V3TransportPreview, error) {
	data, err := PreviewV3CreateCall(binding, params, caller, role, expected)
	if err != nil {
		return V3TransportPreview{}, err
	}
	transport, err := v3TransportForRole(role)
	if err != nil {
		return V3TransportPreview{}, err
	}
	return V3TransportPreview{Transport: transport, To: binding.ProxyAddress, From: caller, CallData: data}, nil
}

// PreviewV3LifecycleTransport likewise cannot bypass caller/state checks or
// convert Safe/project-owner execution into a V2 relayer transaction.
func PreviewV3LifecycleTransport(binding idov3whitelist.ExpectedProjectBinding, observed idov3whitelist.ProjectIdentityObservation,
	method string, caller common.Address, role V3LifecycleRole, expected V3CallerIdentity) (V3TransportPreview, error) {
	data, err := PreviewV3LifecycleCall(binding, observed, method, caller, role, expected)
	if err != nil {
		return V3TransportPreview{}, err
	}
	transport, err := v3TransportForRole(role)
	if err != nil {
		return V3TransportPreview{}, err
	}
	return V3TransportPreview{Transport: transport, To: binding.ProxyAddress, From: caller, CallData: data}, nil
}

// Deposit is always a project-owner wallet transaction. The optional pinned
// balance/allowance preflight is separate and does not turn this into a Safe
// proposal or a relayer action.
func PreviewV3DepositTransport(binding idov3whitelist.ExpectedProjectBinding, observed idov3whitelist.ProjectIdentityObservation,
	caller common.Address, amount *big.Int) (V3TransportPreview, error) {
	data, err := PreviewV3ProjectDepositCall(binding, observed, caller, amount, V3ProjectOwner)
	if err != nil {
		return V3TransportPreview{}, err
	}
	return V3TransportPreview{Transport: V3WalletTransport, To: binding.ProxyAddress, From: caller, CallData: data}, nil
}

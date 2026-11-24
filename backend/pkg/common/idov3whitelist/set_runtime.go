package idov3whitelist

import (
	"bytes"
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"joincare_server/pkg/common/constant"
)

// VerifySetRuntimeAt establishes the independently signed pre-Set identity at
// a confirmed block. This cannot grant transaction signing or a whitelist
// proof. Re-run before approval application; never use latest as a substitute
// for a pinned block or trust an unverified runtime row on its own.
func VerifySetRuntimeAt(ctx context.Context, rpc PolicyRPC, m SetManifest) error {
	_, err := InspectSetRuntimeAt(ctx, rpc, m)
	return err
}

// InspectSetRuntimeAt returns pinned project-token decimals for Set amount
// conversion. Neither the Admin form nor V2 config supplies this value.
func InspectSetRuntimeAt(ctx context.Context, rpc PolicyRPC, m SetManifest) (uint8, error) {
	invalid := errors.New("V3 Set fixed-block runtime unavailable")
	if rpc == nil || m.FundingMode != constant.FundingModeCappedPresale || m.ContractVersion != constant.ContractVersionIDOV3Stablecoin ||
		m.ChainID == 0 || m.ProjectID == 0 || m.StartBlock == 0 || m.Confirmations == 0 ||
		m.ProxyAddress == m.Implementation || m.ProxyAddress == m.ProjectToken || m.ProxyAddress == m.PaymentToken ||
		m.ProjectToken == m.PaymentToken || m.OwnerSafe == m.Governor || m.FeeRate > 1000 ||
		m.PaymentDecimals == 0 || m.PaymentDecimals > 18 ||
		!canonicalAdmissionAddress(m.ProxyAddress) || !canonicalAdmissionAddress(m.Implementation) ||
		!canonicalAdmissionAddress(m.OwnerSafe) || !canonicalAdmissionAddress(m.Operator) ||
		!canonicalAdmissionAddress(m.Governor) ||
		!canonicalAdmissionAddress(m.ProjectToken) || !canonicalAdmissionAddress(m.PaymentToken) ||
		!canonicalAdmissionAddress(m.FeeReceiver) ||
		!canonicalAdmissionHash(m.ProxyCodeHash) || !canonicalAdmissionHash(m.ImplementationCodeHash) ||
		!canonicalAdmissionHash(m.OwnerCodeHash) || !canonicalAdmissionHash(m.GovernorCodeHash) {
		return 0, invalid
	}
	head, err := rpc.HeaderByNumber(ctx, nil)
	if err != nil || head == nil || head.Number == nil || !head.Number.IsUint64() ||
		!HasConfirmations(head.Number.Uint64(), m.StartBlock, m.Confirmations) {
		return 0, invalid
	}
	height := head.Number.Uint64() - (m.Confirmations - 1)
	block := new(big.Int).SetUint64(height)
	header, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || header == nil || header.Number == nil || header.Number.Cmp(block) != 0 {
		return 0, invalid
	}
	scope := Scope{ChainID: new(big.Int).SetUint64(m.ChainID), ProjectID: new(big.Int).SetUint64(m.ProjectID), ProxyAddress: common.HexToAddress(m.ProxyAddress)}
	runtime := RuntimeIdentity{Implementation: common.HexToAddress(m.Implementation), ProxyCodeHash: common.HexToHash(m.ProxyCodeHash), CodeHash: common.HexToHash(m.ImplementationCodeHash)}
	governance := GovernanceIdentity{Owner: common.HexToAddress(m.OwnerSafe), Governor: common.HexToAddress(m.Governor),
		OwnerCodeHash: common.HexToHash(m.OwnerCodeHash), GovernorCodeHash: common.HexToHash(m.GovernorCodeHash)}
	if _, err := ReadLifecycleCallersAt(ctx, rpc, scope, runtime, governance,
		common.HexToAddress(m.Operator), height, header.Hash()); err != nil {
		return 0, invalid
	}
	read := func(address common.Address, signature string, argument common.Address) ([]byte, error) {
		data := append([]byte(nil), crypto.Keccak256([]byte(signature))[:4]...)
		if argument != (common.Address{}) {
			data = append(data, common.LeftPadBytes(argument.Bytes(), 32)...)
		}
		return rpc.CallContract(ctx, ethereum.CallMsg{To: &address, Data: data}, block)
	}
	proxy := scope.ProxyAddress
	fee, err := read(proxy, "feeReceiver()", common.Address{})
	if err != nil || len(fee) != 32 || !bytes.Equal(fee[:12], make([]byte, 12)) || common.BytesToAddress(fee[12:]) != common.HexToAddress(m.FeeReceiver) {
		return 0, invalid
	}
	rate, err := read(proxy, "successFeeRate()", common.Address{})
	if err != nil || len(rate) != 32 || new(big.Int).SetBytes(rate).Cmp(new(big.Int).SetUint64(uint64(m.FeeRate))) != 0 {
		return 0, invalid
	}
	var projectDecimals uint8
	for _, token := range []struct {
		address  common.Address
		decimals uint8
		sig      string
	}{
		{common.HexToAddress(m.PaymentToken), m.PaymentDecimals, "paymentTokenConfigs(address)"},
		{common.HexToAddress(m.ProjectToken), 0, "projectTokenConfigs(address)"},
	} {
		decimals, readErr := read(token.address, "decimals()", common.Address{})
		if readErr != nil || len(decimals) != 32 || new(big.Int).SetBytes(decimals).Sign() <= 0 ||
			new(big.Int).SetBytes(decimals).Cmp(big.NewInt(18)) > 0 ||
			(token.decimals != 0 && new(big.Int).SetBytes(decimals).Uint64() != uint64(token.decimals)) {
			return 0, invalid
		}
		if token.decimals == 0 {
			projectDecimals = uint8(new(big.Int).SetBytes(decimals).Uint64())
		}
		config, readErr := read(proxy, token.sig, token.address)
		if readErr != nil || len(config) != 64 || new(big.Int).SetBytes(config[:32]).Cmp(big.NewInt(1)) != 0 ||
			!bytes.Equal(config[32:], decimals) {
			return 0, invalid
		}
	}
	// Detect a replacement during the last token read. The caller must check
	// canonical history again before using any later execution observation.
	after, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || after == nil || after.Number == nil || after.Number.Cmp(block) != 0 || after.Hash() != header.Hash() {
		return 0, invalid
	}
	return projectDecimals, nil
}

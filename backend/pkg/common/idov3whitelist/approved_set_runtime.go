package idov3whitelist

import (
	"bytes"
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// InspectApprovedSetRuntimeAt observes the pre-create deployment at a
// confirmed block. getWhitelistPolicy(projectId) reverts until the project
// exists, so the policy reader must not be used for a Set or reverted create.
func InspectApprovedSetRuntimeAt(ctx context.Context, rpc PolicyRPC, record ApprovedSetRecord,
	ownerHash, governorHash common.Hash) (uint64, common.Hash, error) {
	invalid := errors.New("V3 approved runtime unavailable")
	if ctx == nil || rpc == nil || ownerHash == (common.Hash{}) || governorHash == (common.Hash{}) {
		return 0, common.Hash{}, invalid
	}
	head, err := rpc.HeaderByNumber(ctx, nil)
	if err != nil || head == nil || head.Number == nil || !head.Number.IsUint64() || head.Number.Uint64() < 128805696+14 {
		return 0, common.Hash{}, invalid
	}
	height := head.Number.Uint64() - 14
	header, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(height))
	if err != nil || header == nil || header.Number == nil || !header.Number.IsUint64() || header.Number.Uint64() != height {
		return 0, common.Hash{}, invalid
	}
	if VerifyApprovedV3RuntimeBlock(ctx, rpc, record, ownerHash, governorHash, height, header.Hash()) != nil {
		return 0, common.Hash{}, invalid
	}
	return height, header.Hash(), nil
}

// VerifyApprovedV3RuntimeBlock uses separately reviewed Safe code hashes.
// It never grants Safe custody, signing, financial application or a proof.
func VerifyApprovedV3RuntimeBlock(ctx context.Context, rpc PolicyRPC, record ApprovedSetRecord,
	ownerHash, governorHash common.Hash, height uint64, hash common.Hash) error {
	return verifyApprovedV3DeploymentBlock(ctx, rpc, record, ownerHash, governorHash, height, hash, true)
}

// A reverted create can have stale fee/token policy. Verify only the pinned
// deployment and operator at its canonical receipt block before releasing its
// original nonce; this narrower check must never authorize a new create.
func VerifyApprovedV3RevertedBlock(ctx context.Context, rpc PolicyRPC, record ApprovedSetRecord,
	ownerHash, governorHash common.Hash, height uint64, hash common.Hash) error {
	return verifyApprovedV3DeploymentBlock(ctx, rpc, record, ownerHash, governorHash, height, hash, false)
}

func verifyApprovedV3DeploymentBlock(ctx context.Context, rpc PolicyRPC, record ApprovedSetRecord,
	ownerHash, governorHash common.Hash, height uint64, hash common.Hash, requireTerms bool) error {
	invalid := errors.New("V3 approved runtime block unavailable")
	id := record.Identity
	if ctx == nil || rpc == nil || height < 128805696 || hash == (common.Hash{}) ||
		ownerHash == (common.Hash{}) || governorHash == (common.Hash{}) ||
		id.ProjectID == 0 || id.ChainID != 56 ||
		id.Proxy != "0x0d4040875742975269c55e16c8884d3ee9510b8e" ||
		id.PaymentToken != "0x55d398326f99059ff775485246999027b3197955" ||
		id.Operator != "0xfd712061ee08e070a930ecd45e935ae5d7f5445c" ||
		!canonicalAdmissionAddress(id.ProjectToken) || !canonicalAdmissionAddress(id.FeeReceiver) ||
		id.ProjectToken == id.PaymentToken || id.ProjectToken == id.Proxy ||
		id.TokenDecimals == 0 || id.TokenDecimals > 18 || id.PaymentDecimals != 18 || id.FeeRate > 1000 {
		return invalid
	}
	chainID, err := rpc.ChainID(ctx)
	if err != nil || chainID == nil || chainID.Cmp(big.NewInt(56)) != 0 {
		return invalid
	}
	block := new(big.Int).SetUint64(height)
	header, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || header == nil || header.Number == nil || header.Number.Cmp(block) != 0 || header.Hash() != hash {
		return invalid
	}
	proxy := common.HexToAddress(id.Proxy)
	implementation := common.HexToAddress("0x887fcc6e54fc8229919166ad90b16c19b4992560")
	owner, e := ReadOwnerSafeAt(ctx, rpc, proxy, implementation, ownerHash, block)
	if e != nil {
		return invalid
	}
	for _, entry := range []struct {
		address common.Address
		digest  common.Hash
	}{
		{proxy, common.HexToHash("0x5974ff3c9404cc36ed5c7d8a3eab148940366bbf46740d064255c13a842f3cb8")},
		{implementation, common.HexToHash("0x015279399127ac168667c7fc5238ec04a16923c7561d54b4d823d77e5f55c590")},
		{owner, ownerHash},
		{common.HexToAddress("0xc46659e67543aaaec553f7f3fc8f004b7dd286ef"), governorHash},
	} {
		code, e := rpc.CodeAt(ctx, entry.address, block)
		if e != nil || len(code) == 0 || crypto.Keccak256Hash(code) != entry.digest {
			return invalid
		}
	}
	slot, e := rpc.StorageAt(ctx, proxy, eip1967ImplementationSlot, block)
	if e != nil || len(slot) != 32 || !bytes.Equal(slot[:12], make([]byte, 12)) || common.BytesToAddress(slot[12:]) != implementation {
		return invalid
	}
	read := func(to common.Address, sig string, token common.Address) ([]byte, error) {
		data := append([]byte(nil), crypto.Keccak256([]byte(sig))[:4]...)
		if token != (common.Address{}) {
			data = append(data, common.LeftPadBytes(token.Bytes(), 32)...)
		}
		return rpc.CallContract(ctx, ethereum.CallMsg{To: &to, Data: data}, block)
	}
	version, e := read(proxy, "contractVersion()", common.Address{})
	if e != nil || len(version) != 32 || !bytes.Equal(version, v3ContractVersion[:]) {
		return invalid
	}
	for _, entry := range []struct {
		sig     string
		address common.Address
	}{
		{"owner()", owner},
		{"whitelistGovernor()", common.HexToAddress("0xc46659e67543aaaec553f7f3fc8f004b7dd286ef")},
		{"operator()", common.HexToAddress(id.Operator)},
	} {
		value, e := read(proxy, entry.sig, common.Address{})
		if e != nil || len(value) != 32 || !bytes.Equal(value[:12], make([]byte, 12)) || common.BytesToAddress(value[12:]) != entry.address {
			return invalid
		}
	}
	if !requireTerms {
		after, e := rpc.HeaderByNumber(ctx, block)
		if e != nil || after == nil || after.Number == nil || after.Number.Cmp(block) != 0 || after.Hash() != hash {
			return invalid
		}
		return nil
	}
	fee, e := read(proxy, "feeReceiver()", common.Address{})
	if e != nil || len(fee) != 32 || !bytes.Equal(fee[:12], make([]byte, 12)) || common.BytesToAddress(fee[12:]) != common.HexToAddress(id.FeeReceiver) {
		return invalid
	}
	rate, e := read(proxy, "successFeeRate()", common.Address{})
	if e != nil || len(rate) != 32 || new(big.Int).SetBytes(rate).Cmp(new(big.Int).SetUint64(uint64(id.FeeRate))) != 0 {
		return invalid
	}
	for _, entry := range []struct {
		token    common.Address
		decimals uint8
		config   string
	}{
		{common.HexToAddress(id.PaymentToken), id.PaymentDecimals, "paymentTokenConfigs(address)"},
		{common.HexToAddress(id.ProjectToken), id.TokenDecimals, "projectTokenConfigs(address)"},
	} {
		decimals, e := read(entry.token, "decimals()", common.Address{})
		if e != nil || len(decimals) != 32 || new(big.Int).SetBytes(decimals).Cmp(new(big.Int).SetUint64(uint64(entry.decimals))) != 0 {
			return invalid
		}
		config, e := read(proxy, entry.config, entry.token)
		if e != nil || len(config) != 64 || new(big.Int).SetBytes(config[:32]).Cmp(big.NewInt(1)) != 0 || !bytes.Equal(config[32:], decimals) {
			return invalid
		}
	}
	after, e := rpc.HeaderByNumber(ctx, block)
	if e != nil || after == nil || after.Number == nil || after.Number.Cmp(block) != 0 || after.Hash() != hash {
		return invalid
	}
	return nil
}

package project

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/viper"
	"joincare_server/pkg/common/chainrpc"
	"joincare_server/pkg/common/idov3whitelist"
)

// Independent V3 token gate. These are deployment-reviewed BSC addresses and
// runtime hashes, not project parameters or an alternative to Owner Safe.
const (
	v3TokenProxy              = "0x0D4040875742975269c55E16C8884d3Ee9510B8E"
	v3TokenImplementation     = "0x887fCC6E54Fc8229919166Ad90B16C19B4992560"
	v3TokenUSDT               = "0x55d398326f99059fF775485246999027B3197955"
	v3TokenTESTCAT            = "0xf77c5b5faabb9945ac8701ad08ef09e12dfc768a"
	v3TokenProxyHash          = "0x5974ff3c9404cc36ed5c7d8a3eab148940366bbf46740d064255c13a842f3cb8"
	v3TokenImplementationHash = "0x015279399127ac168667c7fc5238ec04a16923c7561d54b4d823d77e5f55c590"
)

var v3ImplementationSlot = common.HexToHash("0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc")
var v3TokenTxHash = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)

const v3TokenSafeABI = `[{"type":"function","name":"execTransaction","inputs":[{"name":"to","type":"address"},{"name":"value","type":"uint256"},{"name":"data","type":"bytes"},{"name":"operation","type":"uint8"},{"name":"safeTxGas","type":"uint256"},{"name":"baseGas","type":"uint256"},{"name":"gasPrice","type":"uint256"},{"name":"gasToken","type":"address"},{"name":"refundReceiver","type":"address"},{"name":"signatures","type":"bytes"}],"outputs":[{"type":"bool"}]}]`

var v3TokenSafeSuccess = crypto.Keccak256Hash([]byte("ExecutionSuccess(bytes32,uint256)"))

type V3TokenGateAsset struct {
	Symbol     string `json:"symbol"`
	Token      string `json:"token"`
	Method     string `json:"method"`
	Allowed    bool   `json:"allowed"`
	Decimals   uint8  `json:"decimals"`
	Simulation string `json:"simulation"`
}

type V3TokenGateStatus struct {
	ChainID       uint64           `json:"chain_id"`
	Proxy         string           `json:"proxy"`
	OwnerSafe     string           `json:"owner_safe"`
	SafeThreshold uint64           `json:"safe_threshold"`
	SafeNonce     string           `json:"safe_nonce"`
	Block         uint64           `json:"block"`
	BlockHash     string           `json:"block_hash"`
	Confirmations uint64           `json:"confirmations"`
	USDT          V3TokenGateAsset `json:"usdt"`
	TESTCAT       V3TokenGateAsset `json:"testcat"`
}

type V3TokenGateReceipt struct {
	Asset         string `json:"asset"`
	TxHash        string `json:"tx_hash"`
	Block         uint64 `json:"block"`
	BlockHash     string `json:"block_hash"`
	Confirmations uint64 `json:"confirmations"`
	Allowed       bool   `json:"allowed"`
	Decimals      uint8  `json:"decimals"`
}

// This read-only token handoff uses the operator-approved commercial endpoint
// alone. It never falls back to V2 RPC, the former secondary RPC, or a URL
// supplied in an Admin request. Set, scanner and Job use separate admissions.
func v3TokenGateRPC(ctx context.Context) (*ethclient.Client, error) {
	endpoint, err := v3TokenGateEndpoint()
	if err != nil {
		return nil, err
	}
	return chainrpc.Dial(ctx, endpoint)
}

func v3TokenGateEndpoint() (string, error) {
	if !viper.GetBool("idoV3TokenAdmission.enabled") {
		return "", errors.New("V3 token admission read gate disabled")
	}
	endpoint := strings.TrimSpace(viper.GetString("idoV3TokenAdmission.primaryRPC"))
	if err := idov3whitelist.ValidateApprovedV3RPCEndpoint(endpoint); err != nil {
		return "", errors.New("approved V3 commercial read RPC unavailable")
	}
	return endpoint, nil
}

type v3TokenCaller interface {
	CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)
}

func v3TokenCall(ctx context.Context, rpc v3TokenCaller, to common.Address, signature string, argument *common.Address, from *common.Address, block *big.Int) ([]byte, error) {
	data := v3TokenCallData(signature, argument, from != nil && argument != nil)
	call := ethereum.CallMsg{To: &to, Data: data}
	if from != nil {
		call.From = *from
	}
	return rpc.CallContract(ctx, call, block)
}

func v3TokenCallData(signature string, argument *common.Address, allow bool) []byte {
	data := append([]byte(nil), crypto.Keccak256([]byte(signature))[:4]...)
	if argument != nil {
		data = append(data, common.LeftPadBytes(argument.Bytes(), 32)...)
		if allow {
			data = append(data, common.LeftPadBytes(big.NewInt(1).Bytes(), 32)...)
		}
	}
	return data
}

func v3TokenWord(data []byte) (*big.Int, error) {
	if len(data) != 32 {
		return nil, errors.New("V3 read result malformed")
	}
	return new(big.Int).SetBytes(data), nil
}

type v3CallReader interface {
	CallContext(context.Context, interface{}, string, ...interface{}) error
}

// Use the exact eth_call shape independently verified on the production host:
// data (not input), no default zero-address from, and a fixed block number.
// This is read-only. Retry one failed RPC read at that same block, but never
// accept a malformed or unexpected successful response. Raw provider errors
// may contain a credentialed URL and must not leave this function.
func readV3TokenDecimals(ctx context.Context, rpc v3CallReader, token common.Address, block *big.Int) (uint8, error) {
	if ctx == nil || rpc == nil || block == nil || block.Sign() <= 0 {
		return 0, errors.New("V3 token decimals read unavailable")
	}
	call := map[string]interface{}{"to": token, "data": hexutil.Bytes(v3TokenCallData("decimals()", nil, false))}
	tag := hexutil.EncodeBig(block)
	var raw hexutil.Bytes
	err := rpc.CallContext(ctx, &raw, "eth_call", call, tag)
	if err != nil && ctx.Err() == nil {
		// A short delay helps with a transient provider/network rejection;
		// cancellation and the caller's overall deadline still take precedence.
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return 0, errors.New("V3 token decimals read unavailable")
		case <-timer.C:
		}
		err = rpc.CallContext(ctx, &raw, "eth_call", call, tag)
	}
	if err != nil {
		return 0, errors.New("V3 token decimals read unavailable")
	}
	word, err := v3TokenWord(raw)
	if err != nil {
		return 0, errors.New("V3 token decimals result malformed")
	}
	if word.Cmp(big.NewInt(18)) != 0 {
		return 0, errors.New("V3 token decimals mismatch")
	}
	return 18, nil
}

// Unlike the legacy USDT-only check, project tokens may use 1..18 decimals.
// Use a data-only eth_call at the confirmed block: some BSC tokens reject an
// implicit zero-address from even for decimals(). Never return provider errors
// because the commercial RPC URL may contain a credential.
func readV3ProjectTokenDecimals(ctx context.Context, rpc v3CallReader, token common.Address, block *big.Int) (uint8, error) {
	invalid := errors.New("V3 project token decimals unavailable")
	if ctx == nil || rpc == nil || token == (common.Address{}) || block == nil || block.Sign() <= 0 {
		return 0, invalid
	}
	call := map[string]interface{}{"to": token, "data": hexutil.Bytes(v3TokenCallData("decimals()", nil, false))}
	var raw hexutil.Bytes
	if rpc.CallContext(ctx, &raw, "eth_call", call, hexutil.EncodeBig(block)) != nil || len(raw) != 32 {
		return 0, invalid
	}
	decimals := new(big.Int).SetBytes(raw)
	if !decimals.IsUint64() || decimals.Sign() <= 0 || decimals.Uint64() > 18 {
		return 0, invalid
	}
	return uint8(decimals.Uint64()), nil
}

func v3TokenAsset(symbol string) (common.Address, string, common.Hash, string, string, error) {
	switch symbol {
	case "usdt":
		return common.HexToAddress(v3TokenUSDT), "paymentTokenConfigs(address)", common.Hash{}, "configurePaymentToken(address,bool)", "PaymentTokenConfigured(address,bool,uint8)", nil
	case "testcat":
		return common.HexToAddress(v3TokenTESTCAT), "projectTokenConfigs(address)", common.Hash{}, "configureProjectToken(address,bool)", "ProjectTokenConfigured(address,bool,uint8)", nil
	default:
		return common.Address{}, "", common.Hash{}, "", "", errors.New("unsupported V3 token")
	}
}

func verifyV3TokenSafeTransaction(tx *types.Transaction, receipt *types.Receipt, token common.Address, method, event string) error {
	return verifyV3TokenSafeTransactionDecimals(tx, receipt, token, method, event, 18)
}

func verifyV3TokenSafeTransactionDecimals(tx *types.Transaction, receipt *types.Receipt, token common.Address, method, event string, decimals uint8) error {
	// Structural receipt check only. The public receipt entrypoints independently
	// authenticate tx.To against the block-specific on-chain owner before use.
	invalid := errors.New("original Owner Safe transaction unavailable")
	if tx == nil || receipt == nil || tx.To() == nil || *tx.To() == (common.Address{}) || tx.Value().Sign() != 0 ||
		tx.ChainId().Cmp(big.NewInt(56)) != 0 || tx.Hash() != receipt.TxHash || receipt.Status != types.ReceiptStatusSuccessful ||
		receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Sign() <= 0 || receipt.BlockHash == (common.Hash{}) {
		return invalid
	}
	if _, err := types.Sender(types.LatestSignerForChainID(big.NewInt(56)), tx); err != nil {
		return invalid
	}
	parsed, err := abi.JSON(strings.NewReader(v3TokenSafeABI))
	if err != nil || len(tx.Data()) < 4 || !bytes.Equal(tx.Data()[:4], parsed.Methods["execTransaction"].ID) {
		return invalid
	}
	values, err := parsed.Methods["execTransaction"].Inputs.Unpack(tx.Data()[4:])
	if err != nil || len(values) != 10 {
		return invalid
	}
	encoded, err := parsed.Methods["execTransaction"].Inputs.Pack(values...)
	if err != nil || !bytes.Equal(encoded, tx.Data()[4:]) {
		return invalid
	}
	to, okTo := values[0].(common.Address)
	value, okValue := values[1].(*big.Int)
	inner, okInner := values[2].([]byte)
	op, okOp := values[3].(uint8)
	signatures, okSig := values[9].([]byte)
	if !okTo || to != common.HexToAddress(v3TokenProxy) || !okValue || value.Sign() != 0 ||
		!okInner || !bytes.Equal(inner, v3TokenCallData(method, &token, true)) || !okOp || op != 0 ||
		!okSig || len(signatures) < 130 {
		return invalid
	}
	var successes, configured int
	for _, log := range receipt.Logs {
		if log == nil || log.Removed || log.TxHash != receipt.TxHash || log.BlockHash != receipt.BlockHash || log.BlockNumber != receipt.BlockNumber.Uint64() || len(log.Topics) == 0 {
			return invalid
		}
		// Safe deployments may index the Safe tx hash (topic[1]) or emit it
		// unindexed in data[0:32]. In both layouts payment is a zero word.
		// Never count a topic-only match or a failed/noncanonical receipt.
		if log.Address == *tx.To() && log.Topics[0] == v3TokenSafeSuccess &&
			((len(log.Topics) == 2 && log.Topics[1] != (common.Hash{}) && len(log.Data) == 32 && new(big.Int).SetBytes(log.Data).Sign() == 0) ||
				(len(log.Topics) == 1 && len(log.Data) == 64 && common.BytesToHash(log.Data[:32]) != (common.Hash{}) && new(big.Int).SetBytes(log.Data[32:]).Sign() == 0)) {
			successes++
		}
		if log.Address == common.HexToAddress(v3TokenProxy) && log.Topics[0] == crypto.Keccak256Hash([]byte(event)) && len(log.Topics) == 2 && common.BytesToAddress(log.Topics[1].Bytes()[12:]) == token && len(log.Data) == 64 && new(big.Int).SetBytes(log.Data[:32]).Cmp(big.NewInt(1)) == 0 && new(big.Int).SetBytes(log.Data[32:]).Cmp(new(big.Int).SetUint64(uint64(decimals))) == 0 {
			configured++
		}
	}
	if successes != 1 || configured != 1 {
		return invalid
	}
	return nil
}

func v3TokenReadAt(ctx context.Context, rpc *ethclient.Client, symbol string, height uint64, simulate bool) (V3TokenGateAsset, error) {
	var empty V3TokenGateAsset
	proxy, implementation := common.HexToAddress(v3TokenProxy), common.HexToAddress(v3TokenImplementation)
	block := new(big.Int).SetUint64(height)
	owner, ownerErr := v3CurrentOwnerAt(ctx, rpc, block)
	if ownerErr != nil {
		return empty, ownerErr
	}
	chain, err := rpc.ChainID(ctx)
	if err != nil || chain.Cmp(big.NewInt(56)) != 0 {
		return empty, errors.New("V3 chain identity mismatch")
	}
	code, err := rpc.CodeAt(ctx, proxy, block)
	if err != nil || crypto.Keccak256Hash(code) != common.HexToHash(v3TokenProxyHash) {
		return empty, errors.New("V3 proxy code mismatch")
	}
	code, err = rpc.CodeAt(ctx, implementation, block)
	if err != nil || crypto.Keccak256Hash(code) != common.HexToHash(v3TokenImplementationHash) {
		return empty, errors.New("V3 implementation code mismatch")
	}
	slot, err := rpc.StorageAt(ctx, proxy, v3ImplementationSlot, block)
	if err != nil || len(slot) != 32 || !bytes.Equal(slot[:12], make([]byte, 12)) || common.BytesToAddress(slot[12:]) != implementation {
		return empty, errors.New("V3 implementation slot mismatch")
	}
	owned, err := v3TokenCall(ctx, rpc, proxy, "owner()", nil, nil, block)
	if err != nil || len(owned) != 32 || !bytes.Equal(owned[:12], make([]byte, 12)) || common.BytesToAddress(owned[12:]) != owner {
		return empty, errors.New("V3 Owner Safe mismatch")
	}
	token, getter, hash, method, _, err := v3TokenAsset(symbol)
	if err != nil {
		return empty, err
	}
	if err := verifyV3AssetAddress(symbol, token, hash); err != nil {
		return empty, err
	}
	if _, err := readV3TokenDecimals(ctx, rpc.Client(), token, block); err != nil {
		return empty, err
	}
	raw, err := v3TokenCall(ctx, rpc, proxy, getter, &token, nil, block)
	if err != nil || len(raw) != 64 {
		return empty, errors.New("V3 token config unavailable")
	}
	allowed := new(big.Int).SetBytes(raw[:32])
	configured := new(big.Int).SetBytes(raw[32:])
	if allowed.Cmp(big.NewInt(1)) > 0 || configured.Cmp(big.NewInt(18)) > 0 || (allowed.Sign() != 0 && configured.Cmp(big.NewInt(18)) != 0) {
		return empty, errors.New("V3 token config malformed")
	}
	result := V3TokenGateAsset{Symbol: strings.ToUpper(symbol), Token: token.Hex(), Method: method, Allowed: allowed.Sign() == 1, Decimals: uint8(configured.Uint64()), Simulation: "not_requested"}
	if simulate && !result.Allowed {
		_, err = v3TokenCall(ctx, rpc, proxy, method, &token, &owner, block)
		if err != nil {
			return empty, errors.New("Owner Safe simulation failed")
		}
		result.Simulation = "passed"
	}
	return result, nil
}

// New project tokens are selected by address, not by a global TESTCAT slot.
// This is an Owner Safe handoff, never an automatic allowlist mutation.
func readV3ProjectTokenAt(ctx context.Context, rpc *ethclient.Client, token common.Address, height uint64) (V3TokenGateAsset, error) {
	invalid := errors.New("V3 project token identity unavailable")
	if token == (common.Address{}) || token == common.HexToAddress(v3TokenUSDT) {
		return V3TokenGateAsset{}, invalid
	}
	block := new(big.Int).SetUint64(height)
	proxy, implementation := common.HexToAddress(v3TokenProxy), common.HexToAddress(v3TokenImplementation)
	owner, ownerErr := v3CurrentOwnerAt(ctx, rpc, block)
	if ownerErr != nil {
		return V3TokenGateAsset{}, ownerErr
	}
	chain, err := rpc.ChainID(ctx)
	if err != nil || chain == nil || chain.Cmp(big.NewInt(56)) != 0 {
		return V3TokenGateAsset{}, invalid
	}
	code, err := rpc.CodeAt(ctx, proxy, block)
	if err != nil || crypto.Keccak256Hash(code) != common.HexToHash(v3TokenProxyHash) {
		return V3TokenGateAsset{}, invalid
	}
	code, err = rpc.CodeAt(ctx, implementation, block)
	if err != nil || crypto.Keccak256Hash(code) != common.HexToHash(v3TokenImplementationHash) {
		return V3TokenGateAsset{}, invalid
	}
	slot, err := rpc.StorageAt(ctx, proxy, v3ImplementationSlot, block)
	if err != nil || len(slot) != 32 || !bytes.Equal(slot[:12], make([]byte, 12)) || common.BytesToAddress(slot[12:]) != implementation {
		return V3TokenGateAsset{}, invalid
	}
	owned, err := v3TokenCall(ctx, rpc, proxy, "owner()", nil, nil, block)
	if err != nil || len(owned) != 32 || !bytes.Equal(owned[:12], make([]byte, 12)) || common.BytesToAddress(owned[12:]) != owner {
		return V3TokenGateAsset{}, invalid
	}
	if _, _, err := v3TokenSafeAt(ctx, rpc, block); err != nil {
		return V3TokenGateAsset{}, invalid
	}
	decimalsValue, err := readV3ProjectTokenDecimals(ctx, rpc.Client(), token, block)
	if err != nil {
		return V3TokenGateAsset{}, invalid
	}
	decimals := new(big.Int).SetUint64(uint64(decimalsValue))
	config, err := v3TokenCall(ctx, rpc, proxy, "projectTokenConfigs(address)", &token, nil, block)
	if err != nil || len(config) != 64 {
		return V3TokenGateAsset{}, invalid
	}
	allowed, configured := new(big.Int).SetBytes(config[:32]), new(big.Int).SetBytes(config[32:])
	if allowed.Cmp(big.NewInt(1)) > 0 || (allowed.Sign() != 0 && configured.Cmp(decimals) != 0) ||
		(allowed.Sign() == 0 && configured.Sign() != 0 && configured.Cmp(decimals) != 0) {
		return V3TokenGateAsset{}, invalid
	}
	result := V3TokenGateAsset{Symbol: "PROJECT", Token: token.Hex(), Method: "configureProjectToken(address,bool)", Allowed: allowed.Sign() == 1, Decimals: decimalsValue, Simulation: "not_requested"}
	if !result.Allowed {
		if _, err := v3TokenCall(ctx, rpc, proxy, result.Method, &token, &owner, block); err != nil {
			return V3TokenGateAsset{}, invalid
		}
		result.Simulation = "passed"
	}
	return result, nil
}

type V3ProjectTokenGateStatus struct {
	ChainID       uint64           `json:"chain_id"`
	Proxy         string           `json:"proxy"`
	OwnerSafe     string           `json:"owner_safe"`
	Block         uint64           `json:"block"`
	BlockHash     string           `json:"block_hash"`
	Confirmations uint64           `json:"confirmations"`
	Asset         V3TokenGateAsset `json:"asset"`
}

func ReadV3ProjectTokenGateStatus(address string) (*V3ProjectTokenGateStatus, error) {
	if !common.IsHexAddress(address) || common.HexToAddress(address) == (common.Address{}) || common.HexToAddress(address) == common.HexToAddress(v3TokenUSDT) {
		return nil, errors.New("V3 project token address invalid")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	rpc, err := v3TokenGateRPC(ctx)
	if err != nil {
		return nil, err
	}
	defer rpc.Close()
	head, err := rpc.HeaderByNumber(ctx, nil)
	if err != nil || head == nil || head.Number == nil || !head.Number.IsUint64() || head.Number.Uint64() < 15 {
		return nil, errors.New("V3 confirmed head unavailable")
	}
	height := head.Number.Uint64() - 14
	anchor, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(height))
	if err != nil || anchor == nil || anchor.Number == nil || !anchor.Number.IsUint64() || anchor.Number.Uint64() != height {
		return nil, errors.New("V3 canonical block unavailable")
	}
	asset, err := readV3ProjectTokenAt(ctx, rpc, common.HexToAddress(address), height)
	if err != nil {
		return nil, err
	}
	again, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(height))
	if err != nil || again == nil || again.Hash() != anchor.Hash() {
		return nil, errors.New("V3 canonical block changed")
	}
	owner, err := v3CurrentOwnerAt(ctx, rpc, new(big.Int).SetUint64(height))
	if err != nil {
		return nil, err
	}
	return &V3ProjectTokenGateStatus{ChainID: 56, Proxy: v3TokenProxy, OwnerSafe: owner.Hex(), Block: height, BlockHash: anchor.Hash().Hex(), Confirmations: 15, Asset: asset}, nil
}

// Only the two operator-approved BSC token addresses are accepted. Neither
// preflight nor receipt verification fetches either token's bytecode. The
// pinned V3 Proxy/Implementation and Owner Safe still retain code checks.
func verifyV3AssetAddress(symbol string, token common.Address, expected common.Hash) error {
	if expected != (common.Hash{}) {
		return errors.New("V3 token code policy unavailable")
	}
	switch symbol {
	case "usdt":
		if token == common.HexToAddress(v3TokenUSDT) {
			return nil
		}
	case "testcat":
		if token == common.HexToAddress(v3TokenTESTCAT) {
			return nil
		}
	}
	return errors.New("V3 token address mismatch")
}

func v3CurrentOwnerAt(ctx context.Context, rpc idov3whitelist.PolicyRPC, block *big.Int) (common.Address, error) {
	expected := strings.TrimSpace(viper.GetString("idoV3TokenAdmission.ownerSafeCodeHash"))
	if !v3TokenTxHash.MatchString(expected) {
		return common.Address{}, errors.New("Owner Safe code identity unavailable")
	}
	return idov3whitelist.ReadOwnerSafeAt(ctx, rpc, common.HexToAddress(v3TokenProxy), common.HexToAddress(v3TokenImplementation), common.HexToHash(expected), block)
}

func v3TokenSafeAt(ctx context.Context, rpc *ethclient.Client, block *big.Int) (uint64, string, error) {
	// nil means latest: pin every Safe getter to one concrete block.
	if block == nil {
		h, e := rpc.HeaderByNumber(ctx, nil)
		if e != nil || h == nil || h.Number == nil {
			return 0, "", errors.New("Owner Safe block unavailable")
		}
		block = new(big.Int).Set(h.Number)
	}
	safe, err := v3CurrentOwnerAt(ctx, rpc, block)
	if err != nil {
		return 0, "", err
	}
	code, err := rpc.CodeAt(ctx, safe, block)
	expectedHash := strings.TrimSpace(viper.GetString("idoV3TokenAdmission.ownerSafeCodeHash"))
	if err != nil || len(code) == 0 || !v3TokenTxHash.MatchString(expectedHash) ||
		crypto.Keccak256Hash(code) != common.HexToHash(expectedHash) {
		return 0, "", errors.New("Owner Safe code identity unavailable")
	}
	raw, err := v3TokenCall(ctx, rpc, safe, "getThreshold()", nil, nil, block)
	threshold, e := v3TokenWord(raw)
	if err != nil || e != nil || threshold.Cmp(big.NewInt(2)) != 0 {
		return 0, "", errors.New("Owner Safe threshold mismatch")
	}
	ownerABI, err := abi.JSON(strings.NewReader(`[{"type":"function","name":"getOwners","inputs":[],"outputs":[{"type":"address[]"}]}]`))
	if err != nil {
		return 0, "", errors.New("Owner Safe owners ABI unavailable")
	}
	raw, err = v3TokenCall(ctx, rpc, safe, "getOwners()", nil, nil, block)
	if err != nil {
		return 0, "", errors.New("Owner Safe owners unavailable")
	}
	values, err := ownerABI.Methods["getOwners"].Outputs.Unpack(raw)
	if err != nil || len(values) != 1 {
		return 0, "", errors.New("Owner Safe owners malformed")
	}
	owners, ok := values[0].([]common.Address)
	if !ok || len(owners) != 3 || owners[0] == (common.Address{}) || owners[1] == (common.Address{}) ||
		owners[2] == (common.Address{}) || owners[0] == owners[1] || owners[0] == owners[2] || owners[1] == owners[2] {
		return 0, "", errors.New("Owner Safe owner set mismatch")
	}
	raw, err = v3TokenCall(ctx, rpc, safe, "nonce()", nil, nil, block)
	nonce, e := v3TokenWord(raw)
	if err != nil || e != nil {
		return 0, "", errors.New("Owner Safe nonce unavailable")
	}
	return 2, nonce.String(), nil
}

func ReadV3TokenGateStatus() (*V3TokenGateStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	rpc, err := v3TokenGateRPC(ctx)
	if err != nil {
		return nil, err
	}
	defer rpc.Close()
	head, err := rpc.HeaderByNumber(ctx, nil)
	if err != nil || head == nil || head.Number == nil || !head.Number.IsUint64() || head.Number.Uint64() < 15 {
		return nil, errors.New("V3 confirmed head unavailable")
	}
	height := head.Number.Uint64() - 14
	block := new(big.Int).SetUint64(height)
	first, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || first == nil || first.Number == nil || first.Number.Cmp(block) != 0 {
		return nil, errors.New("V3 canonical block unavailable")
	}
	threshold, _, err := v3TokenSafeAt(ctx, rpc, block)
	if err != nil {
		return nil, err
	}
	// The confirmed snapshot authenticates the Safe; show its current nonce
	// separately so an operator never builds a proposal with a stale nonce.
	_, nonce, err := v3TokenSafeAt(ctx, rpc, nil)
	if err != nil {
		return nil, err
	}
	usdt, err := v3TokenReadAt(ctx, rpc, "usdt", height, true)
	if err != nil {
		return nil, fmt.Errorf("V3 USDT commercial RPC preflight: %w", err)
	}
	cat, err := v3TokenReadAt(ctx, rpc, "testcat", height, true)
	if err != nil {
		return nil, fmt.Errorf("V3 TESTCAT commercial RPC preflight: %w", err)
	}
	after, err := rpc.HeaderByNumber(ctx, block)
	if err != nil || after == nil || first.Hash() != after.Hash() {
		return nil, errors.New("V3 canonical block changed")
	}
	owner, err := v3CurrentOwnerAt(ctx, rpc, block)
	if err != nil {
		return nil, err
	}
	currentOwner, err := v3CurrentOwnerAt(ctx, rpc, nil)
	if err != nil || currentOwner != owner {
		return nil, errors.New("Owner Safe changed; refresh proposal")
	}
	return &V3TokenGateStatus{ChainID: 56, Proxy: v3TokenProxy, OwnerSafe: owner.Hex(), SafeThreshold: threshold, SafeNonce: nonce, Block: height, BlockHash: first.Hash().Hex(), Confirmations: 15, USDT: usdt, TESTCAT: cat}, nil
}

func VerifyV3TokenGateReceipt(symbol, hash string) (*V3TokenGateReceipt, error) {
	if !v3TokenTxHash.MatchString(hash) {
		return nil, errors.New("transaction hash invalid")
	}
	token, _, _, method, event, err := v3TokenAsset(symbol)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	rpc, err := v3TokenGateRPC(ctx)
	if err != nil {
		return nil, err
	}
	defer rpc.Close()
	chainID, chainErr := rpc.ChainID(ctx)
	if chainErr != nil || chainID.Cmp(big.NewInt(56)) != 0 {
		return nil, errors.New("V3 chain identity mismatch")
	}
	receipt, err := rpc.TransactionReceipt(ctx, common.HexToHash(hash))
	if err != nil || receipt == nil || receipt.TxHash != common.HexToHash(hash) || receipt.Status != types.ReceiptStatusSuccessful || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() {
		return nil, errors.New("successful V3 receipt unavailable")
	}
	transaction, pending, err := rpc.TransactionByHash(ctx, common.HexToHash(hash))
	owner, ownerErr := v3CurrentOwnerAt(ctx, rpc, receipt.BlockNumber)
	if err != nil || pending || ownerErr != nil || transaction == nil || transaction.To() == nil || *transaction.To() != owner || verifyV3TokenSafeTransaction(transaction, receipt, token, method, event) != nil {
		return nil, errors.New("original Owner Safe transaction unavailable")
	}
	head, err := rpc.HeaderByNumber(ctx, nil)
	if err != nil || head == nil || head.Number == nil || !head.Number.IsUint64() || head.Number.Uint64() < receipt.BlockNumber.Uint64() || head.Number.Uint64()-receipt.BlockNumber.Uint64() < 14 {
		return nil, errors.New("V3 receipt lacks 15 confirmations")
	}
	anchor, err := rpc.HeaderByNumber(ctx, receipt.BlockNumber)
	if err != nil || anchor == nil || anchor.Hash() != receipt.BlockHash {
		return nil, errors.New("V3 receipt not canonical")
	}
	if _, _, err := v3TokenSafeAt(ctx, rpc, receipt.BlockNumber); err != nil {
		return nil, err
	}
	asset, err := v3TokenReadAt(ctx, rpc, symbol, receipt.BlockNumber.Uint64(), false)
	if err != nil || !asset.Allowed || asset.Decimals != 18 {
		return nil, errors.New("V3 token getter does not match receipt")
	}
	// A historical success is not current admission: the Safe may have
	// disabled this token later. Check the independently confirmed state.
	latest, err := v3TokenReadAt(ctx, rpc, symbol, head.Number.Uint64()-14, false)
	if err != nil || !latest.Allowed || latest.Decimals != 18 {
		return nil, errors.New("V3 token no longer admitted")
	}
	return &V3TokenGateReceipt{Asset: symbol, TxHash: hash, Block: receipt.BlockNumber.Uint64(), BlockHash: receipt.BlockHash.Hex(), Confirmations: head.Number.Uint64() - receipt.BlockNumber.Uint64() + 1, Allowed: true, Decimals: 18}, nil
}

// VerifyV3ProjectTokenReceipt binds a caller-selected token to the exact
// Owner Safe execution and its still-admitted confirmed chain state.
func VerifyV3ProjectTokenReceipt(address, hash string) (*V3TokenGateReceipt, error) {
	invalid := errors.New("V3 project token receipt unavailable")
	if !common.IsHexAddress(address) || common.HexToAddress(address) == (common.Address{}) ||
		common.HexToAddress(address) == common.HexToAddress(v3TokenUSDT) || !v3TokenTxHash.MatchString(hash) {
		return nil, invalid
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	rpc, err := v3TokenGateRPC(ctx)
	if err != nil {
		return nil, invalid
	}
	defer rpc.Close()
	token := common.HexToAddress(address)
	chain, err := rpc.ChainID(ctx)
	if err != nil || chain == nil || chain.Cmp(big.NewInt(56)) != 0 {
		return nil, invalid
	}
	receipt, err := rpc.TransactionReceipt(ctx, common.HexToHash(hash))
	if err != nil || receipt == nil || receipt.TxHash != common.HexToHash(hash) || receipt.Status != types.ReceiptStatusSuccessful ||
		receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() {
		return nil, invalid
	}
	transaction, pending, err := rpc.TransactionByHash(ctx, common.HexToHash(hash))
	owner, ownerErr := v3CurrentOwnerAt(ctx, rpc, receipt.BlockNumber)
	if err != nil || pending || ownerErr != nil || transaction == nil || transaction.To() == nil || *transaction.To() != owner {
		return nil, invalid
	}
	head, err := rpc.HeaderByNumber(ctx, nil)
	if err != nil || head == nil || head.Number == nil || !head.Number.IsUint64() ||
		head.Number.Uint64() < receipt.BlockNumber.Uint64() || head.Number.Uint64()-receipt.BlockNumber.Uint64() < 14 {
		return nil, invalid
	}
	anchor, err := rpc.HeaderByNumber(ctx, receipt.BlockNumber)
	if err != nil || anchor == nil || anchor.Hash() != receipt.BlockHash {
		return nil, invalid
	}
	if _, _, err := v3TokenSafeAt(ctx, rpc, receipt.BlockNumber); err != nil {
		return nil, invalid
	}
	atReceipt, err := readV3ProjectTokenAt(ctx, rpc, token, receipt.BlockNumber.Uint64())
	if err != nil || !atReceipt.Allowed || atReceipt.Decimals == 0 ||
		verifyV3TokenSafeTransactionDecimals(transaction, receipt, token, "configureProjectToken(address,bool)", "ProjectTokenConfigured(address,bool,uint8)", atReceipt.Decimals) != nil {
		return nil, invalid
	}
	confirmedHeight := head.Number.Uint64() - 14
	confirmed, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(confirmedHeight))
	if err != nil || confirmed == nil || confirmed.Number == nil || !confirmed.Number.IsUint64() || confirmed.Number.Uint64() != confirmedHeight {
		return nil, invalid
	}
	current, err := readV3ProjectTokenAt(ctx, rpc, token, confirmedHeight)
	if err != nil || !current.Allowed || current.Decimals != atReceipt.Decimals {
		return nil, invalid
	}
	stable, err := rpc.HeaderByNumber(ctx, new(big.Int).SetUint64(confirmedHeight))
	if err != nil || stable == nil || stable.Hash() != confirmed.Hash() {
		return nil, invalid
	}
	return &V3TokenGateReceipt{Asset: "project", TxHash: hash, Block: receipt.BlockNumber.Uint64(), BlockHash: receipt.BlockHash.Hex(),
		Confirmations: head.Number.Uint64() - receipt.BlockNumber.Uint64() + 1, Allowed: true, Decimals: current.Decimals}, nil
}

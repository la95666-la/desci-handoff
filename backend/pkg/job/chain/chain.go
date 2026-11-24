package chain

import (
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"

	"joincare_server/pkg/common/auth/aes"
	"joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/idoreconciliation"
	"joincare_server/pkg/job/flag"
)

var client *chain.Client
var v2SnapshotClient idoreconciliation.Reader
var v2ContractAddress common.Address
var v2StartBlock uint64
var v2ImplementationAddress common.Address
var v2ImplementationCodeHash common.Hash
var v2CoreOperatorAddress common.Address
var v2PaymentTokenAllowed map[string]bool
var v3ShadowSource *V3ShadowSource
var v1Enabled bool

// skipV2HistoricalRuntimeVerify 允许受信配置跳过历史块 implementation 重验（公共 RPC 限制）。
var skipV2HistoricalRuntimeVerify bool

func NewClient() error {
	var err error
	v2ContractAddress = common.Address{}
	v2StartBlock = 0
	v2ImplementationAddress = common.Address{}
	v2ImplementationCodeHash = common.Hash{}
	v2CoreOperatorAddress = common.Address{}
	v2PaymentTokenAllowed = nil
	v2SnapshotClient = nil
	v3ShadowSource = nil
	if viper.GetString("application.jobMode") == "ido-v3" {
		client = nil
		v1Enabled = false
		if viper.GetString("chain.project.v1.enabled") != "false" || !viper.GetBool("idoV3Approved.enabled") || viper.GetBool("idoV3Scanner.enabled") || viper.GetUint64("chain.project.chainID") != 56 || viper.GetString("chain.project.private") != "" {
			return errors.New("V3-only Job identity unavailable")
		}
		v3ShadowSource, err = readV3ShadowSource(viper.GetViper(), 56, common.HexToAddress(viper.GetString("chain.project.contractAddress")), common.HexToAddress(viper.GetString("chain.project.v2.contractAddress")))
		if err != nil || v3ShadowSource == nil {
			return errors.New("V3-only source identity unavailable")
		}
		// No V2 client/RPC/signer initialization. V3 owns its read-only RPC lane.
		return nil
	}
	prefix := "chain.project"
	v1Configured := strings.TrimSpace(strings.ToLower(viper.GetString(prefix + ".v1.enabled")))
	switch v1Configured {
	case "", "true":
		v1Enabled = true
	case "false":
		v1Enabled = false
	default:
		return errors.New("chain.project.v1.enabled must be true or false")
	}
	skipV2HistoricalRuntimeVerify = strings.EqualFold(strings.TrimSpace(viper.GetString(prefix+".v2.skipRuntimeVerify")), "true")
	// 使用你的钱包私钥
	jobPrivate := strings.TrimSpace(viper.GetString(prefix + ".jobPrivate"))
	if jobPrivate == "" {
		return errors.New("chain.project.jobPrivate is required for durable lifecycle transaction recovery")
	}
	privateKey, err := decodePrivateKey(jobPrivate, flag.Token())
	if err != nil {
		return err
	}
	keeperConfig := strings.TrimSpace(viper.GetString(prefix + ".private"))
	if !v1Enabled && keeperConfig != "" {
		return errors.New("V2-only Job must not receive Core relayer private key")
	}
	if keeperConfig != "" {
		keeperKey, keeperErr := decodePrivateKey(keeperConfig, flag.Token())
		if keeperErr != nil {
			return fmt.Errorf("decode project core relayer for sender separation: %w", keeperErr)
		}
		if crypto.PubkeyToAddress(keeperKey.PublicKey) == crypto.PubkeyToAddress(privateKey.PublicKey) {
			return errors.New("project job relayer must use a different address from the core relayer")
		}
	}
	// 从私钥派生后端钱包地址（这是交易的发送者地址，用于查询 nonce）
	contractAddressRaw := strings.TrimSpace(viper.GetString(prefix + ".contractAddress"))
	if v1Enabled && (!common.IsHexAddress(contractAddressRaw) || common.HexToAddress(contractAddressRaw) == (common.Address{})) {
		return errors.New("chain.project.contractAddress is required when V1 is enabled")
	}
	contractAddress := common.HexToAddress(contractAddressRaw)
	chainID := big.NewInt(viper.GetInt64(prefix + ".chainID"))

	v2AddressRaw := strings.TrimSpace(viper.GetString(prefix + ".v2.contractAddress"))
	if !v1Enabled && v2AddressRaw == "" {
		return errors.New("chain.project.v2.contractAddress is required when V1 is disabled")
	}
	baseAddress := contractAddress
	if !v1Enabled {
		baseAddress = common.HexToAddress(v2AddressRaw)
	}
	if v1Enabled {
		client, err = chain.NewClient(viper.GetString(prefix+".url"), privateKey, baseAddress, chainID)
	} else {
		// V2-only lifecycle and durable recovery must pack V2 calldata, not
		// the legacy ABI, even though both contracts expose endProject.
		client, err = chain.NewIDOV2Client(viper.GetString(prefix+".url"), privateKey, baseAddress, chainID)
	}
	if err != nil {
		return err
	}
	verifyCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	actualChainID, err := client.Client.ChainID(verifyCtx)
	if err != nil {
		return fmt.Errorf("query project RPC chain ID: %w", err)
	}
	if actualChainID.Cmp(chainID) != 0 {
		return fmt.Errorf("project RPC chain ID mismatch: configured=%s actual=%s", chainID, actualChainID)
	}
	// reorg 确认深度：默认 15 块（BSC 约 45s，可覆盖常见短重组）；
	// 本地 hardhat（31337）置 0，否则 automining 下事件处理会停摆；
	// 可用 chain.project.confirmations 显式覆盖
	confirmations, err := chain.ResolveConfirmations(chainID.Uint64(), viper.GetString(prefix+".confirmations"))
	if err != nil {
		return err
	}
	client.Confirmations = confirmations
	v2StartRaw := strings.TrimSpace(viper.GetString(prefix + ".v2.startBlock"))
	if v2AddressRaw == "" && v2StartRaw != "" {
		return fmt.Errorf("chain.project.v2.contractAddress is required when v2.startBlock is configured")
	}
	if v2AddressRaw != "" {
		if !common.IsHexAddress(v2AddressRaw) || common.HexToAddress(v2AddressRaw) == (common.Address{}) {
			return fmt.Errorf("chain.project.v2.contractAddress is invalid")
		}
		if v2StartRaw == "" {
			return fmt.Errorf("chain.project.v2.startBlock is required when V2 is enabled")
		}
		v2ContractAddress = common.HexToAddress(v2AddressRaw)
		implementationRaw := strings.TrimSpace(viper.GetString(prefix + ".v2.implementationAddress"))
		implementationHashRaw := strings.TrimSpace(viper.GetString(prefix + ".v2.implementationCodeHash"))
		if !common.IsHexAddress(implementationRaw) || common.HexToAddress(implementationRaw) == (common.Address{}) {
			return fmt.Errorf("chain.project.v2.implementationAddress is invalid")
		}
		if !isExactHash(implementationHashRaw) || common.HexToHash(implementationHashRaw) == (common.Hash{}) {
			return fmt.Errorf("chain.project.v2.implementationCodeHash is invalid")
		}
		v2ImplementationAddress = common.HexToAddress(implementationRaw)
		v2ImplementationCodeHash = common.HexToHash(implementationHashRaw)
		if v1Enabled && v2ContractAddress == contractAddress {
			return fmt.Errorf("V1 and V2 contract addresses must be different")
		}
		v2StartBlock, err = strconv.ParseUint(v2StartRaw, 10, 64)
		if err != nil || v2StartBlock == 0 {
			return fmt.Errorf("chain.project.v2.startBlock must be a positive integer")
		}
		v2Code, codeErr := client.Client.CodeAt(verifyCtx, v2ContractAddress, nil)
		if codeErr != nil {
			return fmt.Errorf("query V2 contract bytecode: %w", codeErr)
		}
		if len(v2Code) == 0 {
			return fmt.Errorf("V2 contract has no bytecode at %s", v2ContractAddress.Hex())
		}
		if err := verifyV2ContractIdentity(verifyCtx); err != nil {
			return err
		}
		client.EventAddresses = []common.Address{v2ContractAddress}
		if v1Enabled {
			client.EventAddresses = []common.Address{contractAddress, v2ContractAddress}
		}
		v2SnapshotClient, err = chain.NewIDOV2SnapshotView(client, v2ContractAddress)
		if err != nil {
			return fmt.Errorf("create V2 snapshot RPC view: %w", err)
		}
	}
	// Validate the isolated V3 identity but never add it to EventAddresses.
	// Scanner registration remains blocked until dedicated reorg-aware V3
	// financial materialization has been reviewed and implemented.
	v3ShadowSource, err = readV3ShadowSource(viper.GetViper(), chainID.Uint64(), contractAddress, v2ContractAddress)
	if err != nil {
		return err
	}
	return nil
}

func verifyV2ContractIdentity(ctx context.Context) error {
	if err := VerifyV2RuntimeAt(ctx, nil); err != nil {
		return err
	}
	return verifyLatestV2Governance(ctx)
}

func VerifyV2RuntimeAt(ctx context.Context, blockNumber *big.Int) error {
	if blockNumber != nil && skipV2HistoricalRuntimeVerify {
		// 公共 RPC（bsc-dataseed/publicnode/…）普遍不支持历史块的 eth_getStorageAt
		//（missing trie node / not supported / archive token），而扫描器逐批调用此处会导致
		// 永久卡在追块阶段。实现身份在 registerConfiguredV2 时已用最新块完整核验
		//（implementation + codeHash + governance），每批重验只是防“扫描窗口内被换实现”的
		// 额外保险；在已注册 + reorg 防护齐备的场景下，允许受信配置显式跳过历史校验。
		// 生产上线建议运行全节点后关闭本开关（保留完整校验）。
		return nil
	}
	implementationSlot := common.HexToHash("0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc")
	stored, err := client.Client.StorageAt(ctx, v2ContractAddress, implementationSlot, blockNumber)
	if err != nil {
		return fmt.Errorf("read V2 ERC1967 implementation slot: %w", err)
	}
	implementation := common.BytesToAddress(stored)
	if implementation == (common.Address{}) {
		return errors.New("V2 address is not an ERC1967 proxy")
	}
	if implementation != v2ImplementationAddress {
		return fmt.Errorf("V2 implementation mismatch: approved=%s actual=%s", v2ImplementationAddress.Hex(), implementation.Hex())
	}
	implementationCode, err := client.Client.CodeAt(ctx, implementation, blockNumber)
	if err != nil || len(implementationCode) == 0 {
		return fmt.Errorf("V2 implementation bytecode is missing: %w", err)
	}
	actualCodeHash := crypto.Keccak256Hash(implementationCode)
	if actualCodeHash != v2ImplementationCodeHash {
		return fmt.Errorf("V2 implementation runtime code hash mismatch: approved=%s actual=%s", v2ImplementationCodeHash.Hex(), actualCodeHash.Hex())
	}
	return nil
}

func verifyLatestV2Governance(ctx context.Context) error {
	for signature, expected := range map[string]uint64{"BASIS_POINTS()": 10_000, "MAX_FEE_RATE()": 1_000, "MAX_SUPPORTED_DECIMALS()": 18} {
		value, callErr := callV2Uint(ctx, signature, nil)
		if callErr != nil || value != expected {
			return fmt.Errorf("V2 %s identity mismatch: got=%d err=%v", signature, value, callErr)
		}
	}
	for _, signature := range []string{"owner()", "feeReceiver()", "operator()"} {
		result, callErr := callV2(ctx, signature, nil)
		if callErr != nil || len(result) != 32 || common.BytesToAddress(result) == (common.Address{}) {
			return fmt.Errorf("V2 %s getter is invalid: %v", signature, callErr)
		}
	}
	// Core creates/activates V2 projects using the contract's single operator.
	// The Job's independent signer owns only its own durable outbox nonce lane;
	// endProject is permissionless, so it must NOT be assigned Core's operator.
	approvedOperator := strings.TrimSpace(viper.GetString("chain.project.v2.coreOperatorAddress"))
	operatorResult, err := callV2(ctx, "operator()", nil)
	if err != nil {
		return fmt.Errorf("query V2 core operator: %w", err)
	}
	if len(operatorResult) != 32 {
		return errors.New("invalid V2 core operator response")
	}
	actualOperator := common.BytesToAddress(operatorResult)
	if err := validateV2CoreOperator(approvedOperator, client.Address, actualOperator); err != nil {
		return err
	}
	// Keep the separately admitted Core operator identity for strict matching of
	// create/activate events. The Job signer is intentionally a different nonce
	// lane and must never be substituted for the transaction sender.
	v2CoreOperatorAddress = actualOperator
	feeRate, err := callV2Uint(ctx, "successFeeRate()", nil)
	if err != nil || feeRate > 1_000 {
		return fmt.Errorf("V2 successFeeRate is invalid: value=%d err=%v", feeRate, err)
	}
	seenTokens := make(map[common.Address]string, 2)
	allowedSnapshot := make(map[string]bool, 2)
	for _, symbol := range []string{constant.PaymentTokenUSDT, constant.PaymentTokenUSDC} {
		prefix := "chain.project.v2.paymentTokens." + symbol
		addressRaw := strings.TrimSpace(viper.GetString(prefix + ".address"))
		configuredDecimals := viper.GetUint64(prefix + ".decimals")
		if !common.IsHexAddress(addressRaw) || common.HexToAddress(addressRaw) == (common.Address{}) || configuredDecimals == 0 || configuredDecimals > 18 {
			return fmt.Errorf("V2 %s trusted token configuration is invalid", symbol)
		}
		token := common.HexToAddress(addressRaw)
		if previous, exists := seenTokens[token]; exists {
			return fmt.Errorf("V2 trusted token address is shared by %s and %s", previous, symbol)
		}
		seenTokens[token] = symbol
		code, codeErr := client.Client.CodeAt(ctx, token, nil)
		if codeErr != nil || len(code) == 0 {
			return fmt.Errorf("V2 %s token has no bytecode: %v", symbol, codeErr)
		}
		actualDecimals, callErr := callAddressUint(ctx, token, "decimals()", nil)
		if callErr != nil || actualDecimals != configuredDecimals {
			return fmt.Errorf("V2 %s decimals mismatch: configured=%d actual=%d err=%v", symbol, configuredDecimals, actualDecimals, callErr)
		}
		arg := common.LeftPadBytes(token.Bytes(), 32)
		result, callErr := callV2(ctx, "paymentTokenConfigs(address)", arg)
		if callErr != nil || len(result) != 64 {
			return fmt.Errorf("V2 %s paymentTokenConfigs getter is invalid: %v", symbol, callErr)
		}
		allowedWord := new(big.Int).SetBytes(result[:32])
		if !allowedWord.IsUint64() || allowedWord.Uint64() > 1 {
			return fmt.Errorf("V2 %s allowed flag is not an ABI boolean", symbol)
		}
		allowedSnapshot[symbol] = allowedWord.Uint64() == 1
		governedDecimals := new(big.Int).SetBytes(result[32:]).Uint64()
		if governedDecimals != configuredDecimals {
			return fmt.Errorf("V2 %s governed decimals mismatch: configured=%d proxy=%d", symbol, configuredDecimals, governedDecimals)
		}
	}
	v2PaymentTokenAllowed = allowedSnapshot
	return nil
}

func validateV2CoreOperator(configured string, jobSigner, actual common.Address) error {
	if !common.IsHexAddress(configured) {
		return errors.New("chain.project.v2.coreOperatorAddress must be an explicit nonzero address")
	}
	expected := common.HexToAddress(configured)
	if expected == (common.Address{}) || expected == jobSigner || actual != expected {
		return fmt.Errorf("V2 core operator admission failed: expected=%s actual=%s job=%s", expected.Hex(), actual.Hex(), jobSigner.Hex())
	}
	return nil
}

func callV2(ctx context.Context, signature string, args []byte) ([]byte, error) {
	return callAddress(ctx, v2ContractAddress, signature, args)
}
func callV2Uint(ctx context.Context, signature string, args []byte) (uint64, error) {
	return callAddressUint(ctx, v2ContractAddress, signature, args)
}
func callAddressUint(ctx context.Context, address common.Address, signature string, args []byte) (uint64, error) {
	result, err := callAddress(ctx, address, signature, args)
	if err != nil {
		return 0, err
	}
	if len(result) != 32 {
		return 0, fmt.Errorf("unexpected getter result length %d", len(result))
	}
	value := new(big.Int).SetBytes(result)
	if !value.IsUint64() {
		return 0, errors.New("getter result exceeds uint64")
	}
	return value.Uint64(), nil
}
func callAddress(ctx context.Context, address common.Address, signature string, args []byte) ([]byte, error) {
	selector := crypto.Keccak256([]byte(signature))[:4]
	data := append(append([]byte{}, selector...), args...)
	return client.Client.CallContract(ctx, ethereum.CallMsg{To: &address, Data: data}, nil)
}

func decodePrivateKey(private, token string) (*ecdsa.PrivateKey, error) {
	private = strings.TrimSpace(private)
	plain := strings.TrimPrefix(private, "0x")
	if len(plain) == 64 {
		if privateKey, err := crypto.HexToECDSA(plain); err == nil {
			return privateKey, nil
		}
	}
	encodedKey, err := base64.StdEncoding.DecodeString(private)
	if err != nil {
		return nil, fmt.Errorf("private key not plaintext hex (64 chars) or valid base64: %w", err)
	}
	key, err := aes.DecryptAESGCM(encodedKey, []byte(token))
	if err != nil {
		return nil, err
	}
	privateKey, err := crypto.HexToECDSA(string(key))
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

func GetClient() *chain.Client {
	return client
}

func GetV2SnapshotClient() idoreconciliation.Reader { return v2SnapshotClient }

func V2Enabled() bool                          { return v2ContractAddress != (common.Address{}) }
func V2ContractAddress() common.Address        { return v2ContractAddress }
func V2StartBlock() uint64                     { return v2StartBlock }
func V2ImplementationAddress() common.Address  { return v2ImplementationAddress }
func V2ImplementationCodeHash() common.Hash    { return v2ImplementationCodeHash }
func V2CoreOperatorAddress() common.Address    { return v2CoreOperatorAddress }
func V2PaymentTokenAllowed(symbol string) bool { return v2PaymentTokenAllowed[symbol] }
func V3ShadowConfigured() bool                 { return v3ShadowSource != nil }

// MatchesV3Shadow checks the separately configured, inactive deployment
// identity against the signed scanner manifest. Neither input alone admits
// the scanner; admission must still verify signature and the persisted join.
func MatchesV3Shadow(chainID, startBlock uint64, proxy, implementation, governor common.Address, proxyHash, implementationHash common.Hash) bool {
	s := v3ShadowSource
	return s != nil && s.ChainID == chainID && s.StartBlock == startBlock && s.ProxyAddress == proxy &&
		s.ImplementationAddress == implementation && s.GovernorAddress == governor &&
		s.ProxyCodeHash == proxyHash && s.ImplementationCodeHash == implementationHash
}
func IsV2Log(address common.Address) bool { return V2Enabled() && address == v2ContractAddress }
func V1Enabled() bool                     { return v1Enabled }

func isExactHash(value string) bool {
	if len(value) != 66 || !strings.HasPrefix(value, "0x") {
		return false
	}
	for _, ch := range value[2:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", ch) {
			return false
		}
	}
	return true
}

// GetAddress 返回后端钱包地址
func GetAddress() string {
	if client == nil {
		return ""
	}
	return client.Address.Hex()
}

func Close() {
	if client != nil && client.Client != nil {
		client.Client.Close()
	}
}

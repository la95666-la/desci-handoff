package chain

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"math/bits"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/shopspring/decimal"
	"joincare_server/pkg/common/chainrpc"
)

func ResolveConfirmations(chainID uint64, configured string) (uint64, error) {
	configured = strings.TrimSpace(configured)
	if configured != "" {
		value, err := strconv.ParseUint(configured, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid project chain confirmations %q", configured)
		}
		return value, nil
	}
	if chainID == 31337 {
		return 0, nil
	}
	return 15, nil
}

func NewClient(url string, privateKey *ecdsa.PrivateKey, contractAddress common.Address, chainID *big.Int) (*Client, error) {
	return newClient(url, privateKey, contractAddress, chainID, abiJson)
}

func NewTaskClient(url string, privateKey *ecdsa.PrivateKey, contractAddress common.Address, chainID *big.Int) (*Client, error) {
	return newClient(url, privateKey, contractAddress, chainID, taskABIJson)
}

func NewIDOV2Client(url string, privateKey *ecdsa.PrivateKey, contractAddress common.Address, chainID *big.Int) (*Client, error) {
	return newClient(url, privateKey, contractAddress, chainID, idoV2ABIJson)
}

// NewIDOV3Client uses the dedicated V3 ABI and proxy identity. Sharing an
// operator key with V2 is permitted only through the common sender nonce
// fence, never by borrowing the V2 contract address or method encoder.
func NewIDOV3Client(url string, privateKey *ecdsa.PrivateKey, contractAddress common.Address, chainID *big.Int) (*Client, error) {
	return newClient(url, privateKey, contractAddress, chainID, idoV3LifecycleABIJSON)
}

var senderLocks sync.Map

func senderLock(chainID *big.Int, address common.Address) *sync.Mutex {
	key := chainID.String() + ":" + strings.ToLower(address.Hex())
	lock, _ := senderLocks.LoadOrStore(key, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func newClient(url string, privateKey *ecdsa.PrivateKey, contractAddress common.Address, chainID *big.Int, abiSource string) (*Client, error) {
	// 连接 BSC 节点
	c, err := chainrpc.Dial(context.Background(), url)
	if err != nil {
		return nil, err
	}
	contractABI, err := abi.JSON(strings.NewReader(abiSource))
	if err != nil {
		return nil, err
	}
	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	// 从私钥派生后端钱包地址（这是交易的发送者地址，用于查询 nonce）
	walletAddress := crypto.PubkeyToAddress(*publicKey)
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return nil, err
	}
	return &Client{
		PrivateKey:      privateKey,
		PublicKey:       publicKey,
		Address:         walletAddress, // 后端钱包地址（用于查询 nonce）
		ChainID:         chainID,
		ContractAddress: contractAddress, // 智能合约地址
		ContractABI:     contractABI,
		Auth:            auth,
		Client:          c,
	}, nil
}

func (c *Client) CheckInWithSig(user common.Address, day *big.Int, nonce [32]byte, deadline *big.Int, signature []byte) (string, error) {
	data, err := c.ContractABI.Pack("checkInWithSig", user, day, nonce, deadline, signature)
	if err != nil {
		return "", fmt.Errorf("pack function data failed: %w", err)
	}

	txHash, err := c.sendTransaction(data)
	if err != nil {
		return "", fmt.Errorf("send transaction failed: %w", err)
	}

	return txHash, nil
}

func (c *Client) PollEvents(ctx context.Context, lastBlock uint64) ([]types.Log, int, bool, error) {
	_, safeBlock, err := c.LatestAndSafeBlock(ctx)
	if err != nil {
		return nil, 0, false, err
	}
	// 如果没有新区块，跳过
	if safeBlock <= lastBlock {
		return nil, 0, false, nil
	}
	// 计算要查询的区块范围
	fromBlock := lastBlock + 1
	toBlock := safeBlock
	// 限制每次查询的区块数量（避免查询过多区块导致超时）
	var maxBlocksPerQuery uint64 = 500
	hasMore := false
	if toBlock-fromBlock > maxBlocksPerQuery {
		toBlock = fromBlock + maxBlocksPerQuery
		hasMore = true // 还有更多区块未处理
	}
	// 查询所有事件
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(fromBlock)),
		ToBlock:   big.NewInt(int64(toBlock)),
		Addresses: c.eventAddresses(),
	}

	logs, err := c.Client.FilterLogs(ctx, query)
	if err != nil {
		return nil, 0, false, fmt.Errorf("查询日志失败: %w", err)
	}
	sortEventLogs(logs)

	return logs, int(toBlock), hasMore, nil
}

func sortEventLogs(logs []types.Log) {
	sort.SliceStable(logs, func(i, j int) bool {
		if logs[i].BlockNumber != logs[j].BlockNumber {
			return logs[i].BlockNumber < logs[j].BlockNumber
		}
		if logs[i].TxIndex != logs[j].TxIndex {
			return logs[i].TxIndex < logs[j].TxIndex
		}
		return logs[i].Index < logs[j].Index
	})
}

func (c *Client) eventAddresses() []common.Address {
	if len(c.EventAddresses) > 0 {
		return c.EventAddresses
	}
	return []common.Address{c.ContractAddress}
}

// LatestAndSafeBlock 返回链尖和扣除确认深度后的安全链尖。
func (c *Client) LatestAndSafeBlock(ctx context.Context) (uint64, uint64, error) {
	latestBlock, err := c.Client.BlockNumber(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("获取最新区块失败: %w", err)
	}
	safeBlock := latestBlock
	if c.Confirmations > 0 {
		if latestBlock <= c.Confirmations {
			return latestBlock, 0, nil
		}
		safeBlock = latestBlock - c.Confirmations
	}
	return latestBlock, safeBlock, nil
}

// bigIntToDecimal 将 big.Int 转换为 decimal.Decimal
func bigIntToDecimal(b *big.Int) decimal.Decimal {
	return decimal.NewFromBigInt(b, 0)
}

func (c *Client) ProcessLog(vLog types.Log) *Data {
	if len(vLog.Topics) == 0 {
		return nil
	}
	eventTopic := vLog.Topics[0]
	var data *Data
	switch eventTopic {
	case eventSignatures[ProjectCreated]:
		data = handleProjectCreated(vLog)
	case eventSignatures[ProjectTokensDeposited]:
		data = handleProjectTokensDeposited(vLog)
	case eventSignatures[ProjectActivated]:
		data = handleProjectActivated(vLog)
	case eventSignatures[Purchase]:
		data = handlePurchase(vLog)
	case eventSignatures[ProjectEnded]:
		data = handleProjectEnded(vLog)
	case eventSignatures[Claimed]:
		data = handleClaimed(vLog)
	case eventSignatures[Refunded]:
		data = handleRefunded(vLog)
	case eventSignatures[RaisedFundsWithdrawn]:
		data = handleRaisedFundsWithdrawn(vLog)
	case eventSignatures[ProjectTokensWithdrawn]:
		data = handleProjectTokensWithdrawn(vLog)
	case eventSignatures[PlatformFeesUpdated]:
		data = handlePlatformFeesUpdated(vLog)
	case eventSignatures[ProjectCancelled]:
		data = handleProjectCancelled(vLog)
	case eventSignatures[UserCheckedIn]:
		data = handleUserCheckedIn(vLog)
	default:
		return nil
	}
	if data != nil {
		data.BlockNumber = vLog.BlockNumber
		data.BlockHash = vLog.BlockHash.Hex()
		data.TxHash = vLog.TxHash.Hex()
		data.LogIndex = vLog.Index
	}
	return data
}

// ProjectCreated(uint256 indexed projectId, address indexed projectToken, address indexed projectOwner, uint256 tokenPrice, uint256 hardCap, uint256 softCap)
func handleProjectCreated(vLog types.Log) *Data {
	return &Data{
		Event:       ProjectCreated,
		BlockNumber: vLog.BlockNumber,
		TxHash:      vLog.TxHash.String(),
		LogIndex:    vLog.Index,

		ProjectID:    new(big.Int).SetBytes(vLog.Topics[1].Bytes()).Int64(),
		ProjectToken: common.BytesToAddress(vLog.Topics[2].Bytes()),
		ProjectOwner: common.BytesToAddress(vLog.Topics[3].Bytes()),
		TokenPrice:   bigIntToDecimal(new(big.Int).SetBytes(vLog.Data[0:32])),
		HardCap:      bigIntToDecimal(new(big.Int).SetBytes(vLog.Data[32:64])),
		SoftCap:      bigIntToDecimal(new(big.Int).SetBytes(vLog.Data[64:96])),
	}
}

// ProjectTokensDeposited(uint256 indexed projectId, uint256 amount)
func handleProjectTokensDeposited(vLog types.Log) *Data {
	return &Data{
		Event:       ProjectTokensDeposited,
		BlockNumber: vLog.BlockNumber,
		TxHash:      vLog.TxHash.String(),
		LogIndex:    vLog.Index,

		ProjectID: new(big.Int).SetBytes(vLog.Topics[1].Bytes()).Int64(),
		Amount:    bigIntToDecimal(new(big.Int).SetBytes(vLog.Data[0:32])),
	}
}

// ProjectActivated(uint256 indexed projectId)
func handleProjectActivated(vLog types.Log) *Data {
	return &Data{
		Event:       ProjectActivated,
		BlockNumber: vLog.BlockNumber,
		TxHash:      vLog.TxHash.String(),
		LogIndex:    vLog.Index,

		ProjectID: new(big.Int).SetBytes(vLog.Topics[1].Bytes()).Int64(),
	}
}

// Purchase(uint256 indexed projectId, address indexed user, uint256 jccAmount, uint256 tokenAmount)
func handlePurchase(vLog types.Log) *Data {
	return &Data{
		Event:       Purchase,
		BlockNumber: vLog.BlockNumber,
		TxHash:      vLog.TxHash.String(),
		LogIndex:    vLog.Index,

		ProjectID:    new(big.Int).SetBytes(vLog.Topics[1].Bytes()).Int64(),
		User:         common.BytesToAddress(vLog.Topics[2].Bytes()),
		JccAmount:    bigIntToDecimal(new(big.Int).SetBytes(vLog.Data[0:32])),
		TokensAmount: bigIntToDecimal(new(big.Int).SetBytes(vLog.Data[32:64])),
	}
}

// ProjectEnded(uint256 indexed projectId, uint8 status, uint256 totalRaised)
func handleProjectEnded(vLog types.Log) *Data {
	status := vLog.Data[31] // 最后一个字节
	statusNames := []string{"Pending", "Active", "Ended", "Successful", "Failed", "Cancelled"}

	return &Data{
		Event:       ProjectEnded,
		BlockNumber: vLog.BlockNumber,
		TxHash:      vLog.TxHash.String(),
		LogIndex:    vLog.Index,

		ProjectID:   new(big.Int).SetBytes(vLog.Topics[1].Bytes()).Int64(),
		NewStatus:   statusNames[status],
		TotalRaised: bigIntToDecimal(new(big.Int).SetBytes(vLog.Data[32:64])),
	}
}

// Claimed(uint256 indexed projectId, address indexed user, uint256 amount)
func handleClaimed(vLog types.Log) *Data {
	return &Data{
		Event:       Claimed,
		BlockNumber: vLog.BlockNumber,
		TxHash:      vLog.TxHash.String(),
		LogIndex:    vLog.Index,

		ProjectID: new(big.Int).SetBytes(vLog.Topics[1].Bytes()).Int64(),
		User:      common.BytesToAddress(vLog.Topics[2].Bytes()),
		Amount:    bigIntToDecimal(new(big.Int).SetBytes(vLog.Data[0:32])),
	}
}

// Refunded(uint256 indexed projectId, address indexed user, uint256 jccAmount)
func handleRefunded(vLog types.Log) *Data {
	return &Data{
		Event:       Refunded,
		BlockNumber: vLog.BlockNumber,
		TxHash:      vLog.TxHash.String(),
		LogIndex:    vLog.Index,

		ProjectID: new(big.Int).SetBytes(vLog.Topics[1].Bytes()).Int64(),
		User:      common.BytesToAddress(vLog.Topics[2].Bytes()),
		JccAmount: bigIntToDecimal(new(big.Int).SetBytes(vLog.Data[0:32])),
	}
}

// RaisedFundsWithdrawn(uint256 indexed projectId, address indexed projectOwner, uint256 amount, uint256 platformFee)
func handleRaisedFundsWithdrawn(vLog types.Log) *Data {
	return &Data{
		Event:       RaisedFundsWithdrawn,
		BlockNumber: vLog.BlockNumber,
		TxHash:      vLog.TxHash.String(),
		LogIndex:    vLog.Index,

		ProjectID:    new(big.Int).SetBytes(vLog.Topics[1].Bytes()).Int64(),
		ProjectOwner: common.BytesToAddress(vLog.Topics[2].Bytes()),
		Amount:       bigIntToDecimal(new(big.Int).SetBytes(vLog.Data[0:32])),
		PlatformFee:  bigIntToDecimal(new(big.Int).SetBytes(vLog.Data[32:64])),
	}
}

// ProjectTokensWithdrawn(uint256 indexed projectId, address indexed projectOwner, uint256 amount)
func handleProjectTokensWithdrawn(vLog types.Log) *Data {
	return &Data{
		Event:       ProjectTokensWithdrawn,
		BlockNumber: vLog.BlockNumber,
		TxHash:      vLog.TxHash.String(),
		LogIndex:    vLog.Index,

		ProjectID:    new(big.Int).SetBytes(vLog.Topics[1].Bytes()).Int64(),
		ProjectOwner: common.BytesToAddress(vLog.Topics[2].Bytes()),
		Amount:       bigIntToDecimal(new(big.Int).SetBytes(vLog.Data[0:32])),
	}
}

// PlatformFeesUpdated(uint256 successFeeRate, address feeReceiver)
func handlePlatformFeesUpdated(vLog types.Log) *Data {
	return &Data{
		Event:       PlatformFeesUpdated,
		BlockNumber: vLog.BlockNumber,
		TxHash:      vLog.TxHash.String(),
		LogIndex:    vLog.Index,

		FeeRate:     bigIntToDecimal(new(big.Int).SetBytes(vLog.Data[0:32])),
		FeeReceiver: common.BytesToAddress(vLog.Data[32:64]),
	}
}

// ProjectCancelled(uint256 indexed projectId)
func handleProjectCancelled(vLog types.Log) *Data {
	return &Data{
		Event:       ProjectCancelled,
		BlockNumber: vLog.BlockNumber,
		TxHash:      vLog.TxHash.String(),
		LogIndex:    vLog.Index,

		ProjectID: new(big.Int).SetBytes(vLog.Topics[1].Bytes()).Int64(),
	}
}

// UserCheckedIn(address indexed user, uint256 indexed day, uint256 timestamp)
func handleUserCheckedIn(vLog types.Log) *Data {
	return &Data{
		Event:       UserCheckedIn,
		BlockNumber: vLog.BlockNumber,
		TxHash:      vLog.TxHash.String(),
		LogIndex:    vLog.Index,

		User:      common.BytesToAddress(vLog.Topics[1].Bytes()),
		Day:       new(big.Int).SetBytes(vLog.Topics[2].Bytes()).Int64(),
		Timestamp: new(big.Int).SetBytes(vLog.Data[0:32]).Int64(),
	}
}

// ============ Owner函数调用 ============

// ActivateProject 激活项目（仅owner）
// 返回: 交易哈希, 错误
func (c *Client) ActivateProject(projectId *big.Int) (string, error) {
	// 打包函数调用数据
	data, err := c.ContractABI.Pack("activateProject", projectId)
	if err != nil {
		return "", fmt.Errorf("pack function data failed: %w", err)
	}

	// 发送交易
	txHash, err := c.sendTransaction(data)
	if err != nil {
		return "", fmt.Errorf("send transaction failed: %w", err)
	}

	return txHash, nil
}

// EndProject 结束项目（仅owner可随时调用）
// 返回: 交易哈希, 错误
func (c *Client) EndProject(projectId *big.Int) (string, error) {
	return c.EndProjectContext(context.Background(), projectId)
}

func (c *Client) EndProjectContext(ctx context.Context, projectId *big.Int) (string, error) {
	// 打包函数调用数据
	data, err := c.ContractABI.Pack("endProject", projectId)
	if err != nil {
		return "", fmt.Errorf("pack function data failed: %w", err)
	}

	// 发送交易
	txHash, err := c.sendTransactionContext(ctx, data)
	if err != nil {
		return txHash, fmt.Errorf("send transaction failed: %w", err)
	}

	return txHash, nil
}

// SetPlatformFees 已删除（安全审计 LOW-2）：该方法是绕过 durable outbox 的 relayer
// 私钥直发路径，会破坏 chain_relayer_nonce 对账栅栏。setPlatformFees 若需支持，
// 必须像 global_pause 一样走 chaintx durable intent + 事件推进 applied。

// PackPause packs the owner-only global pause calldata for the durable outbox
// (安全审计 HIGH-1：Admin 全局暂停不再提供 relayer 私钥直发路径)。
func (c *Client) PackPause() ([]byte, error) {
	return c.ContractABI.Pack("pause")
}

// PackUnpause packs the owner-only global unpause calldata for the durable outbox.
func (c *Client) PackUnpause() ([]byte, error) {
	return c.ContractABI.Pack("unpause")
}

// BatchRefund 批量退款（仅owner，紧急情况使用）
// ⚠️ 安全审计警告：这是绕过 durable outbox 的 relayer 私钥直发路径，会破坏
// chain_relayer_nonce 对账栅栏。仅允许在受控应急演练中由人工执行并在事后校准
// nonce 槽；禁止接入任何 Admin/HTTP 端点或定时任务。
// 返回: 交易哈希, 错误
func (c *Client) BatchRefund(projectId *big.Int, users []common.Address) (string, error) {
	// 打包函数调用数据
	data, err := c.ContractABI.Pack("batchRefund", projectId, users)
	if err != nil {
		return "", fmt.Errorf("pack function data failed: %w", err)
	}

	// 发送交易
	txHash, err := c.sendTransaction(data)
	if err != nil {
		return "", fmt.Errorf("send transaction failed: %w", err)
	}

	return txHash, nil
}

// ============ View函数调用 ============

// HasCheckedInToday reads the contract's current UTC-day state for a wallet.
func (c *Client) HasCheckedInToday(ctx context.Context, user common.Address) (bool, error) {
	data, err := c.ContractABI.Pack("hasCheckedInToday", user)
	if err != nil {
		return false, fmt.Errorf("pack hasCheckedInToday failed: %w", err)
	}
	result, err := c.Client.CallContract(ctx, ethereum.CallMsg{
		To:   &c.ContractAddress,
		Data: data,
	}, nil)
	if err != nil {
		return false, fmt.Errorf("call hasCheckedInToday failed: %w", err)
	}
	values, err := c.ContractABI.Unpack("hasCheckedInToday", result)
	if err != nil || len(values) != 1 {
		return false, fmt.Errorf("unpack hasCheckedInToday failed: %w", err)
	}
	checked, ok := values[0].(bool)
	if !ok {
		return false, fmt.Errorf("unexpected hasCheckedInToday result type %T", values[0])
	}
	return checked, nil
}

// IsPaused 查询合约全局暂停状态
func (c *Client) IsPaused() (bool, error) {
	data, err := c.ContractABI.Pack("paused")
	if err != nil {
		return false, fmt.Errorf("pack paused: %w", err)
	}
	result, err := c.Client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &c.ContractAddress,
		Data: data,
	}, nil)
	if err != nil {
		return false, fmt.Errorf("call contract failed: %w", err)
	}
	values, err := c.ContractABI.Unpack("paused", result)
	if err != nil || len(values) != 1 {
		return false, fmt.Errorf("unpack paused failed: %w", err)
	}
	paused, ok := values[0].(bool)
	if !ok {
		return false, fmt.Errorf("unexpected paused type %T", values[0])
	}
	return paused, nil
}

// GetProject 查询项目信息
func (c *Client) GetProject(projectId *big.Int) (map[string]interface{}, error) {
	// 打包函数调用数据
	data, err := c.ContractABI.Pack("getProject", projectId)
	if err != nil {
		return nil, fmt.Errorf("pack function data failed: %w", err)
	}

	// 调用合约
	result, err := c.Client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &c.ContractAddress,
		Data: data,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("call contract failed: %w", err)
	}

	// 解包返回值
	var projectData map[string]interface{}
	err = c.ContractABI.UnpackIntoMap(projectData, "getProject", result)
	if err != nil {
		return nil, fmt.Errorf("unpack result failed: %w", err)
	}

	return projectData, nil
}

// CalculateClaimable 计算用户可领取的代币数量
func (c *Client) CalculateClaimable(projectId *big.Int, user common.Address) (*big.Int, error) {
	// 打包函数调用数据
	data, err := c.ContractABI.Pack("calculateClaimable", projectId, user)
	if err != nil {
		return nil, fmt.Errorf("pack function data failed: %w", err)
	}

	// 调用合约
	result, err := c.Client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &c.ContractAddress,
		Data: data,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("call contract failed: %w", err)
	}

	// 解包返回值
	var claimable *big.Int
	err = c.ContractABI.UnpackIntoInterface(&claimable, "calculateClaimable", result)
	if err != nil {
		return nil, fmt.Errorf("unpack result failed: %w", err)
	}

	return claimable, nil
}

// sendTransaction 发送交易的通用函数
func (c *Client) sendTransaction(data []byte) (string, error) {
	return c.sendTransactionContext(context.Background(), data)
}

func (c *Client) sendTransactionContext(ctx context.Context, data []byte) (string, error) {
	lock := senderLock(c.ChainID, c.Address)
	lock.Lock()
	defer lock.Unlock()

	// 获取当前 nonce
	nonce, err := c.Client.PendingNonceAt(ctx, c.Address)
	if err != nil {
		return "", fmt.Errorf("get nonce failed: %w", err)
	}

	// 估算 gas
	gasLimit, err := c.Client.EstimateGas(ctx, ethereum.CallMsg{
		From: c.Address,
		To:   &c.ContractAddress,
		Data: data,
	})
	if err != nil {
		return "", fmt.Errorf("estimate gas failed: %w", err)
	}

	// 获取 gas price
	gasPrice, err := c.Client.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("get gas price failed: %w", err)
	}

	// 构建交易
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &c.ContractAddress,
		Value:    big.NewInt(0),
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Data:     data,
	})

	// 签名交易
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(c.ChainID), c.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("sign transaction failed: %w", err)
	}

	return broadcastSignedTransaction(ctx, c.Client, signedTx)
}

// PackV2CreateProject returns immutable calldata without signing or broadcasting.
func (c *Client) PackV2CreateProject(params CreateProjectV2Params) ([]byte, error) {
	data, err := c.ContractABI.Pack("createProject", params.ProjectID, params.ProjectToken, params.PaymentToken, params.ProjectOwner, params.ExpectedFeeReceiver, params.ExpectedSuccessFeeRate, params.TokenPrice, params.HardCap, params.SoftCap, params.MaxPerUser, params.StartTime, params.EndTime, params.VestingTimes, params.VestingPercents)
	if err != nil {
		return nil, fmt.Errorf("pack V2 createProject: %w", err)
	}
	return data, nil
}

type CreateProjectV2Params struct {
	ProjectID              *big.Int
	ProjectToken           common.Address
	PaymentToken           common.Address
	ProjectOwner           common.Address
	ExpectedFeeReceiver    common.Address
	ExpectedSuccessFeeRate uint16
	TokenPrice             *big.Int
	HardCap                *big.Int
	SoftCap                *big.Int
	MaxPerUser             *big.Int
	StartTime              *big.Int
	EndTime                *big.Int
	VestingTimes           []*big.Int
	VestingPercents        []*big.Int
}

func (c *Client) CreateProjectV2(params CreateProjectV2Params) (string, error) {
	data, err := c.PackV2CreateProject(params)
	if err != nil {
		return "", err
	}
	txHash, err := c.sendTransaction(data)
	if err != nil {
		return txHash, fmt.Errorf("send V2 createProject: %w", err)
	}
	return txHash, nil
}

func (c *Client) PackV2ActivateProject(projectID *big.Int) ([]byte, error) {
	data, err := c.ContractABI.Pack("activateProject", projectID)
	if err != nil {
		return nil, fmt.Errorf("pack V2 activateProject: %w", err)
	}
	return data, nil
}

func (c *Client) ActivateProjectV2(projectID *big.Int) (string, error) {
	data, err := c.PackV2ActivateProject(projectID)
	if err != nil {
		return "", err
	}
	txHash, err := c.sendTransaction(data)
	if err != nil {
		return txHash, fmt.Errorf("send V2 activateProject: %w", err)
	}
	return txHash, nil
}

// PackEndProject returns immutable calldata suitable for durable outbox storage.
func (c *Client) PackEndProject(projectID *big.Int) ([]byte, error) {
	data, err := c.ContractABI.Pack("endProject", projectID)
	if err != nil {
		return nil, fmt.Errorf("pack endProject: %w", err)
	}
	return data, nil
}

// PackCancelProject returns immutable calldata suitable for durable outbox storage.
func (c *Client) PackCancelProject(projectID *big.Int) ([]byte, error) {
	data, err := c.ContractABI.Pack("cancelProject", projectID)
	if err != nil {
		return nil, fmt.Errorf("pack cancelProject: %w", err)
	}
	return data, nil
}

// PackWithdrawPlatformFee returns immutable calldata suitable for durable outbox storage.
func (c *Client) PackWithdrawPlatformFee(projectID *big.Int) ([]byte, error) {
	data, err := c.ContractABI.Pack("withdrawPlatformFee", projectID)
	if err != nil {
		return nil, fmt.Errorf("pack withdrawPlatformFee: %w", err)
	}
	return data, nil
}

// EstimateLegacyTransaction obtains gas parameters before durable nonce allocation.
func (c *Client) EstimateLegacyTransaction(ctx context.Context, to common.Address, data []byte) (uint64, *big.Int, error) {
	estimate, err := c.Client.EstimateGas(ctx, ethereum.CallMsg{From: c.Address, To: &to, Data: data})
	if err != nil {
		return 0, nil, fmt.Errorf("estimate gas failed: %w", err)
	}
	// An RPC estimate is a lower bound for the state it observed, not a safe
	// transaction limit. In particular, an ERC-20 transfer can cost more by the
	// time the signed withdrawal is mined. Reserve 30% before persisting the
	// nonce and signature; do not silently wrap or sign an invalid limit.
	gasLimit, err := bufferedGasLimit(estimate)
	if err != nil {
		return 0, nil, err
	}
	gasPrice, err := c.Client.SuggestGasPrice(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("get gas price failed: %w", err)
	}
	if gasLimit == 0 || gasPrice == nil || gasPrice.Sign() <= 0 {
		return 0, nil, errors.New("invalid gas parameters")
	}
	return gasLimit, gasPrice, nil
}

// bufferedGasLimit rounds the 30% safety margin up without overflowing.
func bufferedGasLimit(estimate uint64) (uint64, error) {
	if estimate == 0 {
		return 0, errors.New("invalid zero gas estimate")
	}
	// ceil(estimate * 13 / 10) via quotient/remainder; the multiplication
	// check prevents a wrapped limit from entering the durable outbox.
	base, carry := bits.Mul64(estimate/10, 13)
	if base != 0 {
		return 0, errors.New("buffered gas limit overflows uint64")
	}
	extra := (estimate%10*13 + 9) / 10
	limit, overflow := bits.Add64(carry, extra, 0)
	if overflow != 0 {
		return 0, errors.New("buffered gas limit overflows uint64")
	}
	return limit, nil
}

// SignLegacyTransaction signs an explicit nonce. Callers must persist the
// returned raw bytes and hash atomically before broadcasting.
func (c *Client) SignLegacyTransaction(nonce, gasLimit uint64, gasPrice *big.Int, to common.Address, data []byte) ([]byte, string, error) {
	if gasPrice == nil || gasPrice.Sign() <= 0 || gasLimit == 0 {
		return nil, "", errors.New("invalid gas parameters")
	}
	tx := types.NewTx(&types.LegacyTx{Nonce: nonce, To: &to, Value: big.NewInt(0), Gas: gasLimit, GasPrice: new(big.Int).Set(gasPrice), Data: append([]byte(nil), data...)})
	signed, err := types.SignTx(tx, types.NewEIP155Signer(c.ChainID), c.PrivateKey)
	if err != nil {
		return nil, "", fmt.Errorf("sign transaction failed: %w", err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		return nil, "", fmt.Errorf("marshal signed transaction: %w", err)
	}
	return raw, signed.Hash().Hex(), nil
}

// BroadcastRawTransaction rebroadcasts only the exact persisted transaction.
func (c *Client) BroadcastRawTransaction(ctx context.Context, raw []byte) (string, error) {
	var tx types.Transaction
	if err := tx.UnmarshalBinary(raw); err != nil {
		return "", fmt.Errorf("decode raw transaction: %w", err)
	}
	hash := tx.Hash().Hex()
	if err := c.Client.SendTransaction(ctx, &tx); err != nil {
		return hash, fmt.Errorf("broadcast outcome unknown: %w", err)
	}
	return hash, nil
}

type transactionSender interface {
	SendTransaction(context.Context, *types.Transaction) error
}

func broadcastSignedTransaction(ctx context.Context, sender transactionSender, signedTx *types.Transaction) (string, error) {
	txHash := signedTx.Hash().Hex()
	if err := sender.SendTransaction(ctx, signedTx); err != nil {
		// The node may have accepted the signed transaction before the RPC
		// connection failed. Return its deterministic hash so callers can
		// persist an outcome-unknown attempt and fail closed.
		return txHash, fmt.Errorf("broadcast outcome unknown: %w", err)
	}
	return txHash, nil
}

// ============ CreateProject 函数 ============

// CreateProjectParams 创建项目的参数
type CreateProjectParams struct {
	ProjectID       int64          // 项目ID（由服务端生成）
	ProjectToken    common.Address // 项目代币地址
	ProjectOwner    common.Address // 项目方地址
	TokenPrice      *big.Int       // 代币价格 (1 token = X JCC, 18位精度)
	HardCap         *big.Int       // 硬顶 (JCC数量)
	SoftCap         *big.Int       // 软顶 (JCC数量)
	MaxPerUser      *big.Int       // 单用户认购上限 (JCC)
	StartTime       *big.Int       // 认购开始时间 (Unix时间戳)
	EndTime         *big.Int       // 认购结束时间 (Unix时间戳)
	VestingTimes    []*big.Int     // 释放时间点数组
	VestingPercents []*big.Int     // 释放百分比数组
	VestingDuration *big.Int       // 释放总时长（秒）
}

// CreateProject 调用智能合约创建项目
// 返回: 交易哈希, 错误
func (c *Client) CreateProject(params CreateProjectParams) (string, error) {
	// 打包函数调用数据
	data, err := c.ContractABI.Pack(
		"createProject",
		big.NewInt(params.ProjectID), // projectId 由服务端传入
		params.ProjectToken,
		params.ProjectOwner,
		params.TokenPrice,
		params.HardCap,
		params.SoftCap,
		params.MaxPerUser,
		params.StartTime,
		params.EndTime,
		params.VestingTimes,
		params.VestingPercents,
		params.VestingDuration,
	)
	if err != nil {
		return "", fmt.Errorf("pack function data failed: %w", err)
	}

	// 发送交易
	txHash, err := c.sendTransaction(data)
	if err != nil {
		return "", fmt.Errorf("send transaction failed: %w", err)
	}

	return txHash, nil
}

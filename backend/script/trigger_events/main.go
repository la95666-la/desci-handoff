package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	RPC_URL          = "https://bsc-testnet-rpc.publicnode.com"
	JOINCARE_ADDRESS = "0xe6F01eC4dEA815c9dAAb79d7A7D0FA16b2cB368A"
	JCC_ADDRESS      = "0xFc6C84ac73c11f6C162B9dd5B4A94e7bfb3308EA"
	CHAIN_ID         = 97
)

// 只包含我们需要调用的函数的 ABI
var joincareABI = `[
  {"inputs":[{"internalType":"uint256","name":"_successFeeRate","type":"uint256"},{"internalType":"address","name":"_feeReceiver","type":"address"}],"name":"setPlatformFees","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[{"internalType":"uint256","name":"_projectId","type":"uint256"},{"internalType":"address","name":"_projectToken","type":"address"},{"internalType":"address","name":"_projectOwner","type":"address"},{"internalType":"uint256","name":"_tokenPrice","type":"uint256"},{"internalType":"uint256","name":"_hardCap","type":"uint256"},{"internalType":"uint256","name":"_softCap","type":"uint256"},{"internalType":"uint256","name":"_maxPerUser","type":"uint256"},{"internalType":"uint256","name":"_startTime","type":"uint256"},{"internalType":"uint256","name":"_endTime","type":"uint256"},{"internalType":"uint256[]","name":"_vestingTimes","type":"uint256[]"},{"internalType":"uint256[]","name":"_vestingPercents","type":"uint256[]"},{"internalType":"uint256","name":"_vestingDuration","type":"uint256"}],"name":"createProject","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[{"internalType":"uint256","name":"_projectId","type":"uint256"},{"internalType":"uint256","name":"_amount","type":"uint256"}],"name":"depositProjectTokens","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[{"internalType":"uint256","name":"_projectId","type":"uint256"}],"name":"activateProject","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[{"internalType":"uint256","name":"_projectId","type":"uint256"},{"internalType":"uint256","name":"_jccAmount","type":"uint256"}],"name":"purchase","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[{"internalType":"uint256","name":"_projectId","type":"uint256"}],"name":"endProject","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[{"internalType":"uint256","name":"_projectId","type":"uint256"}],"name":"claim","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[{"internalType":"uint256","name":"_projectId","type":"uint256"}],"name":"refund","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[{"internalType":"uint256","name":"_projectId","type":"uint256"}],"name":"withdrawRaisedFunds","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[{"internalType":"uint256","name":"_projectId","type":"uint256"}],"name":"withdrawProjectTokens","outputs":[],"stateMutability":"nonpayable","type":"function"}
]`

var erc20ABI = `[
  {"inputs":[{"internalType":"address","name":"spender","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"approve","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[{"internalType":"address","name":"account","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}
]`

type Caller struct {
	client     *ethclient.Client
	privateKey *ecdsa.PrivateKey
	address    common.Address
	chainID    *big.Int
	jcABI      abi.ABI
	erc20ABI   abi.ABI
}

func NewCaller() (*Caller, error) {
	keyHex := strings.TrimSpace(os.Getenv("JOINCARE_TESTNET_PRIVATE_KEY"))
	if keyHex == "" {
		return nil, fmt.Errorf("JOINCARE_TESTNET_PRIVATE_KEY is required")
	}
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(strings.TrimPrefix(keyHex, "0x"), "0X"))
	if err != nil {
		return nil, fmt.Errorf("JOINCARE_TESTNET_PRIVATE_KEY must be a valid 32-byte hex private key")
	}

	proxyUrl, _ := url.Parse("http://127.0.0.1:7897")
	http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}

	client, err := ethclient.Dial(RPC_URL)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	address := crypto.PubkeyToAddress(*publicKey)

	jcABI, _ := abi.JSON(strings.NewReader(joincareABI))
	erc20ABI, _ := abi.JSON(strings.NewReader(erc20ABI))

	return &Caller{
		client:     client,
		privateKey: privateKey,
		address:    address,
		chainID:    big.NewInt(CHAIN_ID),
		jcABI:      jcABI,
		erc20ABI:   erc20ABI,
	}, nil
}

func (c *Caller) sendTx(to common.Address, data []byte) (string, error) {
	ctx := context.Background()
	nonce, err := c.client.PendingNonceAt(ctx, c.address)
	if err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	gasPrice, err := c.client.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("gas price: %w", err)
	}
	gasLimit, err := c.client.EstimateGas(ctx, ethereum.CallMsg{
		From: c.address,
		To:   &to,
		Data: data,
	})
	if err != nil {
		return "", fmt.Errorf("estimate gas: %w", err)
	}
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &to,
		Value:    big.NewInt(0),
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Data:     data,
	})
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(c.chainID), c.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	err = c.client.SendTransaction(ctx, signedTx)
	if err != nil {
		return "", fmt.Errorf("send: %w", err)
	}
	// 等待确认
	hash := signedTx.Hash()
	for i := 0; i < 60; i++ {
		receipt, err := c.client.TransactionReceipt(ctx, hash)
		if err == nil {
			if receipt.Status == 0 {
				return hash.Hex(), fmt.Errorf("tx reverted")
			}
			return hash.Hex(), nil
		}
		time.Sleep(3 * time.Second)
	}
	return hash.Hex(), fmt.Errorf("timeout waiting for receipt")
}

func (c *Caller) callJC(method string, args ...interface{}) (string, error) {
	data, err := c.jcABI.Pack(method, args...)
	if err != nil {
		return "", fmt.Errorf("pack %s: %w", method, err)
	}
	return c.sendTx(common.HexToAddress(JOINCARE_ADDRESS), data)
}

func (c *Caller) approveJCC(spender common.Address, amount *big.Int) (string, error) {
	data, err := c.erc20ABI.Pack("approve", spender, amount)
	if err != nil {
		return "", fmt.Errorf("pack approve: %w", err)
	}
	return c.sendTx(common.HexToAddress(JCC_ADDRESS), data)
}

func (c *Caller) getBlockTimestamp() (uint64, error) {
	header, err := c.client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return 0, err
	}
	return header.Time, nil
}

func wei(n int64) *big.Int {
	one := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	return new(big.Int).Mul(big.NewInt(n), one)
}

func main() {
	caller, err := NewCaller()
	if err != nil {
		panic(err)
	}
	fmt.Println("Owner:", caller.address.Hex())

	jcAddr := common.HexToAddress(JOINCARE_ADDRESS)

	// 获取链上时间
	blockTime, _ := caller.getBlockTimestamp()
	fmt.Printf("链上时间: %d\n", blockTime)

	// ===== 1. PlatformFeesUpdated =====
	fmt.Println("\n--- 1. PlatformFeesUpdated ---")
	hash, err := caller.callJC("setPlatformFees", big.NewInt(200), caller.address)
	if err != nil {
		fmt.Printf("  ❌ %s\n", err)
	} else {
		fmt.Printf("  ✅ tx: %s\n", hash)
	}

	// ===== 2. ProjectCreated (项目 200 - 成功流程) =====
	fmt.Println("\n--- 2. ProjectCreated (项目200) ---")
	projectIdA := big.NewInt(40062)
	startTime := new(big.Int).SetUint64(blockTime + 30)    // 30秒后开始
	endTime := new(big.Int).SetUint64(blockTime + 180)     // 3分钟后结束
	vestingTime := new(big.Int).SetUint64(blockTime + 200) // 结束后20秒释放

	hash, err = caller.callJC("createProject",
		projectIdA,
		common.HexToAddress(JCC_ADDRESS), // 用JCC当项目代币
		caller.address,                   // projectOwner
		wei(1),                           // tokenPrice: 1 JCC = 1 token
		wei(100),                         // hardCap
		wei(10),                          // softCap
		wei(100),                         // maxPerUser
		startTime,
		endTime,
		[]*big.Int{vestingTime},       // vestingTimes
		[]*big.Int{big.NewInt(10000)}, // 100%
		big.NewInt(0),                 // vestingDuration
	)
	if err != nil {
		fmt.Printf("  ❌ %s\n", err)
	} else {
		fmt.Printf("  ✅ tx: %s\n", hash)
	}

	// ===== 3. Approve JCC =====
	fmt.Println("\n--- 3. Approve JCC ---")
	hash, err = caller.approveJCC(jcAddr, wei(10000))
	if err != nil {
		fmt.Printf("  ❌ %s\n", err)
	} else {
		fmt.Printf("  ✅ tx: %s\n", hash)
	}

	// ===== 4. ProjectTokensDeposited =====
	fmt.Println("\n--- 4. ProjectTokensDeposited ---")
	hash, err = caller.callJC("depositProjectTokens", projectIdA, wei(200))
	if err != nil {
		fmt.Printf("  ❌ %s\n", err)
	} else {
		fmt.Printf("  ✅ tx: %s\n", hash)
	}

	// ===== 5. ProjectActivated =====
	fmt.Println("\n--- 5. ProjectActivated ---")
	hash, err = caller.callJC("activateProject", projectIdA)
	if err != nil {
		fmt.Printf("  ❌ %s\n", err)
	} else {
		fmt.Printf("  ✅ tx: %s\n", hash)
	}

	// ===== 6. Purchase (等待开始时间) =====
	fmt.Println("\n--- 6. 等待认购开始... ---")
	for {
		bt, _ := caller.getBlockTimestamp()
		if bt >= blockTime+30 {
			break
		}
		fmt.Print(".")
		time.Sleep(3 * time.Second)
	}
	fmt.Println()
	fmt.Println("--- 6. Purchase ---")
	hash, err = caller.callJC("purchase", projectIdA, wei(100))
	if err != nil {
		fmt.Printf("  ❌ %s\n", err)
	} else {
		fmt.Printf("  ✅ tx: %s\n", hash)
	}

	// ===== 7. ProjectEnded (owner可随时结束) =====
	fmt.Println("\n--- 7. ProjectEnded ---")
	hash, err = caller.callJC("endProject", projectIdA)
	if err != nil {
		fmt.Printf("  ❌ %s\n", err)
	} else {
		fmt.Printf("  ✅ tx: %s\n", hash)
	}

	// ===== 8. RaisedFundsWithdrawn =====
	fmt.Println("\n--- 8. RaisedFundsWithdrawn ---")
	hash, err = caller.callJC("withdrawRaisedFunds", projectIdA)
	if err != nil {
		fmt.Printf("  ❌ %s\n", err)
	} else {
		fmt.Printf("  ✅ tx: %s\n", hash)
	}

	// ===== 9. Claimed (等待释放时间) =====
	fmt.Println("\n--- 9. 等待释放时间... ---")
	for {
		bt, _ := caller.getBlockTimestamp()
		if bt >= blockTime+200 {
			break
		}
		fmt.Print(".")
		time.Sleep(5 * time.Second)
	}
	fmt.Println()
	fmt.Println("--- 9. Claimed ---")
	hash, err = caller.callJC("claim", projectIdA)
	if err != nil {
		fmt.Printf("  ❌ %s\n", err)
	} else {
		fmt.Printf("  ✅ tx: %s\n", hash)
	}

	// ===== 10. ProjectTokensWithdrawn =====
	fmt.Println("\n--- 10. ProjectTokensWithdrawn ---")
	hash, err = caller.callJC("withdrawProjectTokens", projectIdA)
	if err != nil {
		fmt.Printf("  ❌ %s\n", err)
	} else {
		fmt.Printf("  ✅ tx: %s\n", hash)
	}

	// ===== 11. 创建项目 201（失败流程 → Refunded） =====
	fmt.Println("\n--- 11. 创建项目201（失败流程） ---")
	blockTime2, _ := caller.getBlockTimestamp()
	projectIdB := big.NewInt(40071)
	startTimeB := new(big.Int).SetUint64(blockTime2 + 15)
	endTimeB := new(big.Int).SetUint64(blockTime2 + 120)
	vestingTimeB := new(big.Int).SetUint64(blockTime2 + 140)

	hash, err = caller.callJC("createProject",
		projectIdB,
		common.HexToAddress(JCC_ADDRESS),
		caller.address,
		wei(1),
		wei(1000), // hardCap 1000
		wei(500),  // softCap 500（高，不会达到）
		wei(1000),
		startTimeB,
		endTimeB,
		[]*big.Int{vestingTimeB},
		[]*big.Int{big.NewInt(10000)},
		big.NewInt(0),
	)
	if err != nil {
		fmt.Printf("  ❌ 创建项目201失败: %s\n", err)
		fmt.Println("\n========== 完成 ==========")
		return
	}
	fmt.Printf("  ✅ 项目201创建, tx: %s\n", hash)

	// 充值
	hash, err = caller.callJC("depositProjectTokens", projectIdB, wei(1000))
	if err != nil {
		fmt.Printf("  ❌ 充值失败: %s\n", err)
		return
	}
	fmt.Printf("  ✅ 充值完成\n")

	// 激活
	hash, err = caller.callJC("activateProject", projectIdB)
	if err != nil {
		fmt.Printf("  ❌ 激活失败: %s\n", err)
		return
	}
	fmt.Printf("  ✅ 激活完成\n")

	// 等待开始
	fmt.Println("  等待认购开始...")
	for {
		bt, _ := caller.getBlockTimestamp()
		if bt >= blockTime2+15 {
			break
		}
		time.Sleep(3 * time.Second)
	}

	// 认购少量
	hash, err = caller.callJC("purchase", projectIdB, wei(10))
	if err != nil {
		fmt.Printf("  ❌ 认购失败: %s\n", err)
		return
	}
	fmt.Printf("  ✅ 认购10 JCC\n")

	// 结束（未达softCap → Failed）
	hash, err = caller.callJC("endProject", projectIdB)
	if err != nil {
		fmt.Printf("  ❌ 结束失败: %s\n", err)
		return
	}
	fmt.Printf("  ✅ 项目201已结束(Failed)\n")

	// ===== 12. Refunded =====
	fmt.Println("\n--- 12. Refunded ---")
	hash, err = caller.callJC("refund", projectIdB)
	if err != nil {
		fmt.Printf("  ❌ %s\n", err)
	} else {
		fmt.Printf("  ✅ tx: %s\n", hash)
	}

	fmt.Println("\n========== 全部事件触发完成 ==========")
}

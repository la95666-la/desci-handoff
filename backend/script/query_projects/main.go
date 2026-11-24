package main

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	RPC_URL          = "https://bsc-testnet-rpc.publicnode.com"
	JOINCARE_ADDRESS = "0xe6F01eC4dEA815c9dAAb79d7A7D0FA16b2cB368A"
)

// getProject 的完整 ABI
var getProjectABI = `[
  {
    "inputs":[{"internalType":"uint256","name":"_projectId","type":"uint256"}],
    "name":"getProject",
    "outputs":[
      {"components":[
        {"internalType":"uint256","name":"projectId","type":"uint256"},
        {"internalType":"address","name":"projectToken","type":"address"},
        {"internalType":"address","name":"projectOwner","type":"address"},
        {"internalType":"uint256","name":"tokenPrice","type":"uint256"},
        {"internalType":"uint256","name":"hardCap","type":"uint256"},
        {"internalType":"uint256","name":"softCap","type":"uint256"},
        {"internalType":"uint256","name":"maxPerUser","type":"uint256"},
        {"internalType":"uint256","name":"startTime","type":"uint256"},
        {"internalType":"uint256","name":"endTime","type":"uint256"},
        {"internalType":"uint256[]","name":"vestingTimes","type":"uint256[]"},
        {"internalType":"uint256[]","name":"vestingPercents","type":"uint256[]"},
        {"internalType":"uint256","name":"vestingDuration","type":"uint256"},
        {"internalType":"uint8","name":"status","type":"uint8"},
        {"internalType":"uint256","name":"totalRaised","type":"uint256"},
        {"internalType":"uint256","name":"totalTokensSold","type":"uint256"},
        {"internalType":"uint256","name":"totalTokensDeposited","type":"uint256"},
        {"internalType":"bool","name":"fundsWithdrawn","type":"bool"},
        {"internalType":"bool","name":"projectTokensWithdrawn","type":"bool"}
      ],
      "internalType":"struct JoinCare.Project",
      "name":"",
      "type":"tuple"}
    ],
    "stateMutability":"view",
    "type":"function"
  }
]`

var statusMap = map[uint8]string{
	0: "Pending",
	1: "Active",
	2: "Ended",
	3: "Successful",
	4: "Failed",
	5: "Canceled",
}

func weiToEth(wei *big.Int) string {
	if wei == nil {
		return "0"
	}
	one := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	whole := new(big.Int).Div(wei, one)
	frac := new(big.Int).Mod(wei, one)
	if frac.Sign() == 0 {
		return whole.String()
	}
	fracStr := fmt.Sprintf("%018s", frac.String())
	fracStr = strings.TrimRight(fracStr, "0")
	return fmt.Sprintf("%s.%s", whole.String(), fracStr)
}

func main() {
	// 设置代理（如果不需要代理请注释掉下面两行）
	// proxyUrl, _ := url.Parse("http://127.0.0.1:7897")
	// http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}

	client, err := ethclient.Dial(RPC_URL)
	if err != nil {
		panic(fmt.Sprintf("连接 RPC 失败: %v", err))
	}
	defer client.Close()

	parsedABI, err := abi.JSON(strings.NewReader(getProjectABI))
	if err != nil {
		panic(fmt.Sprintf("解析 ABI 失败: %v", err))
	}

	contractAddr := common.HexToAddress(JOINCARE_ADDRESS)

	// ========== 要查询的 projectId 列表 ==========
	projectIDs := []string{
		"9662467600809985", // 0305-1 (新)
	}

	fmt.Println("========================================")
	fmt.Println("  JoinCare 链上项目查询")
	fmt.Printf("  合约地址: %s\n", JOINCARE_ADDRESS)
	fmt.Printf("  RPC: %s\n", RPC_URL)
	fmt.Println("========================================")

	for _, idStr := range projectIDs {
		id := new(big.Int)
		id.SetString(idStr, 10)

		fmt.Printf("\n--- 查询 ProjectID: %s ---\n", idStr)

		// 编码调用数据
		callData, err := parsedABI.Pack("getProject", id)
		if err != nil {
			fmt.Printf("  ❌ Pack 失败: %v\n", err)
			continue
		}

		// eth_call
		result, err := client.CallContract(context.Background(), ethereum.CallMsg{
			To:   &contractAddr,
			Data: callData,
		}, nil)
		if err != nil {
			fmt.Printf("  ❌ 项目不存在或调用失败: %v\n", err)
			continue
		}

		// 解码结果
		res, err := parsedABI.Unpack("getProject", result)
		if err != nil {
			fmt.Printf("  ❌ 解码失败: %v\n", err)
			continue
		}

		// res[0] 是 struct
		project := res[0]
		// 使用反射或类型断言来读取 struct 字段
		// go-ethereum 返回的 struct 是 anonymous struct
		type ProjectData struct {
			ProjectId              *big.Int
			ProjectToken           common.Address
			ProjectOwner           common.Address
			TokenPrice             *big.Int
			HardCap                *big.Int
			SoftCap                *big.Int
			MaxPerUser             *big.Int
			StartTime              *big.Int
			EndTime                *big.Int
			VestingTimes           []*big.Int
			VestingPercents        []*big.Int
			VestingDuration        *big.Int
			Status                 uint8
			TotalRaised            *big.Int
			TotalTokensSold        *big.Int
			TotalTokensDeposited   *big.Int
			FundsWithdrawn         bool
			ProjectTokensWithdrawn bool
		}

		converted := abi.ConvertType(project, new(ProjectData))
		p, ok := converted.(*ProjectData)
		if ok && p.ProjectId != nil && p.ProjectId.Sign() > 0 {
			statusStr := statusMap[p.Status]
			if statusStr == "" {
				statusStr = fmt.Sprintf("Unknown(%d)", p.Status)
			}
			fmt.Printf("  ✅ 项目存在！\n")
			fmt.Printf("     ProjectID:    %s\n", p.ProjectId.String())
			fmt.Printf("     Status:       %s\n", statusStr)
			fmt.Printf("     ProjectToken: %s\n", p.ProjectToken.Hex())
			fmt.Printf("     ProjectOwner: %s\n", p.ProjectOwner.Hex())
			fmt.Printf("     TokenPrice:   %s\n", weiToEth(p.TokenPrice))
			fmt.Printf("     HardCap:      %s\n", weiToEth(p.HardCap))
			fmt.Printf("     SoftCap:      %s\n", weiToEth(p.SoftCap))
			fmt.Printf("     MaxPerUser:   %s\n", weiToEth(p.MaxPerUser))
			fmt.Printf("     TotalRaised:  %s\n", weiToEth(p.TotalRaised))
			fmt.Printf("     Deposited:    %s\n", weiToEth(p.TotalTokensDeposited))
			fmt.Printf("     FundsWithdrawn:  %v\n", p.FundsWithdrawn)
			fmt.Printf("     TokensWithdrawn: %v\n", p.ProjectTokensWithdrawn)
		} else {
			// projectId == 0 表示不存在
			fmt.Printf("  ❌ 项目不存在（链上返回空数据）\n")
		}
	}

	fmt.Println("\n========================================")
	fmt.Println("  查询完成")
	fmt.Println("========================================")
}

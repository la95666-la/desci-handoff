package chain

import (
	"crypto/ecdsa"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/shopspring/decimal"
)

type Client struct {
	PrivateKey      *ecdsa.PrivateKey
	PublicKey       *ecdsa.PublicKey
	Address         common.Address
	ChainID         *big.Int
	ContractAddress common.Address
	EventAddresses  []common.Address
	ContractABI     abi.ABI
	Auth            *bind.TransactOpts
	Client          *ethclient.Client
	// Confirmations 事件处理前等待的确认块数（防 reorg）；0 = 不等待（本地 hardhat）
	Confirmations uint64
}

var abiJson = `[
  {
    "inputs": [
      { "internalType": "uint256", "name": "_projectId", "type": "uint256" },
      { "internalType": "address", "name": "_projectToken", "type": "address" },
      { "internalType": "address", "name": "_projectOwner", "type": "address" },
      { "internalType": "uint256", "name": "_tokenPrice", "type": "uint256" },
      { "internalType": "uint256", "name": "_hardCap", "type": "uint256" },
      { "internalType": "uint256", "name": "_softCap", "type": "uint256" },
      { "internalType": "uint256", "name": "_maxPerUser", "type": "uint256" },
      { "internalType": "uint256", "name": "_startTime", "type": "uint256" },
      { "internalType": "uint256", "name": "_endTime", "type": "uint256" },
      { "internalType": "uint256[]", "name": "_vestingTimes", "type": "uint256[]" },
      { "internalType": "uint256[]", "name": "_vestingPercents", "type": "uint256[]" },
      { "internalType": "uint256", "name": "_vestingDuration", "type": "uint256" }
    ],
    "name": "createProject",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [
      { "internalType": "uint256", "name": "_projectId", "type": "uint256" }
    ],
    "name": "activateProject",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [
      { "internalType": "uint256", "name": "_projectId", "type": "uint256" }
    ],
    "name": "endProject",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [
      { "internalType": "uint256", "name": "_successFeeRate", "type": "uint256" },
      { "internalType": "address", "name": "_feeReceiver", "type": "address" }
    ],
    "name": "setPlatformFees",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "pause",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "unpause",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [
      { "internalType": "uint256", "name": "_projectId", "type": "uint256" },
      { "internalType": "address[]", "name": "_users", "type": "address[]" }
    ],
    "name": "batchRefund",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [
      { "internalType": "uint256", "name": "_projectId", "type": "uint256" }
    ],
    "name": "getProject",
    "outputs": [
      {
        "components": [
          { "internalType": "uint256", "name": "projectId", "type": "uint256" },
          { "internalType": "address", "name": "projectToken", "type": "address" },
          { "internalType": "address", "name": "projectOwner", "type": "address" },
          { "internalType": "uint256", "name": "tokenPrice", "type": "uint256" },
          { "internalType": "uint256", "name": "hardCap", "type": "uint256" },
          { "internalType": "uint256", "name": "softCap", "type": "uint256" },
          { "internalType": "uint256", "name": "maxPerUser", "type": "uint256" },
          { "internalType": "uint256", "name": "startTime", "type": "uint256" },
          { "internalType": "uint256", "name": "endTime", "type": "uint256" },
          { "internalType": "uint256[]", "name": "vestingTimes", "type": "uint256[]" },
          { "internalType": "uint256[]", "name": "vestingPercents", "type": "uint256[]" },
          { "internalType": "uint256", "name": "vestingDuration", "type": "uint256" },
          { "internalType": "enum JoinCare_v1.ProjectStatus", "name": "status", "type": "uint8" },
          { "internalType": "uint256", "name": "totalRaised", "type": "uint256" },
          { "internalType": "uint256", "name": "totalTokensSold", "type": "uint256" },
          { "internalType": "uint256", "name": "totalTokensDeposited", "type": "uint256" },
          { "internalType": "bool", "name": "fundsWithdrawn", "type": "bool" },
          { "internalType": "bool", "name": "projectTokensWithdrawn", "type": "bool" }
        ],
        "internalType": "struct JoinCare_v1.Project",
        "name": "",
        "type": "tuple"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      { "internalType": "uint256", "name": "_projectId", "type": "uint256" },
      { "internalType": "address", "name": "_user", "type": "address" }
    ],
    "name": "calculateClaimable",
    "outputs": [
      { "internalType": "uint256", "name": "", "type": "uint256" }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      { "internalType": "uint256", "name": "_projectId", "type": "uint256" },
      { "internalType": "address", "name": "_user", "type": "address" }
    ],
    "name": "getUserPurchase",
    "outputs": [
      {
        "components": [
          { "internalType": "uint256", "name": "totalJCCSpent", "type": "uint256" },
          { "internalType": "uint256", "name": "totalTokensBought", "type": "uint256" },
          { "internalType": "uint256", "name": "claimedTokens", "type": "uint256" },
          { "internalType": "bool", "name": "refunded", "type": "bool" }
        ],
        "internalType": "struct JoinCare_v1.UserPurchase",
        "name": "",
        "type": "tuple"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "projectCounter",
    "outputs": [
      { "internalType": "uint256", "name": "", "type": "uint256" }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "anonymous": false,
    "inputs": [
      { "indexed": true, "internalType": "uint256", "name": "projectId", "type": "uint256" },
      { "indexed": true, "internalType": "address", "name": "projectToken", "type": "address" },
      { "indexed": true, "internalType": "address", "name": "projectOwner", "type": "address" },
      { "indexed": false, "internalType": "uint256", "name": "tokenPrice", "type": "uint256" },
      { "indexed": false, "internalType": "uint256", "name": "hardCap", "type": "uint256" },
      { "indexed": false, "internalType": "uint256", "name": "softCap", "type": "uint256" }
    ],
    "name": "ProjectCreated",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      { "indexed": true, "internalType": "uint256", "name": "projectId", "type": "uint256" },
      { "indexed": false, "internalType": "uint256", "name": "amount", "type": "uint256" }
    ],
    "name": "ProjectTokensDeposited",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      { "indexed": true, "internalType": "uint256", "name": "projectId", "type": "uint256" }
    ],
    "name": "ProjectActivated",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      { "indexed": true, "internalType": "uint256", "name": "projectId", "type": "uint256" },
      { "indexed": true, "internalType": "address", "name": "user", "type": "address" },
      { "indexed": false, "internalType": "uint256", "name": "jccAmount", "type": "uint256" },
      { "indexed": false, "internalType": "uint256", "name": "tokenAmount", "type": "uint256" }
    ],
    "name": "Purchase",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      { "indexed": true, "internalType": "uint256", "name": "projectId", "type": "uint256" },
      { "indexed": false, "internalType": "enum JoinCare_v1.ProjectStatus", "name": "status", "type": "uint8" },
      { "indexed": false, "internalType": "uint256", "name": "totalRaised", "type": "uint256" }
    ],
    "name": "ProjectEnded",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      { "indexed": true, "internalType": "uint256", "name": "projectId", "type": "uint256" },
      { "indexed": true, "internalType": "address", "name": "user", "type": "address" },
      { "indexed": false, "internalType": "uint256", "name": "amount", "type": "uint256" }
    ],
    "name": "Claimed",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      { "indexed": true, "internalType": "uint256", "name": "projectId", "type": "uint256" },
      { "indexed": true, "internalType": "address", "name": "user", "type": "address" },
      { "indexed": false, "internalType": "uint256", "name": "jccAmount", "type": "uint256" }
    ],
    "name": "Refunded",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      { "indexed": true, "internalType": "uint256", "name": "projectId", "type": "uint256" },
      { "indexed": true, "internalType": "address", "name": "projectOwner", "type": "address" },
      { "indexed": false, "internalType": "uint256", "name": "amount", "type": "uint256" },
      { "indexed": false, "internalType": "uint256", "name": "platformFee", "type": "uint256" }
    ],
    "name": "RaisedFundsWithdrawn",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      { "indexed": true, "internalType": "uint256", "name": "projectId", "type": "uint256" },
      { "indexed": true, "internalType": "address", "name": "projectOwner", "type": "address" },
      { "indexed": false, "internalType": "uint256", "name": "amount", "type": "uint256" }
    ],
    "name": "ProjectTokensWithdrawn",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      { "indexed": false, "internalType": "uint256", "name": "successFeeRate", "type": "uint256" },
      { "indexed": false, "internalType": "address", "name": "feeReceiver", "type": "address" }
    ],
    "name": "PlatformFeesUpdated",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      { "indexed": true, "internalType": "uint256", "name": "projectId", "type": "uint256" }
    ],
    "name": "ProjectCancelled",
    "type": "event"
  }
]`

var idoV2ABIJson = `[
  {
    "inputs": [
      {"internalType":"uint256","name":"projectId","type":"uint256"},
      {"internalType":"address","name":"projectToken","type":"address"},
      {"internalType":"address","name":"paymentToken","type":"address"},
      {"internalType":"address","name":"projectOwner","type":"address"},
      {"internalType":"address","name":"expectedFeeReceiver","type":"address"},
      {"internalType":"uint16","name":"expectedSuccessFeeRate","type":"uint16"},
      {"internalType":"uint256","name":"tokenPrice","type":"uint256"},
      {"internalType":"uint256","name":"hardCap","type":"uint256"},
      {"internalType":"uint256","name":"softCap","type":"uint256"},
      {"internalType":"uint256","name":"maxPerUser","type":"uint256"},
      {"internalType":"uint256","name":"startTime","type":"uint256"},
      {"internalType":"uint256","name":"endTime","type":"uint256"},
      {"internalType":"uint256[]","name":"vestingTimes","type":"uint256[]"},
      {"internalType":"uint256[]","name":"vestingPercents","type":"uint256[]"}
    ],
    "name":"createProject","outputs":[],"stateMutability":"nonpayable","type":"function"
  },
  {
    "inputs":[{"internalType":"uint256","name":"projectId","type":"uint256"}],
    "name":"activateProject","outputs":[],"stateMutability":"nonpayable","type":"function"
  },
  {
    "inputs":[{"internalType":"uint256","name":"projectId","type":"uint256"}],
    "name":"getProject","outputs":[{"components":[
      {"internalType":"uint256","name":"projectId","type":"uint256"},
      {"internalType":"address","name":"projectToken","type":"address"},
      {"internalType":"address","name":"paymentToken","type":"address"},
      {"internalType":"address","name":"projectOwner","type":"address"},
      {"internalType":"address","name":"feeReceiver","type":"address"},
      {"internalType":"uint8","name":"projectTokenDecimals","type":"uint8"},
      {"internalType":"uint8","name":"paymentTokenDecimals","type":"uint8"},
      {"internalType":"uint16","name":"successFeeRate","type":"uint16"},
      {"internalType":"uint8","name":"status","type":"uint8"},
      {"internalType":"uint256","name":"tokenPrice","type":"uint256"},
      {"internalType":"uint256","name":"hardCap","type":"uint256"},
      {"internalType":"uint256","name":"softCap","type":"uint256"},
      {"internalType":"uint256","name":"maxPerUser","type":"uint256"},
      {"internalType":"uint256","name":"startTime","type":"uint256"},
      {"internalType":"uint256","name":"endTime","type":"uint256"},
      {"internalType":"uint256","name":"totalRaised","type":"uint256"},
      {"internalType":"uint256","name":"totalTokensSold","type":"uint256"},
      {"internalType":"uint256","name":"totalTokensDeposited","type":"uint256"},
      {"internalType":"bool","name":"fundsWithdrawn","type":"bool"},
      {"internalType":"bool","name":"platformFeeWithdrawn","type":"bool"},
      {"internalType":"bool","name":"projectTokensWithdrawn","type":"bool"},
      {"internalType":"uint256[]","name":"vestingTimes","type":"uint256[]"},
      {"internalType":"uint256[]","name":"vestingPercents","type":"uint256[]"}
    ],"internalType":"struct JoinCareIDOv2.Project","name":"","type":"tuple"}],"stateMutability":"view","type":"function"
  },
  {
    "inputs":[{"internalType":"uint256","name":"projectId","type":"uint256"},{"internalType":"address","name":"user","type":"address"}],
    "name":"getUserPurchase","outputs":[{"components":[
      {"internalType":"uint256","name":"paymentAmount","type":"uint256"},
      {"internalType":"uint256","name":"tokensBought","type":"uint256"},
      {"internalType":"uint256","name":"claimedTokens","type":"uint256"},
      {"internalType":"bool","name":"refunded","type":"bool"}
    ],"internalType":"struct JoinCareIDOv2.UserPurchase","name":"","type":"tuple"}],"stateMutability":"view","type":"function"
  },
  {
    "inputs":[{"internalType":"uint256","name":"projectId","type":"uint256"}],
    "name":"getProjectParticipants","outputs":[{"internalType":"address[]","name":"","type":"address[]"}],"stateMutability":"view","type":"function"
  },
  {
    "inputs":[{"internalType":"address","name":"","type":"address"}],
    "name":"paymentTokenConfigs","outputs":[{"internalType":"bool","name":"allowed","type":"bool"},{"internalType":"uint8","name":"decimals","type":"uint8"}],"stateMutability":"view","type":"function"
  },
  {
    "inputs":[{"internalType":"address","name":"","type":"address"}],
    "name":"projectTokenConfigs","outputs":[{"internalType":"bool","name":"allowed","type":"bool"},{"internalType":"uint8","name":"decimals","type":"uint8"}],"stateMutability":"view","type":"function"
  },
  {
    "inputs":[{"internalType":"address","name":"","type":"address"}],
    "name":"reservedBalances","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"
  },
  {
    "inputs":[{"internalType":"uint256","name":"projectId","type":"uint256"}],
    "name":"endProject","outputs":[],"stateMutability":"nonpayable","type":"function"
  },
  {
    "inputs":[{"internalType":"uint256","name":"projectId","type":"uint256"}],
    "name":"refund","outputs":[],"stateMutability":"nonpayable","type":"function"
  },
  {
    "inputs":[{"internalType":"uint256","name":"projectId","type":"uint256"},{"internalType":"address[]","name":"users","type":"address[]"}],
    "name":"batchRefund","outputs":[],"stateMutability":"nonpayable","type":"function"
  },
  {
    "inputs":[{"internalType":"uint256","name":"projectId","type":"uint256"}],
    "name":"withdrawRaisedFunds","outputs":[],"stateMutability":"nonpayable","type":"function"
  },
  {
    "inputs":[{"internalType":"uint256","name":"projectId","type":"uint256"}],
    "name":"withdrawPlatformFee","outputs":[],"stateMutability":"nonpayable","type":"function"
  },
  {
    "inputs":[{"internalType":"uint256","name":"projectId","type":"uint256"}],
    "name":"withdrawProjectTokens","outputs":[],"stateMutability":"nonpayable","type":"function"
  },
  {
    "inputs":[{"internalType":"uint256","name":"projectId","type":"uint256"}],
    "name":"cancelProject","outputs":[],"stateMutability":"nonpayable","type":"function"
  },
  {
    "inputs":[],"name":"pause","outputs":[],"stateMutability":"nonpayable","type":"function"
  },
  {
    "inputs":[],"name":"unpause","outputs":[],"stateMutability":"nonpayable","type":"function"
  },
  {
    "inputs":[],"name":"paused","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"
  },
  {
    "inputs":[],"name":"operator","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"
  }
]`

type IDOV2ProjectSnapshot struct {
	ProjectID              *big.Int       `abi:"projectId"`
	ProjectToken           common.Address `abi:"projectToken"`
	PaymentToken           common.Address `abi:"paymentToken"`
	ProjectOwner           common.Address `abi:"projectOwner"`
	FeeReceiver            common.Address `abi:"feeReceiver"`
	ProjectTokenDecimals   uint8          `abi:"projectTokenDecimals"`
	PaymentTokenDecimals   uint8          `abi:"paymentTokenDecimals"`
	SuccessFeeRate         uint16         `abi:"successFeeRate"`
	Status                 uint8          `abi:"status"`
	TokenPrice             *big.Int       `abi:"tokenPrice"`
	HardCap                *big.Int       `abi:"hardCap"`
	SoftCap                *big.Int       `abi:"softCap"`
	MaxPerUser             *big.Int       `abi:"maxPerUser"`
	StartTime              *big.Int       `abi:"startTime"`
	EndTime                *big.Int       `abi:"endTime"`
	TotalRaised            *big.Int       `abi:"totalRaised"`
	TotalTokensSold        *big.Int       `abi:"totalTokensSold"`
	TotalTokensDeposited   *big.Int       `abi:"totalTokensDeposited"`
	FundsWithdrawn         bool           `abi:"fundsWithdrawn"`
	PlatformFeeWithdrawn   bool           `abi:"platformFeeWithdrawn"`
	ProjectTokensWithdrawn bool           `abi:"projectTokensWithdrawn"`
	VestingTimes           []*big.Int     `abi:"vestingTimes"`
	VestingPercents        []*big.Int     `abi:"vestingPercents"`
}

type IDOV2UserPurchaseSnapshot struct {
	PaymentAmount *big.Int `abi:"paymentAmount"`
	TokensBought  *big.Int `abi:"tokensBought"`
	ClaimedTokens *big.Int `abi:"claimedTokens"`
	Refunded      bool     `abi:"refunded"`
}

type IDOV2TokenConfigSnapshot struct {
	Allowed  bool
	Decimals uint8
}

var taskABIJson = `[
  {
    "inputs": [
      { "internalType": "address", "name": "user", "type": "address" }
    ],
    "name": "hasCheckedInToday",
    "outputs": [
      { "internalType": "bool", "name": "", "type": "bool" }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      { "internalType": "address", "name": "user", "type": "address" },
      { "internalType": "uint256", "name": "day", "type": "uint256" },
      { "internalType": "bytes32", "name": "nonce", "type": "bytes32" },
      { "internalType": "uint256", "name": "deadline", "type": "uint256" },
      { "internalType": "bytes", "name": "signature", "type": "bytes" }
    ],
    "name": "checkInWithSig",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "anonymous": false,
    "inputs": [
      { "indexed": true, "internalType": "address", "name": "user", "type": "address" },
      { "indexed": true, "internalType": "uint256", "name": "day", "type": "uint256" },
      { "indexed": false, "internalType": "uint256", "name": "timestamp", "type": "uint256" }
    ],
    "name": "UserCheckedIn",
    "type": "event"
  }
]`

const (
	ProjectCreated         = "ProjectCreated"
	ProjectTokensDeposited = "ProjectTokensDeposited"
	ProjectActivated       = "ProjectActivated"
	Purchase               = "Purchase"
	ProjectEnded           = "ProjectEnded"
	Claimed                = "Claimed"
	Refunded               = "Refunded"
	RaisedFundsWithdrawn   = "RaisedFundsWithdrawn"
	ProjectTokensWithdrawn = "ProjectTokensWithdrawn"
	PlatformFeesUpdated    = "PlatformFeesUpdated"
	ProjectCancelled       = "ProjectCancelled"
	UserCheckedIn          = "UserCheckedIn"

	projectCreated         = "ProjectCreated(uint256,address,address,uint256,uint256,uint256)"
	projectTokensDeposited = "ProjectTokensDeposited(uint256,uint256)"
	projectActivated       = "ProjectActivated(uint256)"
	purchase               = "Purchase(uint256,address,uint256,uint256)"
	projectEnded           = "ProjectEnded(uint256,uint8,uint256)"
	claimed                = "Claimed(uint256,address,uint256)"
	refunded               = "Refunded(uint256,address,uint256)"
	raisedFundsWithdrawn   = "RaisedFundsWithdrawn(uint256,address,uint256,uint256)"
	projectTokensWithdrawn = "ProjectTokensWithdrawn(uint256,address,uint256)"
	platformFeesUpdated    = "PlatformFeesUpdated(uint256,address)"
	projectCancelled       = "ProjectCancelled(uint256)"
	userCheckedIn          = "UserCheckedIn(address,uint256,uint256)"
)

// EventTopic 返回指定事件的 keccak256 topic（供外部包 filter 用）
func EventTopic(name string) common.Hash {
	return eventSignatures[name]
}

// 事件签名
var eventSignatures = map[string]common.Hash{
	ProjectCreated:         crypto.Keccak256Hash([]byte(projectCreated)),
	ProjectTokensDeposited: crypto.Keccak256Hash([]byte(projectTokensDeposited)),
	ProjectActivated:       crypto.Keccak256Hash([]byte(projectActivated)),
	Purchase:               crypto.Keccak256Hash([]byte(purchase)),
	ProjectEnded:           crypto.Keccak256Hash([]byte(projectEnded)),
	Claimed:                crypto.Keccak256Hash([]byte(claimed)),
	Refunded:               crypto.Keccak256Hash([]byte(refunded)),
	RaisedFundsWithdrawn:   crypto.Keccak256Hash([]byte(raisedFundsWithdrawn)),
	ProjectTokensWithdrawn: crypto.Keccak256Hash([]byte(projectTokensWithdrawn)),
	PlatformFeesUpdated:    crypto.Keccak256Hash([]byte(platformFeesUpdated)),
	ProjectCancelled:       crypto.Keccak256Hash([]byte(projectCancelled)),
	UserCheckedIn:          crypto.Keccak256Hash([]byte(userCheckedIn)),
}

type Data struct {
	Event       string `json:"-"`
	BlockNumber uint64 `json:"-"`
	BlockHash   string `json:"-"`
	TxHash      string `json:"-"`
	LogIndex    uint   `json:"-"`

	ProjectID    int64           `json:"projectId,omitempty"`
	ProjectToken common.Address  `json:"projectToken,omitempty"`
	ProjectOwner common.Address  `json:"projectOwner,omitempty"`
	User         common.Address  `json:"user,omitempty"`
	TokenPrice   decimal.Decimal `json:"tokenPrice,omitempty"`
	HardCap      decimal.Decimal `json:"hardCap,omitempty"`
	SoftCap      decimal.Decimal `json:"softCap,omitempty"`
	JccAmount    decimal.Decimal `json:"jccAmount,omitempty"`
	TokensAmount decimal.Decimal `json:"tokensAmount,omitempty"`
	Amount       decimal.Decimal `json:"amount,omitempty"`
	TotalRaised  decimal.Decimal `json:"totalRaised,omitempty"`
	PlatformFee  decimal.Decimal `json:"platformFee,omitempty"`
	FeeRate      decimal.Decimal `json:"feeRate,omitempty"`
	FeeReceiver  common.Address  `json:"feeReceiver,omitempty"`
	OldStatus    string          `json:"oldStatus,omitempty"`
	NewStatus    string          `json:"newStatus,omitempty"`
	Day          int64           `json:"day,omitempty"`
	Timestamp    int64           `json:"timestamp,omitempty"`
	Nonce        common.Hash     `json:"nonce,omitempty"`
}

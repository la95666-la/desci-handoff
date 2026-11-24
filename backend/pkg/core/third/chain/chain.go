package chain

import (
	"crypto/ecdsa"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"joincare_server/pkg/common/auth/aes"
	commonChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
)

type Config struct {
	Name            string
	URL             string
	Private         string
	Token           string
	ChainID         int64
	ContractAddress string
}

var (
	mu      sync.RWMutex
	clients = make(map[string]*commonChain.Client)
)

func NewClient(config Config) error {
	_, err := newClient(config)
	return err
}

func newClient(config Config) (*commonChain.Client, error) {
	key := strings.TrimSpace(config.Name)
	if key == "" {
		return nil, fmt.Errorf("chain name is empty")
	}

	address, err := getContractAddress(config.ContractAddress)
	if err != nil {
		return nil, err
	}

	mu.RLock()
	chainClient := clients[key]
	mu.RUnlock()
	if chainClient != nil {
		return chainClient, nil
	}

	mu.Lock()
	defer mu.Unlock()

	if clients == nil {
		clients = make(map[string]*commonChain.Client)
	}
	if chainClient = clients[key]; chainClient != nil {
		return chainClient, nil
	}

	if strings.TrimSpace(config.URL) == "" {
		return nil, fmt.Errorf("chain url is empty")
	}
	if config.ChainID == 0 {
		return nil, fmt.Errorf("chain id is empty")
	}
	privateKey, err := decryptPrivateKey(config.Private, config.Token)
	if err != nil {
		return nil, err
	}

	if key == constant.ChainTask {
		chainClient, err = commonChain.NewTaskClient(config.URL, privateKey, address, big.NewInt(config.ChainID))
	} else if key == constant.ChainProjectV2 {
		chainClient, err = commonChain.NewIDOV2Client(config.URL, privateKey, address, big.NewInt(config.ChainID))
	} else if key == constant.ChainProjectV3 {
		chainClient, err = commonChain.NewIDOV3Client(config.URL, privateKey, address, big.NewInt(config.ChainID))
	} else {
		chainClient, err = commonChain.NewClient(config.URL, privateKey, address, big.NewInt(config.ChainID))
	}
	if err != nil {
		return nil, err
	}
	clients[key] = chainClient
	return chainClient, nil
}

func decryptPrivateKey(private, token string) (*ecdsa.PrivateKey, error) {
	private = strings.TrimSpace(private)
	// 优先当明文 hex 私钥试（64 位 hex，可带 0x 前缀）
	// 长度精确匹配才 fallback 明文，避免误把加密串当明文解
	plain := strings.TrimPrefix(private, "0x")
	if len(plain) == 64 {
		if pk, err := crypto.HexToECDSA(plain); err == nil {
			return pk, nil
		}
	}
	// AES-GCM 加密 + base64 编码路径（cmd/tool/encrypt-privkey 生成的格式）
	encodeKey, err := base64.StdEncoding.DecodeString(private)
	if err != nil {
		return nil, fmt.Errorf("private key not plaintext hex (64 chars) or valid base64: %w", err)
	}
	key, err := aes.DecryptAESGCM(encodeKey, []byte(token))
	if err != nil {
		return nil, err
	}
	privateKey, err := crypto.HexToECDSA(string(key)) // 不要泄露主网私钥
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

func getContractAddress(contractAddress string) (common.Address, error) {
	address := strings.TrimSpace(contractAddress)
	if !common.IsHexAddress(address) {
		return common.Address{}, fmt.Errorf("invalid chain contract address: %s", address)
	}
	return common.HexToAddress(address), nil
}

func GetClient(name string) *commonChain.Client {
	key := strings.TrimSpace(name)

	mu.RLock()
	defer mu.RUnlock()

	return clients[key]
}

func GetClients() map[string]*commonChain.Client {
	mu.RLock()
	defer mu.RUnlock()

	res := make(map[string]*commonChain.Client, len(clients))
	for name, client := range clients {
		res[name] = client
	}
	return res
}

func Close() {
	mu.Lock()
	defer mu.Unlock()

	for _, client := range clients {
		if client != nil && client.Client != nil {
			client.Client.Close()
		}
	}
	clients = nil
}

// UpdatePlatformFee 已删除（安全审计 LOW-2）：无调用方的死代码，且内部走 relayer
// 私钥直发（SetPlatformFees），会破坏 chain_relayer_nonce 对账栅栏。

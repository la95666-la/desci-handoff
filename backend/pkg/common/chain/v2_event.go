package chain

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	V2ProjectCreated           = "ProjectCreated"
	V2ProjectTokensDeposited   = "ProjectTokensDeposited"
	V2ProjectActivated         = "ProjectActivated"
	V2Purchased                = "Purchased"
	V2ProjectEnded             = "ProjectEnded"
	V2ProjectCancelled         = "ProjectCancelled"
	V2Refunded                 = "Refunded"
	V2Claimed                  = "Claimed"
	V2ProjectProceedsWithdrawn = "ProjectProceedsWithdrawn"
	V2PlatformFeeWithdrawn     = "PlatformFeeWithdrawn"
	V2ProjectTokensWithdrawn   = "ProjectTokensWithdrawn"
)

var v2EventTopics = map[common.Hash]string{
	crypto.Keccak256Hash([]byte("ProjectCreated(uint256,address,address,address,address,uint8,uint8,uint16,uint256,uint256,uint256,uint256,uint256,uint256)")): V2ProjectCreated,
	crypto.Keccak256Hash([]byte("ProjectTokensDeposited(uint256,address,uint256)")):                                                                            V2ProjectTokensDeposited,
	crypto.Keccak256Hash([]byte("ProjectActivated(uint256)")):                                                                                                  V2ProjectActivated,
	crypto.Keccak256Hash([]byte("Purchased(uint256,address,address,uint256,address,uint256)")):                                                                 V2Purchased,
	crypto.Keccak256Hash([]byte("ProjectEnded(uint256,uint8,address,uint256)")):                                                                                V2ProjectEnded,
	crypto.Keccak256Hash([]byte("ProjectCancelled(uint256,address,uint256)")):                                                                                  V2ProjectCancelled,
	crypto.Keccak256Hash([]byte("Refunded(uint256,address,address,uint256)")):                                                                                  V2Refunded,
	crypto.Keccak256Hash([]byte("Claimed(uint256,address,address,uint256)")):                                                                                   V2Claimed,
	crypto.Keccak256Hash([]byte("ProjectProceedsWithdrawn(uint256,address,address,uint256,uint256)")):                                                          V2ProjectProceedsWithdrawn,
	crypto.Keccak256Hash([]byte("PlatformFeeWithdrawn(uint256,address,address,uint256)")):                                                                      V2PlatformFeeWithdrawn,
	crypto.Keccak256Hash([]byte("ProjectTokensWithdrawn(uint256,address,address,uint256)")):                                                                    V2ProjectTokensWithdrawn,
}

var v2ApprovedNonMaterialTopics = map[common.Hash]struct{}{
	crypto.Keccak256Hash([]byte("OwnershipTransferred(address,address)")):      {},
	crypto.Keccak256Hash([]byte("OwnershipTransferStarted(address,address)")): {},
	crypto.Keccak256Hash([]byte("Initialized(uint64)")):                        {},
	crypto.Keccak256Hash([]byte("Paused(address)")):                            {},
	crypto.Keccak256Hash([]byte("Unpaused(address)")):                          {},
	crypto.Keccak256Hash([]byte("PaymentTokenConfigured(address,bool,uint8)")): {},
	crypto.Keccak256Hash([]byte("ProjectTokenConfigured(address,bool,uint8)")): {},
	crypto.Keccak256Hash([]byte("PlatformFeesUpdated(uint16,address)")):        {},
	crypto.Keccak256Hash([]byte("OperatorUpdated(address,address)")):           {},
}

var V2UpgradedTopic = crypto.Keccak256Hash([]byte("Upgraded(address)"))

// V2PausedTopic / V2UnpausedTopic are the global governance pause events. They
// carry no financial amounts, but the chain event consumer uses them to advance
// the matching global_pause / global_unpause durable submission to applied and
// release the relayer signer fence.
var (
	V2PausedTopic   = crypto.Keccak256Hash([]byte("Paused(address)"))
	V2UnpausedTopic = crypto.Keccak256Hash([]byte("Unpaused(address)"))
)

func IsApprovedV2NonMaterialTopic(topic common.Hash) bool {
	_, ok := v2ApprovedNonMaterialTopics[topic]
	return ok
}

// V2Event preserves every financial amount in canonical uint256 base-10 raw units.
type V2Event struct {
	Event           string         `json:"event"`
	ContractAddress common.Address `json:"contractAddress"`
	BlockNumber     uint64         `json:"blockNumber"`
	BlockHash       string         `json:"blockHash"`
	TxHash          string         `json:"txHash"`
	LogIndex        uint           `json:"logIndex"`
	ProjectID       uint64         `json:"projectId"`
	ProjectToken    common.Address `json:"projectToken,omitempty"`
	PaymentToken    common.Address `json:"paymentToken,omitempty"`
	ProjectOwner    common.Address `json:"projectOwner,omitempty"`
	FeeReceiver     common.Address `json:"feeReceiver,omitempty"`
	User            common.Address `json:"user,omitempty"`
	ProjectDecimals uint8          `json:"projectTokenDecimals,omitempty"`
	PaymentDecimals uint8          `json:"paymentTokenDecimals,omitempty"`
	FeeRate         uint16         `json:"successFeeRate,omitempty"`
	Status          uint8          `json:"status,omitempty"`
	TokenPriceRaw   string         `json:"tokenPriceRaw,omitempty"`
	HardCapRaw      string         `json:"hardCapRaw,omitempty"`
	SoftCapRaw      string         `json:"softCapRaw,omitempty"`
	MaxPerUserRaw   string         `json:"maxPerUserRaw,omitempty"`
	StartTimeRaw    string         `json:"startTimeRaw,omitempty"`
	EndTimeRaw      string         `json:"endTimeRaw,omitempty"`
	PaymentRaw      string         `json:"paymentAmountRaw,omitempty"`
	ProjectTokenRaw string         `json:"projectTokenAmountRaw,omitempty"`
	TotalRaisedRaw  string         `json:"totalRaisedRaw,omitempty"`
	PlatformFeeRaw  string         `json:"platformFeeRaw,omitempty"`
}

func DecodeV2Log(info types.Log) (*V2Event, error) {
	if len(info.Topics) == 0 {
		return nil, nil
	}
	eventName, ok := v2EventTopics[info.Topics[0]]
	if !ok {
		return nil, nil
	}
	event := &V2Event{
		Event: eventName, ContractAddress: info.Address, BlockNumber: info.BlockNumber,
		BlockHash: info.BlockHash.Hex(), TxHash: info.TxHash.Hex(), LogIndex: info.Index,
	}
	var err error
	switch eventName {
	case V2ProjectCreated:
		if err = requireV2Shape(info, 4, 11); err == nil {
			event.ProjectID, err = v2TopicUint64(info.Topics[1], "projectId")
		}
		if err == nil {
			event.ProjectToken = topicAddress(info.Topics[2])
			event.PaymentToken = topicAddress(info.Topics[3])
		}
		if err == nil {
			event.ProjectOwner = dataAddress(info.Data, 0)
			event.FeeReceiver = dataAddress(info.Data, 1)
		}
		if err == nil {
			event.ProjectDecimals, err = dataUint8(info.Data, 2, "projectTokenDecimals")
		}
		if err == nil {
			event.PaymentDecimals, err = dataUint8(info.Data, 3, "paymentTokenDecimals")
		}
		if err == nil {
			event.FeeRate, err = dataUint16(info.Data, 4, "successFeeRate")
		}
		if err == nil {
			event.TokenPriceRaw = dataRaw(info.Data, 5)
			event.HardCapRaw = dataRaw(info.Data, 6)
			event.SoftCapRaw = dataRaw(info.Data, 7)
			event.MaxPerUserRaw = dataRaw(info.Data, 8)
			event.StartTimeRaw = dataRaw(info.Data, 9)
			event.EndTimeRaw = dataRaw(info.Data, 10)
		}
	case V2ProjectTokensDeposited:
		err = requireV2Shape(info, 3, 1)
		if err == nil {
			event.ProjectID, err = v2TopicUint64(info.Topics[1], "projectId")
		}
		if err == nil {
			event.ProjectToken = topicAddress(info.Topics[2])
			event.ProjectTokenRaw = dataRaw(info.Data, 0)
		}
	case V2ProjectActivated:
		err = requireV2Shape(info, 2, 0)
		if err == nil {
			event.ProjectID, err = v2TopicUint64(info.Topics[1], "projectId")
		}
	case V2Purchased:
		err = requireV2Shape(info, 4, 3)
		if err == nil {
			event.ProjectID, err = v2TopicUint64(info.Topics[1], "projectId")
		}
		if err == nil {
			event.User = topicAddress(info.Topics[2])
			event.PaymentToken = topicAddress(info.Topics[3])
			event.PaymentRaw = dataRaw(info.Data, 0)
			event.ProjectToken = dataAddress(info.Data, 1)
			event.ProjectTokenRaw = dataRaw(info.Data, 2)
		}
	case V2ProjectEnded:
		err = requireV2Shape(info, 3, 2)
		if err == nil {
			event.ProjectID, err = v2TopicUint64(info.Topics[1], "projectId")
		}
		if err == nil {
			event.Status, err = dataUint8(info.Data, 0, "status")
		}
		if err == nil {
			event.PaymentToken = topicAddress(info.Topics[2])
			event.TotalRaisedRaw = dataRaw(info.Data, 1)
		}
	case V2ProjectCancelled:
		err = requireV2Shape(info, 3, 1)
		if err == nil {
			event.ProjectID, err = v2TopicUint64(info.Topics[1], "projectId")
		}
		if err == nil {
			event.PaymentToken = topicAddress(info.Topics[2])
			event.TotalRaisedRaw = dataRaw(info.Data, 0)
		}
	case V2Refunded:
		err = requireV2Shape(info, 4, 1)
		if err == nil {
			event.ProjectID, err = v2TopicUint64(info.Topics[1], "projectId")
		}
		if err == nil {
			event.User = topicAddress(info.Topics[2])
			event.PaymentToken = topicAddress(info.Topics[3])
			event.PaymentRaw = dataRaw(info.Data, 0)
		}
	case V2Claimed:
		err = requireV2Shape(info, 4, 1)
		if err == nil {
			event.ProjectID, err = v2TopicUint64(info.Topics[1], "projectId")
		}
		if err == nil {
			event.User = topicAddress(info.Topics[2])
			event.ProjectToken = topicAddress(info.Topics[3])
			event.ProjectTokenRaw = dataRaw(info.Data, 0)
		}
	case V2ProjectProceedsWithdrawn:
		err = requireV2Shape(info, 4, 2)
		if err == nil {
			event.ProjectID, err = v2TopicUint64(info.Topics[1], "projectId")
		}
		if err == nil {
			event.PaymentToken = topicAddress(info.Topics[2])
			event.ProjectOwner = topicAddress(info.Topics[3])
			event.PaymentRaw = dataRaw(info.Data, 0)
			event.PlatformFeeRaw = dataRaw(info.Data, 1)
		}
	case V2PlatformFeeWithdrawn:
		err = requireV2Shape(info, 4, 1)
		if err == nil {
			event.ProjectID, err = v2TopicUint64(info.Topics[1], "projectId")
		}
		if err == nil {
			event.PaymentToken = topicAddress(info.Topics[2])
			event.FeeReceiver = topicAddress(info.Topics[3])
			event.PaymentRaw = dataRaw(info.Data, 0)
		}
	case V2ProjectTokensWithdrawn:
		err = requireV2Shape(info, 4, 1)
		if err == nil {
			event.ProjectID, err = v2TopicUint64(info.Topics[1], "projectId")
		}
		if err == nil {
			event.ProjectToken = topicAddress(info.Topics[2])
			event.ProjectOwner = topicAddress(info.Topics[3])
			event.ProjectTokenRaw = dataRaw(info.Data, 0)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("decode V2 %s: %w", eventName, err)
	}
	if event.ProjectID == 0 {
		return nil, errors.New("decode V2 event: projectId must be positive")
	}
	if err := validateV2EventFields(event); err != nil {
		return nil, fmt.Errorf("decode V2 %s fields: %w", eventName, err)
	}
	return event, nil
}

func validateV2EventFields(event *V2Event) error {
	requireAddress := func(name string, address common.Address) error {
		if address == (common.Address{}) {
			return fmt.Errorf("%s is zero", name)
		}
		return nil
	}
	switch event.Event {
	case V2ProjectCreated:
		for name, address := range map[string]common.Address{"projectToken": event.ProjectToken, "paymentToken": event.PaymentToken, "projectOwner": event.ProjectOwner, "feeReceiver": event.FeeReceiver} {
			if err := requireAddress(name, address); err != nil {
				return err
			}
		}
		if event.ProjectDecimals == 0 || event.ProjectDecimals > 18 || event.PaymentDecimals == 0 || event.PaymentDecimals > 18 {
			return errors.New("token decimals must be 1-18")
		}
		if event.FeeRate > 1000 {
			return errors.New("success fee rate exceeds 1000 basis points")
		}
	case V2ProjectTokensDeposited, V2ProjectTokensWithdrawn:
		if err := requireAddress("projectToken", event.ProjectToken); err != nil {
			return err
		}
	case V2Purchased:
		for name, address := range map[string]common.Address{"user": event.User, "paymentToken": event.PaymentToken, "projectToken": event.ProjectToken} {
			if err := requireAddress(name, address); err != nil {
				return err
			}
		}
	case V2ProjectEnded, V2ProjectCancelled:
		if err := requireAddress("paymentToken", event.PaymentToken); err != nil {
			return err
		}
	case V2Refunded:
		if err := requireAddress("user", event.User); err != nil {
			return err
		}
		if err := requireAddress("paymentToken", event.PaymentToken); err != nil {
			return err
		}
	case V2Claimed:
		if err := requireAddress("user", event.User); err != nil {
			return err
		}
		if err := requireAddress("projectToken", event.ProjectToken); err != nil {
			return err
		}
	case V2ProjectProceedsWithdrawn:
		if err := requireAddress("paymentToken", event.PaymentToken); err != nil {
			return err
		}
		if err := requireAddress("projectOwner", event.ProjectOwner); err != nil {
			return err
		}
	case V2PlatformFeeWithdrawn:
		if err := requireAddress("paymentToken", event.PaymentToken); err != nil {
			return err
		}
		if err := requireAddress("feeReceiver", event.FeeReceiver); err != nil {
			return err
		}
	}
	return nil
}

func requireV2Shape(info types.Log, topics, words int) error {
	if len(info.Topics) != topics {
		return fmt.Errorf("topics=%d want=%d", len(info.Topics), topics)
	}
	if len(info.Data) != words*32 {
		return fmt.Errorf("data bytes=%d want=%d", len(info.Data), words*32)
	}
	return nil
}

func v2TopicUint64(topic common.Hash, name string) (uint64, error) {
	value := new(big.Int).SetBytes(topic.Bytes())
	if !value.IsUint64() {
		return 0, fmt.Errorf("%s exceeds uint64", name)
	}
	return value.Uint64(), nil
}

func topicAddress(topic common.Hash) common.Address { return common.BytesToAddress(topic.Bytes()[12:]) }
func dataAddress(data []byte, word int) common.Address {
	return common.BytesToAddress(data[word*32+12 : (word+1)*32])
}
func dataRaw(data []byte, word int) string {
	return new(big.Int).SetBytes(data[word*32 : (word+1)*32]).String()
}

func dataUint8(data []byte, word int, name string) (uint8, error) {
	value := new(big.Int).SetBytes(data[word*32 : (word+1)*32])
	if !value.IsUint64() || value.Uint64() > 255 {
		return 0, fmt.Errorf("%s exceeds uint8", name)
	}
	return uint8(value.Uint64()), nil
}

func dataUint16(data []byte, word int, name string) (uint16, error) {
	value := new(big.Int).SetBytes(data[word*32 : (word+1)*32])
	if !value.IsUint64() || value.Uint64() > 65535 {
		return 0, fmt.Errorf("%s exceeds uint16", name)
	}
	return uint16(value.Uint64()), nil
}

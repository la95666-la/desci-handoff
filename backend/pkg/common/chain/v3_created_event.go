package chain

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"joincare_server/pkg/common/idov3whitelist"
)

var v3CreatedTopic = crypto.Keccak256Hash([]byte("ProjectCreated(uint256,address,address,address,address,uint8,uint8,uint16,uint256,uint256,uint256,uint256,uint256,uint256)"))

// V3CreatedEvent is a decoded, unmaterialized creation snapshot. The event
// signature is shared with V2, so a topic does not authenticate the source.
type V3CreatedEvent struct {
	ChainID, ProjectID, BlockNumber                                                uint64
	Proxy, ProjectToken, PaymentToken, ProjectOwner, FeeReceiver                   common.Address
	ProjectDecimals, PaymentDecimals                                               uint8
	FeeRate                                                                        uint16
	TokenPriceRaw, HardCapRaw, SoftCapRaw, MaxPerUserRaw, StartTimeRaw, EndTimeRaw string
	TxHash, BlockHash                                                              common.Hash
	LogIndex                                                                       uint
}

func DecodeV3CreatedLog(log types.Log, chainID uint64, proxy common.Address) (*V3CreatedEvent, error) {
	if chainID == 0 || proxy == (common.Address{}) || log.Address != proxy || log.Removed || log.BlockNumber == 0 ||
		log.BlockHash == (common.Hash{}) || log.TxHash == (common.Hash{}) || len(log.Topics) != 4 || log.Topics[0] != v3CreatedTopic || len(log.Data) != 11*32 {
		return nil, errors.New("V3 creation source or ABI invalid")
	}
	project := new(big.Int).SetBytes(log.Topics[1][:])
	if project.Sign() <= 0 || !project.IsUint64() {
		return nil, errors.New("V3 creation project invalid")
	}
	decodeAddress := func(word []byte) (common.Address, bool) {
		for _, b := range word[:12] {
			if b != 0 {
				return common.Address{}, false
			}
		}
		a := common.BytesToAddress(word[12:])
		return a, a != (common.Address{}) && a != proxy
	}
	sale, okSale := decodeAddress(log.Topics[2][:])
	payment, okPayment := decodeAddress(log.Topics[3][:])
	owner, okOwner := decodeAddress(log.Data[:32])
	receiver, okReceiver := decodeAddress(log.Data[32:64])
	if !okSale || !okPayment || !okOwner || !okReceiver || sale == payment {
		return nil, errors.New("V3 creation addresses invalid")
	}
	word := func(index int) *big.Int { return new(big.Int).SetBytes(log.Data[index*32 : (index+1)*32]) }
	projectDecimals, paymentDecimals, fee := word(2), word(3), word(4)
	if !projectDecimals.IsUint64() || projectDecimals.Uint64() == 0 || projectDecimals.Uint64() > 18 ||
		!paymentDecimals.IsUint64() || paymentDecimals.Uint64() == 0 || paymentDecimals.Uint64() > 18 ||
		!fee.IsUint64() || fee.Uint64() > 1000 {
		return nil, errors.New("V3 creation scalars invalid")
	}
	price, hard, soft, maxPerUser, start, end := word(5), word(6), word(7), word(8), word(9), word(10)
	if price.Sign() <= 0 || hard.Sign() <= 0 || soft.Sign() <= 0 || soft.Cmp(hard) > 0 ||
		maxPerUser.Sign() <= 0 || maxPerUser.Cmp(hard) > 0 || start.Sign() <= 0 || end.Cmp(start) <= 0 {
		return nil, errors.New("V3 creation funding invalid")
	}
	return &V3CreatedEvent{ChainID: chainID, ProjectID: project.Uint64(), BlockNumber: log.BlockNumber, Proxy: proxy,
		ProjectToken: sale, PaymentToken: payment, ProjectOwner: owner, FeeReceiver: receiver,
		ProjectDecimals: uint8(projectDecimals.Uint64()), PaymentDecimals: uint8(paymentDecimals.Uint64()), FeeRate: uint16(fee.Uint64()),
		TokenPriceRaw: price.String(), HardCapRaw: hard.String(), SoftCapRaw: soft.String(), MaxPerUserRaw: maxPerUser.String(),
		StartTimeRaw: start.String(), EndTimeRaw: end.String(), TxHash: log.TxHash, BlockHash: log.BlockHash, LogIndex: log.Index}, nil
}

// MatchV3CreatedBinding checks token/project identity supplied by a separate
// approved source. Neither the caller's binding nor this log is authoritative.
func MatchV3CreatedBinding(event *V3CreatedEvent, binding idov3whitelist.ExpectedProjectBinding) error {
	if event == nil || binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) ||
		binding.ProjectToken == (common.Address{}) || binding.PaymentToken == (common.Address{}) || binding.ProjectToken == binding.PaymentToken ||
		event.ChainID != binding.ChainID || event.Proxy != binding.ProxyAddress || event.ProjectID != binding.ProjectID ||
		event.ProjectToken != binding.ProjectToken || event.PaymentToken != binding.PaymentToken {
		return errors.New("V3 creation identity mismatched")
	}
	return nil
}

// MatchV3CreatedReceipt compares an independently supplied binding and a
// decoded creation event to exactly one log at its receipt index. It does not
// authenticate the binding, establish canonical finality, or write a project.
func MatchV3CreatedReceipt(event *V3CreatedEvent, binding idov3whitelist.ExpectedProjectBinding, receipt *types.Receipt) error {
	if MatchV3CreatedBinding(event, binding) != nil || receipt == nil || receipt.Status != types.ReceiptStatusSuccessful ||
		receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Sign() <= 0 ||
		receipt.BlockNumber.Uint64() != event.BlockNumber || receipt.BlockHash == (common.Hash{}) ||
		receipt.BlockHash != event.BlockHash || receipt.TxHash == (common.Hash{}) || receipt.TxHash != event.TxHash {
		return errors.New("V3 creation receipt or binding mismatch")
	}
	found := 0
	for _, log := range receipt.Logs {
		if log == nil || log.Index != event.LogIndex {
			continue
		}
		found++
		if found != 1 || log.Address != binding.ProxyAddress || log.Removed || log.BlockNumber != event.BlockNumber ||
			log.BlockHash != receipt.BlockHash || log.TxHash != receipt.TxHash {
			return errors.New("V3 creation receipt log placement mismatch")
		}
		decoded, err := DecodeV3CreatedLog(*log, binding.ChainID, binding.ProxyAddress)
		if err != nil || *decoded != *event {
			return errors.New("V3 creation receipt log content mismatch")
		}
	}
	if found != 1 {
		return errors.New("V3 creation receipt log missing or ambiguous")
	}
	return nil
}

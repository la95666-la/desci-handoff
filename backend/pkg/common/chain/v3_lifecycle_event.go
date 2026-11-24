package chain

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"joincare_server/pkg/common/idov3whitelist"
)

// V3LifecycleEvent is a raw, unmaterialized observation from an explicitly
// supplied V3 proxy. Many signatures are shared with V2: a topic is NEVER
// proof of source admission, canonicality, financial entitlement or finality.
type V3LifecycleEvent struct {
	Kind                            string
	ChainID, ProjectID, BlockNumber uint64
	Proxy, Token, Recipient         common.Address
	AmountRaw, SecondaryRaw         string
	Status                          uint8
	TxHash, BlockHash               common.Hash
	LogIndex                        uint
}

// MatchV3LifecycleBinding cross-checks a decoded log with independently
// authenticated project/token identity. Neither input's provenance is proved
// here. Zero-address recipient is only valid for events without a recipient.
func MatchV3LifecycleBinding(event *V3LifecycleEvent, binding idov3whitelist.ExpectedProjectBinding) error {
	if event == nil || binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) ||
		binding.PaymentToken == (common.Address{}) || binding.ProjectToken == (common.Address{}) ||
		binding.ProjectToken == binding.PaymentToken || event.ChainID != binding.ChainID ||
		event.ProjectID != binding.ProjectID || event.Proxy != binding.ProxyAddress {
		return errors.New("V3 lifecycle identity mismatched")
	}
	var expected common.Address
	switch event.Kind {
	case "ProjectTokensDeposited", "Claimed", "ProjectTokensWithdrawn":
		expected = binding.ProjectToken
	case "ProjectEnded", "ProjectCancelled", "Refunded", "ProjectProceedsWithdrawn", "PlatformFeeWithdrawn":
		expected = binding.PaymentToken
	case "ProjectActivated":
		if event.Token != (common.Address{}) || event.Recipient != (common.Address{}) {
			return errors.New("V3 lifecycle activation identity malformed")
		}
		return nil
	default:
		return errors.New("V3 lifecycle event unsupported")
	}
	if event.Token != expected {
		return errors.New("V3 lifecycle token mismatched")
	}
	return nil
}

type v3LifecycleShape struct {
	name                      string
	topics, words             int
	token, recipient          int
	amount, secondary, status int
}

var v3LifecycleShapes = map[common.Hash]v3LifecycleShape{
	crypto.Keccak256Hash([]byte("ProjectTokensDeposited(uint256,address,uint256)")):                   {"ProjectTokensDeposited", 3, 1, 2, -1, 0, -1, -1},
	crypto.Keccak256Hash([]byte("ProjectActivated(uint256)")):                                         {"ProjectActivated", 2, 0, -1, -1, -1, -1, -1},
	crypto.Keccak256Hash([]byte("ProjectEnded(uint256,uint8,address,uint256)")):                       {"ProjectEnded", 3, 2, 2, -1, 1, -1, 0},
	crypto.Keccak256Hash([]byte("ProjectCancelled(uint256,address,uint256)")):                         {"ProjectCancelled", 3, 1, 2, -1, 0, -1, -1},
	crypto.Keccak256Hash([]byte("Refunded(uint256,address,address,uint256)")):                         {"Refunded", 4, 1, 3, 2, 0, -1, -1},
	crypto.Keccak256Hash([]byte("Claimed(uint256,address,address,uint256)")):                          {"Claimed", 4, 1, 3, 2, 0, -1, -1},
	crypto.Keccak256Hash([]byte("ProjectProceedsWithdrawn(uint256,address,address,uint256,uint256)")): {"ProjectProceedsWithdrawn", 4, 2, 2, 3, 0, 1, -1},
	crypto.Keccak256Hash([]byte("PlatformFeeWithdrawn(uint256,address,address,uint256)")):             {"PlatformFeeWithdrawn", 4, 1, 2, 3, 0, -1, -1},
	crypto.Keccak256Hash([]byte("ProjectTokensWithdrawn(uint256,address,address,uint256)")):           {"ProjectTokensWithdrawn", 4, 1, 2, 3, 0, -1, -1},
}

// DecodeV3LifecycleLog handles only the known non-purchase lifecycle events.
// It does not use DecodeV2Log, register a scanner, or write any ledger entry.
func DecodeV3LifecycleLog(log types.Log, chainID uint64, proxy common.Address) (*V3LifecycleEvent, error) {
	if chainID == 0 || proxy == (common.Address{}) || log.Address != proxy || log.Removed || log.BlockNumber == 0 ||
		log.BlockHash == (common.Hash{}) || log.TxHash == (common.Hash{}) || len(log.Topics) == 0 {
		return nil, errors.New("V3 lifecycle source invalid")
	}
	shape, ok := v3LifecycleShapes[log.Topics[0]]
	if !ok || len(log.Topics) != shape.topics || len(log.Data) != 32*shape.words {
		return nil, errors.New("V3 lifecycle ABI invalid")
	}
	project := new(big.Int).SetBytes(log.Topics[1][:])
	if !project.IsUint64() || project.Sign() <= 0 {
		return nil, errors.New("V3 lifecycle project invalid")
	}
	result := &V3LifecycleEvent{Kind: shape.name, ChainID: chainID, ProjectID: project.Uint64(), Proxy: proxy,
		BlockNumber: log.BlockNumber, BlockHash: log.BlockHash, TxHash: log.TxHash, LogIndex: log.Index}
	decodeAddress := func(index int) (common.Address, bool) {
		if index < 0 {
			return common.Address{}, true
		}
		topic := log.Topics[index]
		for _, b := range topic[:12] {
			if b != 0 {
				return common.Address{}, false
			}
		}
		a := common.BytesToAddress(topic[12:])
		return a, a != (common.Address{}) && a != proxy
	}
	result.Token, ok = decodeAddress(shape.token)
	if !ok {
		return nil, errors.New("V3 lifecycle token invalid")
	}
	result.Recipient, ok = decodeAddress(shape.recipient)
	if !ok {
		return nil, errors.New("V3 lifecycle recipient invalid")
	}
	if shape.status >= 0 {
		value := new(big.Int).SetBytes(log.Data[32*shape.status : 32*(shape.status+1)])
		if !value.IsUint64() || (value.Uint64() != 2 && value.Uint64() != 3) {
			return nil, errors.New("V3 lifecycle terminal status invalid")
		}
		result.Status = uint8(value.Uint64())
	}
	if shape.amount >= 0 {
		amount := new(big.Int).SetBytes(log.Data[32*shape.amount : 32*(shape.amount+1)])
		if amount.Sign() == 0 && shape.name != "ProjectTokensWithdrawn" && shape.name != "PlatformFeeWithdrawn" && shape.name != "ProjectCancelled" && shape.name != "ProjectEnded" {
			return nil, errors.New("V3 lifecycle amount invalid")
		}
		result.AmountRaw = amount.String()
	}
	if shape.secondary >= 0 {
		result.SecondaryRaw = new(big.Int).SetBytes(log.Data[32*shape.secondary : 32*(shape.secondary+1)]).String()
	}
	return result, nil
}

// MatchV3LifecycleReceipt checks one decoded event against the unique log at
// its index in an already-fetched successful receipt. Other lifecycle events
// in the same transaction are permitted. The supplied binding and receipt
// must be independently authenticated and continuously checked for reorgs;
// this function never materializes a project state or financial entry.
func MatchV3LifecycleReceipt(event *V3LifecycleEvent, binding idov3whitelist.ExpectedProjectBinding, receipt *types.Receipt) error {
	if MatchV3LifecycleBinding(event, binding) != nil || receipt == nil || receipt.Status != types.ReceiptStatusSuccessful ||
		receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Sign() <= 0 ||
		receipt.BlockNumber.Uint64() != event.BlockNumber || receipt.BlockHash == (common.Hash{}) ||
		receipt.BlockHash != event.BlockHash || receipt.TxHash == (common.Hash{}) || receipt.TxHash != event.TxHash {
		return errors.New("V3 lifecycle receipt or binding mismatch")
	}
	found := 0
	for _, log := range receipt.Logs {
		if log == nil || log.Index != event.LogIndex {
			continue
		}
		found++
		if found != 1 || log.Address != binding.ProxyAddress || log.Removed || log.BlockNumber != event.BlockNumber ||
			log.BlockHash != receipt.BlockHash || log.TxHash != receipt.TxHash {
			return errors.New("V3 lifecycle receipt log placement mismatched")
		}
		decoded, err := DecodeV3LifecycleLog(*log, binding.ChainID, binding.ProxyAddress)
		if err != nil || *decoded != *event {
			return errors.New("V3 lifecycle receipt log content mismatched")
		}
	}
	if found != 1 {
		return errors.New("V3 lifecycle receipt log missing or ambiguous")
	}
	return nil
}

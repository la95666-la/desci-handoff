package chain

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

var v3PurchasedTopic = crypto.Keccak256Hash([]byte("Purchased(uint256,address,address,uint256,address,uint256)"))

// V3PurchaseEvent is an unmaterialized raw-unit observation. The signature
// matches V2, so the topic alone NEVER establishes the contract version.
// Only a separately admitted V3 proxy and exact project binding can do that.
// This decoder does not register a scanner or write financial state.
type V3PurchaseEvent struct {
	ChainID         uint64
	ProxyAddress    common.Address
	ProjectID       uint64
	Buyer           common.Address
	PaymentToken    common.Address
	ProjectToken    common.Address
	PaymentRaw      string
	ProjectTokenRaw string
	TxHash          common.Hash
	BlockNumber     uint64
	BlockHash       common.Hash
	LogIndex        uint
}

// V3PurchaseBinding must be obtained from an independently admitted, immutable
// V3 project identity. Supplying arbitrary values here is NOT source admission.
type V3PurchaseBinding struct {
	ChainID      uint64
	ProxyAddress common.Address
	ProjectID    uint64
	PaymentToken common.Address
	ProjectToken common.Address
}

// DecodeV3PurchaseLog checks the source and the exact Purchased ABI shape.
// A valid log is not a canonical receipt, finality proof, funding ledger entry
// or authority to publish eligibility. The caller must verify runtime and
// project binding independently, including token identities and reorg state.
func DecodeV3PurchaseLog(info types.Log, chainID uint64, proxy common.Address) (*V3PurchaseEvent, error) {
	if chainID == 0 || proxy == (common.Address{}) || info.Address != proxy || info.Removed ||
		info.BlockNumber == 0 || info.BlockHash == (common.Hash{}) || info.TxHash == (common.Hash{}) {
		return nil, errors.New("invalid V3 purchase source or placement")
	}
	if len(info.Topics) != 4 || info.Topics[0] != v3PurchasedTopic || len(info.Data) != 96 {
		return nil, errors.New("invalid V3 purchase event shape")
	}
	project := new(big.Int).SetBytes(info.Topics[1][:])
	if !project.IsUint64() || project.Sign() == 0 {
		return nil, errors.New("V3 purchase project outside database range")
	}
	// ABI-encoded indexed addresses must have twelve leading zero bytes.
	for _, topic := range info.Topics[2:4] {
		for _, b := range topic[:12] {
			if b != 0 {
				return nil, errors.New("V3 purchase indexed address is malformed")
			}
		}
	}
	for _, b := range info.Data[32:44] {
		if b != 0 {
			return nil, errors.New("V3 purchase project token is malformed")
		}
	}
	buyer := common.BytesToAddress(info.Topics[2][12:])
	payment := common.BytesToAddress(info.Topics[3][12:])
	token := common.BytesToAddress(info.Data[44:64])
	paymentAmount := new(big.Int).SetBytes(info.Data[:32])
	tokenAmount := new(big.Int).SetBytes(info.Data[64:96])
	if buyer == (common.Address{}) || payment == (common.Address{}) || token == (common.Address{}) ||
		paymentAmount.Sign() <= 0 || tokenAmount.Sign() <= 0 {
		return nil, errors.New("V3 purchase has zero address or amount")
	}
	return &V3PurchaseEvent{
		ChainID: chainID, ProxyAddress: proxy, ProjectID: project.Uint64(), Buyer: buyer,
		PaymentToken: payment, ProjectToken: token, PaymentRaw: paymentAmount.String(), ProjectTokenRaw: tokenAmount.String(),
		TxHash: info.TxHash, BlockNumber: info.BlockNumber, BlockHash: info.BlockHash, LogIndex: info.Index,
	}, nil
}

// MatchV3PurchaseReceipt verifies an already-decoded event against a supplied
// successful receipt and expected project/token identity. The receipt must be
// fetched through a separately trusted read path and checked against canonical
// headers/finality continuously; matching here NEVER writes a financial fact.
func MatchV3PurchaseReceipt(event *V3PurchaseEvent, binding V3PurchaseBinding, receipt *types.Receipt) error {
	if event == nil || receipt == nil || binding.ChainID == 0 || binding.ProxyAddress == (common.Address{}) ||
		binding.ProjectID == 0 || binding.PaymentToken == (common.Address{}) || binding.ProjectToken == (common.Address{}) ||
		event.ChainID != binding.ChainID || event.ProxyAddress != binding.ProxyAddress || event.ProjectID != binding.ProjectID ||
		event.PaymentToken != binding.PaymentToken || event.ProjectToken != binding.ProjectToken || event.Buyer == (common.Address{}) ||
		event.PaymentRaw == "" || event.ProjectTokenRaw == "" ||
		receipt.Status != types.ReceiptStatusSuccessful || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() ||
		receipt.BlockNumber.Uint64() == 0 || receipt.BlockNumber.Uint64() != event.BlockNumber ||
		receipt.BlockHash == (common.Hash{}) || receipt.BlockHash != event.BlockHash ||
		receipt.TxHash == (common.Hash{}) || receipt.TxHash != event.TxHash {
		return errors.New("V3 purchase binding or receipt mismatch")
	}
	found := 0
	for _, log := range receipt.Logs {
		if log == nil || log.Address != binding.ProxyAddress || len(log.Topics) == 0 || log.Topics[0] != v3PurchasedTopic {
			continue
		}
		if log.Index != event.LogIndex {
			continue // Other purchases in a governed batch do not authenticate this one.
		}
		found++
		decoded, err := DecodeV3PurchaseLog(*log, binding.ChainID, binding.ProxyAddress)
		if err != nil || *decoded != *event || log.TxHash != receipt.TxHash || log.BlockHash != receipt.BlockHash ||
			log.BlockNumber != receipt.BlockNumber.Uint64() {
			return errors.New("V3 purchase receipt log differs from candidate")
		}
	}
	if found != 1 {
		return errors.New("V3 purchase receipt log missing or ambiguous")
	}
	return nil
}

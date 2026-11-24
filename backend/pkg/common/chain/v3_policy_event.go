package chain

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

var v3PolicyUpdatedTopic = crypto.Keccak256Hash([]byte("WhitelistPolicyUpdated(uint256,uint64,bool,bytes32)"))

// V3PolicyEvent is a decoded log, NOT an approval, finality, or authorization
// to publish a proof. Callers must bind an independently approved source and
// full member snapshot before considering any project state transition.
type V3PolicyEvent struct {
	ChainID      uint64
	ProxyAddress common.Address
	ProjectID    uint64
	Version      uint64
	Enabled      bool
	Root         common.Hash
	TxHash       common.Hash
	BlockNumber  uint64
	BlockHash    common.Hash
	LogIndex     uint
}

// DecodeV3PolicyLog refuses unknown sources and malformed indexed or data
// words. It intentionally does not share the V2 decoder or event allowlist.
func DecodeV3PolicyLog(info types.Log, chainID uint64, proxy common.Address) (*V3PolicyEvent, error) {
	if chainID == 0 || proxy == (common.Address{}) || info.Address != proxy || info.Removed || info.BlockNumber == 0 || info.BlockHash == (common.Hash{}) || info.TxHash == (common.Hash{}) {
		return nil, errors.New("invalid V3 policy source or placement")
	}
	if len(info.Topics) != 3 || info.Topics[0] != v3PolicyUpdatedTopic || len(info.Data) != 64 {
		return nil, errors.New("invalid V3 policy event shape")
	}
	project := new(big.Int).SetBytes(info.Topics[1][:])
	version := new(big.Int).SetBytes(info.Topics[2][:])
	if !project.IsUint64() || project.Sign() == 0 || !version.IsUint64() || version.Sign() == 0 {
		return nil, errors.New("V3 policy project or version outside database range")
	}
	boolWord := new(big.Int).SetBytes(info.Data[:32])
	if boolWord.Cmp(big.NewInt(1)) > 0 {
		return nil, errors.New("V3 policy bool word is malformed")
	}
	enabled := boolWord.Sign() == 1
	root := common.BytesToHash(info.Data[32:])
	if enabled == (root == (common.Hash{})) {
		return nil, errors.New("V3 policy root and mode disagree")
	}
	return &V3PolicyEvent{ChainID: chainID, ProxyAddress: proxy, ProjectID: project.Uint64(), Version: version.Uint64(), Enabled: enabled, Root: root, TxHash: info.TxHash, BlockNumber: info.BlockNumber, BlockHash: info.BlockHash, LogIndex: info.Index}, nil
}

// MatchV3PolicyEvent additionally binds a decoded event to the immutable
// approved candidate and its *actual* executed receipt placement. Both the
// snapshot and receipt must be authenticated elsewhere. This helper never
// marks a policy effective, registers a scanner source or grants a proof.
func MatchV3PolicyEvent(event *V3PolicyEvent, chainID uint64, proxy common.Address, projectID, targetVersion uint64, enabled bool, root common.Hash, receipt *types.Receipt) error {
	if event == nil || receipt == nil || chainID == 0 || proxy == (common.Address{}) || projectID == 0 || targetVersion == 0 || enabled == (root == (common.Hash{})) ||
		event.ChainID != chainID || event.ProxyAddress != proxy || event.ProjectID != projectID || event.Version != targetVersion || event.Enabled != enabled || event.Root != root ||
		receipt.Status != types.ReceiptStatusSuccessful || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Uint64() != event.BlockNumber || receipt.BlockHash != event.BlockHash || receipt.TxHash != event.TxHash || receipt.TxHash == (common.Hash{}) {
		return errors.New("V3 policy candidate or receipt mismatch")
	}
	found := 0
	for _, log := range receipt.Logs {
		if log == nil || log.Address != proxy || len(log.Topics) == 0 || log.Topics[0] != v3PolicyUpdatedTopic {
			continue
		}
		if len(log.Topics) != 3 {
			return errors.New("malformed V3 policy receipt log")
		}
		project := new(big.Int).SetBytes(log.Topics[1][:])
		if !project.IsUint64() || project.Uint64() != projectID {
			continue
		}
		found++
		decoded, err := DecodeV3PolicyLog(*log, chainID, proxy)
		if err != nil || decoded.Version != targetVersion || decoded.Enabled != enabled || decoded.Root != root || decoded.TxHash != event.TxHash || decoded.BlockHash != event.BlockHash || decoded.BlockNumber != event.BlockNumber || decoded.LogIndex != event.LogIndex {
			return errors.New("V3 policy receipt log differs from approved candidate")
		}
	}
	if found != 1 {
		return errors.New("V3 policy receipt log missing or ambiguous")
	}
	return nil
}

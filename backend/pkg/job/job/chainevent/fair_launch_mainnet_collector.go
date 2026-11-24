package chainevent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	commonchain "joincare_server/pkg/common/chain"
)

var fairLaunchMainnetProviderIdentityPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

const (
	FairLaunchMainnetCollectorUnavailable = "fair_launch_mainnet_collector_unavailable"
	FairLaunchMainnetCollectorRejected    = "fair_launch_mainnet_collector_rejected"
	fairLaunchMainnetConfirmations        = uint64(20)
)

type fairLaunchMainnetCollectorError struct {
	reasonCode string
	retryable  bool
	message    string
}

func (err *fairLaunchMainnetCollectorError) Error() string {
	return FairLaunchMainnetCollectorRejected + ": " + err.message
}

func fairLaunchMainnetCollectorRetryable(message string) error {
	return &fairLaunchMainnetCollectorError{retryable: true, message: message}
}

func fairLaunchMainnetCollectorBlocked(reasonCode, message string) error {
	return &fairLaunchMainnetCollectorError{reasonCode: reasonCode, message: message}
}

func classifyFairLaunchMainnetCollectorError(err error) (FairLaunchMainnetArchiveFailure, bool, bool) {
	var collectorErr *fairLaunchMainnetCollectorError
	if !errors.As(err, &collectorErr) {
		return FairLaunchMainnetArchiveFailure{}, false, false
	}
	if collectorErr.retryable {
		return FairLaunchMainnetArchiveFailure{}, true, false
	}
	if _, ok := fairLaunchMainnetArchiveFailureStatus(collectorErr.reasonCode); !ok {
		return FairLaunchMainnetArchiveFailure{}, false, false
	}
	return FairLaunchMainnetArchiveFailure{ReasonCode: collectorErr.reasonCode}, false, true
}

// FairLaunchMainnetRPC is deliberately not an HTTP client or URL. A caller
// must supply a separately admitted RPC implementation and a public stable
// provider identity; credentials remain outside this collector and its logs.
type FairLaunchMainnetRPC interface {
	Call(context.Context, string, any, any) error
	ProviderIdentity() string
}

type fairLaunchMainnetRPCBlock struct {
	Hash         string   `json:"hash"`
	Number       string   `json:"number"`
	ParentHash   string   `json:"parentHash"`
	Transactions []string `json:"transactions"`
}

type fairLaunchMainnetRPCReceipt struct {
	TransactionHash  string                  `json:"transactionHash"`
	TransactionIndex string                  `json:"transactionIndex"`
	BlockHash        string                  `json:"blockHash"`
	BlockNumber      string                  `json:"blockNumber"`
	Status           string                  `json:"status"`
	Logs             []fairLaunchLocalRPCLog `json:"logs"`
}

type FairLaunchMainnetCollectedBatch struct {
	seal *fairLaunchMainnetCollectedBatchSeal
}

type fairLaunchMainnetCollectedBatchSeal struct {
	batch             FairLaunchMainnetArchiveBatch
	authorityRowID    uint64
	chainID           uint64
	sourceID          uint64
	sourceRevisionID  uint64
	leaseGeneration   uint64
	fenceEpoch        uint64
	rpcIdentityDigest string
	batchDigest       string
}

// CollectFairLaunchMainnetFinalizedBatch reads a bounded contiguous range from
// a capability-bound RPC and returns repository input only. It registers no
// cron, persists nothing, invokes no materializer, and submits no transaction.
func CollectFairLaunchMainnetFinalizedBatch(ctx context.Context, capability FairLaunchMainnetCapability, rpc FairLaunchMainnetRPC, fromBlock, throughBlock uint64, now time.Time) (FairLaunchMainnetCollectedBatch, error) {
	admission, err := capability.Admission()
	if err != nil || !capability.valid(now) || rpc == nil || admission.Component != "job" || admission.ChainID != fairLaunchMainnetChainID || admission.ConfirmationPolicy != "20_blocks" || fromBlock < admission.SourceStartBlock || fromBlock > throughBlock || throughBlock-fromBlock+1 > fairLaunchMainnetArchiveMaxBlocks || throughBlock > math.MaxUint64-fairLaunchMainnetConfirmations {
		return FairLaunchMainnetCollectedBatch{}, errors.New(FairLaunchMainnetCollectorUnavailable)
	}
	providerIdentity := rpc.ProviderIdentity()
	rpcIdentityDigest, err := FairLaunchMainnetRPCIdentityDigest(providerIdentity)
	if err != nil || rpcIdentityDigest != admission.RPCIdentityDigest {
		return FairLaunchMainnetCollectedBatch{}, errors.New(FairLaunchMainnetCollectorUnavailable)
	}
	finalizedThroughHash, err := verifyFairLaunchMainnetRPCIdentity(ctx, rpc, admission, throughBlock)
	if err != nil {
		return FairLaunchMainnetCollectedBatch{}, err
	}
	blocks := make([]FairLaunchMainnetFinalizedBlock, 0, throughBlock-fromBlock+1)
	var previousHash string
	for blockNumber := fromBlock; blockNumber <= throughBlock; blockNumber++ {
		block, err := loadFairLaunchMainnetBlock(ctx, rpc, blockNumber)
		if err != nil {
			return FairLaunchMainnetCollectedBatch{}, err
		}
		if blockNumber == admission.SourceStartBlock && block.Hash != admission.SourceStartBlockHash {
			return FairLaunchMainnetCollectedBatch{}, fairLaunchMainnetCollectorBlocked("canonical_conflict", "source start hash mismatch")
		}
		if blockNumber > fromBlock && block.ParentHash != previousHash {
			return FairLaunchMainnetCollectedBatch{}, fairLaunchMainnetCollectorBlocked("canonical_conflict", "collected block chain mismatch")
		}
		events, err := collectFairLaunchMainnetBlockReceipts(ctx, rpc, admission, block, providerIdentity)
		if err != nil {
			return FairLaunchMainnetCollectedBatch{}, err
		}
		blocks = append(blocks, FairLaunchMainnetFinalizedBlock{Number: blockNumber, Hash: block.Hash, ParentHash: block.ParentHash, ProviderIdentity: providerIdentity, Events: events})
		previousHash = block.Hash
	}
	if blocks[len(blocks)-1].Hash != finalizedThroughHash {
		return FairLaunchMainnetCollectedBatch{}, fairLaunchMainnetCollectorBlocked("canonical_conflict", "finalized anchor changed during collection")
	}
	recheckedThroughHash, err := verifyFairLaunchMainnetRPCIdentity(ctx, rpc, admission, throughBlock)
	if err != nil {
		return FairLaunchMainnetCollectedBatch{}, err
	}
	if recheckedThroughHash != finalizedThroughHash {
		return FairLaunchMainnetCollectedBatch{}, fairLaunchMainnetCollectorBlocked("canonical_conflict", "finality changed during collection")
	}
	batch := FairLaunchMainnetArchiveBatch{Blocks: blocks}
	return FairLaunchMainnetCollectedBatch{seal: &fairLaunchMainnetCollectedBatchSeal{batch: batch, authorityRowID: admission.AuthorityRowID, chainID: admission.ChainID, sourceID: admission.SourceID, sourceRevisionID: admission.SourceRevisionID, leaseGeneration: admission.LeaseGeneration, fenceEpoch: admission.FenceEpoch, rpcIdentityDigest: admission.RPCIdentityDigest, batchDigest: fairLaunchMainnetCollectedBatchDigest(batch)}}, nil
}

func verifyFairLaunchMainnetRPCIdentity(ctx context.Context, rpc FairLaunchMainnetRPC, admission FairLaunchMainnetAdmission, throughBlock uint64) (string, error) {
	if err := verifyFairLaunchMainnetRPCNetwork(ctx, rpc, admission); err != nil {
		return "", err
	}
	latest, err := loadFairLaunchMainnetBlockByTag(ctx, rpc, "latest")
	if err != nil {
		return "", err
	}
	latestNumber, ok := parseFairLaunchLocalHexUint64(latest.Number)
	if !ok || latestNumber < throughBlock+fairLaunchMainnetConfirmations {
		return "", fairLaunchMainnetCollectorRetryable("confirmation depth unavailable")
	}
	anchor, err := loadFairLaunchMainnetBlock(ctx, rpc, throughBlock)
	if err != nil {
		return "", err
	}
	previousHash := anchor.Hash
	for blockNumber := throughBlock + 1; blockNumber <= throughBlock+fairLaunchMainnetConfirmations; blockNumber++ {
		successor, err := loadFairLaunchMainnetBlock(ctx, rpc, blockNumber)
		if err != nil {
			return "", err
		}
		if successor.ParentHash != previousHash {
			return "", fairLaunchMainnetCollectorBlocked("canonical_conflict", "canonical confirmation chain mismatch")
		}
		previousHash = successor.Hash
	}
	return anchor.Hash, nil
}

func verifyFairLaunchMainnetRPCNetwork(ctx context.Context, rpc FairLaunchMainnetRPC, admission FairLaunchMainnetAdmission) error {
	var chainID string
	if err := rpc.Call(ctx, "eth_chainId", []any{}, &chainID); err != nil {
		return fairLaunchMainnetCollectorRetryable("chain ID unavailable")
	}
	if chainID != "0x38" {
		return fairLaunchMainnetCollectorBlocked("rpc_inconsistent", "chain ID mismatch")
	}
	genesis, err := loadFairLaunchMainnetBlock(ctx, rpc, 0)
	if err != nil {
		return err
	}
	if genesis.Hash != admission.RPCGenesisHash {
		return fairLaunchMainnetCollectorBlocked("rpc_inconsistent", "genesis mismatch")
	}
	return nil
}

func collectFairLaunchMainnetBlockReceipts(ctx context.Context, rpc FairLaunchMainnetRPC, admission FairLaunchMainnetAdmission, block fairLaunchMainnetRPCBlock, providerIdentity string) ([]FairLaunchMainnetArchiveLog, error) {
	blockNumber, ok := parseFairLaunchLocalHexUint64(block.Number)
	if !ok || len(block.Transactions) > int(math.MaxUint32)+1 {
		return nil, fairLaunchMainnetCollectorBlocked("rpc_inconsistent", "block number invalid")
	}
	events := make([]FairLaunchMainnetArchiveLog, 0)
	seenTransactions := make(map[string]bool, len(block.Transactions))
	var expectedRPCLogIndex uint64
	for expectedTxIndex, txHash := range block.Transactions {
		if !fairLaunchLocalHashPattern.MatchString(txHash) || txHash == "0x"+strings.Repeat("0", 64) || seenTransactions[txHash] {
			return nil, fairLaunchMainnetCollectorBlocked("rpc_inconsistent", "block transaction hash invalid")
		}
		seenTransactions[txHash] = true
		var receipt fairLaunchMainnetRPCReceipt
		if err := rpc.Call(ctx, "eth_getTransactionReceipt", []any{txHash}, &receipt); err != nil {
			return nil, fairLaunchMainnetCollectorRetryable("transaction receipt unavailable")
		}
		transactionIndex, valid := parseFairLaunchLocalHexUint32(receipt.TransactionIndex)
		if !valid || receipt.TransactionHash != txHash || receipt.BlockHash != block.Hash || receipt.BlockNumber != block.Number || transactionIndex != uint32(expectedTxIndex) || receipt.Status != "0x0" && receipt.Status != "0x1" || receipt.Status == "0x0" && len(receipt.Logs) != 0 {
			return nil, fairLaunchMainnetCollectorBlocked("receipt_mismatch", "transaction receipt mismatch")
		}
		if receipt.Status == "0x0" {
			continue
		}
		if len(receipt.Logs) > int(math.MaxUint32)+1 {
			return nil, fairLaunchMainnetCollectorBlocked("receipt_mismatch", "transaction log ordinal overflow")
		}
		for ordinal, row := range receipt.Logs {
			rpcLogIndex, valid := parseFairLaunchLocalHexUint32(row.LogIndex)
			if !valid || uint64(rpcLogIndex) != expectedRPCLogIndex || row.TransactionHash != txHash || row.TransactionIndex != receipt.TransactionIndex || row.BlockNumber != receipt.BlockNumber || row.BlockHash != receipt.BlockHash || row.Removed {
				return nil, fairLaunchMainnetCollectorBlocked("receipt_mismatch", "receipt log envelope mismatch")
			}
			expectedRPCLogIndex++
			if strings.ToLower(row.Address) != admission.ProxyAddress {
				continue
			}
			log, event, err := decodeFairLaunchMainnetReceiptLog(row, blockNumber, transactionIndex, rpcLogIndex)
			if err != nil {
				return nil, err
			}
			if event == nil {
				return nil, fairLaunchMainnetCollectorBlocked("unknown_event", "unknown proxy event")
			}
			topicsDataDigest, rawLogDigest := fairLaunchLocalLogDigests(log)
			events = append(events, FairLaunchMainnetArchiveLog{Event: event, TransactionIndex: transactionIndex, TxLogOrdinal: uint32(ordinal), ProviderIdentity: providerIdentity, TopicsDataDigest: topicsDataDigest, RawLogDigest: rawLogDigest})
		}
	}
	return events, nil
}

func decodeFairLaunchMainnetReceiptLog(row fairLaunchLocalRPCLog, blockNumber uint64, transactionIndex, rpcLogIndex uint32) (types.Log, *commonchain.FairLaunchTypedEvent, error) {
	if !fairLaunchLocalAddressPattern.MatchString(row.Address) || len(row.Topics) == 0 || !fairLaunchLocalHexBytesPattern.MatchString(row.Data) {
		return types.Log{}, nil, fairLaunchMainnetCollectorBlocked("malformed_log", "receipt log invalid")
	}
	log := types.Log{Address: common.HexToAddress(row.Address), Topics: make([]common.Hash, len(row.Topics)), Data: common.FromHex(row.Data), BlockNumber: blockNumber, TxHash: common.HexToHash(row.TransactionHash), TxIndex: uint(transactionIndex), BlockHash: common.HexToHash(row.BlockHash), Index: uint(rpcLogIndex)}
	for index, topic := range row.Topics {
		if !fairLaunchLocalHashPattern.MatchString(topic) {
			return types.Log{}, nil, fairLaunchMainnetCollectorBlocked("malformed_log", "receipt topic invalid")
		}
		log.Topics[index] = common.HexToHash(topic)
	}
	event, err := commonchain.DecodeFairLaunchTypedLog(log)
	if err != nil {
		return types.Log{}, nil, fairLaunchMainnetCollectorBlocked("malformed_log", "typed log malformed")
	}
	return log, event, nil
}

func loadFairLaunchMainnetBlock(ctx context.Context, rpc FairLaunchMainnetRPC, blockNumber uint64) (fairLaunchMainnetRPCBlock, error) {
	return loadFairLaunchMainnetBlockByTag(ctx, rpc, fmt.Sprintf("0x%x", blockNumber))
}

func loadFairLaunchMainnetBlockByTag(ctx context.Context, rpc FairLaunchMainnetRPC, tag string) (fairLaunchMainnetRPCBlock, error) {
	var block fairLaunchMainnetRPCBlock
	if err := rpc.Call(ctx, "eth_getBlockByNumber", []any{tag, false}, &block); err != nil {
		return fairLaunchMainnetRPCBlock{}, fairLaunchMainnetCollectorRetryable("block unavailable")
	}
	if !fairLaunchLocalHashPattern.MatchString(block.Hash) || !fairLaunchLocalHashPattern.MatchString(block.ParentHash) {
		return fairLaunchMainnetRPCBlock{}, fairLaunchMainnetCollectorBlocked("rpc_inconsistent", "block envelope invalid")
	}
	if tag != "latest" && block.Number != tag {
		return fairLaunchMainnetRPCBlock{}, fairLaunchMainnetCollectorBlocked("rpc_inconsistent", "block number mismatch")
	}
	return block, nil
}

func FairLaunchMainnetRPCIdentityDigest(providerIdentity string) (string, error) {
	if !fairLaunchMainnetProviderIdentityPattern.MatchString(providerIdentity) {
		return "", errors.New(FairLaunchMainnetCollectorUnavailable)
	}
	digest := sha256.Sum256(append([]byte("joincare:fair-launch:mainnet-rpc:v1\x00"), []byte(providerIdentity)...))
	return hex.EncodeToString(digest[:]), nil
}

func fairLaunchMainnetCollectedBatchDigest(batch FairLaunchMainnetArchiveBatch) string {
	events := make([]map[string]any, 0)
	for _, block := range batch.Blocks {
		for _, archivedLog := range block.Events {
			if archivedLog.Event == nil {
				events = append(events, map[string]any{"blockNumber": block.Number, "invalid": true})
				continue
			}
			payloadDigest, err := commonchain.FairLaunchTypedEventDigest(archivedLog.Event)
			if err != nil {
				payloadDigest = "invalid"
			}
			events = append(events, map[string]any{"blockNumber": block.Number, "blockHash": block.Hash, "transactionIndex": archivedLog.TransactionIndex, "txLogOrdinal": archivedLog.TxLogOrdinal, "rpcLogIndex": archivedLog.Event.Log.Index, "txHash": strings.ToLower(archivedLog.Event.Log.TxHash.Hex()), "eventName": archivedLog.Event.Name, "payloadDigest": payloadDigest, "providerIdentity": archivedLog.ProviderIdentity, "topicsDataDigest": archivedLog.TopicsDataDigest, "rawLogDigest": archivedLog.RawLogDigest})
		}
	}
	return fairLaunchV3DomainDigest("JC_FL_MAINNET_COLLECTED_BATCH_V1", map[string]any{"blocks": fairLaunchMainnetArchiveBlockEvidence(batch), "events": events})
}

// NewFairLaunchMainnetRPC binds a secret-free provider identity to an
// already-configured JSON-RPC caller. URL construction and credentials remain
// the caller's responsibility and are never retained here.
func NewFairLaunchMainnetRPC(providerIdentity string, caller interface {
	Call(context.Context, string, any, any) error
}) (FairLaunchMainnetRPC, error) {
	if caller == nil {
		return nil, errors.New(FairLaunchMainnetCollectorUnavailable)
	}
	if _, err := FairLaunchMainnetRPCIdentityDigest(providerIdentity); err != nil {
		return nil, errors.New(FairLaunchMainnetCollectorUnavailable)
	}
	return fairLaunchMainnetBoundRPC{providerIdentity: providerIdentity, caller: caller}, nil
}

type fairLaunchMainnetBoundRPC struct {
	providerIdentity string
	caller           interface {
		Call(context.Context, string, any, any) error
	}
}

func (rpc fairLaunchMainnetBoundRPC) ProviderIdentity() string { return rpc.providerIdentity }

func (rpc fairLaunchMainnetBoundRPC) Call(ctx context.Context, method string, params any, target any) error {
	return rpc.caller.Call(ctx, method, params, target)
}

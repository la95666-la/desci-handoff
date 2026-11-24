package chainevent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	commonchain "joincare_server/pkg/common/chain"
)

type fairLaunchLocalRPCLog struct {
	Address          string   `json:"address"`
	Topics           []string `json:"topics"`
	Data             string   `json:"data"`
	BlockNumber      string   `json:"blockNumber"`
	TransactionHash  string   `json:"transactionHash"`
	TransactionIndex string   `json:"transactionIndex"`
	BlockHash        string   `json:"blockHash"`
	LogIndex         string   `json:"logIndex"`
	Removed          bool     `json:"removed"`
}

func WaitFairLaunchLocalE2ETransaction(ctx context.Context, capability FairLaunchLocalE2ECapability, fromBlock uint64) ([]FairLaunchCEWrite, error) {
	if capability.seal == nil || capability.seal.rpc == nil || fromBlock < capability.seal.admission.SignedTipBlock {
		return nil, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	filter := map[string]any{"address": capability.seal.contractAddress, "fromBlock": fmt.Sprintf("0x%x", fromBlock+1), "toBlock": "latest"}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		var logs []fairLaunchLocalRPCLog
		if err := capability.seal.rpc.Call(ctx, "eth_getLogs", []any{filter}, &logs); err == nil {
			for _, row := range logs {
				if row.Removed || !fairLaunchLocalHashPattern.MatchString(row.TransactionHash) || len(row.Topics) == 0 {
					continue
				}
				if name, ok := commonchain.FairLaunchEventName(common.HexToHash(row.Topics[0])); !ok || !fairLaunchLocalForwardEvents[name] {
					continue
				}
				return LoadFairLaunchLocalE2ETransaction(ctx, capability, row.TransactionHash)
			}
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("%s: transaction wait ended", FairLaunchLocalE2EAdmissionRejected)
		case <-ticker.C:
		}
	}
}

// LoadFairLaunchLocalE2ETransaction reads one mined transaction from the
// capability-bound RPC, rejects removed/non-proxy/unknown logs, assigns the
// transaction-local ordinal, and returns only the currently supported forward
// materialization set.
func LoadFairLaunchLocalE2ETransaction(ctx context.Context, capability FairLaunchLocalE2ECapability, txHash string) ([]FairLaunchCEWrite, error) {
	if capability.seal == nil || capability.seal.rpc == nil || !fairLaunchLocalHashPattern.MatchString(txHash) {
		return nil, errors.New(FairLaunchLocalE2EAdmissionRejected)
	}
	var receipt struct {
		TransactionHash  string                  `json:"transactionHash"`
		TransactionIndex string                  `json:"transactionIndex"`
		BlockHash        string                  `json:"blockHash"`
		BlockNumber      string                  `json:"blockNumber"`
		Status           string                  `json:"status"`
		Logs             []fairLaunchLocalRPCLog `json:"logs"`
	}
	if err := capability.seal.rpc.Call(ctx, "eth_getTransactionReceipt", []any{txHash}, &receipt); err != nil || receipt.TransactionHash != txHash || receipt.Status != "0x1" || receipt.BlockHash == "" || receipt.BlockNumber == "" || len(receipt.Logs) == 0 {
		return nil, fmt.Errorf("%s: transaction receipt unavailable", FairLaunchLocalE2EAdmissionRejected)
	}
	blockNumber, ok := parseFairLaunchLocalHexUint64(receipt.BlockNumber)
	if !ok {
		return nil, fmt.Errorf("%s: receipt block number invalid", FairLaunchLocalE2EAdmissionRejected)
	}
	transactionIndex, ok := parseFairLaunchLocalHexUint32(receipt.TransactionIndex)
	if !ok || blockNumber <= capability.seal.admission.SignedTipBlock {
		return nil, fmt.Errorf("%s: receipt position invalid", FairLaunchLocalE2EAdmissionRejected)
	}
	if err := verifyFairLaunchLocalLiveBlock(capability.seal.rpc, ctx, blockNumber, receipt.BlockHash); err != nil {
		return nil, err
	}
	writes := make([]FairLaunchCEWrite, 0, len(receipt.Logs))
	for ordinal, row := range receipt.Logs {
		if row.Removed || row.TransactionHash != txHash || row.TransactionIndex != receipt.TransactionIndex || row.BlockNumber != receipt.BlockNumber || row.BlockHash != receipt.BlockHash || row.Address != capability.seal.contractAddress || len(row.Topics) == 0 || !fairLaunchLocalHashPattern.MatchString(row.Topics[0]) || !fairLaunchLocalHexBytesPattern.MatchString(row.Data) {
			continue
		}
		logIndex, valid := parseFairLaunchLocalHexUint32(row.LogIndex)
		if !valid {
			return nil, fmt.Errorf("%s: receipt log index invalid", FairLaunchLocalE2EAdmissionRejected)
		}
		log := types.Log{Address: common.HexToAddress(row.Address), Topics: make([]common.Hash, len(row.Topics)), Data: common.FromHex(row.Data), BlockNumber: blockNumber, TxHash: common.HexToHash(txHash), TxIndex: uint(transactionIndex), BlockHash: common.HexToHash(receipt.BlockHash), Index: uint(logIndex), Removed: false}
		for index, topic := range row.Topics {
			if !fairLaunchLocalHashPattern.MatchString(topic) {
				return nil, fmt.Errorf("%s: receipt topic invalid", FairLaunchLocalE2EAdmissionRejected)
			}
			log.Topics[index] = common.HexToHash(topic)
		}
		event, err := commonchain.DecodeFairLaunchTypedLog(log)
		if err != nil {
			return nil, err
		}
		if event == nil || !fairLaunchLocalForwardEvents[event.Name] {
			continue
		}
		topicsDataDigest, rawLogDigest := fairLaunchLocalLogDigests(log)
		write, err := NewFairLaunchLocalE2EWrite(capability, event, uint32(ordinal), transactionIndex, "hardhat-local-rpc-v1", topicsDataDigest, rawLogDigest)
		if err != nil {
			return nil, err
		}
		writes = append(writes, write)
	}
	if len(writes) != 1 {
		return nil, fmt.Errorf("%s: expected exactly one supported proxy event", FairLaunchLocalE2EAdmissionRejected)
	}
	return writes, nil
}

func fairLaunchLocalLogDigests(log types.Log) (string, string) {
	topicsData := make([]byte, 0, len(log.Topics)*32+len(log.Data))
	for _, topic := range log.Topics {
		topicsData = append(topicsData, topic.Bytes()...)
	}
	topicsData = append(topicsData, log.Data...)
	raw := make([]byte, 0, 20+len(topicsData))
	raw = append(raw, log.Address.Bytes()...)
	raw = append(raw, topicsData...)
	topicsDigest := sha256.Sum256(topicsData)
	rawDigest := sha256.Sum256(raw)
	return hex.EncodeToString(topicsDigest[:]), hex.EncodeToString(rawDigest[:])
}

func parseFairLaunchLocalHexUint64(value string) (uint64, bool) {
	if value == "0x0" {
		return 0, true
	}
	if len(value) < 3 || !strings.HasPrefix(value, "0x") || value[2] == '0' {
		return 0, false
	}
	parsed, err := strconv.ParseUint(value[2:], 16, 64)
	return parsed, err == nil
}

func parseFairLaunchLocalHexUint32(value string) (uint32, bool) {
	parsed, ok := parseFairLaunchLocalHexUint64(value)
	return uint32(parsed), ok && parsed <= math.MaxUint32
}

package chainevent

import (
	"context"
	jsonv2 "encoding/json/v2"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	commonchain "joincare_server/pkg/common/chain"
)

func TestCollectFairLaunchMainnetFinalizedBatchReceiptOrdinalAndFinality(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	boundRPC, err := NewFairLaunchMainnetRPC(rpc.providerIdentity, rpc)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := CollectFairLaunchMainnetFinalizedBatch(context.Background(), capability, boundRPC, 100, 100, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.seal.batch.Blocks) != 1 || len(batch.seal.batch.Blocks[0].Events) != 1 {
		t.Fatalf("unexpected collected batch: %+v", batch)
	}
	event := batch.seal.batch.Blocks[0].Events[0]
	if event.TxLogOrdinal != 1 || event.TransactionIndex != 0 || event.Event.Name != "FairLaunchActivated" || event.Event.ProjectID.Uint64() != 41 {
		t.Fatalf("receipt-local ordinal or typed event mismatch: %+v", event)
	}
	if event.RawLogDigest == "" || event.TopicsDataDigest == "" || batch.seal.batch.Blocks[0].ProviderIdentity != rpc.providerIdentity {
		t.Fatal("collector evidence missing")
	}
}

func TestNewFairLaunchMainnetRPCRejectsSecretBearingIdentityAndNilCaller(t *testing.T) {
	if _, err := NewFairLaunchMainnetRPC("https://secret.example/rpc", &testFairLaunchMainnetRPC{}); err == nil || err.Error() != FairLaunchMainnetCollectorUnavailable {
		t.Fatalf("secret-bearing identity accepted: %v", err)
	}
	if _, err := NewFairLaunchMainnetRPC("provider-a", nil); err == nil || err.Error() != FairLaunchMainnetCollectorUnavailable {
		t.Fatalf("nil caller accepted: %v", err)
	}
	digest, err := FairLaunchMainnetRPCIdentityDigest("provider-a")
	if err != nil || !fairLaunchLocalDigestPattern.MatchString(digest) {
		t.Fatalf("valid public identity digest unavailable: %s %v", digest, err)
	}
	if _, err := FairLaunchMainnetRPCIdentityDigest("HTTPS://secret"); err == nil || err.Error() != FairLaunchMainnetCollectorUnavailable {
		t.Fatalf("invalid digest producer input accepted: %v", err)
	}
}

func TestArchiveFairLaunchMainnetCollectedBatchRejectsForgedOrCrossAuthoritySeal(t *testing.T) {
	capability, rpc := fairLaunchMainnetCollectorFixture(t)
	collected, err := CollectFairLaunchMainnetFinalizedBatch(context.Background(), capability, rpc, 100, 100, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	admission, err := capability.Admission()
	if err != nil || !validFairLaunchMainnetCollectedBatch(admission, collected) {
		t.Fatal("valid collected seal rejected")
	}
	if validFairLaunchMainnetCollectedBatch(admission, FairLaunchMainnetCollectedBatch{}) {
		t.Fatal("zero collected batch accepted")
	}
	other := capability
	other.seal = &fairLaunchMainnetSeal{admission: capability.seal.admission, leaseHash: capability.seal.leaseHash}
	other.seal.admission.SourceRevisionID++
	if otherAdmission, admissionErr := other.Admission(); admissionErr != nil || validFairLaunchMainnetCollectedBatch(otherAdmission, collected) {
		t.Fatal("cross-authority collected batch accepted")
	}
	other = capability
	other.seal = &fairLaunchMainnetSeal{admission: capability.seal.admission, leaseHash: capability.seal.leaseHash}
	other.seal.admission.Component = "backend"
	if otherAdmission, admissionErr := other.Admission(); admissionErr != nil || otherAdmission.Component == "job" {
		t.Fatal("non-job authority fixture invalid")
	}
	collected.seal.batch.Blocks[0].Hash = hashMainnetArchive('f')
	if validFairLaunchMainnetCollectedBatch(admission, collected) {
		t.Fatal("mutated collected batch accepted")
	}
}

func TestCollectFairLaunchMainnetFinalizedBatchFailsClosed(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*FairLaunchMainnetCapability, *testFairLaunchMainnetRPC)
	}{
		{"provider identity", func(_ *FairLaunchMainnetCapability, rpc *testFairLaunchMainnetRPC) {
			rpc.providerIdentity = "https://secret.example/rpc"
		}},
		{"chain", func(_ *FairLaunchMainnetCapability, rpc *testFairLaunchMainnetRPC) { rpc.chainID = "0x1" }},
		{"genesis", func(_ *FairLaunchMainnetCapability, rpc *testFairLaunchMainnetRPC) {
			block := rpc.blocks[0]
			block.Hash = hashMainnetArchive('f')
			rpc.blocks[0] = block
		}},
		{"depth", func(_ *FairLaunchMainnetCapability, rpc *testFairLaunchMainnetRPC) { rpc.latest = 119 }},
		{"successor parent", func(_ *FairLaunchMainnetCapability, rpc *testFairLaunchMainnetRPC) {
			block := rpc.blocks[110]
			block.ParentHash = hashMainnetArchive('f')
			rpc.blocks[110] = block
		}},
		{"anchor changes during collection", func(_ *FairLaunchMainnetCapability, rpc *testFairLaunchMainnetRPC) {
			replacement := rpc.blocks[100]
			replacement.Hash = hashMainnetArchive('f')
			replacement.Transactions = nil
			rpc.mutateOnRead[100] = replacement
		}},
		{"finality changes after collection", func(_ *FairLaunchMainnetCapability, rpc *testFairLaunchMainnetRPC) {
			replacement := rpc.blocks[110]
			replacement.ParentHash = hashMainnetArchive('f')
			rpc.mutateOnRead[110] = replacement
			rpc.mutateAfterRead = 1
		}},
		{"failed receipt with logs", func(_ *FairLaunchMainnetCapability, rpc *testFairLaunchMainnetRPC) {
			receipt := rpc.receipts[hashMainnetArchive('d')]
			receipt.Status = "0x0"
			rpc.receipts[hashMainnetArchive('d')] = receipt
		}},
		{"unknown proxy event", func(_ *FairLaunchMainnetCapability, rpc *testFairLaunchMainnetRPC) {
			receipt := rpc.receipts[hashMainnetArchive('d')]
			receipt.Logs[1].Topics[0] = hashMainnetArchive('f')
			rpc.receipts[hashMainnetArchive('d')] = receipt
		}},
		{"rpc log index gap", func(_ *FairLaunchMainnetCapability, rpc *testFairLaunchMainnetRPC) {
			receipt := rpc.receipts[hashMainnetArchive('d')]
			receipt.Logs[1].LogIndex = "0x8"
			rpc.receipts[hashMainnetArchive('d')] = receipt
		}},
		{"wrong confirmation policy", func(capability *FairLaunchMainnetCapability, _ *testFairLaunchMainnetRPC) {
			capability.seal.admission.ConfirmationPolicy = "12_blocks"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			capability, rpc := fairLaunchMainnetCollectorFixture(t)
			test.mutate(&capability, rpc)
			if _, err := CollectFairLaunchMainnetFinalizedBatch(context.Background(), capability, rpc, 100, 100, time.Now().UTC()); err == nil {
				t.Fatal("collector mismatch accepted")
			}
		})
	}
}

type testFairLaunchMainnetRPC struct {
	providerIdentity string
	chainID          string
	latest           uint64
	blocks           map[uint64]fairLaunchMainnetRPCBlock
	receipts         map[string]fairLaunchMainnetRPCReceipt
	mutateOnRead     map[uint64]fairLaunchMainnetRPCBlock
	mutateAfterRead  uint32
	blockReads       map[uint64]uint32
	callErrors       map[string]error
}

func (rpc *testFairLaunchMainnetRPC) ProviderIdentity() string { return rpc.providerIdentity }

func (rpc *testFairLaunchMainnetRPC) Call(_ context.Context, method string, params any, target any) error {
	if err := rpc.callErrors[method]; err != nil {
		return err
	}
	var value any
	switch method {
	case "eth_chainId":
		value = rpc.chainID
	case "eth_getBlockByNumber":
		arguments := params.([]any)
		if len(arguments) != 2 || arguments[1] != false {
			return fmt.Errorf("collector must request transaction hashes")
		}
		tag := arguments[0].(string)
		var number uint64
		if tag == "latest" {
			number = rpc.latest
		} else {
			parsed, ok := parseFairLaunchLocalHexUint64(tag)
			if !ok {
				return fmt.Errorf("invalid block tag")
			}
			number = parsed
		}
		if rpc.blockReads == nil {
			rpc.blockReads = map[uint64]uint32{}
		}
		rpc.blockReads[number]++
		value = rpc.blocks[number]
		if replacement, ok := rpc.mutateOnRead[number]; ok && rpc.blockReads[number] > rpc.mutateAfterRead {
			value = replacement
		}
	case "eth_getTransactionReceipt":
		value = rpc.receipts[params.([]any)[0].(string)]
	default:
		return fmt.Errorf("unexpected method %s", method)
	}
	raw, err := jsonv2.Marshal(value)
	if err != nil {
		return err
	}
	return jsonv2.Unmarshal(raw, target)
}

func fairLaunchMainnetCollectorFixture(t *testing.T) (FairLaunchMainnetCapability, *testFairLaunchMainnetRPC) {
	t.Helper()
	now := time.Now().UTC()
	providerIdentity := "bsc-mainnet-provider-a"
	rpcIdentityDigest, err := FairLaunchMainnetRPCIdentityDigest(providerIdentity)
	if err != nil {
		t.Fatal(err)
	}
	genesisHash := hashMainnetArchive('8')
	capability := FairLaunchMainnetCapability{seal: &fairLaunchMainnetSeal{admission: FairLaunchMainnetAdmission{AuthorityRowID: 7, ChainID: 56, Component: "job", RPCGenesisHash: genesisHash, RPCIdentityDigest: rpcIdentityDigest, ProxyAddress: addressMainnet('a'), SourceStartBlock: 100, SourceStartBlockHash: hashMainnetArchive('b'), SourceRegistryDigest: repeatMainnetHex('7', 64), ConfirmationPolicy: "20_blocks", ValidUntil: now.Add(time.Hour)}}}
	blocks := map[uint64]fairLaunchMainnetRPCBlock{0: {Hash: genesisHash, Number: "0x0", ParentHash: hashMainnetArchive('9')}}
	previousHash := hashMainnetArchive('a')
	for number := uint64(100); number <= 120; number++ {
		hash := hashMainnetArchive(byte("bcdef123456789abcdef1"[number-100]))
		blocks[number] = fairLaunchMainnetRPCBlock{Hash: hash, Number: fmt.Sprintf("0x%x", number), ParentHash: previousHash}
		previousHash = hash
	}
	blocks[100] = fairLaunchMainnetRPCBlock{Hash: hashMainnetArchive('b'), Number: "0x64", ParentHash: hashMainnetArchive('a'), Transactions: []string{hashMainnetArchive('d')}}
	previousHash = blocks[100].Hash
	for number := uint64(101); number <= 120; number++ {
		block := blocks[number]
		block.ParentHash = previousHash
		blocks[number] = block
		previousHash = block.Hash
	}
	knownEvent := fairLaunchActivatedRPCLog(t, addressMainnet('a'), hashMainnetArchive('d'), blocks[100].Hash, 100, 0, 1)
	otherLog := knownEvent
	otherLog.Address = addressMainnet('f')
	otherLog.LogIndex = "0x0"
	receipt := fairLaunchMainnetRPCReceipt{TransactionHash: hashMainnetArchive('d'), TransactionIndex: "0x0", BlockHash: blocks[100].Hash, BlockNumber: "0x64", Status: "0x1", Logs: []fairLaunchLocalRPCLog{otherLog, knownEvent}}
	rpc := &testFairLaunchMainnetRPC{providerIdentity: providerIdentity, chainID: "0x38", latest: 120, blocks: blocks, receipts: map[string]fairLaunchMainnetRPCReceipt{hashMainnetArchive('d'): receipt}, mutateOnRead: map[uint64]fairLaunchMainnetRPCBlock{}, mutateAfterRead: 1, blockReads: map[uint64]uint32{}, callErrors: map[string]error{}}
	return capability, rpc
}

func fairLaunchActivatedRPCLog(t *testing.T, address, txHash, blockHash string, blockNumber uint64, txIndex, logIndex uint32) fairLaunchLocalRPCLog {
	t.Helper()
	signature := commonchain.FairLaunchEventSignatures["FairLaunchActivated"]
	topic := crypto.Keccak256Hash([]byte(signature))
	log := types.Log{Address: common.HexToAddress(address), Topics: []common.Hash{topic, common.BigToHash(big.NewInt(41))}, BlockNumber: blockNumber, TxHash: common.HexToHash(txHash), TxIndex: uint(txIndex), BlockHash: common.HexToHash(blockHash), Index: uint(logIndex)}
	return fairLaunchLocalRPCLog{Address: strings.ToLower(log.Address.Hex()), Topics: []string{strings.ToLower(log.Topics[0].Hex()), strings.ToLower(log.Topics[1].Hex())}, Data: "0x", BlockNumber: fmt.Sprintf("0x%x", blockNumber), TransactionHash: strings.ToLower(txHash), TransactionIndex: fmt.Sprintf("0x%x", txIndex), BlockHash: strings.ToLower(blockHash), LogIndex: fmt.Sprintf("0x%x", logIndex)}
}

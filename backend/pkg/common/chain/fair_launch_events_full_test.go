package chain

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestFairLaunchFrozenEventManifestCoversThirtyTwoActions(t *testing.T) {
	if len(FairLaunchEventSignatures) != 32 {
		t.Fatalf("frozen event manifest has %d entries, want 32", len(FairLaunchEventSignatures))
	}
	seen := map[common.Hash]string{}
	for name, signature := range FairLaunchEventSignatures {
		topic := crypto.Keccak256Hash([]byte(signature))
		if previous, exists := seen[topic]; exists {
			t.Fatalf("topic collision between %s and %s", previous, name)
		}
		seen[topic] = name
		if got, ok := FairLaunchEventName(topic); !ok || got != name {
			t.Fatalf("manifest route mismatch name=%s got=%s ok=%v", name, got, ok)
		}
	}
}

func TestDecodeFairLaunchRawLogPreservesUnknownMaterializationEvent(t *testing.T) {
	signature := FairLaunchEventSignatures["FairLaunchPaymentTokenInvalidated"]
	topic := crypto.Keccak256Hash([]byte(signature))
	input := types.Log{
		Address: common.HexToAddress("0x1000000000000000000000000000000000000001"),
		Topics:  []common.Hash{topic, fairTopicUint(8)}, Data: []byte{1, 2, 3}, Index: 9,
	}
	decoded, err := DecodeFairLaunchRawLog(input)
	if err != nil {
		t.Fatal(err)
	}
	if decoded == nil || decoded.Event != "FairLaunchPaymentTokenInvalidated" || decoded.ProjectID == nil || decoded.ProjectID.Uint64() != 8 || decoded.RPCLogIndex != 9 || len(decoded.Data) != 3 {
		t.Fatalf("wrong raw frozen event: %+v", decoded)
	}
}

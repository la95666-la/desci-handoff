package chainevent

import (
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	commonchain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/entity"
)

func TestFairLaunchMainnetArchiveDefaultsFailClosed(t *testing.T) {
	if err := InitializeFairLaunchMainnetArchiveCursorTx(FairLaunchMainnetCapability{}, nil, "", "", fairLaunchMainnetArchiveTestTime()); err == nil || err.Error() != FairLaunchMainnetArchiveUnavailable {
		t.Fatalf("unexpected cursor default: %v", err)
	}
	if _, err := archiveFairLaunchMainnetBatchTx(FairLaunchMainnetCapability{}, nil, FairLaunchMainnetArchiveBatch{}, fairLaunchMainnetArchiveTestTime()); err == nil || err.Error() != FairLaunchMainnetArchiveUnavailable {
		t.Fatalf("unexpected archive default: %v", err)
	}
}

func TestValidateFairLaunchMainnetArchiveBatchExactAndFailClosed(t *testing.T) {
	admission, cursor, batch := fairLaunchMainnetArchiveValidationFixture(t)
	if err := validateFairLaunchMainnetArchiveBatch(admission, cursor, batch); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*FairLaunchMainnetAdmission, *entity.FairLaunchMainnetArchiveCursorV1, *FairLaunchMainnetArchiveBatch)
	}{
		{"gap", func(_ *FairLaunchMainnetAdmission, _ *entity.FairLaunchMainnetArchiveCursorV1, value *FairLaunchMainnetArchiveBatch) {
			value.Blocks[0].Number++
		}},
		{"parent", func(_ *FairLaunchMainnetAdmission, _ *entity.FairLaunchMainnetArchiveCursorV1, value *FairLaunchMainnetArchiveBatch) {
			value.Blocks[0].ParentHash = hashMainnetArchive('f')
		}},
		{"start hash", func(_ *FairLaunchMainnetAdmission, _ *entity.FairLaunchMainnetArchiveCursorV1, value *FairLaunchMainnetArchiveBatch) {
			value.Blocks[0].Hash = hashMainnetArchive('f')
		}},
		{"proxy", func(_ *FairLaunchMainnetAdmission, _ *entity.FairLaunchMainnetArchiveCursorV1, value *FairLaunchMainnetArchiveBatch) {
			value.Blocks[0].Events[0].Event.Log.Address = common.HexToAddress(addressMainnet('b'))
		}},
		{"removed", func(_ *FairLaunchMainnetAdmission, _ *entity.FairLaunchMainnetArchiveCursorV1, value *FairLaunchMainnetArchiveBatch) {
			value.Blocks[0].Events[0].Event.Log.Removed = true
		}},
		{"source position", func(_ *FairLaunchMainnetAdmission, _ *entity.FairLaunchMainnetArchiveCursorV1, value *FairLaunchMainnetArchiveBatch) {
			value.Blocks[0].Events[0].Event.Log.BlockNumber++
		}},
		{"unknown topic", func(_ *FairLaunchMainnetAdmission, _ *entity.FairLaunchMainnetArchiveCursorV1, value *FairLaunchMainnetArchiveBatch) {
			value.Blocks[0].Events[0].Event.Log.Topics[0] = common.HexToHash("0x1234")
		}},
		{"digest", func(_ *FairLaunchMainnetAdmission, _ *entity.FairLaunchMainnetArchiveCursorV1, value *FairLaunchMainnetArchiveBatch) {
			value.Blocks[0].Events[0].RawLogDigest = "bad"
		}},
		{"typed raw mismatch", func(_ *FairLaunchMainnetAdmission, _ *entity.FairLaunchMainnetArchiveCursorV1, value *FairLaunchMainnetArchiveBatch) {
			value.Blocks[0].Events[0].Event.ProjectID = big.NewInt(42)
			value.Blocks[0].Events[0].Event.Indexed["arg0"] = big.NewInt(42)
			value.Blocks[0].Events[0].Event.RawDecimal["arg0"] = "42"
		}},
		{"order", func(_ *FairLaunchMainnetAdmission, _ *entity.FairLaunchMainnetArchiveCursorV1, value *FairLaunchMainnetArchiveBatch) {
			duplicate := value.Blocks[0].Events[0]
			value.Blocks[0].Events = append(value.Blocks[0].Events, duplicate)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidateAdmission, candidateCursor, candidateBatch := fairLaunchMainnetArchiveValidationFixture(t)
			test.mutate(&candidateAdmission, &candidateCursor, &candidateBatch)
			if err := validateFairLaunchMainnetArchiveBatch(candidateAdmission, candidateCursor, candidateBatch); err == nil || !strings.Contains(err.Error(), FairLaunchMainnetArchiveRejected) {
				t.Fatalf("expected rejection, got %v", err)
			}
		})
	}
}

func TestFairLaunchMainnetArchiveFailureClassificationAndCoordinates(t *testing.T) {
	for _, reason := range []string{"unknown_event", "malformed_log", "receipt_mismatch", "archive_conflict"} {
		if status, ok := fairLaunchMainnetArchiveFailureStatus(reason); !ok || status != "blocked" {
			t.Fatalf("reason %s classified as %s/%v", reason, status, ok)
		}
	}
	for _, reason := range []string{"canonical_conflict", "rpc_inconsistent"} {
		if status, ok := fairLaunchMainnetArchiveFailureStatus(reason); !ok || status != "reorg_detected" {
			t.Fatalf("reason %s classified as %s/%v", reason, status, ok)
		}
	}
	if _, ok := fairLaunchMainnetArchiveFailureStatus("unknown"); ok {
		t.Fatal("unknown reason must fail closed")
	}
	if !validFairLaunchMainnetArchiveFailureCoordinates(FairLaunchMainnetArchiveFailure{ReasonCode: "rpc_inconsistent"}) {
		t.Fatal("batch-level failure without coordinates should be valid")
	}
	invalid := fairLaunchMainnetArchiveFailureFixture()
	invalid.TxHash = nil
	if validFairLaunchMainnetArchiveFailureCoordinates(invalid) {
		t.Fatal("partial event coordinates must be rejected")
	}
}

func fairLaunchMainnetArchiveValidationFixture(t *testing.T) (FairLaunchMainnetAdmission, entity.FairLaunchMainnetArchiveCursorV1, FairLaunchMainnetArchiveBatch) {
	t.Helper()
	event := &commonchain.FairLaunchTypedEvent{Name: "FairLaunchActivated", ProjectID: big.NewInt(41), Indexed: map[string]any{"arg0": big.NewInt(41)}, Values: map[string]any{}, RawDecimal: map[string]string{"arg0": "41"}}
	event.Log.Address = common.HexToAddress(addressMainnet('a'))
	event.Log.TxHash = common.HexToHash(hashMainnetArchive('d'))
	event.Log.BlockHash = common.HexToHash(hashMainnetArchive('b'))
	event.Log.BlockNumber = 100
	event.Log.TxIndex = 2
	event.Log.Index = 3
	for topic, name := range commonchainTopicNamesForArchiveTest() {
		if name == event.Name {
			event.Log.Topics = []common.Hash{topic, common.BigToHash(big.NewInt(41))}
			break
		}
	}
	topicsDataDigest, rawLogDigest := fairLaunchLocalLogDigests(event.Log)
	rpcIdentityDigest, err := FairLaunchMainnetRPCIdentityDigest("provider-a")
	if err != nil {
		t.Fatal(err)
	}
	admission := FairLaunchMainnetAdmission{ChainID: 56, RPCIdentityDigest: rpcIdentityDigest, ProxyAddress: addressMainnet('a'), SourceStartBlock: 100, SourceStartBlockHash: hashMainnetArchive('b')}
	cursor := entity.FairLaunchMainnetArchiveCursorV1{ArchivedThroughBlock: 99, ArchivedThroughBlockHash: hashMainnetArchive('a')}
	batch := FairLaunchMainnetArchiveBatch{Blocks: []FairLaunchMainnetFinalizedBlock{{Number: 100, Hash: hashMainnetArchive('b'), ParentHash: hashMainnetArchive('a'), ProviderIdentity: "provider-a", Events: []FairLaunchMainnetArchiveLog{{Event: event, TransactionIndex: 2, TxLogOrdinal: 0, ProviderIdentity: "provider-a", TopicsDataDigest: topicsDataDigest, RawLogDigest: rawLogDigest}}}}}
	return admission, cursor, batch
}

func hashMainnetArchive(digit byte) string { return "0x" + strings.Repeat(string(digit), 64) }

func fairLaunchMainnetArchiveTestTime() time.Time { return time.Unix(1_800_000_000, 0).UTC() }

func commonchainTopicNamesForArchiveTest() map[common.Hash]string {
	result := make(map[common.Hash]string)
	for name, signature := range commonchain.FairLaunchEventSignatures {
		result[crypto.Keccak256Hash([]byte(signature))] = name
	}
	return result
}

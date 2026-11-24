package chainevent

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"joincare_server/pkg/common/entity"
)

func TestRecoveryEvidenceDigestsAreDeterministicAndDomainSeparated(t *testing.T) {
	entry := types.Log{
		Address:     common.HexToAddress("0x1000000000000000000000000000000000000001"),
		BlockNumber: 101,
		BlockHash:   common.HexToHash("0x" + strings.Repeat("a", 64)),
		TxHash:      common.HexToHash("0x" + strings.Repeat("b", 64)),
		TxIndex:     2,
		Index:       3,
	}
	run := entity.ChainEventReorgRecoveryRun{ID: 7, ContractAddress: entry.Address.Hex(), LegacyContractAddress: "0x2000000000000000000000000000000000000002", LegacySourceRegistered: true}
	previous := strings.Repeat("0", 64)
	logDigest := recoveryLogDigest(run, 1, entry, strings.Repeat("c", 64), previous)
	batchDigest := recoveryBatchDigest(7, 1, 100, 200, entry.BlockHash.Hex(), entry.BlockHash.Hex(), 1, 1, logDigest, previous)
	manifestDigest := recoveryManifestDigest(7, 9, 1, 1, batchDigest, logDigest)
	eventDigest := recoveryEventDigest(7, 1, "CREATED", "boundary", "", "created recovery", previous)
	for name, digest := range map[string]string{"log": logDigest, "batch": batchDigest, "manifest": manifestDigest, "event": eventDigest} {
		if len(digest) != 64 {
			t.Fatalf("%s digest has invalid length: %q", name, digest)
		}
	}
	if logDigest == batchDigest || logDigest == manifestDigest || batchDigest == manifestDigest || eventDigest == logDigest {
		t.Fatal("reorg recovery evidence digest domains collide")
	}
}

func TestRecoveryBatchSizingCoversRangeWithoutSampling(t *testing.T) {
	for _, test := range []struct {
		blocks uint64
		want   uint
	}{{1, 1}, {100, 1}, {101, 2}, {500, 5}, {501, 6}} {
		got := uint((test.blocks + reorgRecoveryBatchSize - 1) / reorgRecoveryBatchSize)
		if got != test.want {
			t.Fatalf("blocks=%d batch count=%d want=%d", test.blocks, got, test.want)
		}
	}
}

func TestRecoveryResourceLimitsStayBounded(t *testing.T) {
	if reorgRecoveryMaxBatchLogs < 1 || reorgRecoveryMaxBatchLogs > 50_000 {
		t.Fatalf("unexpected recovery batch log limit %d", reorgRecoveryMaxBatchLogs)
	}
	if reorgRecoveryMaxBatchRaw < 1024 || reorgRecoveryMaxBatchRaw > 12*1024*1024 {
		t.Fatalf("unexpected recovery raw evidence limit %d", reorgRecoveryMaxBatchRaw)
	}
}

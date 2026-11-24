package chainevent

import (
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/spf13/viper"

	"joincare_server/pkg/common/entity"
)

func TestInitialCursorForStartBlock(t *testing.T) {
	tests := []struct {
		start uint64
		want  int
	}{
		{start: 0, want: 0},
		{start: 1, want: 0},
		{start: 100, want: 99},
	}
	for _, tc := range tests {
		if got := initialCursorForStartBlock(tc.start); got != tc.want {
			t.Fatalf("startBlock=%d: got cursor=%d want=%d", tc.start, got, tc.want)
		}
	}
}

func TestV2RegistrationTargetRewindsOnceAndRejectsDrift(t *testing.T) {
	address := "0x1000000000000000000000000000000000000001"
	state := &entity.ChainEventSyncState{Initialized: true, CursorBlock: 500}
	target, rewind, err := v2RegistrationTarget(state, address, 100, 600)
	if err != nil || target != 99 || !rewind {
		t.Fatalf("target=%d rewind=%t err=%v", target, rewind, err)
	}
	state.V2Registered = true
	state.V2ContractAddress = common.HexToAddress(address).Hex()
	state.V2StartBlock = 100
	state.CursorBlock = 550
	target, rewind, err = v2RegistrationTarget(state, address, 100, 600)
	if err != nil || target != 550 || rewind {
		t.Fatalf("registered target=%d rewind=%t err=%v", target, rewind, err)
	}
	if _, _, err := v2RegistrationTarget(state, address, 101, 600); err == nil {
		t.Fatal("V2 start block drift accepted")
	}
	if _, _, err := v2RegistrationTarget(&entity.ChainEventSyncState{}, address, 0, 600); err == nil {
		t.Fatal("zero V2 start block accepted")
	}
}

func TestValidateRegisteredV2IdentityRejectsImplementationDrift(t *testing.T) {
	state := &entity.ChainEventSyncState{V2Registered: true, V2ContractAddress: "0x1000000000000000000000000000000000000001", V2StartBlock: 100, V2ImplementationAddress: "0x2000000000000000000000000000000000000002", V2ImplementationCodeHash: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := validateRegisteredV2Identity(state, state.V2ContractAddress, state.V2StartBlock, state.V2ImplementationAddress, state.V2ImplementationCodeHash); err != nil {
		t.Fatalf("matching identity rejected: %v", err)
	}
	if err := validateRegisteredV2Identity(state, state.V2ContractAddress, state.V2StartBlock, "0x3000000000000000000000000000000000000003", state.V2ImplementationCodeHash); err == nil {
		t.Fatal("implementation address drift accepted")
	}
	if err := validateRegisteredV2Identity(state, state.V2ContractAddress, state.V2StartBlock, state.V2ImplementationAddress, "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"); err == nil {
		t.Fatal("implementation code hash drift accepted")
	}
}

func TestResolveUniqueWalletUID(t *testing.T) {
	if uid, err := resolveUniqueWalletUID([]uint64{7}, []uint64{7}, "0x1"); err != nil || uid != 7 {
		t.Fatalf("same owner rejected: uid=%d err=%v", uid, err)
	}
	if _, err := resolveUniqueWalletUID([]uint64{7}, []uint64{8}, "0x1"); err == nil {
		t.Fatal("conflicting wallet owners accepted")
	}
	if _, err := resolveUniqueWalletUID(nil, nil, "0x1"); err == nil {
		t.Fatal("missing wallet owner accepted")
	}
}

func TestResolveUniqueWalletUIDDoesNotNeedUnverifiedSecondaryCandidates(t *testing.T) {
	// The DB query feeding secondaryUIDs now selects only verified EVM rows.
	// An empty secondary set must preserve the primary owner without requiring
	// legacy unsigned user_wallet rows.
	uid, err := resolveUniqueWalletUID([]uint64{9}, nil, "0x1")
	if err != nil || uid != 9 {
		t.Fatalf("primary wallet owner was not authoritative: uid=%d err=%v", uid, err)
	}
}

func TestRequireInitialV2Governance(t *testing.T) {
	if err := requireInitialV2Governance(true, true); err != nil {
		t.Fatalf("enabled governed tokens rejected: %v", err)
	}
	if err := requireInitialV2Governance(false, true); err == nil {
		t.Fatal("disabled USDT accepted for initial registration")
	}
	if err := requireInitialV2Governance(true, false); err == nil {
		t.Fatal("disabled USDC accepted for initial registration")
	}
}

func TestProcessLogBatchStopsAtFirstFailure(t *testing.T) {
	logs := []types.Log{
		{BlockNumber: 100, TxHash: common.HexToHash("0x1"), Index: 0},
		{BlockNumber: 100, TxHash: common.HexToHash("0x2"), Index: 1},
		{BlockNumber: 101, TxHash: common.HexToHash("0x3"), Index: 0},
	}
	processed := 0
	wantErr := errors.New("business failure")
	err := processLogBatch(logs, func(info types.Log) error {
		processed++
		if info.TxHash == logs[1].TxHash {
			return wantErr
		}
		return nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped business failure, got %v", err)
	}
	if processed != 2 {
		t.Fatalf("expected processing to stop after second log, processed=%d", processed)
	}
}

func TestProcessLogBatchSkipsRemovedLogs(t *testing.T) {
	logs := []types.Log{
		{BlockNumber: 100, Removed: true},
		{BlockNumber: 101},
	}
	processed := 0
	if err := processLogBatch(logs, func(types.Log) error {
		processed++
		return nil
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if processed != 1 {
		t.Fatalf("expected only the canonical log to be processed, got %d", processed)
	}
}

func TestBlockLagNeverUnderflows(t *testing.T) {
	tests := []struct {
		cursor uint64
		safe   uint64
		want   uint64
	}{
		{cursor: 100, safe: 125, want: 25},
		{cursor: 125, safe: 125, want: 0},
		{cursor: 130, safe: 125, want: 0},
	}
	for _, test := range tests {
		if got := blockLag(test.cursor, test.safe); got != test.want {
			t.Fatalf("blockLag(%d,%d)=%d want=%d", test.cursor, test.safe, got, test.want)
		}
	}
}

func TestChainEventMaxRetriesDefaultsAndOverrides(t *testing.T) {
	viper.Set("chainEvent.maxRetries", 0)
	t.Cleanup(viper.Reset)
	if got := chainEventMaxRetries(); got != 3 {
		t.Fatalf("default max retries=%d want=3", got)
	}
	viper.Set("chainEvent.maxRetries", 7)
	if got := chainEventMaxRetries(); got != 7 {
		t.Fatalf("configured max retries=%d want=7", got)
	}
}

func TestDecodeChainLogWithoutTopicsIsIgnored(t *testing.T) {
	data, err := decodeChainLog(types.Log{})
	if err != nil {
		t.Fatalf("empty topics should be ignored without error: %v", err)
	}
	if data != nil {
		t.Fatalf("empty topics should not decode data: %+v", data)
	}
}

func TestTruncateFailureErrorPreservesUTF8(t *testing.T) {
	message := "错误错误错误"
	got := truncateFailureError(message, 5)
	if got != "错" {
		t.Fatalf("unexpected UTF-8 truncation result %q", got)
	}
}

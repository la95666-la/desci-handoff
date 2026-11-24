package chainevent

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	commonChain "joincare_server/pkg/common/chain"
	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/entity"
)

func TestOldRecordSourceDigestBindsCommittedOrder(t *testing.T) {
	rows := []entity.ChainEventRecord{
		{ID: 7, Event: "Purchased", BlockNumber: 100, BlockHash: "0x" + repeatHex("1", 64), TxHash: "0x" + repeatHex("2", 64), LogIndex: 1, ContractAddress: "0x" + repeatHex("3", 40), ContractVersion: constant.ContractVersionV2Stablecoin, Data: `{"event":"Purchased","projectId":1}`},
		{ID: 8, Event: "Claimed", BlockNumber: 101, BlockHash: "0x" + repeatHex("4", 64), TxHash: "0x" + repeatHex("5", 64), LogIndex: 2, ContractAddress: "0x" + repeatHex("3", 40), ContractVersion: constant.ContractVersionV2Stablecoin, Data: `{"event":"Claimed","projectId":1}`},
	}
	first := oldRecordSourceDigest(rows)
	rows[0].Event = "Refunded"
	if first == oldRecordSourceDigest(rows) {
		t.Fatal("old committed event type was not bound")
	}
	rows[0].Event = "Purchased"
	rows[0], rows[1] = rows[1], rows[0]
	if first == oldRecordSourceDigest(rows) {
		t.Fatal("old committed record ordering was not bound")
	}
}

func TestOldSourceEffectUsesImmutableSnapshotIdentity(t *testing.T) {
	event := commonChain.V2Event{Event: commonChain.V2Purchased, ProjectID: 7, ContractAddress: common.HexToAddress("0x1000000000000000000000000000000000000001"), BlockNumber: 10, BlockHash: common.HexToHash("0x01").Hex(), TxHash: common.HexToHash("0x02").Hex()}
	payload, err := jsonv2.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	record := entity.ChainEventRecord{ID: 9, Event: event.Event, BlockNumber: 10, BlockHash: event.BlockHash, TxHash: event.TxHash, ContractAddress: event.ContractAddress.Hex(), ContractVersion: constant.ContractVersionV2Stablecoin, Data: string(payload)}
	root := oldRecordSourceRowDigest(1, record, reorgZeroDigest)
	snapshot := entity.ChainEventReorgMaterializationOldSource{ID: 11, RunID: 3, Ordinal: 1, ChainEventRecordID: record.ID, Event: record.Event, BlockNumber: uint64(record.BlockNumber), BlockHash: strings.ToLower(record.BlockHash), TxHash: strings.ToLower(record.TxHash), ContractAddress: strings.ToLower(record.ContractAddress), ContractVersion: record.ContractVersion, EventData: payload, EventDataDigest: sha256Hex(payload), PreviousRowDigest: reorgZeroDigest, RowDigest: root}
	effect, err := oldSourceEffect(snapshot, reorgZeroDigest)
	if err != nil {
		t.Fatal(err)
	}
	if effect.SourceTable != "chain_event_reorg_materialization_old_source" || effect.SourceID != snapshot.ID || effect.ProjectID != event.ProjectID {
		t.Fatalf("snapshot effect identity drifted: %+v", effect)
	}
	snapshot.EventData[0] ^= 1
	if _, err := oldSourceEffect(snapshot, reorgZeroDigest); err == nil {
		t.Fatal("mutated old snapshot payload was accepted")
	}
}

func TestReplacementLogEffectRejectsRawIdentityDrift(t *testing.T) {
	entry := types.Log{Address: common.HexToAddress("0x1000000000000000000000000000000000000001"), BlockNumber: 50, BlockHash: common.HexToHash("0x01"), TxHash: common.HexToHash("0x02"), Index: 3}
	raw, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	row := entity.ChainEventReorgRecoveryLog{ID: 1, Ordinal: 1, BlockNumber: entry.BlockNumber, BlockHash: entry.BlockHash.Hex(), TxHash: entry.TxHash.Hex(), LogIndex: entry.Index, ContractAddress: entry.Address.Hex(), ContractVersion: constant.ContractVersionV2Stablecoin, RawLog: raw, RawLogDigest: sha256Hex(raw)}
	row.BlockNumber++
	if _, err := replacementLogEffect(row); err == nil {
		t.Fatal("replacement staged-log identity drift was accepted")
	}
}

func TestMaterializationDigestsBindBranchAndCounts(t *testing.T) {
	input := materializationEffectInput{Branch: reorgMaterializationBranchOld, SourceTable: "chain_event_record", SourceID: 7, BlockNumber: 100, BlockHash: "0xabc", TxHash: "0xdef", LogIndex: 1, ContractAddress: "0x123", ContractVersion: constant.ContractVersionV2Stablecoin, EventType: "Purchased", ProjectID: 9, Classification: reorgMaterializationEffectSemantic}
	one := materializationEffectDigest(1, 1, input, 1, repeatHex("a", 64), reorgZeroDigest)
	input.Branch = reorgMaterializationBranchReplacement
	two := materializationEffectDigest(1, 1, input, 1, repeatHex("a", 64), reorgZeroDigest)
	if one == two {
		t.Fatal("materialization branch was not bound")
	}
	manifest := materializationManifestDigest(1, 2, repeatHex("b", 64), 3, 4, 1, one, two)
	if manifest == materializationManifestDigest(1, 2, repeatHex("b", 64), 3, 4, 2, one, two) {
		t.Fatal("ignored-log count was not bound")
	}
}

func TestIgnoredMaterializationLogRequiresApprovedTopic(t *testing.T) {
	approved := types.Log{Topics: []common.Hash{crypto.Keccak256Hash([]byte("OwnershipTransferred(address,address)"))}}
	if !allowedIgnoredMaterializationLog(approved, constant.ContractVersionV2Stablecoin) {
		t.Fatal("approved ownership event was not classified as non-material")
	}
	upgrade := types.Log{Topics: []common.Hash{crypto.Keccak256Hash([]byte("Upgraded(address)"))}}
	if allowedIgnoredMaterializationLog(upgrade, constant.ContractVersionV2Stablecoin) {
		t.Fatal("UUPS upgrade was silently classified as non-material")
	}
	unknown := types.Log{Topics: []common.Hash{crypto.Keccak256Hash([]byte("FutureFinancialEvent(uint256)"))}}
	if allowedIgnoredMaterializationLog(unknown, constant.ContractVersionV2Stablecoin) {
		t.Fatal("unknown replacement event was silently classified as non-material")
	}
}

func TestOldAndReplacementV2SemanticPayloadUseSameEncoding(t *testing.T) {
	event := commonChain.V2Event{Event: commonChain.V2Purchased, ProjectID: 7, ContractAddress: common.HexToAddress("0x1000000000000000000000000000000000000001"), BlockNumber: 10, BlockHash: common.HexToHash("0x01").Hex(), TxHash: common.HexToHash("0x02").Hex(), User: common.HexToAddress("0x2000000000000000000000000000000000000002"), PaymentRaw: "1000000", ProjectTokenRaw: "2500000000000000000"}
	payload, err := jsonv2.Marshal(&event)
	if err != nil {
		t.Fatal(err)
	}
	row := entity.ChainEventRecord{ID: 1, Event: commonChain.V2Purchased, BlockNumber: 10, BlockHash: common.HexToHash("0x01").Hex(), TxHash: common.HexToHash("0x02").Hex(), ContractAddress: event.ContractAddress.Hex(), ContractVersion: constant.ContractVersionV2Stablecoin, Data: string(payload)}
	effect, err := oldRecordEffect(row)
	if err != nil {
		t.Fatal(err)
	}
	if string(effect.SemanticPayload) != string(payload) || effect.ProjectID != event.ProjectID {
		t.Fatalf("old V2 semantic payload drifted: %+v", effect)
	}
	row.TxHash = common.HexToHash("0x03").Hex()
	if _, err := oldRecordEffect(row); err == nil {
		t.Fatal("old V2 payload/record identity mismatch was accepted")
	}
}

func TestPlatformFeesUpdatedIsGlobalSemanticEffect(t *testing.T) {
	payload, err := json.Marshal(&commonChain.Data{Event: commonChain.PlatformFeesUpdated})
	if err != nil {
		t.Fatal(err)
	}
	record := entity.ChainEventRecord{ID: 1, Event: commonChain.PlatformFeesUpdated, BlockNumber: 10, BlockHash: common.HexToHash("0x01").Hex(), TxHash: common.HexToHash("0x02").Hex(), ContractAddress: common.HexToAddress("0x1000000000000000000000000000000000000001").Hex(), ContractVersion: constant.ContractVersionV1Legacy, Data: string(payload)}
	effect, err := oldRecordEffect(record)
	if err != nil {
		t.Fatal(err)
	}
	if effect.ProjectID != 0 || effect.Classification != reorgMaterializationEffectSemantic || effect.EventType != commonChain.PlatformFeesUpdated {
		t.Fatalf("global platform-fee event identity drifted: %+v", effect)
	}
	record.Event = commonChain.ProjectCreated
	if _, err := oldRecordEffect(record); err == nil {
		t.Fatal("V1 payload/event identity mismatch was accepted")
	}
}

func TestReplacementV2GlobalConfigurationEventsAreSemantic(t *testing.T) {
	contract := common.HexToAddress("0x1000000000000000000000000000000000000001")
	target := common.HexToAddress("0x2000000000000000000000000000000000000002")
	word := func(value uint64) []byte { return common.LeftPadBytes(new(big.Int).SetUint64(value).Bytes(), 32) }
	tests := []struct {
		name, signature, event string
		data                   []byte
	}{
		{name: "payment token disabled", signature: "PaymentTokenConfigured(address,bool,uint8)", event: reorgV2PaymentTokenConfigured, data: append(word(0), word(6)...)},
		{name: "project token enabled", signature: "ProjectTokenConfigured(address,bool,uint8)", event: reorgV2ProjectTokenConfigured, data: append(word(1), word(18)...)},
		{name: "platform fee", signature: "PlatformFeesUpdated(uint16,address)", event: reorgV2PlatformFeesUpdated, data: word(250)},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry := types.Log{Address: contract, Topics: []common.Hash{crypto.Keccak256Hash([]byte(test.signature)), common.BytesToHash(common.LeftPadBytes(target.Bytes(), 32))}, Data: test.data, BlockNumber: 50, BlockHash: common.HexToHash("0x01"), TxHash: common.HexToHash("0x02"), Index: uint(index)}
			raw, err := json.Marshal(entry)
			if err != nil {
				t.Fatal(err)
			}
			row := entity.ChainEventReorgRecoveryLog{ID: uint64(index + 1), BlockNumber: entry.BlockNumber, BlockHash: entry.BlockHash.Hex(), TxHash: entry.TxHash.Hex(), LogIndex: entry.Index, ContractAddress: entry.Address.Hex(), ContractVersion: constant.ContractVersionV2Stablecoin, RawLog: raw, RawLogDigest: sha256Hex(raw)}
			effect, err := replacementLogEffect(row)
			if err != nil {
				t.Fatal(err)
			}
			if effect.EventType != test.event || effect.ProjectID != 0 || effect.Classification != reorgMaterializationEffectSemantic || len(effect.SemanticPayload) == 0 {
				t.Fatalf("global V2 effect identity drifted: %+v", effect)
			}
			if test.name == "payment token disabled" && !strings.Contains(string(effect.SemanticPayload), `"allowed":false`) {
				t.Fatalf("disabled-token semantic payload omitted explicit false: %s", effect.SemanticPayload)
			}
		})
	}
}

func TestMaterializationV2GlobalConfigurationRejectsInvalidValues(t *testing.T) {
	address := common.HexToAddress("0x2000000000000000000000000000000000000002")
	word := func(value uint64) []byte { return common.LeftPadBytes(new(big.Int).SetUint64(value).Bytes(), 32) }
	tests := []types.Log{
		{Address: address, Topics: []common.Hash{crypto.Keccak256Hash([]byte("PaymentTokenConfigured(address,bool,uint8)")), common.BytesToHash(common.LeftPadBytes(address.Bytes(), 32))}, Data: append(word(2), word(6)...)},
		{Address: address, Topics: []common.Hash{crypto.Keccak256Hash([]byte("ProjectTokenConfigured(address,bool,uint8)")), common.BytesToHash(common.LeftPadBytes(address.Bytes(), 32))}, Data: append(word(1), word(19)...)},
		{Address: address, Topics: []common.Hash{crypto.Keccak256Hash([]byte("PlatformFeesUpdated(uint16,address)")), common.BytesToHash(common.LeftPadBytes(address.Bytes(), 32))}, Data: word(1001)},
	}
	for index, entry := range tests {
		if _, err := decodeMaterializationV2GlobalLog(entry); err == nil {
			t.Fatalf("invalid V2 global configuration %d was accepted", index)
		}
	}
}

func repeatHex(value string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += value
	}
	return result
}

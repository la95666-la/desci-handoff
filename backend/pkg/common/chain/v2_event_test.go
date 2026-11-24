package chain

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func word(value *big.Int) []byte                   { return common.LeftPadBytes(value.Bytes(), 32) }
func addressWord(address common.Address) []byte    { return common.LeftPadBytes(address.Bytes(), 32) }
func topicUint(value uint64) common.Hash           { return common.BigToHash(new(big.Int).SetUint64(value)) }
func topicAddr(address common.Address) common.Hash { return common.BytesToHash(address.Bytes()) }

func TestDecodeV2PurchasedPreservesRawAmounts(t *testing.T) {
	contract := common.HexToAddress("0x1000000000000000000000000000000000000001")
	user := common.HexToAddress("0x2000000000000000000000000000000000000002")
	payment := common.HexToAddress("0x3000000000000000000000000000000000000003")
	projectToken := common.HexToAddress("0x4000000000000000000000000000000000000004")
	logEntry := types.Log{
		Address: contract, BlockNumber: 123, BlockHash: common.HexToHash("0xabc"), TxHash: common.HexToHash("0xdef"), Index: 7,
		Topics: []common.Hash{
			crypto.Keccak256Hash([]byte("Purchased(uint256,address,address,uint256,address,uint256)")),
			topicUint(42), topicAddr(user), topicAddr(payment),
		},
		Data: append(append(word(big.NewInt(1_000_001)), addressWord(projectToken)...), word(big.NewInt(2_500_000_000_000_000_000))...),
	}
	got, err := DecodeV2Log(logEntry)
	if err != nil {
		t.Fatal(err)
	}
	if got.Event != V2Purchased || got.ProjectID != 42 || got.User != user || got.PaymentToken != payment || got.ProjectToken != projectToken {
		t.Fatalf("unexpected event: %+v", got)
	}
	if got.PaymentRaw != "1000001" || got.ProjectTokenRaw != "2500000000000000000" {
		t.Fatalf("raw amounts changed: %+v", got)
	}
}

func TestDecodeV2ProjectCreatedSnapshots(t *testing.T) {
	projectToken := common.HexToAddress("0x4000000000000000000000000000000000000004")
	payment := common.HexToAddress("0x3000000000000000000000000000000000000003")
	owner := common.HexToAddress("0x5000000000000000000000000000000000000005")
	feeReceiver := common.HexToAddress("0x6000000000000000000000000000000000000006")
	data := make([]byte, 0, 11*32)
	data = append(data, addressWord(owner)...)
	data = append(data, addressWord(feeReceiver)...)
	for _, value := range []int64{18, 6, 200, 1_000_001, 1_000_000_000, 500_000_000, 100_000_000, 1_800_000_000, 1_800_086_400} {
		data = append(data, word(big.NewInt(value))...)
	}
	got, err := DecodeV2Log(types.Log{
		Address: common.HexToAddress("0x1000000000000000000000000000000000000001"),
		Topics:  []common.Hash{crypto.Keccak256Hash([]byte("ProjectCreated(uint256,address,address,address,address,uint8,uint8,uint16,uint256,uint256,uint256,uint256,uint256,uint256)")), topicUint(42), topicAddr(projectToken), topicAddr(payment)},
		Data:    data, BlockNumber: 100, BlockHash: common.HexToHash("0x1"), TxHash: common.HexToHash("0x2"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectOwner != owner || got.FeeReceiver != feeReceiver || got.ProjectDecimals != 18 || got.PaymentDecimals != 6 || got.FeeRate != 200 || got.TokenPriceRaw != "1000001" || got.EndTimeRaw != "1800086400" {
		t.Fatalf("unexpected create snapshot: %+v", got)
	}
}

func TestDecodeV2LogRejectsMalformedABI(t *testing.T) {
	topic := crypto.Keccak256Hash([]byte("Refunded(uint256,address,address,uint256)"))
	tests := []types.Log{
		{Topics: []common.Hash{topic, topicUint(1)}, Data: word(big.NewInt(1))},
		{Topics: []common.Hash{topic, topicUint(1), topicUint(2), topicUint(3)}, Data: []byte{1}},
		{Topics: []common.Hash{topic, common.HexToHash("0x10000000000000000"), topicUint(2), topicUint(3)}, Data: word(big.NewInt(1))},
	}
	for i, logEntry := range tests {
		if _, err := DecodeV2Log(logEntry); err == nil {
			t.Fatalf("malformed log %d accepted", i)
		}
	}
}

func TestDecodeV2UnknownTopicIsIgnored(t *testing.T) {
	got, err := DecodeV2Log(types.Log{Topics: []common.Hash{common.HexToHash("0x1234")}})
	if err != nil || got != nil {
		t.Fatalf("unknown topic got=%+v err=%v", got, err)
	}
}

func TestV2TopicPolicySeparatesGovernanceUpgradeAndUnknown(t *testing.T) {
	if !IsApprovedV2NonMaterialTopic(crypto.Keccak256Hash([]byte("OperatorUpdated(address,address)"))) {
		t.Fatal("approved operator governance event was not allowlisted")
	}
	if IsApprovedV2NonMaterialTopic(V2UpgradedTopic) {
		t.Fatal("Upgraded must always require controlled runtime admission")
	}
	if IsApprovedV2NonMaterialTopic(crypto.Keccak256Hash([]byte("FutureFinancialEvent(uint256)"))) {
		t.Fatal("unknown financial topic was allowlisted")
	}
}

func TestDecodeV2ProjectCreatedRejectsGovernanceBounds(t *testing.T) {
	projectToken := common.HexToAddress("0x4000000000000000000000000000000000000004")
	payment := common.HexToAddress("0x3000000000000000000000000000000000000003")
	data := make([]byte, 0, 11*32)
	data = append(data, addressWord(common.Address{})...)
	data = append(data, addressWord(common.HexToAddress("0x6000000000000000000000000000000000000006"))...)
	for _, value := range []int64{19, 6, 1001, 1, 1, 1, 1, 1, 2} {
		data = append(data, word(big.NewInt(value))...)
	}
	_, err := DecodeV2Log(types.Log{Topics: []common.Hash{
		crypto.Keccak256Hash([]byte("ProjectCreated(uint256,address,address,address,address,uint8,uint8,uint16,uint256,uint256,uint256,uint256,uint256,uint256)")), topicUint(1), topicAddr(projectToken), topicAddr(payment),
	}, Data: data})
	if err == nil {
		t.Fatal("invalid V2 project governance snapshot accepted")
	}
}

package chain

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func fairTopicUint(value uint64) common.Hash { return common.BigToHash(new(big.Int).SetUint64(value)) }
func topicAddressValue(value common.Address) common.Hash {
	return common.BytesToHash(common.LeftPadBytes(value.Bytes(), 32))
}

func TestPackFairLaunchActionsUseFrozenSelectors(t *testing.T) {
	cases := []struct {
		signature string
		pack      func() ([]byte, error)
	}{
		{"activateProject(uint256)", func() ([]byte, error) { return PackFairLaunchActivate(big.NewInt(8)) }},
		{"endProject(uint256)", func() ([]byte, error) { return PackFairLaunchEnd(big.NewInt(8)) }},
		{"cancelProject(uint256,bytes32,bytes32)", func() ([]byte, error) {
			return PackFairLaunchCancel(big.NewInt(8), common.HexToHash("0x1"), common.HexToHash("0x2"))
		}},
		{"withdrawProjectProceeds(uint256)", func() ([]byte, error) { return PackFairLaunchWithdrawProjectProceeds(big.NewInt(8)) }},
		{"withdrawPlatformFee(uint256)", func() ([]byte, error) { return PackFairLaunchWithdrawPlatformFee(big.NewInt(8)) }},
	}
	for _, tc := range cases {
		data, err := tc.pack()
		if err != nil {
			t.Fatal(err)
		}
		want := FairLaunchSelector(tc.signature)
		if len(data) < 4 || string(data[:4]) != string(want.Bytes()[:4]) {
			t.Fatalf("%s selector mismatch: %x", tc.signature, data)
		}
	}
}

func TestDecodeFairLaunchContributionPreservesRawUint256(t *testing.T) {
	user := common.HexToAddress("0x2000000000000000000000000000000000000002")
	token := common.HexToAddress("0x3000000000000000000000000000000000000003")
	amount, _ := new(big.Int).SetString("900719925474099312345678901234567890", 10)
	userTotal := new(big.Int).Add(amount, big.NewInt(1))
	projectTotal := new(big.Int).Add(amount, big.NewInt(2))
	data, err := fairLaunchV1ABI.Events[FairLaunchContribution].Inputs.NonIndexed().Pack(amount, userTotal, projectTotal)
	if err != nil {
		t.Fatal(err)
	}
	topic, _ := FairLaunchEventTopic(FairLaunchContribution)
	decoded, err := DecodeFairLaunchLog(types.Log{
		Address: common.HexToAddress("0x1000000000000000000000000000000000000001"),
		Topics:  []common.Hash{topic, fairTopicUint(8), topicAddressValue(user), topicAddressValue(token)}, Data: data,
	})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.ProjectID != 8 || decoded.User != user || decoded.Token != token || decoded.AmountRaw != amount.String() || decoded.TotalRaisedRaw != projectTotal.String() {
		t.Fatalf("wrong contribution decode: %+v", decoded)
	}
}

func TestDecodeFairLaunchEndedPreservesSettlementAnchor(t *testing.T) {
	payment := common.HexToAddress("0x3000000000000000000000000000000000000003")
	anchor := common.HexToHash("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	data, err := fairLaunchV1ABI.Events[FairLaunchEnded].Inputs.NonIndexed().Pack(
		uint8(2), uint8(0), big.NewInt(150), big.NewInt(150), big.NewInt(1000), big.NewInt(1234), big.NewInt(1233), anchor,
	)
	if err != nil {
		t.Fatal(err)
	}
	topic, _ := FairLaunchEventTopic(FairLaunchEnded)
	decoded, err := DecodeFairLaunchLog(types.Log{Topics: []common.Hash{topic, fairTopicUint(8), topicAddressValue(payment)}, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Status != 2 || decoded.TotalRaisedRaw != "150" || decoded.FinalPriceDenominatorRaw != "1000" || decoded.SettlementBlockNumber != "1234" || decoded.SettlementAnchorBlockHash != anchor.Hex() {
		t.Fatalf("wrong ended decode: %+v", decoded)
	}
}

func TestDecodeFairLaunchLogRejectsUnknownAndMalformed(t *testing.T) {
	unknown, err := DecodeFairLaunchLog(types.Log{Topics: []common.Hash{common.HexToHash("0x1234")}})
	if err != nil || unknown != nil {
		t.Fatalf("unknown topic result=%+v err=%v", unknown, err)
	}
	topic, _ := FairLaunchEventTopic(FairLaunchContribution)
	if _, err := DecodeFairLaunchLog(types.Log{Topics: []common.Hash{topic, fairTopicUint(8)}}); err == nil {
		t.Fatal("malformed contribution accepted")
	}
}

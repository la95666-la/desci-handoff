package chain

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestUserCheckedInTopicMatchesProductionEvent(t *testing.T) {
	want := crypto.Keccak256Hash([]byte("UserCheckedIn(address,uint256,uint256)"))
	if got := EventTopic(UserCheckedIn); got != want {
		t.Fatalf("UserCheckedIn topic = %s, want %s", got.Hex(), want.Hex())
	}
}

func TestTaskABIContainsProductionCheckInSurface(t *testing.T) {
	contractABI, err := abi.JSON(strings.NewReader(taskABIJson))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := contractABI.Methods["hasCheckedInToday"]; !ok {
		t.Fatal("task ABI is missing hasCheckedInToday")
	}
	event, ok := contractABI.Events[UserCheckedIn]
	if !ok {
		t.Fatal("task ABI is missing UserCheckedIn")
	}
	if len(event.Inputs) != 3 {
		t.Fatalf("UserCheckedIn input count = %d, want 3", len(event.Inputs))
	}
}

func TestHandleUserCheckedInThreeParameterEvent(t *testing.T) {
	user := common.HexToAddress("0x1234567890123456789012345678901234567890")
	day := int64(20648)
	timestamp := int64(1783987200)
	data := common.LeftPadBytes(big.NewInt(timestamp).Bytes(), 32)

	got := handleUserCheckedIn(types.Log{
		Topics: []common.Hash{
			EventTopic(UserCheckedIn),
			common.BytesToHash(user.Bytes()),
			common.BigToHash(big.NewInt(day)),
		},
		Data: data,
	})

	if got.User != user || got.Day != day || got.Timestamp != timestamp {
		t.Fatalf("decoded event = user %s day %d timestamp %d", got.User.Hex(), got.Day, got.Timestamp)
	}
}

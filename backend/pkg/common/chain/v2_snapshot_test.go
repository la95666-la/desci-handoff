package chain

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

func TestIDOV2SnapshotABIDecodesProjectAndPurchase(t *testing.T) {
	parsed, err := abi.JSON(strings.NewReader(idoV2ABIJson))
	if err != nil {
		t.Fatal(err)
	}
	projectValue := IDOV2ProjectSnapshot{
		ProjectID: big.NewInt(7), ProjectToken: common.HexToAddress("0x3000000000000000000000000000000000000003"), PaymentToken: common.HexToAddress("0x2000000000000000000000000000000000000002"),
		ProjectOwner: common.HexToAddress("0x4000000000000000000000000000000000000004"), FeeReceiver: common.HexToAddress("0x5000000000000000000000000000000000000005"),
		ProjectTokenDecimals: 18, PaymentTokenDecimals: 6, SuccessFeeRate: 200, Status: 2,
		TokenPrice: big.NewInt(1_000_000), HardCap: big.NewInt(1_000_000_000), SoftCap: big.NewInt(500_000_000), MaxPerUser: big.NewInt(100_000_000),
		StartTime: big.NewInt(1_800_000_000), EndTime: big.NewInt(1_800_086_400), TotalRaised: big.NewInt(750_000_000), TotalTokensSold: mustBig("750000000000000000000"), TotalTokensDeposited: mustBig("1000000000000000000000"),
		FundsWithdrawn: true, PlatformFeeWithdrawn: true, ProjectTokensWithdrawn: false,
		VestingTimes: []*big.Int{big.NewInt(1_800_100_000)}, VestingPercents: []*big.Int{big.NewInt(10_000)},
	}
	encoded, err := parsed.Methods["getProject"].Outputs.Pack(projectValue)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := parsed.Unpack("getProject", encoded)
	if err != nil || len(decoded) != 1 {
		t.Fatalf("decode project err=%v values=%d", err, len(decoded))
	}
	project := *abiConvertIDOV2Project(decoded[0])
	if project.ProjectID.Uint64() != 7 || project.Status != 2 || project.PaymentTokenDecimals != 6 || project.TotalRaised.String() != "750000000" || !project.FundsWithdrawn || !project.PlatformFeeWithdrawn {
		t.Fatalf("unexpected project snapshot: %+v", project)
	}

	purchaseEncoded, err := parsed.Methods["getUserPurchase"].Outputs.Pack(IDOV2UserPurchaseSnapshot{PaymentAmount: big.NewInt(100), TokensBought: big.NewInt(250), ClaimedTokens: big.NewInt(50), Refunded: true})
	if err != nil {
		t.Fatal(err)
	}
	purchaseDecoded, err := parsed.Unpack("getUserPurchase", purchaseEncoded)
	if err != nil || len(purchaseDecoded) != 1 {
		t.Fatalf("decode purchase err=%v values=%d", err, len(purchaseDecoded))
	}
	purchase := *abiConvertIDOV2UserPurchase(purchaseDecoded[0])
	if purchase.PaymentAmount.String() != "100" || purchase.TokensBought.String() != "250" || purchase.ClaimedTokens.String() != "50" || !purchase.Refunded {
		t.Fatalf("unexpected purchase snapshot: %+v", purchase)
	}
}

func mustBig(value string) *big.Int {
	parsed, ok := new(big.Int).SetString(value, 10)
	if !ok {
		panic(value)
	}
	return parsed
}

func TestResolveConfirmations(t *testing.T) {
	for _, fixture := range []struct {
		chainID    uint64
		configured string
		want       uint64
	}{
		{97, "", 15}, {31337, "", 0}, {97, "20", 20}, {31337, "5", 5},
	} {
		got, err := ResolveConfirmations(fixture.chainID, fixture.configured)
		if err != nil || got != fixture.want {
			t.Fatalf("fixture %+v got=%d err=%v", fixture, got, err)
		}
	}
	if _, err := ResolveConfirmations(97, "not-a-number"); err == nil {
		t.Fatal("invalid confirmations accepted")
	}
}
